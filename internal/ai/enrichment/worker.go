package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/ai"
	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/ai/prompts"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/geocoder"
	"github.com/Alevsk/respondent/internal/llm"
)

// defaultMaxDeliver is the fallback MaxDeliver when no valid value is provided.
const defaultMaxDeliver = 3

// defaultAckWait is the fallback AckWait when no valid value is provided.
const defaultAckWait = 60 * time.Second

// reaperInterval is how often the reaper sweeps for stranded enrichment logs.
// reaperThreshold is the minimum age (since created_at) a non-terminal row must
// reach before it is considered orphaned by a crash/timeout/panic.
const (
	reaperInterval  = 15 * time.Minute
	reaperThreshold = 30 * time.Minute
)

// schemaProvider supplies JSON Schemas by key and validates/extracts LLM output
// against them. *schema.Registry satisfies it; the worker depends on the
// interface, not the concrete registry (DIP).
type schemaProvider interface {
	SchemaJSON(key string) string
	ValidateAndExtract(key, rawResponse string) (map[string]any, error)
}

// Worker processes AI enrichment jobs from a message stream.
type Worker struct {
	consumer      domain.StreamConsumer
	llmProvider   llm.Provider
	schemas       schemaProvider
	entityRepo    domain.EntityRepository
	obsRepo       domain.ObservationRepository
	aiLogRepo     domain.AIEnrichmentLogRepository
	kvCache       domain.KeyValueCache                // dedup cache (nil disables caching)
	sourceConfigs map[string]*aiconfig.SourceAIConfig // source_name -> AI config
	logger        zerolog.Logger
	concurrency   int
	mu            sync.Mutex // protects cancelFn
	cancelFn      context.CancelFunc
	wg            sync.WaitGroup // tracks background goroutines (e.g. the reaper)
	subject       string         // e.g., "respondent.ai.enrich"
	consumerGroup string
	streamName    string
	maxDeliver    int
	ackWait       time.Duration

	// notifier is an optional WebSocket notifier. When set, the worker pushes
	// ai_enrichment messages after successfully patching ai_metadata.
	notifier notify.Notifier

	// geocoder is an optional geocoder for resolving locations during enrichment.
	geocoder geocoder.Geocoder

	// countryCentroids is a lookup table for country centroid fallback.
	countryCentroids map[string][2]float64

	// spatialCache is the hot cache (Valkey) refreshed after coordinate patching.
	// When set, UpdateCoordinates refreshes both DB and cache geo index.
	spatialCache domain.CacheStorage

	// onEnrichmentComplete is an optional callback invoked when all operations
	// for an entity complete successfully. Used to trigger event-based task
	// pipelines from the tasks engine.
	onEnrichmentComplete func(ctx context.Context, entityID, sourceName, layerType string, metadata map[string]any)

	// celEnv is the shared CEL environment for evaluating operation filters.
	// Safe for concurrent use after creation.
	celEnv *cel.Env

	// compiledFilters caches compiled CEL programs keyed by filter expression
	// string. CEL programs are safe for concurrent evaluation after creation,
	// so we compile once and reuse across all messages that share the same
	// filter expression.
	compiledFilters sync.Map // map[string]cel.Program
}

// WorkerConfig holds all dependencies and settings for the enrichment Worker.
// Using a config struct avoids a long positional parameter list where string
// arguments can be accidentally swapped.
type WorkerConfig struct {
	Consumer      domain.StreamConsumer
	LLMProvider   llm.Provider
	Schemas       schemaProvider
	EntityRepo    domain.EntityRepository
	ObsRepo       domain.ObservationRepository
	AILogRepo     domain.AIEnrichmentLogRepository
	KVCache       domain.KeyValueCache // optional dedup cache
	SourceConfigs map[string]*aiconfig.SourceAIConfig
	Logger        zerolog.Logger
	Concurrency   int
	Subject       string
	ConsumerGroup string
	StreamName    string
	MaxDeliver    int
	AckWait       time.Duration
}

