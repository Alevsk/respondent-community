package declarative

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/grid"
	"github.com/Alevsk/respondent/internal/logging"
)

// spatialState holds per-adapter spatial crawl state.
// It is initialized on the first spatial fetch call and persists across cycles.
type spatialState struct {
	regions []grid.Region
	cursor  int
}

// entityCache provides TTL-based, per-entity caching with accumulation.
// Used by spatial adapters to merge entities from multiple region fetches.
type entityCache struct {
	mu      sync.Mutex
	entries map[string]*entityCacheEntry
	ttl     time.Duration
	clock   Clock
}

type entityCacheEntry struct {
	entity      *domain.Entity
	observation *domain.Observation
	lastSeen    time.Time
}

func newEntityCache(ttl time.Duration, clock Clock) *entityCache {
	if clock == nil {
		clock = realClock{}
	}
	return &entityCache{
		entries: make(map[string]*entityCacheEntry),
		ttl:     ttl,
		clock:   clock,
	}
}

// merge upserts an entity/observation pair into the cache by key.
func (c *entityCache) merge(key string, entity *domain.Entity, observation *domain.Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = &entityCacheEntry{
		entity:      entity,
		observation: observation,
		lastSeen:    c.clock.Now(),
	}
}

// snapshot returns all non-expired entities and observations, evicting stale entries.
func (c *entityCache) snapshot() ([]*domain.Entity, []*domain.Observation) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	var entities []*domain.Entity
	var observations []*domain.Observation

	for key, entry := range c.entries {
		if now.Sub(entry.lastSeen) > c.ttl {
			delete(c.entries, key)
			continue
		}
		entities = append(entities, entry.entity)
		observations = append(observations, entry.observation)
	}

	return entities, observations
}

