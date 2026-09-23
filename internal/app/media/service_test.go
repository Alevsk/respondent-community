package media

import (
	"context"
	"errors"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
)

type stubEntities struct {
	detail *domain.EntityDetail
	err    error
	gotID  string
}

func (s *stubEntities) GetEntityDetail(_ context.Context, entityID string) (*domain.EntityDetail, error) {
	s.gotID = entityID
	if s.err != nil {
		return nil, s.err
	}
	return s.detail, nil
}

type stubDisplay struct {
	configs map[domain.LayerType]*domain.LayerDisplayConfig
}

func (s *stubDisplay) LookupDisplayConfig(lt domain.LayerType) (*domain.LayerDisplayConfig, bool) {
	dc, ok := s.configs[lt]
	return dc, ok
}

type stubActions struct {
	resolveErr   error
	executeErr   error
	gotLayer     domain.LayerType
	gotName      string
	gotMetadata  map[string]string
	executeCalls int
}

func (s *stubActions) ResolveMediaAction(lt domain.LayerType, name string, metadata map[string]string) (*domain.MediaPlaybackAction, error) {
	s.gotLayer, s.gotName, s.gotMetadata = lt, name, metadata
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return &domain.MediaPlaybackAction{LayerType: lt, Name: name, Method: "GET", Path: "/json/url/" + metadata["station_uuid"]}, nil
}

func (s *stubActions) ExecuteMediaAction(_ context.Context, _ *domain.MediaPlaybackAction) error {
	s.executeCalls++
	return s.executeErr
}

func radioFixture() (*stubEntities, *stubDisplay) {
	entities := &stubEntities{detail: &domain.EntityDetail{
		Entity: &domain.Entity{
			ID:         "uuid-1",
			ExternalID: "station-1",
			LayerType:  "radio_stations",
			Metadata:   map[string]string{"station_uuid": "from-entity", "codec": "MP3"},
		},
		LatestObservation: &domain.Observation{
			Metadata: map[string]string{"station_uuid": "from-observation"},
		},
	}}
	display := &stubDisplay{configs: map[domain.LayerType]*domain.LayerDisplayConfig{
		"radio_stations": {Media: []domain.MediaConfig{
			{ID: "radio", Kind: "audio", Label: "Live stream", URLKey: "stream_url", PlaybackAction: "report_play"},
			{ID: "silent", Kind: "audio", Label: "No notification", URLKey: "stream_url"},
		}},
	}}
	return entities, display
}

func TestReportPlaybackNotifiesUsingMergedMetadata(t *testing.T) {
	entities, display := radioFixture()
	actions := &stubActions{}
	svc := NewService(entities, display, actions, actions)

	reported, err := svc.ReportPlayback(context.Background(), "radio_stations:station-1", "radio")
	if err != nil {
		t.Fatalf("ReportPlayback: %v", err)
	}
	if !reported {
		t.Error("reported = false, want true")
	}
	if actions.executeCalls != 1 {
		t.Errorf("execute calls = %d, want 1", actions.executeCalls)
	}
	if actions.gotName != "report_play" {
		t.Errorf("action = %q, want report_play", actions.gotName)
	}
	if actions.gotLayer != "radio_stations" {
		t.Errorf("layer = %q, want radio_stations", actions.gotLayer)
	}
	// Observation metadata wins, matching the overview's field resolution.
	if got := actions.gotMetadata["station_uuid"]; got != "from-observation" {
		t.Errorf("station_uuid = %q, want from-observation", got)
	}
	if got := actions.gotMetadata["codec"]; got != "MP3" {
		t.Errorf("entity-only key lost in merge: codec = %q", got)
	}
}

