package layer

import (
	"context"
	"sort"

	"github.com/Alevsk/respondent/internal/domain"
)

// SourceRegistry is the read-only view of declarative layer metadata the layer
// service depends on. *domain.DynamicSourceRegistry satisfies it; depending on
// the interface keeps the service decoupled from the concrete registry (DIP).
type SourceRegistry interface {
	GetLayerStyle(lt domain.LayerType) domain.LayerStyle
	LookupDisplayConfig(lt domain.LayerType) (*domain.LayerDisplayConfig, bool)
	LookupLayerDisplayName(lt domain.LayerType) (string, bool)
	LookupFilteringMode(lt domain.LayerType) (string, bool)
	LookupHistoryConfig(lt domain.LayerType) (*domain.HistoryConfig, bool)
	LookupRenderingMode(lt domain.LayerType) (string, bool)
}

// LayerService implements the LayerService gRPC service.
// It depends on interfaces (domain.EntityRepository, domain.ObservationRepository, domain.CacheStorage)
// rather than concrete implementations, following the Dependency Inversion Principle.
type LayerService struct {
	entityRepo domain.EntityRepository
	obsRepo    domain.ObservationRepository
	cache      domain.CacheStorage // nil if cache unavailable
	dynReg     SourceRegistry
}

// NewLayerService creates a new LayerService with interface-based dependencies.
func NewLayerService(
	entityRepo domain.EntityRepository,
	obsRepo domain.ObservationRepository,
	cache domain.CacheStorage,
	dynReg SourceRegistry,
) *LayerService {
	return &LayerService{
		entityRepo: entityRepo,
		obsRepo:    obsRepo,
		cache:      cache,
		dynReg:     dynReg,
	}
}

// GetLayers returns all available layers discovered from the database and
// enriched with display/history metadata from the declarative dynamic registry.
func (s *LayerService) GetLayers(ctx context.Context) ([]*domain.Layer, error) {
	var layers []*domain.Layer
	dynReg := s.dynReg

	// Discover all layer types from the database.
	dbLayerTypes, err := s.entityRepo.GetDistinctLayerTypes(ctx)
	if err != nil {
		return nil, err
	}

	// Source per-layer counts from the durable store (SQLite) in a single grouped
	// query — the same source layer discovery and the ENABLE snapshot fallback use.
	// The volatile hot cache is empty on cold start and only partially filled
	// afterward, so counting it would under-report (or zero) layers that hold data.
	counts, err := s.entityRepo.CountByLayerType(ctx)
	if err != nil {
		return nil, err
	}

	for _, lt := range dbLayerTypes {
		count := counts[lt]

		style := dynReg.GetLayerStyle(domain.LayerType(lt))
		layer := &domain.Layer{
			ID:        lt,
			Name:      domain.FormatLayerName(lt),
			Type:      lt,
			Enabled:   true,
			Mode:      "full",
			Density:   100,
			Source:    "feeder",
			Count:     count,
			Color:     style.Color,
			PointSize: style.PointSize,
		}

		// Optional per-layer label override (declarative layer_display_name). When a
		// source sets it, it replaces the title-cased layer_type so the panel can show
		// e.g. "Traffic Stations (Mexico)" instead of "Traffic Stations".
		if dn, ok := dynReg.LookupLayerDisplayName(domain.LayerType(lt)); ok && dn != "" {
			layer.Name = dn
		}

		// Attach display config from the dynamic registry (declarative sources).
		if dc, ok := dynReg.LookupDisplayConfig(domain.LayerType(lt)); ok {
			layer.DisplayConfig = dc
			if dc.Style != nil && dc.Style.Color != "" {
				layer.Color = dc.Style.Color
			}
			if dc.Style != nil && dc.Style.PointSize > 0 {
				layer.PointSize = dc.Style.PointSize
			}
		}
		if hc, ok := dynReg.LookupHistoryConfig(domain.LayerType(lt)); ok {
			layer.HistoryConfig = hc
		}
		// Set rendering mode from dynamic registry; default to "map".
		if rm, ok := dynReg.LookupRenderingMode(domain.LayerType(lt)); ok {
			layer.RenderingMode = rm
		} else {
			layer.RenderingMode = "map"
		}
		// Set filtering mode from dynamic registry (e.g. "viewport").
		if fm, ok := dynReg.LookupFilteringMode(domain.LayerType(lt)); ok {
			layer.FilteringMode = fm
		}

		layers = append(layers, layer)
	}

	// Sort by ID for stable ordering.
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].ID < layers[j].ID
	})

	return layers, nil
}

