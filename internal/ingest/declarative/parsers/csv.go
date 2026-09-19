package parsers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
)

// CSVParser parses CSV data into records.
//
// Options (via ParserConfig.CSVOptions):
//   - Delimiter: field separator (default ",")
//   - HasHeader: if true (default), the first row after skip_lines is used as field names
//   - SkipLines: number of leading lines to skip before parsing
//
// Each row (after header) becomes a map[string]interface{} with string values
// keyed by the header column names. If has_header is false, keys are "col_0",
// "col_1", etc.
type CSVParser struct{}

// Parse implements RecordParser.
func (p *CSVParser) Parse(data []byte, config ParserConfig) ([]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("csv parser: empty input")
	}

	opts := p.resolveOptions(config.CSVOptions)

	// Skip leading lines from raw input before CSV parsing, since skipped
	// lines may not be valid CSV (e.g., comment lines with different field counts).
	csvData := data
	if opts.SkipLines > 0 {
		csvData = skipRawLines(data, opts.SkipLines)
		if len(csvData) == 0 {
			return []map[string]interface{}{}, nil
		}
	}

	// Strip comment prefix from remaining lines (e.g., "#" from "#STN LAT LON").
	if opts.CommentPrefix != "" {
		csvData = stripCommentPrefix(csvData, opts.CommentPrefix)
	}

	// Collapse whitespace runs into single tabs for fixed-width/whitespace-delimited formats.
	if opts.CollapseWhitespace {
		csvData = collapseWhitespace(csvData)
		opts.Delimiter = "\t"
	}

	reader := csv.NewReader(bytes.NewReader(csvData))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	if len(opts.Delimiter) > 0 {
		reader.Comma = rune(opts.Delimiter[0])
	}

	// Read all rows.
	allRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv parser: read failed: %w", err)
	}

	if len(allRows) == 0 {
		return []map[string]interface{}{}, nil
	}

	var headers []string
	var dataRows [][]string

	if opts.HasHeader {
		headers = allRows[0]
		dataRows = allRows[1:]
	} else {
		// Generate synthetic headers: col_0, col_1, ...
		if len(allRows) > 0 {
			headers = make([]string, len(allRows[0]))
			for i := range headers {
				headers[i] = fmt.Sprintf("col_%d", i)
			}
		}
		dataRows = allRows
	}

	records := make([]map[string]interface{}, 0, len(dataRows))
	for _, row := range dataRows {
		record := make(map[string]interface{}, len(headers))
		for i, header := range headers {
			if i < len(row) {
				record[header] = row[i]
			} else {
				record[header] = ""
			}
		}
		records = append(records, record)
	}

	return truncateRecords(records, config.MaxRecords), nil
}

func (p *CSVParser) resolveOptions(opts *CSVOpts) CSVOpts {
	resolved := CSVOpts{
		Delimiter: ",",
		HasHeader: true,
		SkipLines: 0,
	}

	if opts != nil {
		if opts.Delimiter != "" {
			resolved.Delimiter = opts.Delimiter
		}
		// HasHeader defaults to true; explicitly copy the value.
		resolved.HasHeader = opts.HasHeader
		resolved.SkipLines = opts.SkipLines
		resolved.CollapseWhitespace = opts.CollapseWhitespace
		resolved.CommentPrefix = opts.CommentPrefix
	}

	return resolved
}

// wsRun matches one or more consecutive whitespace characters (spaces or tabs).
var wsRun = regexp.MustCompile(`[ \t]+`)

// collapseWhitespace normalizes whitespace-delimited text into tab-separated
// format suitable for csv.Reader. For each line it trims leading/trailing
// whitespace and collapses runs of spaces/tabs into a single tab character.
// Empty lines are skipped. This is O(n) over input size.
func collapseWhitespace(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var buf bytes.Buffer
	buf.Grow(len(data))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		collapsed := wsRun.ReplaceAll(trimmed, []byte("\t"))
		buf.Write(collapsed)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// stripCommentPrefix processes lines that start with a comment prefix (e.g., "#").
// The first line with the prefix has it stripped and is kept (typically the header
// with field names). Subsequent lines with the prefix are removed entirely
// (typically metadata like units rows in scientific data formats).
// Lines without the prefix are kept unchanged.
func stripCommentPrefix(data []byte, prefix string) []byte {
	pfx := []byte(prefix)
	lines := bytes.Split(data, []byte("\n"))
	var buf bytes.Buffer
	buf.Grow(len(data))
	headerSeen := false
	for _, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(trimmed, pfx) {
			if !headerSeen {
				// First comment line: strip prefix and keep as header.
				headerSeen = true
				buf.Write(bytes.TrimPrefix(trimmed, pfx))
				buf.WriteByte('\n')
			}
			// Subsequent comment lines: skip entirely.
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// skipRawLines skips the first n newline-delimited lines from raw bytes
// and returns the remainder. This is used to skip non-CSV header lines
// (comments, metadata) before feeding data to the CSV reader.
func skipRawLines(data []byte, n int) []byte {
	offset := 0
	for i := 0; i < n; i++ {
		idx := bytes.IndexByte(data[offset:], '\n')
		if idx < 0 {
			// Fewer lines than requested; return empty.
			return nil
		}
		offset += idx + 1
	}
	return data[offset:]
}
