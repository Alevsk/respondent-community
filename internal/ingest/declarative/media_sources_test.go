package declarative

import (
	"encoding/json"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// These tests run the SHIPPED source definitions against captured catalog
// responses, through the real parser, the real CEL filter and the real mapping.
// They are not YAML unmarshalling checks: a broken filter expression or a
// coordinate coerced to zero fails here.

func mediaFixture(t *testing.T, name string) []byte {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "testdata", "media", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return body
}

// mapFixture loads a shipped source definition and runs one captured response
// through it, returning the entities and observations it produces.
func mapFixture(t *testing.T, sourceFile, fixture string) ([]*domain.Entity, []*domain.Observation) {
	t.Helper()
	cs, err := newTestLoader(t, false).LoadFile(filepath.Join(sourcesDir(t), sourceFile))
	if err != nil {
		t.Fatalf("load %s: %v", sourceFile, err)
	}
	adapter, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), testHTTPClient())
	if err != nil {
		t.Fatalf("adapter for %s: %v", sourceFile, err)
	}
	records, err := adapter.parseBody(cs.Definition(), mediaFixture(t, fixture))
	if err != nil {
		t.Fatalf("parse %s: %v", fixture, err)
	}
	entities, observations := adapter.processRecords(cs, records)
	return entities, observations
}

func entityByID(entities []*domain.Entity, externalID string) *domain.Entity {
	for _, e := range entities {
		if e.ExternalID == externalID {
			return e
		}
	}
	return nil
}

func observationFor(t *testing.T, entities []*domain.Entity, observations []*domain.Observation, externalID string) *domain.Observation {
	t.Helper()
	for i, e := range entities {
		if e.ExternalID == externalID && i < len(observations) {
			return observations[i]
		}
	}
	t.Fatalf("no observation for %q", externalID)
	return nil
}

func TestAustinCameraCatalogMapping(t *testing.T) {
	entities, observations := mapFixture(t, "cctv_austin.yaml", "austin_cameras.json")

	// Switched-off cameras, cameras with a missing or null location, cameras on
	// an undeclared host and cameras with no image URL are all dropped.
	got := make([]string, 0, len(entities))
	for _, e := range entities {
		got = append(got, e.ExternalID)
	}
	if len(entities) != 2 {
		t.Fatalf("entities = %v, want exactly austin_1 and austin_2", got)
	}

	cam := entityByID(entities, "austin_1")
	if cam == nil {
		t.Fatalf("austin_1 missing from %v", got)
	}
	if cam.Name != "830 BLK W RUNDBERG LN (Little Walnut Creek Library, HEB)" {
		t.Errorf("name = %q", cam.Name)
	}
	if cam.Metadata["snapshot_url"] != "https://cctv.austinmobility.io/image/1.jpg" {
		t.Errorf("snapshot_url = %q", cam.Metadata["snapshot_url"])
	}
	if cam.Metadata["attribution"] == "" {
		t.Error("attribution metadata is empty; the media section has nothing to credit")
	}

	obs := observationFor(t, entities, observations, "austin_1")
	if obs.Position == nil || obs.Position.Lat != 30.363686 || obs.Position.Lon != -97.698158 {
		t.Errorf("position = %+v, want lat 30.363686 lon -97.698158", obs.Position)
	}

	// A camera at 0,0 is a real coordinate, not a missing one.
	nullIsland := entityByID(entities, "austin_2")
	if nullIsland == nil {
		t.Fatal("a camera at 0,0 was dropped; zero is a valid coordinate")
	}
	zeroObs := observationFor(t, entities, observations, "austin_2")
	if zeroObs.Position == nil || zeroObs.Position.Lat != 0 || zeroObs.Position.Lon != 0 {
		t.Errorf("0,0 position = %+v", zeroObs.Position)
	}
}

func TestAustinContentHashTracksImageURLChanges(t *testing.T) {
	_, before := mapFixture(t, "cctv_austin.yaml", "austin_cameras.json")
	_, after := mapFixture(t, "cctv_austin.yaml", "austin_cameras_updated.json")

	if before[0].ContentHash == "" {
		t.Fatal("dedupe mode produced an empty content hash")
	}
	if before[0].ContentHash == after[0].ContentHash {
		t.Error("a changed screenshot URL did not change the content hash, so a moved camera would be deduped away")
	}
}

