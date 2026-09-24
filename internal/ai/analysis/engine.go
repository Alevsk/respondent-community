package analysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/ai"
	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/llm"
	sqlsafety "github.com/Alevsk/respondent/internal/sqlsafety"
)

// QueryExecutor abstracts read-only SQL execution for analysis data selection.
// This avoids coupling to pgxpool directly and enables testing.
type QueryExecutor interface {
	// QueryRows executes a read-only SQL query and returns rows as generic maps.
	QueryRows(ctx context.Context, sql string) ([]map[string]any, error)
}

// defaultMaxRecordsPerLayer caps the number of records fetched per layer to
// prevent unbounded queries. Analysis definitions can set their own limits
// via min_records (which acts as a minimum threshold), but this ensures we
// never send an unreasonable amount of data to the LLM.
const defaultMaxRecordsPerLayer = 500

// Engine loads analysis.d/*.yaml definitions and runs them on schedule.
type Engine struct {
	definitions map[string]*AnalysisDefinition
	cronRunner  *cron.Cron
	tickers     []*time.Ticker
	llmRegistry llm.Registry
	entityRepo  domain.EntityRepository
	obsRepo     domain.ObservationRepository
	layers      LayerRegistry
	insightRepo domain.AIInsightRepository
	schemas     *schema.Registry
	logger      zerolog.Logger
	notifier    notify.Notifier // optional: pushes ai_insight WS messages when set
	queryExec   QueryExecutor   // optional: executes SQL for data.sql analysis definitions

	// defaultMinAttention is the engine-wide floor applied when an operation does
	// not set its own output.min_attention. Empty means no default gate.
	defaultMinAttention string

	cancelFn context.CancelFunc
	wg       sync.WaitGroup

	// running tracks analysis names with an active run so an overlapping tick for
	// the same definition is skipped instead of double-writing insights. It is the
	// uniform in-flight guard across BOTH scheduleCron and scheduleInterval.
	running sync.Map // map[string]struct{}

	// celEnv is the shared CEL environment for evaluating data filters at runtime.
	celEnv          *cel.Env
	compiledFilters sync.Map // map[string]cel.Program

	// Clock allows test injection.
	clock Clock
}

// Clock provides time for testability.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// EngineConfig holds the dependencies for constructing an Engine.
type EngineConfig struct {
	LLMRegistry llm.Registry
	EntityRepo  domain.EntityRepository
	ObsRepo     domain.ObservationRepository
	// Layers decides which layer types exist. Required: an analysis that names
	// no layers means "all layers", and without the declarations that resolves
	// to every layer_type the database has ever held.
	Layers      LayerRegistry
	InsightRepo domain.AIInsightRepository
	Schemas     *schema.Registry
	Logger      zerolog.Logger
	Clock       Clock
	Notifier    notify.Notifier // optional: when set, ai_insight WS messages are pushed after storing insights
	QueryExec   QueryExecutor   // optional: when set, enables SQL-based data selection in analysis definitions
	// DefaultMinAttention is the engine-wide minimum attention floor for storing
	// and notifying insights, applied when an operation sets no output.min_attention.
	// One of info/low/medium/high/critical, or empty for no default gate.
	DefaultMinAttention string
}

// NewEngine creates a new analysis engine with the given dependencies.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.Layers == nil {
		return nil, fmt.Errorf("analysis engine: Layers registry is required")
	}
	celEnv, err := newEngineCELEnv()
	if err != nil {
		return nil, fmt.Errorf("create engine CEL env: %w", err)
	}

	clk := cfg.Clock
	if clk == nil {
		clk = realClock{}
	}

	return &Engine{
		definitions:         make(map[string]*AnalysisDefinition),
		llmRegistry:         cfg.LLMRegistry,
		entityRepo:          cfg.EntityRepo,
		obsRepo:             cfg.ObsRepo,
		layers:              cfg.Layers,
		insightRepo:         cfg.InsightRepo,
		schemas:             cfg.Schemas,
		logger:              cfg.Logger,
		notifier:            cfg.Notifier,
		queryExec:           cfg.QueryExec,
		defaultMinAttention: cfg.DefaultMinAttention,
		celEnv:              celEnv,
		clock:               clk,
	}, nil
}

// newEngineCELEnv creates the CEL environment used for runtime filter evaluation.
func newEngineCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("entity", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("observation", cel.MapType(cel.StringType, cel.DynType)),
		ext.Strings(),
	)
}

// SetDefinitions sets the loaded definitions on the engine.
// Typically called after Loader.LoadDefinitions().
func (e *Engine) SetDefinitions(defs map[string]*AnalysisDefinition) {
	e.definitions = defs
}

// Definitions returns the currently loaded definitions (for testing/inspection).
func (e *Engine) Definitions() map[string]*AnalysisDefinition {
	return e.definitions
}

// ListDefinitions returns all loaded definitions as a slice.
// This implements the bff.AnalysisDefinitionLoader interface so the engine
// can be passed directly to the AI BFF service for the ListAnalysisDefinitions RPC.
func (e *Engine) ListDefinitions() []*AnalysisDefinition {
	defs := make([]*AnalysisDefinition, 0, len(e.definitions))
	for _, def := range e.definitions {
		defs = append(defs, def)
	}
	return defs
}

// Start begins running all enabled analysis definitions on their configured schedules.
// It blocks until the context is cancelled or Stop() is called.
func (e *Engine) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	e.cancelFn = cancel

	// SkipIfStillRunning is defense-in-depth at the cron layer; the sync.Map
	// guard below is the uniform mechanism that also covers interval scheduling
	// (which cron.SkipIfStillRunning does not).
	e.cronRunner = cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)))

	enabledCount := 0
	for name, def := range e.definitions {
		if !def.Enabled {
			e.logger.Debug().Str("name", name).Msg("analysis definition disabled, skipping")
			continue
		}

		if !def.AI.Enabled {
			e.logger.Debug().Str("name", name).Msg("analysis AI section disabled, skipping")
			continue
		}

		if err := e.scheduleDefinition(ctx, def); err != nil {
			cancel()
			return fmt.Errorf("schedule %q: %w", name, err)
		}
		enabledCount++
	}

	e.cronRunner.Start()

	e.logger.Info().
		Int("total", len(e.definitions)).
		Int("enabled", enabledCount).
		Msg("analysis engine started")

	// Start expired insight cleanup goroutine.
	e.wg.Add(1)
	go e.cleanupExpiredInsights(ctx)

	<-ctx.Done()
	return nil
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	if e.cronRunner != nil {
		e.cronRunner.Stop()
	}
	for _, t := range e.tickers {
		t.Stop()
	}
	if e.cancelFn != nil {
		e.cancelFn()
	}
	e.wg.Wait()
	e.logger.Info().Msg("analysis engine stopped")
}

