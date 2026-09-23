package declarative

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
)

const snapshotMediaYAML = `
  media:
    - id: camera
      kind: snapshot
      label: Camera feed
      url_key: type
      attribution_key: status
      allowed_origins: ["https://camera.example.com"]
      snapshot:
        refresh_interval: 30s
        cache_bust_param: frame
`

func loadMediaTestSource(t *testing.T, source string) (*CompiledSource, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return newTestLoader(t, false).LoadFile(path)
}

func TestMediaContractLoadsInBothPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "camera.yaml")
	if err := os.WriteFile(path, []byte(validSourceYAML+snapshotMediaYAML), 0600); err != nil {
		t.Fatal(err)
	}
	cs, err := newTestLoader(t, false).LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reg := domain.NewDynamicSourceRegistry()
	if err := RegisterDisplayConfigs(dir, reg, nil); err != nil {
		t.Fatal(err)
	}
	lightweight, ok := reg.LookupDisplayConfig("test_layer")
	if !ok {
		t.Fatal("display registration missing")
	}
	for name, dc := range map[string]*domain.LayerDisplayConfig{"full": DisplaySpecToDomain(&cs.Definition().Display), "display": lightweight} {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(dc)
			if err != nil {
				t.Fatal(err)
			}
			var data map[string]interface{}
			if err := json.Unmarshal(b, &data); err != nil {
				t.Fatal(err)
			}
			media, ok := data["Media"].([]interface{})
			if !ok || len(media) != 1 {
				t.Fatalf("media contract lost: %s", b)
			}
			entry := media[0].(map[string]interface{})
			if entry["ID"] != "camera" || entry["Kind"] != "snapshot" || entry["URLKey"] != "type" {
				t.Fatalf("wrong media: %#v", entry)
			}
		})
	}
}

func TestMediaRejectsInvalidDeclarationsInBothPaths(t *testing.T) {
	tests := map[string]string{
		"unknown kind":        strings.Replace(snapshotMediaYAML, "kind: snapshot", "kind: iframe", 1),
		"duplicate ID":        snapshotMediaYAML + strings.TrimPrefix(snapshotMediaYAML, "\n  media:\n"),
		"missing metadata":    strings.Replace(snapshotMediaYAML, "url_key: type", "url_key: missing", 1),
		"missing attribution": strings.Replace(snapshotMediaYAML, "attribution_key: status", "attribution_key: missing", 1),
		"fast refresh":        strings.Replace(snapshotMediaYAML, "30s", "1s", 1),
		"slow refresh":        strings.Replace(snapshotMediaYAML, "30s", "2h", 1),
		"missing snapshot":    strings.Split(snapshotMediaYAML, "      snapshot:")[0],
		"audio with refresh":  strings.Replace(snapshotMediaYAML, "kind: snapshot", "kind: audio", 1),
		"HTTP origin":         strings.Replace(snapshotMediaYAML, "https://camera", "http://camera", 1),
		"origin with path":    strings.Replace(snapshotMediaYAML, "camera.example.com", "camera.example.com/path", 1),
		"private origin":      strings.Replace(snapshotMediaYAML, "camera.example.com", "127.0.0.1", 1),
		"credential origin":   strings.Replace(snapshotMediaYAML, "camera.example.com", "user:secret@camera.example.com", 1),
		"invalid ID":          strings.Replace(snapshotMediaYAML, "id: camera", "id: ../camera", 1),
		"empty label":         strings.Replace(snapshotMediaYAML, "label: Camera feed", "label: ''", 1),
		"undefined action":    strings.Replace(snapshotMediaYAML, "      snapshot:", "      playback_action: missing\n      snapshot:", 1),
	}
	for name, media := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "camera.yaml")
			if err := os.WriteFile(path, []byte(validSourceYAML+media), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := newTestLoader(t, false).LoadFile(path); err == nil {
				t.Error("full loader accepted invalid media")
			}
			reg := domain.NewDynamicSourceRegistry()
			if err := RegisterDisplayConfigs(dir, reg, nil); err != nil {
				t.Fatal(err)
			}
			if _, ok := reg.LookupDisplayConfig("test_layer"); ok {
				t.Error("display loader exposed invalid media")
			}
		})
	}
}