func TestCalgaryCameraCatalogMapping(t *testing.T) {
	entities, observations := mapFixture(t, "cctv_calgary.yaml", "calgary_cameras.json")

	got := make([]string, 0, len(entities))
	for _, e := range entities {
		got = append(got, e.ExternalID)
	}
	if len(entities) != 2 {
		t.Fatalf("entities = %v, want exactly calgary_loc86 and calgary_loc12", got)
	}

	cam := entityByID(entities, "calgary_loc86")
	if cam == nil {
		t.Fatalf("calgary_loc86 missing from %v", got)
	}
	// The catalog publishes HTTP; the browser is served HTTPS and will not load
	// mixed content, so the one verified host is upgraded at mapping time.
	if cam.Metadata["snapshot_url"] != "https://trafficcam.calgary.ca/loc86.jpg" {
		t.Errorf("snapshot_url = %q, want the HTTPS form", cam.Metadata["snapshot_url"])
	}
	// `quadrant` is the street address quadrant, never a camera heading.
	if cam.Metadata["address_quadrant"] != "SE" {
		t.Errorf("address_quadrant = %q", cam.Metadata["address_quadrant"])
	}
	if _, heading := cam.Metadata["heading"]; heading {
		t.Error("quadrant leaked into a heading field; it describes the address, not the camera")
	}

	obs := observationFor(t, entities, observations, "calgary_loc86")
	if obs.Position == nil || obs.Position.Lat != 50.9007257 || obs.Position.Lon != -113.9766063 {
		t.Errorf("position = %+v", obs.Position)
	}

	if entityByID(entities, "calgary_loc12") == nil {
		t.Error("a camera at 0,0 was dropped; zero is a valid coordinate")
	}
}

func TestRadioBrowserCatalogMapping(t *testing.T) {
	entities, observations := mapFixture(t, "radio_browser_stations.yaml", "radio_stations.json")

	got := make([]string, 0, len(entities))
	for _, e := range entities {
		got = append(got, e.ExternalID)
	}
	// Kept: one MP3 with coordinates and one AAC at 0,0.
	// Dropped: HLS, an unsupported codec, a failed health check, a plain-HTTP
	// resolved URL, null coordinates, a malformed UUID and a blank name.
	if len(entities) != 2 {
		t.Fatalf("entities = %v, want exactly the MP3 and AAC stations", got)
	}

	station := entityByID(entities, "ffc0d701-f43b-4232-8be6-f7ff72add8ef")
	if station == nil {
		t.Fatalf("station missing from %v", got)
	}
	if station.Name != "Blasmusikradio mit Bernd" {
		t.Errorf("name = %q, want the trimmed directory name", station.Name)
	}
	if station.Metadata["stream_url"] != "https://stream.laut.fm/blasmusikradio_mit_bernd" {
		t.Errorf("stream_url = %q", station.Metadata["stream_url"])
	}
	// The playback notification interpolates exactly this value.
	if station.Metadata["station_uuid"] != station.ExternalID {
		t.Errorf("station_uuid = %q, want it to match the external id", station.Metadata["station_uuid"])
	}
	if station.Metadata["country_code"] != "DE" {
		t.Errorf("country_code = %q", station.Metadata["country_code"])
	}

	obs := observationFor(t, entities, observations, station.ExternalID)
	if obs.Position == nil || obs.Position.Lat != 48.1372 || obs.Position.Lon != 11.5755 {
		t.Errorf("position = %+v", obs.Position)
	}
	if obs.Metadata["codec"] != "MP3" || obs.Metadata["bitrate_kbps"] != "128" {
		t.Errorf("codec/bitrate = %q/%q", obs.Metadata["codec"], obs.Metadata["bitrate_kbps"])
	}
	// The directory's own check time is labelled as the provider's, not ours.
	if obs.Metadata["provider_last_check"] != "2026-09-21T05:21:11Z" {
		t.Errorf("provider_last_check = %q", obs.Metadata["provider_last_check"])
	}

	nullIsland := entityByID(entities, "11111111-2222-3333-4444-555555555555")
	if nullIsland == nil {
		t.Fatal("a station at 0,0 was dropped; zero is a valid coordinate")
	}
	zeroObs := observationFor(t, entities, observations, nullIsland.ExternalID)
	if zeroObs.Position == nil || zeroObs.Position.Lat != 0 || zeroObs.Position.Lon != 0 {
		t.Errorf("0,0 position = %+v", zeroObs.Position)
	}
}

