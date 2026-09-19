package declarative

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// simpleYAML returns a minimal declarative source YAML that maps records with
// id, name, lat, lon fields. An optional filter expression can be specified.
func simpleYAML(url, filter string) string {
	filterLine := ""
	if filter != "" {
		filterLine = fmt.Sprintf("filter: '%s'\n", filter)
	}
	return fmt.Sprintf(`schema_version: 1
name: test_dedup
source_type: test_dedup
layer_type: test_layer
display_name: "Test Dedup"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "items"
%sentity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`, url, filterLine)
}

// compileAndFetch compiles a YAML source definition, starts it against the given
// server, and returns the snapshot entities and observations.
func compileAndFetch(t *testing.T, yamlContent string) ([]*domain.Entity, []*domain.Observation) {
	t.Helper()

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "source.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return entities, observations
}

func TestProcessRecords_Dedup(t *testing.T) {
	tests := []struct {
		name             string
		records          []map[string]interface{}
		filter           string
		wantEntities     int
		wantObservations int // total observations (all records that pass filter)
		// checkLastWins verifies the last duplicate's data is kept for entities.
		// Key: expected ExternalID, Value: expected name.
		checkLastWins map[string]string
	}{
		{
			name: "no duplicates — all records unique",
			records: []map[string]interface{}{
				{"id": "sat-1", "name": "ISS", "lat": 10.0, "lon": 20.0},
				{"id": "sat-2", "name": "HST", "lat": 30.0, "lon": 40.0},
				{"id": "sat-3", "name": "TIANGONG", "lat": 50.0, "lon": 60.0},
			},
			wantEntities:     3,
			wantObservations: 3,
		},
		{
			name: "single duplicate — entity deduped, all observations kept",
			records: []map[string]interface{}{
				{"id": "sat-1", "name": "ISS-OLD", "lat": 10.0, "lon": 20.0},
				{"id": "sat-2", "name": "HST", "lat": 30.0, "lon": 40.0},
				{"id": "sat-1", "name": "ISS-NEW", "lat": 15.0, "lon": 25.0},
			},
			wantEntities:     2,
			wantObservations: 3,
			checkLastWins:    map[string]string{"sat-1": "ISS-NEW"},
		},
		{
			name: "triple duplicate — one entity, all observations kept",
			records: []map[string]interface{}{
				{"id": "sat-1", "name": "V1", "lat": 1.0, "lon": 2.0},
				{"id": "sat-1", "name": "V2", "lat": 3.0, "lon": 4.0},
				{"id": "sat-1", "name": "V3", "lat": 5.0, "lon": 6.0},
			},
			wantEntities:     1,
			wantObservations: 3,
			checkLastWins:    map[string]string{"sat-1": "V3"},
		},
		{
			name: "multiple distinct duplicates — TLE API scenario",
			records: []map[string]interface{}{
				{"id": "25544", "name": "ISS (ZARYA)", "lat": 10.0, "lon": 20.0},
				{"id": "37867", "name": "COSMOS 2476 (epoch1)", "lat": -5.0, "lon": -12.0},
				{"id": "39155", "name": "COSMOS 2485 (epoch1)", "lat": -30.0, "lon": 80.0},
				{"id": "20580", "name": "HST", "lat": -19.0, "lon": 142.0},
				{"id": "37867", "name": "COSMOS 2476 (epoch2)", "lat": -33.0, "lon": 82.0},
				{"id": "39155", "name": "COSMOS 2485 (epoch2)", "lat": -28.0, "lon": 81.0},
			},
			wantEntities:     4,
			wantObservations: 6,
			checkLastWins: map[string]string{
				"37867": "COSMOS 2476 (epoch2)",
				"39155": "COSMOS 2485 (epoch2)",
			},
		},
		{
			name: "all duplicates of same entity — one entity, all observations kept",
			records: []map[string]interface{}{
				{"id": "sat-1", "name": "A", "lat": 1.0, "lon": 1.0},
				{"id": "sat-1", "name": "B", "lat": 2.0, "lon": 2.0},
				{"id": "sat-1", "name": "C", "lat": 3.0, "lon": 3.0},
				{"id": "sat-1", "name": "D", "lat": 4.0, "lon": 4.0},
				{"id": "sat-1", "name": "E", "lat": 5.0, "lon": 5.0},
			},
			wantEntities:     1,
			wantObservations: 5,
			checkLastWins:    map[string]string{"sat-1": "E"},
		},
		{
			name:             "empty input — no entities produced",
			records:          []map[string]interface{}{},
			wantEntities:     0,
			wantObservations: 0,
		},
		{
			name: "single record — no dedup needed",
			records: []map[string]interface{}{
				{"id": "only-one", "name": "Solo", "lat": 42.0, "lon": -71.0},
			},
			wantEntities:     1,
			wantObservations: 1,
		},
		{
			name: "duplicates interleaved with unique — ordering preserved",
			records: []map[string]interface{}{
				{"id": "A", "name": "A1", "lat": 1.0, "lon": 1.0},
				{"id": "B", "name": "B1", "lat": 2.0, "lon": 2.0},
				{"id": "A", "name": "A2", "lat": 3.0, "lon": 3.0},
				{"id": "C", "name": "C1", "lat": 4.0, "lon": 4.0},
				{"id": "B", "name": "B2", "lat": 5.0, "lon": 5.0},
			},
			wantEntities:     3,
			wantObservations: 5,
			checkLastWins: map[string]string{
				"A": "A2",
				"B": "B2",
				"C": "C1",
			},
		},
		{
			name: "duplicates with filter — filtered records don't count",
			records: []map[string]interface{}{
				{"id": "A", "name": "A-good", "lat": 10.0, "lon": 20.0, "value": float64(5.0)},
				{"id": "B", "name": "B-filtered", "lat": 30.0, "lon": 40.0, "value": float64(1.0)},
				{"id": "A", "name": "A-updated", "lat": 15.0, "lon": 25.0, "value": float64(4.0)},
			},
			filter:           "record.value >= 3.0",
			wantEntities:     1,
			wantObservations: 2,
			checkLastWins:    map[string]string{"A": "A-updated"},
		},
		{
			name: "duplicate where first passes filter but second is filtered — first survives",
			records: []map[string]interface{}{
				{"id": "A", "name": "A-passes", "lat": 10.0, "lon": 20.0, "value": float64(5.0)},
				{"id": "A", "name": "A-filtered", "lat": 30.0, "lon": 40.0, "value": float64(1.0)},
			},
			filter:           "record.value >= 3.0",
			wantEntities:     1,
			wantObservations: 1,
			checkLastWins:    map[string]string{"A": "A-passes"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			response := map[string]interface{}{
				"items": toInterfaceSlice(tc.records),
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			yaml := simpleYAML(server.URL, tc.filter)
			entities, observations := compileAndFetch(t, yaml)

			if len(entities) != tc.wantEntities {
				t.Fatalf("len(entities) = %d, want %d", len(entities), tc.wantEntities)
			}
			if len(observations) != tc.wantObservations {
				t.Fatalf("len(observations) = %d, want %d", len(observations), tc.wantObservations)
			}

			// Verify every observation references a valid entity ID.
			entityIDs := make(map[string]bool, len(entities))
			for _, e := range entities {
				entityIDs[e.ID] = true
			}
			for i, obs := range observations {
				if !entityIDs[obs.EntityID] {
					t.Errorf("observations[%d].EntityID = %q does not match any entity", i, obs.EntityID)
				}
			}

			// Verify last-writer-wins for duplicate entity names.
			if tc.checkLastWins != nil {
				entityByExtID := make(map[string]string)
				for _, e := range entities {
					entityByExtID[e.ExternalID] = e.Name
				}
				for extID, wantName := range tc.checkLastWins {
					if got, ok := entityByExtID[extID]; !ok {
						t.Errorf("entity with ExternalID %q not found", extID)
					} else if got != wantName {
						t.Errorf("entity %q name = %q, want %q (last writer wins)", extID, got, wantName)
					}
				}
			}
		})
	}
}

func TestProcessRecords_Dedup_PaginatedEndToEnd(t *testing.T) {
	// Simulates the TLE API scenario: paginated responses where the same
	// satellite appears on different pages with different TLE data.
	page1 := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": "25544", "name": "ISS", "lat": 17.9, "lon": -124.2},
			map[string]interface{}{"id": "37867", "name": "COSMOS 2476 (old TLE)", "lat": -5.0, "lon": -12.8},
		},
	}
	page2 := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": "20580", "name": "HST", "lat": -19.3, "lon": 142.1},
			map[string]interface{}{"id": "37867", "name": "COSMOS 2476 (new TLE)", "lat": -33.4, "lon": 82.1},
		},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch requestCount {
		case 1:
			_ = json.NewEncoder(w).Encode(page1)
		case 2:
			_ = json.NewEncoder(w).Encode(page2)
		default:
			// Empty page stops pagination.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
		}
	}))
	defer server.Close()

	yamlContent := fmt.Sprintf(`schema_version: 1
name: test_paginated_dedup
source_type: test_paginated_dedup
layer_type: test_satellites
display_name: "Test Paginated Dedup"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
  pagination:
    type: page_number
    page_param: "page"
    size_param: "page_size"
    size: 100
    max_pages: 5
    stop_when: "size(records) == 0"
parser:
  format: json
  records_path: "items"
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: satellite
    scale: 1.0
  trail:
    color: "#00bfff"
  style:
    color: "#00bfff"
    point_size: 8
`, server.URL)

	entities, observations := compileAndFetch(t, yamlContent)

	// Should have 3 unique entities (ISS, HST, COSMOS 2476), not 4.
	if len(entities) != 3 {
		t.Fatalf("len(entities) = %d, want 3 (COSMOS 2476 deduped)", len(entities))
	}
	// All 4 observations are kept (one per record across both pages).
	if len(observations) != 4 {
		t.Fatalf("len(observations) = %d, want 4", len(observations))
	}

	// COSMOS 2476 entity should have the page 2 data (last writer wins).
	for _, e := range entities {
		if e.ExternalID == "37867" {
			if e.Name != "COSMOS 2476 (new TLE)" {
				t.Errorf("COSMOS 2476 name = %q, want %q", e.Name, "COSMOS 2476 (new TLE)")
			}
			return
		}
	}
	t.Error("COSMOS 2476 (external_id=37867) not found in entities")
}