// scheduleDefinition sets up the scheduling goroutine for a single definition.
func (e *Engine) scheduleDefinition(ctx context.Context, def *AnalysisDefinition) error {
	if def.Schedule.Cron != "" {
		return e.scheduleCron(ctx, def)
	}
	return e.scheduleInterval(ctx, def)
}

// tryAcquireRun returns false if an analysis with this name is already running.
// It is the in-flight guard used by both the cron and interval schedulers to
// prevent overlapping runs of the same definition from double-writing insights.
func (e *Engine) tryAcquireRun(name string) bool {
	_, loaded := e.running.LoadOrStore(name, struct{}{})
	return !loaded
}

// releaseRun clears the in-flight marker for an analysis name, allowing the next
// tick to run. Always invoked via defer once a run is acquired.
func (e *Engine) releaseRun(name string) { e.running.Delete(name) }

// scheduleCron uses robfig/cron for cron-based scheduling.
func (e *Engine) scheduleCron(ctx context.Context, def *AnalysisDefinition) error {
	// robfig/cron/v3 with WithSeconds() expects 6-field cron expressions.
	// Standard 5-field expressions need a leading "0 " for the seconds field.
	cronExpr := def.Schedule.Cron

	defCopy := def // capture for closure
	_, err := e.cronRunner.AddFunc(cronExpr, func() {
		if ctx.Err() != nil {
			return
		}
		if !e.tryAcquireRun(defCopy.Name) {
			e.logger.Warn().Str("name", defCopy.Name).Msg("previous analysis run still active, skipping tick")
			return
		}
		defer e.releaseRun(defCopy.Name)
		e.wg.Add(1)
		defer e.wg.Done()
		if err := e.RunAnalysis(ctx, defCopy); err != nil {
			e.logger.Error().Err(err).Str("name", defCopy.Name).Msg("analysis run failed")
		}
	})
	if err != nil {
		return fmt.Errorf("add cron job %q: %w", cronExpr, err)
	}

	e.logger.Info().
		Str("name", def.Name).
		Str("cron", cronExpr).
		Msg("scheduled analysis (cron)")
	return nil
}

// scheduleInterval uses time.Ticker for interval-based scheduling.
func (e *Engine) scheduleInterval(ctx context.Context, def *AnalysisDefinition) error {
	interval, err := time.ParseDuration(def.Schedule.Interval)
	if err != nil {
		return fmt.Errorf("parse interval %q: %w", def.Schedule.Interval, err)
	}

	ticker := time.NewTicker(interval)
	e.tickers = append(e.tickers, ticker)

	defCopy := def // capture for closure
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					if !e.tryAcquireRun(defCopy.Name) {
						e.logger.Warn().Str("name", defCopy.Name).Msg("previous analysis run still active, skipping tick")
						return
					}
					defer e.releaseRun(defCopy.Name)
					e.wg.Add(1)
					defer e.wg.Done()
					if err := e.RunAnalysis(ctx, defCopy); err != nil {
						e.logger.Error().Err(err).Str("name", defCopy.Name).Msg("analysis run failed")
					}
				}()
			}
		}
	}()

	e.logger.Info().
		Str("name", def.Name).
		Str("interval", def.Schedule.Interval).
		Msg("scheduled analysis (interval)")
	return nil
}

// RunAnalysis executes a single analysis run for the given definition.
// This is the core analysis pipeline: fetch data -> render prompt -> call LLM -> store insights.
func (e *Engine) RunAnalysis(ctx context.Context, def *AnalysisDefinition) error {
	log := e.logger.With().Str("analysis", def.Name).Logger()
	startTime := e.clock.Now()

	log.Info().Msg("starting analysis run")

	// 1. Fetch data based on data config.
	records, err := e.fetchData(ctx, def, log)
	if err != nil {
		return fmt.Errorf("fetch data: %w", err)
	}

	// 2. Check min_records threshold.
	if def.Data.MinRecords > 0 && len(records) < def.Data.MinRecords {
		log.Info().
			Int("records", len(records)).
			Int("min_records", def.Data.MinRecords).
			Msg("insufficient records, skipping analysis")
		return nil
	}

	// 3. Get LLM provider.
	provider, err := e.llmRegistry.GetPreferredProvider(ctx)
	if err != nil {
		return fmt.Errorf("get LLM provider: %w", err)
	}

	// 4. Run each operation with per-operation dedup.
	for i := range def.AI.Operations {
		op := &def.AI.Operations[i]

		// Apply dedup filter.
		opRecords, dedupKeys := e.dedupRecords(ctx, records, def, op.Name, log)
		if len(opRecords) == 0 && def.Data.Dedup != nil {
			log.Info().Str("operation", op.Name).Msg("all records deduped, skipping operation")
			continue
		}

		// Build prompt data from (possibly filtered) records.
		lookbackDur, _ := time.ParseDuration(def.Data.Lookback)
		promptData := e.buildPromptData(opRecords, def, lookbackDur)

		if err := e.runOperation(ctx, def, op, promptData, provider, log, opRecords, dedupKeys); err != nil {
			log.Error().Err(err).Str("operation", op.Name).Msg("operation failed")
			// Continue with other operations; don't abort the entire run.
		}
	}

	log.Info().
		Dur("elapsed", e.clock.Now().Sub(startTime)).
		Int("records", len(records)).
		Msg("analysis run completed")

	return nil
}

