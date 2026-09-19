package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/declarative"
	"github.com/Alevsk/respondent/internal/logging"
	"github.com/Alevsk/respondent/internal/ratelimit"
	"github.com/Alevsk/respondent/internal/realtime"
)

// configureHostRateLimits applies each source's declarative rate_limit to the
// shared transport, keyed by the host of its transport URL, so the feeder crawl
// and on-demand fills hitting that host share one request budget.
func configureHostRateLimits(rt *ratelimit.Transport, compiled []*declarative.CompiledSource, logger zerolog.Logger) {
	for _, cs := range compiled {
		def := cs.Definition()
		rl := def.Transport.RateLimit
		if rl == nil || rl.RequestsPerSecond <= 0 {
			continue
		}
		host := hostOf(def.Transport.URL)
		if host == "" {
			host = hostOf(def.Transport.OnDemandURL)
		}
		if host == "" {
			continue
		}
		burst := rl.Burst
		if burst < 1 {
			burst = 1
		}
		rt.SetHostLimit(host, rl.RequestsPerSecond, burst)
		logger.Info().Str("host", host).Float64("rps", rl.RequestsPerSecond).Int("burst", burst).
			Msg("per-host rate limit configured")
	}
}

func hostOf(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// onDemandPollInterval is how often the realtime server re-polls an external API
// to sustain data flow in a sparse viewport, once an on-demand fill has fired.
// A single default applies to every on-demand-capable layer (not layer-specific).
const onDemandPollInterval = 30 * time.Second

// onDemandQueryTimeout bounds a single on-demand cell fetch, INCLUDING the wait for
// a shared rate-limit token. A viewport fill tiles into many cells that drain at the
// host's rate (~1/s for adsb.lol), so a later cell may wait ~20s for its token; the
// timeout must exceed that drain or late cells fail with "would exceed deadline".
// It stays under the on-demand poll interval so successive ticker fills don't overlap.
const onDemandQueryTimeout = 30 * time.Second

// applyRealtimeConfig wires the realtime server's on-demand viewport fills and
// per-layer sparse-region thresholds from the declarative source definitions.
// The registry (populated from sources.d/*.yaml during LoadSources) is the single
// source of truth: any source declaring an on_demand_url participates in Tier 1.5
// live fills, and any source declaring a backfill threshold tunes its sparse
// trigger. Nothing here is layer-specific — it scales to every declarative source.
func applyRealtimeConfig(
	ws *realtime.Server,
	dynReg *domain.DynamicSourceRegistry,
	compiled []*declarative.CompiledSource,
	rt *ratelimit.Transport,
	logger zerolog.Logger,
) {
	odCfg := realtime.DefaultOnDemandConfig()
	odCfg.QueryTimeout = onDemandQueryTimeout
	// Share the feeder's per-host rate budget; on-demand requests are sent at high
	// priority (set per-request in the realtime server) so they preempt the crawl.
	odCfg.Transport = rt
	layerAPIs := make(map[string]string)
	pollIntervals := make(map[string]time.Duration)
	for lt, url := range dynReg.AllOnDemandURLs() {
		layerAPIs[string(lt)] = url
		pollIntervals[string(lt)] = onDemandPollInterval
	}
	// Per-layer fetch radius (from each source's spatial.radius_nm) so the server
	// tiles the viewport with a hex grid and fills the whole view, not just its
	// center. Keyed by layer type; any source declaring a radius participates.
	layerRadii := make(map[string]float64)
	for _, cs := range compiled {
		def := cs.Definition()
		if def.Transport.OnDemandURL == "" || def.Transport.Spatial == nil || def.Transport.Spatial.RadiusNM <= 0 {
			continue
		}
		layerRadii[def.LayerType] = def.Transport.Spatial.RadiusNM
	}
	if len(layerAPIs) > 0 {
		odCfg.Enabled = true
		odCfg.LayerAPIs = layerAPIs
		odCfg.PollIntervals = pollIntervals
		odCfg.LayerRadii = layerRadii
		if parser := buildOnDemandParser(compiled, logger); parser != nil {
			ws.SetOnDemandParser(parser)
		}
	}
	ws.SetOnDemandConfig(odCfg)

	// Per-layer sparse-region thresholds: when a viewport returns fewer entities
	// than the threshold, the server triggers on-demand + backfill for that layer.
	bfCfg := realtime.DefaultBackfillConfig()
	thresholds := make(map[string]int)
	for lt, t := range dynReg.AllBackfillThresholds() {
		thresholds[string(lt)] = t
	}
	if len(thresholds) > 0 {
		bfCfg.Thresholds = thresholds
	}
	ws.SetBackfillConfig(bfCfg)
}

// buildOnDemandParser indexes the declarative adapters of every source that
// declares an on_demand_url, keyed by layer type, and returns an
// OnDemandParserFunc that routes a raw API response to the owning source's
// declarative parser. Live viewport fills therefore reuse the exact
// parse → CEL/field-mapping pipeline the feeder uses for polling, so on-demand
// data is identical to crawled data. Fully declarative: any source with an
// on_demand_url participates automatically — nothing is layer-specific here.
//
// Returns nil when no source declares an on_demand_url (on-demand fills off).
func buildOnDemandParser(
	compiled []*declarative.CompiledSource,
	logger zerolog.Logger,
) realtime.OnDemandParserFunc {
	declLogger := logging.NewLogger("ondemand-parser", nil)
	byLayer := make(map[string]*declarative.DeclarativeAdapter)
	for _, cs := range compiled {
		def := cs.Definition()
		if def.Transport.OnDemandURL == "" {
			continue
		}
		adapter, err := declarative.NewDeclarativeAdapter(cs, declLogger)
		if err != nil {
			logger.Warn().Err(err).Str("source", def.Name).
				Msg("on-demand: skipping source with adapter build error")
			continue
		}
		byLayer[def.LayerType] = adapter
	}
	if len(byLayer) == 0 {
		return nil
	}
	return func(body []byte, layerType string) ([]*domain.Entity, []*domain.Observation, error) {
		adapter, ok := byLayer[layerType]
		if !ok {
			return nil, nil, fmt.Errorf("on-demand: no declarative parser registered for layer type %q", layerType)
		}
		return adapter.ParseOnDemand(body)
	}
}
