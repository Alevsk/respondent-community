package grpctransport

import (
	"context"
	"fmt"
	"testing"
	"time"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/app/layer"
	"github.com/Alevsk/respondent/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewLayerServer(t *testing.T) {
	eRepo := &testEntityRepo{}
	oRepo := &testObsRepo{}
	cache := &testCacheStorage{}
	svc := layer.NewLayerService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
	srv := NewLayerServer(svc)
	if srv == nil {
		t.Fatal("expected server, got nil")
	}
}

func TestLayerServer_GetLayers(t *testing.T) {
	tests := []struct {
		name          string
		layerTypes    []string
		layerTypesErr error
		cacheCount    int64
		wantCode      codes.Code
		wantCount     int
	}{
		{
			name:       "successful empty layers",
			layerTypes: []string{},
			wantCode:   codes.OK,
			wantCount:  0,
		},
		{
			name:       "successful with layers",
			layerTypes: []string{"flights_commercial", "satellites"},
			cacheCount: 100,
			wantCode:   codes.OK,
			wantCount:  2,
		},
		{
			name:          "repo error",
			layerTypes:    nil,
			layerTypesErr: fmt.Errorf("db error"),
			wantCode:      codes.Internal,
		},
		{
			name:       "single layer",
			layerTypes: []string{"marine_vessels"},
			cacheCount: 50,
			wantCode:   codes.OK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				layerTypes:    tt.layerTypes,
				layerTypesErr: tt.layerTypesErr,
			}
			oRepo := &testObsRepo{}
			cache := &testCacheStorage{totalCount: tt.cacheCount}
			svc := layer.NewLayerService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
			srv := NewLayerServer(svc)

			resp, err := srv.GetLayers(context.Background(), &respondentv1.GetLayersRequest{})

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
			if len(resp.Layers) != tt.wantCount {
				t.Errorf("expected %d layers, got %d", tt.wantCount, len(resp.Layers))
			}
			for i, layer := range resp.Layers {
				if layer.Id == "" {
					t.Errorf("layer %d: expected non-empty id", i)
				}
				if layer.Name == "" {
					t.Errorf("layer %d: expected non-empty name", i)
				}
			}
		})
	}
}

func TestLayerServer_ToggleLayer(t *testing.T) {
	tests := []struct {
		name     string
		req      *respondentv1.ToggleLayerRequest
		wantCode codes.Code
		wantID   string
		wantMode respondentv1.LayerMode
	}{
		{
			name:     "nil toggle",
			req:      &respondentv1.ToggleLayerRequest{Toggle: nil},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "enable layer",
			req:      &respondentv1.ToggleLayerRequest{Toggle: &respondentv1.LayerToggle{LayerId: "flights_commercial", Enabled: true, Mode: respondentv1.LayerMode_LAYER_MODE_FULL, Density: 100}},
			wantCode: codes.OK,
			wantID:   "flights_commercial",
			wantMode: respondentv1.LayerMode_LAYER_MODE_FULL,
		},
		{
			name:     "disable layer",
			req:      &respondentv1.ToggleLayerRequest{Toggle: &respondentv1.LayerToggle{LayerId: "satellites", Enabled: false, Mode: respondentv1.LayerMode_LAYER_MODE_SPARSE, Density: 50}},
			wantCode: codes.OK,
			wantID:   "satellites",
			wantMode: respondentv1.LayerMode_LAYER_MODE_SPARSE,
		},
		{
			name:     "empty layer_id",
			req:      &respondentv1.ToggleLayerRequest{Toggle: &respondentv1.LayerToggle{LayerId: "", Enabled: true, Mode: respondentv1.LayerMode_LAYER_MODE_FULL, Density: 100}},
			wantCode: codes.OK,
			wantID:   "",
			wantMode: respondentv1.LayerMode_LAYER_MODE_FULL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{}
			oRepo := &testObsRepo{}
			cache := &testCacheStorage{}
			svc := layer.NewLayerService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
			srv := NewLayerServer(svc)

			resp, err := srv.ToggleLayer(context.Background(), tt.req)

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
			if resp.Layer == nil {
				t.Fatal("expected layer in response, got nil")
			}
			if resp.Layer.Id != tt.wantID {
				t.Errorf("expected layer_id %s, got %s", tt.wantID, resp.Layer.Id)
			}
			if resp.Layer.Mode != tt.wantMode {
				t.Errorf("expected mode %s, got %s", tt.wantMode, resp.Layer.Mode)
			}
		})
	}
}

