package grpctransport

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

// EntityServer implements the EntityService gRPC interface.
type EntityServer struct {
	respondentv1.UnimplementedEntityServiceServer
	svc    domain.EntityServicer
	logger zerolog.Logger
}

// NewEntityServer creates a new EntityServer.
// The optional logger variadic allows callers to inject a zerolog.Logger.
func NewEntityServer(svc domain.EntityServicer, logger ...zerolog.Logger) *EntityServer {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &EntityServer{svc: svc, logger: l}
}

// GetEntityDetail returns a single entity with its latest observation.
func (s *EntityServer) GetEntityDetail(ctx context.Context, req *respondentv1.GetEntityDetailRequest) (*respondentv1.GetEntityDetailResponse, error) {
	if req.EntityId == "" {
		return nil, status.Error(codes.InvalidArgument, "entity_id is required")
	}

	detail, err := s.svc.GetEntityDetail(ctx, req.EntityId)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	resp := &respondentv1.GetEntityDetailResponse{
		Entity: domainEntityToProto(detail.Entity),
	}
	if detail.LatestObservation != nil {
		resp.LatestObservation = domainObservationToProto(detail.LatestObservation)
	}

	return resp, nil
}

// GetObservationHistory returns paginated observation history for an entity.
func (s *EntityServer) GetObservationHistory(ctx context.Context, req *respondentv1.GetObservationHistoryRequest) (*respondentv1.GetObservationHistoryResponse, error) {
	if req.EntityId == "" {
		return nil, status.Error(codes.InvalidArgument, "entity_id is required")
	}

	var before time.Time
	if req.BeforeMs > 0 {
		before = time.UnixMilli(req.BeforeMs)
	}

	observations, hasMore, err := s.svc.GetObservationHistory(ctx, req.EntityId, int(req.Limit), before)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoObs := make([]*respondentv1.Observation, len(observations))
	for i, obs := range observations {
		protoObs[i] = domainObservationToProto(obs)
	}

	return &respondentv1.GetObservationHistoryResponse{
		Observations: protoObs,
		HasMore:      hasMore,
	}, nil
}

// GetEntitiesBatch returns details for multiple entities by their composite IDs.
func (s *EntityServer) GetEntitiesBatch(ctx context.Context, req *respondentv1.GetEntitiesBatchRequest) (*respondentv1.GetEntitiesBatchResponse, error) {
	if len(req.EntityIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "entity_ids is required")
	}
	if len(req.EntityIds) > 50 {
		return nil, status.Errorf(codes.InvalidArgument, "batch size %d exceeds maximum of 50", len(req.EntityIds))
	}

	details, err := s.svc.GetEntitiesBatch(ctx, req.EntityIds)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoEntities := make([]*respondentv1.BatchEntityDetail, 0, len(details))
	for _, d := range details {
		bd := &respondentv1.BatchEntityDetail{
			Entity: domainEntityToProto(d.Entity),
		}
		if d.LatestObservation != nil {
			bd.LatestObservation = domainObservationToProto(d.LatestObservation)
		}
		protoEntities = append(protoEntities, bd)
	}

	return &respondentv1.GetEntitiesBatchResponse{
		Entities: protoEntities,
	}, nil
}

// SearchEntities searches entities by query string with optional layer type filter.
func (s *EntityServer) SearchEntities(ctx context.Context, req *respondentv1.SearchEntitiesRequest) (*respondentv1.SearchEntitiesResponse, error) {
	if len(req.Query) < 2 {
		return nil, status.Error(codes.InvalidArgument, "query must be at least 2 characters")
	}

	limit := int(req.Limit)

	s.logger.Debug().
		Str("query", req.Query).
		Str("layer_type", req.LayerType).
		Int("limit", limit).
		Msg("searching entities")

	results, totalCount, err := s.svc.SearchEntities(ctx, req.Query, req.LayerType, limit)
	if err != nil {
		return nil, domainErrorToGRPC(err)
	}

	protoResults := make([]*respondentv1.EntitySearchResult, len(results))
	for i, r := range results {
		entityID := domain.EntityID(domain.LayerType(r.Entity.LayerType), r.Entity.ExternalID)
		pr := &respondentv1.EntitySearchResult{
			EntityId:   entityID,
			ExternalId: r.Entity.ExternalID,
			LayerType:  r.Entity.LayerType,
			Name:       r.Entity.Name,
			Metadata:   r.Entity.Metadata,
		}
		if r.LatestObservation != nil {
			pr.LatestObservation = domainObservationToProto(r.LatestObservation)
		}
		protoResults[i] = pr
	}

	return &respondentv1.SearchEntitiesResponse{
		Results:    protoResults,
		TotalCount: int32(totalCount),
	}, nil
}
