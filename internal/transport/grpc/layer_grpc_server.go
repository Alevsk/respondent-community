// Package grpctransport provides gRPC server implementations for the API services.
package grpctransport

import (
	"context"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

// LayerServer implements the LayerService gRPC interface.
type LayerServer struct {
	respondentv1.UnimplementedLayerServiceServer
	svc    domain.LayerServicer
	logger zerolog.Logger
}

// NewLayerServer creates a new LayerServer.
// The optional logger variadic allows callers to inject a zerolog.Logger.
func NewLayerServer(svc domain.LayerServicer, logger ...zerolog.Logger) *LayerServer {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &LayerServer{svc: svc, logger: l}
}

// GetLayers returns all available layers.
func (s *LayerServer) GetLayers(ctx context.Context, req *respondentv1.GetLayersRequest) (*respondentv1.GetLayersResponse, error) {
	layers, err := s.svc.GetLayers(ctx)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoLayers := make([]*respondentv1.Layer, len(layers))
	for i, layer := range layers {
		protoLayers[i] = domainLayerToProto(layer)
	}

	return &respondentv1.GetLayersResponse{
		Layers: protoLayers,
	}, nil
}

// ToggleLayer toggles layer enable/disable, mode, and density.
func (s *LayerServer) ToggleLayer(ctx context.Context, req *respondentv1.ToggleLayerRequest) (*respondentv1.ToggleLayerResponse, error) {
	if req.Toggle == nil {
		return nil, status.Error(codes.InvalidArgument, "toggle is required")
	}

	toggle := &domain.LayerToggle{
		LayerID: req.Toggle.LayerId,
		Enabled: req.Toggle.Enabled,
		Mode:    domain.LayerModeToString(int32(req.Toggle.Mode)),
		Density: req.Toggle.Density,
	}

	layer, err := s.svc.ToggleLayer(ctx, toggle)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	return &respondentv1.ToggleLayerResponse{
		Layer: domainLayerToProto(layer),
	}, nil
}

// GetLayerSnapshot returns current snapshot of entities for a layer.
func (s *LayerServer) GetLayerSnapshot(ctx context.Context, req *respondentv1.GetLayerSnapshotRequest) (*respondentv1.GetLayerSnapshotResponse, error) {
	if req.LayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "layer_id is required")
	}

	result, err := s.svc.GetLayerSnapshot(ctx, req.LayerId, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoEntities := make([]*respondentv1.Entity, len(result.Entities))
	for i, entity := range result.Entities {
		protoEntities[i] = domainEntityToProto(entity)
	}

	protoObservations := make([]*respondentv1.Observation, len(result.Observations))
	for i, obs := range result.Observations {
		protoObservations[i] = domainObservationToProto(obs)
	}

	return &respondentv1.GetLayerSnapshotResponse{
		Entities:     protoEntities,
		Observations: protoObservations,
		TotalCount:   result.TotalCount,
		HasMore:      result.HasMore,
	}, nil
}
