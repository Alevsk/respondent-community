// Package feeder provides the core ingestion service and supporting types.
package feeder

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/enrichment"
	"github.com/Alevsk/respondent/internal/config"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest"
)

// SourceConfig holds configuration for a data source.
type SourceConfig struct {
	Name                  string
	Interval              time.Duration
	Timeout               time.Duration
	APIURL                string
	ObservationRecordMode config.ObservationRecordMode
	DryRun                bool // When true, fetch+parse+log but do NOT persist to DB/cache
}

// sourceProvider resolves a registered layer source by name. The concrete
// *ingest.SourceRegistry satisfies it; the service depends on the interface (DIP).
type sourceProvider interface {
	Get(name string) (ingest.LayerSource, bool)
}

// IngestionService runs the ingestion daemon: it polls each enabled source,
// writes entities/observations through the injected repositories, and
// publishes layer updates via the injected publisher.
type IngestionService struct {
	logger          zerolog.Logger
	registry        sourceProvider
	publisher       FeederPublisher // pub/sub adapter (nil skips publishing)
	entityRepo      domain.EntityRepository
	obsRepo         domain.ObservationRepository
	enabledSources  []string
	sourceConfigs   map[string]SourceConfig
	stopCh          chan struct{}
	wg              sync.WaitGroup
	enrichPublisher enrichment.JobPublisher             // nil if AI not configured
	sourceAIConfigs map[string]*aiconfig.SourceAIConfig // per-source AI config from YAML
	sem             chan struct{}                       // concurrency limit
}

// NewIngestionService creates a new ingestion service.
// publisher is optional (pass nil to skip publishing).
// enrichPublisher and sourceAIConfigs are optional (pass nil to disable AI enrichment publishing).
func NewIngestionService(
	logger zerolog.Logger,
	registry sourceProvider,
	publisher FeederPublisher,
	entityRepo domain.EntityRepository,
	obsRepo domain.ObservationRepository,
	enabledSources []string,
	sourceConfigs map[string]SourceConfig,
	enrichPublisher enrichment.JobPublisher,
	sourceAIConfigs map[string]*aiconfig.SourceAIConfig,
	concurrency int,
) *IngestionService {
	return &IngestionService{
		logger:          logger,
		registry:        registry,
		publisher:       publisher,
		entityRepo:      entityRepo,
		obsRepo:         obsRepo,
		enabledSources:  enabledSources,
		sourceConfigs:   sourceConfigs,
		stopCh:          make(chan struct{}),
		enrichPublisher: enrichPublisher,
		sourceAIConfigs: sourceAIConfigs,
		sem:             make(chan struct{}, IngestConcurrency(concurrency)),
	}
}

// IngestConcurrency resolves how many sources may ingest at once.
//
// Each in-flight ingest holds a whole decoded catalog, so this number
// multiplies the resident working set directly. A hardcoded 4 was the worst
// possible choice on a single-vCPU host: four pipelines there do not run in
// parallel, they interleave — no throughput is gained while four working sets
// stay live at once and every slot is held four times longer. Deriving it from
// the CPUs actually available keeps the multiplier honest: 1 on a droplet, 8
// on a 16-core box. A configured value wins.
func IngestConcurrency(configured int) int {
	if configured > 0 {
		return configured
	}
	if half := runtime.GOMAXPROCS(0) / 2; half > 1 {
		return half
	}
	return 1
}

// executeIngest acquires the concurrency semaphore before running ingestSource
// to prevent "thundering herd" memory spikes (e.g. at startup).
func (s *IngestionService) executeIngest(ctx context.Context, name string) {
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return
	case <-s.stopCh:
		return
	}

	if err := s.ingestSource(ctx, name); err != nil {
		s.logger.Error().Str("source", name).Err(err).Msg("ingestion failed")
	}
}

