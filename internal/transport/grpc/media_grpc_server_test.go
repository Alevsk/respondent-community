package grpctransport

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

type stubReporter struct {
	reported  bool
	err       error
	gotEntity string
	gotMedia  string
	callCount int
}

func (s *stubReporter) ReportPlayback(_ context.Context, entityID, mediaID string) (bool, error) {
	s.callCount++
	s.gotEntity, s.gotMedia = entityID, mediaID
	return s.reported, s.err
}

func TestReportMediaPlaybackPassesIdentifiersThroughAndReturnsOnlyStatus(t *testing.T) {
	svc := &stubReporter{reported: true}
	resp, err := NewMediaServer(svc).ReportMediaPlayback(context.Background(),
		&respondentv1.ReportMediaPlaybackRequest{EntityId: "radio_stations:station-1", MediaId: "radio"})
	if err != nil {
		t.Fatalf("ReportMediaPlayback: %v", err)
	}
	if !resp.GetReported() {
		t.Error("reported = false, want true")
	}
	if svc.gotEntity != "radio_stations:station-1" || svc.gotMedia != "radio" {
		t.Errorf("service got (%q, %q)", svc.gotEntity, svc.gotMedia)
	}
	// The response type has exactly one meaningful field: no upstream URL,
	// body or header can ride back to the caller.
	if got := resp.ProtoReflect().Descriptor().Fields().Len(); got != 1 {
		t.Errorf("response has %d fields, want exactly 1 (reported)", got)
	}
}

func TestReportMediaPlaybackMapsDomainErrors(t *testing.T) {
	tests := map[string]struct {
		err  error
		want codes.Code
	}{
		"not found":     {domain.NewNotFoundError("media not declared for this layer", nil), codes.NotFound},
		"invalid input": {domain.NewInvalidInputError("invalid media id", nil), codes.InvalidArgument},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewMediaServer(&stubReporter{err: tc.err}).ReportMediaPlayback(context.Background(),
				&respondentv1.ReportMediaPlaybackRequest{EntityId: "x:y", MediaId: "radio"})
			if status.Code(err) != tc.want {
				t.Errorf("code = %v, want %v (err=%v)", status.Code(err), tc.want, err)
			}
		})
	}
}

// A failed upstream notification is a false result, never a 5xx: the caller's
// audio is already playing and must not be told the API broke.
func TestReportMediaPlaybackReportsFalseWithoutError(t *testing.T) {
	resp, err := NewMediaServer(&stubReporter{reported: false}).ReportMediaPlayback(context.Background(),
		&respondentv1.ReportMediaPlaybackRequest{EntityId: "x:y", MediaId: "radio"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetReported() {
		t.Error("reported = true, want false")
	}
}
