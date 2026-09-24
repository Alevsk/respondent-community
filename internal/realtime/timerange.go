package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// handleTimeRange handles time_range requests from clients.
func (c *Client) handleTimeRange(msg WSMessage) {
	if msg.LayerID == "" {
		c.Logger.Warn().Msg("time_range message missing layer_id")
		return
	}

	if c.server == nil {
		c.Logger.Warn().Msg("time_range: no server reference")
		return
	}
	trCfg := c.server.getTimeRangeConfig()
	if !trCfg.Enabled {
		c.Logger.Warn().Msg("time_range not enabled")
		return
	}

	// Parse time range data early so we can check mode before cooldown.
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		c.Logger.Error().Err(err).Msg("failed to marshal time_range data")
		return
	}

	var rangeData struct {
		From     string       `json:"from"`
		To       string       `json:"to"`
		Viewport *domain.BBox `json:"viewport"`
		Mode     string       `json:"mode"`
	}
	if err := json.Unmarshal(dataBytes, &rangeData); err != nil {
		c.Logger.Error().Err(err).Msg("failed to parse time_range data")
		return
	}

	globalMode := rangeData.Mode == "global"
	c.SetGlobalMode(msg.LayerID, globalMode)

	// Rate-limit time_range queries to prevent DB spam during rapid viewport
	// changes. Global-mode requests (dashboard time-preset switches) are exempt
	// because they are explicit user actions, not automated viewport updates.
	if !globalMode {
		cooldown := trCfg.Cooldown
		if cooldown > 0 && !c.canTimeRange(msg.LayerID, cooldown) {
			c.Logger.Debug().Str("layer", msg.LayerID).Dur("cooldown", cooldown).Msg("time_range: cooldown active, skipping")
			return
		}
	}

	from, err := time.Parse(time.RFC3339, rangeData.From)
	if err != nil {
		c.Logger.Error().Err(err).Str("from", rangeData.From).Msg("invalid time_range from timestamp")
		return
	}

	to, err := time.Parse(time.RFC3339, rangeData.To)
	if err != nil {
		c.Logger.Error().Err(err).Str("to", rangeData.To).Msg("invalid time_range to timestamp")
		return
	}

	now := time.Now()

	// Validation
	if !from.Before(to) {
		c.Logger.Warn().Msg("time_range: from must be before to")
		return
	}

	// Per-layer history limits are resolved but only applied when non-zero.
	// A zero (or very large) config value means "no limit" — the user can
	// query any range they want.
	maxLookback := trCfg.MaxLookback
	maxSpan := trCfg.MaxRangeSpan

	layerType := c.server.layerTypeForID(msg.LayerID)
	if hc, ok := c.server.dynReg.LookupHistoryConfig(layerType); ok {
		if hc.MaxLookbackHours > 0 {
			maxLookback = time.Duration(hc.MaxLookbackHours) * time.Hour
		}
		if hc.MaxRangeSpanHours > 0 {
			maxSpan = time.Duration(hc.MaxRangeSpanHours) * time.Hour
		}
	}

	// Clamp from to max lookback (skip when limit is zero / unlimited)
	if maxLookback > 0 {
		maxFrom := now.Add(-maxLookback)
		if from.Before(maxFrom) {
			from = maxFrom
		}
	}

	// Clamp range span (skip when limit is zero / unlimited)
	if maxSpan > 0 && to.Sub(from) > maxSpan {
		from = to.Add(-maxSpan)
	}

	// Determine if frozen (to is in the past by > 10m).
	// The threshold is generous to tolerate clock skew between the client
	// and the server — anything under 10 minutes is treated as "live".
	frozen := now.Sub(to) > 10*time.Minute

	// Store time range state on client
	c.SetTimeRange(msg.LayerID, from, to, frozen)

	// For live (non-frozen) ranges, enable Pub/Sub subscription so real-time
	// updates flow to this client after the initial snapshot is delivered.
	if !frozen {
		c.EnableSubscription(msg.LayerID)
	}

	// Use viewport from the message, or fall back to stored viewport
	viewport := rangeData.Viewport
	if viewport == nil {
		viewport = c.GetViewport(msg.LayerID)
	} else {
		c.SetViewport(msg.LayerID, viewport)
	}

	// Global mode bypasses viewport filtering
	if globalMode {
		viewport = nil
	}

	c.doTimeRangeQuery(msg.LayerID, from, to, frozen, viewport)
}

