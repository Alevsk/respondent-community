package parsers

import (
	"encoding/json"
	"fmt"
)

// JSONTableParser parses array-of-arrays JSON where the first row is headers
// and subsequent rows are positional data arrays.
//
// Input:  [["time_tag","Kp"], ["2026-03-15 21:00:00","3.33"], ...]
// Output: [{"time_tag":"2026-03-15 21:00:00","Kp":"3.33"}, ...]
//
// This is a standalone parser (format: "json_table") that handles the entire
// response as a table. Unlike the JSONParser's ArrayOfArrays mode, it does
// not support records_path or other JSON reshaping transforms.
type JSONTableParser struct{}

// Parse implements RecordParser.
func (p *JSONTableParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("json_table: empty input")
	}

	var raw [][]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("json_table: %w", err)
	}

	if len(raw) < 2 {
		return nil, fmt.Errorf("json_table: need at least header + 1 data row, got %d rows", len(raw))
	}

	// Extract headers from row 0.
	headers := make([]string, len(raw[0]))
	for i, h := range raw[0] {
		s, ok := h.(string)
		if !ok {
			return nil, fmt.Errorf("json_table: header[%d] is not a string: %v", i, h)
		}
		headers[i] = s
	}

	// Convert data rows to maps.
	records := make([]map[string]interface{}, 0, len(raw)-1)
	for _, row := range raw[1:] {
		record := make(map[string]interface{}, len(headers))
		for j, val := range row {
			if j < len(headers) {
				record[headers[j]] = val
			}
		}
		records = append(records, record)
	}

	return truncateRecords(records, config.MaxRecords), nil
}
