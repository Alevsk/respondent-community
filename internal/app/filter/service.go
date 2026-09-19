package filter

import (
	"context"
	"strings"

	"github.com/Alevsk/respondent/internal/domain"
)

// validFilterStyles is the set of valid filter preset styles.
var validFilterStyles = map[string]bool{
	"CRT":    true,
	"NVG":    true,
	"FLIR":   true,
	"NORMAL": true,
	"Anime":  true,
	"Noir":   true,
	"Snow":   true,
	"AI":     true,
}

// FilterService implements the FilterService gRPC service.
// It depends on the domain.FilterPresetRepository interface rather than a concrete
// implementation, following the Dependency Inversion Principle.
type FilterService struct {
	filterPresetRepo domain.FilterPresetRepository
}

// NewFilterService creates a new FilterService with interface-based dependencies.
func NewFilterService(filterPresetRepo domain.FilterPresetRepository) *FilterService {
	return &FilterService{
		filterPresetRepo: filterPresetRepo,
	}
}

// GetFilterPresets returns all filter presets
func (s *FilterService) GetFilterPresets(ctx context.Context) ([]*domain.FilterPreset, error) {
	return s.filterPresetRepo.List(ctx)
}

// GetFilterPreset returns a specific filter preset
func (s *FilterService) GetFilterPreset(ctx context.Context, id string) (*domain.FilterPreset, error) {
	return s.filterPresetRepo.GetByID(ctx, id)
}

// CreateFilterPreset creates a new filter preset after validating inputs.
func (s *FilterService) CreateFilterPreset(ctx context.Context, preset *domain.FilterPreset) error {
	if strings.TrimSpace(preset.Name) == "" {
		return domain.NewInvalidInputError("filter preset name is required", nil)
	}
	if !validFilterStyles[preset.Style] {
		return domain.NewInvalidInputError("invalid filter style: must be one of CRT, NVG, FLIR, NORMAL, Anime, Noir, Snow, AI", nil)
	}
	if preset.Params == nil {
		return domain.NewInvalidInputError("filter preset params must not be nil", nil)
	}
	return s.filterPresetRepo.Create(ctx, preset)
}
