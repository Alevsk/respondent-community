package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/ai/analysis"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/llm"
	sqlsafety "github.com/Alevsk/respondent/internal/sqlsafety"
)

// ErrLLMUnavailable is returned when an operation requires the LLM provider
// but none is configured (e.g., AI is disabled or the provider failed its
// startup sanity check). Read-only operations like GetInsights are unaffected.
var ErrLLMUnavailable = errors.New("no LLM provider configured")

// validLayerTypeRe restricts layer_type values to safe characters only.
// This prevents prompt injection when interpolating into LLM prompts.
var validLayerTypeRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// QueryExecutor abstracts read-only SQL execution for NL search.
// This avoids coupling to pgxpool directly and enables testing.
type QueryExecutor interface {
	// QueryRows executes a read-only SQL query and returns rows as generic maps.
	QueryRows(ctx context.Context, sql string) ([]map[string]any, error)

	// QueryRowsWithTimeout executes a read-only SQL query with an optional
	// SET LOCAL statement (e.g., statement_timeout) applied first within
	// the same transaction. If setStmt is empty it behaves like QueryRows.
	QueryRowsWithTimeout(ctx context.Context, setStmt, queryStmt string) ([]map[string]any, error)
}

// AnalysisDefinitionLoader provides access to loaded analysis definitions.
type AnalysisDefinitionLoader interface {
	// ListDefinitions returns all loaded analysis definitions.
	ListDefinitions() []*analysis.AnalysisDefinition
}

// AIService handles AI-powered search, analysis, and insight operations.
// It depends on repo interfaces and an LLM provider, not concrete implementations.
type AIService struct {
	entities       domain.EntityRepository
	observations   domain.ObservationRepository
	insights       domain.AIInsightRepository
	llmProvider    llm.Provider
	queryExec      QueryExecutor
	queryLog       domain.AIQueryLogRepository
	analysisLoader AnalysisDefinitionLoader
	logger         zerolog.Logger
}

// NewAIService creates a new AIService.
// The optional logger variadic allows callers to inject a zerolog.Logger. If
// omitted a no-op logger is used, preserving backward compatibility with tests.
func NewAIService(
	entities domain.EntityRepository,
	observations domain.ObservationRepository,
	insights domain.AIInsightRepository,
	llmProvider llm.Provider,
	queryExec QueryExecutor,
	queryLog domain.AIQueryLogRepository,
	analysisLoader AnalysisDefinitionLoader,
	logger ...zerolog.Logger,
) *AIService {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &AIService{
		entities:       entities,
		observations:   observations,
		insights:       insights,
		llmProvider:    llmProvider,
		queryExec:      queryExec,
		queryLog:       queryLog,
		analysisLoader: analysisLoader,
		logger:         l,
	}
}

// databaseSchemaContext provides table structure for the LLM to generate SQL.
// This is hardcoded for now; a future iteration will introspect the schema dynamically.
const databaseSchemaContext = `Database schema:

TABLE entities (
  id UUID PRIMARY KEY,
  external_id TEXT,
  layer_type TEXT NOT NULL,
  name TEXT,
  metadata JSONB,
  ai_metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT now()
);

TABLE observations (
  id UUID PRIMARY KEY,
  entity_id UUID REFERENCES entities(id),
  ts TIMESTAMPTZ NOT NULL,
  lat DOUBLE PRECISION,
  lon DOUBLE PRECISION,
  altitude_m DOUBLE PRECISION,
  velocity JSONB,
  metadata JSONB,
  source_type TEXT,
  event_time TIMESTAMPTZ,
  event_end TIMESTAMPTZ,
  ai_metadata JSONB,
  created_at TIMESTAMPTZ DEFAULT now()
);

Common layer_type values: flights_commercial, flights_military, satellites, earthquakes, traffic, cctv

Important: Generate SELECT-only queries. Never use INSERT, UPDATE, DELETE, DROP, ALTER, CREATE, or TRUNCATE.`

// nowFunc is a package-level variable that returns the current time.
// Tests can override this for deterministic behavior.
var nowFunc = time.Now

