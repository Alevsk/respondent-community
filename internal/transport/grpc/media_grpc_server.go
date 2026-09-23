package grpctransport

import (
	"context"

	"github.com/rs/zerolog"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

// MediaServer implements the MediaService gRPC interface.
//
// The request carries only an entity id and a media id. There is deliberately
// no field for a URL, header or expression: the notification target comes from
// the source definition the server already trusts.
type MediaServer struct {
	respondentv1.UnimplementedMediaServiceServer
	svc    domain.MediaServicer
	logger zerolog.Logger
}

// NewMediaServer creates a new MediaServer.
// The optional logger variadic allows callers to inject a zerolog.Logger.
func NewMediaServer(svc domain.MediaServicer, logger ...zerolog.Logger) *MediaServer {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &MediaServer{svc: svc, logger: l}
}

// ReportMediaPlayback records that a user started playing an entity's declared
// media. The response says only whether the source was notified.
func (s *MediaServer) ReportMediaPlayback(ctx context.Context, req *respondentv1.ReportMediaPlaybackRequest) (*respondentv1.ReportMediaPlaybackResponse, error) {
	reported, err := s.svc.ReportPlayback(ctx, req.GetEntityId(), req.GetMediaId())
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}
	return &respondentv1.ReportMediaPlaybackResponse{Reported: reported}, nil
}
