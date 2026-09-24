package grpctransport

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	entitysvc "github.com/Alevsk/respondent/internal/app/entity"
	"github.com/Alevsk/respondent/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ domain.EntityRepository      = (*testEntityRepo)(nil)
	_ domain.ObservationRepository = (*testObsRepo)(nil)
)

type testEntityRepo struct {
	searchResults    []*domain.EntitySearchResult
	searchCount      int
	searchErr        error
	entities         map[string]*domain.Entity
	entityByExtID    *domain.Entity
	entityByExtErr   error
	entityByExtIDs   []*domain.Entity
	entityByExtIDsFn func(layerType string, externalIDs []string) ([]*domain.Entity, error)
	layerTypes       []string
	layerTypesErr    error
	getByIDEntity    *domain.Entity
	getByIDErr       error
}

func (s *testEntityRepo) Create(_ context.Context, _ *domain.Entity) error { return nil }
func (s *testEntityRepo) CreateBatch(_ context.Context, _ []*domain.Entity) error {
	return nil
}
func (s *testEntityRepo) GetByID(_ context.Context, _ string) (*domain.Entity, error) {
	return s.getByIDEntity, s.getByIDErr
}
func (s *testEntityRepo) GetByIDs(_ context.Context, ids []string) ([]*domain.Entity, error) {
	if s.entities != nil {
		var out []*domain.Entity
		for _, id := range ids {
			for _, e := range s.entities {
				if e.ID == id {
					out = append(out, e)
					break
				}
			}
		}
		return out, nil
	}
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	if s.getByIDEntity != nil {
		// Return a synthetic entity per requested ID so callers that need distinct
		// entities (keyed by ID) work correctly in tests.
		out := make([]*domain.Entity, len(ids))
		for i, id := range ids {
			copy := *s.getByIDEntity
			copy.ID = id
			out[i] = &copy
		}
		return out, nil
	}
	return nil, nil
}
func (s *testEntityRepo) GetByExternalID(_ context.Context, _, _ string) (*domain.Entity, error) {
	return s.entityByExtID, s.entityByExtErr
}
func (s *testEntityRepo) GetByExternalIDs(_ context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	if s.entityByExtIDsFn != nil {
		return s.entityByExtIDsFn(layerType, externalIDs)
	}
	if s.entityByExtIDs != nil {
		return s.entityByExtIDs, nil
	}
	if s.entities == nil {
		return nil, nil
	}
	var out []*domain.Entity
	for _, eid := range externalIDs {
		key := layerType + ":" + eid
		if e, ok := s.entities[key]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}
func (s *testEntityRepo) GetDistinctLayerTypes(_ context.Context) ([]string, error) {
	return s.layerTypes, s.layerTypesErr
}
func (s *testEntityRepo) CountByLayerType(_ context.Context) (map[string]int64, error) {
	if s.layerTypesErr != nil {
		return nil, s.layerTypesErr
	}
	counts := make(map[string]int64)
	for _, e := range s.entities {
		counts[e.LayerType]++
	}
	return counts, nil
}
func (s *testEntityRepo) Update(_ context.Context, _ *domain.Entity) error { return nil }
func (s *testEntityRepo) Delete(_ context.Context, _ string) error         { return nil }
func (s *testEntityRepo) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return s.searchResults, s.searchCount, s.searchErr
}

func (s *testEntityRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	panic("not implemented")
}

func (s *testEntityRepo) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

type testObsRepo struct {
	observations         map[string]*domain.Observation
	latestObservation    *domain.Observation
	latestObsErr         error
	observationsByEntity []*domain.Observation
	obsByEntityErr       error
	layerPage            []*domain.EntitySnapshot
	layerPageErr         error
	pageableTotal        int64
}

func (s *testObsRepo) Create(_ context.Context, _ *domain.Observation) error { return nil }
func (s *testObsRepo) CreateBatch(_ context.Context, _ []*domain.Observation) error {
	return nil
}
func (s *testObsRepo) CreateBatchUpsert(_ context.Context, _ []*domain.Observation) error {
	return nil
}
func (s *testObsRepo) GetByEntityID(_ context.Context, _ string, _ int, _ time.Time) ([]*domain.Observation, error) {
	return s.observationsByEntity, s.obsByEntityErr
}
func (s *testObsRepo) GetLatest(_ context.Context, _ string) (*domain.Observation, error) {
	return s.latestObservation, s.latestObsErr
}
func (s *testObsRepo) CountLatestForLayer(_ context.Context, _ string) (int64, error) {
	if s.layerPageErr != nil {
		return 0, s.layerPageErr
	}
	if s.pageableTotal > 0 {
		return s.pageableTotal, nil
	}
	return int64(len(s.layerPage)), nil
}