// fetchData retrieves entities and observations based on the data config.
func (e *Engine) fetchData(ctx context.Context, def *AnalysisDefinition, log zerolog.Logger) ([]AnalysisRecord, error) {
	lookbackDur, _ := time.ParseDuration(def.Data.Lookback) // already validated
	cutoff := e.clock.Now().Add(-lookbackDur)

	// If SQL is set and a query executor is available, validate and execute
	// the SQL directly instead of the layer-based query path.
	if def.Data.SQL != "" {
		// A definition that declares data.sql must run that SQL. Silently falling
		// back to a layer query would return DIFFERENT (wrong) data while looking
		// healthy, masking the misconfiguration — so fail loudly instead.
		if e.queryExec == nil {
			return nil, fmt.Errorf("analysis %q sets data.sql but no query executor is configured", def.Name)
		}
		records, sqlErr := e.fetchDataFromSQL(ctx, def, cutoff, log)
		if sqlErr != nil {
			return nil, fmt.Errorf("SQL data fetch: %w", sqlErr)
		}
		return records, nil
	}

	// Determine which layers to query.
	layers := def.Data.Layers
	if len(layers) == 0 {
		// "All layers" means every layer a source still declares that holds
		// data — not every layer_type the database has ever held.
		distinctLayers, err := e.entityRepo.GetDistinctLayerTypes(ctx)
		if err != nil {
			return nil, fmt.Errorf("get distinct layers: %w", err)
		}
		layers = declaredLayersOnly(e.layers, distinctLayers)
	}

	var records []AnalysisRecord

	for _, layerType := range layers {
		// One page carries each entity alongside its latest observation, so the
		// per-observation entity lookup this loop used to make is gone.
		snapshots, err := e.obsRepo.GetLatestForLayerPage(ctx, layerType, defaultMaxRecordsPerLayer, 0)
		if err != nil {
			log.Warn().Err(err).Str("layer", layerType).Msg("failed to fetch observations, skipping layer")
			continue
		}

		// Filter by lookback window.
		for _, snapshot := range snapshots {
			entity, obs := &snapshot.Entity, &snapshot.Observation
			if obs.Timestamp.Before(cutoff) {
				continue
			}

			// Build merged metadata: entity metadata as base, observation metadata overlaid.
			// This ensures prompt templates can access fields from either source via .Metadata.
			merged := make(map[string]string, len(entity.Metadata)+len(obs.Metadata))
			for k, v := range entity.Metadata {
				merged[k] = v
			}
			for k, v := range obs.Metadata {
				merged[k] = v
			}

			record := AnalysisRecord{
				EntityID:       entity.ID,
				EntityName:     entity.Name,
				ExternalID:     entity.ExternalID,
				LayerType:      entity.LayerType,
				Altitude:       obs.AltitudeM,
				Metadata:       merged,
				EntityMetadata: entity.Metadata,
				ObsMetadata:    obs.Metadata,
				Timestamp:      obs.Timestamp.Format(time.RFC3339),
			}
			if obs.Position != nil {
				record.Lat = obs.Position.Lat
				record.Lon = obs.Position.Lon
			}

			records = append(records, record)
		}
	}

	// Apply CEL filter if present.
	if def.Data.Filter != "" && len(records) > 0 {
		filtered, err := e.applyFilter(def.Data.Filter, records)
		if err != nil {
			log.Warn().Err(err).Msg("CEL filter evaluation failed, using unfiltered records")
		} else {
			records = filtered
		}
	}

	// Cap records to max_records if set, otherwise use the default.
	cap := defaultMaxRecordsPerLayer
	if def.Data.MaxRecords > 0 {
		cap = def.Data.MaxRecords
	}
	if len(records) > cap {
		records = records[:cap]
	}

	return records, nil
}

// fetchDataFromSQL validates and executes a SQL query from the definition's data.sql field,
// converts the result rows into AnalysisRecords, and applies CEL filtering if configured.
func (e *Engine) fetchDataFromSQL(ctx context.Context, def *AnalysisDefinition, cutoff time.Time, log zerolog.Logger) ([]AnalysisRecord, error) {
	// Validate SQL is read-only using the PostgreSQL AST parser.
	if err := sqlsafety.ValidateReadOnlySQL(def.Data.SQL); err != nil {
		return nil, fmt.Errorf("SQL validation failed: %w", err)
	}

	log.Debug().Str("sql", def.Data.SQL).Msg("executing SQL data source")

	rows, err := e.queryExec.QueryRows(ctx, def.Data.SQL)
	if err != nil {
		return nil, fmt.Errorf("execute SQL: %w", err)
	}

	records := make([]AnalysisRecord, 0, len(rows))
	for _, row := range rows {
		rec := rowToAnalysisRecord(row)

		// Filter by lookback window if the row has a timestamp.
		if rec.Timestamp != "" {
			ts, parseErr := time.Parse(time.RFC3339, rec.Timestamp)
			if parseErr == nil && ts.Before(cutoff) {
				continue
			}
		}

		records = append(records, rec)
	}

	// Apply CEL filter if present.
	if def.Data.Filter != "" && len(records) > 0 {
		filtered, filterErr := e.applyFilter(def.Data.Filter, records)
		if filterErr != nil {
			log.Warn().Err(filterErr).Msg("CEL filter evaluation failed on SQL results, using unfiltered records")
		} else {
			records = filtered
		}
	}

	log.Debug().Int("rows", len(records)).Msg("SQL data source returned records")
	return records, nil
}

// rowToAnalysisRecord converts a generic map row from SQL execution into an AnalysisRecord.
// Standard columns (entity_id, external_id, name, layer_type, lat, lon, altitude_m, ts)
// are mapped to their corresponding fields; all other columns go into Metadata.
func rowToAnalysisRecord(row map[string]any) AnalysisRecord {
	rec := AnalysisRecord{
		Metadata:       make(map[string]string),
		EntityMetadata: make(map[string]string),
		ObsMetadata:    make(map[string]string),
	}

	// Standard column mappings.
	standardKeys := map[string]bool{
		"entity_id": true, "external_id": true, "name": true,
		"layer_type": true, "lat": true, "lon": true,
		"altitude_m": true, "ts": true,
	}

	for k, v := range row {
		if v == nil {
			continue
		}
		switch k {
		case "entity_id":
			rec.EntityID = anyToString(v)
		case "external_id":
			rec.ExternalID = anyToString(v)
		case "name":
			rec.EntityName = anyToString(v)
		case "layer_type":
			rec.LayerType = anyToString(v)
		case "lat":
			rec.Lat = toFloat64(v)
		case "lon":
			rec.Lon = toFloat64(v)
		case "altitude_m":
			rec.Altitude = toFloat64(v)
		case "ts":
			rec.Timestamp = toRFC3339(v)
		default:
			if !standardKeys[k] {
				rec.Metadata[k] = anyToString(v)
			}
		}
	}

	// Mirror structural fields into Metadata so that dedup key_fields,
	// CEL filters, and prompt templates can reference ALL record fields
	// uniformly via Metadata without needing special-case fallback logic.
	// This keeps the system fully declarative: a YAML author can use
	// "entity_external_id" in key_fields and it resolves the same way as
	// any SQL-aliased column like "conflict_zone" — both are just Metadata
	// lookups. We use canonical names with the "entity_" prefix to avoid
	// colliding with SQL aliases (e.g. a query might SELECT ... AS name).
	rec.Metadata["entity_id"] = rec.EntityID
	rec.Metadata["entity_external_id"] = rec.ExternalID
	rec.Metadata["entity_name"] = rec.EntityName
	rec.Metadata["layer_type"] = rec.LayerType

	return rec
}

