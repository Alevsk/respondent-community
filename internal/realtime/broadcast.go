package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/domain"
)

// pendingUpdate holds a parsed Pub/Sub update awaiting batched delivery.
type pendingUpdate struct {
	data        json.RawMessage // the original entity update payload (entity + observation)
	lat, lon    float64
	hasSpatial  bool
	eventTime   time.Time // event_time or timestamp from the observation
	hasTemporal bool
}

// flushBatchedUpdates sends accumulated Pub/Sub updates to subscribed clients
// as a single batched message per layer. Viewport filtering is applied per-client.
func (s *Server) flushBatchedUpdates() {
	s.pendingMu.Lock()
	if len(s.pendingUpdates) == 0 {
		s.pendingMu.Unlock()
		return
	}
	// Swap out the pending map so we release the lock quickly.
	pending := s.pendingUpdates
	s.pendingUpdates = make(map[string][]pendingUpdate)
	s.pendingMu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for layerID, updates := range pending {
		if len(updates) == 0 {
			continue
		}

		layerType := s.layerTypeForID(layerID)
		spatial := s.isSpatialLayer(string(layerType))

		for client := range s.clients {
			if !client.IsSubscribed(layerID) {
				continue
			}
			if client.IsFrozen(layerID) {
				continue
			}

			// Global mode clients receive ALL updates (no viewport filtering).
			isGlobal := client.IsGlobalMode(layerID)

			// Filter updates by viewport for spatial layers.
			// Also cap the batch size for very large viewports to prevent flooding.
			viewport := client.GetViewport(layerID)
			maxBatch := len(updates)
			if spatial && viewport != nil && !isGlobal {
				maxBatch = viewportEntityLimit(viewport, len(updates))
			}

			// Pre-compute time range filter state for this client+layer.
			var trFrom, trTo time.Time
			hasTimeRange := false
			if client.IsInTimeRange(layerID) {
				trFrom, trTo, hasTimeRange = client.GetTimeRange(layerID)
			}

			var filtered []json.RawMessage
			for i := range updates {
				// Temporal filter for non-frozen time ranges.
				if hasTimeRange && updates[i].hasTemporal {
					if updates[i].eventTime.Before(trFrom) || updates[i].eventTime.After(trTo) {
						continue
					}
				}

				if spatial && updates[i].hasSpatial && viewport != nil && !isGlobal {
					if !viewport.Contains(updates[i].lat, updates[i].lon) {
						continue
					}
				}
				filtered = append(filtered, updates[i].data)
				if len(filtered) >= maxBatch {
					break
				}
			}

			if len(filtered) == 0 {
				continue
			}

			// Build the batched message.
			msg := WSMessage{
				Type:    "layer.batch_update",
				LayerID: layerID,
				Data: map[string]any{
					"updates": filtered,
				},
			}
			data, err := json.Marshal(msg)
			if err != nil {
				s.logger.Error().Err(err).Str("layer", layerID).Msg("failed to marshal batched update")
				continue
			}

			select {
			case client.Send <- data:
			default:
				client.droppedCount.Add(1)
				s.logger.Warn().Str("client", client.ID).Str("layer", layerID).Msg("client send buffer full (batch)")
			}
		}
	}
}

// runFlushTicker runs a standalone 150ms ticker that drains pendingUpdates.
// Used in community mode (no Redis) so that updates queued via QueueUpdate
// are batched identically to the Redis Pub/Sub path.
func (s *Server) runFlushTicker(ctx context.Context) {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushBatchedUpdates()
		}
	}
}

// QueueUpdate adds a raw entity update to the pending batch for a layer.
// The update is delivered to subscribed clients on the next 150ms flush tick
// as part of a layer.batch_update message, with viewport filtering applied.
// This is the community-edition equivalent of Redis Pub/Sub ingestion.
func (s *Server) QueueUpdate(layerID string, data json.RawMessage) {
	pu := pendingUpdate{data: data}

	// Extract spatial coordinates for viewport filtering.
	layerType := s.layerTypeForID(layerID)
	if s.isSpatialLayer(string(layerType)) {
		var updateData struct {
			Observation *struct {
				Position *struct {
					Lat float64 `json:"lat"`
					Lon float64 `json:"lon"`
				} `json:"position"`
			} `json:"observation"`
		}
		if json.Unmarshal(data, &updateData) == nil && updateData.Observation != nil && updateData.Observation.Position != nil {
			pu.lat = updateData.Observation.Position.Lat
			pu.lon = updateData.Observation.Position.Lon
			pu.hasSpatial = true
		}
	}

	// Extract temporal data for time-range filtering.
	var updateMap map[string]any
	if json.Unmarshal(data, &updateMap) == nil {
		if obsMap, ok := updateMap["observation"].(map[string]any); ok {
			if et, ok := obsMap["event_time"].(string); ok && et != "" {
				if parsed, err := time.Parse(time.RFC3339, et); err == nil {
					pu.eventTime = parsed
					pu.hasTemporal = true
				}
			}
			if !pu.hasTemporal {
				if ts, ok := obsMap["timestamp"].(string); ok && ts != "" {
					if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
						pu.eventTime = parsed
						pu.hasTemporal = true
					}
				}
			}
		}
	}

	s.pendingMu.Lock()
	s.pendingUpdates[layerID] = append(s.pendingUpdates[layerID], pu)
	s.pendingMu.Unlock()
}