func (s *testObsRepo) GetLatestForLayerPage(_ context.Context, _ string, _, _ int) ([]*domain.EntitySnapshot, error) {
	if s.layerPageErr != nil {
		return nil, s.layerPageErr
	}
	return s.layerPage, nil
}
func (s *testObsRepo) GetLatestForEntityIDs(_ context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	if s.observations == nil {
		return make(map[string]*domain.Observation), nil
	}
	result := make(map[string]*domain.Observation, len(entityIDs))
	for _, id := range entityIDs {
		if o, ok := s.observations[id]; ok {
			result[id] = o
		}
	}
	return result, nil
}
func (s *testObsRepo) GetLatestContentHashes(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}
func (s *testObsRepo) GetLayerSnapshotAt(_ context.Context, _ string, _ time.Time, _ time.Duration, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (s *testObsRepo) GetLatestForLayerByBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (s *testObsRepo) GetLatestByCurrentPositionInBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (s *testObsRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	panic("not implemented")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestEntityServer_SearchEntities(t *testing.T) {
	now := time.Now()

	sampleResults := []*domain.EntitySearchResult{
		{
			Entity: domain.Entity{
				ID:         "uuid-1",
				ExternalID: "UAL1234",
				LayerType:  "flights_commercial",
				Name:       "UAL1234",
				Metadata:   map[string]string{"icao24": "abc123"},
			},
			LatestObservation: &domain.Observation{
				EntityID:  "uuid-1",
				Timestamp: now,
				Position:  &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194},
				AltitudeM: 10000,
				Velocity:  map[string]float64{"speed": 250.5},
			},
		},
		{
			Entity: domain.Entity{
				ID:         "uuid-2",
				ExternalID: "UAL5678",
				LayerType:  "flights_commercial",
				Name:       "UAL5678",
			},
		},
	}

	tests := []struct {
		name        string
		req         *respondentv1.SearchEntitiesRequest
		repoResults []*domain.EntitySearchResult
		repoCount   int
		repoErr     error
		wantCode    codes.Code
		wantCount   int
	}{
		{
			name:        "successful search",
			req:         &respondentv1.SearchEntitiesRequest{Query: "UAL", Limit: 20},
			repoResults: sampleResults,
			repoCount:   2,
			wantCode:    codes.OK,
			wantCount:   2,
		},
		{
			name:        "search with layer filter",
			req:         &respondentv1.SearchEntitiesRequest{Query: "UAL", LayerType: "flights_commercial", Limit: 20},
			repoResults: sampleResults,
			repoCount:   2,
			wantCode:    codes.OK,
			wantCount:   2,
		},
		{
			name:     "query too short",
			req:      &respondentv1.SearchEntitiesRequest{Query: "U", Limit: 20},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "empty query",
			req:      &respondentv1.SearchEntitiesRequest{Query: "", Limit: 20},
			wantCode: codes.InvalidArgument,
		},
		{
			name:        "default limit",
			req:         &respondentv1.SearchEntitiesRequest{Query: "UAL"},
			repoResults: sampleResults,
			repoCount:   2,
			wantCode:    codes.OK,
			wantCount:   2,
		},
		{
			name:     "repo error",
			req:      &respondentv1.SearchEntitiesRequest{Query: "UAL", Limit: 20},
			repoErr:  fmt.Errorf("db error"),
			wantCode: codes.Internal,
		},
		{
			name:        "no results",
			req:         &respondentv1.SearchEntitiesRequest{Query: "NONEXISTENT", Limit: 20},
			repoResults: nil,
			repoCount:   0,
			wantCode:    codes.OK,
			wantCount:   0,
		},
		{
			name:        "result has latest observation",
			req:         &respondentv1.SearchEntitiesRequest{Query: "UAL1234", Limit: 5},
			repoResults: sampleResults[:1],
			repoCount:   1,
			wantCode:    codes.OK,
			wantCount:   1,
		},
		{
			name:        "result without latest observation",
			req:         &respondentv1.SearchEntitiesRequest{Query: "UAL5678", Limit: 5},
			repoResults: sampleResults[1:2],
			repoCount:   1,
			wantCode:    codes.OK,
			wantCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				searchResults: tt.repoResults,
				searchCount:   tt.repoCount,
				searchErr:     tt.repoErr,
			}
			oRepo := &testObsRepo{}
			svc := entitysvc.NewEntityService(eRepo, oRepo)
			srv := NewEntityServer(svc)

			resp, err := srv.SearchEntities(context.Background(), tt.req)

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
			if len(resp.Results) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(resp.Results))
			}
			if int(resp.TotalCount) != tt.repoCount {
				t.Errorf("expected total_count %d, got %d", tt.repoCount, resp.TotalCount)
			}

			// Verify entity_id format for results with observations
			for _, r := range resp.Results {
				expectedID := r.LayerType + ":" + r.ExternalId
				if r.EntityId != expectedID {
					t.Errorf("expected entity_id %q, got %q", expectedID, r.EntityId)
				}
			}

			// Verify observation mapping for the first result if it has one
			if tt.wantCount > 0 && tt.repoResults[0].LatestObservation != nil {
				first := resp.Results[0]
				if first.LatestObservation == nil {
					t.Error("expected latest_observation, got nil")
				} else if first.LatestObservation.Position == nil {
					t.Error("expected position in observation, got nil")
				}
			}
		})
	}
}

