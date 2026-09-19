package filtertest

import (
	"context"
	"sync"

	"github.com/Alevsk/respondent/internal/domain"
)

// FakeFilterPresetRepo implements domain.FilterPresetRepository for testing.
type FakeFilterPresetRepo struct {
	mu      sync.RWMutex
	Presets map[string]*domain.FilterPreset
}

func NewFakeFilterPresetRepo() *FakeFilterPresetRepo {
	return &FakeFilterPresetRepo{Presets: make(map[string]*domain.FilterPreset)}
}

func (f *FakeFilterPresetRepo) Create(_ context.Context, p *domain.FilterPreset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Presets[p.ID] = p
	return nil
}

func (f *FakeFilterPresetRepo) GetByID(_ context.Context, id string) (*domain.FilterPreset, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.Presets[id]
	if !ok {
		return nil, domain.NewNotFoundError("preset not found", nil)
	}
	return p, nil
}

func (f *FakeFilterPresetRepo) List(_ context.Context) ([]*domain.FilterPreset, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []*domain.FilterPreset
	for _, p := range f.Presets {
		result = append(result, p)
	}
	return result, nil
}

var _ domain.FilterPresetRepository = (*FakeFilterPresetRepo)(nil)