// BroadcastUpdate broadcasts an entity update to all connected clients
func (s *Server) BroadcastUpdate(update *domain.EntityUpdate) {
	if update == nil || update.Entity == nil {
		s.logger.Warn().Msg("BroadcastUpdate called with nil update or nil entity, skipping")
		return
	}

	// Ensure data provenance is always set for frontend timestamp-guarded merging.
	if update.Entity.Source == "" {
		update.Entity.Source = "live"
	}
	if update.Observation != nil && update.Observation.Source == "" {
		update.Observation.Source = "live"
	}

	msg := WSMessage{
		Type:    "layer.update",
		LayerID: update.Entity.LayerType,
		Data:    update,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal update")
		return
	}

	s.broadcast <- data
}

// BroadcastLayerUpdate broadcasts a layer update to subscribed clients only
// CRITICAL FIX: This method was rewritten to avoid a race condition where a goroutine
// was spawned while holding RLock, which could cause deadlock when the goroutine
// tried to write to the unregister channel (which requires Lock in Run()).
func (s *Server) BroadcastLayerUpdate(layerID string, entities []*domain.Entity, observations []*domain.Observation) {
	// Ensure data provenance is always set for frontend timestamp-guarded merging.
	for _, e := range entities {
		if e.Source == "" {
			e.Source = "live"
		}
	}
	for _, o := range observations {
		if o.Source == "" {
			o.Source = "live"
		}
	}

	msg := WSMessage{
		Type:    "layer.update",
		LayerID: layerID,
		Data: map[string]any{
			"entities":     entities,
			"observations": observations,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal layer update")
		return
	}

	// Collect clients that need to be unregistered (to avoid holding lock while sending to channel)
	var clientsToUnregister []*Client

	s.mu.RLock()
	for client := range s.clients {
		if client.IsSubscribed(layerID) {
			select {
			case client.Send <- data:
				// Message sent successfully
			default:
				// Channel full — increment counter before marking for unregister
				// so that if the client reconnects it sees the drop count.
				client.droppedCount.Add(1)
				s.logger.Warn().Str("id", client.ID).Msg("client send buffer full, marking for unregister")
				clientsToUnregister = append(clientsToUnregister, client)
			}
		}
	}
	s.mu.RUnlock()

	// Unregister clients outside of the lock to avoid deadlock
	for _, client := range clientsToUnregister {
		select {
		case s.unregister <- client:
			// Unregister request sent
		default:
			// Unregister channel full, log and skip
			s.logger.Warn().Str("id", client.ID).Msg("unregister channel full, skipping client cleanup")
		}
	}
}

// BroadcastSnapshot broadcasts a layer snapshot to all connected clients
func (s *Server) BroadcastSnapshot(layerID string, entities []*domain.Entity, observations []*domain.Observation) {
	msg := WSMessage{
		Type:    "layer.snapshot",
		LayerID: layerID,
		Data: map[string]any{
			"entities":     entities,
			"observations": observations,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal snapshot")
		return
	}

	s.broadcast <- data
}

// BroadcastCCTVFrame broadcasts a CCTV frame update
func (s *Server) BroadcastCCTVFrame(cameraFeedID string, frameData []byte) {
	msg := WSMessage{
		Type:    "cctv.frame",
		LayerID: "cctv",
		Data: map[string]any{
			"camera_feed_id": cameraFeedID,
			"frame":          frameData,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to marshal CCTV frame")
		return
	}

	s.broadcast <- data
}

// BroadcastRaw pushes a pre-serialized JSON message to all connected clients.
// This is used by external packages (e.g., ai/notify) that construct their own
// message format and only need access to the broadcast fan-out.
func (s *Server) BroadcastRaw(data []byte) {
	s.broadcast <- data
}

// BroadcastNotification pushes a pre-serialized AI insight notification to
// clients whose notification filter matches the given metadata.
func (s *Server) BroadcastNotification(meta notify.NotificationMeta, data []byte) {
	s.notificationBroadcast <- notificationBroadcastMsg{Meta: meta, Data: data}
}