func TestEntityServer_GetEntitiesBatch(t *testing.T) {
	now := time.Now()

	entity1 := &domain.Entity{
		ID:         "uuid-1",
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
		Metadata:   map[string]string{"callsign": "AAL1234"},
	}
	entity2 := &domain.Entity{
		ID:         "uuid-2",
		ExternalID: "25544",
		LayerType:  "satellites",
		Name:       "ISS",
	}

	obs1 := &domain.Observation{
		EntityID:  "uuid-1",
		Timestamp: now,
		Position:  &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194},
		AltitudeM: 10000,
		Velocity:  map[string]float64{"speed": 250.5},
	}

	tests := []struct {
		name             string
		req              *respondentv1.GetEntitiesBatchRequest
		entities         map[string]*domain.Entity
		entityByExtIDsFn func(layerType string, externalIDs []string) ([]*domain.Entity, error)
		obsMap           map[string]*domain.Observation
		wantCode         codes.Code
		wantCount        int
	}{
		{
			name: "successful batch lookup",
			req: &respondentv1.GetEntitiesBatchRequest{
				EntityIds: []string{"flights_commercial:AAL1234", "satellites:25544"},
			},
			entities: map[string]*domain.Entity{
				"flights_commercial:AAL1234": entity1,
				"satellites:25544":           entity2,
			},
			obsMap: map[string]*domain.Observation{
				"uuid-1": obs1,
			},
			wantCode:  codes.OK,
			wantCount: 2,
		},
		{
			name: "empty entity_ids",
			req: &respondentv1.GetEntitiesBatchRequest{
				EntityIds: []string{},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "exceeds max batch size",
			req: &respondentv1.GetEntitiesBatchRequest{
				EntityIds: make([]string, 51),
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "partial results - some not found",
			req: &respondentv1.GetEntitiesBatchRequest{
				EntityIds: []string{"flights_commercial:AAL1234", "flights_commercial:NONEXISTENT"},
			},
			entities: map[string]*domain.Entity{
				"flights_commercial:AAL1234": entity1,
			},
			obsMap: map[string]*domain.Observation{
				"uuid-1": obs1,
			},
			wantCode:  codes.OK,
			wantCount: 1,
		},
		{
			name: "service error",
			req: &respondentv1.GetEntitiesBatchRequest{
				EntityIds: []string{"flights_commercial:AAL1234"},
			},
			entityByExtIDsFn: func(layerType string, externalIDs []string) ([]*domain.Entity, error) {
				return nil, fmt.Errorf("db error")
			},
			wantCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				entities:         tt.entities,
				entityByExtIDsFn: tt.entityByExtIDsFn,
			}
			oRepo := &testObsRepo{observations: tt.obsMap}
			svc := entitysvc.NewEntityService(eRepo, oRepo)
			srv := NewEntityServer(svc)

			resp, err := srv.GetEntitiesBatch(context.Background(), tt.req)

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
			if len(resp.Entities) != tt.wantCount {
				t.Errorf("expected %d entities, got %d", tt.wantCount, len(resp.Entities))
			}

			// Verify proto conversion
			for _, bd := range resp.Entities {
				if bd.Entity == nil {
					t.Error("expected entity in BatchEntityDetail, got nil")
				}
			}
		})
	}
}