// size returns the current number of cache entries.
func (c *entityCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// SubstituteURLTemplate replaces URL template variables with coordinate values.
// Supported variables: {lat}, {lon}, {lat_min}, {lat_max}, {lon_min}, {lon_max}.
// Bounding box values are computed from the grid cell center and the configured radius.
func SubstituteURLTemplate(urlTemplate string, lat, lon, radiusNM float64) string {
	// Compute approximate bounding box from center + radius
	radiusKm := radiusNM * 1.852
	latOffset := radiusKm / 111.32
	lonOffset := radiusKm / (111.32 * math.Cos(lat*math.Pi/180.0))
	if math.IsInf(lonOffset, 0) || math.IsNaN(lonOffset) {
		lonOffset = 360 // near poles, use full longitude range
	}

	result := urlTemplate
	result = strings.ReplaceAll(result, "{lat}", fmt.Sprintf("%.4f", lat))
	result = strings.ReplaceAll(result, "{lon}", fmt.Sprintf("%.4f", lon))
	result = strings.ReplaceAll(result, "{lat_min}", fmt.Sprintf("%.4f", lat-latOffset))
	result = strings.ReplaceAll(result, "{lat_max}", fmt.Sprintf("%.4f", lat+latOffset))
	result = strings.ReplaceAll(result, "{lon_min}", fmt.Sprintf("%.4f", lon-lonOffset))
	result = strings.ReplaceAll(result, "{lon_max}", fmt.Sprintf("%.4f", lon+lonOffset))
	return result
}

// fetchSpatial performs a spatial crawl cycle: select batch, fetch each region, merge into cache.
func (a *DeclarativeAdapter) fetchSpatial(ctx context.Context) error {
	cs := a.compiled.Load()
	if cs == nil {
		return fmt.Errorf("no compiled source loaded for %q", a.name)
	}

	def := cs.Definition()
	spatial := def.Transport.Spatial
	if spatial == nil {
		return fmt.Errorf("spatial spec not configured for %q", a.name)
	}

	// Initialize spatial state on first call
	a.initSpatialState(spatial, def.Transport.Interval.Duration)

	// Initialize entity cache on first call
	a.initEntityCache(def.EntityCache)

	// Select batch
	a.spatialMu.Lock()
	batch := a.selectSpatialBatch()
	a.spatialMu.Unlock()

	if len(batch) == 0 {
		return nil
	}

	method := def.Transport.Method

	radiusNM := spatial.RadiusNM
	if radiusNM <= 0 {
		radiusNM = 250
	}

	fetched := 0
	for _, region := range batch {
		if ctx.Err() != nil {
			break
		}

		url := SubstituteURLTemplate(def.Transport.URL, region.Lat, region.Lon, radiusNM)

		body, _, err := a.fetchURLWithRetry(ctx, cs, method, url)
		if err != nil {
			a.logger.Warn("failed to fetch spatial region",
				logging.String("source_name", a.name),
				logging.String("label", region.Label),
				logging.Err("error", err),
			)
			continue
		}

		records, err := a.parseBody(def, body)
		if err != nil {
			a.logger.Warn("failed to parse spatial region response",
				logging.String("source_name", a.name),
				logging.String("label", region.Label),
				logging.Err("error", err),
			)
			continue
		}

		entities, observations := a.processRecords(cs, records)

		// Merge into entity cache
		if a.ecache != nil {
			for i, entity := range entities {
				key := entity.ExternalID
				// If a cache key CEL program is configured, evaluate it against the record
				if prog := cs.EntityCacheKey(); prog != nil && i < len(records) {
					activation := map[string]interface{}{"record": records[i]}
					out, _, err := prog.Eval(activation)
					if err == nil {
						if s, ok := out.Value().(string); ok && s != "" {
							key = s
						}
					}
				}
				a.ecache.merge(key, entity, observations[i])
			}
		}

		fetched++
	}

	// Build snapshot from cache
	if a.ecache != nil {
		entities, observations := a.ecache.snapshot()
		a.SetEntities(entities, observations)

		a.logger.Info("processed spatial source",
			logging.String("source_name", a.name),
			logging.Int("entities_cached", len(entities)),
			logging.Int("regions_fetched", fetched),
			logging.Int("regions_batch", len(batch)),
		)
	}

	return nil
}

// initSpatialState initializes the grid/regions and cursor on first call.
func (a *DeclarativeAdapter) initSpatialState(spatial *SpatialSpec, interval time.Duration) {
	a.spatialMu.Lock()
	defer a.spatialMu.Unlock()

	if a.spatial != nil {
		return // already initialized
	}

	var regions []grid.Region

	switch spatial.Type {
	case "hex_grid":
		radiusNM := spatial.RadiusNM
		if radiusNM <= 0 {
			radiusNM = 250
		}
		allRegions := grid.GenerateGlobalGrid(radiusNM)

		// Filter by lat_min/lat_max if configured (skip polar regions with no data)
		if spatial.LatMin != nil || spatial.LatMax != nil {
			for _, r := range allRegions {
				if spatial.LatMin != nil && r.Lat < *spatial.LatMin {
					continue
				}
				if spatial.LatMax != nil && r.Lat > *spatial.LatMax {
					continue
				}
				regions = append(regions, r)
			}
		} else {
			regions = allRegions
		}
	case "static_regions":
		for _, sr := range spatial.Regions {
			regions = append(regions, grid.Region{
				Lat:   sr.Lat,
				Lon:   sr.Lon,
				Label: sr.Label,
			})
		}
	}

	// Calculate batch size if auto
	batchSize := spatial.BatchSize
	if batchSize <= 0 {
		targetRefresh := spatial.TargetRefresh.Duration
		if targetRefresh <= 0 {
			targetRefresh = 30 * time.Minute
		}
		if interval <= 0 {
			interval = 30 * time.Second
		}
		batchSize = grid.ComputeBatchSize(len(regions), interval, targetRefresh)
	}

	a.spatial = &spatialState{
		regions: regions,
		cursor:  0,
	}
	a.spatialBatchSize = batchSize

	a.logger.Info("initialized spatial state",
		logging.String("source_name", a.name),
		logging.Int("regions", len(regions)),
		logging.Int("batch_size", batchSize),
		logging.String("type", spatial.Type),
	)
}

// initEntityCache creates the entity cache on first call.
func (a *DeclarativeAdapter) initEntityCache(spec *EntityCacheSpec) {
	if a.ecache != nil {
		return // already initialized
	}
	if spec == nil || !spec.Enabled {
		// Create a default cache with 5-minute TTL for spatial mode
		a.ecache = newEntityCache(5*time.Minute, a.clock)
		return
	}
	a.ecache = newEntityCache(spec.TTL.Duration, a.clock)
}

// selectSpatialBatch returns the next batch of regions using round-robin.
// Must be called with spatialMu held.
func (a *DeclarativeAdapter) selectSpatialBatch() []grid.Region {
	if a.spatial == nil || len(a.spatial.regions) == 0 {
		return nil
	}

	n := len(a.spatial.regions)
	count := min(a.spatialBatchSize, n)
	batch := make([]grid.Region, 0, count)

	// Round-robin selection starting from the current cursor.
	for i := 0; i < count; i++ {
		idx := (a.spatial.cursor + i) % n
		batch = append(batch, a.spatial.regions[idx])
	}
	a.spatial.cursor = (a.spatial.cursor + count) % n

	return batch
}

// fetchURLWithRetry performs an HTTP fetch to a specific URL with retry logic.
// Delegates to the adapter's Transport for protocol-agnostic fetching.
func (a *DeclarativeAdapter) fetchURLWithRetry(ctx context.Context, cs *CompiledSource, method, url string) ([]byte, int, error) {
	return a.transport.Fetch(ctx, method, url)
}

// parseBody parses raw HTTP response body into records using the adapter's parser.
func (a *DeclarativeAdapter) parseBody(def *SourceDefinition, body []byte) ([]map[string]interface{}, error) {
	parserCfg := a.buildParserConfig(def)
	return a.parser.Parse(body, parserCfg)
}

// EntityCacheKey returns the compiled entity cache key CEL program, or nil if not defined.
func (cs *CompiledSource) EntityCacheKey() cel.Program {
	return cs.entityCacheKey
}
