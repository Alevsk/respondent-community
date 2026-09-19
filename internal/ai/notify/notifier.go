// Package notify provides WebSocket push notifications for AI insights and
// enrichment results. It decouples the AI engine/worker from the realtime
// package by depending on a small Broadcaster interface rather than the
// concrete WebSocket server.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// WebSocket message type constants for AI notifications.
const (
	// WSTypeAIInsight is the WebSocket message type for new AI insights.
	WSTypeAIInsight = "ai_insight"

	// WSTypeAIEnrichment is the WebSocket message type for AI enrichment updates.
	WSTypeAIEnrichment = "ai_enrichment"
)

// NotificationMeta carries the filterable fields of an ai_insight message
// so the broadcast loop can filter without re-parsing the JSON payload.
type NotificationMeta struct {
	InsightType string
	Attention   string
	LayerType   string
}

// Broadcaster is the minimal interface required to push messages to all
// connected WebSocket clients. The realtime.Server satisfies this via its
// BroadcastRaw and BroadcastNotification methods.
type Broadcaster interface {
	BroadcastRaw(data []byte)
	BroadcastNotification(meta NotificationMeta, data []byte)
}

// Notifier publishes AI events to connected WebSocket clients.
// Implementations must be safe for concurrent use.
type Notifier interface {
	// NotifyInsight pushes an ai_insight WebSocket message for the given insight.
	NotifyInsight(ctx context.Context, insight *domain.AIInsight) error

	// NotifyEnrichment pushes an ai_enrichment WebSocket message after an
	// entity's AI metadata has been updated.
	NotifyEnrichment(ctx context.Context, entityID, operationName, sourceName string, enrichedFields []string) error
}

// wsMessage mirrors the realtime.WSMessage structure for JSON marshaling.
// We intentionally duplicate this small struct to avoid an import cycle
// (notify -> realtime -> domain -> notify).
type wsMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// InsightEntityRefPayload is a compact entity reference included in WebSocket notifications.
type InsightEntityRefPayload struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	LayerType  string `json:"layer_type,omitempty"`
}

// InsightPayload is the JSON payload for an ai_insight WebSocket message.
// Field names and structure match the spec (Section 10.2).
type InsightPayload struct {
	ID             string                    `json:"id"`
	InsightType    string                    `json:"insight_type"`
	SourceName     string                    `json:"source_name"`
	OperationName  string                    `json:"operation_name"`
	LayerType      string                    `json:"layer_type,omitempty"`
	Attention      string                    `json:"attention,omitempty"`
	Result         map[string]any            `json:"result"`
	EntityIDs      []string                  `json:"entity_ids"`
	Entities       []InsightEntityRefPayload `json:"entities"`
	ObservationIDs []string                  `json:"observation_ids"`
	CreatedAt      string                    `json:"created_at"`
}

// EnrichmentPayload is the JSON payload for an ai_enrichment WebSocket message.
// Field names and structure match the spec (Section 10.2).
type EnrichmentPayload struct {
	EntityID       string   `json:"entity_id"`
	OperationName  string   `json:"operation_name"`
	EnrichedFields []string `json:"enriched_fields"`
	SourceName     string   `json:"source_name"`
}

// WSNotifier implements Notifier by marshaling messages and pushing them
// through a Broadcaster. It is safe for concurrent use.
type WSNotifier struct {
	broadcaster Broadcaster
	logger      zerolog.Logger
}

// NewWSNotifier creates a new WSNotifier. The broadcaster must not be nil.
func NewWSNotifier(broadcaster Broadcaster, logger zerolog.Logger) *WSNotifier {
	return &WSNotifier{
		broadcaster: broadcaster,
		logger:      logger,
	}
}

// NotifyInsight pushes an ai_insight WebSocket message for the given insight.
func (n *WSNotifier) NotifyInsight(_ context.Context, insight *domain.AIInsight) error {
	if insight == nil {
		return nil
	}

	var layerType string
	if insight.LayerType != nil {
		layerType = *insight.LayerType
	}

	entityIDs := insight.EntityIDs
	if entityIDs == nil {
		entityIDs = []string{}
	}

	observationIDs := insight.ObservationIDs
	if observationIDs == nil {
		observationIDs = []string{}
	}

	entities := make([]InsightEntityRefPayload, 0, len(insight.Entities))
	for _, e := range insight.Entities {
		entities = append(entities, InsightEntityRefPayload{
			ID:         e.ID,
			ExternalID: e.ExternalID,
			Name:       e.Name,
			LayerType:  e.LayerType,
		})
	}

	var attention string
	if insight.Attention != nil {
		attention = *insight.Attention
	}

	payload := InsightPayload{
		ID:             insight.ID,
		InsightType:    insight.InsightType,
		SourceName:     insight.SourceName,
		OperationName:  insight.OperationName,
		LayerType:      layerType,
		Attention:      attention,
		Result:         insight.Result,
		EntityIDs:      entityIDs,
		Entities:       entities,
		ObservationIDs: observationIDs,
		CreatedAt:      insight.CreatedAt.UTC().Format(time.RFC3339),
	}

	msg := wsMessage{
		Type:    WSTypeAIInsight,
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal ai_insight message: %w", err)
	}

	meta := NotificationMeta{
		InsightType: insight.InsightType,
		Attention:   attention,
		LayerType:   layerType,
	}
	n.broadcaster.BroadcastNotification(meta, data)

	n.logger.Debug().
		Str("insight_id", insight.ID).
		Str("insight_type", insight.InsightType).
		Str("source", insight.SourceName).
		Msg("published ai_insight WebSocket notification")

	return nil
}

// NotifyEnrichment pushes an ai_enrichment WebSocket message.
func (n *WSNotifier) NotifyEnrichment(_ context.Context, entityID, operationName, sourceName string, enrichedFields []string) error {
	if enrichedFields == nil {
		enrichedFields = []string{}
	}

	payload := EnrichmentPayload{
		EntityID:       entityID,
		OperationName:  operationName,
		EnrichedFields: enrichedFields,
		SourceName:     sourceName,
	}

	msg := wsMessage{
		Type:    WSTypeAIEnrichment,
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal ai_enrichment message: %w", err)
	}

	n.broadcaster.BroadcastRaw(data)

	n.logger.Debug().
		Str("entity_id", entityID).
		Str("operation", operationName).
		Str("source", sourceName).
		Int("fields", len(enrichedFields)).
		Msg("published ai_enrichment WebSocket notification")

	return nil
}

// Ensure WSNotifier implements Notifier at compile time.
var _ Notifier = (*WSNotifier)(nil)