func TestEntityServer_GetEntityDetail(t *testing.T) {
	now := time.Now()

	entity := &domain.Entity{
		ID:         "uuid-1",
		ExternalID: "UAL1234",
		LayerType:  "flights_commercial",
		Name:       "UAL1234",
		Metadata:   map[string]string{"icao24": "abc123"},
	}

	observation := &domain.Observation{
		EntityID:  "uuid-1",
		Timestamp: now,
		Position:  &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194},
		AltitudeM: 10000,
		Velocity:  map[string]float64{"speed": 250.5},
	}

	tests := []struct {
		name         string
		req          *respondentv1.GetEntityDetailRequest
		entity       *domain.Entity
		entityErr    error
		obs          *domain.Observation
		obsErr       error
		wantCode     codes.Code
		wantEntityID string
		wantObsNil   bool
	}{
		{
			name:     "empty entity_id",
			req:      &respondentv1.GetEntityDetailRequest{EntityId: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name:      "entity not found",
			req:       &respondentv1.GetEntityDetailRequest{EntityId: "flights_commercial:NONEXISTENT"},
			entityErr: domain.NewNotFoundError("entity not found", nil),
			wantCode:  codes.NotFound,
		},
		{
			name:         "successful lookup with observation",
			req:          &respondentv1.GetEntityDetailRequest{EntityId: "flights_commercial:UAL1234"},
			entity:       entity,
			obs:          observation,
			wantCode:     codes.OK,
			wantEntityID: "uuid-1",
			wantObsNil:   false,
		},
		{
			name:         "successful lookup without observation",
			req:          &respondentv1.GetEntityDetailRequest{EntityId: "flights_commercial:UAL1234"},
			entity:       entity,
			obs:          nil,
			wantCode:     codes.OK,
			wantEntityID: "uuid-1",
			wantObsNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				entityByExtID:  tt.entity,
				entityByExtErr: tt.entityErr,
			}
			oRepo := &testObsRepo{
				latestObservation: tt.obs,
				latestObsErr:      tt.obsErr,
			}
			svc := entitysvc.NewEntityService(eRepo, oRepo)
			srv := NewEntityServer(svc)

			resp, err := srv.GetEntityDetail(context.Background(), tt.req)

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
			if resp.Entity == nil {
				t.Fatal("expected entity in response, got nil")
			}
			if resp.Entity.Id != tt.wantEntityID {
				t.Errorf("expected entity_id %s, got %s", tt.wantEntityID, resp.Entity.Id)
			}
			if (resp.LatestObservation == nil) != tt.wantObsNil {
				t.Errorf("expected LatestObservation nil=%v, got nil=%v", tt.wantObsNil, resp.LatestObservation == nil)
			}
			if !tt.wantObsNil && resp.LatestObservation != nil {
				if resp.LatestObservation.EntityId != observation.EntityID {
					t.Errorf("expected observation entity_id %s, got %s", observation.EntityID, resp.LatestObservation.EntityId)
				}
			}
		})
	}
}

