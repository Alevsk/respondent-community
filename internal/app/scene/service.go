package scene

import (
	"context"
	"strings"

	"github.com/Alevsk/respondent/internal/domain"
)

// SceneService implements the SceneService gRPC service.
// It depends on the domain.SceneRepository interface rather than a concrete
// implementation, following the Dependency Inversion Principle.
type SceneService struct {
	sceneRepo domain.SceneRepository
}

// NewSceneService creates a new SceneService with interface-based dependencies.
func NewSceneService(sceneRepo domain.SceneRepository) *SceneService {
	return &SceneService{
		sceneRepo: sceneRepo,
	}
}

// GetScenes returns all saved scenes
func (s *SceneService) GetScenes(ctx context.Context) ([]*domain.Scene, error) {
	return s.sceneRepo.List(ctx)
}

// GetScene returns a specific scene
func (s *SceneService) GetScene(ctx context.Context, id string) (*domain.Scene, error) {
	return s.sceneRepo.GetByID(ctx, id)
}

// CreateScene creates a new scene after validating inputs.
func (s *SceneService) CreateScene(ctx context.Context, scene *domain.Scene) error {
	if err := validateScene(scene); err != nil {
		return err
	}
	return s.sceneRepo.Create(ctx, scene)
}

// UpdateScene updates an existing scene after validating inputs.
func (s *SceneService) UpdateScene(ctx context.Context, scene *domain.Scene) error {
	if err := validateScene(scene); err != nil {
		return err
	}
	return s.sceneRepo.Update(ctx, scene)
}

// validateScene checks that a scene has reasonable field values.
func validateScene(scene *domain.Scene) error {
	if strings.TrimSpace(scene.Name) == "" {
		return domain.NewInvalidInputError("scene name is required", nil)
	}
	cam := scene.Camera
	if cam.Position.Lat < -90 || cam.Position.Lat > 90 {
		return domain.NewInvalidInputError("camera latitude must be between -90 and 90", nil)
	}
	if cam.Position.Lon < -180 || cam.Position.Lon > 180 {
		return domain.NewInvalidInputError("camera longitude must be between -180 and 180", nil)
	}
	if cam.Range < 0 {
		return domain.NewInvalidInputError("camera range must be non-negative", nil)
	}
	return nil
}

// DeleteScene deletes a scene
func (s *SceneService) DeleteScene(ctx context.Context, id string) error {
	return s.sceneRepo.Delete(ctx, id)
}