// anyToString converts an arbitrary value to a clean string representation.
// It handles every type that pgx.RowToMap may return, ensuring numeric values
// are never rendered in scientific notation (e.g. 1.66e+08 → "166217243").
func anyToString(v any) string {
	switch b := v.(type) {
	case string:
		return b
	case [16]byte:
		return uuid.UUID(b).String()
	case float64:
		if b == math.Trunc(b) && !math.IsInf(b, 0) && !math.IsNaN(b) {
			return strconv.FormatInt(int64(b), 10)
		}
		return strconv.FormatFloat(b, 'f', -1, 64)
	case float32:
		f := float64(b)
		if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case int:
		return strconv.Itoa(b)
	case int64:
		return strconv.FormatInt(b, 10)
	case int32:
		return strconv.FormatInt(int64(b), 10)
	case bool:
		return strconv.FormatBool(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toFloat64 attempts to convert a generic value to float64.
func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	default:
		// Attempt string parse as a fallback.
		var f float64
		if _, err := fmt.Sscanf(fmt.Sprintf("%v", v), "%f", &f); err == nil {
			return f
		}
		return 0
	}
}

// toRFC3339 converts a value to an RFC3339 timestamp string.
func toRFC3339(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.Format(time.RFC3339)
	case string:
		// Try to parse and re-format for consistency.
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed.Format(time.RFC3339)
		}
		// Try other common formats.
		if parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", t); err == nil {
			return parsed.Format(time.RFC3339)
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
		return t // return as-is if unparseable
	default:
		return fmt.Sprintf("%v", v)
	}
}