func TestEntityServer_GetObservationHistory(t *testing.T) {
	now := time.Now()

	entity := &domain.Entity{
		ID:         "uuid-1",
		ExternalID: "UAL1234",
		LayerType:  "flights_commercial",
		Name:       "UAL1234",
	}

	observations := []*domain.Observation{
		{
			EntityID:  "uuid-1",
			Timestamp: now,
			Position:  &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194},
			AltitudeM: 10000,
		},
		{
			EntityID:  "uuid-1",
			Timestamp: now.Add(-time.Hour),
			Position:  &domain.GeoPoint{Lat: 37.0, Lon: -122.0},
			AltitudeM: 9000,
		},
	}

	tests := []struct {
		name        string
		req         *respondentv1.GetObservationHistoryRequest
		entity      *domain.Entity
		entityErr   error
		obs         []*domain.Observation
		obsErr      error
		wantCode    codes.Code
		wantCount   int
		wantHasMore bool
	}{
		{
			name:     "empty entity_id",
			req:      &respondentv1.GetObservationHistoryRequest{EntityId: ""},
			wantCode: codes.InvalidArgument,
		},
		{
			name:      "entity not found",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:NONEXISTENT"},
			entityErr: fmt.Errorf("not found"),
			wantCode:  codes.Internal,
		},
		{
			name:     "repo error on observations",
			req:      &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 10},
			entity:   entity,
			obsErr:   fmt.Errorf("db error"),
			wantCode: codes.Internal,
		},
		{
			name:      "successful lookup with default limit",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234"},
			entity:    entity,
			obs:       observations,
			wantCode:  codes.OK,
			wantCount: 2,
		},
		{
			name:      "successful lookup with custom limit",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 1},
			entity:    entity,
			obs:       observations[:1],
			wantCode:  codes.OK,
			wantCount: 1,
		},
		{
			name:      "successful lookup with before_ms",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 10, BeforeMs: now.Add(-30 * time.Minute).UnixMilli()},
			entity:    entity,
			obs:       observations[1:],
			wantCode:  codes.OK,
			wantCount: 1,
		},
		{
			name:        "has_more true",
			req:         &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 1},
			entity:      entity,
			obs:         append(observations, &domain.Observation{EntityID: "uuid-1", Timestamp: now.Add(-2 * time.Hour)}),
			wantCode:    codes.OK,
			wantCount:   1,
			wantHasMore: true,
		},
		{
			name:      "empty results",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 10},
			entity:    entity,
			obs:       nil,
			wantCode:  codes.OK,
			wantCount: 0,
		},
		{
			name:      "zero limit defaults to 50",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: 0},
			entity:    entity,
			obs:       observations,
			wantCode:  codes.OK,
			wantCount: 2,
		},
		{
			name:      "negative limit defaults to 50",
			req:       &respondentv1.GetObservationHistoryRequest{EntityId: "flights_commercial:UAL1234", Limit: -5},
			entity:    entity,
			obs:       observations,
			wantCode:  codes.OK,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &testEntityRepo{
				entityByExtID:  tt.entity,
				entityByExtErr: tt.entityErr,
			}
			oRepo := &testObsRepo{
				observationsByEntity: tt.obs,
				obsByEntityErr:       tt.obsErr,
			}
			svc := entitysvc.NewEntityService(eRepo, oRepo)
			srv := NewEntityServer(svc)

			resp, err := srv.GetObservationHistory(context.Background(), tt.req)

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
			if len(resp.Observations) != tt.wantCount {
				t.Errorf("expected %d observations, got %d", tt.wantCount, len(resp.Observations))
			}
			if resp.HasMore != tt.wantHasMore {
				t.Errorf("expected HasMore %v, got %v", tt.wantHasMore, resp.HasMore)
			}
			for i, obs := range resp.Observations {
				if obs.Position == nil {
					t.Errorf("observation %d: expected position, got nil", i)
				}
			}
		})
	}
}

func TestNewEntityServer(t *testing.T) {
	eRepo := &testEntityRepo{}
	oRepo := &testObsRepo{}
	svc := entitysvc.NewEntityService(eRepo, oRepo)
	srv := NewEntityServer(svc)
	if srv == nil {
		t.Fatal("expected server, got nil")
	}
}

func TestDomainEntityToProto(t *testing.T) {
	tests := []struct {
		name   string
		entity *domain.Entity
		wantID string
		wantN  string
		isNil  bool
	}{
		{
			name:   "nil entity",
			entity: nil,
			isNil:  true,
		},
		{
			name: "valid entity",
			entity: &domain.Entity{
				ID:         "uuid-1",
				ExternalID: "UAL1234",
				LayerType:  "flights_commercial",
				Name:       "UAL1234",
				Metadata:   map[string]string{"key": "value"},
			},
			wantID: "uuid-1",
			wantN:  "UAL1234",
			isNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainEntityToProto(tt.entity)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil proto entity")
			}
			if got.Id != tt.wantID {
				t.Errorf("expected id %s, got %s", tt.wantID, got.Id)
			}
			if got.Name != tt.wantN {
				t.Errorf("expected name %s, got %s", tt.wantN, got.Name)
			}
		})
	}
}