func TestRadioBrowserContentHashTracksStreamURLChanges(t *testing.T) {
	_, before := mapFixture(t, "radio_browser_stations.yaml", "radio_stations.json")
	_, after := mapFixture(t, "radio_browser_stations.yaml", "radio_stations_updated.json")

	if before[0].ContentHash == "" {
		t.Fatal("dedupe mode produced an empty content hash")
	}
	if before[0].ContentHash == after[0].ContentHash {
		t.Error("a re-pointed stream URL did not change the content hash, so listeners would keep the dead URL")
	}
}

func TestRadioBrowserEmptyPageStopsPagination(t *testing.T) {
	entities, _ := mapFixture(t, "radio_browser_stations.yaml", "radio_stations_empty.json")
	if len(entities) != 0 {
		t.Errorf("entities = %d, want 0 for an empty page", len(entities))
	}
}

// The declared playback action must resolve against the metadata the shipped
// source actually produces — a mismatch here would only surface when a user
// pressed Play.
func TestRadioBrowserPlaybackActionResolvesFromShippedMetadata(t *testing.T) {
	cs, err := newTestLoader(t, false).LoadFile(filepath.Join(sourcesDir(t), "radio_browser_stations.yaml"))
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, nil, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}

	entities, observations := mapFixture(t, "radio_browser_stations.yaml", "radio_stations.json")
	merged := map[string]string{}
	for k, v := range entities[0].Metadata {
		merged[k] = v
	}
	for k, v := range observations[0].Metadata {
		merged[k] = v
	}

	action, err := reg.ResolveMediaAction("radio_stations", "report_play", merged)
	if err != nil {
		t.Fatalf("ResolveMediaAction: %v", err)
	}
	if want := "/json/url/" + entities[0].ExternalID; action.Path != want {
		t.Errorf("path = %q, want %q", action.Path, want)
	}

	// A malformed UUID yields an empty path, which admission rejects rather
	// than sending upstream.
	if _, err := reg.ResolveMediaAction("radio_stations", "report_play", map[string]string{"station_uuid": "../../admin"}); err == nil {
		t.Error("a malformed station uuid produced a sendable path")
	}
}

// Every shipped media declaration must reference metadata the source really
// emits, and every camera URL must sit on a declared origin.
func TestShippedMediaSourcesDeclareUsableMedia(t *testing.T) {
	cases := []struct {
		sourceFile string
		fixture    string
		mediaID    string
	}{
		{"cctv_austin.yaml", "austin_cameras.json", "camera"},
		{"cctv_calgary.yaml", "calgary_cameras.json", "camera"},
		{"radio_browser_stations.yaml", "radio_stations.json", "radio"},
	}

	for _, tc := range cases {
		t.Run(tc.sourceFile, func(t *testing.T) {
			cs, err := newTestLoader(t, false).LoadFile(filepath.Join(sourcesDir(t), tc.sourceFile))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			display := DisplaySpecToDomain(&cs.Definition().Display)

			var media *domain.MediaConfig
			for i := range display.Media {
				if display.Media[i].ID == tc.mediaID {
					media = &display.Media[i]
				}
			}
			if media == nil {
				t.Fatalf("source declares no media entry %q", tc.mediaID)
			}

			entities, observations := mapFixture(t, tc.sourceFile, tc.fixture)
			if len(entities) == 0 {
				t.Fatal("fixture produced no entities")
			}
			merged := map[string]string{}
			for k, v := range entities[0].Metadata {
				merged[k] = v
			}
			for k, v := range observations[0].Metadata {
				merged[k] = v
			}

			url := merged[media.URLKey]
			if url == "" {
				t.Fatalf("media url_key %q resolved to an empty value", media.URLKey)
			}
			if media.AttributionKey != "" && merged[media.AttributionKey] == "" {
				t.Errorf("attribution_key %q resolved to an empty value", media.AttributionKey)
			}
			parsed, err := neturl.Parse(url)
			if err != nil || parsed.Scheme != "https" {
				t.Errorf("media URL %q is not a valid HTTPS URL", url)
				return
			}
			if len(media.AllowedOrigins) > 0 {
				// Compare the way the browser does — origin equality, not a
				// prefix match, which would let "https://host/" pass here and
				// fail in the client.
				origin := parsed.Scheme + "://" + parsed.Host
				allowed := false
				for _, declared := range media.AllowedOrigins {
					if declared == origin {
						allowed = true
					}
				}
				if !allowed {
					t.Errorf("media URL origin %q is outside the declared origins %v", origin, media.AllowedOrigins)
				}
			}
		})
	}
}

