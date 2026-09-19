package scene

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

var _ domain.SceneRepository = (*stubSceneRepo)(nil)

type stubSceneRepo struct {
	mu        sync.RWMutex
	scenes    map[string]*domain.Scene
	getErr    error
	listErr   error
	createErr error
	updateErr error
	deleteErr error
}

func newStubSceneRepo() *stubSceneRepo {
	return &stubSceneRepo{scenes: make(map[string]*domain.Scene)}
}

func (s *stubSceneRepo) Create(_ context.Context, scene *domain.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.scenes[scene.ID] = scene
	return nil
}

func (s *stubSceneRepo) GetByID(_ context.Context, id string) (*domain.Scene, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	scene, ok := s.scenes[id]
	if !ok {
		return nil, domain.NewNotFoundError("scene not found", nil)
	}
	return scene, nil
}

func (s *stubSceneRepo) List(_ context.Context) ([]*domain.Scene, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]*domain.Scene, 0, len(s.scenes))
	for _, scene := range s.scenes {
		result = append(result, scene)
	}
	return result, nil
}

func (s *stubSceneRepo) Update(_ context.Context, scene *domain.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.updateErr != nil {
		return s.updateErr
	}
	if _, ok := s.scenes[scene.ID]; !ok {
		return domain.NewNotFoundError("scene not found", nil)
	}
	s.scenes[scene.ID] = scene
	return nil
}

func (s *stubSceneRepo) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.scenes[id]; !ok {
		return domain.NewNotFoundError("scene not found", nil)
	}
	delete(s.scenes, id)
	return nil
}

func TestNewSceneService(t *testing.T) {
	repo := newStubSceneRepo()
	svc := NewSceneService(repo)
	if svc == nil {
		t.Fatal("expected non-nil SceneService")
	}
	if svc.sceneRepo != repo {
		t.Error("sceneRepo not wired correctly")
	}
}

func TestSceneService_GetScenes(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns empty list when no scenes", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		scenes, err := svc.GetScenes(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(scenes) != 0 {
			t.Errorf("expected 0 scenes, got %d", len(scenes))
		}
	})

	t.Run("returns all scenes", func(t *testing.T) {
		repo := newStubSceneRepo()
		scene1 := &domain.Scene{ID: "scene-1", Name: "Scene 1", CreatedAt: now}
		scene2 := &domain.Scene{ID: "scene-2", Name: "Scene 2", CreatedAt: now}
		_ = repo.Create(ctx, scene1)
		_ = repo.Create(ctx, scene2)

		svc := NewSceneService(repo)
		scenes, err := svc.GetScenes(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(scenes) != 2 {
			t.Errorf("expected 2 scenes, got %d", len(scenes))
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubSceneRepo()
		repo.listErr = errors.New("database error")
		svc := NewSceneService(repo)

		_, err := svc.GetScenes(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "database error" {
			t.Errorf("expected 'database error', got %q", err.Error())
		}
	})
}

func TestSceneService_GetScene(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns scene by id", func(t *testing.T) {
		repo := newStubSceneRepo()
		scene := &domain.Scene{
			ID:        "scene-1",
			Name:      "Test Scene",
			Camera:    domain.Camera{Position: domain.GeoPoint{Lat: 37.77, Lon: -122.42}},
			CreatedAt: now,
		}
		_ = repo.Create(ctx, scene)

		svc := NewSceneService(repo)
		result, err := svc.GetScene(ctx, "scene-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != "scene-1" {
			t.Errorf("expected ID 'scene-1', got %q", result.ID)
		}
		if result.Name != "Test Scene" {
			t.Errorf("expected Name 'Test Scene', got %q", result.Name)
		}
	})

	t.Run("returns error for nonexistent scene", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		_, err := svc.GetScene(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubSceneRepo()
		repo.getErr = errors.New("connection lost")
		svc := NewSceneService(repo)

		_, err := svc.GetScene(ctx, "any-id")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "connection lost" {
			t.Errorf("expected 'connection lost', got %q", err.Error())
		}
	})
}

