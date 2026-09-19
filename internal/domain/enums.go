package domain

import "strings"

// ── FilteringMode (moved from internal/config) ─────────────────────

// FilteringModeValue defines how a source delivers data to clients.
type FilteringModeValue string

const (
	// FilteringViewport sends only entities visible in the client's viewport (default).
	FilteringViewport FilteringModeValue = "viewport"
	// FilteringAll sends all entities regardless of viewport.
	FilteringAll FilteringModeValue = "all"
)

// ── ObservationRecordMode (moved from internal/config) ─────────────

// ObservationRecordMode defines how observations are recorded for a source.
type ObservationRecordMode string

const (
	// RecordAppend inserts every observation unconditionally (default, current behavior).
	RecordAppend ObservationRecordMode = "append"
	// RecordUpsert upserts on (entity_id, ts) — one row per entity per timestamp.
	RecordUpsert ObservationRecordMode = "upsert"
	// RecordDedupe skips observations whose content hash matches the latest stored hash.
	RecordDedupe ObservationRecordMode = "dedupe"
)

// Valid returns true if the mode is a recognized observation recording mode.
func (m ObservationRecordMode) Valid() bool {
	switch m {
	case RecordAppend, RecordUpsert, RecordDedupe:
		return true
	default:
		return false
	}
}

// ── Proto enum string↔value conversion maps ───────────────────────
// These map domain string constants to proto enum-compatible integers
// and back. Used by the gRPC server conversion layer.

// LayerModeFromString converts a domain string ("sparse", "full") to the proto enum value.
func LayerModeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "sparse":
		return 1 // LAYER_MODE_SPARSE
	case "full":
		return 2 // LAYER_MODE_FULL
	default:
		return 0 // LAYER_MODE_UNSPECIFIED
	}
}

// LayerModeToString converts a proto enum value to a domain string.
func LayerModeToString(v int32) string {
	switch v {
	case 1:
		return "sparse"
	case 2:
		return "full"
	default:
		return ""
	}
}

// RenderingModeFromString converts a domain string ("map", "indicator") to the proto enum value.
func RenderingModeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "map":
		return 1 // RENDERING_MODE_MAP
	case "indicator":
		return 2 // RENDERING_MODE_INDICATOR
	default:
		return 0 // RENDERING_MODE_UNSPECIFIED
	}
}

// RenderingModeToString converts a proto enum value to a domain string.
func RenderingModeToString(v int32) string {
	switch v {
	case 1:
		return "map"
	case 2:
		return "indicator"
	default:
		return ""
	}
}

// FilteringModeFromString converts a domain string ("viewport") to the proto enum value.
func FilteringModeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "viewport":
		return 1 // FILTERING_MODE_VIEWPORT
	default:
		return 0 // FILTERING_MODE_UNSPECIFIED
	}
}

// FilteringModeToString converts a proto enum value to a domain string.
func FilteringModeToString(v int32) string {
	switch v {
	case 1:
		return "viewport"
	default:
		return ""
	}
}

// IconShapeFromString converts a domain string to the proto enum value.
func IconShapeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "flight":
		return 1
	case "satellite":
		return 2
	case "diamond":
		return 3
	case "ripple":
		return 4
	case "radio":
		return 5
	case "dot":
		return 6
	case "warning":
		return 7
	case "radiation":
		return 8
	case "fire":
		return 9
	default:
		return 0
	}
}

// IconShapeToString converts a proto enum value to a domain string.
func IconShapeToString(v int32) string {
	switch v {
	case 1:
		return "flight"
	case 2:
		return "satellite"
	case 3:
		return "diamond"
	case 4:
		return "ripple"
	case 5:
		return "radio"
	case 6:
		return "dot"
	case 7:
		return "warning"
	case 8:
		return "radiation"
	case 9:
		return "fire"
	default:
		return ""
	}
}

// FieldTypeFromString converts a domain string to the proto enum value.
func FieldTypeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "string":
		return 1
	case "float":
		return 2
	case "integer":
		return 3
	case "raw":
		return 4
	default:
		return 0
	}
}

// FieldTypeToString converts a proto enum value to a domain string.
func FieldTypeToString(v int32) string {
	switch v {
	case 1:
		return "string"
	case 2:
		return "float"
	case 3:
		return "integer"
	case 4:
		return "raw"
	default:
		return ""
	}
}

// FieldTransformFromString converts a domain string to the proto enum value.
func FieldTransformFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "upper":
		return 1
	case "lower":
		return 2
	default:
		return 0
	}
}

// FieldTransformToString converts a proto enum value to a domain string.
func FieldTransformToString(v int32) string {
	switch v {
	case 1:
		return "upper"
	case 2:
		return "lower"
	default:
		return ""
	}
}

// FilterStyleFromString converts a domain string to the proto enum value.
func FilterStyleFromString(s string) int32 {
	switch strings.ToUpper(s) {
	case "NORMAL":
		return 1
	case "CRT":
		return 2
	case "NVG":
		return 3
	case "FLIR":
		return 4
	default:
		return 0
	}
}

// FilterStyleToString converts a proto enum value to a domain string.
func FilterStyleToString(v int32) string {
	switch v {
	case 1:
		return "NORMAL"
	case 2:
		return "CRT"
	case 3:
		return "NVG"
	case 4:
		return "FLIR"
	default:
		return ""
	}
}

// LocationTypeFromString converts a domain string to the proto enum value.
func LocationTypeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "city":
		return 1
	case "landmark":
		return 2
	default:
		return 0
	}
}

// LocationTypeToString converts a proto enum value to a domain string.
func LocationTypeToString(v int32) string {
	switch v {
	case 1:
		return "city"
	case 2:
		return "landmark"
	default:
		return ""
	}
}

// SourceTypeFromString converts a domain string to the proto enum value.
func SourceTypeFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "http_json":
		return 1
	case "geojson":
		return 2
	case "csv":
		return 3
	case "tle":
		return 4
	default:
		return 0
	}
}

// SourceTypeToString converts a proto enum value to a domain string.
func SourceTypeToString(v int32) string {
	switch v {
	case 1:
		return "http_json"
	case 2:
		return "geojson"
	case 3:
		return "csv"
	case 4:
		return "tle"
	default:
		return ""
	}
}

// AttentionLevelFromString converts a domain string to the proto enum value.
func AttentionLevelFromString(s string) int32 {
	switch strings.ToLower(s) {
	case "info":
		return 1
	case "low":
		return 2
	case "medium":
		return 3
	case "high":
		return 4
	case "critical":
		return 5
	default:
		return 0
	}
}

// AttentionLevelToString converts a proto enum value to a domain string.
func AttentionLevelToString(v int32) string {
	switch v {
	case 1:
		return "info"
	case 2:
		return "low"
	case 3:
		return "medium"
	case 4:
		return "high"
	case 5:
		return "critical"
	default:
		return ""
	}
}
