package realtime_test

import "github.com/Alevsk/respondent/internal/domain"

// viewportRegistry returns a dynamic source registry with the given layer types
// marked as viewport-filtered (spatial). Tests that exercise spatial behavior use
// this so the server's isSpatialLayer resolves from the registry — the single
// source of truth — exactly as it does in production.
func viewportRegistry(layerTypes ...string) *domain.DynamicSourceRegistry {
	reg := domain.NewDynamicSourceRegistry()
	for _, lt := range layerTypes {
		reg.SetFilteringMode(domain.LayerType(lt), string(domain.FilteringViewport))
	}
	return reg
}
