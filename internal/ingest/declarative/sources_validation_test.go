package declarative

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sourcesDir resolves the path to the sources.d/ directory relative to this test file.
func sourcesDir(t *testing.T) string {
	t.Helper()
	// Get the directory of this test file, then navigate up to the project root.
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path via runtime.Caller")
	}
	// filename = .../internal/ingest/declarative/sources_validation_test.go
	// project root = 4 levels up
	dir := filepath.Dir(filename)
	root := filepath.Join(dir, "..", "..", "..", "sources.d")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("failed to resolve sources.d path: %v", err)
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Skipf("sources.d directory not found at %s; skipping", abs)
	}
	return abs
}

// isCommentOnlyFile checks if a YAML file contains only comments and blank lines
// (no actual YAML content to parse). These are documentation-only files.
func isCommentOnlyFile(t *testing.T, path string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return false
	}
	return true
}

// TestValidateSourceDefinitions loads all YAML source definitions from sources.d/,
// verifies they parse without error, all CEL expressions compile, source names are
// unique, and layer types are set.
func TestValidateSourceDefinitions(t *testing.T) {
	// Set dummy values for all env vars referenced by source auth configs.
	// The test validates parsing and CEL compilation, not API connectivity,
	// so placeholder values are sufficient.
	for _, key := range []string{
		"RESPONDENT_CLOUDFLARE_RADAR_TOKEN",
		"RESPONDENT_OPENAQ_API_KEY",
		"RESPONDENT_PURPLEAIR_API_KEY",
		"RESPONDENT_UKRAINE_ALARM_TOKEN",
		"RESPONDENT_NASA_FIRMS_MAP_KEY",
		"RESPONDENT_AISSTREAM_APY_KEY",
		"RESPONDENT_ACLED_EMAIL",
		"RESPONDENT_ACLED_PASSWORD",
		"RESPONDENT_APRS_FI_API_KEY",
		"RESPONDENT_MESHTASTIC_USER",
		"RESPONDENT_MESHTASTIC_PASS",
	} {
		t.Setenv(key, "test-placeholder")
	}

	dir := sourcesDir(t)

	loader := newTestLoader(t, false)

	// Read all YAML/YML files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read sources.d directory: %v", err)
	}

	var (
		validFiles   []string
		skippedFiles []string
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		// Skip template files (contain placeholder values, not meant to parse).
		if strings.HasPrefix(strings.ToUpper(entry.Name()), "TEMPLATE") {
			skippedFiles = append(skippedFiles, entry.Name())
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if isCommentOnlyFile(t, path) {
			skippedFiles = append(skippedFiles, entry.Name())
			continue
		}
		validFiles = append(validFiles, entry.Name())
	}

	t.Logf("found %d parseable YAML files, %d comment-only files", len(validFiles), len(skippedFiles))
	for _, name := range skippedFiles {
		t.Logf("  skipped (comment-only): %s", name)
	}

	if len(validFiles) == 0 {
		t.Fatal("no parseable YAML source definitions found in sources.d/")
	}

	// Load and validate each source definition
	sourceNames := make(map[string]string) // name -> filename
	sourceTypes := make(map[string]string) // source_type -> filename
	layerTypes := make(map[string]string)  // layer_type -> filename
	var loadedSources []*CompiledSource

	for _, name := range validFiles {
		path := filepath.Join(dir, name)
		t.Run(name, func(t *testing.T) {
			cs, err := loader.LoadFile(path)
			if err != nil {
				t.Fatalf("failed to load %s: %v", name, err)
			}

			def := cs.Definition()

			// Verify required fields are populated
			if def.Name == "" {
				t.Error("source name is empty")
			}
			if def.SourceType == "" {
				t.Error("source_type is empty")
			}
			if def.LayerType == "" {
				t.Error("layer_type is empty")
			}
			if def.DisplayName == "" {
				t.Error("display_name is empty")
			}

			// Verify CEL programs compiled (non-nil)
			if cs.EntityID() == nil {
				t.Error("entity.external_id CEL program is nil")
			}
			if cs.EntityName() == nil {
				t.Error("entity.name CEL program is nil")
			}
			// Lat/lon programs are nil for global_indicator entities (no coordinates)
			if def.EntityType != "global_indicator" {
				if cs.ObservationLat() == nil {
					t.Error("observation.latitude CEL program is nil")
				}
				if cs.ObservationLon() == nil {
					t.Error("observation.longitude CEL program is nil")
				}
			}
			if cs.ObservationTS() == nil {
				t.Error("observation.timestamp CEL program is nil")
			}

			loadedSources = append(loadedSources, cs)
		})
	}

	// Check uniqueness across all loaded sources
	t.Run("uniqueness", func(t *testing.T) {
		for _, cs := range loadedSources {
			def := cs.Definition()

			if existing, ok := sourceNames[def.Name]; ok {
				t.Errorf("duplicate source name %q: found in %s and %s", def.Name, existing, def.Name)
			}
			sourceNames[def.Name] = def.Name

			if existing, ok := sourceTypes[def.SourceType]; ok {
				t.Errorf("duplicate source_type %q: found in %s and %s", def.SourceType, existing, def.Name)
			}
			sourceTypes[def.SourceType] = def.Name

			// layer_type uniqueness is NOT enforced — multiple sources can feed the
			// same layer (e.g. open_sky_flights and adsb_lol_flights both target
			// flights_commercial). Only one would typically be enabled at a time.
			layerTypes[def.LayerType] = def.Name
		}
	})

	// Summary
	t.Logf("validated %d source definitions successfully", len(loadedSources))
	for _, cs := range loadedSources {
		def := cs.Definition()
		t.Logf("  [OK] %s (type=%s, layer=%s, format=%s)",
			def.Name, def.SourceType, def.LayerType, def.Parser.Format)
	}
}