func TestDomainEntityToProto_AIMetadata(t *testing.T) {
	tests := []struct {
		name               string
		entity             *domain.Entity
		wantAiMetadataJson string
		wantValidJSON      bool
		wantKeys           []string
	}{
		{
			name: "entity without ai_metadata has empty field",
			entity: &domain.Entity{
				ID:        "uuid-1",
				LayerType: "flights_commercial",
				Name:      "UAL1234",
				Metadata:  map[string]string{"icao24": "ABC123"},
			},
			wantAiMetadataJson: "",
		},
		{
			name: "entity with ai_metadata serializes to JSON string",
			entity: &domain.Entity{
				ID:        "uuid-2",
				LayerType: "flights_military",
				Name:      "Military Flight",
				AIMetadata: map[string]any{
					"classification": map[string]any{
						"type":       "military",
						"confidence": 0.95,
					},
					"threat_assessment": map[string]any{
						"risk_level": "elevated",
						"summary":    "Unusual flight pattern",
					},
				},
			},
			wantValidJSON: true,
			wantKeys:      []string{"classification", "threat_assessment"},
		},
		{
			name: "entity with empty ai_metadata map produces no JSON",
			entity: &domain.Entity{
				ID:         "uuid-3",
				LayerType:  "flights_commercial",
				Name:       "DAL5678",
				AIMetadata: map[string]any{},
			},
			wantAiMetadataJson: "",
		},
		{
			name: "entity with nil ai_metadata produces no JSON",
			entity: &domain.Entity{
				ID:         "uuid-4",
				LayerType:  "satellites",
				Name:       "ISS",
				AIMetadata: nil,
			},
			wantAiMetadataJson: "",
		},
		{
			name: "entity with single ai_metadata key",
			entity: &domain.Entity{
				ID:        "uuid-5",
				LayerType: "flights_military",
				Name:      "Recon",
				AIMetadata: map[string]any{
					"classification": map[string]any{"type": "surveillance"},
				},
			},
			wantValidJSON: true,
			wantKeys:      []string{"classification"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainEntityToProto(tt.entity)
			if got == nil {
				t.Fatal("expected non-nil proto entity")
			}

			if tt.wantAiMetadataJson != "" {
				if got.AiMetadataJson != tt.wantAiMetadataJson {
					t.Errorf("AiMetadataJson = %q, want %q", got.AiMetadataJson, tt.wantAiMetadataJson)
				}
				return
			}

			if tt.wantValidJSON {
				if got.AiMetadataJson == "" {
					t.Fatal("AiMetadataJson is empty, expected serialized JSON")
				}
				var parsed map[string]any
				if err := json.Unmarshal([]byte(got.AiMetadataJson), &parsed); err != nil {
					t.Fatalf("AiMetadataJson is not valid JSON: %v\nvalue: %s", err, got.AiMetadataJson)
				}
				for _, key := range tt.wantKeys {
					if _, ok := parsed[key]; !ok {
						t.Errorf("expected key %q in parsed AiMetadataJson, got keys: %v", key, mapKeys(parsed))
					}
				}
				return
			}

			// No AI metadata expected
			if got.AiMetadataJson != "" {
				t.Errorf("AiMetadataJson = %q, want empty string", got.AiMetadataJson)
			}
		})
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestDomainObservationToProto(t *testing.T) {
	now := time.Now()
	eventTime := now.Add(-time.Hour)
	eventEnd := now.Add(time.Hour)

	tests := []struct {
		name          string
		obs           *domain.Observation
		isNil         bool
		wantPos       bool
		wantEventTime bool
		wantEventEnd  bool
	}{
		{
			name:  "nil observation",
			obs:   nil,
			isNil: true,
		},
		{
			name: "full observation",
			obs: &domain.Observation{
				EntityID:  "uuid-1",
				Timestamp: now,
				EventTime: &eventTime,
				EventEnd:  &eventEnd,
				Position:  &domain.GeoPoint{Lat: 37.7749, Lon: -122.4194},
				AltitudeM: 10000,
				Velocity:  map[string]float64{"speed": 250.5},
				Metadata:  map[string]string{"key": "value"},
			},
			isNil:         false,
			wantPos:       true,
			wantEventTime: true,
			wantEventEnd:  true,
		},
		{
			name: "observation without position",
			obs: &domain.Observation{
				EntityID:  "uuid-1",
				Timestamp: now,
				AltitudeM: 10000,
			},
			isNil:         false,
			wantPos:       false,
			wantEventTime: false,
			wantEventEnd:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainObservationToProto(tt.obs)
			if tt.isNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil proto observation")
			}
			if (got.Position != nil) != tt.wantPos {
				t.Errorf("expected position nil=%v, got nil=%v", !tt.wantPos, got.Position == nil)
			}
			if (got.EventTimeMs > 0) != tt.wantEventTime {
				t.Errorf("expected event_time set=%v, got set=%v", tt.wantEventTime, got.EventTimeMs > 0)
			}
			if (got.EventEndMs > 0) != tt.wantEventEnd {
				t.Errorf("expected event_end set=%v, got set=%v", tt.wantEventEnd, got.EventEndMs > 0)
			}
			if got.Ts != tt.obs.Timestamp.UnixMilli() {
				t.Errorf("expected ts %d, got %d", tt.obs.Timestamp.UnixMilli(), got.Ts)
			}
		})
	}
}
