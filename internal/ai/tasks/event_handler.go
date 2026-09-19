package tasks

import (
	"context"
	"encoding/json"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// EventHandler processes enrichment completion events and triggers matching
// task pipelines. It checks all loaded task definitions for event-type
// triggers that match the given source, then executes those tasks in
// background goroutines.
type EventHandler struct {
	engine      *Engine
	definitions map[string]*TaskDefinition
	logger      zerolog.Logger
}

// NewEventHandler creates an EventHandler wired to the given engine and definitions.
func NewEventHandler(engine *Engine, definitions map[string]*TaskDefinition, logger zerolog.Logger) *EventHandler {
	return &EventHandler{
		engine:      engine,
		definitions: definitions,
		logger:      logger,
	}
}

// HandleEnrichmentComplete is called when an entity's enrichment completes.
// It iterates all loaded task definitions, selects those with a matching
// event trigger, and executes them asynchronously. Errors are logged but
// do not propagate to the caller (fire-and-forget semantics).
func (h *EventHandler) HandleEnrichmentComplete(ctx context.Context, entityID, sourceName, layerType string, metadata map[string]any) {
	for _, def := range h.definitions {
		if !def.Enabled {
			continue
		}
		if def.Trigger.Type != "event" {
			continue
		}
		if def.Trigger.Source != "" && def.Trigger.Source != sourceName {
			continue
		}

		triggerData := map[string]any{
			"entity_id":  entityID,
			"source":     sourceName,
			"layer_type": layerType,
			"metadata":   metadata,
		}

		go func(d *TaskDefinition) {
			if _, err := h.engine.ExecuteTask(ctx, d, triggerData); err != nil {
				h.logger.Error().Err(err).Str("task", d.Name).Msg("event-triggered task failed")
			}
		}(def)
	}
}

// EnrichmentCompleteEvent is the payload published by the analyzer when
// enrichment completes for an entity. Deserialized from NATS messages.
type EnrichmentCompleteEvent struct {
	EntityID   string         `json:"entity_id"`
	SourceName string         `json:"source_name"`
	LayerType  string         `json:"layer_type"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// StartNATSConsumer starts consuming enrichment completion events from NATS.
// Blocks until ctx is cancelled. Used by the tasker binary.
func (h *EventHandler) StartNATSConsumer(ctx context.Context, consumer domain.StreamConsumer, cfg domain.StreamConsumerConfig) error {
	return consumer.Consume(ctx, cfg, func(msg domain.ConsumedMessage) {
		var event EnrichmentCompleteEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error().Err(err).Msg("failed to unmarshal enrichment complete event")
			if ackErr := msg.Ack(); ackErr != nil {
				h.logger.Error().Err(ackErr).Msg("failed to ack malformed message")
			}
			return
		}

		h.HandleEnrichmentComplete(ctx, event.EntityID, event.SourceName, event.LayerType, event.Metadata)

		if err := msg.Ack(); err != nil {
			h.logger.Error().Err(err).Msg("failed to ack enrichment complete event")
		}
	})
}
