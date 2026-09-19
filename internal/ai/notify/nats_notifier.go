package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// Compile-time interface assertion.
var _ Notifier = (*NATSNotifier)(nil)

// NATSNotifier implements Notifier by publishing JSON messages to NATS subjects.
// Used by analyzer and tasker services to push notifications that the respondent
// API server bridges to WebSocket clients.
type NATSNotifier struct {
	publisher     domain.MessagePublisher
	subjectPrefix string // e.g., "respondent.notifications"
	logger        zerolog.Logger
}

// NewNATSNotifier creates a NATSNotifier that publishes to {subjectPrefix}.{type}.
func NewNATSNotifier(publisher domain.MessagePublisher, subjectPrefix string, logger zerolog.Logger) *NATSNotifier {
	return &NATSNotifier{
		publisher:     publisher,
		subjectPrefix: subjectPrefix,
		logger:        logger,
	}
}

// NotifyInsight publishes an ai_insight notification to NATS.
func (n *NATSNotifier) NotifyInsight(ctx context.Context, insight *domain.AIInsight) error {
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

	subject := fmt.Sprintf("%s.%s", n.subjectPrefix, WSTypeAIInsight)
	if err := n.publisher.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("publish ai_insight to %s: %w", subject, err)
	}

	n.logger.Debug().
		Str("insight_id", insight.ID).
		Str("insight_type", insight.InsightType).
		Msg("published ai_insight NATS notification")

	return nil
}

// NotifyEnrichment publishes an ai_enrichment notification to NATS.
func (n *NATSNotifier) NotifyEnrichment(ctx context.Context, entityID, operationName, sourceName string, enrichedFields []string) error {
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

	subject := fmt.Sprintf("%s.%s", n.subjectPrefix, WSTypeAIEnrichment)
	if err := n.publisher.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("publish ai_enrichment to %s: %w", subject, err)
	}

	n.logger.Debug().
		Str("entity_id", entityID).
		Str("operation", operationName).
		Msg("published ai_enrichment NATS notification")

	return nil
}
