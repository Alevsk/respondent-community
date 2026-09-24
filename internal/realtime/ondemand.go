package realtime

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/grid"
	"github.com/Alevsk/respondent/internal/ratelimit"
)

const (
	// onDemandFetchConcurrency bounds how many viewport grid cells are fetched
	// from an external API at once, keeping upstream load (and rate-limit risk)
	// in check while still filling the viewport quickly.
	onDemandFetchConcurrency = 6
	// maxOnDemandGridPoints caps the number of grid cells fetched for a single
	// viewport fill. Kept so that, at a rate-limited host (~1/s), the cells drain
	// within the on-demand request timeout and before the next poll tick — a very
	// wide view is partially filled on-demand and the global crawl covers the rest.
	maxOnDemandGridPoints = 20
)

// startOnDemandTicker starts a recurring on-demand fetch for a sparse viewport.
// If a ticker is already running for this layer, it is replaced (viewport moved).
func (c *Client) startOnDemandTicker(layerID, layerType string, bbox *domain.BBox) {
	c.stopOnDemandTicker(layerID) // cancel any existing ticker

	interval := c.server.getOnDemandPollInterval(layerType)
	if interval <= 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.onDemandTickers[layerID] = cancel
	c.mu.Unlock()

	go c.runOnDemandPoll(ctx, layerID, layerType, bbox, interval)
}

// stopOnDemandTicker cancels the on-demand polling ticker for a layer.
func (c *Client) stopOnDemandTicker(layerID string) {
	c.mu.Lock()
	if cancel, ok := c.onDemandTickers[layerID]; ok {
		cancel()
		delete(c.onDemandTickers, layerID)
	}
	c.mu.Unlock()
}

// stopAllOnDemandTickers cancels all tickers (called on disconnect).
func (c *Client) stopAllOnDemandTickers() {
	c.mu.Lock()
	for layerID, cancel := range c.onDemandTickers {
		cancel()
		delete(c.onDemandTickers, layerID)
	}
	c.mu.Unlock()
}

// runOnDemandPoll runs the recurring on-demand fetch loop.
// The ticker interval IS the rate limiter — no cooldown check needed.
func (c *Client) runOnDemandPoll(ctx context.Context, layerID, layerType string, bbox *domain.BBox, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.Logger.Info().
		Str("layer", layerID).
		Dur("interval", interval).
		Msg("on-demand ticker started")

	for {
		select {
		case <-ctx.Done():
			c.Logger.Debug().Str("layer", layerID).Msg("on-demand ticker stopped")
			return
		case <-ticker.C:
			if c.server == nil {
				return
			}

			entities, obs := c.server.doOnDemandFetchDirect(ctx, layerID, layerType, bbox)
			if len(entities) > 0 {
				c.Logger.Debug().Str("layer", layerID).Int("entities", len(entities)).Msg("on-demand poll delivered")
				c.server.SendSnapshotToClient(c, layerID, entities, obs)
			}
		}
	}
}

// fetchResult holds the output of a concurrent fetch goroutine.
type fetchResult struct {
	entities     []*domain.Entity
	observations []*domain.Observation
}

// fetchSparseRegion runs on-demand fetch (Tier 1.5) and durable-store backfill (Tier 2)
// concurrently, then merges results with live > stale priority.
// Called when the durable store returns fewer entities than the backfill threshold.
func (s *Server) fetchSparseRegion(
	ctx context.Context,
	c *Client,
	layerID, layerType string,
	bbox *domain.BBox,
	cacheEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	var wg sync.WaitGroup
	odCh := make(chan fetchResult, 1)
	bfCh := make(chan fetchResult, 1)

	// Goroutine A: On-demand external API fetch (Tier 1.5)
	wg.Add(1)
	go func() {
		defer wg.Done()
		e, o := s.doOnDemandFetch(ctx, c, layerID, layerType, bbox, cacheEntities)
		odCh <- fetchResult{e, o}
	}()

	// Goroutine B: durable-store backfill (Tier 2)
	bfCfg := s.getBackfillConfig()
	wg.Add(1)
	go func() {
		defer wg.Done()
		cooldown := bfCfg.Cooldown
		if len(cacheEntities) == 0 {
			cooldown = time.Second
		}
		if bfCfg.Enabled && s.obsRepo != nil && c.canBackfill(layerID, cooldown) {
			e, o := s.doBackfill(ctx, layerID, layerType, bbox, cacheEntities)
			bfCh <- fetchResult{e, o}
		} else {
			bfCh <- fetchResult{}
		}
	}()

	wg.Wait()
	close(odCh)
	close(bfCh)

	odResult := <-odCh
	bfResult := <-bfCh

	// On-demand entities first (source: "live", full opacity)
	clonedCacheEntities := make([]*domain.Entity, len(cacheEntities))
	copy(clonedCacheEntities, cacheEntities)
	entities := clonedCacheEntities
	observations := make([]*domain.Observation, 0, len(cacheEntities))

	entities = append(entities, odResult.entities...)
	observations = append(observations, odResult.observations...)

	// Backfill entities fill remaining gaps (source: "stale")
	// Deduplicate against cache + on-demand
	existingIDs := make(map[string]bool, len(entities))
	for _, e := range entities {
		existingIDs[e.ExternalID] = true
	}
	for i, e := range bfResult.entities {
		if !existingIDs[e.ExternalID] {
			entities = append(entities, e)
			observations = append(observations, bfResult.observations[i])
		}
	}

	// Tier 3: Priority signal for sustained feeder coverage (Redis-based
	// priority crawling is not available in the zero-dependency community
	// edition; viewport on-demand fetching above covers immediate coverage).

	return entities, observations
}

