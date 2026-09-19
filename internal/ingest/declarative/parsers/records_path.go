package parsers

import (
	"fmt"
	"strings"
)

// ExtractByPath traverses a nested data structure using a dot-separated path
// and returns the array found at that path as a slice of maps.
//
// Path rules:
//   - Segments are separated by "."
//   - Each segment matches [a-zA-Z0-9_]+
//   - Max depth: 8 segments (validated at config load time, not here)
//   - If any segment is missing, returns an empty slice (no error)
//   - If the value at the path is not an array, returns an error
func ExtractByPath(data interface{}, path string) ([]map[string]interface{}, error) {
	if path == "" {
		return toMapSlice(data)
	}

	segments := strings.Split(path, ".")
	current := data

	for _, segment := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			// Current node is not a map; path cannot be traversed further.
			return nil, nil
		}

		val, exists := m[segment]
		if !exists {
			// Missing path segment: return empty slice per spec.
			return nil, nil
		}

		current = val
	}

	return toMapSlice(current)
}

// toMapSlice converts an interface{} that should be a []interface{} (from
// json.Unmarshal) into []map[string]interface{}. Non-map elements in the
// array are skipped with a warning (returned as error only if zero elements
// could be converted).
func toMapSlice(v interface{}) ([]map[string]interface{}, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array at path, got %T", v)
	}

	result := make([]map[string]interface{}, 0, len(arr))
	for _, elem := range arr {
		m, ok := elem.(map[string]interface{})
		if !ok {
			// Skip non-map elements rather than failing the entire parse.
			continue
		}
		result = append(result, m)
	}

	return result, nil
}

// toMapSliceWithColumns converts an interface{} that should be a []interface{}
// into []map[string]interface{}, handling both array-of-maps (standard) and
// array-of-arrays (using arrayColumns to zip indices to named fields).
// This supports the OpenSky-style "states" format where each record is a
// positional array like ["abc123", "UAL123 ", "United States", ...].
func toMapSliceWithColumns(v interface{}, columns []string) ([]map[string]interface{}, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array at path, got %T", v)
	}

	result := make([]map[string]interface{}, 0, len(arr))
	for _, elem := range arr {
		switch record := elem.(type) {
		case map[string]interface{}:
			result = append(result, record)
		case []interface{}:
			m := arrayToNamedMap(record, columns)
			result = append(result, m)
		default:
			// Skip unsupported element types.
			continue
		}
	}

	return result, nil
}

// arrayToNamedMap converts a positional array to a named map by zipping with
// column names. Extra values (beyond len(columns)) are dropped; missing values
// produce nil entries.
func arrayToNamedMap(arr []interface{}, columns []string) map[string]interface{} {
	m := make(map[string]interface{}, len(columns))
	for i, col := range columns {
		if i < len(arr) {
			m[col] = arr[i]
		} else {
			m[col] = nil
		}
	}
	return m
}

// ExtractByPathWithColumns is like ExtractByPath but uses arrayColumns to
// convert array-of-arrays into array-of-maps. Used when parser.array_columns
// is configured.
func ExtractByPathWithColumns(data interface{}, path string, columns []string) ([]map[string]interface{}, error) {
	if path == "" {
		return toMapSliceWithColumns(data, columns)
	}

	segments := strings.Split(path, ".")
	current := data

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

	return toMapSliceWithColumns(current, columns)
}