// Start begins the ingestion daemon.
func (s *IngestionService) Start(ctx context.Context) {
	s.logger.Info().Msg("starting ingestion daemon")

	for _, sourceName := range s.enabledSources {
		cfg, ok := s.sourceConfigs[sourceName]
		if !ok {
			s.logger.Warn().Str("source", sourceName).Msg("source configuration not found")
			continue
		}

		s.wg.Add(1)
		go func(name string, interval time.Duration) {
			defer s.wg.Done()
			s.logger.Info().
				Str("source", name).
				Dur("interval", interval).
				Msg("starting source ticker")

			// Initial ingest bounded by semaphore
			s.executeIngest(ctx, name)

			// The ticker starts only once this source's first ingest has
			// returned. Constructing every ticker up front phase-locked all
			// sources sharing an interval to the same instant, so the 16
			// hourly sources re-stampeded together forever. Starting the clock
			// after a semaphore-serialised first run spreads them out by
			// construction, with no offset to invent or tune.
			t := time.NewTicker(interval)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-s.stopCh:
					return
				case <-t.C:
					s.executeIngest(ctx, name)
				}
			}
		}(sourceName, cfg.Interval)
	}
}

// Stop gracefully stops the ingestion daemon and waits for in-flight
// ingestion goroutines to return, so no goroutine writes to the database
// after Stop() (and therefore after db.Close on shutdown).
func (s *IngestionService) Stop() {
	s.logger.Info().Msg("stopping ingestion daemon")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info().Msg("stopped all source tickers")
}

// IngestAll runs a one-time ingestion for all enabled sources.
func (s *IngestionService) IngestAll(ctx context.Context) error {
	s.logger.Info().Msg("running one-time ingestion for all enabled sources")

	for _, sourceName := range s.enabledSources {
		if err := s.ingestSource(ctx, sourceName); err != nil {
			s.logger.Error().Str("source", sourceName).Err(err).Msg("ingestion failed")
		}
	}

	s.logger.Info().Msg("one-time ingestion completed")
	return nil
}

// IngestSource runs a one-time ingestion for a specific source.
func (s *IngestionService) IngestSource(ctx context.Context, sourceName string) error {
	return s.ingestSource(ctx, sourceName)
}

