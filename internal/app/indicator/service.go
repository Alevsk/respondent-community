package indicator

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// SourceRegistry is the read-only view of declarative indicator metadata the
// indicator service depends on. *domain.DynamicSourceRegistry satisfies it;
// depending on the interface keeps the service decoupled from the concrete
// registry (DIP).
type SourceRegistry interface {
	AllIndicatorLayerTypes() []domain.LayerType
	LookupIndicatorSpec(lt domain.LayerType) (*domain.IndicatorSpec, bool)
	LookupRenderingMode(lt domain.LayerType) (string, bool)
}

// IndicatorService provides access to global indicator data.
type IndicatorService struct {
	entityRepo domain.EntityRepository
	obsRepo    domain.ObservationRepository
	dynReg     SourceRegistry
	logger     zerolog.Logger
}

// NewIndicatorService creates a new IndicatorService with interface-based
// dependencies. The optional logger allows callers to inject a configured
// zerolog.Logger; if omitted, logging is discarded (Nop).
func NewIndicatorService(
	entityRepo domain.EntityRepository,
	obsRepo domain.ObservationRepository,
	dynReg SourceRegistry,
	logger ...zerolog.Logger,
) *IndicatorService {
	l := zerolog.Nop()
	if len(logger) > 0 {
		l = logger[0]
	}
	return &IndicatorService{
		entityRepo: entityRepo,
		obsRepo:    obsRepo,
		dynReg:     dynReg,
		logger:     l,
	}
}

// GetGlobalIndicators returns the latest indicator snapshots for global indicator layers.
// If layerIDs is empty, all indicator layers are returned.
func (s *IndicatorService) GetGlobalIndicators(ctx context.Context, layerIDs []string) ([]domain.IndicatorSnapshot, error) {
	dynReg := s.dynReg

	// Determine which indicator layer types to query.
	var targetLayers []domain.LayerType
	if len(layerIDs) > 0 {
		for _, id := range layerIDs {
			lt := domain.LayerType(id)
			if rm, ok := dynReg.LookupRenderingMode(lt); ok && rm == "indicator" {
				targetLayers = append(targetLayers, lt)
			}
		}
	} else {
		targetLayers = dynReg.AllIndicatorLayerTypes()
	}

	if len(targetLayers) == 0 {
		return nil, nil
	}

	var snapshots []domain.IndicatorSnapshot

	for _, lt := range targetLayers {
		spec, ok := dynReg.LookupIndicatorSpec(lt)
		if !ok || spec == nil {
			continue
		}

		snapshot, err := s.buildSnapshot(ctx, lt, spec, dynReg)
		if err != nil {
			s.logger.Warn().
				Str("layer_type", string(lt)).
				Err(err).
				Msg("failed to build indicator snapshot")
			continue
		}
		if snapshot != nil {
			snapshots = append(snapshots, *snapshot)
		}
	}

	return snapshots, nil
}

// buildSnapshot builds an IndicatorSnapshot for a single layer type by reading
// the latest observations for all entities in that layer.
func (s *IndicatorService) buildSnapshot(ctx context.Context, lt domain.LayerType, spec *domain.IndicatorSpec, _ SourceRegistry) (*domain.IndicatorSnapshot, error) {
	if s.obsRepo == nil {
		return nil, nil
	}
	snapshots, err := s.obsRepo.GetLatestForLayerPage(ctx, string(lt), 100, 0)
	if err != nil {
		return nil, err
	}
	latestObs := make([]*domain.Observation, 0, len(snapshots))
	for i := range snapshots {
		latestObs = append(latestObs, &snapshots[i].Observation)
	}

	if len(latestObs) == 0 {
		return nil, nil
	}

	// Merge metadata from all observations in this layer into a single map.
	// Multiple indicator entities (e.g., noaa_space_weather + noaa_kp_index)
	// may feed the same layer. We merge their metadata.
	merged := make(map[string]string)
	var latestTS time.Time
	for _, obs := range latestObs {
		for k, v := range obs.Metadata {
			merged[k] = v
		}
		if obs.Timestamp.After(latestTS) {
			latestTS = obs.Timestamp
		}
	}

	// Map metadata to IndicatorValues using the spec.
	var values []domain.IndicatorValue
	var maxLevel int32

	for _, vs := range spec.Values {
		rawVal, exists := merged[vs.SourceField]
		if !exists {
			rawVal = "0"
		}

		var changePctStr string
		var changePct, changeAbs float64
		if vs.ChangeSourceField != "" {
			if raw, ok := merged[vs.ChangeSourceField]; ok {
				changePctStr = raw
				changePct, _ = strconv.ParseFloat(raw, 64)
			}
			if val, err := strconv.ParseFloat(rawVal, 64); err == nil && changePct != 0 {
				changeAbs = val * changePct / 100
			}
		}

		level := vs.ComputeLevel(rawVal, changePctStr)
		if level > maxLevel {
			maxLevel = level
		}

		values = append(values, domain.IndicatorValue{
			Key:       vs.Key,
			Value:     rawVal,
			Label:     vs.Label,
			Unit:      vs.Unit,
			Level:     level,
			ChangePct: changePct,
			ChangeAbs: changeAbs,
			Format:    vs.Format,
			Precision: int32(vs.Precision),
			Prefix:    vs.Prefix,
		})
	}

	// Determine summary from overall level.
	summary := spec.ComputeSummary(merged, maxLevel)

	// Look up display name from the dynamic registry.
	layerName := domain.FormatLayerName(string(lt))

	return &domain.IndicatorSnapshot{
		LayerID:      string(lt),
		LayerName:    layerName,
		TimestampMs:  latestTS.UnixMilli(),
		Values:       values,
		Summary:      summary,
		OverallLevel: maxLevel,
	}, nil
}