// NewWorker creates a new enrichment worker from the provided config.
// MaxDeliver and AckWait control the NATS consumer retry policy. If MaxDeliver <= 0
// it defaults to 3; if AckWait <= 0 it defaults to 60s.
func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}
	if cfg.MaxDeliver <= 0 {
		cfg.MaxDeliver = defaultMaxDeliver
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = defaultAckWait
	}
	return &Worker{
		consumer:      cfg.Consumer,
		llmProvider:   cfg.LLMProvider,
		schemas:       cfg.Schemas,
		entityRepo:    cfg.EntityRepo,
		obsRepo:       cfg.ObsRepo,
		aiLogRepo:     cfg.AILogRepo,
		kvCache:       cfg.KVCache,
		sourceConfigs: cfg.SourceConfigs,
		logger:        cfg.Logger,
		concurrency:   cfg.Concurrency,
		subject:       cfg.Subject,
		consumerGroup: cfg.ConsumerGroup,
		streamName:    cfg.StreamName,
		maxDeliver:    cfg.MaxDeliver,
		ackWait:       cfg.AckWait,
	}
}

// SetNotifier configures an optional WebSocket notifier. When set, the worker
// pushes ai_enrichment messages after successfully patching ai_metadata.
// Must be called before Start().
func (w *Worker) SetNotifier(n notify.Notifier) {
	w.notifier = n
}

// SetGeocoder sets the geocoder for coordinate resolution during enrichment.
// Must be called before Start().
func (w *Worker) SetGeocoder(g geocoder.Geocoder) {
	w.geocoder = g
}

// SetCountryCentroids sets the country centroid lookup table used as the
// final fallback tier in geo-resolution. Must be called before Start().
func (w *Worker) SetCountryCentroids(centroids map[string][2]float64) {
	w.countryCentroids = centroids
}

// SetSpatialCache sets the hot cache used to refresh Valkey after coordinate patching.
// Must be called before Start().
func (w *Worker) SetSpatialCache(c domain.CacheStorage) {
	w.spatialCache = c
}

// SetOnEnrichmentComplete configures an optional callback that fires when all
// operations for an entity complete successfully. This is used to trigger
// event-based task pipelines. Must be called before Start().
func (w *Worker) SetOnEnrichmentComplete(fn func(ctx context.Context, entityID, sourceName, layerType string, metadata map[string]any)) {
	w.onEnrichmentComplete = fn
}

// Start creates a durable consumer and begins processing enrichment jobs.
// It blocks until the provided context is cancelled or Stop() is called.
func (w *Worker) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.mu.Lock()
	w.cancelFn = cancel
	w.mu.Unlock()

	if err := w.initCELEnv(); err != nil {
		cancel()
		return fmt.Errorf("init CEL env: %w", err)
	}

	w.logger.Info().
		Str("consumer", w.consumerGroup).
		Str("subject", w.subject+".>").
		Int("concurrency", w.concurrency).
		Msg("starting enrichment worker")

	// Launch the stranded-log reaper BEFORE the blocking Consume call below.
	// It is tracked by w.wg and drained in Stop() via wg.Wait().
	w.wg.Add(1)
	go w.runReaper(ctx)

	err := w.consumer.Consume(ctx, domain.StreamConsumerConfig{
		StreamName:    w.streamName,
		ConsumerGroup: w.consumerGroup,
		FilterSubject: w.subject + ".>",
		MaxDeliver:    w.maxDeliver,
		AckWait:       w.ackWait,
		MaxMessages:   w.concurrency,
	}, func(msg domain.ConsumedMessage) {
		w.processConsumedMessage(ctx, msg)
	})
	if err != nil {
		cancel()
		return fmt.Errorf("consume: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	w.mu.Lock()
	cf := w.cancelFn
	w.mu.Unlock()

	if cf != nil {
		cf()
	}
	// Drain background goroutines (the reaper) launched in Start. The reaper
	// selects on ctx.Done(), so cancelling above guarantees it returns and
	// wg.Wait() does not deadlock.
	w.wg.Wait()
	w.logger.Info().Msg("enrichment worker stopped")
}

// runReaper periodically reconciles audit rows stranded in a non-terminal status
// (pending/processing) by a crash/timeout/panic before a terminal UpdateStatus.
// Such rows would otherwise remain "pending" forever. It mirrors the analysis
// engine's cleanupExpiredInsights: it runs on a ticker and exits on context
// cancellation so Stop()'s wg.Wait() drains cleanly without deadlocking.
func (w *Worker) runReaper(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := w.aiLogRepo.DeleteStrandedLogs(ctx, reaperThreshold, []string{domain.AIStatusPending, domain.AIStatusProcessing})
			if err != nil {
				w.logger.Warn().Err(err).Msg("reaper: failed to reconcile stranded enrichment logs")
			} else if n > 0 {
				w.logger.Info().Int64("count", n).Msg("reaper: marked stranded enrichment logs failed")
			}
		}
	}
}

