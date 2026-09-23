package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	// Domain
	"github.com/Alevsk/respondent/internal/domain"

	// Infrastructure — community adapters (Plan 1)
	"github.com/Alevsk/respondent/internal/infra/inmem"
	"github.com/Alevsk/respondent/internal/infra/inproc"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
	"github.com/Alevsk/respondent/internal/ratelimit"

	// Feeder
	"github.com/Alevsk/respondent/internal/app/feeder"
	feederconfig "github.com/Alevsk/respondent/internal/app/feeder/config"
	"github.com/Alevsk/respondent/internal/ingest"
	"github.com/Alevsk/respondent/internal/ingest/declarative"
	"github.com/Alevsk/respondent/internal/logging"

	// AI
	"github.com/Alevsk/respondent/internal/ai/analysis"
	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/enrichment"
	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/llm"
	"github.com/Alevsk/respondent/internal/llm/providers"

	// App services
	appai "github.com/Alevsk/respondent/internal/app/ai"
	"github.com/Alevsk/respondent/internal/app/entity"
	"github.com/Alevsk/respondent/internal/app/indicator"
	"github.com/Alevsk/respondent/internal/app/layer"
	"github.com/Alevsk/respondent/internal/app/media"

	// Transport
	grpctransport "github.com/Alevsk/respondent/internal/transport/grpc"

	// Embedded frontend
	"github.com/Alevsk/respondent/internal/ui"

	// WebSocket
	"github.com/Alevsk/respondent/internal/realtime"

	// Config
	"github.com/Alevsk/respondent/internal/config"

	// Geocoder — enrichment geo-resolution (tier 2 provider + tier 3 centroids)
	"github.com/Alevsk/respondent/internal/geocoder"

	// Proto — gRPC-gateway registration
	respondentv1 "github.com/Alevsk/respondent/gen/go"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the community edition server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context())
		},
	}
}