func TestSceneService_CreateScene(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("creates scene successfully", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		scene := &domain.Scene{
			ID:        "scene-1",
			Name:      "New Scene",
			Camera:    domain.Camera{Position: domain.GeoPoint{Lat: 40.71, Lon: -74.01}, Heading: 90},
			CreatedAt: now,
		}

		err := svc.CreateScene(ctx, scene)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		retrieved, err := repo.GetByID(ctx, "scene-1")
		if err != nil {
			t.Fatalf("failed to retrieve scene: %v", err)
		}
		if retrieved.Name != "New Scene" {
			t.Errorf("expected Name 'New Scene', got %q", retrieved.Name)
		}
	})

	t.Run("creates scene with layer toggles", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		scene := &domain.Scene{
			ID:   "scene-2",
			Name: "Scene with Layers",
			LayerToggles: []domain.LayerToggle{
				{LayerID: "flights", Enabled: true, Mode: "full", Density: 100},
				{LayerID: "satellites", Enabled: true, Mode: "sparse", Density: 50},
			},
			CreatedAt: now,
		}

		err := svc.CreateScene(ctx, scene)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("creates scene with filter preset", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		scene := &domain.Scene{
			ID:             "scene-3",
			Name:           "Scene with Filter",
			FilterPresetID: "filter-1",
			CreatedAt:      now,
		}

		err := svc.CreateScene(ctx, scene)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubSceneRepo()
		repo.createErr = errors.New("insert failed")
		svc := NewSceneService(repo)

		scene := &domain.Scene{ID: "scene-1", Name: "Test", CreatedAt: now}
		err := svc.CreateScene(ctx, scene)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "insert failed" {
			t.Errorf("expected 'insert failed', got %q", err.Error())
		}
	})
}

func TestSceneService_UpdateScene(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("updates existing scene", func(t *testing.T) {
		repo := newStubSceneRepo()
		original := &domain.Scene{
			ID:        "scene-1",
			Name:      "Original Name",
			Camera:    domain.Camera{Position: domain.GeoPoint{Lat: 0, Lon: 0}},
			CreatedAt: now,
		}
		_ = repo.Create(ctx, original)

		svc := NewSceneService(repo)
		updated := &domain.Scene{
			ID:        "scene-1",
			Name:      "Updated Name",
			Camera:    domain.Camera{Position: domain.GeoPoint{Lat: 51.5, Lon: -0.1}},
			CreatedAt: now,
		}

		err := svc.UpdateScene(ctx, updated)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		retrieved, _ := repo.GetByID(ctx, "scene-1")
		if retrieved.Name != "Updated Name" {
			t.Errorf("expected Name 'Updated Name', got %q", retrieved.Name)
		}
		if retrieved.Camera.Position.Lat != 51.5 {
			t.Errorf("expected Lat 51.5, got %f", retrieved.Camera.Position.Lat)
		}
	})

	t.Run("returns error for nonexistent scene", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		scene := &domain.Scene{ID: "nonexistent", Name: "Test", CreatedAt: now}
		err := svc.UpdateScene(ctx, scene)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubSceneRepo()
		repo.updateErr = errors.New("update failed")
		svc := NewSceneService(repo)

		scene := &domain.Scene{ID: "scene-1", Name: "Test", CreatedAt: now}
		err := svc.UpdateScene(ctx, scene)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestSceneService_DeleteScene(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("deletes existing scene", func(t *testing.T) {
		repo := newStubSceneRepo()
		scene := &domain.Scene{ID: "scene-1", Name: "To Delete", CreatedAt: now}
		_ = repo.Create(ctx, scene)

		svc := NewSceneService(repo)
		err := svc.DeleteScene(ctx, "scene-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = repo.GetByID(ctx, "scene-1")
		if err == nil {
			t.Error("expected scene to be deleted")
		}
	})

	t.Run("returns error for nonexistent scene", func(t *testing.T) {
		repo := newStubSceneRepo()
		svc := NewSceneService(repo)

		err := svc.DeleteScene(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubSceneRepo()
		repo.deleteErr = errors.New("delete failed")
		svc := NewSceneService(repo)

		err := svc.DeleteScene(ctx, "any-id")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "delete failed" {
			t.Errorf("expected 'delete failed', got %q", err.Error())
		}
	})
}
