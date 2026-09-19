package filter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

var _ domain.FilterPresetRepository = (*stubFilterPresetRepo)(nil)

type stubFilterPresetRepo struct {
	mu        sync.RWMutex
	presets   map[string]*domain.FilterPreset
	getErr    error
	listErr   error
	createErr error
}

func newStubFilterPresetRepo() *stubFilterPresetRepo {
	return &stubFilterPresetRepo{presets: make(map[string]*domain.FilterPreset)}
}

func (s *stubFilterPresetRepo) Create(_ context.Context, preset *domain.FilterPreset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return s.createErr
	}
	s.presets[preset.ID] = preset
	return nil
}

func (s *stubFilterPresetRepo) GetByID(_ context.Context, id string) (*domain.FilterPreset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	preset, ok := s.presets[id]
	if !ok {
		return nil, domain.NewNotFoundError("filter preset not found", nil)
	}
	return preset, nil
}

func (s *stubFilterPresetRepo) List(_ context.Context) ([]*domain.FilterPreset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]*domain.FilterPreset, 0, len(s.presets))
	for _, preset := range s.presets {
		result = append(result, preset)
	}
	return result, nil
}

func TestNewFilterService(t *testing.T) {
	repo := newStubFilterPresetRepo()
	svc := NewFilterService(repo)
	if svc == nil {
		t.Fatal("expected non-nil FilterService")
	}
	if svc.filterPresetRepo != repo {
		t.Error("filterPresetRepo not wired correctly")
	}
}

func TestFilterService_GetFilterPresets(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns empty list when no presets", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		svc := NewFilterService(repo)

		presets, err := svc.GetFilterPresets(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(presets) != 0 {
			t.Errorf("expected 0 presets, got %d", len(presets))
		}
	})

	t.Run("returns all presets", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		preset1 := &domain.FilterPreset{
			ID:        "preset-1",
			Name:      "CRT Style",
			Style:     "CRT",
			Params:    map[string]float64{"scanlines": 1.0, "curvature": 0.5},
			CreatedAt: now,
		}
		preset2 := &domain.FilterPreset{
			ID:        "preset-2",
			Name:      "NVG Mode",
			Style:     "NVG",
			Params:    map[string]float64{"gain": 2.0},
			CreatedAt: now,
		}
		_ = repo.Create(ctx, preset1)
		_ = repo.Create(ctx, preset2)

		svc := NewFilterService(repo)
		presets, err := svc.GetFilterPresets(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(presets) != 2 {
			t.Errorf("expected 2 presets, got %d", len(presets))
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		repo.listErr = errors.New("database error")
		svc := NewFilterService(repo)

		_, err := svc.GetFilterPresets(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "database error" {
			t.Errorf("expected 'database error', got %q", err.Error())
		}
	})
}

func TestFilterService_GetFilterPreset(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("returns preset by id", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		preset := &domain.FilterPreset{
			ID:        "preset-1",
			Name:      "FLIR Thermal",
			Style:     "FLIR",
			Params:    map[string]float64{"heatmap": 1.0, "contrast": 1.5},
			CreatedAt: now,
		}
		_ = repo.Create(ctx, preset)

		svc := NewFilterService(repo)
		result, err := svc.GetFilterPreset(ctx, "preset-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ID != "preset-1" {
			t.Errorf("expected ID 'preset-1', got %q", result.ID)
		}
		if result.Name != "FLIR Thermal" {
			t.Errorf("expected Name 'FLIR Thermal', got %q", result.Name)
		}
		if result.Style != "FLIR" {
			t.Errorf("expected Style 'FLIR', got %q", result.Style)
		}
	})

	t.Run("returns error for nonexistent preset", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		svc := NewFilterService(repo)

		_, err := svc.GetFilterPreset(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		repo.getErr = errors.New("connection lost")
		svc := NewFilterService(repo)

		_, err := svc.GetFilterPreset(ctx, "any-id")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "connection lost" {
			t.Errorf("expected 'connection lost', got %q", err.Error())
		}
	})
}

func TestFilterService_CreateFilterPreset(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("creates preset successfully", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		svc := NewFilterService(repo)

		preset := &domain.FilterPreset{
			ID:        "preset-1",
			Name:      "Anime Style",
			Style:     "Anime",
			Params:    map[string]float64{"saturation": 1.3, "edge_detect": 0.8},
			CreatedAt: now,
		}

		err := svc.CreateFilterPreset(ctx, preset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		retrieved, err := repo.GetByID(ctx, "preset-1")
		if err != nil {
			t.Fatalf("failed to retrieve preset: %v", err)
		}
		if retrieved.Name != "Anime Style" {
			t.Errorf("expected Name 'Anime Style', got %q", retrieved.Name)
		}
	})

	t.Run("creates preset with minimal params", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		svc := NewFilterService(repo)

		preset := &domain.FilterPreset{
			ID:        "preset-2",
			Name:      "Normal",
			Style:     "NORMAL",
			Params:    map[string]float64{},
			CreatedAt: now,
		}

		err := svc.CreateFilterPreset(ctx, preset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("creates preset with all styles", func(t *testing.T) {
		styles := []string{"CRT", "NVG", "FLIR", "NORMAL", "Anime", "Noir", "Snow", "AI"}
		for _, style := range styles {
			repo := newStubFilterPresetRepo()
			svc := NewFilterService(repo)

			preset := &domain.FilterPreset{
				ID:        "preset-" + style,
				Name:      style + " Style",
				Style:     style,
				Params:    map[string]float64{"intensity": 1.0},
				CreatedAt: now,
			}

			err := svc.CreateFilterPreset(ctx, preset)
			if err != nil {
				t.Errorf("style %s: unexpected error: %v", style, err)
			}
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := newStubFilterPresetRepo()
		repo.createErr = errors.New("insert failed")
		svc := NewFilterService(repo)

		preset := &domain.FilterPreset{ID: "preset-1", Name: "Test", Style: "NORMAL", Params: map[string]float64{}, CreatedAt: now}
		err := svc.CreateFilterPreset(ctx, preset)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "insert failed" {
			t.Errorf("expected 'insert failed', got %q", err.Error())
		}
	})
}
