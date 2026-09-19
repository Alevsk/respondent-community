package grpctransport

import (
	"context"
	"testing"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/app/indicator"
	"github.com/Alevsk/respondent/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testCacheStorage struct {
	entities     []*domain.Entity
	observations []*domain.Observation
	totalCount   int64
	err          error
	countErr     error
}

func (c *testCacheStorage) SetEntity(_ context.Context, _ *domain.Entity, _ *domain.Observation) error {
	return nil
}
func (c *testCacheStorage) GetEntity(_ context.Context, _, _ string) (*domain.Entity, *domain.Observation, error) {
	return nil, nil, nil
}
func (c *testCacheStorage) GetLayerEntities(_ context.Context, _ string, _, _ int) ([]*domain.Entity, []*domain.Observation, int64, error) {
	return c.entities, c.observations, c.totalCount, c.err
}
func (c *testCacheStorage) GetLayerCount(_ context.Context, _ string) (int64, error) {
	return c.totalCount, c.countErr
}
func (c *testCacheStorage) ClearLayer(_ context.Context, _ string) error {
	return nil
}
func (c *testCacheStorage) GetStats(_ context.Context) (map[string]any, error) {
	return nil, nil
}
func (c *testCacheStorage) HealthCheck(_ context.Context) error {
	return nil
}
func (c *testCacheStorage) Close() error {
	return nil
}

var _ domain.CacheStorage = (*testCacheStorage)(nil)

func TestNewIndicatorServer(t *testing.T) {
	eRepo := &testEntityRepo{}
	oRepo := &testObsRepo{}
	cache := &testCacheStorage{}
	svc := indicator.NewIndicatorService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
	srv := NewIndicatorServer(svc)
	if srv == nil {
		t.Fatal("expected server, got nil")
	}
}

func TestIndicatorServer_GetGlobalIndicators(t *testing.T) {
	tests := []struct {
		name      string
		req       *respondentv1.GetGlobalIndicatorsRequest
		wantCode  codes.Code
		wantCount int
	}{
		{
			name:      "empty layer_ids returns all",
			req:       &respondentv1.GetGlobalIndicatorsRequest{LayerIds: []string{}},
			wantCode:  codes.OK,
			wantCount: 0,
		},
		{
			name:      "specific layer_ids",
			req:       &respondentv1.GetGlobalIndicatorsRequest{LayerIds: []string{"noaa_space_weather"}},
			wantCode:  codes.OK,
			wantCount: 0,
		},
		{
			name:      "nil layer_ids",
			req:       &respondentv1.GetGlobalIndicatorsRequest{LayerIds: nil},
			wantCode:  codes.OK,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{}
			oRepo := &testObsRepo{}
			cache := &testCacheStorage{}
			svc := indicator.NewIndicatorService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
			srv := NewIndicatorServer(svc)

			resp, err := srv.GetGlobalIndicators(context.Background(), tt.req)

			if tt.wantCode != codes.OK {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v: %s", tt.wantCode, st.Code(), st.Message())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected response, got nil")
			}
			if len(resp.Indicators) != tt.wantCount {
				t.Errorf("expected %d indicators, got %d", tt.wantCount, len(resp.Indicators))
			}
		})
	}
}