// initCELEnv creates the shared CEL environment used for filter evaluation.
// Variables exposed: entity.id, entity.external_id, entity.name, entity.layer_type,
// entity.metadata (map), observation.metadata (map), observation.altitude,
// observation.lat, observation.lon.
func (w *Worker) initCELEnv() error {
	env, err := cel.NewEnv(
		cel.Variable("entity", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("observation", cel.MapType(cel.StringType, cel.DynType)),
		ext.Strings(),
	)
	if err != nil {
		return fmt.Errorf("create CEL env: %w", err)
	}
	w.celEnv = env
	return nil
}

// processConsumedMessage deserializes a message and dispatches it through
// processJob. In-process delivery is at-most-once: the bus has no redelivery
// (Ack/Nak/NakWithDelay are no-ops), so every outcome is terminal and the
// message is always acked. Transient and terminal failures differ only in log
// level. A top-level recover keeps one poison job from killing the single
// consume goroutine for the rest of the process lifetime.
func (w *Worker) processConsumedMessage(ctx context.Context, msg domain.ConsumedMessage) {
	defer func() {
		if r := recover(); r != nil {
			// Do not swallow a graceful shutdown: if the context was cancelled,
			// the panic is almost certainly a side effect of teardown — re-raise
			// so it is not silently masked.
			if ctx.Err() != nil {
				panic(r)
			}
			w.logger.Error().
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("recovered from panic processing enrichment job; dropping message")
			_ = msg.Ack()
		}
	}()

	if err := w.processJob(ctx, msg.Data()); err != nil {
		// In-process delivery is at-most-once: there is no redelivery, so every
		// outcome is terminal here. The feeder re-publishes a job per entity each
		// ingest cycle, which provides natural retry for transient failures.
		if isTransient(err) {
			w.logger.Warn().Err(err).Msg("transient failure dropped (at-most-once); will retry on next feeder cycle")
		} else {
			w.logger.Error().Err(err).Msg("terminal error processing job; dropping")
		}
	}
	_ = msg.Ack()
}

// processJob is the core job handler. Exported-friendly (lower-case) so
// tests can call it directly without a NATS message wrapper.
func (w *Worker) processJob(ctx context.Context, data []byte) error {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		// Malformed JSON is terminal — redelivery will not help.
		w.logger.Error().Err(err).Msg("unmarshal enrichment job")
		return fmt.Errorf("unmarshal job: %w", err)
	}

	log := w.logger.With().
		Str("entity_id", job.EntityID).
		Str("source", job.SourceName).
		Str("layer", job.LayerType).
		Logger()

	cfg, ok := w.sourceConfigs[job.SourceName]
	if !ok || !cfg.Enabled {
		log.Debug().Msg("no AI config for source, skipping")
		return nil
	}

	var errs []error
	for i := range cfg.Operations {
		op := &cfg.Operations[i]
		if err := w.processOperation(ctx, &job, op, log); err != nil {
			log.Error().Err(err).Str("operation", op.Name).Msg("operation failed")
			errs = append(errs, fmt.Errorf("op %s: %w", op.Name, err))
		}
	}

	// Fire enrichment-complete callback when all operations succeed.
	// This triggers event-based task pipelines.
	if len(errs) == 0 && w.onEnrichmentComplete != nil {
		w.onEnrichmentComplete(ctx, job.EntityID, job.SourceName, job.LayerType, nil)
	}

	return errors.Join(errs...)
}

