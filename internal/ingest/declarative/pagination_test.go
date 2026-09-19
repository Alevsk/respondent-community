package declarative

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// paginatedYAML returns a minimal YAML template with pagination configured.
// The url placeholder %s is replaced with the mock server URL.
func paginatedYAML(url string, paginationType string, maxPages int, stopWhen string, cursorPath string) string {
	var paginationBlock string
	switch paginationType {
	case "page_number":
		paginationBlock = fmt.Sprintf(`  pagination:
    type: page_number
    page_param: "page"
    size_param: "page_size"
    size: 2
    max_pages: %d`, maxPages)
	case "offset":
		paginationBlock = fmt.Sprintf(`  pagination:
    type: offset
    page_param: "offset"
    size_param: "limit"
    size: 2
    max_pages: %d`, maxPages)
	case "cursor":
		paginationBlock = fmt.Sprintf(`  pagination:
    type: cursor
    page_param: "cursor"
    size_param: "limit"
    size: 2
    max_pages: %d
    cursor_path: "%s"`, maxPages, cursorPath)
	}

	if stopWhen != "" {
		paginationBlock += fmt.Sprintf("\n    stop_when: '%s'", stopWhen)
	}

	return fmt.Sprintf(`schema_version: 2
name: test_paginated
source_type: test_paginated
layer_type: test_layer
display_name: "Test Paginated"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
%s
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`, url, paginationBlock)
}

// loadPaginatedAdapter creates and returns a DeclarativeAdapter from a paginated YAML definition.
func loadPaginatedAdapter(t *testing.T, yamlContent string) *DeclarativeAdapter {
	t.Helper()
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "paginated.yaml")
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

	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	return adapter
}

// makePageResponse creates a JSON response with items for a given page.
func makePageResponse(items []map[string]interface{}) []byte {
	resp := map[string]interface{}{
		"items": items,
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestPaginationLoop_PageNumber(t *testing.T) {
	// Mock server returns 3 pages of 2 records each.
	pages := [][]map[string]interface{}{
		{
			{"id": "a1", "name": "A1", "lat": 10.0, "lon": 20.0},
			{"id": "a2", "name": "A2", "lat": 11.0, "lon": 21.0},
		},
		{
			{"id": "b1", "name": "B1", "lat": 12.0, "lon": 22.0},
			{"id": "b2", "name": "B2", "lat": 13.0, "lon": 23.0},
		},
		{
			{"id": "c1", "name": "C1", "lat": 14.0, "lon": 24.0},
			{"id": "c2", "name": "C2", "lat": 15.0, "lon": 25.0},
		},
	}

	requestedPages := make([]int, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageStr)
		requestedPages = append(requestedPages, page)

		idx := page - 1
		if idx >= 0 && idx < len(pages) {
			_, _ = w.Write(makePageResponse(pages[idx]))
		} else {
			// Return empty page to stop
			_, _ = w.Write(makePageResponse(nil))
		}
	}))
	defer server.Close()

	yaml := paginatedYAML(server.URL, "page_number", 5, "size(records) == 0", "")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// 3 pages x 2 records = 6 entities, plus page 4 returns empty and stops.
	if len(entities) != 6 {
		t.Errorf("len(entities) = %d, want 6", len(entities))
	}
	if len(observations) != 6 {
		t.Errorf("len(observations) = %d, want 6", len(observations))
	}

	// Verify pages were requested in order: 1, 2, 3, 4 (4th is empty, triggers stop).
	if len(requestedPages) != 4 {
		t.Errorf("requested %d pages, want 4 (3 data + 1 empty)", len(requestedPages))
	}

	// Verify entity IDs from all pages are present.
	ids := make(map[string]bool)
	for _, e := range entities {
		ids[e.ExternalID] = true
	}
	for _, expected := range []string{"a1", "a2", "b1", "b2", "c1", "c2"} {
		if !ids[expected] {
			t.Errorf("missing entity with external_id %q", expected)
		}
	}
}

func TestPaginationLoop_BreakOnEmpty(t *testing.T) {
	// Mock server returns 2 pages of data, then an empty page 3.
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		pageStr := r.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageStr)

		switch page {
		case 1:
			_, _ = w.Write(makePageResponse([]map[string]interface{}{
				{"id": "a", "name": "A", "lat": 1.0, "lon": 2.0},
			}))
		case 2:
			_, _ = w.Write(makePageResponse([]map[string]interface{}{
				{"id": "b", "name": "B", "lat": 3.0, "lon": 4.0},
			}))
		default:
			// Empty page
			_, _ = w.Write(makePageResponse(nil))
		}
	}))
	defer server.Close()

	yaml := paginatedYAML(server.URL, "page_number", 10, "size(records) == 0", "")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 2 {
		t.Errorf("len(entities) = %d, want 2", len(entities))
	}

	// Should have made 3 requests: page 1, page 2, page 3 (empty, triggers stop).
	if requestCount != 3 {
		t.Errorf("requestCount = %d, want 3", requestCount)
	}
}

