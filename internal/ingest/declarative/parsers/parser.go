package parsers

import "fmt"

// RecordParser converts raw response bytes into a slice of records.
// Each record is a map[string]interface{} suitable for CEL evaluation.
type RecordParser interface {
	// Parse converts raw response bytes into a slice of records.
	// config provides parser-specific options (records_path, csv_options, etc.)
	Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error)
}

// ParserConfig holds parser-specific options derived from the source definition.
type ParserConfig struct {
	RecordsPath  string
	MaxRecords   int
	CSVOptions   *CSVOpts
	ArrayColumns []string // v2: map array-of-arrays indices to named fields

	// v2: JSON reshaping transforms.
	ObjectToRecords bool   // Convert top-level object values to record array.
	ObjectKeyField  string // Inject object key as this field (requires ObjectToRecords).
	ArrayOfArrays   bool   // Treat array-of-arrays as header+data table.
}

// CSVOpts holds CSV-specific parsing options.
type CSVOpts struct {
	Delimiter          string
	HasHeader          bool
	SkipLines          int
	CollapseWhitespace bool   // Collapse runs of spaces/tabs into a single tab before parsing.
	CommentPrefix      string // Strip this prefix from the start of lines (e.g., "#").
}

// DefaultMaxRecords is applied when MaxRecords is zero or negative.
const DefaultMaxRecords = 10000

// MaxMaxRecords is the absolute upper bound for MaxRecords.
const MaxMaxRecords = 100000

// effectiveMaxRecords returns the MaxRecords value to use, applying defaults
// and clamping to the upper bound.
func effectiveMaxRecords(configured int) int {
	if configured <= 0 {
		return DefaultMaxRecords
	}
	if configured > MaxMaxRecords {
		return MaxMaxRecords
	}
	return configured
}

// truncateRecords enforces the MaxRecords limit by truncating the slice.
func truncateRecords(records []map[string]interface{}, maxRecords int) []map[string]interface{} {
	limit := effectiveMaxRecords(maxRecords)
	if len(records) > limit {
		return records[:limit]
	}
	return records
}

// NewParser returns a RecordParser for the given format string.
// Supported formats: json, json_table, geojson, csv, tle, xml, rss.
func NewParser(format string) (RecordParser, error) {
	switch format {
	case "json":
		return &JSONParser{}, nil
	case "json_table":
		return &JSONTableParser{}, nil
	case "geojson":
		return &GeoJSONParser{}, nil
	case "csv":
		return &CSVParser{}, nil
	case "tle":
		return &TLEParser{}, nil
	case "xml":
		return &XMLParser{}, nil
	case "rss":
		return &RSSParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported parser format: %q", format)
	}
}