func TestLayerServer_GetLayerSnapshot(t *testing.T) {
	now := time.Now()

	entities := []*domain.Entity{
		{ID: "uuid-1", ExternalID: "UAL1234", LayerType: "flights_commercial", Name: "UAL1234"},
		{ID: "uuid-2", ExternalID: "DAL5678", LayerType: "flights_commercial", Name: "DAL5678"},
	}

	observations := []*domain.Observation{
		{EntityID: "uuid-1", Timestamp: now, Position: &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194}, AltitudeM: 10000},
		{EntityID: "uuid-2", Timestamp: now, Position: &domain.GeoPoint{Lat: 37.0, Lon: -122.0}, AltitudeM: 9000},
	}

	tests := []struct {
		name            string
		req             *respondentv1.GetLayerSnapshotRequest
		cacheEntities   []*domain.Entity
		cacheObs        []*domain.Observation
		cacheCount      int64
		cacheErr        error
		dbEntities      []*domain.Entity
		dbEntityErr     error
		dbObs           []*domain.Observation
		dbObsErr        error
		wantCode        codes.Code
		wantEntityCount int
		wantObsCount    int
		wantHasMore     bool
	}{
		{
			name:     "empty layer_id",
			req:      &respondentv1.GetLayerSnapshotRequest{LayerId: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name:            "cache hit",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 100},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      2,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:            "cache miss fallback to db",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 100},
			cacheErr:        fmt.Errorf("cache miss"),
			dbEntities:      entities,
			dbObs:           observations,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:     "db error on observation",
			req:      &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 100},
			cacheErr: fmt.Errorf("cache miss"),
			dbObsErr: fmt.Errorf("db error"),
			wantCode: codes.Internal,
		},
		{
			name:            "empty cache returns db data",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 100},
			cacheEntities:   nil,
			cacheObs:        nil,
			cacheCount:      0,
			dbEntities:      entities,
			dbObs:           observations,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:            "default limit",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial"},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      2,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:            "zero limit defaults to 500",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 0},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      2,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:            "negative limit defaults to 500",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: -10},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      2,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
		{
			name:            "has_more true",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 1},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      10,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
			wantHasMore:     true,
		},
		{
			name:            "with offset",
			req:             &respondentv1.GetLayerSnapshotRequest{LayerId: "flights_commercial", Limit: 100, Offset: 1},
			cacheEntities:   entities,
			cacheObs:        observations,
			cacheCount:      2,
			wantCode:        codes.OK,
			wantEntityCount: 2,
			wantObsCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				getByIDEntity: entities[0],
			}
			oRepo := &testObsRepo{
				latestForLayer:    tt.dbObs,
				latestForLayerErr: tt.dbObsErr,
			}
			cache := &testCacheStorage{
				entities:     tt.cacheEntities,
				observations: tt.cacheObs,
				totalCount:   tt.cacheCount,
				err:          tt.cacheErr,
			}
			svc := layer.NewLayerService(eRepo, oRepo, cache, domain.NewDynamicSourceRegistry())
			srv := NewLayerServer(svc)

			resp, err := srv.GetLayerSnapshot(context.Background(), tt.req)

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
			if len(resp.Entities) != tt.wantEntityCount {
				t.Errorf("expected %d entities, got %d", tt.wantEntityCount, len(resp.Entities))
			}
			if len(resp.Observations) != tt.wantObsCount {
				t.Errorf("expected %d observations, got %d", tt.wantObsCount, len(resp.Observations))
			}
			if resp.HasMore != tt.wantHasMore {
				t.Errorf("expected HasMore %v, got %v", tt.wantHasMore, resp.HasMore)
			}
		})
	}
}

func TestDomainLayerToProto_NilLayer(t *testing.T) {
	if got := domainLayerToProto(nil); got != nil {
		t.Errorf("domainLayerToProto(nil) = %+v, want nil", got)
	}
}