// processOperation runs a single AI operation for an entity.
func (w *Worker) processOperation(ctx context.Context, job *Job, op *aiconfig.OperationConfig, log zerolog.Logger) error {
	startTime := time.Now()

	log = log.With().Str("operation", op.Name).Logger()

	// 1. Entity-level dedup cache check.
	entityCacheKey := fmt.Sprintf("ai:enrich:%s:%s", job.EntityID, op.Name)
	if w.kvCache != nil {
		cached, err := w.kvCache.Get(ctx, entityCacheKey)
		if err == nil && cached != "" {
			log.Debug().Msg("entity-level cache hit, skipping")
			return nil
		}
	}

	// 2. Load entity from DB.
	entity, err := w.entityRepo.GetByID(ctx, job.EntityID)
	if err != nil {
		if domain.IsNotFound(err) {
			// NOT_FOUND is terminal: the entity was deleted (e.g. DB reset) or
			// never existed. Return nil so the message is acked and dropped —
			// returning an error would log a spurious "terminal error" and, under
			// any future redelivery transport, cause a retry storm.
			log.Warn().Msg("entity not found (stale message?), dropping job")
			return nil
		}
		return &transientError{err: fmt.Errorf("load entity: %w", err)}
	}

	// 3. Load latest observation (optional, non-fatal if missing).
	var obs *domain.Observation
	obs, err = w.obsRepo.GetLatest(ctx, job.EntityID)
	if err != nil {
		// A missing observation is expected for entities that don't have one yet
		// (e.g. news articles before geo-resolution); only a real DB error warrants
		// a warning. Either way the operation proceeds using entity fields.
		if domain.IsNotFound(err) {
			log.Debug().Msg("no observation for entity, proceeding without")
		} else {
			log.Warn().Err(err).Msg("failed to load latest observation, proceeding without")
		}
		obs = nil
	}

	// 4. Prepare audit log fields (created later, after prompt hash is known).
	// Use the observation ID from the DB-fetched observation (not the job) to
	// avoid FK violations when the feeder-generated ID wasn't actually inserted
	// (e.g. ON CONFLICT DO NOTHING deduplication).
	logID := uuid.New().String()
	var obsIDPtr *string
	if obs != nil && obs.ID != "" {
		obsIDPtr = &obs.ID
	}

	// 5. Evaluate CEL filter if present.
	if op.Filter != "" {
		pass, filterErr := w.evaluateCELFilter(op.Filter, entity, obs)
		if filterErr != nil {
			log.Warn().Err(filterErr).Msg("CEL filter evaluation error, treating as reject")
			pass = false
		}
		if !pass {
			log.Debug().Msg("CEL filter rejected entity")
			// Create audit log directly as skipped (no prompt hash for filtered operations).
			w.createAuditLog(ctx, logID, job.EntityID, obsIDPtr, job.SourceName, op.Name, nil, domain.AIStatusSkipped, log)
			return nil
		}
	}

	// 6. Render prompt.
	// Schema key matches the registration format (single source of truth: SchemaKey).
	schemaKey := SchemaKey(job.SourceName, op.Name)
	schemaJSON := w.schemas.SchemaJSON(schemaKey)
	promptData := prompts.NewPromptData(entity, obs, schemaJSON)
	rendered, err := prompts.Render(op.Prompt, promptData)
	if err != nil {
		// Create audit log as failed (no prompt hash since rendering failed).
		w.createAuditLog(ctx, logID, job.EntityID, obsIDPtr, job.SourceName, op.Name, nil, domain.AIStatusFailed, log)
		return fmt.Errorf("render prompt: %w", err)
	}

	promptHash := prompts.Hash(rendered)

	// 7. Create audit log entry with the computed prompt hash.
	w.createAuditLog(ctx, logID, job.EntityID, obsIDPtr, job.SourceName, op.Name, &promptHash, domain.AIStatusPending, log)

	// 8. Prompt-hash cache check.
	promptCacheKey := fmt.Sprintf("ai:prompt:%s", promptHash)
	var llmResult map[string]any

	if w.kvCache != nil {
		cachedResult, cacheErr := w.kvCache.Get(ctx, promptCacheKey)
		if cacheErr == nil && cachedResult != "" {
			log.Debug().Msg("prompt-hash cache hit, using cached result")
			if unmarshalErr := json.Unmarshal([]byte(cachedResult), &llmResult); unmarshalErr != nil {
				log.Warn().Err(unmarshalErr).Msg("corrupt prompt cache, calling LLM")
				llmResult = nil
			}
		}
	}

	var usage *domain.AIUsage

	// 9. Call LLM if no cached result.
	if llmResult == nil {
		req := &llm.CompletionRequest{
			Messages: []llm.Message{
				{Role: "user", Content: rendered},
			},
			MaxTokens:   op.MaxTokens,
			Temperature: op.Temperature,
		}

		// When an output schema is defined, enforce JSON mode at the API level
		// and prepend a JSON-only system message as defense-in-depth.
		if len(op.OutputSchema) > 0 {
			req.ResponseFormat = llm.ResponseFormatJSON
			req.Messages = append([]llm.Message{
				{Role: "system", Content: "You MUST respond with a valid JSON object. Do not include markdown fences, commentary, or any text outside the JSON structure."},
			}, req.Messages...)
		}

		// The LLM occasionally returns JSON that does not match the schema —
		// wrapped in an extra object, or with missing/renamed fields. That is
		// usually a one-off formatting glitch, but a schema failure is terminal
		// (not retried by RetryableComplete, which only covers the call itself),
		// so a single bad response permanently drops the enrichment. Retry a
		// BOUNDED number of times, each time appending a corrective instruction
		// that names the exact required top-level keys. Bounded by
		// maxSchemaAttempts — no unbounded loop, no token-burn.
		const maxSchemaAttempts = 3
		for attempt := 1; ; attempt++ {
			llmStart := time.Now()
			resp, llmErr := llm.RetryableComplete(ctx, w.llmProvider, req, llm.DefaultMaxRetries)
			if llmErr != nil {
				w.updateAuditLog(ctx, logID, domain.AIStatusFailed, nil, nil, fmt.Sprintf("LLM call: %v", llmErr), log)
				if isLLMTransient(llmErr) {
					return &transientError{err: fmt.Errorf("LLM call: %w", llmErr)}
				}
				return fmt.Errorf("LLM call: %w", llmErr)
			}

			usage = &domain.AIUsage{
				Provider:         w.llmProvider.Name(),
				Model:            resp.Model,
				PromptTokens:     resp.Usage.PromptTokens,
				CompletionTokens: resp.Usage.CompletionTokens,
				LatencyMS:        int(time.Since(llmStart).Milliseconds()),
			}

			// No schema — try to parse as JSON, accept raw if valid. No retry.
			if len(op.OutputSchema) == 0 {
				extracted, extractErr := schema.ExtractJSON(resp.Content)
				if extractErr != nil {
					llmResult = map[string]any{"result": resp.Content}
				} else if unmarshalErr := json.Unmarshal([]byte(extracted), &llmResult); unmarshalErr != nil {
					llmResult = map[string]any{"result": resp.Content}
				}
				break
			}

			// 10. Validate response via schema.
			validated, valErr := w.schemas.ValidateAndExtract(schemaKey, resp.Content)
			if valErr == nil {
				llmResult = validated
				break
			}
			if attempt >= maxSchemaAttempts {
				w.updateAuditLog(ctx, logID, domain.AIStatusFailed, nil, usage, fmt.Sprintf("schema validation after %d attempts: %v", attempt, valErr), log)
				return fmt.Errorf("schema validation after %d attempts: %w", attempt, valErr)
			}
			log.Warn().Err(valErr).Int("attempt", attempt).Msg("LLM output failed schema validation; retrying with corrective prompt")
			req.Messages = append(req.Messages, llm.Message{
				Role:    "system",
				Content: schema.CorrectionMessage(op.OutputSchema),
			})
		}
	}

	// 11. Apply output_mapping allowlist.
	patch := w.applyOutputMapping(op, llmResult)

	// 12. Patch entity or observation ai_metadata.
	// IMPORTANT: Always use the DB-fetched observation (obs) for its ID, never
	// job.ObservationID. The feeder generates speculative UUIDs that may not
	// exist in the DB due to ON CONFLICT deduplication.
	target := op.GetOutputTarget()
	if target == "observation" && obs != nil && obs.ID != "" {
		if err := w.obsRepo.PatchAIMetadata(ctx, obs.ID, patch); err != nil {
			w.updateAuditLog(ctx, logID, domain.AIStatusFailed, llmResult, usage, fmt.Sprintf("patch observation: %v", err), log)
			return &transientError{err: fmt.Errorf("patch observation: %w", err)}
		}
	} else {
		if err := w.entityRepo.PatchAIMetadata(ctx, job.EntityID, patch); err != nil {
			w.updateAuditLog(ctx, logID, domain.AIStatusFailed, llmResult, usage, fmt.Sprintf("patch entity: %v", err), log)
			return &transientError{err: fmt.Errorf("patch entity: %w", err)}
		}
	}

	// 12a. Patch entity coordinates if patch_coordinates is enabled.
	if op.Output.PatchCoordinates.Enabled {
		patchCfg := &PatchCoordinatesCfg{
			LatField:        op.Output.PatchCoordinates.LatField,
			LonField:        op.Output.PatchCoordinates.LonField,
			ConfidenceField: op.Output.PatchCoordinates.ConfidenceField,
			MinConfidence:   op.Output.PatchCoordinates.MinConfidence,
		}
		resolvedLat, resolvedLon, resolvedSource, resolveErr := ResolveCoordinates(
			ctx, llmResult, patchCfg, w.geocoder, w.countryCentroids,
		)
		if resolveErr != nil {
			// No resolvable location is an expected outcome for non-geographic
			// content; only a genuine geocoder/provider failure warrants a warning.
			if errors.Is(resolveErr, ErrNoGeoContent) {
				log.Debug().Msg("no resolvable location, coordinates not updated")
			} else {
				log.Warn().Err(resolveErr).Msg("geo resolution failed, coordinates not updated")
			}
		} else {
			if coordErr := w.entityRepo.UpdateCoordinates(ctx, job.EntityID, resolvedLat, resolvedLon); coordErr != nil {
				log.Warn().Err(coordErr).Msg("failed to update entity coordinates")
			} else {
				log.Info().
					Float64("lat", resolvedLat).
					Float64("lon", resolvedLon).
					Str("source", resolvedSource).
					Msg("entity coordinates updated via geo enrichment")

				// Refresh Valkey cache (geo index + entity hash) so the
				// frontend sees updated coordinates immediately.
				if w.spatialCache != nil && obs != nil {
					if obs.Position == nil {
						obs.Position = &domain.GeoPoint{}
					}
					obs.Position.Lat = resolvedLat
					obs.Position.Lon = resolvedLon
					if cacheErr := w.spatialCache.SetEntity(ctx, entity, obs); cacheErr != nil {
						log.Warn().Err(cacheErr).Msg("failed to refresh spatial cache after coordinate patch")
					}
				}
			}
		}
	}

	// 12b. Push WebSocket notification if notifier is available.
	if w.notifier != nil {
		enrichedFields := make([]string, 0, len(patch))
		for k := range patch {
			enrichedFields = append(enrichedFields, k)
		}
		if notifyErr := w.notifier.NotifyEnrichment(ctx, job.EntityID, op.Name, job.SourceName, enrichedFields); notifyErr != nil {
			log.Warn().Err(notifyErr).Msg("failed to push ai_enrichment WebSocket notification")
		}
	}

	// 13. Set Valkey caches.
	cacheTTL := w.parseCacheTTL(op.CacheTTL)
	if cacheTTL > 0 && w.kvCache != nil {
		resultJSON, _ := json.Marshal(llmResult)
		_ = w.kvCache.Set(ctx, entityCacheKey, "1", cacheTTL)
		_ = w.kvCache.Set(ctx, promptCacheKey, string(resultJSON), cacheTTL)
	}

	// 14. Update audit log as completed.
	w.updateAuditLog(ctx, logID, domain.AIStatusCompleted, llmResult, usage, "", log)

	log.Info().
		Str("target", target).
		Dur("elapsed", time.Since(startTime)).
		Msg("operation completed")

	return nil
}