// computeDedupKey hashes the specified key fields from a record into a stable
// hex-encoded SHA-256 digest used for dedup lookups.
//
// All field values are resolved from rec.Metadata — including structural fields
// like entity_external_id, entity_id, entity_name, and layer_type which are
// mirrored into Metadata by rowToAnalysisRecord. This keeps the function purely
// declarative: whatever the YAML author writes in key_fields is a direct Metadata
// lookup with no hidden fallback logic.
func computeDedupKey(rec AnalysisRecord, keyFields []string) string {
	h := sha256.New()
	for _, field := range keyFields {
		h.Write([]byte(field))
		h.Write([]byte{0})
		h.Write([]byte(rec.Metadata[field]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// dedupRecords filters out records that already have recent insights within
// the configured dedup window. Returns the filtered records and a map of
// record-index -> dedup-key for the surviving records.
func (e *Engine) dedupRecords(
	ctx context.Context,
	records []AnalysisRecord,
	def *AnalysisDefinition,
	opName string,
	log zerolog.Logger,
) ([]AnalysisRecord, map[int]string) {
	cfg := def.Data.Dedup
	if cfg == nil {
		return records, nil
	}

	window, err := time.ParseDuration(cfg.Window)
	if err != nil {
		log.Warn().Err(err).Msg("invalid dedup window, skipping dedup")
		return records, nil
	}

	recordKeys := make(map[int]string, len(records))
	for i, rec := range records {
		recordKeys[i] = computeDedupKey(rec, cfg.KeyFields)
	}

	since := e.clock.Now().Add(-window)
	existingKeys, err := e.insightRepo.GetRecentDedupKeys(ctx, def.Name, opName, since)
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch dedup keys, skipping dedup")
		return records, recordKeys
	}

	var filtered []AnalysisRecord
	filteredKeys := make(map[int]string)
	deduped := 0
	for i, rec := range records {
		if existingKeys[recordKeys[i]] {
			deduped++
			continue
		}
		filteredKeys[len(filtered)] = recordKeys[i]
		filtered = append(filtered, rec)
	}

	log.Info().
		Int("total", len(records)).
		Int("deduped", deduped).
		Int("remaining", len(filtered)).
		Str("window", cfg.Window).
		Msg("dedup filtered records")

	return filtered, filteredKeys
}

// applyFilter evaluates a CEL expression against each record and returns those that pass.
func (e *Engine) applyFilter(filter string, records []AnalysisRecord) ([]AnalysisRecord, error) {
	prg, err := e.getOrCompileCELProgram(filter)
	if err != nil {
		return nil, fmt.Errorf("compile filter: %w", err)
	}

	var result []AnalysisRecord
	for _, rec := range records {
		// Build entity metadata map from the entity's own metadata.
		entityMetaMap := make(map[string]any, len(rec.EntityMetadata))
		for k, v := range rec.EntityMetadata {
			entityMetaMap[k] = v
		}
		entityMap := map[string]any{
			"id":          rec.EntityID,
			"external_id": rec.ExternalID,
			"name":        rec.EntityName,
			"layer_type":  rec.LayerType,
			"metadata":    entityMetaMap,
		}

		// Build observation metadata map from the observation's own metadata.
		obsMetaMap := make(map[string]any, len(rec.ObsMetadata))
		for k, v := range rec.ObsMetadata {
			obsMetaMap[k] = v
		}
		obsMap := map[string]any{
			"lat":      rec.Lat,
			"lon":      rec.Lon,
			"altitude": rec.Altitude,
			"metadata": obsMetaMap,
		}

		out, _, err := prg.Eval(map[string]any{
			"entity":      entityMap,
			"observation": obsMap,
		})
		if err != nil {
			continue // skip records that fail filter evaluation
		}

		b, ok := out.Value().(bool)
		if ok && b {
			result = append(result, rec)
		}
	}

	return result, nil
}

// getOrCompileCELProgram returns a cached CEL program, compiling on first access.
func (e *Engine) getOrCompileCELProgram(filter string) (cel.Program, error) {
	if cached, ok := e.compiledFilters.Load(filter); ok {
		return cached.(cel.Program), nil
	}

	ast, issues := e.celEnv.Compile(filter)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile: %w", issues.Err())
	}

	prg, err := e.celEnv.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(ai.CELFilterCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("program: %w", err)
	}

	e.compiledFilters.Store(filter, prg)
	return prg, nil
}

// buildPromptData assembles the AnalysisPromptData from fetched records.
func (e *Engine) buildPromptData(records []AnalysisRecord, _ *AnalysisDefinition, lookback time.Duration) AnalysisPromptData {
	// Compute per-layer statistics.
	layerMap := make(map[string][]AnalysisRecord)
	for _, rec := range records {
		layerMap[rec.LayerType] = append(layerMap[rec.LayerType], rec)
	}

	var layerStats []LayerStat
	for lt, recs := range layerMap {
		stat := computeLayerStat(lt, recs)
		layerStats = append(layerStats, stat)
	}

	return AnalysisPromptData{
		RecordCount: len(records),
		Lookback:    lookback.String(),
		Records:     records,
		LayerCount:  len(layerMap),
		LayerStats:  layerStats,
	}
}

// computeLayerStat computes statistics for a set of records in a single layer.
func computeLayerStat(layerType string, records []AnalysisRecord) LayerStat {
	stat := LayerStat{
		LayerType: layerType,
		Count:     len(records),
	}

	if len(records) == 0 {
		return stat
	}

	totalMetaKeys := 0
	hasCoord := 0
	hasAlt := 0
	extIDs := make(map[string]int)

	for _, rec := range records {
		totalMetaKeys += len(rec.Metadata)
		if rec.Lat != 0 || rec.Lon != 0 {
			hasCoord++
		}
		if rec.Altitude != 0 {
			hasAlt++
		}
		extIDs[rec.ExternalID]++
	}

	stat.AvgMetadataKeys = totalMetaKeys / len(records)
	stat.CoordPercent = (hasCoord * 100) / len(records)
	stat.AltPercent = (hasAlt * 100) / len(records)
	stat.UniqueExtIDs = len(extIDs)

	dupes := 0
	for _, count := range extIDs {
		if count > 1 {
			dupes++
		}
	}
	stat.DuplicateExtIDs = dupes

	// Build field coverage summary.
	fieldSet := make(map[string]struct{})
	for _, rec := range records {
		for k := range rec.Metadata {
			fieldSet[k] = struct{}{}
		}
	}
	fields := make([]string, 0, len(fieldSet))
	for k := range fieldSet {
		fields = append(fields, k)
	}
	if len(fields) > 10 {
		stat.FieldCoverage = fmt.Sprintf("%d fields", len(fields))
	} else {
		stat.FieldCoverage = fmt.Sprintf("%v", fields)
	}

	return stat
}

// runOperation executes a single AI operation within an analysis run.
func (e *Engine) runOperation(
	ctx context.Context,
	def *AnalysisDefinition,
	op *aiconfig.OperationConfig,
	promptData AnalysisPromptData,
	provider llm.Provider,
	log zerolog.Logger,
	records []AnalysisRecord,
	dedupKeys map[int]string,
) error {
	log = log.With().Str("operation", op.Name).Logger()

	// Inject output schema into prompt data.
	schemaKey := SchemaKey(def.Name, op.Name)
	promptData.OutputSchema = e.schemas.SchemaJSON(schemaKey)

	// Render prompt template.
	rendered, err := renderAnalysisPrompt(op.Prompt, promptData)
	if err != nil {
		return fmt.Errorf("render prompt: %w", err)
	}

	// Append base schema prompt suffix to instruct LLM to include attention field.
	if len(op.OutputSchema) > 0 {
		rendered += BaseSchemaPromptSuffix
	}

	// Call LLM.
	req := &llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: "user", Content: rendered},
		},
		MaxTokens:   op.MaxTokens,
		Temperature: op.Temperature,
	}

	// When output_schema is defined, enforce JSON mode at the API level and
	// prepend a system message as defense-in-depth.
	if len(op.OutputSchema) > 0 {
		req.ResponseFormat = llm.ResponseFormatJSON
		req.Messages = append([]llm.Message{
			{Role: "system", Content: "You MUST respond with a valid JSON object. Do not include markdown fences, commentary, or any text outside the JSON structure."},
		}, req.Messages...)
	}

	// Validate and extract response.
	var llmResult map[string]any
	if len(op.OutputSchema) > 0 {
		// The LLM occasionally returns JSON that doesn't match the schema —
		// wrapped in an extra object, missing keys, or (for OMIT-non-threats
		// analyses) an empty/partial object. A schema failure discards the whole
		// run, so retry a BOUNDED number of times, each appending a corrective
		// instruction that names the exact required top-level keys.
		const maxSchemaAttempts = 3
		for attempt := 1; ; attempt++ {
			resp, llmErr := llm.RetryableComplete(ctx, provider, req, llm.DefaultMaxRetries)
			if llmErr != nil {
				return fmt.Errorf("LLM call: %w", llmErr)
			}
			validated, valErr := e.schemas.ValidateAndExtract(schemaKey, resp.Content)
			if valErr == nil {
				llmResult = validated
				break
			}
			if attempt >= maxSchemaAttempts {
				return fmt.Errorf("schema validation after %d attempts: %w", attempt, valErr)
			}
			log.Warn().Err(valErr).Int("attempt", attempt).Msg("LLM output failed schema validation; retrying with corrective prompt")
			req.Messages = append(req.Messages, llm.Message{
				Role:    "system",
				Content: schema.CorrectionMessage(op.OutputSchema),
			})
		}
	} else {
		resp, llmErr := llm.RetryableComplete(ctx, provider, req, llm.DefaultMaxRetries)
		if llmErr != nil {
			return fmt.Errorf("LLM call: %w", llmErr)
		}
		extracted, extractErr := schema.ExtractJSON(resp.Content)
		if extractErr != nil {
			llmResult = map[string]any{"result": resp.Content}
		} else if unmarshalErr := json.Unmarshal([]byte(extracted), &llmResult); unmarshalErr != nil {
			llmResult = map[string]any{"result": resp.Content}
		}
	}

	// Store insights.
	if op.Output.StoreInsights {
		if err := e.storeInsights(ctx, def, op, llmResult, records, dedupKeys, log); err != nil {
			return fmt.Errorf("store insights: %w", err)
		}
	}

	log.Info().Msg("operation completed")
	return nil
}