// doBackfill queries the durable store for entities within the viewport.
// Returns entities and observations tagged with source "stale".
func (s *Server) doBackfill(
	ctx context.Context,
	layerID, layerType string,
	bbox *domain.BBox,
	existingEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	bfCfg := s.getBackfillConfig()
	now := time.Now()
	from := now.Add(-bfCfg.StalenessWindow)

	snapshots, err := s.obsRepo.GetLatestForLayerByBBox(
		ctx, layerType,
		bbox.South, bbox.North, bbox.West, bbox.East,
		from, now,
		bfCfg.MaxResults,
	)
	if err != nil {
		s.logger.Error().Err(err).Str("layer", layerID).Msg("backfill query failed")
		return nil, nil
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	// Build set of existing entity external IDs to avoid duplicates
	existingIDs := make(map[string]bool, len(existingEntities))
	for _, e := range existingEntities {
		existingIDs[e.ExternalID] = true
	}

	var entities []*domain.Entity
	var observations []*domain.Observation
	for _, snap := range snapshots {
		if existingIDs[snap.Entity.ExternalID] {
			continue // Already have this entity from cache
		}
		e := snap.Entity
		o := snap.Observation
		e.Source = "stale"
		o.Source = "stale"
		// Normalize DB UUID → composite ID at the transport boundary
		// so frontend ParseEntityID (layerType:externalID) works correctly.
		compositeID := domain.EntityID(domain.LayerType(e.LayerType), e.ExternalID)
		e.ID = compositeID
		o.EntityID = compositeID
		entities = append(entities, &e)
		observations = append(observations, &o)
	}

	if len(entities) > 0 {
		s.logger.Info().
			Str("layer", layerID).
			Int("backfill_count", len(entities)).
			Msg("viewport backfill completed")
	}

	return entities, observations
}

// getBackfillThreshold returns the minimum entity count for a layer before triggering backfill.
func (s *Server) getBackfillThreshold(layerType string) int {
	bfCfg := s.getBackfillConfig()
	if t, ok := bfCfg.Thresholds[layerType]; ok {
		return t
	}
	return 10 // default
}

// doOnDemandFetch fetches live entity data directly from the external API
// for the viewport center, guarded by the per-client cooldown.
// Delegates to doOnDemandFetchCore for the actual HTTP fetch + cache pipeline.
func (s *Server) doOnDemandFetch(
	ctx context.Context,
	c *Client,
	layerID, layerType string,
	bbox *domain.BBox,
	existingEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	odCfg := s.getOnDemandConfig()
	if !odCfg.Enabled {
		s.logger.Debug().Str("layer", layerID).Msg("on-demand: disabled")
		return nil, nil
	}

	if _, ok := odCfg.LayerAPIs[layerType]; !ok {
		s.logger.Debug().Str("layer", layerID).Str("layer_type", layerType).Msg("on-demand: no API URL configured for layer type")
		return nil, nil
	}

	if !c.canOnDemand(layerID, odCfg.Cooldown) {
		s.logger.Debug().Str("layer", layerID).Msg("on-demand: cooldown active")
		return nil, nil
	}

	return s.doOnDemandFetchCore(ctx, layerID, layerType, bbox, existingEntities)
}

// doOnDemandFetchDirect fetches live data without cooldown checks.
// Used by the continuous on-demand ticker where the ticker interval
// acts as the rate limiter instead of the per-client cooldown.
func (s *Server) doOnDemandFetchDirect(
	ctx context.Context,
	layerID, layerType string,
	bbox *domain.BBox,
) ([]*domain.Entity, []*domain.Observation) {
	odCfg := s.getOnDemandConfig()
	if !odCfg.Enabled {
		return nil, nil
	}

	if s.obsRepo == nil {
		return nil, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, odCfg.QueryTimeout)
	defer cancel()

	// What is already stored for this viewport, so the fetch can skip it. This
	// is the same query doBackfill makes; it used to come from the hot cache,
	// which returned an arbitrary subset and so let already-known entities be
	// re-fetched from the upstream.
	bfCfg := s.getBackfillConfig()
	now := time.Now()
	snaps, err := s.obsRepo.GetLatestForLayerByBBox(
		fetchCtx, layerType,
		bbox.South, bbox.North, bbox.West, bbox.East,
		time.Time{}, now,
		bfCfg.MaxResults,
	)
	if err != nil {
		s.logger.Error().Err(err).Str("layer", layerID).Msg("on-demand: dedup query failed")
		return nil, nil
	}
	existingEntities := make([]*domain.Entity, 0, len(snaps))
	for _, snap := range snaps {
		e := snap.Entity
		existingEntities = append(existingEntities, &e)
	}
	return s.doOnDemandFetchCore(ctx, layerID, layerType, bbox, existingEntities)
}

// doOnDemandFetchCore contains the shared fetch logic used by both
// doOnDemandFetch (with cooldown) and doOnDemandFetchDirect (without cooldown).
// Performs HTTP fetch → parse → dedup → persist → Pub/Sub.
func (s *Server) doOnDemandFetchCore(
	ctx context.Context,
	layerID, layerType string,
	bbox *domain.BBox,
	existingEntities []*domain.Entity,
) ([]*domain.Entity, []*domain.Observation) {
	odCfg := s.getOnDemandConfig()
	urlTemplate, ok := odCfg.LayerAPIs[layerType]
	if !ok {
		return nil, nil
	}
	parser := s.onDemandParser
	if parser == nil {
		s.logger.Warn().Str("layer", layerID).Msg("on-demand: no parser configured, skipping")
		return nil, nil
	}

	// Cover the whole viewport, not just its center: when the layer declares a
	// fetch radius, tile the bbox with a hex grid (aligned to the global crawl)
	// and fetch every cell concurrently. Otherwise fall back to a single point.
	points := s.onDemandPoints(layerType, bbox, odCfg)

	results := make(chan fetchResult, len(points))
	sem := make(chan struct{}, onDemandFetchConcurrency)
	var wg sync.WaitGroup
	for _, p := range points {
		wg.Add(1)
		go func(lat, lon float64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			e, o := s.fetchOnDemandPoint(ctx, layerID, layerType, urlTemplate, lat, lon, odCfg.QueryTimeout, parser)
			results <- fetchResult{e, o}
		}(p.Lat, p.Lon)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	// Merge all cells, deduplicating against the cache and across overlapping cells.
	existingIDs := make(map[string]bool, len(existingEntities))
	for _, e := range existingEntities {
		existingIDs[e.ExternalID] = true
	}
	seen := make(map[string]bool)
	newEntities := make([]*domain.Entity, 0)
	newObservations := make([]*domain.Observation, 0)
	for r := range results {
		for i, e := range r.entities {
			if e == nil || i >= len(r.observations) {
				continue
			}
			if existingIDs[e.ExternalID] || seen[e.ExternalID] {
				continue
			}
			seen[e.ExternalID] = true
			e.Source = "live"
			r.observations[i].Source = "live"
			compositeID := domain.EntityID(domain.LayerType(e.LayerType), e.ExternalID)
			e.ID = compositeID
			r.observations[i].EntityID = compositeID

			newEntities = append(newEntities, e)
			newObservations = append(newObservations, r.observations[i])
		}
	}

	if len(newEntities) == 0 {
		return nil, nil
	}

	// Persist before returning. This used to run in a goroutine behind a
	// semaphore whose full branch logged and DROPPED the write, so entities
	// fetched on demand could be handed to the client and never stored — they
	// existed only in a hot cache that has since been removed. The insert is
	// single-digit milliseconds and already sits behind an upstream HTTP fetch
	// of hundreds, so doing it inline costs nothing measurable.
	s.persistOnDemand(newEntities, newObservations)

	s.logger.Info().
		Str("layer", layerID).
		Int("on_demand_count", len(newEntities)).
		Int("grid_points", len(points)).
		Msg("on-demand fetch completed")

	return newEntities, newObservations
}

// onDemandPoints returns the external-API fetch points covering the viewport. When
// the layer declares a fetch radius, the bbox is tiled with a hex grid (aligned to
// the global crawl, capped at maxOnDemandGridPoints) so the whole visible region
// is filled; otherwise a single center point is used.
func (s *Server) onDemandPoints(layerType string, bbox *domain.BBox, odCfg OnDemandConfig) []grid.Region {
	if radius := odCfg.LayerRadii[layerType]; radius > 0 {
		if pts := grid.GenerateGridForBBox(bbox.West, bbox.South, bbox.East, bbox.North, radius, maxOnDemandGridPoints); len(pts) > 0 {
			return pts
		}
	}
	lat, lon := bboxCenter(bbox)
	return []grid.Region{{Lat: lat, Lon: lon}}
}

// fetchOnDemandPoint performs a single external-API fetch + parse for one center
// point. It returns the parsed (raw, not yet deduped/tagged) entities and
// observations, or nil on any error (logged) so one failing cell never aborts the
// whole viewport fill.
func (s *Server) fetchOnDemandPoint(
	ctx context.Context,
	layerID, layerType, urlTemplate string,
	lat, lon float64,
	timeout time.Duration,
	parser OnDemandParserFunc,
) ([]*domain.Entity, []*domain.Observation) {
	url := strings.ReplaceAll(urlTemplate, "{lat}", fmt.Sprintf("%.1f", lat))
	url = strings.ReplaceAll(url, "{lon}", fmt.Sprintf("%.1f", lon))

	// High priority: on-demand fills are user-driven and preempt the background
	// crawl when both contend for the same host's shared rate budget.
	fetchCtx, cancel := context.WithTimeout(ratelimit.WithPriority(ctx), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		s.logger.Warn().Err(err).Str("layer", layerID).Msg("on-demand: failed to create request")
		return nil, nil
	}

	resp, err := s.getHTTPClient().Do(req)
	if err != nil {
		s.logger.Warn().Err(err).Str("layer", layerID).Msg("on-demand: fetch failed")
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn().Int("status", resp.StatusCode).Str("layer", layerID).Msg("on-demand: non-200 response")
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxOnDemandResponseSize))
	if err != nil {
		s.logger.Warn().Err(err).Str("layer", layerID).Msg("on-demand: failed to read body")
		return nil, nil
	}

	entities, observations, err := parser(body, layerType)
	if err != nil {
		s.logger.Warn().Err(err).Str("layer", layerID).Msg("on-demand: parse failed")
		return nil, nil
	}
	return entities, observations
}

// persistOnDemand writes on-demand fetched entities and observations to the
// durable store before the viewport response is returned.
func (s *Server) persistOnDemand(entities []*domain.Entity, observations []*domain.Observation) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.entityRepo != nil {
		if err := s.entityRepo.CreateBatch(ctx, entities); err != nil {
			s.logger.Warn().Err(err).Int("count", len(entities)).Msg("on-demand: entity persist failed")
		}
	}
	if s.obsRepo != nil && s.entityRepo != nil {
		// Resolve DB UUIDs for each entity so observation FK references are valid.
		// CreateBatch generates UUIDs internally; we need to look them up.
		uuidMap := make(map[string]string, len(entities))
		for _, e := range entities {
			dbEntity, err := s.entityRepo.GetByExternalID(ctx, e.LayerType, e.ExternalID)
			if err != nil {
				s.logger.Warn().Err(err).Str("external_id", e.ExternalID).Msg("on-demand: entity UUID lookup failed")
				continue
			}
			uuidMap[e.ID] = dbEntity.ID
		}

		// Rewrite observation EntityID from composite ID to DB UUID
		validObs := make([]*domain.Observation, 0, len(observations))
		for _, obs := range observations {
			if dbUUID, ok := uuidMap[obs.EntityID]; ok {
				obs.EntityID = dbUUID
				validObs = append(validObs, obs)
			}
		}

		if len(validObs) > 0 {
			if err := s.obsRepo.CreateBatchUpsert(ctx, validObs); err != nil {
				s.logger.Warn().Err(err).Int("count", len(validObs)).Msg("on-demand: observation persist failed")
			}
		}
	}
}

// bboxCenter computes the center point of a bounding box, handling antimeridian wrapping.
func bboxCenter(bbox *domain.BBox) (lat, lon float64) {
	lat = (bbox.South + bbox.North) / 2
	if bbox.West <= bbox.East {
		lon = (bbox.West + bbox.East) / 2
	} else {
		// Antimeridian: west > east (e.g., 170 to -170)
		lon = math.Mod((bbox.West+bbox.East+360)/2, 360) - 180
	}
	return lat, lon
}