// NaturalLanguageSearch generates SQL from a natural language query via LLM,
// validates it, executes it, and returns matching entity results.
func (s *AIService) NaturalLanguageSearch(ctx context.Context, query string, layerType string, limit int) (*domain.NLSearchResponse, error) {
	if s.llmProvider == nil {
		return nil, ErrLLMUnavailable
	}
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	startTime := nowFunc()

	// Build LLM prompt for SQL generation.
	systemPrompt := fmt.Sprintf(`You are a SQL expert assistant. Given the following database schema, generate a PostgreSQL SELECT query that answers the user's question.

%s

Rules:
- Return ONLY the SQL query, no explanation, no markdown formatting.
- Always include a LIMIT clause (max %d).
- Always include layer_type, external_id, name columns in the SELECT.
- If a layer_type filter is specified, include it in the WHERE clause.
- Use ILIKE for text matching.
- Join entities and observations when the query involves position, altitude, or time data.
- Never generate destructive queries (INSERT, UPDATE, DELETE, DROP, etc).`, databaseSchemaContext, limit)

	userPrompt := query
	if layerType != "" {
		if !validLayerTypeRe.MatchString(layerType) {
			return nil, fmt.Errorf("invalid layer_type: contains disallowed characters")
		}
		userPrompt = fmt.Sprintf("%s (filter to layer_type = '%s')", query, layerType)
	}

	resp, err := s.llmProvider.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.0,
		MaxTokens:   1024,
	})
	if err != nil {
		s.writeQueryLog(ctx, query, "", "", resp, startTime, 0, err)
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	generatedSQL := strings.TrimSpace(resp.Content)
	// Strip markdown code fences if present.
	generatedSQL = stripCodeFences(generatedSQL)

	// Validate generated SQL using the PostgreSQL AST parser (defense-in-depth).
	// This ensures only single read-only SELECT statements pass through.
	if err := sqlsafety.ValidateReadOnlySQL(generatedSQL); err != nil {
		s.writeQueryLog(ctx, query, generatedSQL, "", resp, startTime, 0, err)
		return nil, fmt.Errorf("generated SQL failed safety check: %w", err)
	}

	// Enforce safety bounds: LIMIT + statement_timeout via SET LOCAL.
	setStmt, safeSQL := sqlsafety.EnforceLimitAndTimeout(generatedSQL, limit, 5*time.Second)

	s.logger.Debug().
		Str("query", query).
		Str("generated_sql", safeSQL).
		Msg("NL search: executing generated SQL")

	// Execute the generated SQL.
	if s.queryExec == nil {
		explanation := "Query generated but no executor configured"
		s.writeQueryLog(ctx, query, generatedSQL, explanation, resp, startTime, 0, nil)
		return &domain.NLSearchResponse{
			GeneratedSQL: generatedSQL,
			Explanation:  explanation,
		}, nil
	}

	rows, err := s.queryExec.QueryRowsWithTimeout(ctx, setStmt, safeSQL)
	if err != nil {
		s.writeQueryLog(ctx, query, generatedSQL, "", resp, startTime, 0, err)
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	// Convert rows to domain.NLSearchResult.
	results := make([]*domain.NLSearchResult, 0, len(rows))
	for _, row := range rows {
		r := &domain.NLSearchResult{}
		if v, ok := row["external_id"].(string); ok {
			r.ExternalID = v
		}
		if v, ok := row["layer_type"].(string); ok {
			r.LayerType = v
		}
		if v, ok := row["name"].(string); ok {
			r.Name = v
		}
		// Build composite entity ID.
		if r.LayerType != "" && r.ExternalID != "" {
			r.EntityID = r.LayerType + ":" + r.ExternalID
		}
		// Extract any additional metadata fields.
		r.Metadata = extractMetadataFromRow(row)
		results = append(results, r)
	}

	explanation := fmt.Sprintf("Found %d results for: %s", len(results), query)
	s.writeQueryLog(ctx, query, generatedSQL, explanation, resp, startTime, len(results), nil)

	return &domain.NLSearchResponse{
		GeneratedSQL: generatedSQL,
		Results:      results,
		TotalCount:   len(results),
		Explanation:  explanation,
	}, nil
}

// writeQueryLog writes a best-effort audit entry to the ai_query_log table.
// Failures are logged as warnings but never propagated to the caller.
func (s *AIService) writeQueryLog(
	ctx context.Context,
	userQuery, generatedSQL, explanation string,
	llmResp *llm.CompletionResponse,
	startTime time.Time,
	resultCount int,
	queryErr error,
) {
	if s.queryLog == nil {
		return
	}

	entry := &domain.AIQueryLog{
		UserQuery:    userQuery,
		GeneratedSQL: generatedSQL,
		Explanation:  explanation,
		ResultCount:  resultCount,
		LatencyMS:    int(nowFunc().Sub(startTime).Milliseconds()),
	}

	if llmResp != nil {
		entry.Provider = s.llmProvider.Name()
		entry.Model = llmResp.Model
		entry.PromptTokens = llmResp.Usage.PromptTokens
		entry.CompletionTokens = llmResp.Usage.CompletionTokens
	}

	if queryErr != nil {
		entry.ErrorMessage = queryErr.Error()
	}

	if err := s.queryLog.Create(ctx, entry); err != nil {
		s.logger.Warn().Err(err).Str("user_query", userQuery).Msg("failed to write AI query log")
	}
}

// AnalyzeEntity fetches entity data and observations, sends them to the LLM for analysis,
// and returns the AI-generated analysis.
func (s *AIService) AnalyzeEntity(ctx context.Context, entityID string, obsLimit int) (*domain.AnalyzeEntityResult, error) {
	if s.llmProvider == nil {
		return nil, ErrLLMUnavailable
	}
	if entityID == "" {
		return nil, fmt.Errorf("entity_id must not be empty")
	}

	if obsLimit <= 0 {
		obsLimit = 10
	}

	// Resolve the composite entity ID to the DB entity.
	layerType, externalID := domain.ParseEntityID(entityID)
	if layerType == "" || externalID == "" {
		return nil, fmt.Errorf("invalid entity ID format: %s", entityID)
	}

	entity, err := s.entities.GetByExternalID(ctx, string(layerType), externalID)
	if err != nil {
		return nil, fmt.Errorf("entity not found: %w", err)
	}

	// Fetch recent observations using current time as the "before" cursor.
	observations, err := s.observations.GetByEntityID(ctx, entity.ID, obsLimit, nowFunc())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch observations: %w", err)
	}

	// Build the analysis prompt.
	prompt := buildAnalysisPrompt(entity, observations)

	resp, err := s.llmProvider.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a geospatial intelligence analyst. Analyze the provided entity data and observations. Provide a concise, actionable analysis including patterns, anomalies, and notable characteristics."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2048,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	analysisText := strings.TrimSpace(resp.Content)

	// Store as an insight.
	insightID := ""
	if s.insights != nil {
		insight := &domain.AIInsight{
			InsightType:   "entity_analysis",
			SourceName:    "ai_service",
			OperationName: "analyze_entity",
			Result: map[string]any{
				"analysis": analysisText,
			},
			EntityIDs: []string{entity.ID},
		}
		if lt := string(layerType); lt != "" {
			insight.LayerType = &lt
		}
		id, createErr := s.insights.Create(ctx, insight)
		if createErr != nil {
			s.logger.Warn().Err(createErr).Str("entity_id", entityID).Msg("failed to store analysis insight")
		} else {
			insightID = id
			// Persist entity association in the join table.
			if refErr := s.insights.CreateRef(ctx, id, &entity.ID, nil); refErr != nil {
				s.logger.Warn().Err(refErr).Str("insight_id", id).Str("entity_id", entityID).
					Msg("failed to create insight ref")
			}
		}
	}

	return &domain.AnalyzeEntityResult{
		EntityID:  entityID,
		Analysis:  analysisText,
		InsightID: insightID,
	}, nil
}

