package scenetest

import (
	"context"
	"sync"

	"github.com/Alevsk/respondent/internal/domain"
)

// FakeSceneRepo implements domain.SceneRepository for testing.
type FakeSceneRepo struct {
	mu     sync.RWMutex
	Scenes map[string]*domain.Scene
}

func NewFakeSceneRepo() *FakeSceneRepo {
	return &FakeSceneRepo{Scenes: make(map[string]*domain.Scene)}
}

func (f *FakeSceneRepo) Create(_ context.Context, s *domain.Scene) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Scenes[s.ID] = s
	return nil
}

func (f *FakeSceneRepo) GetByID(_ context.Context, id string) (*domain.Scene, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	s, ok := f.Scenes[id]
	if !ok {
		return nil, domain.NewNotFoundError("scene not found", nil)
	}
	return s, nil
}

func (f *FakeSceneRepo) List(_ context.Context) ([]*domain.Scene, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []*domain.Scene
	for _, s := range f.Scenes {
		result = append(result, s)
	}
	return result, nil
}

func (f *FakeSceneRepo) Update(_ context.Context, s *domain.Scene) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Scenes[s.ID] = s
	return nil
}

func (f *FakeSceneRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Scenes, id)
	return nil
}

var _ domain.SceneRepository = (*FakeSceneRepo)(nil)
