package entity

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestEntityService_GetEntityDetail(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	entityID := "ent-1"
	compositeID := "flights_commercial:AAL1234"

	entity := &domain.Entity{
		ID:         entityID,
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
		Metadata:   map[string]string{"callsign": "AAL1234"},
		CreatedAt:  now,
	}

	obs := &domain.Observation{
		ID:        "obs-1",
		EntityID:  entityID,
		Timestamp: now,
		Position:  &domain.GeoPoint{Lat: 37.77, Lon: -122.42},
		AltitudeM: 10668,
	}

	t.Run("found with observation", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)
		oRepo.add(obs)

		svc := NewEntityService(eRepo, oRepo)
		detail, err := svc.GetEntityDetail(ctx, compositeID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if detail.Entity.ID != entityID {
			t.Errorf("expected entity ID %s, got %s", entityID, detail.Entity.ID)
		}
		if detail.LatestObservation == nil {
			t.Fatal("expected latest observation, got nil")
		}
		if detail.LatestObservation.ID != "obs-1" {
			t.Errorf("expected observation ID obs-1, got %s", detail.LatestObservation.ID)
		}
	})

	t.Run("found without observation", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		svc := NewEntityService(eRepo, oRepo)
		detail, err := svc.GetEntityDetail(ctx, compositeID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if detail.Entity.ID != entityID {
			t.Errorf("expected entity ID %s, got %s", entityID, detail.Entity.ID)
		}
		if detail.LatestObservation != nil {
			t.Errorf("expected nil observation, got %+v", detail.LatestObservation)
		}
	})

	t.Run("not found", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()

		svc := NewEntityService(eRepo, oRepo)
		_, err := svc.GetEntityDetail(ctx, "flights_commercial:nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent entity")
		}
	})
}