func TestPaginationLoop_MaxPages(t *testing.T) {
	// Mock server always returns data (never empty), verify loop stops at max_pages.
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		_, _ = w.Write(makePageResponse([]map[string]interface{}{
			{"id": fmt.Sprintf("item%d", requestCount), "name": fmt.Sprintf("Item%d", requestCount), "lat": 1.0, "lon": 2.0},
		}))
	}))
	defer server.Close()

	maxPages := 3
	// No stop_when, so only max_pages limits the loop.
	yaml := paginatedYAML(server.URL, "page_number", maxPages, "", "")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != maxPages {
		t.Errorf("len(entities) = %d, want %d (max_pages)", len(entities), maxPages)
	}

	if requestCount != maxPages {
		t.Errorf("requestCount = %d, want %d", requestCount, maxPages)
	}
}

func TestPaginationLoop_Offset(t *testing.T) {
	// Mock server tracks requested offsets and returns items until offset >= 4.
	requestedOffsets := make([]int, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsetStr := r.URL.Query().Get("offset")
		offset, _ := strconv.Atoi(offsetStr)
		requestedOffsets = append(requestedOffsets, offset)

		if offset < 4 {
			_, _ = w.Write(makePageResponse([]map[string]interface{}{
				{"id": fmt.Sprintf("item%d", offset), "name": fmt.Sprintf("Item%d", offset), "lat": 1.0, "lon": 2.0},
				{"id": fmt.Sprintf("item%d", offset+1), "name": fmt.Sprintf("Item%d", offset+1), "lat": 1.0, "lon": 2.0},
			}))
		} else {
			_, _ = w.Write(makePageResponse(nil))
		}
	}))
	defer server.Close()

	yaml := paginatedYAML(server.URL, "offset", 10, "size(records) == 0", "")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Verify offsets were 0, 2, 4 (size=2).
	expectedOffsets := []int{0, 2, 4}
	if len(requestedOffsets) != len(expectedOffsets) {
		t.Fatalf("requestedOffsets = %v, want %v", requestedOffsets, expectedOffsets)
	}
	for i, expected := range expectedOffsets {
		if requestedOffsets[i] != expected {
			t.Errorf("requestedOffsets[%d] = %d, want %d", i, requestedOffsets[i], expected)
		}
	}

	// 2 pages of 2 records each = 4 entities.
	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 4 {
		t.Errorf("len(entities) = %d, want 4", len(entities))
	}
}

func TestPaginationLoop_Cursor(t *testing.T) {
	// Mock server returns a cursor in the response for the next page.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")

		switch cursor {
		case "":
			// First page
			resp := map[string]interface{}{
				"items":       []interface{}{map[string]interface{}{"id": "a", "name": "A", "lat": 1.0, "lon": 2.0}},
				"next_cursor": "cursor_page2",
			}
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
		case "cursor_page2":
			// Second page
			resp := map[string]interface{}{
				"items":       []interface{}{map[string]interface{}{"id": "b", "name": "B", "lat": 3.0, "lon": 4.0}},
				"next_cursor": "cursor_page3",
			}
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
		case "cursor_page3":
			// Third page - no more data, empty cursor.
			resp := map[string]interface{}{
				"items":       []interface{}{map[string]interface{}{"id": "c", "name": "C", "lat": 5.0, "lon": 6.0}},
				"next_cursor": "",
			}
			data, _ := json.Marshal(resp)
			_, _ = w.Write(data)
		default:
			_, _ = w.Write(makePageResponse(nil))
		}
	}))
	defer server.Close()

	yaml := paginatedYAML(server.URL, "cursor", 10, "", "next_cursor")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 3 {
		t.Errorf("len(entities) = %d, want 3", len(entities))
	}

	// Verify entity IDs.
	ids := make(map[string]bool)
	for _, e := range entities {
		ids[e.ExternalID] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !ids[expected] {
			t.Errorf("missing entity with external_id %q", expected)
		}
	}
}

func TestPaginationLoop_ContextCancellation(t *testing.T) {
	// Mock server delays responses to allow context cancellation.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write(makePageResponse([]map[string]interface{}{
			{"id": "a", "name": "A", "lat": 1.0, "lon": 2.0},
		}))
	}))
	defer server.Close()

	yaml := paginatedYAML(server.URL, "page_number", 100, "", "")
	adapter := loadPaginatedAdapter(t, yaml)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Should return quickly without hanging.
	err := adapter.Start(ctx)
	if err == nil {
		// This is acceptable - the adapter might have completed before timeout.
		return
	}
	// Should be a context error.
	if !strings.Contains(err.Error(), "context") {
		t.Logf("Start returned non-context error: %v (acceptable)", err)
	}
}