// GetInsights returns stored AI insights matching the given filter.
func (s *AIService) GetInsights(ctx context.Context, filter domain.InsightFilter) ([]*domain.AIInsight, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	return s.insights.List(ctx, filter)
}

// ExplainQuery sends a SQL query to the LLM for a plain-English explanation.
func (s *AIService) ExplainQuery(ctx context.Context, sql string) (string, error) {
	if s.llmProvider == nil {
		return "", ErrLLMUnavailable
	}
	if sql == "" {
		return "", fmt.Errorf("sql must not be empty")
	}

	resp, err := s.llmProvider.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "system", Content: fmt.Sprintf(`You are a SQL expert. Explain the following SQL query in plain English. Be concise but thorough. Describe what data is being selected, any filters/joins, and the expected output.

Context about the database:
%s`, databaseSchemaContext)},
			{Role: "user", Content: fmt.Sprintf("Explain this SQL query:\n\n%s", sql)},
		},
		Temperature: 0.1,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", fmt.Errorf("LLM explanation failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

// ListAnalysisDefinitions returns summary info about all loaded analysis definitions.
func (s *AIService) ListAnalysisDefinitions(_ context.Context) []*domain.AnalysisDefInfo {
	if s.analysisLoader == nil {
		return nil
	}

	defs := s.analysisLoader.ListDefinitions()
	results := make([]*domain.AnalysisDefInfo, 0, len(defs))
	for _, def := range defs {
		schedule := def.Schedule.Interval
		if schedule == "" {
			schedule = def.Schedule.Cron
		}
		var insightTypes []string
		for _, op := range def.AI.Operations {
			if op.Output.InsightType != "" {
				insightTypes = append(insightTypes, op.Output.InsightType)
			}
		}
		results = append(results, &domain.AnalysisDefInfo{
			Name:         def.Name,
			DisplayName:  def.DisplayName,
			Enabled:      def.Enabled,
			Schedule:     schedule,
			Layers:       def.Data.Layers,
			InsightTypes: insightTypes,
		})
	}
	return results
}

