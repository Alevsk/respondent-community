package grpctransport

import (
	"context"
	"fmt"
	"testing"
	"time"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/ai/analysis"
	appai "github.com/Alevsk/respondent/internal/app/ai"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/llm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Fake implementations for AI tests ───────────────────────────────

// Compile-time interface assertions.
var (
	_ llm.Provider                   = (*fakeAILLMProvider)(nil)
	_ appai.QueryExecutor            = (*fakeAIQueryExecutor)(nil)
	_ domain.AIInsightRepository     = (*fakeAIInsightRepo)(nil)
	_ appai.AnalysisDefinitionLoader = (*fakeAIAnalysisLoader)(nil)
)

// fakeAILLMProvider is a controllable LLM provider stub for AI server tests.
type fakeAILLMProvider struct {
	response *llm.CompletionResponse
	err      error
}

func (f *fakeAILLMProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return f.response, f.err
}
func (f *fakeAILLMProvider) Name() string                              { return "fake" }
func (f *fakeAILLMProvider) SupportsProvider(providerType string) bool { return providerType == "fake" }
func (f *fakeAILLMProvider) HealthCheck(_ context.Context) error       { return nil }

// fakeAIQueryExecutor is a controllable query executor stub.
type fakeAIQueryExecutor struct {
	rows []map[string]any
	err  error
}

func (f *fakeAIQueryExecutor) QueryRows(_ context.Context, _ string) ([]map[string]any, error) {
	return f.rows, f.err
}

func (f *fakeAIQueryExecutor) QueryRowsWithTimeout(_ context.Context, _, _ string) ([]map[string]any, error) {
	return f.rows, f.err
}

// fakeAIInsightRepo is a controllable insight repository stub.
type fakeAIInsightRepo struct {
	insights  []*domain.AIInsight
	count     int
	listErr   error
	createID  string
	createErr error
}

func (f *fakeAIInsightRepo) Create(_ context.Context, _ *domain.AIInsight) (string, error) {
	return f.createID, f.createErr
}
func (f *fakeAIInsightRepo) CreateRef(_ context.Context, _ string, _, _ *string) error {
	return nil
}
func (f *fakeAIInsightRepo) GetByID(_ context.Context, id string) (*domain.AIInsight, error) {
	for _, i := range f.insights {
		if i.ID == id {
			return i, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (f *fakeAIInsightRepo) List(_ context.Context, _ domain.InsightFilter) ([]*domain.AIInsight, int, error) {
	return f.insights, f.count, f.listErr
}
func (f *fakeAIInsightRepo) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}

func (f *fakeAIInsightRepo) GetRecentDedupKeys(_ context.Context, _, _ string, _ time.Time) (map[string]bool, error) {
	return nil, nil
}

// fakeAIAnalysisLoader is a controllable analysis definition loader stub.
type fakeAIAnalysisLoader struct {
	definitions []*analysis.AnalysisDefinition
}

func (f *fakeAIAnalysisLoader) ListDefinitions() []*analysis.AnalysisDefinition {
	return f.definitions
}

// newTestAIServer creates an AIServer wired with controllable fakes.
func newTestAIServer(opts ...func(*aiTestOpts)) *AIServer {
	o := &aiTestOpts{}
	for _, opt := range opts {
		opt(o)
	}

	eRepo := &testEntityRepo{
		entityByExtID:  o.entity,
		entityByExtErr: o.entityErr,
	}
	oRepo := &testObsRepo{
		observationsByEntity: o.observations,
		obsByEntityErr:       o.obsErr,
	}

	provider := &fakeAILLMProvider{response: o.llmResp, err: o.llmErr}

	var qe appai.QueryExecutor
	if o.queryRows != nil || o.queryErr != nil {
		qe = &fakeAIQueryExecutor{rows: o.queryRows, err: o.queryErr}
	}

	insightRepo := &fakeAIInsightRepo{
		insights:  o.insights,
		count:     o.insightCount,
		listErr:   o.insightListErr,
		createID:  o.insightCreateID,
		createErr: o.insightCreateErr,
	}

	var loader appai.AnalysisDefinitionLoader
	if o.analysisDefs != nil {
		loader = &fakeAIAnalysisLoader{definitions: o.analysisDefs}
	}

	svc := appai.NewAIService(eRepo, oRepo, insightRepo, provider, qe, nil, loader)
	return NewAIServer(svc)
}

type aiTestOpts struct {
	entity           *domain.Entity
	entityErr        error
	observations     []*domain.Observation
	obsErr           error
	llmResp          *llm.CompletionResponse
	llmErr           error
	queryRows        []map[string]any
	queryErr         error
	insights         []*domain.AIInsight
	insightCount     int
	insightListErr   error
	insightCreateID  string
	insightCreateErr error
	analysisDefs     []*analysis.AnalysisDefinition
}

// ── Tests: NewAIServer ──────────────────────────────────────────

func TestNewAIServer(t *testing.T) {
	svc := appai.NewAIService(
		&testEntityRepo{},
		&testObsRepo{},
		&fakeAIInsightRepo{},
		&fakeAILLMProvider{},
		nil,
		nil,
		nil,
	)
	srv := NewAIServer(svc)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

// ── Tests: NaturalLanguageSearch ────────────────────────────────────

func TestAIServer_NaturalLanguageSearch(t *testing.T) {
	tests := []struct {
		name        string
		req         *respondentv1.NaturalLanguageSearchRequest
		llmResp     *llm.CompletionResponse
		llmErr      error
		queryRows   []map[string]any
		queryErr    error
		wantCode    codes.Code
		wantResults int
	}{
		{
			name:     "empty query",
			req:      &respondentv1.NaturalLanguageSearchRequest{Query: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "successful search",
			req:  &respondentv1.NaturalLanguageSearchRequest{Query: "find flights", Limit: 10},
			llmResp: &llm.CompletionResponse{
				Content: "SELECT layer_type, external_id, name FROM entities LIMIT 10",
			},
			queryRows: []map[string]any{
				{"layer_type": "flights_commercial", "external_id": "UAL1234", "name": "UAL1234"},
			},
			wantCode:    codes.OK,
			wantResults: 1,
		},
		{
			name:     "LLM error",
			req:      &respondentv1.NaturalLanguageSearchRequest{Query: "find flights"},
			llmErr:   fmt.Errorf("provider timeout"),
			wantCode: codes.Internal,
		},
		{
			name: "query execution error",
			req:  &respondentv1.NaturalLanguageSearchRequest{Query: "find flights"},
			llmResp: &llm.CompletionResponse{
				Content: "SELECT layer_type, external_id, name FROM entities LIMIT 20",
			},
			queryErr: fmt.Errorf("db error"),
			wantCode: codes.Internal,
		},
		{
			name: "with layer filter",
			req:  &respondentv1.NaturalLanguageSearchRequest{Query: "find all", LayerType: "satellites", Limit: 5},
			llmResp: &llm.CompletionResponse{
				Content: "SELECT layer_type, external_id, name FROM entities WHERE layer_type = 'satellites' LIMIT 5",
			},
			queryRows:   []map[string]any{},
			wantCode:    codes.OK,
			wantResults: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestAIServer(func(o *aiTestOpts) {
				o.llmResp = tt.llmResp
				o.llmErr = tt.llmErr
				o.queryRows = tt.queryRows
				o.queryErr = tt.queryErr
			})

			resp, err := srv.NaturalLanguageSearch(context.Background(), tt.req)

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v: %s", tt.wantCode, st.Code(), st.Message())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
			if len(resp.Results) != tt.wantResults {
				t.Errorf("expected %d results, got %d", tt.wantResults, len(resp.Results))
			}
			if resp.GeneratedSql == "" {
				t.Error("expected non-empty generated_sql")
			}
		})
	}
}

// ── Tests: AnalyzeEntity ────────────────────────────────────────────

func TestAIServer_AnalyzeEntity(t *testing.T) {
	now := time.Now()

	entity := &domain.Entity{
		ID:         "uuid-1",
		ExternalID: "UAL1234",
		LayerType:  "flights_commercial",
		Name:       "UAL1234",
		Metadata:   map[string]string{"icao24": "abc123"},
	}

	obs := []*domain.Observation{
		{
			ID:        "obs-1",
			EntityID:  "uuid-1",
			Timestamp: now,
			Position:  &domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			AltitudeM: 10000,
		},
	}

	tests := []struct {
		name            string
		req             *respondentv1.AnalyzeEntityRequest
		entity          *domain.Entity
		entityErr       error
		observations    []*domain.Observation
		llmResp         *llm.CompletionResponse
		llmErr          error
		insightCreateID string
		wantCode        codes.Code
	}{
		{
			name:     "empty entity_id",
			req:      &respondentv1.AnalyzeEntityRequest{EntityId: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name:            "successful analysis",
			req:             &respondentv1.AnalyzeEntityRequest{EntityId: "flights_commercial:UAL1234", ObservationLimit: 5},
			entity:          entity,
			observations:    obs,
			llmResp:         &llm.CompletionResponse{Content: "Flight analysis: cruising at 10000m"},
			insightCreateID: "insight-123",
			wantCode:        codes.OK,
		},
		{
			name:      "entity not found",
			req:       &respondentv1.AnalyzeEntityRequest{EntityId: "flights_commercial:NONEXISTENT"},
			entityErr: fmt.Errorf("entity not found"),
			llmResp:   &llm.CompletionResponse{Content: "analysis"},
			wantCode:  codes.Internal,
		},
		{
			name:         "LLM failure",
			req:          &respondentv1.AnalyzeEntityRequest{EntityId: "flights_commercial:UAL1234"},
			entity:       entity,
			observations: obs,
			llmErr:       fmt.Errorf("model overloaded"),
			wantCode:     codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestAIServer(func(o *aiTestOpts) {
				o.entity = tt.entity
				o.entityErr = tt.entityErr
				o.observations = tt.observations
				o.llmResp = tt.llmResp
				o.llmErr = tt.llmErr
				o.insightCreateID = tt.insightCreateID
			})

			resp, err := srv.AnalyzeEntity(context.Background(), tt.req)

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v: %s", tt.wantCode, st.Code(), st.Message())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
			if resp.EntityId != tt.req.EntityId {
				t.Errorf("EntityId = %q, want %q", resp.EntityId, tt.req.EntityId)
			}
			if resp.Analysis == "" {
				t.Error("expected non-empty analysis")
			}
			if tt.insightCreateID != "" && resp.InsightId != tt.insightCreateID {
				t.Errorf("InsightId = %q, want %q", resp.InsightId, tt.insightCreateID)
			}
		})
	}
}

// ── Tests: GetInsights ──────────────────────────────────────────────

func TestAIServer_GetInsights(t *testing.T) {
	now := time.Now()
	lt := "flights_commercial"

	sampleInsights := []*domain.AIInsight{
		{
			ID:            "insight-1",
			InsightType:   "entity_analysis",
			SourceName:    "ai_service",
			OperationName: "analyze_entity",
			LayerType:     &lt,
			Result:        map[string]any{"analysis": "test"},
			EntityIDs:     []string{"uuid-1"},
			CreatedAt:     now,
		},
	}

	tests := []struct {
		name         string
		req          *respondentv1.GetInsightsRequest
		insights     []*domain.AIInsight
		insightCount int
		listErr      error
		wantCode     codes.Code
		wantCount    int
	}{
		{
			name:         "successful list",
			req:          &respondentv1.GetInsightsRequest{Limit: 20},
			insights:     sampleInsights,
			insightCount: 1,
			wantCode:     codes.OK,
			wantCount:    1,
		},
		{
			name:         "with filters",
			req:          &respondentv1.GetInsightsRequest{InsightType: "entity_analysis", LayerType: "flights_commercial", Limit: 10},
			insights:     sampleInsights,
			insightCount: 1,
			wantCode:     codes.OK,
			wantCount:    1,
		},
		{
			name:     "repository error",
			req:      &respondentv1.GetInsightsRequest{},
			listErr:  fmt.Errorf("db error"),
			wantCode: codes.Internal,
		},
		{
			name:         "empty results",
			req:          &respondentv1.GetInsightsRequest{InsightType: "nonexistent"},
			insights:     nil,
			insightCount: 0,
			wantCode:     codes.OK,
			wantCount:    0,
		},
		{
			name: "with attention filter",
			req:  &respondentv1.GetInsightsRequest{Attention: respondentv1.AttentionLevel_ATTENTION_LEVEL_HIGH, Limit: 10},
			insights: []*domain.AIInsight{
				{
					ID:          "insight-attn",
					InsightType: "alert",
					Attention:   strPtr("high"),
					CreatedAt:   now,
				},
			},
			insightCount: 1,
			wantCode:     codes.OK,
			wantCount:    1,
		},
		{
			name: "with min_attention filter",
			req:  &respondentv1.GetInsightsRequest{MinAttention: respondentv1.AttentionLevel_ATTENTION_LEVEL_HIGH, Limit: 10},
			insights: []*domain.AIInsight{
				{
					ID:          "insight-high",
					InsightType: "alert",
					Attention:   strPtr("high"),
					CreatedAt:   now,
				},
				{
					ID:          "insight-crit",
					InsightType: "alert",
					Attention:   strPtr("critical"),
					CreatedAt:   now,
				},
			},
			insightCount: 2,
			wantCode:     codes.OK,
			wantCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestAIServer(func(o *aiTestOpts) {
				o.insights = tt.insights
				o.insightCount = tt.insightCount
				o.insightListErr = tt.listErr
			})

			resp, err := srv.GetInsights(context.Background(), tt.req)

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v: %s", tt.wantCode, st.Code(), st.Message())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
			if len(resp.Insights) != tt.wantCount {
				t.Errorf("expected %d insights, got %d", tt.wantCount, len(resp.Insights))
			}
			if int(resp.TotalCount) != tt.insightCount {
				t.Errorf("expected total_count %d, got %d", tt.insightCount, resp.TotalCount)
			}
		})
	}
}

func TestAIServer_GetInsights_ProtoConversion(t *testing.T) {
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	lt := "satellites"

	insight := &domain.AIInsight{
		ID:             "insight-1",
		InsightType:    "anomaly_detection",
		SourceName:     "analysis_engine",
		OperationName:  "detect_anomalies",
		LayerType:      &lt,
		Result:         map[string]any{"count": float64(5)},
		EntityIDs:      []string{"uuid-1", "uuid-2"},
		ObservationIDs: []string{"obs-1"},
		ExpiresAt:      &expires,
		CreatedAt:      now,
	}

	srv := newTestAIServer(func(o *aiTestOpts) {
		o.insights = []*domain.AIInsight{insight}
		o.insightCount = 1
	})

	resp, err := srv.GetInsights(context.Background(), &respondentv1.GetInsightsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(resp.Insights))
	}

	pi := resp.Insights[0]
	if pi.Id != "insight-1" {
		t.Errorf("Id = %q, want %q", pi.Id, "insight-1")
	}
	if pi.InsightType != "anomaly_detection" {
		t.Errorf("InsightType = %q, want %q", pi.InsightType, "anomaly_detection")
	}
	if pi.LayerType != "satellites" {
		t.Errorf("LayerType = %q, want %q", pi.LayerType, "satellites")
	}
	if len(pi.EntityIds) != 2 {
		t.Errorf("expected 2 entity_ids, got %d", len(pi.EntityIds))
	}
	if len(pi.ObservationIds) != 1 {
		t.Errorf("expected 1 observation_id, got %d", len(pi.ObservationIds))
	}
	if pi.Result == nil {
		t.Error("expected non-nil result struct")
	}
	if pi.ExpiresAt == nil {
		t.Error("expected non-nil expires_at")
	}
	if pi.CreatedAt == nil {
		t.Error("expected non-nil created_at")
	}
}

// ── Tests: ExplainQuery ─────────────────────────────────────────────

func TestAIServer_ExplainQuery(t *testing.T) {
	tests := []struct {
		name     string
		req      *respondentv1.ExplainQueryRequest
		llmResp  *llm.CompletionResponse
		llmErr   error
		wantCode codes.Code
	}{
		{
			name:     "empty SQL",
			req:      &respondentv1.ExplainQueryRequest{Sql: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "successful explanation",
			req:  &respondentv1.ExplainQueryRequest{Sql: "SELECT * FROM entities"},
			llmResp: &llm.CompletionResponse{
				Content: "This query selects all entities.",
			},
			wantCode: codes.OK,
		},
		{
			name:     "LLM failure",
			req:      &respondentv1.ExplainQueryRequest{Sql: "SELECT 1"},
			llmErr:   fmt.Errorf("rate limited"),
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestAIServer(func(o *aiTestOpts) {
				o.llmResp = tt.llmResp
				o.llmErr = tt.llmErr
			})

			resp, err := srv.ExplainQuery(context.Background(), tt.req)

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v: %s", tt.wantCode, st.Code(), st.Message())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
			if resp.Explanation == "" {
				t.Error("expected non-empty explanation")
			}
		})
	}
}

// ── Tests: ListAnalysisDefinitions ──────────────────────────────────

func TestAIServer_ListAnalysisDefinitions(t *testing.T) {
	t.Run("no loader returns empty list", func(t *testing.T) {
		srv := newTestAIServer()

		resp, err := srv.ListAnalysisDefinitions(context.Background(), &respondentv1.ListAnalysisDefinitionsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatal("expected response, got nil")
		}
		if len(resp.Definitions) != 0 {
			t.Errorf("expected 0 definitions, got %d", len(resp.Definitions))
		}
	})

	t.Run("returns loaded definitions", func(t *testing.T) {
		srv := newTestAIServer(func(o *aiTestOpts) {
			o.analysisDefs = []*analysis.AnalysisDefinition{
				{
					Name:        "flight_patterns",
					DisplayName: "Flight Pattern Analysis",
					Enabled:     true,
					Schedule:    analysis.ScheduleConfig{Interval: "5m"},
					Data:        analysis.DataConfig{Layers: []string{"flights_commercial"}},
				},
			}
		})

		resp, err := srv.ListAnalysisDefinitions(context.Background(), &respondentv1.ListAnalysisDefinitionsRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Definitions) != 1 {
			t.Fatalf("expected 1 definition, got %d", len(resp.Definitions))
		}

		def := resp.Definitions[0]
		if def.Name != "flight_patterns" {
			t.Errorf("Name = %q, want %q", def.Name, "flight_patterns")
		}
		if def.DisplayName != "Flight Pattern Analysis" {
			t.Errorf("DisplayName = %q, want %q", def.DisplayName, "Flight Pattern Analysis")
		}
		if !def.Enabled {
			t.Error("expected Enabled=true")
		}
		if def.Schedule != "5m" {
			t.Errorf("Schedule = %q, want %q", def.Schedule, "5m")
		}
		if len(def.Layers) != 1 || def.Layers[0] != "flights_commercial" {
			t.Errorf("Layers = %v, want [flights_commercial]", def.Layers)
		}
	})
}

// ── Tests: domainInsightToProto ─────────────────────────────────────

func TestDomainInsightToProto(t *testing.T) {
	t.Run("nil insight returns nil", func(t *testing.T) {
		got := domainInsightToProto(nil)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("full insight converts correctly", func(t *testing.T) {
		now := time.Now()
		expires := now.Add(time.Hour)
		lt := "flights_commercial"
		rank := 4

		insight := &domain.AIInsight{
			ID:             "insight-1",
			InsightType:    "entity_analysis",
			SourceName:     "ai_service",
			OperationName:  "analyze_entity",
			LayerType:      &lt,
			Attention:      strPtr("critical"),
			AttentionRank:  &rank,
			Result:         map[string]any{"analysis": "test result"},
			EntityIDs:      []string{"uuid-1"},
			ObservationIDs: []string{"obs-1", "obs-2"},
			ExpiresAt:      &expires,
			CreatedAt:      now,
		}

		proto := domainInsightToProto(insight)
		if proto == nil {
			t.Fatal("expected non-nil proto")
		}
		if proto.Id != "insight-1" {
			t.Errorf("Id = %q, want %q", proto.Id, "insight-1")
		}
		if proto.InsightType != "entity_analysis" {
			t.Errorf("InsightType = %q, want %q", proto.InsightType, "entity_analysis")
		}
		if proto.LayerType != "flights_commercial" {
			t.Errorf("LayerType = %q, want %q", proto.LayerType, "flights_commercial")
		}
		if proto.Attention != respondentv1.AttentionLevel_ATTENTION_LEVEL_CRITICAL {
			t.Errorf("Attention = %v, want ATTENTION_LEVEL_CRITICAL", proto.Attention)
		}
		if proto.AttentionRank != 4 {
			t.Errorf("AttentionRank = %d, want %d", proto.AttentionRank, 4)
		}
		if proto.Result == nil {
			t.Error("expected non-nil result")
		}
		if len(proto.EntityIds) != 1 {
			t.Errorf("expected 1 entity_id, got %d", len(proto.EntityIds))
		}
		if len(proto.ObservationIds) != 2 {
			t.Errorf("expected 2 observation_ids, got %d", len(proto.ObservationIds))
		}
		if proto.ExpiresAt == nil {
			t.Error("expected non-nil expires_at")
		}
		if proto.CreatedAt == nil {
			t.Error("expected non-nil created_at")
		}
	})

	t.Run("nil attention and rank default to zero values in proto", func(t *testing.T) {
		now := time.Now()

		insight := &domain.AIInsight{
			ID:            "insight-nil-attn",
			InsightType:   "test",
			SourceName:    "test",
			OperationName: "test",
			Attention:     nil,
			AttentionRank: nil,
			CreatedAt:     now,
		}

		p := domainInsightToProto(insight)
		if p.Attention != respondentv1.AttentionLevel_ATTENTION_LEVEL_UNSPECIFIED {
			t.Errorf("Attention = %v, want ATTENTION_LEVEL_UNSPECIFIED for nil", p.Attention)
		}
		if p.AttentionRank != 0 {
			t.Errorf("AttentionRank = %d, want 0 for nil", p.AttentionRank)
		}
	})

	t.Run("insight without optional fields", func(t *testing.T) {
		now := time.Now()

		insight := &domain.AIInsight{
			ID:            "insight-2",
			InsightType:   "test",
			SourceName:    "test",
			OperationName: "test",
			CreatedAt:     now,
		}

		proto := domainInsightToProto(insight)
		if proto == nil {
			t.Fatal("expected non-nil proto")
		}
		if proto.LayerType != "" {
			t.Errorf("expected empty LayerType, got %q", proto.LayerType)
		}
		if proto.ExpiresAt != nil {
			t.Error("expected nil expires_at for insight without expiry")
		}
		if proto.Result != nil {
			t.Error("expected nil result for insight without result map")
		}
	})
}

func strPtr(s string) *string { return &s }