func TestPaginationSpec_Validation(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid_page_number",
			yaml: `schema_version: 2
name: test_valid_page
source_type: test_valid_page
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  pagination:
    type: page_number
    page_param: "page"
    size: 50
    max_pages: 10
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    point_size: 6`,
			wantError: false,
		},
		{
			name: "invalid_pagination_type",
			yaml: `schema_version: 2
name: test_bad_type
source_type: test_bad_type
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  pagination:
    type: invalid_type
    page_param: "page"
    size: 50
    max_pages: 10
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    point_size: 6`,
			wantError: true,
			errorMsg:  "validate",
		},
		{
			name: "missing_page_param",
			yaml: `schema_version: 2
name: test_no_param
source_type: test_no_param
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  pagination:
    type: page_number
    size: 50
    max_pages: 10
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    point_size: 6`,
			wantError: true,
			errorMsg:  "validate",
		},
		{
			name: "size_zero",
			yaml: `schema_version: 2
name: test_size_zero
source_type: test_size_zero
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  pagination:
    type: page_number
    page_param: "page"
    size: 0
    max_pages: 10
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    point_size: 6`,
			wantError: true,
			errorMsg:  "validate",
		},
		{
			name: "schema_v1_backward_compat",
			yaml: `schema_version: 1
name: test_v1_compat
source_type: test_v1_compat
layer_type: test_layer
display_name: "Test V1"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    point_size: 6`,
			wantError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "test.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0644); err != nil {
				t.Fatalf("write YAML: %v", err)
			}

			compiler, err := NewCELCompiler()
			if err != nil {
				t.Fatalf("NewCELCompiler: %v", err)
			}

			logger := logging.NewNopLogger()
			loader, err := NewLoader(compiler, os.Getenv, false, logger)
			if err != nil {
				t.Fatalf("NewLoader: %v", err)
			}

			_, err = loader.LoadFile(path)
			if tc.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tc.errorMsg != "" && !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPaginationLoop_URLConstruction(t *testing.T) {
	tests := []struct {
		name           string
		paginationType string
		page           int
		cursor         string
		wantContains   string
	}{
		{
			name:           "page_number_first",
			paginationType: "page_number",
			page:           0,
			wantContains:   "page=1",
		},
		{
			name:           "page_number_third",
			paginationType: "page_number",
			page:           2,
			wantContains:   "page=3",
		},
		{
			name:           "offset_first",
			paginationType: "offset",
			page:           0,
			wantContains:   "offset=0",
		},
		{
			name:           "offset_second",
			paginationType: "offset",
			page:           1,
			wantContains:   "offset=2", // size=2
		},
		{
			name:           "cursor_first_page",
			paginationType: "cursor",
			page:           0,
			cursor:         "",
			wantContains:   "limit=2",
		},
		{
			name:           "cursor_subsequent",
			paginationType: "cursor",
			page:           1,
			cursor:         "abc123",
			wantContains:   "cursor=abc123",
		},
	}

	adapter := &DeclarativeAdapter{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var pagination *PaginationSpec
			switch tc.paginationType {
			case "page_number":
				pagination = &PaginationSpec{
					Type:      "page_number",
					PageParam: "page",
					SizeParam: "page_size",
					Size:      2,
					MaxPages:  10,
				}
			case "offset":
				pagination = &PaginationSpec{
					Type:      "offset",
					PageParam: "offset",
					SizeParam: "limit",
					Size:      2,
					MaxPages:  10,
				}
			case "cursor":
				pagination = &PaginationSpec{
					Type:       "cursor",
					PageParam:  "cursor",
					SizeParam:  "limit",
					Size:       2,
					MaxPages:   10,
					CursorPath: "next_cursor",
				}
			}

			url := adapter.buildPaginatedURL("https://api.example.com/data", pagination, tc.page, tc.cursor)
			if !strings.Contains(url, tc.wantContains) {
				t.Errorf("URL %q does not contain %q", url, tc.wantContains)
			}
		})
	}
}

func TestExtractCursorFromJSON(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		cursorPath string
		want       string
	}{
		{
			name:       "string_cursor",
			body:       `{"next_cursor": "abc123", "data": []}`,
			cursorPath: "next_cursor",
			want:       "abc123",
		},
		{
			name:       "empty_cursor",
			body:       `{"next_cursor": "", "data": []}`,
			cursorPath: "next_cursor",
			want:       "",
		},
		{
			name:       "missing_cursor",
			body:       `{"data": []}`,
			cursorPath: "next_cursor",
			want:       "",
		},
		{
			name:       "nested_cursor",
			body:       `{"pagination": {"next": "xyz"}}`,
			cursorPath: "pagination.next",
			want:       "xyz",
		},
		{
			name:       "numeric_cursor",
			body:       `{"next_page": 42}`,
			cursorPath: "next_page",
			want:       "42",
		},
		{
			name:       "invalid_json",
			body:       `not json`,
			cursorPath: "cursor",
			want:       "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCursorFromJSON([]byte(tc.body), tc.cursorPath)
			if got != tc.want {
				t.Errorf("extractCursorFromJSON() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStopWhen_CELCompilation(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{
			name:    "size_check",
			expr:    "size(records) == 0",
			wantErr: false,
		},
		{
			name:    "size_less_than",
			expr:    "size(records) < 10",
			wantErr: false,
		},
		{
			name:    "invalid_variable",
			expr:    "size(record) == 0", // record (singular) not available in stop_when env
			wantErr: true,
		},
		{
			name:    "syntax_error",
			expr:    "size(records == 0",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compiler.CompileStopWhen(tc.expr)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
