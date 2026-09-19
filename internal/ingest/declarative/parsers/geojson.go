package parsers

import (
	"encoding/json"
	"fmt"
)

// GeoJSONParser parses GeoJSON FeatureCollection (or single Feature) responses
// into records. Each Feature becomes a record with fields:
//
//	geometry, properties, id, type
//
// Behavior:
//   - FeatureCollection: extract the "features" array.
//   - Single Feature: wrap as a one-element slice.
//   - MaxRecords enforced by truncation.
type GeoJSONParser struct{}

// Parse implements RecordParser.
func (p *GeoJSONParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("geojson parser: empty input")
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("geojson parser: unmarshal failed: %w", err)
	}

	geoType, _ := raw["type"].(string)

	switch geoType {
	case "FeatureCollection":
		return p.parseFeatureCollection(raw, config)
	case "Feature":
		record := featureToRecord(raw)
		return truncateRecords([]map[string]interface{}{record}, config.MaxRecords), nil
	default:
		return nil, fmt.Errorf("geojson parser: unsupported type %q, expected FeatureCollection or Feature", geoType)
	}
}

func (p *GeoJSONParser) parseFeatureCollection(raw map[string]interface{}, config ParserConfig) ([]map[string]interface{}, error) {
	featuresRaw, ok := raw["features"]
	if !ok {
		return []map[string]interface{}{}, nil
	}

	featuresArr, ok := featuresRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("geojson parser: 'features' is not an array")
	}

	records := make([]map[string]interface{}, 0, len(featuresArr))
	for _, f := range featuresArr {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		records = append(records, featureToRecord(fm))
	}

	return truncateRecords(records, config.MaxRecords), nil
}

// featureToRecord converts a GeoJSON Feature map into a flat record
// preserving geometry, properties, id, and type fields.
func featureToRecord(feature map[string]interface{}) map[string]interface{} {
	record := make(map[string]interface{}, 4)

	if geom, ok := feature["geometry"]; ok {
		record["geometry"] = geom
	}
	if props, ok := feature["properties"]; ok {
		record["properties"] = props
	}
	if id, ok := feature["id"]; ok {
		record["id"] = id
	}
	if t, ok := feature["type"]; ok {
		record["type"] = t
	}

	return record
}