func TestProcessRecords_Dedup_SliceIntegrity(t *testing.T) {
	// Verifies that after processing, entities are deduped and observations
	// retain all records. No nil entries in either slice.
	records := []map[string]interface{}{
		{"id": "A", "name": "A1", "lat": 1.0, "lon": 1.0},
		{"id": "B", "name": "B1", "lat": 2.0, "lon": 2.0},
		{"id": "C", "name": "C1", "lat": 3.0, "lon": 3.0},
		{"id": "A", "name": "A2", "lat": 4.0, "lon": 4.0},
		{"id": "B", "name": "B2", "lat": 5.0, "lon": 5.0},
		{"id": "D", "name": "D1", "lat": 6.0, "lon": 6.0},
		{"id": "A", "name": "A3", "lat": 7.0, "lon": 7.0},
	}

	response := map[string]interface{}{
		"items": toInterfaceSlice(records),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yaml := simpleYAML(server.URL, "")
	entities, observations := compileAndFetch(t, yaml)

	// 4 unique entity IDs: A, B, C, D
	if len(entities) != 4 {
		t.Fatalf("len(entities) = %d, want 4", len(entities))
	}
	// All 7 records produce observations.
	if len(observations) != 7 {
		t.Fatalf("len(observations) = %d, want 7", len(observations))
	}

	// No nil entries in either slice.
	for i, e := range entities {
		if e == nil {
			t.Errorf("entities[%d] is nil", i)
		}
	}
	for i, o := range observations {
		if o == nil {
			t.Errorf("observations[%d] is nil", i)
		}
	}

	// Verify the final entity values (last writer wins).
	extIDToName := make(map[string]string)
	for _, e := range entities {
		extIDToName[e.ExternalID] = e.Name
	}

	want := map[string]string{"A": "A3", "B": "B2", "C": "C1", "D": "D1"}
	for id, wantName := range want {
		if got := extIDToName[id]; got != wantName {
			t.Errorf("entity %q name = %q, want %q", id, got, wantName)
		}
	}
}

// toInterfaceSlice converts a slice of maps to []interface{} for JSON encoding.
func toInterfaceSlice(records []map[string]interface{}) []interface{} {
	result := make([]interface{}, len(records))
	for i, r := range records {
		result[i] = r
	}
	return result
}