// applyOutputMapping filters the LLM result through the output_mapping allowlist.
// If no mapping is defined, the entire result is stored under the operation name key.
func (w *Worker) applyOutputMapping(op *aiconfig.OperationConfig, result map[string]any) map[string]any {
	if len(op.OutputMapping) == 0 {
		// No mapping — store entire result under operation name.
		return map[string]any{op.Name: result}
	}

	patch := make(map[string]any, len(op.OutputMapping))
	for sourceKey, targetKey := range op.OutputMapping {
		if val, ok := result[sourceKey]; ok {
			patch[targetKey] = val
		}
	}
	return patch
}

// evaluateCELFilter compiles and evaluates a CEL filter expression against
// entity and observation data. Returns true if the entity passes the filter.
// Compiled programs are cached in compiledFilters so repeated evaluations of
// the same filter expression skip compilation.
func (w *Worker) evaluateCELFilter(filter string, entity *domain.Entity, obs *domain.Observation) (bool, error) {
	if w.celEnv == nil {
		return false, fmt.Errorf("CEL environment not initialized")
	}

	prg, err := w.getOrCompileCELProgram(filter)
	if err != nil {
		return false, err
	}

	// Build entity activation map.
	entityMap := map[string]any{
		"id":          entity.ID,
		"external_id": entity.ExternalID,
		"name":        entity.Name,
		"layer_type":  entity.LayerType,
	}
	// Convert metadata map[string]string to map[string]any for CEL.
	metaMap := make(map[string]any, len(entity.Metadata))
	for k, v := range entity.Metadata {
		metaMap[k] = v
	}
	entityMap["metadata"] = metaMap

	// Build observation activation map.
	obsMap := map[string]any{
		"lat":      0.0,
		"lon":      0.0,
		"altitude": 0.0,
		"metadata": map[string]any{},
	}
	if obs != nil {
		if obs.Position != nil {
			obsMap["lat"] = obs.Position.Lat
			obsMap["lon"] = obs.Position.Lon
		}
		obsMap["altitude"] = obs.AltitudeM
		obsMeta := make(map[string]any, len(obs.Metadata))
		for k, v := range obs.Metadata {
			obsMeta[k] = v
		}
		obsMap["metadata"] = obsMeta
	}

	out, _, err := prg.Eval(map[string]any{
		"entity":      entityMap,
		"observation": obsMap,
	})
	if err != nil {
		return false, fmt.Errorf("eval filter: %w", err)
	}

	b, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("filter did not return bool, got %T", out.Value())
	}
	return b, nil
}