func runServe(ctx context.Context) error {
	// ── 1. Config ──────────────────────────────────────────────────────
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	logger := newLogger(cfg.Logging.Level)
	logger.Info().Msg("starting respondent community edition")

	// ── 2. SQLite ──────────────────────────────────────────────────────
	db, err := sqlitedb.Open(cfg.Database.Path, logger)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.RunMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	// Give the query planner statistics up front (SQLite ships with none, which
	// makes the multi-plan cross-layer analysis queries pick poor plans). Bounded
	// and best-effort — a failure here must not block startup.
	if err := db.OptimizeStartup(ctx); err != nil {
		logger.Warn().Err(err).Msg("startup PRAGMA optimize failed (non-fatal)")
	}
	logger.Info().Str("path", cfg.Database.Path).Msg("sqlite database opened")

	// ── 3. Repositories ────────────────────────────────────────────────
	repoLogger := logger.With().Str("component", "sqlite").Logger()
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), repoLogger)
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), repoLogger)
	aiInsightRepo := sqlitedb.NewAIInsightRepository(db.SqlDB(), repoLogger)
	aiLogRepo := sqlitedb.NewAIEnrichmentLogRepository(db.SqlDB(), repoLogger)
	layerRepo := sqlitedb.NewLayerRepository(db.SqlDB(), repoLogger)
	_ = layerRepo // layerRepo used for future layer management endpoints

	// ── 4. In-memory cache ─────────────────────────────────────────────
	cacheStorage := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = cacheStorage.Close() }()

	kvCache := inmem.NewKVCache()

	// ── 5. In-process messaging ────────────────────────────────────────
	bus := inproc.NewBus(256)
	msgPublisher := inproc.NewPublisher(bus)
	msgConsumer := inproc.NewConsumer(bus)

	// ── 6. Dynamic source registry ─────────────────────────────────────
	dynReg := domain.NewDynamicSourceRegistry()

	// Register declarative display/layer metadata (control plane) for EVERY source
	// definition, independent of whether its ingestion credentials (data plane)
	// resolve. A source that can't fetch (e.g. a missing API key) must still keep
	// its layer identity — display config, color, filtering/rendering mode — so its
	// persisted entities render with the correct icon/color instead of the bare
	// #ffffff fallback. LoadSources below then overwrites these keys with the fully
	// compiled metadata for sources that load successfully (last-write-wins per
	// layer_type). Best-effort: a parse error must not abort startup.
	if err := declarative.RegisterDisplayConfigs(
		cfg.Ingest.SourcesDir,
		dynReg,
		logging.NewLogger("display-config", nil),
	); err != nil {
		logger.Warn().Err(err).Msg("failed to register declarative display configs")
	}

	// ── 7. Source loading ──────────────────────────────────────────────
	sourceRegistry := ingest.NewSourceRegistry()
	feederCfg := &feederconfig.Config{
		Ingest: config.IngestConfig{
			SourcesDir: cfg.Ingest.SourcesDir,
			DevMode:    cfg.Ingest.DevMode,
		},
	}

	// Shared per-host rate limiter: the feeder crawl AND on-demand viewport fills
	// hit the same upstreams (e.g. api.adsb.lol), so they share one request budget.
	// The feeder client carries it (low priority); on-demand carries it too (high
	// priority) via applyRealtimeConfig below. Per-host limits are applied from each
	// source's declarative rate_limit after loading.
	rlTransport := ratelimit.NewTransport(nil)
	feederHTTPClient := &http.Client{Timeout: 30 * time.Second, Transport: rlTransport}

	enabledSources, sourceConfigs, compiledSources, err := LoadSources(
		logger.With().Str("component", "feeder").Logger(),
		sourceRegistry,
		feederCfg,
		dynReg,
		feederHTTPClient,
	)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}
	logger.Info().Int("count", len(enabledSources)).Msg("loaded sources from sources.d/")

	// Apply per-host rate limits declared by sources (rate_limit:). Keyed by host
	// so every source + on-demand fill hitting that host shares the budget.
	configureHostRateLimits(rlTransport, compiledSources, logger)

	// Extract AI configs from compiled sources for enrichment.
	sourceAIConfigs := CollectSourceAIConfigs(compiledSources)

	// ── 8. LLM provider (optional) ────────────────────────────────────
	var llmProvider llm.Provider
	var llmRegistry *llm.DefaultRegistry

	if cfg.AI.Enabled {
		llmRegistry = buildLLMRegistry(cfg.LLM)
		llmProvider = selectLLMProvider(llmRegistry, cfg.LLM)
		if llmProvider == nil {
			logger.Warn().Msg("AI enabled but no LLM provider available — enrichment/analysis disabled")
		} else {
			logger.Info().Str("provider", llmProvider.Name()).Msg("LLM provider initialized")
		}
	}

	// ── 9. Schema registry ─────────────────────────────────────────────
	schemas := schema.NewRegistry()
	registerEnrichmentSchemas(schemas, sourceAIConfigs, logger)

	// ── 10. WebSocket server ───────────────────────────────────────────
	// The server resolves per-layer spatial filtering from dynReg (populated from
	// the declarative source definitions during LoadSources), so no precomputed
	// layer set is passed — dynReg is the single source of truth.
	wsServer := realtime.NewServer(
		logger.With().Str("component", "websocket").Logger(),
		cacheStorage,
		entityRepo,
		obsRepo,
		dynReg,
	)
	applyRealtimeConfig(wsServer, dynReg, compiledSources, rlTransport, logger)

	// ── 11. Notifier — direct WS bridge ────────────────────────────────
	notifier := notify.Notifier(notify.NewWSNotifier(wsServer, logger.With().Str("component", "notifier").Logger()))

	// ── 12. Enrichment worker (optional) ───────────────────────────────
	var enrichWorker *enrichment.Worker
	var enrichPub enrichment.JobPublisher

	if cfg.AI.Enabled && llmProvider != nil {
		enrichPub = enrichment.NewPublisher(msgPublisher, inproc.EnrichJobRoot)

		enrichWorker = enrichment.NewWorker(enrichment.WorkerConfig{
			Consumer:      msgConsumer,
			LLMProvider:   llmProvider,
			Schemas:       schemas,
			EntityRepo:    entityRepo,
			ObsRepo:       obsRepo,
			AILogRepo:     aiLogRepo,
			KVCache:       kvCache,
			SourceConfigs: sourceAIConfigs,
			Logger:        logger.With().Str("component", "enrichment-worker").Logger(),
			Concurrency:   cfg.AI.Workers.Enrichment,
			Subject:       inproc.EnrichJobRoot,
			ConsumerGroup: "community-enrichment",
			// In-process delivery is at-most-once; redelivery knobs (MaxDeliver/
			// AckWait) do not apply and are intentionally omitted.
			StreamName: "COMMUNITY_AI",
		})
		enrichWorker.SetNotifier(notifier)
		enrichWorker.SetSpatialCache(cacheStorage)

		// Geo-resolution: tier 2 (Nominatim, for specific localities) + tier 3
		// (offline country centroids). The built-in centroid table covers every
		// country; config country_centroids only override specific entries.
		geo, geoErr := geocoder.NewFromConfig(toGeocoderConfig(cfg.Geocoder), kvCache)
		if geoErr != nil {
			return fmt.Errorf("build geocoder: %w", geoErr)
		}
		enrichWorker.SetGeocoder(geo)
		centroids := geocoder.DefaultCentroids()
		for iso3, coords := range cfg.Geocoder.CentroidMap() {
			centroids[iso3] = coords
		}
		enrichWorker.SetCountryCentroids(centroids)

		if err := inproc.ValidateAIPipeline(inproc.EnrichJobRoot); err != nil {
			return fmt.Errorf("AI pipeline validation: %w", err)
		}
	}

	// ── 13. Analysis engine (optional) ─────────────────────────────────
	var analysisEngine *analysis.Engine

	if cfg.AI.Enabled && llmRegistry != nil && cfg.AI.AnalysisDir != "" {
		analysisEngine, err = analysis.NewEngine(analysis.EngineConfig{
			LLMRegistry: llmRegistry,
			EntityRepo:  entityRepo,
			ObsRepo:     obsRepo,
			InsightRepo: aiInsightRepo,
			Schemas:     schemas,
			Logger:      logger.With().Str("component", "analysis-engine").Logger(),
			Notifier:    notifier,
			// Read-only SQLite executor so analysis definitions with data.sql run
			// their (SQLite-dialect) query instead of silently degrading to layer
			// queries. haversine_km is registered for proximity (no PostGIS).
			// Uses the dedicated READ pool: long cross-layer analysis queries run
			// concurrently (WAL readers) instead of monopolizing the single writer
			// connection and stalling ingestion + interactive WebSocket reads.
			QueryExec: sqlitedb.NewReadOnlyQueryExecutor(db.ReadDB()),
			// Engine-wide attention floor for storing/notifying insights; a
			// per-analysis output.min_attention overrides it.
			DefaultMinAttention: cfg.AI.MinAttention,
		})
		if err != nil {
			return fmt.Errorf("create analysis engine: %w", err)
		}

		// NOTE: NewLoader takes (schemas, logger) — corrected from plan
		analysisLoader, err := analysis.NewLoader(schemas, logger)
		if err != nil {
			return fmt.Errorf("create analysis loader: %w", err)
		}
		defs, err := analysisLoader.LoadDefinitions(cfg.AI.AnalysisDir)
		if err != nil {
			return fmt.Errorf("load analysis definitions: %w", err)
		}
		analysisEngine.SetDefinitions(defs)
		logger.Info().Int("count", len(defs)).Msg("loaded analysis definitions from analysis.d/")
	}

	// ── 14. Feeder adapters ────────────────────────────────────────────
	feederCache := &memcacheFeederCache{cache: cacheStorage}
	feederPub := &directFeederPublisher{ws: wsServer}

	// ── 15. Ingestion service ──────────────────────────────────────────
	ingestionSvc := feeder.NewIngestionService(
		logger.With().Str("component", "feeder").Logger(),
		sourceRegistry,
		feederCache,
		feederPub,
		entityRepo,
		obsRepo,
		enabledSources,
		sourceConfigs,
		enrichPub, // nil if AI disabled
		sourceAIConfigs,
	)

	// ── 16. gRPC-gateway API ───────────────────────────────────────────
	layerService := layer.NewLayerService(entityRepo, obsRepo, cacheStorage, dynReg)
	entityService := entity.NewEntityService(entityRepo, obsRepo, logger.With().Str("component", "entity-service").Logger())
	indicatorService := indicator.NewIndicatorService(entityRepo, obsRepo, cacheStorage, dynReg, logger.With().Str("component", "indicator-service").Logger())

	// Playback notifications: the registry owns every outbound media call, and
	// it only ever executes actions a source definition declared. Its client
	// carries the shared per-host rate budget so notifications and catalog
	// polls spend from the same bucket.
	mediaActions, err := declarative.NewMediaActionRegistry(
		compiledSources,
		&http.Client{Timeout: 5 * time.Second, Transport: rlTransport},
		logging.NewLogger("media-actions", nil),
	)
	if err != nil {
		return fmt.Errorf("build media action registry: %w", err)
	}
	mediaService := media.NewService(
		entityService,
		dynReg,
		mediaActions,
		mediaActions,
		logger.With().Str("component", "media-service").Logger(),
	)

	grpcLogger := logger.With().Str("component", "grpc").Logger()
	layerGRPC := grpctransport.NewLayerServer(layerService, grpcLogger)
	mediaGRPC := grpctransport.NewMediaServer(mediaService, grpcLogger)
	entityGRPC := grpctransport.NewEntityServer(entityService, grpcLogger)
	indicatorGRPC := grpctransport.NewIndicatorServer(indicatorService, grpcLogger)

	// AI service — always registered for read-only routes (insights, analysis defs).
	aiSvc := appai.NewAIService(
		entityRepo, obsRepo, aiInsightRepo,
		llmProvider, // nil if AI disabled — read-only routes still work
		nil,         // no query executor in community (no PostgreSQL)
		nil,         // no AI query log in community
		nil,         // analysis loader (community doesn't expose loader via API)
		logger.With().Str("component", "ai-service").Logger(),
	)
	aiGRPC := grpctransport.NewAIServer(aiSvc, grpcLogger)

	gwMux := runtime.NewServeMux()
	if err := respondentv1.RegisterLayerServiceHandlerServer(ctx, gwMux, layerGRPC); err != nil {
		return fmt.Errorf("register layer gateway: %w", err)
	}
	if err := respondentv1.RegisterEntityServiceHandlerServer(ctx, gwMux, entityGRPC); err != nil {
		return fmt.Errorf("register entity gateway: %w", err)
	}
	if err := respondentv1.RegisterIndicatorServiceHandlerServer(ctx, gwMux, indicatorGRPC); err != nil {
		return fmt.Errorf("register indicator gateway: %w", err)
	}
	if err := respondentv1.RegisterAIServiceHandlerServer(ctx, gwMux, aiGRPC); err != nil {
		return fmt.Errorf("register AI gateway: %w", err)
	}
	if err := respondentv1.RegisterMediaServiceHandlerServer(ctx, gwMux, mediaGRPC); err != nil {
		return fmt.Errorf("register media gateway: %w", err)
	}

	// ── 17. HTTP mux ───────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.Handle("GET /readyz", &readinessHandler{db: db.SqlDB()})
	// Runtime config for the embedded SPA (client-safe values from RESPONDENT_FRONTEND_*),
	// so per-deployment frontend config reaches the compiled bundle without a rebuild.
	mux.HandleFunc("GET /config.json", frontendConfigHandler(cfg.Frontend))
	mux.Handle("/v1/", gwMux)
	mux.HandleFunc("/ws", wsServer.HandleConnection)
	mux.Handle("/", ui.NewHandler(ui.EarthDist))

	handler := corsMiddleware(mux)

	// ── 18. Signal handling ────────────────────────────────────────────
	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// ── 19. Start subsystems ───────────────────────────────────────────

	// WebSocket server run loop (handles client connections + broadcasts).
	go wsServer.Run(srvCtx)

	// Feeder — periodic ingestion. Skipped when ingestion is disabled
	// (RESPONDENT_INGEST_ENABLED=false) so the server serves a static instance
	// with no live source fetching or WebSocket data churn (deterministic e2e).
	if cfg.Ingest.Enabled {
		go func() {
			ingestionSvc.Start(srvCtx)
		}()
		logger.Info().Int("sources", len(enabledSources)).Msg("feeder started")
	} else {
		logger.Info().Msg("ingestion disabled (ingest.enabled=false); feeder not started")
	}

	// Enrichment worker.
	if enrichWorker != nil {
		go func() {
			if err := enrichWorker.Start(srvCtx); err != nil {
				logger.Error().Err(err).Msg("enrichment worker stopped with error")
			}
		}()
		logger.Info().Int("workers", cfg.AI.Workers.Enrichment).Msg("enrichment worker started")
	}

	// Analysis engine.
	if analysisEngine != nil {
		go func() {
			if err := analysisEngine.Start(srvCtx); err != nil {
				logger.Error().Err(err).Msg("analysis engine stopped with error")
			}
		}()
		logger.Info().Msg("analysis engine started")
	}

	// Storage retention — opt-in SQLite size cap. Started only when
	// database.retention.max_size is configured; otherwise the database grows
	// unbounded as before. Config was already validated in cfg.Validate().
	if maxBytes, err := cfg.Database.Retention.MaxSizeBytes(); err != nil {
		return fmt.Errorf("parse database.retention.max_size: %w", err)
	} else if maxBytes > 0 {
		ret := cfg.Database.Retention
		maintRepo := sqlitedb.NewMaintenanceRepository(
			db.SqlDB(), repoLogger, ret.BatchSize, ret.MaxBatchesPerTick, ret.IncrementalVacuumPages,
		)
		go runStorageRetention(srvCtx, maintRepo, maxBytes, ret.LowWaterRatio, ret.CheckInterval,
			logger.With().Str("component", "storage-retention").Logger())
		logger.Info().
			Str("max_size", ret.MaxSize).
			Float64("low_water_ratio", ret.LowWaterRatio).
			Dur("check_interval", ret.CheckInterval).
			Msg("storage retention enabled")
	}

	// Refresh query-planner statistics periodically so plans stay good as tables grow.
	go runPeriodicOptimize(srvCtx, db, time.Hour, logger.With().Str("component", "sqlite-optimize").Logger())

	// HTTP server.
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	go func() {
		logger.Info().Int("port", cfg.Server.Port).Msg("server listening")
		logger.Info().Msgf("open http://localhost:%d in your browser", cfg.Server.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("http server error")
		}
	}()

	// ── 20. Graceful shutdown ──────────────────────────────────────────
	<-sigCh
	logger.Info().Msg("received shutdown signal, stopping...")

	srvCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Drain the feeder BEFORE the deferred db.Close so no ingestion goroutine
	// writes to a closed database (lost observations) or leaves the WAL dirty.
	// Bounded by shutdownCtx so a stuck source cannot hang shutdown forever.
	// Skipped when ingestion was never started (ingest.enabled=false).
	if cfg.Ingest.Enabled {
		feederDone := make(chan struct{})
		go func() { ingestionSvc.Stop(); close(feederDone) }()
		select {
		case <-feederDone:
			logger.Info().Msg("feeder drained")
		case <-shutdownCtx.Done():
			logger.Warn().Msg("feeder drain timed out; proceeding to shutdown")
		}
	}

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("http server shutdown error")
	}

	if enrichWorker != nil {
		enrichWorker.Stop()
	}
	if analysisEngine != nil {
		analysisEngine.Stop()
	}

	logger.Info().Msg("respondent community edition stopped")
	return nil
}

