// Package domain provides core domain types, constants, and utilities.
package domain

import (
	"fmt"
	"strings"
)

// LayerType represents the type of data layer
type LayerType string

// SourceType represents the type of data source for ingestion.
// Each source type has its own adapter implementation and configuration.
type SourceType string

// String returns the string representation of the LayerType
func (lt LayerType) String() string {
	return string(lt)
}

// String returns the string representation of the SourceType
func (s SourceType) String() string {
	return string(s)
}

// EntityID generates a consistent entity ID from layer type and external ID
func EntityID(layerType LayerType, externalID string) string {
	return fmt.Sprintf("%s:%s", layerType, externalID)
}

// ParseEntityID extracts layer type and external ID from entity ID
func ParseEntityID(entityID string) (LayerType, string) {
	parts := strings.SplitN(entityID, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return LayerType(parts[0]), parts[1]
}

// FormatLayerName converts a source_type to readable name
// e.g., "open_sky_flights" -> "Open Sky Flights"
func FormatLayerName(sourceType string) string {
	parts := strings.Split(sourceType, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// ParseLayerName converts a readable name back to source_type
// e.g., "Open Sky Flights" -> "open_sky_flights"
func ParseLayerName(name string) string {
	parts := strings.Split(name, " ")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToLower(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "_")
}

// LayerStyle defines rendering configuration for a layer type.
type LayerStyle struct {
	Color     string // CSS color string (e.g., "#00ff9d")
	PointSize int32  // Point size in pixels
}

// DefaultLayerStyle is the fallback style for layer types with no registered display config.
var DefaultLayerStyle = LayerStyle{Color: "#ffffff", PointSize: 8}

// GeoCacheConfig controls the Valkey geo sorted set maintenance for spatial layers.
// Over-fetching compensates for stale geo index members (entity hashes expired but
// geo sorted set entries remain), and lazy GC prunes dead members when the alive
// ratio drops below the threshold.
type GeoCacheConfig struct {
	OverfetchRatio      float64 `yaml:"overfetch_ratio" json:"overfetch_ratio"`
	AliveRatioThreshold float64 `yaml:"alive_ratio_threshold" json:"alive_ratio_threshold"`
	GCBatchSize         int     `yaml:"gc_batch_size" json:"gc_batch_size"`
}

// DefaultGeoCacheConfig returns conservative defaults for geo cache maintenance.
func DefaultGeoCacheConfig() GeoCacheConfig {
	return GeoCacheConfig{
		OverfetchRatio:      3.0,
		AliveRatioThreshold: 0.5,
		GCBatchSize:         500,
	}
}

// SourceConfig holds configuration for a data source adapter.
type SourceConfig struct {
	Type     SourceType
	APIURL   string
	Interval int // seconds
	Timeout  int // seconds
	Options  map[string]interface{}
}
