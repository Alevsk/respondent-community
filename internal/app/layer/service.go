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
	// AllLayerTypes returns every layer type currently declared by a loaded
	// source. It is the authority on which layers exist: rows left behind by a
	// source that has since changed its layer_type are data without a layer.
	AllLayerTypes() []domain.LayerType
	GetLayerStyle(lt domain.LayerType) domain.LayerStyle
	LookupDisplayConfig(lt domain.LayerType) (*domain.LayerDisplayConfig, bool)
	LookupLayerDisplayName(lt domain.LayerType) (string, bool)
	LookupFilteringMode(lt domain.LayerType) (string, bool)
	LookupHistoryConfig(lt domain.LayerType) (*domain.HistoryConfig, bool)
	LookupRenderingMode(lt domain.LayerType) (string, bool)
}

// LayerService implements the LayerService gRPC service.
// It depends on interfaces (domain.EntityRepository, domain.ObservationRepository)
// rather than concrete implementations, following the Dependency Inversion Principle.
type LayerService struct {
	entityRepo domain.EntityRepository
	obsRepo    domain.ObservationRepository
	dynReg     SourceRegistry
}

// maxSnapshotLimit caps how many entities one snapshot request may materialise.
// The limit arrives from an unauthenticated HTTP query parameter, and without a
// ceiling a single request would build a response holding an entire layer —
// 34,936 entities for power_plants — on a host sized for far less.
const maxSnapshotLimit = 5000

// NewLayerService creates a new LayerService with interface-based dependencies.
func NewLayerService(
	entityRepo domain.EntityRepository,
	obsRepo domain.ObservationRepository,
	dynReg SourceRegistry,
) *LayerService {
	return &LayerService{
		entityRepo: entityRepo,
		obsRepo:    obsRepo,
		dynReg:     dynReg,
	}
}

// GetLayers returns every layer a loaded source declares that also holds data,
// enriched with display/history metadata from the declarative dynamic registry.
//
// Declarations decide which layers exist; the database only says which of them
// have entities. Sourcing existence from the database instead would resurrect
// every layer_type ever written: renaming a source's layer_type in YAML (or
// deleting the source) strands its old rows, and those rows would otherwise
// keep answering "this layer exists" for the life of the database file.
func (s *LayerService) GetLayers(ctx context.Context) ([]*domain.Layer, error) {
	var layers []*domain.Layer
	dynReg := s.dynReg

	declared := make(map[string]struct{})
	for _, lt := range dynReg.AllLayerTypes() {
		declared[string(lt)] = struct{}{}
	}

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
		if _, ok := declared[lt]; !ok {
			continue
		}
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

// GetLayerSnapshot returns one page of a layer's entities with their latest
// observations.
//
// It reads the durable store directly. It used to consult a hot cache first and
// return whatever that held as soon as it held anything, which made the answer
// depend on what had been ingested since boot and, because the cache iterated a
// Go map, differ between two identical requests. The store answers the same
// question the same way every time, and TotalCount now comes from the store's
// own per-layer count rather than from the length of the page.
func (s *LayerService) GetLayerSnapshot(ctx context.Context, layerID string, limit, offset int) (*domain.SnapshotResult, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > maxSnapshotLimit {
		limit = maxSnapshotLimit
	}
	if offset < 0 {
		offset = 0
	}

	layerType := string(domain.LayerType(layerID))

	snapshots, err := s.obsRepo.GetLatestForLayerPage(ctx, layerType, limit, offset)
	if err != nil {
		return nil, err
	}

	entities := make([]*domain.Entity, 0, len(snapshots))
	observations := make([]*domain.Observation, 0, len(snapshots))
	for i := range snapshots {
		entities = append(entities, &snapshots[i].Entity)
		observations = append(observations, &snapshots[i].Observation)
	}

	counts, err := s.entityRepo.CountByLayerType(ctx)
	if err != nil {
		return nil, err
	}
	totalCount := counts[layerType]

	return &domain.SnapshotResult{
		Entities:     entities,
		Observations: observations,
		TotalCount:   totalCount,
		HasMore:      int64(offset+len(entities)) < totalCount,
	}, nil
}