func TestEntityService_GetObservationHistory(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	dbEntityID := "ent-1"
	compositeID := "flights_commercial:AAL1234"

	entity := &domain.Entity{
		ID:         dbEntityID,
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
	}

	makeObs := func(id string, entityID string, ts time.Time) *domain.Observation {
		return &domain.Observation{
			ID:        id,
			EntityID:  entityID,
			Timestamp: ts,
			Position:  &domain.GeoPoint{Lat: 37.77, Lon: -122.42},
			AltitudeM: 10000,
		}
	}

	t.Run("paginated with has_more", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		// Add 5 observations
		for i := 0; i < 5; i++ {
			oRepo.add(makeObs(
				"obs-"+string(rune('a'+i)),
				dbEntityID,
				now.Add(-time.Duration(i)*time.Minute),
			))
		}

		svc := NewEntityService(eRepo, oRepo)
		obs, hasMore, err := svc.GetObservationHistory(ctx, compositeID, 3, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obs) != 3 {
			t.Errorf("expected 3 observations, got %d", len(obs))
		}
		if !hasMore {
			t.Error("expected hasMore=true")
		}
	})

	t.Run("all returned no has_more", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		oRepo.add(makeObs("obs-1", dbEntityID, now.Add(-time.Minute)))
		oRepo.add(makeObs("obs-2", dbEntityID, now.Add(-2*time.Minute)))

		svc := NewEntityService(eRepo, oRepo)
		obs, hasMore, err := svc.GetObservationHistory(ctx, compositeID, 50, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obs) != 2 {
			t.Errorf("expected 2 observations, got %d", len(obs))
		}
		if hasMore {
			t.Error("expected hasMore=false")
		}
	})

	t.Run("empty history", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		svc := NewEntityService(eRepo, oRepo)
		obs, hasMore, err := svc.GetObservationHistory(ctx, compositeID, 50, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obs) != 0 {
			t.Errorf("expected 0 observations, got %d", len(obs))
		}
		if hasMore {
			t.Error("expected hasMore=false")
		}
	})

	t.Run("with before cursor", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		oRepo.add(makeObs("obs-1", dbEntityID, now.Add(-1*time.Minute)))
		oRepo.add(makeObs("obs-2", dbEntityID, now.Add(-5*time.Minute)))
		oRepo.add(makeObs("obs-3", dbEntityID, now.Add(-10*time.Minute)))

		svc := NewEntityService(eRepo, oRepo)
		before := now.Add(-3 * time.Minute)
		obs, _, err := svc.GetObservationHistory(ctx, compositeID, 50, before)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only obs-2 and obs-3 should be returned (before -3min)
		if len(obs) != 2 {
			t.Errorf("expected 2 observations before cursor, got %d", len(obs))
		}
	})

	t.Run("default limit applied", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity)

		svc := NewEntityService(eRepo, oRepo)
		// Pass 0 limit — should default to 50
		_, _, err := svc.GetObservationHistory(ctx, compositeID, 0, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// searchEntityRepo is a controllable stub for SearchEntities testing.
// ---------------------------------------------------------------------------

type searchEntityRepo struct {
	stubEntityRepo
	results    []*domain.EntitySearchResult
	totalCount int
	err        error
	// Captured args for verification
	capturedQuery     string
	capturedLayerType string
	capturedLimit     int
}

func (s *searchEntityRepo) SearchEntities(_ context.Context, query string, layerType string, limit int) ([]*domain.EntitySearchResult, int, error) {
	s.capturedQuery = query
	s.capturedLayerType = layerType
	s.capturedLimit = limit
	return s.results, s.totalCount, s.err
}

func TestEntityService_SearchEntities(t *testing.T) {
	ctx := context.Background()
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
		name          string
		query         string
		layerType     string
		limit         int
		repoResults   []*domain.EntitySearchResult
		repoCount     int
		repoErr       error
		wantResults   int
		wantErr       bool
		wantErrSubstr string
		wantLimit     int // expected limit passed to repo
	}{
		{
			name:        "successful search",
			query:       "UAL",
			limit:       20,
			repoResults: sampleResults,
			repoCount:   2,
			wantResults: 2,
			wantLimit:   20,
		},
		{
			name:        "search with layer filter",
			query:       "UAL",
			layerType:   "flights_commercial",
			limit:       20,
			repoResults: sampleResults,
			repoCount:   2,
			wantResults: 2,
			wantLimit:   20,
		},
		{
			name:          "query too short - single char",
			query:         "U",
			limit:         20,
			wantErr:       true,
			wantErrSubstr: "at least 2 characters",
		},
		{
			name:          "query too short - empty",
			query:         "",
			limit:         20,
			wantErr:       true,
			wantErrSubstr: "at least 2 characters",
		},
		{
			name:        "default limit when zero",
			query:       "UAL",
			limit:       0,
			repoResults: sampleResults,
			repoCount:   2,
			wantResults: 2,
			wantLimit:   20,
		},
		{
			name:        "limit capped at 100",
			query:       "UAL",
			limit:       200,
			repoResults: sampleResults,
			repoCount:   2,
			wantResults: 2,
			wantLimit:   100,
		},
		{
			name:        "negative limit defaults to 20",
			query:       "UAL",
			limit:       -5,
			repoResults: sampleResults,
			repoCount:   2,
			wantResults: 2,
			wantLimit:   20,
		},
		{
			name:          "repo error propagated",
			query:         "UAL",
			limit:         20,
			repoErr:       fmt.Errorf("database connection lost"),
			wantErr:       true,
			wantErrSubstr: "database connection lost",
		},
		{
			name:        "no results",
			query:       "NONEXISTENT",
			limit:       20,
			repoResults: nil,
			repoCount:   0,
			wantResults: 0,
			wantLimit:   20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eRepo := &searchEntityRepo{
				stubEntityRepo: *newStubEntityRepo(),
				results:        tt.repoResults,
				totalCount:     tt.repoCount,
				err:            tt.repoErr,
			}
			oRepo := newStubObsRepo()
			svc := NewEntityService(eRepo, oRepo)

			results, count, err := svc.SearchEntities(ctx, tt.query, tt.layerType, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrSubstr != "" {
					if got := err.Error(); !contains(got, tt.wantErrSubstr) {
						t.Errorf("error %q does not contain %q", got, tt.wantErrSubstr)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantResults {
				t.Errorf("expected %d results, got %d", tt.wantResults, len(results))
			}
			if count != tt.repoCount {
				t.Errorf("expected count %d, got %d", tt.repoCount, count)
			}
			// Verify limit was normalized before passing to repo
			if tt.wantLimit > 0 && eRepo.capturedLimit != tt.wantLimit {
				t.Errorf("expected limit %d passed to repo, got %d", tt.wantLimit, eRepo.capturedLimit)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestEntityService_GetEntityDetail_InvalidID(t *testing.T) {
	ctx := context.Background()
	eRepo := newStubEntityRepo()
	oRepo := newStubObsRepo()
	svc := NewEntityService(eRepo, oRepo)

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"empty string", "", true},
		{"no colon", "invalidid", true},
		{"only colon", ":", true},
		{"missing external id", "flights_commercial:", true},
		{"missing layer type", ":AAL1234", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetEntityDetail(ctx, tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEntityDetail(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestEntityService_GetObservationHistory_InvalidID(t *testing.T) {
	ctx := context.Background()
	eRepo := newStubEntityRepo()
	oRepo := newStubObsRepo()
	svc := NewEntityService(eRepo, oRepo)

	_, _, err := svc.GetObservationHistory(ctx, "invalid", 10, time.Now())
	if err == nil {
		t.Fatal("expected error for invalid entity ID")
	}
}

func TestEntityService_GetObservationHistory_NegativeLimit(t *testing.T) {
	ctx := context.Background()
	entity := &domain.Entity{
		ID:         "ent-1",
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
	}

	eRepo := newStubEntityRepo()
	oRepo := newStubObsRepo()
	eRepo.add(entity)

	svc := NewEntityService(eRepo, oRepo)
	obs, _, err := svc.GetObservationHistory(ctx, "flights_commercial:AAL1234", -10, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("expected 0 observations for negative limit, got %d", len(obs))
	}
}

func TestEntityService_GetObservationHistory_RepoError(t *testing.T) {
	ctx := context.Background()
	entity := &domain.Entity{
		ID:         "ent-1",
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
	}

	eRepo := newStubEntityRepo()
	eRepo.add(entity)

	oRepo := newStubObsRepo()
	oRepo.getErr = fmt.Errorf("database error")

	svc := NewEntityService(eRepo, oRepo)
	_, _, err := svc.GetObservationHistory(ctx, "flights_commercial:AAL1234", 10, time.Now())
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

func TestEntityService_GetEntitiesBatch_RepoError(t *testing.T) {
	ctx := context.Background()

	t.Run("entity repo error", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		eRepo.getErr = fmt.Errorf("entity lookup failed")
		oRepo := newStubObsRepo()

		svc := NewEntityService(eRepo, oRepo)
		_, err := svc.GetEntitiesBatch(ctx, []string{"flights_commercial:AAL1234"})
		if err == nil {
			t.Fatal("expected error from entity repo")
		}
	})

	t.Run("observation repo error", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		entity := &domain.Entity{
			ID:         "ent-1",
			ExternalID: "AAL1234",
			LayerType:  "flights_commercial",
			Name:       "AAL1234",
		}
		eRepo.add(entity)

		oRepo := newStubObsRepo()
		oRepo.getErr = fmt.Errorf("observation lookup failed")

		svc := NewEntityService(eRepo, oRepo)
		_, err := svc.GetEntitiesBatch(ctx, []string{"flights_commercial:AAL1234"})
		if err == nil {
			t.Fatal("expected error from observation repo")
		}
	})
}

func TestEntityService_GetObservationHistory_ZeroBeforeTime(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	entity := &domain.Entity{
		ID:         "ent-1",
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
	}

	eRepo := newStubEntityRepo()
	oRepo := newStubObsRepo()
	eRepo.add(entity)

	for i := 0; i < 60; i++ {
		obs := &domain.Observation{
			ID:        fmt.Sprintf("obs-%d", i),
			EntityID:  "ent-1",
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Position:  &domain.GeoPoint{Lat: 37.77, Lon: -122.42},
		}
		oRepo.add(obs)
	}

	svc := NewEntityService(eRepo, oRepo)
	obs, hasMore, err := svc.GetObservationHistory(ctx, "flights_commercial:AAL1234", 50, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 50 {
		t.Errorf("expected 50 observations, got %d", len(obs))
	}
	if !hasMore {
		t.Error("expected hasMore=true")
	}
}

func TestEntityService_GetEntitiesBatch(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	entity1 := &domain.Entity{
		ID:         "ent-1",
		ExternalID: "AAL1234",
		LayerType:  "flights_commercial",
		Name:       "AAL1234",
	}
	entity2 := &domain.Entity{
		ID:         "ent-2",
		ExternalID: "25544",
		LayerType:  "satellites",
		Name:       "ISS",
	}

	obs1 := &domain.Observation{
		ID:        "obs-1",
		EntityID:  "ent-1",
		Timestamp: now,
		Position:  &domain.GeoPoint{Lat: 37.77, Lon: -122.42},
		AltitudeM: 10668,
	}
	obs2 := &domain.Observation{
		ID:        "obs-2",
		EntityID:  "ent-2",
		Timestamp: now,
		Position:  &domain.GeoPoint{Lat: 51.5, Lon: -0.1},
		AltitudeM: 408000,
	}

	t.Run("multiple entities with observations", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity1)
		eRepo.add(entity2)
		oRepo.add(obs1)
		oRepo.add(obs2)

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{
			"flights_commercial:AAL1234",
			"satellites:25544",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		// Verify both entities have observations
		for _, r := range results {
			if r.LatestObservation == nil {
				t.Errorf("expected observation for entity %s", r.Entity.ID)
			}
		}
	})

	t.Run("partial success - skip not found", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity1)
		oRepo.add(obs1)

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{
			"flights_commercial:AAL1234",
			"flights_commercial:NONEXISTENT",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result (partial success), got %d", len(results))
		}
		if results[0].Entity.ID != "ent-1" {
			t.Errorf("expected entity ent-1, got %s", results[0].Entity.ID)
		}
	})

	t.Run("entity without observation", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity1)

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{"flights_commercial:AAL1234"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].LatestObservation != nil {
			t.Errorf("expected nil observation, got %+v", results[0].LatestObservation)
		}
	})

	t.Run("empty entity_ids", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil {
			t.Errorf("expected nil results for empty input, got %d", len(results))
		}
	})

	t.Run("exceeds max batch size", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()

		ids := make([]string, 51)
		for i := range ids {
			ids[i] = fmt.Sprintf("flights_commercial:FLIGHT%d", i)
		}

		svc := NewEntityService(eRepo, oRepo)
		_, err := svc.GetEntitiesBatch(ctx, ids)
		if err == nil {
			t.Fatal("expected error for exceeding batch size")
		}
		if !contains(err.Error(), "exceeds maximum") {
			t.Errorf("expected 'exceeds maximum' in error, got: %v", err)
		}
	})

	t.Run("malformed IDs skipped", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()
		eRepo.add(entity1)
		oRepo.add(obs1)

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{
			"flights_commercial:AAL1234",
			"malformed-no-colon",
			"",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("all entities not found returns nil", func(t *testing.T) {
		eRepo := newStubEntityRepo()
		oRepo := newStubObsRepo()

		svc := NewEntityService(eRepo, oRepo)
		results, err := svc.GetEntitiesBatch(ctx, []string{"flights_commercial:NONEXISTENT"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results != nil {
			t.Errorf("expected nil results, got %d", len(results))
		}
	})
}
