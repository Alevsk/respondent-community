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

	// Send the initial snapshot from the durable store
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

	// Build the initial snapshot from the durable store — the authoritative,
	// complete source (latest observation per entity). Live push updates are
	// published by the feeder straight to the broadcaster and never travel
	// through a snapshot read.
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
	} else {
		// Spatial layer with a viewport, read from the durable store. Shared with
		// handleViewportUpdate so a fresh subscribe and a pan to the same region
		// return identical results.
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
// within a viewport. Shared by handleSubscribe and handleViewportUpdate so a
// fresh subscribe and a pan to the same region load identically.
//
// It reads the durable store. It used to consult a hot geo-cache first and keep
// that result whenever it was non-empty, falling through only when the cache
// returned fewer rows than the backfill threshold (default 10) — so a cache
// holding 10 of a viewport's 2,000 entities suppressed the store entirely. The
// cache also iterated a Go map and stopped at the limit, so two identical
// viewport requests returned two arbitrary, sometimes disjoint, sets of
// entities. The store is deterministic and complete.
func (s *Server) buildSpatialViewportSnapshot(ctx context.Context, layerType string, bbox domain.BBox, limit int) ([]*domain.Entity, []*domain.Observation) {
	entities, observations, err := s.snapshotFromDBByBBox(ctx, layerType, bbox, limit)
	if err != nil {
		s.logger.Error().Err(err).Str("layer", layerType).Msg("viewport snapshot from database failed")
		return nil, nil
	}
	return entities, observations
}

// snapshotFromDB builds a layer snapshot — each entity with its latest
// observation — from the durable store, for non-spatial and global-mode layers.
// Returns (nil, nil, nil) when the repositories are not wired.
func (s *Server) snapshotFromDB(ctx context.Context, layerType string, limit int) ([]*domain.Entity, []*domain.Observation, error) {
	if s.obsRepo == nil {
		return nil, nil, nil
	}
	snaps, err := s.obsRepo.GetLatestForLayerPage(ctx, layerType, limit, 0)
	if err != nil {
		return nil, nil, err
	}
	entities, observations := snapshotsToLive(snaps)
	return entities, observations, nil
}

// snapshotPage returns one index-ordered page of a layer from the durable store.
func (s *Server) snapshotPage(ctx context.Context, layerType string, limit, offset int) ([]*domain.Entity, []*domain.Observation, error) {
	if s.obsRepo == nil {
		return nil, nil, nil
	}
	snaps, err := s.obsRepo.GetLatestForLayerPage(ctx, layerType, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	entities, observations := snapshotsToLive(snaps)
	return entities, observations, nil
}

// layerTotal reports how many entities a layer can page out, for the "total
// available" badge and the cursor. It counts the same population the page
// draws from — entities that still have an observation — rather than every row
// in the entities table, which would include entities retention has pruned the
// observations from and so keep the cursor advancing past the end.
func (s *Server) layerTotal(ctx context.Context, layerType string, pageLen int) int64 {
	if s.obsRepo == nil {
		return int64(pageLen)
	}
	total, err := s.obsRepo.CountLatestForLayer(ctx, layerType)
	if err != nil {
		s.logger.Warn().Err(err).Str("layer", layerType).Msg("failed to count layer entities")
		return int64(pageLen)
	}
	return total
}

// snapshotsToLive rewrites each snapshot to the composite (layer_type:external_id)
// ID the frontend keys on, copying first so the durable store's rows are never
// mutated in place.
func snapshotsToLive(snaps []*domain.EntitySnapshot) ([]*domain.Entity, []*domain.Observation) {
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
	return entities, observations
}

// snapshotFromDBByBBox builds a viewport-scoped snapshot from the durable store:
// each entity's truly-latest observation whose current position falls inside the
// bounding box. Used for spatial layers before resorting to rate-limited
// on-demand fetches.
func (s *Server) snapshotFromDBByBBox(ctx context.Context, layerType string, vp domain.BBox, limit int) ([]*domain.Entity, []*domain.Observation, error) {
	if s.obsRepo == nil {
		return nil, nil, nil
	}
	// from = zero time (all history) → to = now: take each entity's truly-latest
	// observation whose current position is in the bbox.
	snaps, err := s.obsRepo.GetLatestByCurrentPositionInBBox(
		ctx, layerType, vp.South, vp.North, vp.West, vp.East, time.Time{}, time.Now(), limit)
	if err != nil {
		return nil, nil, err
	}
	entities, observations := snapshotsToLive(snaps)
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

	// Page the durable store at the cursor offset. This used to page the hot
	// cache by ranging a Go map and counting to the offset — with randomised
	// iteration order, so the same request returned different rows each time and
	// pages both overlapped and skipped. The store's page is index-ordered.
	if c.server == nil {
		return
	}
	layerType := c.server.layerTypeForID(msg.LayerID)
	entities, obs, err := c.server.snapshotPage(ctx, string(layerType), limit, cursor.Offset)
	if err != nil {
		c.Logger.Error().Err(err).Str("layer", msg.LayerID).Msg("page: database query failed")
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

	total := c.server.layerTotal(ctx, string(layerType), len(entities))

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

	// Tier 1.5 + Tier 2: on-demand fetch and durable-store backfill (concurrent)
	// when cache is sparse. Also fires Tier 3 priority signal.
	threshold := c.server.getBackfillThreshold(string(layerType))
	if len(entities) < threshold {
		c.Logger.Debug().
			Int("geo_entities", len(entities)).
			Int("threshold", threshold).
			Msg("viewport sparse: launching on-demand + backfill")

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
			Msg("viewport sufficient: skipping backfill")

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
	total := s.layerTotal(ctx, layerType, len(entities))

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

	// Build a cursor if the layer holds more than this page. The comparison is
	// against the layer total, not against len(entities): the store already
	// applied the limit, so the page length can never exceed it and comparing
	// the two would report "no more" for every layer.
	var cursor string
	if int64(len(pageEntities)) < total {
		cursor = encodeCursor(len(pageEntities))
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
