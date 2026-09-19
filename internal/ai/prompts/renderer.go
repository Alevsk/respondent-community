// Package prompts provides Go template rendering for AI operation prompts.
package prompts

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/Alevsk/respondent/internal/domain"
)

// PromptData is the template context available to AI operation prompts.
type PromptData struct {
	Entity         EntityData
	Observation    ObservationData
	OutputSchema   string
	LayerType      string
	HasCoordinates bool
	HasAltitude    bool
}

// EntityData exposes entity fields to templates.
type EntityData struct {
	ID           string
	ExternalID   string
	Name         string
	LayerType    string
	Metadata     map[string]string
	MetadataKeys string
	MetadataJSON string
}

// ObservationData exposes observation fields to templates.
type ObservationData struct {
	Lat      float64
	Lon      float64
	Altitude float64
	Metadata map[string]string
}

// NewPromptData creates a PromptData from domain objects.
func NewPromptData(entity *domain.Entity, obs *domain.Observation, schemaJSON string) PromptData {
	var keys []string
	for k := range entity.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	metaJSON, _ := json.Marshal(entity.Metadata)

	pd := PromptData{
		Entity: EntityData{
			ID:           entity.ID,
			ExternalID:   entity.ExternalID,
			Name:         entity.Name,
			LayerType:    entity.LayerType,
			Metadata:     entity.Metadata,
			MetadataKeys: strings.Join(keys, ", "),
			MetadataJSON: string(metaJSON),
		},
		OutputSchema: schemaJSON,
		LayerType:    entity.LayerType,
	}

	if obs != nil {
		pd.Observation = ObservationData{
			Metadata: obs.Metadata,
			Altitude: obs.AltitudeM,
		}
		if obs.Position != nil {
			pd.Observation.Lat = obs.Position.Lat
			pd.Observation.Lon = obs.Position.Lon
			pd.HasCoordinates = true
		}
		pd.HasAltitude = obs.AltitudeM != 0
	}

	return pd
}

// Render renders a prompt template with the given data.
func Render(promptTemplate string, data PromptData) (string, error) {
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}
	return buf.String(), nil
}

// Hash returns a SHA256 hash of the rendered prompt for caching.
func Hash(rendered string) string {
	h := sha256.Sum256([]byte(rendered))
	return fmt.Sprintf("%x", h)
}
