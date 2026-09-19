package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/app/feeder"
	feederconfig "github.com/Alevsk/respondent/internal/app/feeder/config"
	"github.com/Alevsk/respondent/internal/config"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest"
	"github.com/Alevsk/respondent/internal/ingest/declarative"
	"github.com/Alevsk/respondent/internal/logging"
)

// LoadSources loads all declarative sources from sources_dir, registers their
// adapters in the source registry, and returns the enabled sources list, source
// configs, and compiled sources (for extracting AI configs).
//
// This is composition-root work: it constructs concrete declarative adapters and
// therefore belongs in cmd/ (the wiring layer), NOT in the app/feeder package —
// keeping the app layer decoupled from concrete ingest adapters (DIP).
//
// httpClient, when non-nil, is used by every HTTP source adapter — pass a client
// backed by a shared rate-limiting transport so all sources hitting the same host
// share its request budget. nil lets each adapter build its own default client.
func LoadSources(logger zerolog.Logger, registry *ingest.SourceRegistry, cfg *feederconfig.Config, dynReg *domain.DynamicSourceRegistry, httpClient *http.Client) ([]string, map[string]feeder.SourceConfig, []*declarative.CompiledSource, error) {
	declConfigs, compiledSources, err := RegisterDeclarativeSources(logger, registry, cfg, dynReg, httpClient)
	if err != nil {
		return nil, nil, nil, err
	}

	var enabledSources []string
	for name := range declConfigs {
		enabledSources = append(enabledSources, name)
	}

	return enabledSources, declConfigs, compiledSources, nil
}

// RegisterDeclarativeSources loads YAML source definitions from sources_dir,
// compiles their CEL expressions, creates DeclarativeAdapters, and registers
// them in the SourceRegistry. Returns source configs for the ingestion service,
// the compiled sources, and any error encountered during loading.
func RegisterDeclarativeSources(logger zerolog.Logger, registry *ingest.SourceRegistry, cfg *feederconfig.Config, dynReg *domain.DynamicSourceRegistry, httpClient *http.Client) (map[string]feeder.SourceConfig, []*declarative.CompiledSource, error) {
	sourcesDir := cfg.Ingest.SourcesDir
	if sourcesDir == "" {
		logger.Debug().Msg("no sources_dir configured, skipping declarative source loading")
		return nil, nil, nil
	}

	// Create CEL compiler
	compiler, err := declarative.NewCELCompiler()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CEL compiler: %w", err)
	}

	// Create loader — devMode allows http:// URLs for local development.
	declLogger := logging.NewLogger("declarative", nil)
	loader, err := declarative.NewLoader(compiler, config.EnvResolve, cfg.Ingest.DevMode, declLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create declarative loader: %w", err)
	}

	// Load all source definitions from the directory
	compiledSources, err := loader.LoadDir(sourcesDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load sources directory %q: %w", sourcesDir, err)
	}

	logger.Info().Int("count", len(compiledSources)).Str("dir", sourcesDir).Msg("loaded declarative source definitions")

	sourceConfigs := make(map[string]feeder.SourceConfig)

	for _, cs := range compiledSources {
		def := cs.Definition()

		// Create the declarative adapter, sharing the injected HTTP client (and its
		// rate-limiting transport) when provided so all sources share the budget.
		adapter, err := declarative.NewDeclarativeAdapterWithClient(cs, declLogger, httpClient)
		if err != nil {
			logger.Error().Err(err).Str("source", def.Name).Msg("failed to create declarative adapter")
			continue
		}

		// Register in the ingest source registry
		registry.Register(adapter)

		// Register display config and v2 metadata in the domain dynamic registry
		// so LayerType() lookups work and GetLayers gRPC response has display info.
		if err := declarative.RegisterSourceMetadata(
			declarative.SourceMetadataFromDefinition(def),
			dynReg,
		); err != nil {
			logger.Error().Err(err).Str("source", def.Name).Msg("failed to register source metadata")
			continue
		}

		// Build source config for the ingestion service ticker
		interval := def.Transport.Interval.Duration
		if interval <= 0 {
			interval = 60 * time.Second
		}
		timeout := def.Transport.Timeout.Duration
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		cacheTTL := def.Cache.TTL.Duration
		if cacheTTL <= 0 {
			cacheTTL = 60 * time.Second
		}

		recordMode := config.ObservationRecordMode(def.Recording.Mode)
		if !recordMode.Valid() {
			recordMode = config.RecordAppend
		}

		sourceConfigs[def.Name] = feeder.SourceConfig{
			Name:                  def.Name,
			Interval:              interval,
			Timeout:               timeout,
			APIURL:                def.Transport.URL,
			CacheTTL:              cacheTTL,
			ObservationRecordMode: recordMode,
			DryRun:                def.DryRun,
		}

		logger.Info().
			Str("source", def.Name).
			Str("source_type", def.SourceType).
			Str("layer_type", def.LayerType).
			Dur("interval", interval).
			Msg("declarative source registered")
	}

	return sourceConfigs, compiledSources, nil
}

// CollectSourceAIConfigs extracts AI configurations from compiled sources.
// Returns a map keyed by source name for O(1) lookup during ingestion.
func CollectSourceAIConfigs(compiledSources []*declarative.CompiledSource) map[string]*aiconfig.SourceAIConfig {
	configs := make(map[string]*aiconfig.SourceAIConfig, len(compiledSources))
	for _, cs := range compiledSources {
		def := cs.Definition()
		if def.AI != nil && def.AI.Enabled {
			configs[def.Name] = def.AI
		}
	}
	return configs
}