func TestDomainLayerToProto_WithHistoryConfig(t *testing.T) {
	tests := []struct {
		name              string
		maxLookbackHours  int32
		maxRangeSpanHours int32
	}{
		{
			name:              "one year lookback, one week span",
			maxLookbackHours:  8760,
			maxRangeSpanHours: 168,
		},
		{
			name:              "small window",
			maxLookbackHours:  24,
			maxRangeSpanHours: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := &domain.Layer{
				ID:         "marine_ais",
				Name:       "Marine AIS",
				Type:       "marine_vessels",
				Enabled:    true,
				Mode:       "full",
				Density:    100,
				Source:     "feeder",
				LastUpdate: time.Now(),
				HistoryConfig: &domain.HistoryConfig{
					MaxLookbackHours:  tt.maxLookbackHours,
					MaxRangeSpanHours: tt.maxRangeSpanHours,
				},
			}

			proto := domainLayerToProto(layer)
			if proto == nil {
				t.Fatal("domainLayerToProto returned nil for non-nil layer")
			}
			if proto.HistoryConfig == nil {
				t.Fatal("proto.HistoryConfig is nil, want non-nil")
			}
			if proto.HistoryConfig.MaxLookbackHours != tt.maxLookbackHours {
				t.Errorf("MaxLookbackHours = %d, want %d",
					proto.HistoryConfig.MaxLookbackHours, tt.maxLookbackHours)
			}
			if proto.HistoryConfig.MaxRangeSpanHours != tt.maxRangeSpanHours {
				t.Errorf("MaxRangeSpanHours = %d, want %d",
					proto.HistoryConfig.MaxRangeSpanHours, tt.maxRangeSpanHours)
			}
		})
	}
}

func TestDomainLayerToProto_NilHistoryConfig(t *testing.T) {
	layer := &domain.Layer{
		ID:            "usgs_earthquakes",
		Name:          "USGS Earthquakes",
		Type:          "earthquakes",
		Enabled:       true,
		Mode:          "full",
		Density:       100,
		Source:        "feeder",
		LastUpdate:    time.Now(),
		HistoryConfig: nil,
	}

	proto := domainLayerToProto(layer)
	if proto == nil {
		t.Fatal("domainLayerToProto returned nil for non-nil layer")
	}
	if proto.HistoryConfig != nil {
		t.Errorf("proto.HistoryConfig = %+v, want nil when domain HistoryConfig is nil",
			proto.HistoryConfig)
	}
}

