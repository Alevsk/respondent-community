package parsers

import (
	"encoding/json"
	"fmt"
)

// JSONParser parses JSON responses into records.
//
// Behavior:
//   - If the top-level value is an array, use it directly as the record list.
//   - If the top-level value is an object, use records_path to locate the array.
//   - Missing records_path returns an empty slice (no error).
//   - JSON numbers are unmarshaled as float64 (standard json.Unmarshal behavior).
//   - MaxRecords is enforced by truncation, not by error.
//
// v2 reshaping transforms (applied before records_path extraction):
//   - ObjectToRecords: converts {"k": {...}, ...} → [{...}, ...] with optional key injection.
//   - ArrayOfArrays: converts [["h1","h2"],[v1,v2],...] → [{"h1":v1,"h2":v2},...].
type JSONParser struct{}

// Parse implements RecordParser.
func (p *JSONParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("json parser: empty input")
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json parser: unmarshal failed: %w", err)
	}

	// Apply reshaping transforms before standard extraction.
	if config.ObjectToRecords {
		reshaped, err := objectToRecords(raw, config)
		if err != nil {
			return nil, fmt.Errorf("json parser: object_to_records: %w", err)
		}
		return truncateRecords(reshaped, config.MaxRecords), nil
	}
	if config.ArrayOfArrays {
		reshaped, err := arrayOfArraysToRecords(raw, config)
		if err != nil {
			return nil, fmt.Errorf("json parser: array_of_arrays: %w", err)
		}
		return truncateRecords(reshaped, config.MaxRecords), nil
	}

	var records []map[string]interface{}

	hasArrayColumns := len(config.ArrayColumns) > 0

	switch v := raw.(type) {
	case []interface{}:
		// Top-level array: use directly, ignore records_path.
		var err error
		if hasArrayColumns {
			records, err = toMapSliceWithColumns(v, config.ArrayColumns)
		} else {
			records, err = toMapSlice(v)
		}
		if err != nil {
			return nil, fmt.Errorf("json parser: %w", err)
		}

	case map[string]interface{}:
		if config.RecordsPath == "" {
			// Single object with no records_path: wrap as single-element slice.
			records = []map[string]interface{}{v}
		} else {
			var err error
			if hasArrayColumns {
				records, err = ExtractByPathWithColumns(v, config.RecordsPath, config.ArrayColumns)
			} else {
				records, err = ExtractByPath(v, config.RecordsPath)
			}
			if err != nil {
				return nil, fmt.Errorf("json parser: records_path %q: %w", config.RecordsPath, err)
			}
			if records == nil {
				records = []map[string]interface{}{}
			}
		}

	default:
		return nil, fmt.Errorf("json parser: unexpected top-level type %T", raw)
	}

	return truncateRecords(records, config.MaxRecords), nil
}

// objectToRecords converts a JSON object into an array of its values.
// If the top-level value is an object at records_path (or at the root when
// records_path is empty), each value becomes a record. If ObjectKeyField is
// set, the object key is injected into each record under that field name.
//
// Input:  {"serial1": {"lat": 34.7}, "serial2": {"lat": 35.1}}
// Output: [{"lat": 34.7, "_key": "serial1"}, {"lat": 35.1, "_key": "serial2"}]
func objectToRecords(raw interface{}, config ParserConfig) ([]map[string]interface{}, error) {
	target := raw

	// Navigate to records_path if specified.
	if config.RecordsPath != "" {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at root, got %T", raw)
		}
		extracted, err := extractByPathRaw(obj, config.RecordsPath)
		if err != nil {
			return nil, err
		}
		if extracted == nil {
			return []map[string]interface{}{}, nil
		}
		target = extracted
	}

	obj, ok := target.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected object for object_to_records, got %T", target)
	}

	records := make([]map[string]interface{}, 0, len(obj))
	for key, val := range obj {
		record, ok := val.(map[string]interface{})
		if !ok {
			// Skip non-object values.
			continue
		}
		if config.ObjectKeyField != "" {
			record[config.ObjectKeyField] = key
		}
		records = append(records, record)
	}

	return records, nil
}

// arrayOfArraysToRecords converts a JSON array where the first element is a
// header row (array of strings) and subsequent elements are data rows (arrays
// of values) into an array of objects keyed by the header names.
//
// Input:  [["time_tag","Kp"], ["2026-03-01","2.67"], ["2026-03-02","3.00"]]
// Output: [{"time_tag":"2026-03-01","Kp":"2.67"}, {"time_tag":"2026-03-02","Kp":"3.00"}]
func arrayOfArraysToRecords(raw interface{}, config ParserConfig) ([]map[string]interface{}, error) {
	target := raw

	// Navigate to records_path if specified.
	if config.RecordsPath != "" {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object at root, got %T", raw)
		}
		extracted, err := extractByPathRaw(obj, config.RecordsPath)
		if err != nil {
			return nil, err
		}
		if extracted == nil {
			return []map[string]interface{}{}, nil
		}
		target = extracted
	}

	arr, ok := target.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array for array_of_arrays, got %T", target)
	}

	if len(arr) < 2 {
		// Need at least a header row and one data row.
		return []map[string]interface{}{}, nil
	}

	// Extract header row.
	headerArr, ok := arr[0].([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array for header row, got %T", arr[0])
	}

	headers := make([]string, len(headerArr))
	for i, h := range headerArr {
		s, ok := h.(string)
		if !ok {
			headers[i] = fmt.Sprintf("col_%d", i)
		} else {
			headers[i] = s
		}
	}

	// Convert data rows.
	records := make([]map[string]interface{}, 0, len(arr)-1)
	for _, row := range arr[1:] {
		rowArr, ok := row.([]interface{})
		if !ok {
			// Skip non-array rows.
			continue
		}
		record := make(map[string]interface{}, len(headers))
		for i, col := range headers {
			if i < len(rowArr) {
				record[col] = rowArr[i]
			}
		}
		records = append(records, record)
	}

	return records, nil
}

// extractByPathRaw traverses a nested object using a dot-separated path and
// returns the raw value found (without converting to []map[string]interface{}).
func extractByPathRaw(data map[string]interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	segments := splitPath(path)
	var current interface{} = data

	for _, segment := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, nil
		}
		val, exists := m[segment]
		if !exists {
			return nil, nil
		}
		current = val
	}

	return current, nil
}

// splitPath splits a dot-separated path into segments.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	result := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				result = append(result, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		result = append(result, path[start:])
	}
	return result
}