// getOrCompileCELProgram returns a cached CEL program for the given filter
// expression, compiling and caching it on first access. CEL programs are safe
// for concurrent evaluation after creation.
func (w *Worker) getOrCompileCELProgram(filter string) (cel.Program, error) {
	if cached, ok := w.compiledFilters.Load(filter); ok {
		return cached.(cel.Program), nil
	}

	ast, issues := w.celEnv.Compile(filter)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile filter: %w", issues.Err())
	}

	prg, err := w.celEnv.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(ai.CELFilterCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("create program: %w", err)
	}

	// Store the compiled program. If another goroutine raced us, the first
	// writer wins and subsequent loads return the stored value; compiling
	// twice is harmless.
	w.compiledFilters.Store(filter, prg)
	return prg, nil
}

// createAuditLog creates an enrichment log entry with the given fields.
// promptHash is nullable: nil indicates no hash is available (e.g., CEL filter
// rejected the entity or prompt rendering failed).
// It is a best-effort operation — failures are logged but do not abort processing.
func (w *Worker) createAuditLog(ctx context.Context, logID, entityID string, obsID *string, sourceName, opName string, promptHash *string, status string, log zerolog.Logger) {
	auditLog := &domain.AIEnrichmentLog{
		ID:            logID,
		EntityID:      entityID,
		SourceName:    sourceName,
		OperationName: opName,
		PromptHash:    promptHash,
		Status:        status,
		ObservationID: obsID,
	}
	if err := w.aiLogRepo.Create(ctx, auditLog); err != nil {
		log.Warn().Err(err).Msg("failed to create audit log, continuing")
	}
}

