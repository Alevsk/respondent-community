package declarative

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxLookupEntriesPerTable is the maximum number of entries allowed in a single lookup table.
const maxLookupEntriesPerTable = 100000

// maxLookupTablesPerSource is the maximum number of lookup tables allowed per source definition.
const maxLookupTablesPerSource = 10

// LookupTable holds an indexed lookup table loaded from YAML or file.
type LookupTable struct {
	Name     string
	KeyField string
	Data     map[string]map[string]interface{} // key -> field -> value
}

// LoadLookupTables loads and indexes all lookup tables for a source definition.
// sourcesDir is the path to the sources.d/ directory (for resolving file references).
func LoadLookupTables(specs []LookupTableSpec, sourcesDir string) (map[string]*LookupTable, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	if len(specs) > maxLookupTablesPerSource {
		return nil, fmt.Errorf("too many lookup tables: %d exceeds maximum of %d", len(specs), maxLookupTablesPerSource)
	}

	tables := make(map[string]*LookupTable, len(specs))
	for i, spec := range specs {
		// Validate mutual exclusivity
		hasEntries := len(spec.Entries) > 0
		hasFile := spec.File != ""
		if hasEntries == hasFile {
			return nil, fmt.Errorf("lookup_tables[%d] %q: exactly one of 'entries' or 'file' must be set", i, spec.Name)
		}

		// Validate format required when file is set
		if hasFile && spec.Format == "" {
			return nil, fmt.Errorf("lookup_tables[%d] %q: 'format' is required when 'file' is set", i, spec.Name)
		}

		var entries []map[string]interface{}
		if hasFile {
			var err error
			entries, err = loadLookupFile(spec.File, spec.Format, sourcesDir)
			if err != nil {
				return nil, fmt.Errorf("lookup_tables[%d] %q: %w", i, spec.Name, err)
			}
		} else {
			entries = spec.Entries
		}

		// Validate memory limits
		if len(entries) > maxLookupEntriesPerTable {
			return nil, fmt.Errorf("lookup_tables[%d] %q: %d entries exceeds maximum of %d", i, spec.Name, len(entries), maxLookupEntriesPerTable)
		}

		table, err := indexLookupTable(spec.Name, spec.KeyField, entries)
		if err != nil {
			return nil, fmt.Errorf("lookup_tables[%d] %q: %w", i, spec.Name, err)
		}

		if _, exists := tables[spec.Name]; exists {
			return nil, fmt.Errorf("lookup_tables[%d]: duplicate table name %q", i, spec.Name)
		}
		tables[spec.Name] = table
	}

	return tables, nil
}

// indexLookupTable builds the indexed map from a list of entries.
func indexLookupTable(name, keyField string, entries []map[string]interface{}) (*LookupTable, error) {
	data := make(map[string]map[string]interface{}, len(entries))
	for i, entry := range entries {
		keyVal, ok := entry[keyField]
		if !ok {
			return nil, fmt.Errorf("entry[%d] missing key field %q", i, keyField)
		}
		key := fmt.Sprintf("%v", keyVal)
		data[key] = entry
	}
	return &LookupTable{Name: name, KeyField: keyField, Data: data}, nil
}

// loadLookupFile reads a lookup table from an external file.
func loadLookupFile(filePath, format, sourcesDir string) ([]map[string]interface{}, error) {
	// Security: reject absolute paths and traversal
	if filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("absolute paths not allowed: %q", filePath)
	}
	cleaned := filepath.Clean(filePath)
	if strings.HasPrefix(cleaned, "..") {
		return nil, fmt.Errorf("path traversal not allowed: %q", filePath)
	}

	fullPath := filepath.Join(sourcesDir, cleaned)

	// Security: resolve symlinks and verify the real path is still within sourcesDir.
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return nil, fmt.Errorf("resolve file path %q: %w", fullPath, err)
	}
	realSourcesDir, err := filepath.EvalSymlinks(sourcesDir)
	if err != nil {
		return nil, fmt.Errorf("resolve sources dir %q: %w", sourcesDir, err)
	}
	if !strings.HasPrefix(realPath, realSourcesDir+string(filepath.Separator)) && realPath != realSourcesDir {
		return nil, fmt.Errorf("path %q resolves outside sources directory", filePath)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", fullPath, err)
	}

	switch format {
	case "json":
		return parseJSONLookup(data)
	case "csv":
		return parseCSVLookup(data)
	default:
		return nil, fmt.Errorf("unsupported lookup file format %q", format)
	}
}

// parseJSONLookup parses a JSON array of objects.
func parseJSONLookup(data []byte) ([]map[string]interface{}, error) {
	var entries []map[string]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return entries, nil
}

// parseCSVLookup parses a CSV file with headers into a list of maps.
func parseCSVLookup(data []byte) ([]map[string]interface{}, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	headers := records[0]
	entries := make([]map[string]interface{}, 0, len(records)-1)
	for _, row := range records[1:] {
		entry := make(map[string]interface{}, len(headers))
		for j, header := range headers {
			if j < len(row) {
				entry[header] = coerceCSVValue(row[j])
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// coerceCSVValue attempts to parse a CSV string value into a more specific Go type.
// Order: int64 -> float64 -> bool -> string (fallback).
// This ensures CSV lookup tables behave consistently with JSON lookup tables
// where numbers and booleans are preserved as their native types.
func coerceCSVValue(s string) interface{} {
	// Try integer first (avoids matching "3.14" as int).
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Try float.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Try bool.
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}
