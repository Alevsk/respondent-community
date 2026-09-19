package feeder

import (
	"encoding/json"

	"github.com/Alevsk/respondent/internal/domain"
)

// wsMessage is the wire shape for a layer update published to subscribers.
type wsMessage struct {
	Type    string `json:"type"`
	LayerID string `json:"layer_id,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// MarshalLayerUpdate builds the standard JSON payload for a layer update. It is
// transport-agnostic: the bytes are handed to whatever FeederPublisher is wired
// (in-process bus in the community edition).
func MarshalLayerUpdate(entity *domain.Entity, obs *domain.Observation) ([]byte, error) {
	msg := wsMessage{
		Type:    "layer.update",
		LayerID: entity.LayerType,
		Data: map[string]any{
			"entity":      entity,
			"observation": obs,
		},
	}
	return json.Marshal(msg)
}