// GetNotificationFilterOptions returns the available filter options derived
// from loaded analysis definitions.
func (s *AIService) GetNotificationFilterOptions(_ context.Context) *domain.NotificationFilterOptions {
	opts := &domain.NotificationFilterOptions{
		AttentionLevels: []string{"info", "low", "medium", "high", "critical"},
	}

	if s.analysisLoader == nil {
		return opts
	}

	defs := s.analysisLoader.ListDefinitions()
	seenTypes := make(map[string]bool)
	seenLayers := make(map[string]bool)

	for _, def := range defs {
		if !def.Enabled {
			continue
		}
		for _, op := range def.AI.Operations {
			if op.Output.InsightType != "" && !seenTypes[op.Output.InsightType] {
				seenTypes[op.Output.InsightType] = true
				opts.InsightTypes = append(opts.InsightTypes, domain.InsightTypeOption{
					Value:       op.Output.InsightType,
					DisplayName: humanize(op.Output.InsightType),
					SourceName:  def.Name,
				})
			}
		}
		for _, layer := range def.Data.Layers {
			if !seenLayers[layer] {
				seenLayers[layer] = true
				opts.LayerTypes = append(opts.LayerTypes, layer)
			}
		}
	}

	return opts
}

// humanize converts snake_case to Title Case.
func humanize(s string) string {
	words := strings.Split(s, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// ── Internal helpers ────────────────────────────────────────────────

// stripCodeFences removes markdown code fence wrappers from a string.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```sql") {
		s = strings.TrimPrefix(s, "```sql")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// buildAnalysisPrompt constructs a detailed prompt for entity analysis.
func buildAnalysisPrompt(entity *domain.Entity, observations []*domain.Observation) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Entity: %s\n", entity.Name))
	b.WriteString(fmt.Sprintf("Type: %s\n", entity.LayerType))
	b.WriteString(fmt.Sprintf("External ID: %s\n", entity.ExternalID))

	if len(entity.Metadata) > 0 {
		metaJSON, _ := json.Marshal(entity.Metadata)
		b.WriteString(fmt.Sprintf("Metadata: %s\n", string(metaJSON)))
	}

	if len(entity.AIMetadata) > 0 {
		aiMetaJSON, _ := json.Marshal(entity.AIMetadata)
		b.WriteString(fmt.Sprintf("AI Metadata: %s\n", string(aiMetaJSON)))
	}

	b.WriteString(fmt.Sprintf("\nObservations (%d):\n", len(observations)))
	for i, obs := range observations {
		b.WriteString(fmt.Sprintf("  [%d] Time: %s", i+1, obs.Timestamp.Format("2006-01-02 15:04:05 MST")))
		if obs.Position != nil {
			b.WriteString(fmt.Sprintf(", Position: (%.4f, %.4f)", obs.Position.Lat, obs.Position.Lon))
		}
		if obs.AltitudeM != 0 {
			b.WriteString(fmt.Sprintf(", Altitude: %.0fm", obs.AltitudeM))
		}
		if len(obs.Velocity) > 0 {
			velJSON, _ := json.Marshal(obs.Velocity)
			b.WriteString(fmt.Sprintf(", Velocity: %s", string(velJSON)))
		}
		if len(obs.Metadata) > 0 {
			metaJSON, _ := json.Marshal(obs.Metadata)
			b.WriteString(fmt.Sprintf(", Metadata: %s", string(metaJSON)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// extractMetadataFromRow extracts non-standard columns from a query result row.
func extractMetadataFromRow(row map[string]any) map[string]string {
	meta := make(map[string]string)
	standardKeys := map[string]bool{
		"id": true, "external_id": true, "layer_type": true,
		"name": true, "entity_id": true,
	}
	for k, v := range row {
		if standardKeys[k] {
			continue
		}
		if v != nil {
			meta[k] = fmt.Sprintf("%v", v)
		}
	}
	return meta
}