// renderAnalysisPrompt renders a Go template with AnalysisPromptData.
func renderAnalysisPrompt(tmplStr string, data AnalysisPromptData) (string, error) {
	tmpl, err := template.New("analysis_prompt").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// storeInsights implements the insight storage protocol (Spec Section 6.5).
// attentionPasses reports whether an insight with the given attention clears the
// minimum-attention floor for this operation. The floor is the per-operation
// output.min_attention when set, otherwise the engine-wide default. An empty
// floor (neither configured) disables gating (backward compatible). Absent or
// unclassified attention is treated as the lowest level ("info"), so it is
// dropped whenever any floor is configured.
func (e *Engine) attentionPasses(op *aiconfig.OperationConfig, attention *string) bool {
	floor := op.Output.MinAttention
	if floor == "" {
		floor = e.defaultMinAttention
	}
	if floor == "" {
		return true
	}
	level := "info"
	if attention != nil {
		level = *attention
	}
	return domain.AttentionRank(level) >= domain.AttentionRank(floor)
}

// attentionOrUnclassified renders an attention pointer for logging.
func attentionOrUnclassified(attention *string) string {
	if attention == nil {
		return "unclassified"
	}
	return *attention
}

func (e *Engine) storeInsights(
	ctx context.Context,
	def *AnalysisDefinition,
	op *aiconfig.OperationConfig,
	result map[string]any,
	records []AnalysisRecord,
	dedupKeys map[int]string,
	log zerolog.Logger,
) error {
	// Compute expiration time from retention.
	var expiresAt *time.Time
	if op.Output.Retention != "" {
		d, err := time.ParseDuration(op.Output.Retention)
		if err == nil {
			t := e.clock.Now().Add(d)
			expiresAt = &t
		}
	}

	// Determine layer_type: set when there is exactly one layer, or derived
	// per-item from matched records in storeResultsArray for multi-layer analyses.
	var layerType *string
	if len(def.Data.Layers) == 1 {
		lt := def.Data.Layers[0]
		layerType = &lt
	} else if len(records) > 0 {
		// For multi-layer analyses, use the first record's layer as default.
		// storeResultsArray will override per-item from matched records.
		lt := records[0].LayerType
		if lt != "" {
			layerType = &lt
		}
	}

	// If results_path is set, iterate the array and create one insight per item.
	// storeResultsArray derives each item's dedup key from its matched record, so
	// it does not need the precomputed per-record dedupKeys map.
	if op.Output.ResultsPath != "" {
		return e.storeResultsArray(ctx, def, op, result, layerType, expiresAt, records, log)
	}

	// Otherwise, store the entire response as one insight.
	return e.storeSingleInsight(ctx, def, op, result, layerType, expiresAt, records, dedupKeys, log)
}

// storeResultsArray extracts an array at results_path and creates one insight per item.
func (e *Engine) storeResultsArray(
	ctx context.Context,
	def *AnalysisDefinition,
	op *aiconfig.OperationConfig,
	result map[string]any,
	layerType *string,
	expiresAt *time.Time,
	records []AnalysisRecord,
	log zerolog.Logger,
) error {
	arr, ok := result[op.Output.ResultsPath]
	if !ok {
		return fmt.Errorf("results_path %q not found in response", op.Output.ResultsPath)
	}

	items, ok := arr.([]any)
	if !ok {
		return fmt.Errorf("results_path %q is not an array", op.Output.ResultsPath)
	}

	// When the results array is empty, the analysis ran and found nothing to
	// report. With no attention floor we store an "all clear" summary so the run
	// is visible in ai_insights (legacy behavior). With a floor configured, an
	// all-clear is by definition below it (nil attention -> "info"), so we skip
	// the summary — the run is still visible in the logs, and ai_insights stays
	// free of per-tick "nothing found" rows that would otherwise accumulate.
	if len(items) == 0 {
		if !e.attentionPasses(op, nil) {
			log.Info().Msg("empty results array; all-clear summary gated by min_attention (not stored)")
			return nil
		}
		summary := &domain.AIInsight{
			ID:            uuid.New().String(),
			InsightType:   op.Output.InsightType,
			SourceName:    def.Name,
			OperationName: op.Name,
			LayerType:     layerType,
			Result:        result, // store the full LLM response (e.g. {"anomalies": []})
			ExpiresAt:     expiresAt,
			CreatedAt:     e.clock.Now(),
		}
		if _, err := e.insightRepo.Create(ctx, summary); err != nil {
			log.Error().Err(err).Msg("failed to store empty-result summary insight")
		}
		log.Info().Msg("stored summary insight (empty results array)")
		return nil
	}

	gated := 0
	for i, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			log.Warn().Int("index", i).Msg("results_path item is not an object, skipping")
			continue
		}

		// Match this result item to its specific input entity by external_id (and
		// entity_layer_type when present). Drives entity associations, per-item
		// layer_type, and the dedup key below.
		matchedRecords := matchRecordsByExternalID(itemMap, records)

		// Resolve the dedup key for this result item.
		//
		// Derive it from the input record this item identifies, so SQL-only key
		// components the LLM never echoes (e.g. a rounded spatiotemporal key used
		// to collapse re-issued warnings or cross-source duplicates) are included
		// and stay STABLE across runs; the item's own values overlay the record's
		// (handles per-item composite choices like a chosen conflict zone). The
		// pre-LLM filter keys the same records the same way, so a recently-stored
		// pair is correctly skipped on the next run. Positional item->record
		// matching is unreliable once the prompt omits or reorders items, so it
		// is never used.
		var dedupKey *string
		if def.Data.Dedup != nil {
			rec := dedupRecordForItem(itemMap, matchedRecords, def.Data.Dedup.KeyFields)
			k := computeDedupKey(rec, def.Data.Dedup.KeyFields)
			dedupKey = &k
		}

		// Derive per-item layer_type from the matched record when the
		// definition-level layerType is generic (multi-layer or first-record default).
		itemLayerType := layerType
		if len(matchedRecords) > 0 && matchedRecords[0].LayerType != "" {
			lt := matchedRecords[0].LayerType
			itemLayerType = &lt
		}

		attention := extractAttention(itemMap)
		// Attention gate: drop results below the configured floor (the analysis's
		// own "no threat / routine" conclusions) before they are stored or pushed.
		if !e.attentionPasses(op, attention) {
			gated++
			continue
		}

		insight := &domain.AIInsight{
			ID:            uuid.New().String(),
			InsightType:   op.Output.InsightType,
			SourceName:    def.Name,
			OperationName: op.Name,
			LayerType:     itemLayerType,
			Attention:     attention,
			DedupKey:      dedupKey,
			Result:        itemMap,
			ExpiresAt:     expiresAt,
			CreatedAt:     e.clock.Now(),
		}

		insightID, err := e.insightRepo.Create(ctx, insight)
		if err != nil {
			log.Error().Err(err).Int("index", i).Msg("failed to create insight")
			continue
		}

		e.createAssociations(ctx, insightID, matchedRecords, log)

		// Resolve additional cross-layer entity refs from ref_fields.
		refIDs, refEntities := e.resolveRefFieldEntities(ctx, insightID, itemMap, op.Output.RefFields, log)

		// Push WebSocket notification if configured and notifier is available.
		if op.Output.WebSocketPush && e.notifier != nil {
			insight.ID = insightID // use the persisted ID
			insight.EntityIDs, insight.Entities = collectEntityRefs(matchedRecords)
			// Append cross-layer refs so the WS payload includes all associated entities.
			insight.EntityIDs = append(insight.EntityIDs, refIDs...)
			insight.Entities = append(insight.Entities, refEntities...)
			if notifyErr := e.notifier.NotifyInsight(ctx, insight); notifyErr != nil {
				log.Warn().Err(notifyErr).Str("insight_id", insightID).Int("index", i).Msg("failed to push ai_insight WebSocket notification")
			}
		}
	}

	log.Info().Int("count", len(items)-gated).Int("gated", gated).Int("total", len(items)).Msg("stored insights from results_path array")
	return nil
}

