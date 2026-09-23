package domain

import "time"

// Layer represents a configurable data layer
type Layer struct {
	ID            string
	Name          string
	Type          string // flights_commercial, flights_military, satellites, earthquakes, traffic, cctv
	Enabled       bool
	Mode          string // sparse/full
	Density       int32  // 0-100
	Source        string
	LastUpdate    time.Time
	Count         int64
	Color         string // CSS color string for rendering
	PointSize     int32  // Point size in pixels
	Config        map[string]interface{}
	RenderingMode string              // "map" (default) or "indicator"
	FilteringMode string              // "viewport" or "" — drives frontend spatial subscription
	DisplayConfig *LayerDisplayConfig // Rendering metadata from declarative sources (nil for imperative)
	HistoryConfig *HistoryConfig      // Per-layer time range limits (nil = server defaults)
}

// LayerDisplayConfig holds rendering metadata populated from declarative source YAML.
type LayerDisplayConfig struct {
	Icon           *IconConfig
	Trail          *TrailConfig
	Style          *StyleConfig
	FieldRenderers []FieldRendererConfig
	Media          []MediaConfig
	ColorBy        *ColorByConfig // nil unless the layer colors entities by a metadata field
}

// ColorByConfig derives an entity's render color from a metadata field's value,
// keeping per-value color rules declarative instead of hardcoded in the client.
type ColorByConfig struct {
	Field        string            // metadata field whose value selects the color
	Values       map[string]string // field value -> hex color
	DefaultColor string            // hex color used when the value is not in Values
}

// IconConfig defines icon rendering properties.
type IconConfig struct {
	Shape         string
	Rotatable     bool
	Interpolation bool
	Scale         float64
}

// TrailConfig defines trail rendering properties.
type TrailConfig struct {
	Color   string
	Width   float64
	Opacity float64
}

// StyleConfig defines base point styling.
type StyleConfig struct {
	Color     string
	PointSize int32
}

// FieldRendererConfig defines how a metadata field is displayed in the UI.
type FieldRendererConfig struct {
	Keys     []string
	Label    string
	Format   FieldFormat
	Priority int32
}

// FieldFormat describes formatting rules for a rendered field value.
type FieldFormat struct {
	Type      string
	Precision *int32
	Prefix    string
	Suffix    string
	Transform string
}

// HistoryConfig defines per-layer time range limits for historical exploration.
// Zero values mean "use server default" (48h lookback, 24h span).
type HistoryConfig struct {
	MaxLookbackHours  int32
	MaxRangeSpanHours int32
}

// Default layer configuration constants.
const (
	DefaultLayerMode    = "full"
	DefaultLayerDensity = int32(100)
	DefaultLayerSource  = "feeder"
	DefaultRenderMode   = "map"
)

// NewDefaultLayer creates a Layer with standard defaults for the given layer
// type. Callers should enrich the returned layer with display config, history
// config, rendering mode, and filtering mode from the dynamic registry.
func NewDefaultLayer(layerType string, style LayerStyle) *Layer {
	return &Layer{
		ID:            layerType,
		Name:          FormatLayerName(layerType),
		Type:          layerType,
		Enabled:       true,
		Mode:          DefaultLayerMode,
		Density:       DefaultLayerDensity,
		Source:        DefaultLayerSource,
		Color:         style.Color,
		PointSize:     style.PointSize,
		RenderingMode: DefaultRenderMode,
	}
}

// LayerToggle represents layer configuration changes
type LayerToggle struct {
	LayerID string
	Enabled bool
	Mode    string // sparse/full
	Density int32  // 0-100
}