// Source URLs pass through os.Expand so a definition can embed ${ENV_VAR}. That
// also means a literal "$" in a query string — Socrata's "$limit", for one — is
// read as an undefined variable and silently deleted, turning "?$limit=2000"
// into "?=2000" and the request into an HTTP 400. Percent-encode it instead.
func TestShippedSourceURLsSurviveEnvExpansion(t *testing.T) {
	entries, err := os.ReadDir(sourcesDir(t))
	if err != nil {
		t.Fatalf("read sources dir: %v", err)
	}
	// Resolve every reference to the empty string, exactly as an unset variable
	// would, and require the URL line to come out unchanged.
	drop := func(string) string { return "" }

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(sourcesDir(t), entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		for _, line := range strings.Split(string(body), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "url:") && !strings.HasPrefix(trimmed, "on_demand_url:") {
				continue
			}
			// ${VAR} references are the supported form and are expected to change.
			if strings.Contains(trimmed, "${") {
				continue
			}
			if got := os.Expand(trimmed, drop); got != trimmed {
				t.Errorf("%s: %q becomes %q after env expansion; percent-encode the $ (%%24)",
					entry.Name(), trimmed, got)
			}
		}
	}
}

// Sources that share a layer share its display configuration: the registry
// keeps the last one registered, unioning only media origins. So the two camera
// sources must declare the same contract apart from those origins — otherwise
// whichever file loads last silently decides the icon, the colour, the refresh
// interval and which fields the panel shows for BOTH cities.
func TestCameraSourcesSharingTheCctvLayerDeclareOneContract(t *testing.T) {
	load := func(file string) *SourceDefinition {
		cs, err := newTestLoader(t, false).LoadFile(filepath.Join(sourcesDir(t), file))
		if err != nil {
			t.Fatalf("load %s: %v", file, err)
		}
		return cs.Definition()
	}
	austin, calgary := load("cctv_austin.yaml"), load("cctv_calgary.yaml")

	if austin.LayerType != "cctv" || calgary.LayerType != "cctv" {
		t.Fatalf("layer types = %q / %q, want both cctv", austin.LayerType, calgary.LayerType)
	}
	if austin.LayerDisplayName != calgary.LayerDisplayName {
		t.Errorf("layer label differs: %q vs %q", austin.LayerDisplayName, calgary.LayerDisplayName)
	}

	// Everything the browser renders for the layer, compared as JSON so a new
	// display field is covered without editing this test.
	strip := func(d *SourceDefinition) string {
		dc := DisplaySpecToDomain(&d.Display)
		for i := range dc.Media {
			dc.Media[i].AllowedOrigins = nil // per-provider; the registry unions these
		}
		b, err := json.Marshal(dc)
		if err != nil {
			t.Fatalf("marshal display: %v", err)
		}
		return string(b)
	}
	if a, c := strip(austin), strip(calgary); a != c {
		t.Errorf("the two cctv sources declare different display config:\n austin:  %s\n calgary: %s", a, c)
	}

	// The origins themselves must differ — that is the whole reason the union
	// exists, and it is what keeps each city's cameras admissible.
	originsOf := func(d *SourceDefinition) []string {
		for _, m := range DisplaySpecToDomain(&d.Display).Media {
			if m.ID == "camera" {
				return m.AllowedOrigins
			}
		}
		return nil
	}
	if ao, co := originsOf(austin), originsOf(calgary); len(ao) == 0 || len(co) == 0 || ao[0] == co[0] {
		t.Errorf("expected distinct provider origins, got %v and %v", ao, co)
	}
}
