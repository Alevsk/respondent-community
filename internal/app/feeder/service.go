// Package feeder provides the core ingestion service and supporting types.
package feeder

import (
	"context"
	"fmt"
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
	CacheTTL              time.Duration
	ObservationRecordMode config.ObservationRecordMode
	DryRun                bool // When true, fetch+parse+log but do NOT persist to DB/cache
}

// sourceProvider resolves a registered layer source by name. The concrete
// *ingest.SourceRegistry satisfies it; the service depends on the interface (DIP).
type sourceProvider interface {
	Get(name string) (ingest.LayerSource, bool)
}

// IngestionService runs the ingestion daemon: it polls each enabled source,
// writes entities/observations through the injected cache and repositories, and
// publishes layer updates via the injected publisher.
type IngestionService struct {
	logger          zerolog.Logger
	registry        sourceProvider
	cache           FeederCache     // hot cache adapter (nil skips caching)
	publisher       FeederPublisher // pub/sub adapter (nil skips publishing)
	entityRepo      domain.EntityRepository
	obsRepo         domain.ObservationRepository
	enabledSources  []string
	sourceConfigs   map[string]SourceConfig
	tickers         []*time.Ticker
	stopCh          chan struct{}
	wg              sync.WaitGroup
	enrichPublisher enrichment.JobPublisher             // nil if AI not configured
	sourceAIConfigs map[string]*aiconfig.SourceAIConfig // per-source AI config from YAML
	sem             chan struct{}                       // concurrency limit
}

// NewIngestionService creates a new ingestion service.
// cache and publisher are optional (pass nil to skip caching/publishing).
// enrichPublisher and sourceAIConfigs are optional (pass nil to disable AI enrichment publishing).
func NewIngestionService(
	logger zerolog.Logger,
	registry sourceProvider,
	cache FeederCache,
	publisher FeederPublisher,
	entityRepo domain.EntityRepository,
	obsRepo domain.ObservationRepository,
	enabledSources []string,
	sourceConfigs map[string]SourceConfig,
	enrichPublisher enrichment.JobPublisher,
	sourceAIConfigs map[string]*aiconfig.SourceAIConfig,
) *IngestionService {
	return &IngestionService{
		logger:          logger,
		registry:        registry,
		cache:           cache,
		publisher:       publisher,
		entityRepo:      entityRepo,
		obsRepo:         obsRepo,
		enabledSources:  enabledSources,
		sourceConfigs:   sourceConfigs,
		tickers:         make([]*time.Ticker, 0),
		stopCh:          make(chan struct{}),
		enrichPublisher: enrichPublisher,
		sourceAIConfigs: sourceAIConfigs,
		sem:             make(chan struct{}, 4), // bounded to 4 to prevent OOM on startup
	}
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

		ticker := time.NewTicker(cfg.Interval)
		s.tickers = append(s.tickers, ticker)

		s.wg.Add(1)
		go func(name string, t *time.Ticker) {
			defer s.wg.Done()
			s.logger.Info().
				Str("source", name).
				Dur("interval", cfg.Interval).
				Msg("starting source ticker")

			// Initial ingest bounded by semaphore
			s.executeIngest(ctx, name)

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
		}(sourceName, ticker)
	}
}

// Stop gracefully stops the ingestion daemon and waits for in-flight
// ingestion goroutines to return, so no goroutine writes to the database
// after Stop() (and therefore after db.Close on shutdown).
func (s *IngestionService) Stop() {
	s.logger.Info().Msg("stopping ingestion daemon")
	close(s.stopCh)
	for _, ticker := range s.tickers {
		ticker.Stop()
	}
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

	// Dry-run mode: log results but skip all persistence (DB + cache)
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

// persistEntities dual-writes entities to the hot cache and the durable store (SQLite).
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

	// --- Hot cache with TTL ---
	// The index SET is additive via SAdd. Stale members are harmless because
	// GetLayerEntities already skips expired hashes.
	//
	// Only cache entities that have new observations (passed dedupe). Stale
	// entities already in the cache may carry enriched coordinates from the
	// AI pipeline — blindly overwriting them with the raw 0,0 from the feed
	// would undo geo-enrichment.
	cacheCount := 0
	if s.cache != nil {
		obsMap := make(map[string]*domain.Observation)
		for _, obs := range observations {
			obsMap[obs.EntityID] = obs
		}

		// Get TTL from source config
		ttl := s.sourceConfigs[sourceName].CacheTTL
		if ttl <= 0 {
			ttl = 60 * time.Second
		}

		for _, entity := range entities {
			dbID := entityIDMap[entity.ExternalID]
			if dbID == "" {
				continue
			}

			// In dedupe mode, skip cache writes for entities whose observations
			// were filtered out (unchanged content hash). Their cache entries
			// already exist and may contain enriched coordinates.
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

			if err := s.cache.SetEntity(ctx, entity, obs, ttl); err != nil {
				s.logger.Error().Str("external_id", entity.ExternalID).Err(err).Msg("failed to cache entity")
				continue
			}
			cacheCount++

			// Publish update to Pub/Sub
			if s.publisher != nil {
				updateJSON, marshalErr := MarshalLayerUpdate(entity, obs)
				if marshalErr == nil {
					_ = s.publisher.PublishLayerUpdate(ctx, entity.LayerType, updateJSON)
				}
			}
		}
	}

	s.logger.Info().
		Int("total", len(entities)).
		Int("cached", cacheCount).
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