// ToggleLayer toggles layer enable/disable, mode, and density
func (s *LayerService) ToggleLayer(ctx context.Context, toggle *domain.LayerToggle) (*domain.Layer, error) {
	layerType := domain.LayerType(toggle.LayerID)

	dynReg := s.dynReg
	style := dynReg.GetLayerStyle(layerType)
	layer := &domain.Layer{
		ID:        toggle.LayerID,
		Name:      domain.FormatLayerName(toggle.LayerID),
		Type:      string(layerType),
		Enabled:   toggle.Enabled,
		Mode:      toggle.Mode,
		Density:   toggle.Density,
		Source:    "feeder",
		Color:     style.Color,
		PointSize: style.PointSize,
	}
	if dn, ok := dynReg.LookupLayerDisplayName(layerType); ok && dn != "" {
		layer.Name = dn
	}
	if rm, ok := dynReg.LookupRenderingMode(layerType); ok {
		layer.RenderingMode = rm
	} else {
		layer.RenderingMode = "map"
	}
	if fm, ok := dynReg.LookupFilteringMode(layerType); ok {
		layer.FilteringMode = fm
	}

	return layer, nil
}

// GetLayerSnapshot returns current snapshot of entities for a layer with pagination
func (s *LayerService) GetLayerSnapshot(ctx context.Context, layerID string, limit, offset int) (*domain.SnapshotResult, error) {
	if limit <= 0 {
		limit = 500
	}

	layerType := domain.LayerType(layerID)

	// Try to get from cache first (real-time data)
	if s.cache != nil {
		entities, observations, totalCount, err := s.cache.GetLayerEntities(ctx, string(layerType), limit, offset)
		if err == nil && len(entities) > 0 {
			hasMore := int64(offset+limit) < totalCount
			return &domain.SnapshotResult{
				Entities:     entities,
				Observations: observations,
				TotalCount:   totalCount,
				HasMore:      hasMore,
			}, nil
		}
	}

	// Fallback to database (historical data)
	observations, err := s.obsRepo.GetLatestForLayer(ctx, string(layerType), 1000)
	if err != nil {
		return nil, err
	}

	// Collect all entity IDs for a single batch fetch (avoids N+1 queries).
	ids := make([]string, 0, len(observations))
	for _, obs := range observations {
		ids = append(ids, obs.EntityID)
	}
	fetched, err := s.entityRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	entityMap := make(map[string]*domain.Entity, len(fetched))
	for _, e := range fetched {
		entityMap[e.ID] = e
	}

	// Keep only observations that have a matching entity, maintaining
	// index alignment between both slices for the pagination logic below.
	entities := make([]*domain.Entity, 0, len(fetched))
	aligned := make([]*domain.Observation, 0, len(fetched))
	for _, obs := range observations {
		if e, ok := entityMap[obs.EntityID]; ok {
			entities = append(entities, e)
			aligned = append(aligned, obs)
		}
	}
	observations = aligned

	// Apply pagination to database results
	totalCount := int64(len(entities))
	start := offset
	if start > len(entities) {
		start = len(entities)
	}
	end := offset + limit
	if end > len(entities) {
		end = len(entities)
	}

	obsStart := start
	if obsStart > len(observations) {
		obsStart = len(observations)
	}
	obsEnd := end
	if obsEnd > len(observations) {
		obsEnd = len(observations)
	}

	hasMore := int64(offset+limit) < totalCount

	return &domain.SnapshotResult{
		Entities:     entities[start:end],
		Observations: observations[obsStart:obsEnd],
		TotalCount:   totalCount,
		HasMore:      hasMore,
	}, nil
}
