package grpctransport

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	appai "github.com/Alevsk/respondent/internal/app/ai"
	"github.com/Alevsk/respondent/internal/domain"
)

// AIServer implements the AIService gRPC interface.
type AIServer struct {
	respondentv1.UnimplementedAIServiceServer
	svc    domain.AIServicer
	logger zerolog.Logger
}

// NewAIServer creates a new AIServer.
// The optional logger variadic allows callers to inject a zerolog.Logger.
func NewAIServer(svc domain.AIServicer, logger ...zerolog.Logger) *AIServer {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &AIServer{svc: svc, logger: l}
}

// NaturalLanguageSearch handles the NaturalLanguageSearch RPC.
func (s *AIServer) NaturalLanguageSearch(ctx context.Context, req *respondentv1.NaturalLanguageSearchRequest) (*respondentv1.NaturalLanguageSearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	result, err := s.svc.NaturalLanguageSearch(ctx, req.Query, req.LayerType, int(req.Limit))
	if err != nil {
		if errors.Is(err, appai.ErrLLMUnavailable) {
			return nil, status.Error(codes.Unavailable, "AI features are disabled")
		}
		s.logger.Error().Err(err).Str("query", req.Query).Msg("natural language search failed")
		return nil, status.Error(codes.Internal, "search failed")
	}

	protoResults := make([]*respondentv1.EntitySearchResult, len(result.Results))
	for i, r := range result.Results {
		protoResults[i] = &respondentv1.EntitySearchResult{
			EntityId:   r.EntityID,
			ExternalId: r.ExternalID,
			LayerType:  r.LayerType,
			Name:       r.Name,
			Metadata:   r.Metadata,
		}
	}

	return &respondentv1.NaturalLanguageSearchResponse{
		GeneratedSql: result.GeneratedSQL,
		Results:      protoResults,
		TotalCount:   int32(result.TotalCount),
		Explanation:  result.Explanation,
	}, nil
}

// AnalyzeEntity handles the AnalyzeEntity RPC.
func (s *AIServer) AnalyzeEntity(ctx context.Context, req *respondentv1.AnalyzeEntityRequest) (*respondentv1.AnalyzeEntityResponse, error) {
	if req.EntityId == "" {
		return nil, status.Error(codes.InvalidArgument, "entity_id is required")
	}

	result, err := s.svc.AnalyzeEntity(ctx, req.EntityId, int(req.ObservationLimit))
	if err != nil {
		if errors.Is(err, appai.ErrLLMUnavailable) {
			return nil, status.Error(codes.Unavailable, "AI features are disabled")
		}
		s.logger.Error().Err(err).Str("entity_id", req.EntityId).Msg("entity analysis failed")
		return nil, status.Error(codes.Internal, "analysis failed")
	}

	return &respondentv1.AnalyzeEntityResponse{
		EntityId:  result.EntityID,
		Analysis:  result.Analysis,
		InsightId: result.InsightID,
	}, nil
}

// GetInsights handles the GetInsights RPC.
func (s *AIServer) GetInsights(ctx context.Context, req *respondentv1.GetInsightsRequest) (*respondentv1.GetInsightsResponse, error) {
	filter := domain.InsightFilter{
		InsightType:  req.InsightType,
		LayerType:    req.LayerType,
		EntityID:     req.EntityId,
		Attention:    domain.AttentionLevelToString(int32(req.Attention)),
		MinAttention: domain.AttentionLevelToString(int32(req.MinAttention)),
		Limit:        int(req.Limit),
		Offset:       int(req.Offset),
	}

	insights, totalCount, err := s.svc.GetInsights(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get insights")
		return nil, status.Error(codes.Internal, "failed to get insights")
	}

	protoInsights := make([]*respondentv1.AIInsight, len(insights))
	for i, insight := range insights {
		protoInsights[i] = domainInsightToProto(insight, s.logger)
	}

	return &respondentv1.GetInsightsResponse{
		Insights:   protoInsights,
		TotalCount: int32(totalCount),
	}, nil
}

// ExplainQuery handles the ExplainQuery RPC.
func (s *AIServer) ExplainQuery(ctx context.Context, req *respondentv1.ExplainQueryRequest) (*respondentv1.ExplainQueryResponse, error) {
	if req.Sql == "" {
		return nil, status.Error(codes.InvalidArgument, "sql is required")
	}

	explanation, err := s.svc.ExplainQuery(ctx, req.Sql)
	if err != nil {
		if errors.Is(err, appai.ErrLLMUnavailable) {
			return nil, status.Error(codes.Unavailable, "AI features are disabled")
		}
		s.logger.Error().Err(err).Msg("query explanation failed")
		return nil, status.Error(codes.Internal, "explanation failed")
	}

	return &respondentv1.ExplainQueryResponse{
		Explanation: explanation,
	}, nil
}

// ListAnalysisDefinitions handles the ListAnalysisDefinitions RPC.
func (s *AIServer) ListAnalysisDefinitions(ctx context.Context, _ *respondentv1.ListAnalysisDefinitionsRequest) (*respondentv1.ListAnalysisDefinitionsResponse, error) {
	defs := s.svc.ListAnalysisDefinitions(ctx)

	protoDefs := make([]*respondentv1.AnalysisDefinitionInfo, len(defs))
	for i, def := range defs {
		protoDefs[i] = &respondentv1.AnalysisDefinitionInfo{
			Name:         def.Name,
			DisplayName:  def.DisplayName,
			Enabled:      def.Enabled,
			Schedule:     def.Schedule,
			Layers:       def.Layers,
			InsightTypes: def.InsightTypes,
		}
	}

	return &respondentv1.ListAnalysisDefinitionsResponse{
		Definitions: protoDefs,
	}, nil
}

// GetNotificationFilterOptions handles the GetNotificationFilterOptions RPC.
func (s *AIServer) GetNotificationFilterOptions(ctx context.Context, _ *respondentv1.GetNotificationFilterOptionsRequest) (*respondentv1.GetNotificationFilterOptionsResponse, error) {
	opts := s.svc.GetNotificationFilterOptions(ctx)

	protoTypes := make([]*respondentv1.InsightTypeOption, len(opts.InsightTypes))
	for i, t := range opts.InsightTypes {
		protoTypes[i] = &respondentv1.InsightTypeOption{
			Value:       t.Value,
			DisplayName: t.DisplayName,
			SourceName:  t.SourceName,
		}
	}

	return &respondentv1.GetNotificationFilterOptionsResponse{
		InsightTypes:    protoTypes,
		AttentionLevels: opts.AttentionLevels,
		LayerTypes:      opts.LayerTypes,
	}, nil
}
