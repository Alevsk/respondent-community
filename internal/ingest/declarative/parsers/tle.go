package parsers

import (
	"fmt"
	"strings"
)

// TLEParser parses Two-Line Element Set (TLE) data into records.
//
// TLE format (3-line variant from CelesTrak):
//
//	Line 0: Name (up to 24 chars, trimmed)
//	Line 1: 1 NNNNN... (69 chars, starts with "1")
//	Line 2: 2 NNNNN... (69 chars, starts with "2")
//
// Each TLE entry produces a record with fields:
//   - name:     trimmed satellite name
//   - line1:    full line 1 text
//   - line2:    full line 2 text
//   - norad_id: NORAD catalog number extracted from line 1 columns 3-7
//
// Blank lines between entries are tolerated and skipped.
type TLEParser struct{}

// Parse implements RecordParser.
func (p *TLEParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("tle parser: empty input")
	}

	lines := strings.Split(string(data), "\n")

	// Filter out blank lines and collect non-empty lines.
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r \t")
		if trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}

	if len(nonEmpty) == 0 {
		return []map[string]interface{}{}, nil
	}

	// TLE entries are groups of 3 lines.
	if len(nonEmpty)%3 != 0 {
		return nil, fmt.Errorf("tle parser: line count %d is not a multiple of 3", len(nonEmpty))
	}

	records := make([]map[string]interface{}, 0, len(nonEmpty)/3)
	for i := 0; i+2 < len(nonEmpty); i += 3 {
		nameLine := nonEmpty[i]
		line1 := nonEmpty[i+1]
		line2 := nonEmpty[i+2]

		// Validate line 1 starts with "1" and line 2 starts with "2".
		if len(line1) < 8 || line1[0] != '1' {
			return nil, fmt.Errorf("tle parser: expected line 1 at index %d, got %q", i+1, truncateStr(line1, 20))
		}
		if len(line2) < 8 || line2[0] != '2' {
			return nil, fmt.Errorf("tle parser: expected line 2 at index %d, got %q", i+2, truncateStr(line2, 20))
		}

		// Extract NORAD ID from line 1, columns 3-7 (0-indexed: 2-6).
		noradID := strings.TrimSpace(line1[2:7])

		record := map[string]interface{}{
			"name":     strings.TrimSpace(nameLine),
			"line1":    line1,
			"line2":    line2,
			"norad_id": noradID,
		}
		records = append(records, record)
	}

	return truncateRecords(records, config.MaxRecords), nil
}

// truncateStr truncates a string to maxLen characters for error messages.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