// updateAuditLog updates an enrichment log entry with the final status.
func (w *Worker) updateAuditLog(ctx context.Context, logID, status string, result map[string]any, usage *domain.AIUsage, errMsg string, log zerolog.Logger) {
	if err := w.aiLogRepo.UpdateStatus(ctx, logID, status, result, usage, errMsg); err != nil {
		log.Warn().Err(err).Str("log_id", logID).Msg("failed to update audit log")
	}
}

// parseCacheTTL parses a duration string like "10m", "1h", "300s".
// Returns 0 on empty or invalid input (no cache).
func (w *Worker) parseCacheTTL(ttl string) time.Duration {
	if ttl == "" {
		return 0
	}
	d, err := time.ParseDuration(ttl)
	if err != nil {
		w.logger.Warn().Str("ttl", ttl).Err(err).Msg("invalid cache TTL, disabling cache")
		return 0
	}
	return d
}

// transientError wraps errors that should trigger NATS redelivery (Nak).
type transientError struct {
	err error
}

func (e *transientError) Error() string { return e.err.Error() }
func (e *transientError) Unwrap() error { return e.err }

// isTransient returns true if the error indicates a retryable condition.
func isTransient(err error) bool {
	var te *transientError
	return errors.As(err, &te)
}

// isLLMTransient returns true if an LLM error is likely transient (rate limit,
// provider unavailable, timeout) vs. terminal (bad request, auth failure).
func isLLMTransient(err error) bool {
	if errors.Is(err, llm.ErrRateLimited) || errors.Is(err, llm.ErrProviderUnavailable) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "Client.Timeout")
}