// doTimeRangeQuery executes a time range query and sends results to the client.
// When frozen is false (live mode) and a viewport is provided, it uses
// GetLatestByCurrentPositionInBBox which finds each entity's truly-latest
// observation first and then filters by position. This prevents the "jumping"
// problem where different viewports show different historical positions.
func (c *Client) doTimeRangeQuery(layerID string, from, to time.Time, frozen bool, viewport *domain.BBox) {
	if c.server == nil || c.server.obsRepo == nil {
		return
	}

	layerType := c.server.layerTypeForID(layerID)

	cfg := c.server.getTimeRangeConfig()
	// Use a longer timeout for the initial time_range query (up to 10s)
	// since it may return more results than a viewport update.
	queryTimeout := cfg.QueryTimeout
	if queryTimeout < 10*time.Second {
		queryTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	// Use a higher limit for the initial query to front-load more entities.
	// The snapshot will be chunked for delivery, so large results are fine.
	maxResults := cfg.MaxResults
	if maxResults < 5000 {
		maxResults = 5000
	}

	var snapshots []*domain.EntitySnapshot
	var err error

	if viewport != nil {
		if frozen {
			// Historical/frozen mode: filter observations by position within the time window.
			// This is correct for historical replay — show all entities that were in this area.
			snapshots, err = c.server.obsRepo.GetLatestForLayerByBBox(
				ctx, string(layerType),
				viewport.South, viewport.North, viewport.West, viewport.East,
				from, to,
				maxResults,
			)
		} else {
			// Live mode: find each entity's truly-latest observation, then filter
			// by current position. Prevents position-jumping artifacts.
			snapshots, err = c.server.obsRepo.GetLatestByCurrentPositionInBBox(
				ctx, string(layerType),
				viewport.South, viewport.North, viewport.West, viewport.East,
				from, to,
				maxResults,
			)
		}
	} else {
		// No viewport — use layer-wide query with time bounds
		snapshots, err = c.server.obsRepo.GetLayerSnapshotAt(ctx, string(layerType), to, to.Sub(from))
	}

	if err != nil {
		c.Logger.Error().Err(err).Str("layer", layerID).Msg("time_range query failed")
		return
	}

	// The durable store is the only source here. A second leg used to merge in
	// hot-cache entities on the premise that the cache held fresher data, but
	// the feeder writes SQLite before it writes the cache (and the enrichment
	// worker does the same), so the cache was never ahead — it was only less
	// complete and, because it iterated a Go map, non-deterministic about which
	// subset it returned.

	// Convert snapshots to entities + observations.
	entities := make([]*domain.Entity, 0, len(snapshots))
	observations := make([]*domain.Observation, 0, len(snapshots))
	for _, s := range snapshots {
		e := s.Entity
		o := s.Observation
		// For non-hybrid paths (frozen, viewport), source defaults to "historical".
		if e.Source == "" {
			e.Source = "historical"
			o.Source = "historical"
		}
		// Normalize DB UUID → composite ID at the transport boundary
		// so frontend ParseEntityID (layerType:externalID) works correctly.
		compositeID := domain.EntityID(domain.LayerType(e.LayerType), e.ExternalID)
		e.ID = compositeID
		o.EntityID = compositeID
		entities = append(entities, &e)
		observations = append(observations, &o)
	}

	c.server.SendSnapshotToClient(c, layerID, entities, observations)
	c.Logger.Info().
		Str("layer", layerID).
		Int("entities", len(entities)).
		Time("from", from).
		Time("to", to).
		Msg("sent time_range snapshot")
}
