package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

const (
	defaultGlobalLimit = 5000  // Layers <= this get all entities in one shot
	maxGlobalLimit     = 10000 // Hard cap per page request
	defaultPageSize    = 1000  // Default page size for cursor pagination
)

// snapshotChunkSize is the maximum number of entities per snapshot chunk.
// Large snapshots are split into multiple messages for progressive rendering.
const snapshotChunkSize = 500

// handleSubscribe handles subscription requests
func (c *Client) handleSubscribe(msg WSMessage) {
	if msg.LayerID == "" {
		c.Logger.Warn().Msg("subscribe message missing layer_id")
		return
	}

	// Enforce per-client subscription limit to prevent resource exhaustion.
	c.mu.RLock()
	count := len(c.subscriptions)
	c.mu.RUnlock()
	if count >= maxSubscriptionsPerClient {
		c.Logger.Warn().Int("count", count).Int("max", maxSubscriptionsPerClient).Msg("subscription limit reached, ignoring")
		return
	}

	c.Subscribe(msg.LayerID) // Also resets time range state to live mode
	c.Logger.Info().Str("layer", msg.LayerID).Msg("client subscribed to layer")

	// Parse optional viewport, mode, and limit from subscribe data
	var globalMode bool
	var requestedLimit int
	if msg.Data != nil {
		dataBytes, err := json.Marshal(msg.Data)
		if err == nil {
			var subData struct {
				Viewport *domain.BBox `json:"viewport"`
				Mode     string       `json:"mode"`
				Limit    int          `json:"limit"`
			}
			if json.Unmarshal(dataBytes, &subData) == nil {
				if subData.Viewport != nil {
					c.SetViewport(msg.LayerID, subData.Viewport)
				}
				globalMode = subData.Mode == "global"
				requestedLimit = subData.Limit
			}
		}
	}
	c.SetGlobalMode(msg.LayerID, globalMode)

	// Send initial snapshot from cache, with database fallback
	if c.server == nil {
		return
	}

	// Translate layer ID to layer type via the registry.
	layerType := c.server.layerTypeForID(msg.LayerID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var entities []*domain.Entity
	var observations []*domain.Observation

	viewport := c.GetViewport(msg.LayerID)
	// Global mode bypasses spatial filtering entirely.
	effectiveViewport := viewport
	if globalMode {
		effectiveViewport = nil
	}

	// Determine entity limit
	limit := 2000 // default for non-global
	if globalMode {
		limit = defaultGlobalLimit
		if requestedLimit > 0 && requestedLimit <= maxGlobalLimit {
			limit = requestedLimit
		}
	}

	// Build the initial snapshot from the DURABLE store, which is the authoritative,
	// complete source (latest observation per entity). The hot cache holds only a
	// partial subset — whatever has been polled since startup — so trusting it as the
	// snapshot source under-reports: a static layer would render 5 of 506 entities, or
	// a viewport 25 of thousands. The cache drives live push updates via broadcast; it
	// is not the snapshot source of truth.
	threshold := c.server.getBackfillThreshold(string(layerType))
	// A viewport scopes the snapshot only for spatial layers; non-spatial layers (and
	// global mode) always get their full latest-per-entity set regardless of viewport.
	useViewport := effectiveViewport != nil && c.server.isSpatialLayer(string(layerType))

	if !useViewport {
		dbEntities, dbObs, err := c.server.snapshotFromDB(ctx, string(layerType), limit)
		if err != nil {
			c.Logger.Error().Err(err).Str("layer", msg.LayerID).Msg("failed to get snapshot from database")
		} else if len(dbEntities) > 0 {
			entities = dbEntities
			observations = dbObs
			c.Logger.Debug().Str("layer_id", msg.LayerID).Int("entities", len(entities)).Msg("snapshot from database (latest per entity)")
		}
		// Defensive: only if the durable store is unavailable/empty, try the cache.
		if len(entities) == 0 && c.server.cache != nil {
			cachedEntities, cachedObs, _, cErr := c.server.cache.GetLayerEntities(ctx, string(layerType), limit, 0)
			if cErr == nil && len(cachedEntities) > 0 {
				entities = cachedEntities
				observations = cachedObs
				c.Logger.Debug().Str("layer_id", msg.LayerID).Int("entities", len(entities)).Msg("snapshot from cache (db unavailable)")
			}
		}
	} else {
		// Spatial layer with a viewport: hot geo-cache first, then the durable store
		// when sparse. Shared with handleViewportUpdate so a fresh subscribe and a pan
		// to the same region return identical results.
		vpLimit := viewportEntityLimit(effectiveViewport, c.server.getBackfillConfig().MaxResults)
		entities, observations = c.server.buildSpatialViewportSnapshot(ctx, string(layerType), *effectiveViewport, vpLimit)
	}

	// Tag the snapshot entities as "live".
	for _, e := range entities {
		e.Source = "live"
	}
	for _, o := range observations {
		o.Source = "live"
	}

	// Tier 1.5: when even the durable store is sparse within this viewport, trigger
	// on-demand external fills (e.g. live flights) to populate the region.
	if useViewport && len(entities) < threshold {
		sparseEntities, sparseObs := c.server.fetchSparseRegion(ctx, c, msg.LayerID, string(layerType), effectiveViewport, entities)
		entities = sparseEntities
		observations = append(observations, sparseObs...)
	}

	if len(entities) > 0 {
		// For global mode, send paginated snapshot with total count + cursor
		if globalMode {
			c.server.SendPaginatedSnapshot(ctx, c, msg.LayerID, string(layerType), entities, observations, limit)
		} else {
			c.server.SendSnapshotToClient(c, msg.LayerID, entities, observations)
		}
	}
}

// buildSpatialViewportSnapshot returns the latest entities for a spatial layer
// within a viewport: the hot geo-cache first, then the durable store when the cache
// is sparse — BEFORE any rate-limited on-demand fetch. A small geo-cache result
// usually means the cache is not warmed for that region, not that the region is
// empty. Shared by handleSubscribe and handleViewportUpdate so a fresh subscribe
// and a pan to the same region load identically (the bug was that only subscribe
// consulted the durable store, so panning to an uncached region rendered nothing).
func (s *Server) buildSpatialViewportSnapshot(ctx context.Context, layerType string, bbox domain.BBox, limit int) ([]*domain.Entity, []*domain.Observation) {
	var entities []*domain.Entity
	var observations []*domain.Observation

	if s.cache != nil {
		if sc, ok := s.cache.(spatialCacheQuerier); ok {
			if e, o, err := sc.GetLayerEntitiesByBBox(ctx, layerType, bbox, limit); err == nil && len(e) > 0 {
				entities = e
				observations = o
			}
		}
	}

	if len(entities) < s.getBackfillThreshold(layerType) {
		if de, do, err := s.snapshotFromDBByBBox(ctx, layerType, bbox, limit); err == nil && len(de) > len(entities) {
			entities = de
			observations = do
		}
	}

	return entities, observations
}

// snapshotFromDB builds a complete layer snapshot — the latest observation per
// entity — from the durable store. It is the authoritative snapshot source for
// non-spatial (and global-mode) layers, where the hot cache holds only a partial
// subset. Returns (nil, nil, nil) when the repositories are not wired.
func (s *Server) snapshotFromDB(ctx context.Context, layerType string, limit int) ([]*domain.Entity, []*domain.Observation, error) {
	if s.obsRepo == nil || s.entityRepo == nil {
		return nil, nil, nil
	}
	dbObs, err := s.obsRepo.GetLatestForLayer(ctx, layerType, limit)
	if err != nil {
		return nil, nil, err
	}
	return s.resolveSnapshotEntities(ctx, dbObs)
}

// resolveSnapshotEntities batch-fetches the entities for a set of latest
// observations and rewrites both to the composite (layer_type:external_id) ID the
// frontend keys on. Observations whose entity is missing are dropped.
func (s *Server) resolveSnapshotEntities(ctx context.Context, dbObs []*domain.Observation) ([]*domain.Entity, []*domain.Observation, error) {
	if len(dbObs) == 0 {
		return nil, nil, nil
	}
	entityIDs := make([]string, 0, len(dbObs))
	for _, obs := range dbObs {
		entityIDs = append(entityIDs, obs.EntityID)
	}
	dbEntities, err := s.entityRepo.GetByIDs(ctx, entityIDs)
	if err != nil {
		return nil, nil, err
	}
	entityMap := make(map[string]*domain.Entity, len(dbEntities))
	for _, e := range dbEntities {
		entityMap[e.ID] = e
	}
	entities := make([]*domain.Entity, 0, len(dbObs))
	observations := make([]*domain.Observation, 0, len(dbObs))
	for _, obs := range dbObs {
		e, ok := entityMap[obs.EntityID]
		if !ok {
			continue
		}
		// Copy before rewriting IDs: the durable-store pointers may be reused by the
		// caller (or shared by test doubles), so this helper must not mutate its input.
		entityCopy := *e
		obsCopy := *obs
		compositeID := domain.EntityID(domain.LayerType(entityCopy.LayerType), entityCopy.ExternalID)
		entityCopy.ID = compositeID
		obsCopy.EntityID = compositeID
		entities = append(entities, &entityCopy)
		observations = append(observations, &obsCopy)
	}
	return entities, observations, nil
}

// snapshotFromDBByBBox builds a viewport-scoped snapshot from the durable store:
// each entity's truly-latest observation whose current position falls inside the
// bounding box. Used for spatial layers when the hot geo-cache is sparse, before
// resorting to rate-limited on-demand fetches.
func (s *Server) snapshotFromDBByBBox(ctx context.Context, layerType string, vp domain.BBox, limit int) ([]*domain.Entity, []*domain.Observation, error) {
	if s.obsRepo == nil || s.entityRepo == nil {
		return nil, nil, nil
	}
	// from = zero time (all history) → to = now: take each entity's truly-latest
	// observation whose current position is in the bbox.
	snaps, err := s.obsRepo.GetLatestByCurrentPositionInBBox(
		ctx, layerType, vp.South, vp.North, vp.West, vp.East, time.Time{}, time.Now(), limit)
	if err != nil {
		return nil, nil, err
	}
	entities := make([]*domain.Entity, 0, len(snaps))
	observations := make([]*domain.Observation, 0, len(snaps))
	for _, snap := range snaps {
		e := snap.Entity
		o := snap.Observation
		compositeID := domain.EntityID(domain.LayerType(e.LayerType), e.ExternalID)
		e.ID = compositeID
		o.EntityID = compositeID
		entities = append(entities, &e)
		observations = append(observations, &o)
	}
	return entities, observations, nil
}

// handlePage handles pagination requests for global-mode subscriptions.
func (c *Client) handlePage(msg WSMessage) {
	if msg.LayerID == "" {
		c.Logger.Warn().Msg("page message missing layer_id")
		return
	}
	if !c.IsSubscribed(msg.LayerID) {
		c.Logger.Warn().Str("layer", msg.LayerID).Msg("page request for unsubscribed layer")
		return
	}
	if !c.IsGlobalMode(msg.LayerID) {
		c.Logger.Warn().Str("layer", msg.LayerID).Msg("page request for non-global subscription")
		return
	}

	// Parse cursor and limit
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.Logger.Error().Err(err).Msg("failed to marshal page data")
		return
	}
	var pageData struct {
		Cursor string `json:"cursor"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(dataBytes, &pageData); err != nil {
		c.Logger.Error().Err(err).Msg("failed to parse page data")
		return
	}

	cursor, err := decodeCursor(pageData.Cursor)
	if err != nil {
		c.Logger.Error().Err(err).Msg("invalid cursor")
		return
	}

	limit := pageData.Limit
	if limit <= 0 || limit > maxGlobalLimit {
		limit = defaultPageSize
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch entities from cache at the cursor offset
	if c.server == nil || c.server.cache == nil {
		return
	}
	layerType := c.server.layerTypeForID(msg.LayerID)
	entities, obs, _, err := c.server.cache.GetLayerEntities(ctx, string(layerType), limit, cursor.Offset)
	if err != nil {
		c.Logger.Error().Err(err).Str("layer", msg.LayerID).Msg("page: cache query failed")
		return
	}

	// Tag as live
	for _, e := range entities {
		e.Source = "live"
	}
	for _, o := range obs {
		o.Source = "live"
	}

	// Build next cursor if there might be more
	var nextCursor string
	if len(entities) >= limit {
		nextCursor = encodeCursor(cursor.Offset + len(entities))
	}

	// Get total count using existing GetLayerCount (SCARD — O(1))
	var total int64
	total, _ = c.server.cache.GetLayerCount(ctx, string(layerType))

	// Build response
	chunk := map[string]any{
		"entities":     entities,
		"observations": obs,
		"total":        total,
	}
	if nextCursor != "" {
		chunk["cursor"] = nextCursor
	}

	wsMsg := WSMessage{
		Type:    "layer.snapshot",
		LayerID: msg.LayerID,
		Data:    chunk,
	}
	data, err := json.Marshal(wsMsg)
	if err != nil {
		return
	}
	select {
	case c.Send <- data:
		c.Logger.Debug().Str("layer", msg.LayerID).Int("entities", len(entities)).Int("offset", cursor.Offset).Msg("sent page")
	default:
		c.droppedCount.Add(1)
	}
}

// handleUnsubscribe handles unsubscription requests
func (c *Client) handleUnsubscribe(msg WSMessage) {
	if msg.LayerID == "" {
		c.Logger.Warn().Msg("unsubscribe message missing layer_id")
		return
	}
	c.stopOnDemandTicker(msg.LayerID)
	c.Unsubscribe(msg.LayerID)
	c.Logger.Info().Str("layer", msg.LayerID).Msg("client unsubscribed from layer")
}

// handleViewportUpdate handles viewport update messages for spatial layers.
// When a client sends a new viewport, we query the geo index and send matching entities.
func (c *Client) handleViewportUpdate(msg WSMessage) {
	if msg.LayerID == "" {
		c.Logger.Warn().Msg("viewport_update missing layer_id")
		return
	}

	// Parse bbox from data
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.Logger.Error().Err(err).Msg("failed to marshal viewport data")
		return
	}

	var bbox domain.BBox
	if err := json.Unmarshal(dataBytes, &bbox); err != nil {
		c.Logger.Error().Err(err).Msg("failed to parse viewport bbox")
		return
	}

	// Validate bbox ranges to reject malformed client input.
	if bbox.South < -90 || bbox.North > 90 || bbox.South > bbox.North ||
		bbox.West < -180 || bbox.East > 180 {
		c.Logger.Warn().
			Float64("south", bbox.South).Float64("north", bbox.North).
			Float64("west", bbox.West).Float64("east", bbox.East).
			Msg("viewport_update: invalid bbox, ignoring")
		return
	}

	c.SetViewport(msg.LayerID, &bbox)
	c.Logger.Debug().
		Str("layer", msg.LayerID).
		Float64("west", bbox.West).
		Float64("south", bbox.South).
		Float64("east", bbox.East).
		Float64("north", bbox.North).
		Msg("viewport updated")

	// If client has an active time range for this layer, handle as time range viewport change
	if c.IsInTimeRange(msg.LayerID) {
		c.mu.RLock()
		from := c.timeFrom[msg.LayerID]
		to := c.timeTo[msg.LayerID]
		frozen := c.timeFrozen[msg.LayerID]
		c.mu.RUnlock()
		c.doTimeRangeQuery(msg.LayerID, from, to, frozen, &bbox)
		return
	}

	// Build and send the snapshot for the new viewport. The shared helper falls back
	// to the durable store, so a nil cache is fine — do not gate on it.
	if c.server == nil {
		return
	}

	layerType := c.server.layerTypeForID(msg.LayerID)

	if !c.server.isSpatialLayer(string(layerType)) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := viewportEntityLimit(&bbox, c.server.getBackfillConfig().MaxResults)

	// Hot geo-cache, then the durable store when sparse — identical to a fresh
	// subscribe (shared helper) so panning to an uncached region loads its entities
	// instead of rendering nothing until the layer is toggled off and on.
	entities, observations := c.server.buildSpatialViewportSnapshot(ctx, string(layerType), bbox, limit)

	// Tag cache entities as "live"
	for _, e := range entities {
		e.Source = "live"
	}
	for _, o := range observations {
		o.Source = "live"
	}

	// Tier 1.5 + Tier 2: On-demand fetch and Postgres backfill (concurrent)
	// when cache is sparse. Also fires Tier 3 priority signal.
	threshold := c.server.getBackfillThreshold(string(layerType))
	if len(entities) < threshold {
		c.Logger.Debug().
			Int("geo_entities", len(entities)).
			Int("threshold", threshold).
			Msg("cache sparse: launching on-demand + backfill")

		sparseEntities, sparseObs := c.server.fetchSparseRegion(ctx, c, msg.LayerID, string(layerType), &bbox, entities)
		// fetchSparseRegion returns the merged result including the original cache entities
		entities = sparseEntities
		observations = append(observations, sparseObs...)

		// Start continuous polling for this sparse viewport
		c.startOnDemandTicker(msg.LayerID, string(layerType), &bbox)
	} else {
		c.Logger.Debug().
			Int("entities", len(entities)).
			Int("threshold", threshold).
			Msg("cache sufficient: skipping backfill")

		// Cache sufficient — stop any running ticker (user moved to a populated area)
		c.stopOnDemandTicker(msg.LayerID)
	}

	if len(entities) > 0 {
		c.server.SendSnapshotToClient(c, msg.LayerID, entities, observations)
		c.Logger.Debug().Str("layer", msg.LayerID).Int("entities", len(entities)).Msg("sent viewport snapshot")
	} else {
		c.Logger.Debug().Str("layer", msg.LayerID).Msg("viewport returned 0 entities")
	}
}

// SendSnapshotToClient sends a layer snapshot to a specific client.
// Large snapshots are automatically chunked into multiple messages of up to
// snapshotChunkSize entities each, allowing progressive rendering on the frontend.
func (s *Server) SendSnapshotToClient(client *Client, layerID string, entities []*domain.Entity, observations []*domain.Observation) {
	// For indicator layers, merge all observations and send as indicator.update.
	if len(entities) > 0 && isIndicatorLayer(s.dynReg, entities[0].LayerType) {
		merged := make(map[string]string)
		var latestObs *domain.Observation
		for _, obs := range observations {
			if obs == nil {
				continue
			}
			for k, v := range obs.Metadata {
				merged[k] = v
			}
			if latestObs == nil || obs.Timestamp.After(latestObs.Timestamp) {
				latestObs = obs
			}
		}
		if latestObs != nil {
			syntheticObs := *latestObs
			syntheticObs.Metadata = merged
			indicatorData := buildIndicatorUpdate(s.dynReg, entities[0].LayerType, &syntheticObs)
			if indicatorData != nil {
				msg := WSMessage{
					Type:    "indicator.update",
					LayerID: layerID,
					Data:    indicatorData,
				}
				data, err := json.Marshal(msg)
				if err == nil {
					select {
					case client.Send <- data:
					default:
						client.droppedCount.Add(1)
						s.logger.Warn().Str("client", client.ID).Msg("client send buffer full (indicator)")
					}
				}
				return
			}
		}
		// Fall through to standard snapshot if indicator build failed.
	}

	// Build an observation lookup by entity ID for efficient chunking.
	obsMap := make(map[string]*domain.Observation, len(observations))
	for _, o := range observations {
		if o != nil {
			obsMap[o.EntityID] = o
		}
	}

	// Send in chunks for progressive rendering.
	for i := 0; i < len(entities); i += snapshotChunkSize {
		end := i + snapshotChunkSize
		if end > len(entities) {
			end = len(entities)
		}
		chunkEntities := entities[i:end]
		chunkObs := make([]*domain.Observation, 0, len(chunkEntities))
		for _, e := range chunkEntities {
			if o, ok := obsMap[e.ID]; ok {
				chunkObs = append(chunkObs, o)
			}
		}

		msg := WSMessage{
			Type:    "layer.snapshot",
			LayerID: layerID,
			Data: map[string]any{
				"entities":     chunkEntities,
				"observations": chunkObs,
			},
		}
		data, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to marshal snapshot chunk for client")
			return
		}
		select {
		case client.Send <- data:
		default:
			client.droppedCount.Add(1)
			s.logger.Warn().Str("client", client.ID).Int("chunk", i/snapshotChunkSize).Msg("client send buffer full (snapshot chunk)")
			return // Stop sending further chunks if buffer is full
		}
	}
}

// SendPaginatedSnapshot sends a paginated layer snapshot with total count and cursor.
// If the dataset fits within `limit`, no cursor is sent (single-page result).
func (s *Server) SendPaginatedSnapshot(ctx context.Context, client *Client, layerID, layerType string, entities []*domain.Entity, observations []*domain.Observation, limit int) {
	// Get total entity count for this layer using existing GetLayerCount (SCARD — O(1))
	var total int64
	if s.cache != nil {
		var err error
		total, err = s.cache.GetLayerCount(ctx, layerType)
		if err != nil {
			s.logger.Warn().Err(err).Str("layer", layerID).Msg("failed to get entity count")
			total = int64(len(entities))
		}
	} else {
		total = int64(len(entities))
	}

	// Check for indicator layers — delegate to existing indicator path
	if len(entities) > 0 && isIndicatorLayer(s.dynReg, entities[0].LayerType) {
		s.SendSnapshotToClient(client, layerID, entities, observations)
		return
	}

	// Paginate: send up to `limit` entities
	pageEntities := entities
	if len(entities) > limit {
		pageEntities = entities[:limit]
	}

	// Build observation lookup
	obsMap := make(map[string]*domain.Observation, len(observations))
	for _, o := range observations {
		if o != nil {
			obsMap[o.EntityID] = o
		}
	}

	// Build cursor if there are more entities
	var cursor string
	if len(entities) > limit {
		cursor = encodeCursor(limit)
	}

	// Send in chunks for progressive rendering (same as existing chunking)
	// but the first chunk carries the total + cursor metadata.
	for i := 0; i < len(pageEntities); i += snapshotChunkSize {
		end := i + snapshotChunkSize
		if end > len(pageEntities) {
			end = len(pageEntities)
		}

		chunkEntities := pageEntities[i:end]
		var chunkObs []*domain.Observation
		for _, e := range chunkEntities {
			if obs, ok := obsMap[e.ID]; ok {
				chunkObs = append(chunkObs, obs)
			}
		}

		chunk := map[string]any{
			"entities":     chunkEntities,
			"observations": chunkObs,
		}
		// Only first chunk gets pagination metadata
		if i == 0 {
			chunk["total"] = total
			if cursor != "" {
				chunk["cursor"] = cursor
			}
		}

		msg := WSMessage{
			Type:    "layer.snapshot",
			LayerID: layerID,
			Data:    chunk,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to marshal paginated snapshot chunk")
			continue
		}
		select {
		case client.Send <- data:
		default:
			client.droppedCount.Add(1)
			s.logger.Warn().Str("client", client.ID).Msg("client send buffer full (paginated snapshot)")
			return
		}
	}
}

// viewportEntityLimit returns a dynamic max entity count based on viewport area.
// Large viewports (global/orbital) get fewer entities to prevent message flooding.
// Smaller viewports (regional/city) get higher limits for better coverage.
func viewportEntityLimit(bbox *domain.BBox, baseLimit int) int {
	if bbox == nil {
		return baseLimit
	}
	latSpan := bbox.North - bbox.South
	lonSpan := bbox.East - bbox.West
	if lonSpan < 0 {
		lonSpan += 360 // antimeridian wrapping
	}
	area := latSpan * lonSpan
	// Scale: <=500 deg² → full limit, 500-10000 → linear reduction, >10000 → 20% of limit
	switch {
	case area <= 500:
		return baseLimit
	case area <= 10000:
		// Linear scale from 100% to 20%
		frac := 1.0 - 0.8*(area-500)/9500
		limit := int(float64(baseLimit) * frac)
		if limit < 500 {
			return 500
		}
		return limit
	default:
		limit := baseLimit / 5
		if limit < 500 {
			return 500
		}
		return limit
	}
}
