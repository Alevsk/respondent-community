package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Entity represents a normalized geospatial object
type Entity struct {
	ID         string            `json:"id"`
	ExternalID string            `json:"external_id"`
	LayerType  string            `json:"layer_type"` // flights_commercial, flights_military, satellites, earthquakes, traffic, cctv
	Name       string            `json:"name"`
	Metadata   map[string]string `json:"metadata"`
	AIMetadata map[string]any    `json:"ai_metadata,omitempty"`
	Source     string            `json:"source,omitempty"` // "live", "stale", or "historical" — data provenance
	CreatedAt  time.Time         `json:"created_at"`
}

// Observation represents a time-series record of an entity
type Observation struct {
	ID          string             `json:"id"`
	EntityID    string             `json:"entity_id"`
	Timestamp   time.Time          `json:"timestamp"`
	EventTime   *time.Time         `json:"event_time,omitempty"`
	EventEnd    *time.Time         `json:"event_end,omitempty"`
	Position    *GeoPoint          `json:"position,omitempty"` // nil for global_indicator entities (no geographic coordinates)
	AltitudeM   float64            `json:"altitude_m"`
	Velocity    map[string]float64 `json:"velocity"`
	Metadata    map[string]string  `json:"metadata"`
	AIMetadata  map[string]any     `json:"ai_metadata,omitempty"`
	SourceType  string             `json:"source_type"` // which adapter produced this observation
	ContentHash string             `json:"content_hash"`
	Source      string             `json:"source,omitempty"` // "live", "stale", or "historical" — data provenance
	CreatedAt   time.Time          `json:"created_at"`
}

// Validate checks that the Entity has required fields.
func (e *Entity) Validate() error {
	if e.ID == "" {
		return NewInvalidInputError("entity ID is required", nil)
	}
	if e.LayerType == "" {
		return NewInvalidInputError("entity layer_type is required", nil)
	}
	return nil
}

// ComputeContentHash computes a SHA-256 hash of the observation's payload fields.
// This is used by the dedupe recording mode to detect unchanged observations.
func (o *Observation) ComputeContentHash() (string, error) {
	velJSON, err := json.Marshal(o.Velocity)
	if err != nil {
		return "", fmt.Errorf("failed to marshal velocity: %w", err)
	}
	metaJSON, err := json.Marshal(o.Metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	var lat, lon float64
	if o.Position != nil {
		lat, lon = o.Position.Lat, o.Position.Lon
	}
	data := fmt.Sprintf("%f|%f|%f|%s|%s|%d",
		lat, lon, o.AltitudeM,
		velJSON, metaJSON, o.Timestamp.UnixNano())
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h), nil
}

// EntitySnapshot pairs an entity with its observation at a point in time.
// Used for historical replay queries.
type EntitySnapshot struct {
	Entity      Entity      `json:"entity"`
	Observation Observation `json:"observation"`
}

// EntitySearchResult represents a search result combining entity info with its latest observation.
type EntitySearchResult struct {
	Entity            Entity       `json:"entity"`
	LatestObservation *Observation `json:"latest_observation,omitempty"`
}

// GeoPoint represents a geographic position
type GeoPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Alt float64 `json:"alt"`
}

// EntityType discriminates between geographic (globe-rendered) and
// non-geographic (HUD-rendered) entities.
type EntityType string

const (
	// EntityTypeGeo is the default entity type: has lat/lon, rendered on globe.
	EntityTypeGeo EntityType = "geo_entity"
	// EntityTypeIndicator represents a global indicator: no lat/lon, rendered as HUD overlay.
	EntityTypeIndicator EntityType = "global_indicator"
)

// IndicatorSpec defines how a global indicator entity's metadata fields
// map to structured indicator values for HUD rendering.
// ComputeSummary is a closure set during source registration — either compiled
// from a CEL summary_expr or built from DefaultComputeSummary.
type IndicatorSpec struct {
	Values         []IndicatorValueSpec
	ComputeSummary func(metadata map[string]string, level int32) string `json:"-"`
}

// IndicatorValueSpec defines a single indicator reading within an IndicatorSpec.
// ComputeLevel is a closure set during source registration — either compiled
// from a CEL level_expr or built from DefaultComputeLevel using LevelThresholds/MaxLevel.
type IndicatorValueSpec struct {
	Key             string
	Label           string
	SourceField     string
	Unit            string
	MaxLevel        int
	LevelThresholds []float64
	ComputeLevel    func(rawVal string, changePct string) int32 `json:"-"`
	// Extended display fields
	ChangeSourceField string // metadata key for % change, e.g. "vix_change_pct"
	Format            string // "scale", "number", "currency", "percent"
	Precision         int    // decimal places
	Prefix            string // "$", etc.
}

// DefaultComputeLevel returns a closure that computes indicator level from
// thresholds and max_level. This is the fallback when no CEL level_expr is provided.
// When thresholds are present, the level is the highest index whose threshold the
// value meets or exceeds. Otherwise the raw value is used directly as the level.
// The result is always clamped to [0, maxLevel].
func DefaultComputeLevel(thresholds []float64, maxLevel int) func(string, string) int32 {
	if maxLevel <= 0 {
		maxLevel = 5
	}
	clamp := int32(maxLevel)

	return func(rawVal string, _ string) int32 {
		if len(thresholds) == 0 {
			v, err := strconv.ParseFloat(rawVal, 64)
			if err != nil {
				return 0
			}
			level := int32(v)
			if level < 0 {
				return 0
			}
			if level > clamp {
				return clamp
			}
			return level
		}

		val, err := strconv.ParseFloat(rawVal, 64)
		if err != nil {
			return 0
		}

		var level int32
		for i, threshold := range thresholds {
			if val >= threshold {
				level = int32(i)
			} else {
				break
			}
		}

		if level > clamp {
			level = clamp
		}
		return level
	}
}

// DefaultComputeSummary returns a closure that maps severity levels to
// human-readable labels. This is the fallback when no CEL summary_expr is provided.
func DefaultComputeSummary() func(map[string]string, int32) string {
	return func(_ map[string]string, level int32) string {
		switch {
		case level == 0:
			return "Quiet"
		case level == 1:
			return "Minor Activity"
		case level == 2:
			return "Moderate"
		case level == 3:
			return "Strong"
		case level == 4:
			return "Severe"
		case level >= 5:
			return "Extreme"
		default:
			return "Unknown"
		}
	}
}

// EntityUpdate represents a delta update from the real-time stream
type EntityUpdateType int

const (
	EntityUpdateUnknown EntityUpdateType = iota
	EntityUpdateCreate
	EntityUpdateUpdate
	EntityUpdateDelete
)

type EntityUpdate struct {
	Type        EntityUpdateType `json:"type"`
	Entity      *Entity          `json:"entity"`
	Observation *Observation     `json:"observation"`
}
