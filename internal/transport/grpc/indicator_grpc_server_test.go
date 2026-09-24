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

func TestNewIndicatorServer(t *testing.T) {
	eRepo := &testEntityRepo{}
	oRepo := &testObsRepo{}
	svc := indicator.NewIndicatorService(eRepo, oRepo, domain.NewDynamicSourceRegistry())
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
			svc := indicator.NewIndicatorService(eRepo, oRepo, domain.NewDynamicSourceRegistry())
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