// newLogger creates a zerolog logger at the specified level.
func newLogger(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger().Level(lvl)
}

// corsMiddleware adds CORS headers for browser access.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// registerEnrichmentSchemas registers each enrichment operation's output schema
// under the compound "<source>.<operation>" key the worker looks up at runtime
// (enrichment.SchemaKey). Registering under op.Name alone silently fails every
// enrichment with "schema not found". Source name is the map key from
// CollectSourceAIConfigs.
func registerEnrichmentSchemas(reg *schema.Registry, cfgs map[string]*aiconfig.SourceAIConfig, logger zerolog.Logger) {
	for sourceName, sac := range cfgs {
		if sac == nil {
			continue
		}
		for _, op := range sac.Operations {
			if len(op.OutputSchema) == 0 {
				continue
			}
			key := enrichment.SchemaKey(sourceName, op.Name)
			if err := reg.RegisterFromYAML(key, op.OutputSchema); err != nil {
				logger.Warn().Err(err).Str("schema_key", key).Msg("failed to register enrichment schema")
			}
		}
	}
}

// buildLLMRegistry creates and populates the LLM provider registry.
func buildLLMRegistry(llmCfg config.LLMConfig) *llm.DefaultRegistry {
	registry := llm.NewRegistry()

	if llmCfg.OpenAI.APIKey != "" {
		registry.RegisterWithPriority(providers.NewOpenAIProvider(providers.OpenAIConfig{
			APIKey: llmCfg.OpenAI.APIKey, Model: llmCfg.OpenAI.Model,
			MaxTokens: llmCfg.OpenAI.MaxTokens, BaseURL: llmCfg.OpenAI.BaseURL,
		}), 10)
	}
	if llmCfg.Anthropic.APIKey != "" {
		registry.RegisterWithPriority(providers.NewAnthropicProvider(providers.AnthropicConfig{
			APIKey: llmCfg.Anthropic.APIKey, Model: llmCfg.Anthropic.Model,
			MaxTokens: llmCfg.Anthropic.MaxTokens, BaseURL: llmCfg.Anthropic.BaseURL,
		}), 9)
	}
	if llmCfg.Gemini.APIKey != "" {
		registry.RegisterWithPriority(providers.NewGeminiProvider(providers.GeminiConfig{
			APIKey: llmCfg.Gemini.APIKey, Model: llmCfg.Gemini.Model, BaseURL: llmCfg.Gemini.BaseURL,
		}), 8)
	}
	if llmCfg.XAI.APIKey != "" {
		registry.RegisterWithPriority(providers.NewXAIProvider(providers.XAIConfig{
			APIKey: llmCfg.XAI.APIKey, Model: llmCfg.XAI.Model,
			MaxTokens: llmCfg.XAI.MaxTokens, BaseURL: llmCfg.XAI.BaseURL,
		}), 7)
	}
	if llmCfg.ZAI.APIKey != "" {
		zaiTimeout, _ := time.ParseDuration(llmCfg.ZAI.Timeout) // 0 on empty/invalid → provider default
		registry.RegisterWithPriority(providers.NewZAIProvider(providers.ZAIConfig{
			APIKey: llmCfg.ZAI.APIKey, Model: llmCfg.ZAI.Model,
			MaxTokens: llmCfg.ZAI.MaxTokens, BaseURL: llmCfg.ZAI.BaseURL,
			Timeout: zaiTimeout, EnableThinking: llmCfg.ZAI.EnableThinking,
		}), 6)
	}
	if llmCfg.Ollama.BaseURL != "" {
		registry.RegisterWithPriority(providers.NewOllamaProvider(providers.OllamaConfig{
			Model: llmCfg.Ollama.Model, BaseURL: llmCfg.Ollama.BaseURL,
		}), 5)
	}
	if llmCfg.LMStudio.BaseURL != "" {
		registry.RegisterWithPriority(providers.NewLMStudioProvider(providers.LMStudioConfig{
			Model: llmCfg.LMStudio.Model, MaxTokens: llmCfg.LMStudio.MaxTokens, BaseURL: llmCfg.LMStudio.BaseURL,
		}), 4)
	}

	if llmCfg.Provider != "" {
		registry.SetPreferred(llmCfg.Provider)
	}

	return registry
}

// selectLLMProvider picks the preferred LLM provider from the registry.
func selectLLMProvider(registry *llm.DefaultRegistry, llmCfg config.LLMConfig) llm.Provider {
	if registry == nil {
		return nil
	}

	if llmCfg.Provider != "" {
		p, err := registry.GetProvider(llmCfg.Provider)
		if err == nil {
			return p
		}
	}

	p, err := registry.GetPreferredProvider(context.Background())
	if err != nil {
		return nil
	}
	return p
}
