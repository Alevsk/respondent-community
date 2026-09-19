package grpctransport

import (
	"context"

	"github.com/rs/zerolog"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

// IndicatorServer implements the IndicatorService gRPC interface.
type IndicatorServer struct {
	respondentv1.UnimplementedIndicatorServiceServer
	svc    domain.IndicatorServicer
	logger zerolog.Logger
}

// NewIndicatorServer creates a new IndicatorServer.
// The optional logger variadic allows callers to inject a zerolog.Logger.
func NewIndicatorServer(svc domain.IndicatorServicer, logger ...zerolog.Logger) *IndicatorServer {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &IndicatorServer{svc: svc, logger: l}
}

// GetGlobalIndicators returns the latest indicator values for global indicator layers.
func (s *IndicatorServer) GetGlobalIndicators(ctx context.Context, req *respondentv1.GetGlobalIndicatorsRequest) (*respondentv1.GetGlobalIndicatorsResponse, error) {
	snapshots, err := s.svc.GetGlobalIndicators(ctx, req.LayerIds)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoSnapshots := make([]*respondentv1.IndicatorSnapshot, len(snapshots))
	for i, snap := range snapshots {
		protoSnapshots[i] = domainIndicatorSnapshotToProto(snap)
	}

	return &respondentv1.GetGlobalIndicatorsResponse{
		Indicators: protoSnapshots,
	}, nil
}