// storeSingleInsight stores the entire LLM response as a single insight.
func (e *Engine) storeSingleInsight(
	ctx context.Context,
	def *AnalysisDefinition,
	op *aiconfig.OperationConfig,
	result map[string]any,
	layerType *string,
	expiresAt *time.Time,
	records []AnalysisRecord,
	dedupKeys map[int]string,
	log zerolog.Logger,
) error {
	// Compute a combined dedup key from all record keys.
	var dedupKey *string
	if len(dedupKeys) > 0 {
		keys := make([]string, 0, len(dedupKeys))
		for _, k := range dedupKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		h := sha256.New()
		for _, k := range keys {
			h.Write([]byte(k))
		}
		combined := hex.EncodeToString(h.Sum(nil))
		dedupKey = &combined
	}

	attention := extractAttention(result)
	// Attention gate: a single-insight result below the floor is dropped (the
	// analysis's own "no threat / routine" conclusion is not worth an alert).
	if !e.attentionPasses(op, attention) {
		log.Info().Str("attention", attentionOrUnclassified(attention)).Msg("single insight gated below min_attention; not stored")
		return nil
	}

	insight := &domain.AIInsight{
		ID:            uuid.New().String(),
		InsightType:   op.Output.InsightType,
		SourceName:    def.Name,
		OperationName: op.Name,
		LayerType:     layerType,
		Attention:     attention,
		DedupKey:      dedupKey,
		Result:        result,
		ExpiresAt:     expiresAt,
		CreatedAt:     e.clock.Now(),
	}

	insightID, err := e.insightRepo.Create(ctx, insight)
	if err != nil {
		return fmt.Errorf("create insight: %w", err)
	}

	e.createAssociations(ctx, insightID, records, log)

	// Push WebSocket notification if configured and notifier is available.
	if op.Output.WebSocketPush && e.notifier != nil {
		insight.ID = insightID // use the persisted ID
		insight.EntityIDs, insight.Entities = collectEntityRefs(records)
		if notifyErr := e.notifier.NotifyInsight(ctx, insight); notifyErr != nil {
			log.Warn().Err(notifyErr).Str("insight_id", insightID).Msg("failed to push ai_insight WebSocket notification")
		}
	}

	log.Info().Str("insight_id", insightID).Msg("stored single insight")
	return nil
}

// collectEntityRefs builds deduplicated EntityIDs and Entities slices from the
// input records. Used to populate the insight before WebSocket notification so
// the frontend receives rich entity refs without a separate REST fetch.
func collectEntityRefs(records []AnalysisRecord) ([]string, []domain.InsightEntityRef) {
	seen := make(map[string]bool, len(records))
	var ids []string
	var refs []domain.InsightEntityRef

	for _, rec := range records {
		if rec.EntityID == "" || seen[rec.EntityID] {
			continue
		}
		seen[rec.EntityID] = true
		ids = append(ids, rec.EntityID)
		refs = append(refs, domain.InsightEntityRef{
			ID:         rec.EntityID,
			ExternalID: rec.ExternalID,
			Name:       rec.EntityName,
			LayerType:  rec.LayerType,
		})
	}

	return ids, refs
}

