package declarative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flightsCommercialEnvKeys mirrors the placeholder env vars set by
// TestValidateSourceDefinitions so the directory scan in
// TestFlightsCommercialSourceTopology can load every source without auth
// resolution errors. The flights_commercial sources themselves need no keys.
var flightsCommercialEnvKeys = []string{
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
}

// TestFlightsCommercialSourceTopology pins how the flights_commercial layer is
// sourced: adsb_theairtraffic_flights is the single enabled primary — a global
// single-poll feed that returns the whole globe in one request — and adsb.lol,
// adsb.fi, and airplanes.live are disabled drop-in failover siblings that crawl
// a hex grid one cell at a time.
//
// It guards the distinctive config each relies on, none of which the generic
// TestValidateSourceDefinitions checks: the browser headers theairtraffic needs
// to pass Cloudflare (without them every fetch is a silent 403), the records_path
// each provider's response uses, the global-vs-spatial split, and the invariant
// that exactly one source feeds the layer at a time.
func TestFlightsCommercialSourceTopology(t *testing.T) {
	for _, key := range flightsCommercialEnvKeys {
		t.Setenv(key, "test-placeholder")
	}

	dir := sourcesDir(t)
	loader := newTestLoader(t, false)

	load := func(t *testing.T, file string) *SourceDefinition {
		t.Helper()
		cs, err := loader.LoadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("load %s: %v", file, err)
		}
		return cs.Definition()
	}

	t.Run("theairtraffic_is_global_primary", func(t *testing.T) {
		def := load(t, "adsb_theairtraffic_flights.yaml")

		if !def.IsEnabled() {
			t.Error("theairtraffic must be the enabled primary for flights_commercial")
		}
		if def.LayerType != "flights_commercial" {
			t.Errorf("layer_type = %q, want flights_commercial", def.LayerType)
		}
		// A global feed returns every region per poll, so it must not declare a
		// spatial crawl — that would re-introduce the per-cell latency it replaces.
		if def.Transport.Spatial != nil {
			t.Error("theairtraffic is a global single-poll feed; it must NOT declare transport.spatial")
		}
		if def.Parser.RecordsPath != "aircraft" {
			t.Errorf("records_path = %q, want \"aircraft\" (TheAirTraffic global snapshot key)", def.Parser.RecordsPath)
		}
		// Cloudflare's edge rejects non-browser clients with 403; without these
		// headers every fetch fails silently and the layer goes dark.
		if ua := def.Transport.Headers["User-Agent"]; !strings.Contains(ua, "Mozilla") {
			t.Errorf("User-Agent header = %q, want a browser UA to pass Cloudflare", ua)
		}
		if def.Transport.Headers["Referer"] == "" {
			t.Error("missing Referer header required to pass Cloudflare")
		}
	})

	siblings := []string{
		"adsb_lol_flights.yaml",
		"adsb_fi_flights.yaml",
		"adsb_live_flights.yaml",
	}
	for _, file := range siblings {
		t.Run("failover_sibling/"+file, func(t *testing.T) {
			def := load(t, file)

			// Siblings stay disabled so only the primary feeds the layer; a peer
			// is enabled only to fail over when theairtraffic degrades.
			if def.IsEnabled() {
				t.Errorf("%s must be disabled (failover sibling; theairtraffic is the primary)", file)
			}
			if def.LayerType != "flights_commercial" {
				t.Errorf("layer_type = %q, want flights_commercial", def.LayerType)
			}
			// The siblings are spatial-crawl sources; the grid is what they trade
			// off against the primary's one-shot global coverage.
			if def.Transport.Spatial == nil {
				t.Errorf("%s is a spatial-crawl source; it must declare transport.spatial", file)
			}
			// All three return the readsb "ac" array (not TheAirTraffic's "aircraft").
			if def.Parser.RecordsPath != "ac" {
				t.Errorf("records_path = %q, want \"ac\" (readsb array key)", def.Parser.RecordsPath)
			}
		})
	}

	// Exactly one source may feed flights_commercial at a time, and it must be
	// theairtraffic. Catches an accidentally re-enabled sibling or open_sky.
	t.Run("single_enabled_source", func(t *testing.T) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read sources.d: %v", err)
		}

		var enabled []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			if strings.HasPrefix(strings.ToUpper(e.Name()), "TEMPLATE") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if isCommentOnlyFile(t, path) {
				continue
			}
			cs, err := loader.LoadFile(path)
			if err != nil {
				// Load/compile errors are owned by TestValidateSourceDefinitions;
				// a non-flights source failing here must not mask this assertion.
				continue
			}
			def := cs.Definition()
			if def.LayerType == "flights_commercial" && def.IsEnabled() {
				enabled = append(enabled, def.Name)
			}
		}

		if len(enabled) != 1 || enabled[0] != "adsb_theairtraffic_flights" {
			t.Errorf("enabled flights_commercial sources = %v, want exactly [adsb_theairtraffic_flights]", enabled)
		}
	})
}