// ingestSource fetches and processes data from a single source.
func (s *IngestionService) ingestSource(ctx context.Context, sourceName string) error {
	// Look up the adapter directly by its config name
	source, ok := s.registry.Get(sourceName)
	if !ok {
		return fmt.Errorf("source %q not registered", sourceName)
	}

	s.logger.Info().Str("source", sourceName).Msg("starting ingestion")

	// Start the source (this triggers data fetching in adapters)
	if err := source.Start(ctx); err != nil {
		s.logger.Error().Str("source", sourceName).Err(err).Msg("failed to start source")
		return fmt.Errorf("failed to start source: %w", err)
	}

	// Get entities from the source
	entities, observations, err := source.Snapshot(ctx)
	if err != nil {
		s.logger.Error().Str("source", sourceName).Err(err).Msg("failed to get snapshot")
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	s.logger.Info().
		Str("source", sourceName).
		Int("entities", len(entities)).
		Int("observations", len(observations)).
		Msg("data fetched")

	// Dry-run mode: log results but skip all persistence
	if sc, ok := s.sourceConfigs[sourceName]; ok && sc.DryRun {
		s.logger.Info().
			Str("source", sourceName).
			Int("entities", len(entities)).
			Int("observations", len(observations)).
			Msg("dry-run: skipping persistence")
		return nil
	}

	// Persist to the hot cache
	if err := s.persistEntities(ctx, sourceName, entities, observations); err != nil {
		s.logger.Error().Str("source", sourceName).Err(err).Msg("failed to persist entities")
		return fmt.Errorf("failed to persist entities: %w", err)
	}

	s.logger.Info().Str("source", sourceName).Msg("ingestion completed")
	return nil
}

// persistEntities writes entities and observations to the durable store (SQLite).
func (s *IngestionService) persistEntities(ctx context.Context, sourceName string, entities []*domain.Entity, observations []*domain.Observation) error {
	if len(entities) == 0 {
		return nil
	}

	// Stamp source_type on all observations for provenance tracking
	for _, obs := range observations {
		obs.SourceType = sourceName
	}

	// --- Durable store: durable historical storage ---
	// Upsert entities (idempotent on layer_type + external_id)
	if err := s.entityRepo.CreateBatch(ctx, entities); err != nil {
		s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to persist entities to the database")
		// Continue to cache write — cache should still work even if DB is down
	} else {
		s.logger.Debug().
			Str("source", sourceName).
			Int("count", len(entities)).
			Msg("entities upserted to the database")
	}

	// We need the DB-assigned entity IDs (UUIDs) for the observations FK.
	// After the batch upsert, look up all entities in one query by (layer_type, external_id).
	entityIDMap := make(map[string]string, len(entities)) // externalID -> DB UUID
	if len(entities) > 0 {
		// Group entities by layer type for batch lookup.
		byLayer := make(map[string][]string)
		for _, entity := range entities {
			byLayer[entity.LayerType] = append(byLayer[entity.LayerType], entity.ExternalID)
		}
		for layerType, extIDs := range byLayer {
			dbEntities, err := s.entityRepo.GetByExternalIDs(ctx, layerType, extIDs)
			if err != nil {
				s.logger.Error().Err(err).Str("source", sourceName).Str("layer_type", layerType).Msg("failed to batch lookup entities")
				continue
			}
			for _, e := range dbEntities {
				entityIDMap[e.ExternalID] = e.ID
			}
		}
	}

	// Build a lookup from in-memory entity ID (e.g. "conflict_events:804") to DB UUID.
	// This avoids an O(n*m) nested loop when observations outnumber entities
	// (e.g., time-series sources where many observations map to the same entity).
	inMemoryToDBID := make(map[string]string, len(entities))
	for _, entity := range entities {
		if dbID, ok := entityIDMap[entity.ExternalID]; ok {
			inMemoryToDBID[entity.ID] = dbID
		}
	}

	// Map observations to their DB entity IDs and insert
	var dbObservations []*domain.Observation
	for _, obs := range observations {
		if dbID, ok := inMemoryToDBID[obs.EntityID]; ok {
			// Create a copy with the DB entity ID for the durable store
			dbObs := *obs
			dbObs.EntityID = dbID
			dbObservations = append(dbObservations, &dbObs)
		}
	}

	// newObsEntityIDs tracks entity IDs that had observations actually
	// persisted (passed dedupe). Populated inside the dedupe branch below.
	newObsEntityIDs := make(map[string]struct{})

	recordMode := config.RecordAppend
	if sc, ok := s.sourceConfigs[sourceName]; ok {
		recordMode = sc.ObservationRecordMode
	}

	if len(dbObservations) > 0 {
		// Compute content hash for observations that don't already have one.
		// Sources with a CEL content_hash expression set it during mapping;
		// only fall back to the position+metadata hash when no explicit hash exists.
		for _, obs := range dbObservations {
			if obs.ContentHash == "" {
				hash, err := obs.ComputeContentHash()
				if err != nil {
					s.logger.Warn().Err(err).Str("entity_id", obs.EntityID).Msg("failed to compute content hash, skipping")
					continue
				}
				obs.ContentHash = hash
			}
		}

		switch recordMode {
		case config.RecordUpsert:
			if err := s.obsRepo.CreateBatchUpsert(ctx, dbObservations); err != nil {
				s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to upsert observations to the database")
			} else {
				s.logger.Debug().
					Str("source", sourceName).
					Int("count", len(dbObservations)).
					Msg("observations upserted to the database")
			}

		case config.RecordDedupe:
			// Batch-fetch latest content hashes and filter out unchanged observations
			entityIDs := make([]string, len(dbObservations))
			for i, obs := range dbObservations {
				entityIDs[i] = obs.EntityID
			}
			existingHashes, err := s.obsRepo.GetLatestContentHashes(ctx, entityIDs)
			if err != nil {
				s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to fetch content hashes")
				// Fall through to append on error
				if err := s.obsRepo.CreateBatch(ctx, dbObservations); err != nil {
					s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to persist observations to the database")
				}
			} else {
				var filtered []*domain.Observation
				skipped := 0
				for _, obs := range dbObservations {
					if existing, ok := existingHashes[obs.EntityID]; ok && existing == obs.ContentHash {
						skipped++
						continue
					}
					filtered = append(filtered, obs)
					newObsEntityIDs[obs.EntityID] = struct{}{}
				}
				s.logger.Debug().
					Str("source", sourceName).
					Int("total", len(dbObservations)).
					Int("skipped", skipped).
					Int("new", len(filtered)).
					Msg("dedupe filtered observations")
				if len(filtered) > 0 {
					if err := s.obsRepo.CreateBatch(ctx, filtered); err != nil {
						s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to persist observations to the database")
					} else {
						s.logger.Debug().
							Str("source", sourceName).
							Int("count", len(filtered)).
							Msg("observations inserted to the database")
					}
				}
			}

		default: // RecordAppend
			if err := s.obsRepo.CreateBatch(ctx, dbObservations); err != nil {
				s.logger.Error().Err(err).Str("source", sourceName).Msg("failed to persist observations to the database")
			} else {
				s.logger.Debug().
					Str("source", sourceName).
					Int("count", len(dbObservations)).
					Msg("observations inserted to the database")
			}
		}
	}

	// --- Broadcast, and the hot cache alongside it ---
	//
	// Only entities with new observations are broadcast. In dedupe mode the rest
	// are unchanged, so rebroadcasting them would say nothing new.
	broadcast := 0
	obsMap := make(map[string]*domain.Observation, len(observations))
	for _, obs := range observations {
		obsMap[obs.EntityID] = obs
	}

	for _, entity := range entities {
		dbID := entityIDMap[entity.ExternalID]
		if dbID == "" {
			continue
		}

		if recordMode == config.RecordDedupe {
			if _, isNew := newObsEntityIDs[dbID]; !isNew {
				continue
			}
		}

		obs, ok := obsMap[entity.ID]
		if !ok {
			obs = &domain.Observation{
				ID:        fmt.Sprintf("%s-obs", entity.ID),
				EntityID:  entity.ID,
				Timestamp: time.Now(),
			}
		}

		if s.publisher != nil {
			updateJSON, marshalErr := MarshalLayerUpdate(entity, obs)
			if marshalErr == nil {
				_ = s.publisher.PublishLayerUpdate(ctx, entity.LayerType, updateJSON)
				broadcast++
			}
		}
	}

	s.logger.Info().
		Int("total", len(entities)).
		Int("broadcast", broadcast).
		Int("db_persisted", len(dbObservations)).
		Msg("persisted entities")

	// Publish AI enrichment jobs (fire-and-forget — workers handle filtering/caching)
	if s.enrichPublisher != nil {
		if aiCfg, ok := s.sourceAIConfigs[sourceName]; ok && aiCfg.Enabled {
			// Build entity->observation ID mapping from the persisted observations.
			// After CreateBatch/CreateBatchUpsert the observation IDs are written
			// back to the domain.Observation structs by buildObservationArrays.
			obsIDByEntity := make(map[string]string, len(dbObservations))
			for _, obs := range dbObservations {
				obsIDByEntity[obs.EntityID] = obs.ID
			}

			var jobs []enrichment.Job
			seen := make(map[string]struct{})
			for _, entity := range entities {
				dbID := entityIDMap[entity.ExternalID]
				if dbID == "" {
					continue
				}
				if _, dup := seen[dbID]; dup {
					continue
				}
				seen[dbID] = struct{}{}
				jobs = append(jobs, enrichment.Job{
					EntityID:      dbID,
					ObservationID: obsIDByEntity[dbID],
					ExternalID:    entity.ExternalID,
					SourceName:    sourceName,
					LayerType:     entity.LayerType,
				})
			}
			if len(jobs) > 0 {
				if err := s.enrichPublisher.PublishBatch(ctx, jobs); err != nil {
					s.logger.Error().Err(err).Str("source", sourceName).Int("count", len(jobs)).
						Msg("failed to publish enrichment jobs")
				} else {
					s.logger.Debug().Str("source", sourceName).Int("count", len(jobs)).
						Msg("published enrichment jobs")
				}
			}
		}
	}

	return nil
}
