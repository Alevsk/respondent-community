package grpctransport

import (
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestMediaDisplayWireContract(t *testing.T) {
	dc := &domain.LayerDisplayConfig{Media: []domain.MediaConfig{
		{ID: "camera", Kind: "snapshot", Label: "Camera", URLKey: "snapshot_url", AttributionKey: "attribution", AllowedOrigins: []string{"https://camera.example.com"}, Snapshot: &domain.SnapshotMediaConfig{RefreshIntervalSeconds: 30, CacheBustParam: "frame"}},
		{ID: "radio", Kind: "audio", Label: "Radio", URLKey: "stream_url", PlaybackAction: "report_play"},
	}}
	b, err := protojson.Marshal(domainDisplayConfigToProto(dc))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`"media"`, `"kind":"snapshot"`, `"refreshIntervalSeconds":30`, `"urlKey":"snapshot_url"`, `"audio":{}`, `"playbackAction":"report_play"`} {
		if !strings.Contains(strings.ReplaceAll(string(b), " ", ""), part) {
			t.Errorf("missing %s in %s", part, b)
		}
	}
}