// TODO: Remove normalizeNumericID and simplify matchRecordsByExternalID to use
// direct string comparison once the existing scientific-notation external_ids
// have been cleaned up in production. Run these two queries:
//
//	UPDATE entities
//	SET external_id = CAST(CAST(external_id AS DOUBLE PRECISION) AS BIGINT)::TEXT
//	WHERE external_id ~ '^-?[0-9]+(\.[0-9]+)?[eE]\+[0-9]+$'
//	  AND CAST(external_id AS DOUBLE PRECISION) = TRUNC(CAST(external_id AS DOUBLE PRECISION));
//
//	SELECT COUNT(*) FROM entities WHERE external_id ~ '[eE]\+';
//	-- should return 0
//
// normalizeNumericID normalizes numeric strings so different representations
// of the same number compare equal (e.g. "2.58187e+08" and "258187000").
// Returns the original string if it is not a valid number.
func normalizeNumericID(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// matchRecordsByExternalID returns the input record(s) that a per-item LLM result
// identifies via "entity_external_id" — and, when present, "entity_layer_type".
// A results_path item describes ONE entity, so its associations must be that
// entity, not the whole batch. The layer is part of the identity because the same
// external_id can exist in different layers (e.g. a volcano "211010" and an
// unrelated entity); when the item names its entity's layer we require it to match.
//
// When the item carries no usable identity, or names one that matches no input
// record, this returns nil — NOT every record. Associating an item with all
// records produced cross-geography "ghost" associations (an earthquake-near-Hirara
// insight linked to 15 unrelated volcanoes from other rows). Cross-layer refs
// still come from ref_fields; summary analyses that genuinely span all records
// use storeSingleInsight, not this path.
//
// Comparison uses normalizeNumericID for backward compatibility with existing DB
// records where external_id was stored in scientific notation (e.g. "2.58187e+08")
// due to a bug in the CEL eval %g formatting. The root cause is fixed in eval.go's
// formatFloat(), but already-ingested data retains the old format until re-ingested.
func matchRecordsByExternalID(itemMap map[string]any, records []AnalysisRecord) []AnalysisRecord {
	target, ok := stringField(itemMap, "entity_external_id")
	if !ok || target == "" {
		return nil
	}
	normalizedTarget := normalizeNumericID(target)
	// Optional layer scope: the same external_id can exist across layers, so when
	// the item names its entity's layer, require the record's layer to match too.
	layer, _ := stringField(itemMap, "entity_layer_type")

	var matched []AnalysisRecord
	for _, rec := range records {
		if normalizeNumericID(rec.ExternalID) != normalizedTarget {
			continue
		}
		if layer != "" && rec.LayerType != layer {
			continue
		}
		matched = append(matched, rec)
	}
	return matched
}

// stringField returns m[key] as a string when present and of string type.
func stringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// dedupRecordForItem builds the record whose key_fields produce a result item's
// dedup key. It starts from the input record the item identifies (so SQL-only
// key components the LLM never echoes — e.g. a rounded spatiotemporal key — are
// present and stable across runs), then overlays the item's own values so
// per-item composite choices win. This deliberately avoids positional
// item->record matching, which breaks once the prompt omits or reorders items.
func dedupRecordForItem(itemMap map[string]any, matched []AnalysisRecord, keyFields []string) AnalysisRecord {
	base := make(map[string]string)
	if rec, ok := pickRecordForItem(itemMap, matched); ok {
		for k, v := range rec.Metadata {
			base[k] = v
		}
	}
	for _, kf := range keyFields {
		if v, ok := itemMap[kf]; ok {
			base[kf] = anyToString(v)
		}
	}
	return AnalysisRecord{Metadata: base}
}

// pickRecordForItem chooses which matched input record a result item corresponds
// to. With one match it is unambiguous; with several (one hazard shared across
// many infrastructure entities, say) it picks the record whose fields agree most
// with the item — disambiguating on the secondary identifiers the item echoes.
func pickRecordForItem(itemMap map[string]any, matched []AnalysisRecord) (AnalysisRecord, bool) {
	switch len(matched) {
	case 0:
		return AnalysisRecord{}, false
	case 1:
		return matched[0], true
	}
	best, bestScore := matched[0], -1
	for _, rec := range matched {
		score := 0
		for k, v := range itemMap {
			if s, ok := v.(string); ok && s != "" && rec.Metadata[k] == s {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = rec, score
		}
	}
	return best, true
}

// createAssociations links an insight to the unique entities from the provided
// records. When called with matched records (filtered by entity_external_id),
// this links only the relevant entity. When called with all records (fallback),
// every input entity is associated.
func (e *Engine) createAssociations(
	ctx context.Context,
	insightID string,
	records []AnalysisRecord,
	log zerolog.Logger,
) {
	seen := make(map[string]bool, len(records))

	for _, rec := range records {
		if rec.EntityID == "" || seen[rec.EntityID] {
			continue
		}
		seen[rec.EntityID] = true

		entityID := rec.EntityID
		if err := e.insightRepo.CreateRef(ctx, insightID, &entityID, nil); err != nil {
			log.Debug().Err(err).Str("entity_id", entityID).Msg("failed to create entity ref")
		}
	}

	if len(seen) > 0 {
		log.Debug().Int("entity_refs", len(seen)).Msg("linked insight to input entities")
	}
}

// resolveRefFieldEntities looks up entities by external_id from the specified
// ref_fields in the LLM result item and creates additional insight refs. This
// enables cross-layer associations — e.g. linking an infrastructure threat
// insight to both the hazard entity (primary) and the infrastructure entity.
// Each RefField specifies the layer_type to scope the lookup and prevent
// collisions when different layers share the same external_id.
func (e *Engine) resolveRefFieldEntities(
	ctx context.Context,
	insightID string,
	itemMap map[string]any,
	refFields []aiconfig.RefField,
	log zerolog.Logger,
) ([]string, []domain.InsightEntityRef) {
	var ids []string
	var refs []domain.InsightEntityRef

	for _, rf := range refFields {
		extID, ok := itemMap[rf.Field]
		if !ok {
			continue
		}
		extIDStr, ok := extID.(string)
		if !ok || extIDStr == "" {
			continue
		}

		ent, err := e.entityRepo.GetByExternalID(ctx, rf.LayerType, extIDStr)
		if err != nil {
			log.Debug().Err(err).Str("field", rf.Field).Str("layer_type", rf.LayerType).Str("external_id", extIDStr).Msg("failed to resolve ref_field entity")
			continue
		}
		entID := ent.ID
		if err := e.insightRepo.CreateRef(ctx, insightID, &entID, nil); err != nil {
			log.Debug().Err(err).Str("entity_id", entID).Str("field", rf.Field).Msg("failed to create ref_field entity ref")
			continue
		}
		ids = append(ids, entID)
		refs = append(refs, domain.InsightEntityRef{
			ID:         entID,
			ExternalID: ent.ExternalID,
			Name:       ent.Name,
			LayerType:  ent.LayerType,
		})
	}

	return ids, refs
}

// cleanupExpiredInsights periodically removes expired insights.
func (e *Engine) cleanupExpiredInsights(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := e.insightRepo.DeleteExpired(ctx)
			if err != nil {
				e.logger.Warn().Err(err).Msg("failed to cleanup expired insights")
			} else if deleted > 0 {
				e.logger.Info().Int64("deleted", deleted).Msg("cleaned up expired insights")
			}
		}
	}
}