func TestDomainLayerToProto_ScalarFields(t *testing.T) {
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	layer := &domain.Layer{
		ID:         "adsb_lol_flights",
		Name:       "ADS-B Flights",
		Type:       "flights_commercial",
		Enabled:    true,
		Mode:       "sparse",
		Density:    75,
		Source:     "feeder",
		LastUpdate: ts,
		Count:      42,
		Color:      "#00ff9d",
		PointSize:  8,
	}

	proto := domainLayerToProto(layer)
	if proto == nil {
		t.Fatal("domainLayerToProto returned nil for non-nil layer")
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ID", proto.Id, layer.ID},
		{"Name", proto.Name, layer.Name},
		{"Type", proto.Type, layer.Type},
		{"Enabled", proto.Enabled, layer.Enabled},
		{"Mode", proto.Mode, respondentv1.LayerMode(domain.LayerModeFromString(layer.Mode))},
		{"Density", proto.Density, layer.Density},
		{"Source", proto.Source, layer.Source},
		{"LastUpdate", proto.LastUpdate, ts.UnixMilli()},
		{"Count", proto.Count, layer.Count},
		{"Color", proto.Color, layer.Color},
		{"PointSize", proto.PointSize, layer.PointSize},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestDomainLayerToProto_RenderingAndFilteringModes(t *testing.T) {
	layer := &domain.Layer{
		ID:            "test_layer",
		Name:          "Test Layer",
		Type:          "test_type",
		Enabled:       true,
		RenderingMode: "indicator",
		FilteringMode: "viewport",
		LastUpdate:    time.Now(),
	}

	proto := domainLayerToProto(layer)
	if proto == nil {
		t.Fatal("domainLayerToProto returned nil for non-nil layer")
	}
	if proto.RenderingMode != respondentv1.RenderingMode_RENDERING_MODE_INDICATOR {
		t.Errorf("RenderingMode = %v, want RENDERING_MODE_INDICATOR", proto.RenderingMode)
	}
	if proto.FilteringMode != respondentv1.FilteringMode_FILTERING_MODE_VIEWPORT {
		t.Errorf("FilteringMode = %v, want FILTERING_MODE_VIEWPORT", proto.FilteringMode)
	}
}

func TestDomainLayerToProto_WithDisplayConfig(t *testing.T) {
	precision := int32(2)
	layer := &domain.Layer{
		ID:         "test_layer",
		Name:       "Test Layer",
		Type:       "test_type",
		Enabled:    true,
		LastUpdate: time.Now(),
		DisplayConfig: &domain.LayerDisplayConfig{
			Icon: &domain.IconConfig{
				Shape:         "plane",
				Rotatable:     true,
				Interpolation: true,
				Scale:         1.5,
			},
			Trail: &domain.TrailConfig{
				Color:   "#ff0000",
				Width:   2.0,
				Opacity: 0.8,
			},
			Style: &domain.StyleConfig{
				Color:     "#00ff00",
				PointSize: 10,
			},
			FieldRenderers: []domain.FieldRendererConfig{
				{
					Keys:     []string{"speed"},
					Label:    "Speed",
					Priority: 1,
					Format: domain.FieldFormat{
						Type:      "number",
						Precision: &precision,
						Prefix:    "",
						Suffix:    " kts",
						Transform: "",
					},
				},
			},
		},
	}

	proto := domainLayerToProto(layer)
	if proto == nil {
		t.Fatal("domainLayerToProto returned nil for non-nil layer")
	}
	if proto.DisplayConfig == nil {
		t.Fatal("proto.DisplayConfig is nil, want non-nil")
	}
	if proto.DisplayConfig.Icon == nil {
		t.Fatal("proto.DisplayConfig.Icon is nil, want non-nil")
	}
	if proto.DisplayConfig.Icon.Shape != "plane" {
		t.Errorf("Icon.Shape = %v, want %q", proto.DisplayConfig.Icon.Shape, "plane")
	}
	if !proto.DisplayConfig.Icon.Rotatable {
		t.Error("Icon.Rotatable = false, want true")
	}
	if proto.DisplayConfig.Icon.Scale != 1.5 {
		t.Errorf("Icon.Scale = %f, want 1.5", proto.DisplayConfig.Icon.Scale)
	}
	if proto.DisplayConfig.Trail == nil {
		t.Fatal("proto.DisplayConfig.Trail is nil, want non-nil")
	}
	if proto.DisplayConfig.Trail.Color != "#ff0000" {
		t.Errorf("Trail.Color = %s, want #ff0000", proto.DisplayConfig.Trail.Color)
	}
	if proto.DisplayConfig.Style == nil {
		t.Fatal("proto.DisplayConfig.Style is nil, want non-nil")
	}
	if proto.DisplayConfig.Style.Color != "#00ff00" {
		t.Errorf("Style.Color = %s, want #00ff00", proto.DisplayConfig.Style.Color)
	}
	if len(proto.DisplayConfig.FieldRenderers) != 1 {
		t.Fatalf("FieldRenderers count = %d, want 1", len(proto.DisplayConfig.FieldRenderers))
	}
	if proto.DisplayConfig.FieldRenderers[0].Label != "Speed" {
		t.Errorf("FieldRenderers[0].Label = %s, want Speed", proto.DisplayConfig.FieldRenderers[0].Label)
	}
	if proto.DisplayConfig.FieldRenderers[0].Format.Precision == nil || *proto.DisplayConfig.FieldRenderers[0].Format.Precision != 2 {
		t.Errorf("FieldRenderers[0].Format.Precision = %v, want 2", proto.DisplayConfig.FieldRenderers[0].Format.Precision)
	}
}

func TestDomainDisplayConfigToProto(t *testing.T) {
	precision := int32(3)
	tests := []struct {
		name   string
		dc     *domain.LayerDisplayConfig
		checks func(t *testing.T, pdc *respondentv1.LayerDisplayConfig)
	}{
		{
			name: "icon only",
			dc: &domain.LayerDisplayConfig{
				Icon: &domain.IconConfig{
					Shape:         "marker",
					Rotatable:     false,
					Interpolation: true,
					Scale:         2.0,
				},
			},
			checks: func(t *testing.T, pdc *respondentv1.LayerDisplayConfig) {
				if pdc.Icon == nil {
					t.Fatal("Icon is nil")
				}
				if pdc.Icon.Shape != "marker" {
					t.Errorf("Icon.Shape = %v, want %q", pdc.Icon.Shape, "marker")
				}
				if pdc.Trail != nil {
					t.Error("Trail should be nil")
				}
			},
		},
		{
			name: "trail only",
			dc: &domain.LayerDisplayConfig{
				Trail: &domain.TrailConfig{
					Color:   "#blue",
					Width:   3.5,
					Opacity: 0.5,
				},
			},
			checks: func(t *testing.T, pdc *respondentv1.LayerDisplayConfig) {
				if pdc.Trail == nil {
					t.Fatal("Trail is nil")
				}
				if pdc.Trail.Width != 3.5 {
					t.Errorf("Trail.Width = %f, want 3.5", pdc.Trail.Width)
				}
			},
		},
		{
			name: "style only",
			dc: &domain.LayerDisplayConfig{
				Style: &domain.StyleConfig{
					Color:     "#red",
					PointSize: 15,
				},
			},
			checks: func(t *testing.T, pdc *respondentv1.LayerDisplayConfig) {
				if pdc.Style == nil {
					t.Fatal("Style is nil")
				}
				if pdc.Style.PointSize != 15 {
					t.Errorf("Style.PointSize = %d, want 15", pdc.Style.PointSize)
				}
			},
		},
		{
			name: "field renderers",
			dc: &domain.LayerDisplayConfig{
				FieldRenderers: []domain.FieldRendererConfig{
					{
						Keys:     []string{"altitude"},
						Label:    "Altitude",
						Priority: 2,
						Format: domain.FieldFormat{
							Type:      "number",
							Precision: &precision,
							Suffix:    " ft",
						},
					},
					{
						Keys:     []string{"heading"},
						Label:    "Heading",
						Priority: 1,
						Format: domain.FieldFormat{
							Type:      "number",
							Suffix:    "°",
							Transform: "normalize",
						},
					},
				},
			},
			checks: func(t *testing.T, pdc *respondentv1.LayerDisplayConfig) {
				if len(pdc.FieldRenderers) != 2 {
					t.Fatalf("FieldRenderers count = %d, want 2", len(pdc.FieldRenderers))
				}
				if pdc.FieldRenderers[0].Format.Precision == nil || *pdc.FieldRenderers[0].Format.Precision != 3 {
					t.Errorf("FieldRenderers[0].Format.Precision = %v, want 3", pdc.FieldRenderers[0].Format.Precision)
				}
				wantTransform := respondentv1.FieldTransform(domain.FieldTransformFromString("normalize"))
				if pdc.FieldRenderers[1].Format.Transform != wantTransform {
					t.Errorf("FieldRenderers[1].Format.Transform = %v, want %v (from domain 'normalize')", pdc.FieldRenderers[1].Format.Transform, wantTransform)
				}
			},
		},
		{
			name: "empty display config",
			dc:   &domain.LayerDisplayConfig{},
			checks: func(t *testing.T, pdc *respondentv1.LayerDisplayConfig) {
				if pdc.Icon != nil {
					t.Error("Icon should be nil")
				}
				if pdc.Trail != nil {
					t.Error("Trail should be nil")
				}
				if pdc.Style != nil {
					t.Error("Style should be nil")
				}
				if len(pdc.FieldRenderers) != 0 {
					t.Errorf("FieldRenderers count = %d, want 0", len(pdc.FieldRenderers))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto := domainDisplayConfigToProto(tt.dc)
			if proto == nil {
				t.Fatal("domainDisplayConfigToProto returned nil")
			}
			tt.checks(t, proto)
		})
	}
}

func TestDomainLayerToProto_EmptyRenderingAndFilteringModes(t *testing.T) {
	layer := &domain.Layer{
		ID:            "test_layer",
		Name:          "Test Layer",
		Type:          "test_type",
		Enabled:       true,
		RenderingMode: "",
		FilteringMode: "",
		LastUpdate:    time.Now(),
	}

	proto := domainLayerToProto(layer)
	if proto == nil {
		t.Fatal("domainLayerToProto returned nil for non-nil layer")
	}
	if proto.RenderingMode != respondentv1.RenderingMode_RENDERING_MODE_UNSPECIFIED {
		t.Errorf("RenderingMode = %v, want RENDERING_MODE_UNSPECIFIED", proto.RenderingMode)
	}
	if proto.FilteringMode != respondentv1.FilteringMode_FILTERING_MODE_UNSPECIFIED {
		t.Errorf("FilteringMode = %v, want FILTERING_MODE_UNSPECIFIED", proto.FilteringMode)
	}
}