func TestReportPlaybackRejectsUnknownEntityOrMedia(t *testing.T) {
	t.Run("unknown entity", func(t *testing.T) {
		entities, display := radioFixture()
		entities.err = domain.NewNotFoundError("entity not found", nil)
		actions := &stubActions{}
		if _, err := NewService(entities, display, actions, actions).
			ReportPlayback(context.Background(), "radio_stations:nope", "radio"); err == nil {
			t.Error("reported playback for an entity that does not exist")
		}
		if actions.executeCalls != 0 {
			t.Error("notified upstream for an unknown entity")
		}
	})

	t.Run("unknown media id", func(t *testing.T) {
		entities, display := radioFixture()
		actions := &stubActions{}
		if _, err := NewService(entities, display, actions, actions).
			ReportPlayback(context.Background(), "radio_stations:station-1", "not-declared"); err == nil {
			t.Error("reported playback for a media id the layer never declared")
		}
		if actions.executeCalls != 0 {
			t.Error("notified upstream for an unknown media id")
		}
	})

	t.Run("layer without display config", func(t *testing.T) {
		entities, display := radioFixture()
		entities.detail.Entity.LayerType = "unregistered"
		actions := &stubActions{}
		if _, err := NewService(entities, display, actions, actions).
			ReportPlayback(context.Background(), "unregistered:x", "radio"); err == nil {
			t.Error("reported playback for a layer with no display config")
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		entities, display := radioFixture()
		actions := &stubActions{}
		svc := NewService(entities, display, actions, actions)
		if _, err := svc.ReportPlayback(context.Background(), "", "radio"); err == nil {
			t.Error("accepted an empty entity id")
		}
		if _, err := svc.ReportPlayback(context.Background(), "radio_stations:station-1", ""); err == nil {
			t.Error("accepted an empty media id")
		}
	})
}

// A media slot with no declared playback_action is a no-op, not an error: the
// UI may call this for any media entry and must not be told the entity is bad.
func TestReportPlaybackIsANoOpWithoutADeclaredAction(t *testing.T) {
	entities, display := radioFixture()
	actions := &stubActions{}
	reported, err := NewService(entities, display, actions, actions).
		ReportPlayback(context.Background(), "radio_stations:station-1", "silent")
	if err != nil {
		t.Fatalf("ReportPlayback: %v", err)
	}
	if reported {
		t.Error("reported = true for a media entry that declares no action")
	}
	if actions.executeCalls != 0 {
		t.Error("notified upstream for a media entry that declares no action")
	}
}

// An upstream notification failure is observable in the result but is never an
// API error: it must not stop the listener's audio or trigger a retry loop.
func TestReportPlaybackSurfacesUpstreamFailureWithoutError(t *testing.T) {
	for name, actions := range map[string]*stubActions{
		"resolve rejected the path": {resolveErr: errors.New("resolved path must be origin-relative")},
		"upstream request failed":   {executeErr: errors.New("connection refused")},
	} {
		t.Run(name, func(t *testing.T) {
			entities, display := radioFixture()
			reported, err := NewService(entities, display, actions, actions).
				ReportPlayback(context.Background(), "radio_stations:station-1", "radio")
			if err != nil {
				t.Fatalf("upstream failure became an API error: %v", err)
			}
			if reported {
				t.Error("reported = true despite a failed notification")
			}
		})
	}
}

// The use case takes an entity id and a media id and returns a boolean. There
// is no seam for a caller-supplied URL, header or expression.
func TestReportPlaybackTakesNoCallerSuppliedTarget(t *testing.T) {
	entities, display := radioFixture()
	actions := &stubActions{}
	svc := NewService(entities, display, actions, actions)

	// A caller stuffing a URL into either identifier resolves nothing.
	if _, err := svc.ReportPlayback(context.Background(), "https://evil.example.com/steal", "radio"); err == nil {
		t.Error("accepted a URL as an entity id")
	}
	if _, err := svc.ReportPlayback(context.Background(), "radio_stations:station-1", "https://evil.example.com/steal"); err == nil {
		t.Error("accepted a URL as a media id")
	}
	if actions.executeCalls != 0 {
		t.Errorf("made %d upstream calls for caller-supplied targets, want 0", actions.executeCalls)
	}
}
