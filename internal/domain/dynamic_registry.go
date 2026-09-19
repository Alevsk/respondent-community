package domain

import (
	"sync"
	"time"
)

// DynamicSourceRegistry stores all runtime-registered SourceType/LayerType
// mappings from declarative source definitions. It is safe for concurrent use.
type DynamicSourceRegistry struct {
	mu             sync.RWMutex
	sourceToLayer  map[SourceType]LayerType
	layerToSources map[LayerType][]SourceType
	displayConfigs map[LayerType]*LayerDisplayConfig

	// v2 declarative source metadata (per layer type)
	filteringModes     map[LayerType]string // "viewport" or "all"
	backfillThresholds map[LayerType]int
	onDemandURLs       map[LayerType]string
	historyConfigs     map[LayerType]*HistoryConfig
	cacheTTLs          map[LayerType]time.Duration
	renderingModes     map[LayerType]string // "map" (default) or "indicator"
	indicatorSpecs     map[LayerType]*IndicatorSpec
	geoCacheConfigs    map[LayerType]GeoCacheConfig
	layerDisplayNames  map[LayerType]string // optional per-layer label override
}

func NewDynamicSourceRegistry() *DynamicSourceRegistry {
	return &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
		historyConfigs:     make(map[LayerType]*HistoryConfig),
		cacheTTLs:          make(map[LayerType]time.Duration),
		renderingModes:     make(map[LayerType]string),
		indicatorSpecs:     make(map[LayerType]*IndicatorSpec),
		geoCacheConfigs:    make(map[LayerType]GeoCacheConfig),
		layerDisplayNames:  make(map[LayerType]string),
	}
}

// Register adds a dynamic source type -> layer type mapping.
// If the source type is already registered, it updates the mapping.
func (r *DynamicSourceRegistry) Register(st SourceType, lt LayerType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove from old layer mapping if changing layer type
	if oldLT, exists := r.sourceToLayer[st]; exists && oldLT != lt {
		r.removeFromLayerLocked(oldLT, st)
	}

	r.sourceToLayer[st] = lt

	// Add to layer -> sources mapping (avoid duplicates)
	sources := r.layerToSources[lt]
	for _, s := range sources {
		if s == st {
			return
		}
	}
	r.layerToSources[lt] = append(sources, st)
}

// RegisterWithDisplay adds a dynamic source type -> layer type mapping along with display config.
func (r *DynamicSourceRegistry) RegisterWithDisplay(st SourceType, lt LayerType, dc *LayerDisplayConfig) {
	r.Register(st, lt)
	if dc != nil {
		r.mu.Lock()
		r.displayConfigs[lt] = dc
		r.mu.Unlock()
	}
}

// LookupDisplayConfig returns the display config for a layer type, if registered.
func (r *DynamicSourceRegistry) LookupDisplayConfig(lt LayerType) (*LayerDisplayConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dc, ok := r.displayConfigs[lt]
	return dc, ok
}

// Unregister removes a dynamic source type.
func (r *DynamicSourceRegistry) Unregister(st SourceType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lt, exists := r.sourceToLayer[st]
	if !exists {
		return
	}

	delete(r.sourceToLayer, st)
	r.removeFromLayerLocked(lt, st)
}

// removeFromLayerLocked removes a source type from the layer -> sources mapping.
// Caller must hold mu.
func (r *DynamicSourceRegistry) removeFromLayerLocked(lt LayerType, st SourceType) {
	sources := r.layerToSources[lt]
	for i, s := range sources {
		if s == st {
			r.layerToSources[lt] = append(sources[:i], sources[i+1:]...)
			break
		}
	}
	if len(r.layerToSources[lt]) == 0 {
		delete(r.layerToSources, lt)
	}
}

// LookupLayerType returns the layer type for a dynamic source type.
// Returns ("", false) if not registered.
func (r *DynamicSourceRegistry) LookupLayerType(st SourceType) (LayerType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lt, ok := r.sourceToLayer[st]
	return lt, ok
}

// IsRegistered returns true if the source type is dynamically registered.
func (r *DynamicSourceRegistry) IsRegistered(st SourceType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.sourceToLayer[st]
	return ok
}

// LookupSourceTypes returns all source types that feed the given layer type.
// Returns nil if no dynamic sources feed this layer.
func (r *DynamicSourceRegistry) LookupSourceTypes(lt LayerType) []SourceType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sources := r.layerToSources[lt]
	if sources == nil {
		return nil
	}
	result := make([]SourceType, len(sources))
	copy(result, sources)
	return result
}

// AllSourceTypes returns all dynamically registered source types.
func (r *DynamicSourceRegistry) AllSourceTypes() map[SourceType]LayerType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[SourceType]LayerType, len(r.sourceToLayer))
	for st, lt := range r.sourceToLayer {
		result[st] = lt
	}
	return result
}

// SetFilteringMode stores the filtering mode for a layer type.
// mode should be "viewport" or "all".
func (r *DynamicSourceRegistry) SetFilteringMode(lt LayerType, mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.filteringModes[lt] = mode
}

// LookupFilteringMode returns the filtering mode for a layer type.
// Returns ("", false) if not registered.
func (r *DynamicSourceRegistry) LookupFilteringMode(lt LayerType) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mode, ok := r.filteringModes[lt]
	return mode, ok
}

// SetLayerDisplayName stores an optional human-readable label override for a layer
// type. When set, the layer service uses it as the layer's Name instead of the
// title-cased layer_type (FormatLayerName), letting a source label its layer (e.g.
// "Traffic Stations (Mexico)") rather than the bare "Traffic Stations".
func (r *DynamicSourceRegistry) SetLayerDisplayName(lt LayerType, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layerDisplayNames[lt] = name
}

// LookupLayerDisplayName returns the label override for a layer type.
// Returns ("", false) if none was registered.
func (r *DynamicSourceRegistry) LookupLayerDisplayName(lt LayerType) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok := r.layerDisplayNames[lt]
	return name, ok
}

// SetBackfillThreshold stores the backfill threshold for a layer type.
func (r *DynamicSourceRegistry) SetBackfillThreshold(lt LayerType, threshold int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backfillThresholds[lt] = threshold
}

// LookupBackfillThreshold returns the backfill threshold for a layer type.
// Returns (0, false) if not registered.
func (r *DynamicSourceRegistry) LookupBackfillThreshold(lt LayerType) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	threshold, ok := r.backfillThresholds[lt]
	return threshold, ok
}

// SetOnDemandURL stores the on-demand URL template for a layer type.
func (r *DynamicSourceRegistry) SetOnDemandURL(lt LayerType, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onDemandURLs[lt] = url
}

// LookupOnDemandURL returns the on-demand URL template for a layer type.
// Returns ("", false) if not registered.
func (r *DynamicSourceRegistry) LookupOnDemandURL(lt LayerType) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	url, ok := r.onDemandURLs[lt]
	return url, ok
}

// AllFilteringModes returns all registered filtering modes.
func (r *DynamicSourceRegistry) AllFilteringModes() map[LayerType]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[LayerType]string, len(r.filteringModes))
	for lt, mode := range r.filteringModes {
		result[lt] = mode
	}
	return result
}

// AllBackfillThresholds returns all registered backfill thresholds.
func (r *DynamicSourceRegistry) AllBackfillThresholds() map[LayerType]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[LayerType]int, len(r.backfillThresholds))
	for lt, t := range r.backfillThresholds {
		result[lt] = t
	}
	return result
}

// SetHistoryConfig stores the history config for a layer type.
func (r *DynamicSourceRegistry) SetHistoryConfig(lt LayerType, hc *HistoryConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.historyConfigs[lt] = hc
}

// LookupHistoryConfig returns the history config for a layer type.
// Returns (nil, false) if not registered.
func (r *DynamicSourceRegistry) LookupHistoryConfig(lt LayerType) (*HistoryConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hc, ok := r.historyConfigs[lt]
	return hc, ok
}

// AllOnDemandURLs returns all registered on-demand URL templates.
func (r *DynamicSourceRegistry) AllOnDemandURLs() map[LayerType]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[LayerType]string, len(r.onDemandURLs))
	for lt, url := range r.onDemandURLs {
		result[lt] = url
	}
	return result
}

// SetCacheTTL stores the cache TTL for a layer type.
func (r *DynamicSourceRegistry) SetCacheTTL(lt LayerType, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheTTLs[lt] = ttl
}

// SetRenderingMode stores the rendering mode for a layer type.
// mode should be "map" or "indicator".
func (r *DynamicSourceRegistry) SetRenderingMode(lt LayerType, mode string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renderingModes[lt] = mode
}

// LookupRenderingMode returns the rendering mode for a layer type.
// Returns ("", false) if not registered.
func (r *DynamicSourceRegistry) LookupRenderingMode(lt LayerType) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mode, ok := r.renderingModes[lt]
	return mode, ok
}

// SetIndicatorSpec stores the indicator spec for a layer type.
func (r *DynamicSourceRegistry) SetIndicatorSpec(lt LayerType, spec *IndicatorSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.indicatorSpecs[lt] = spec
}

// LookupIndicatorSpec returns the indicator spec for a layer type.
// Returns (nil, false) if not registered.
func (r *DynamicSourceRegistry) LookupIndicatorSpec(lt LayerType) (*IndicatorSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.indicatorSpecs[lt]
	return spec, ok
}

// SetGeoCacheConfig stores the geo cache config for a layer type.
func (r *DynamicSourceRegistry) SetGeoCacheConfig(lt LayerType, cfg GeoCacheConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.geoCacheConfigs[lt] = cfg
}

// LookupGeoCacheConfig returns the geo cache config for a layer type.
// Returns (GeoCacheConfig{}, false) if not registered.
func (r *DynamicSourceRegistry) LookupGeoCacheConfig(lt LayerType) (GeoCacheConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.geoCacheConfigs[lt]
	return cfg, ok
}

// AllIndicatorLayerTypes returns all layer types that have rendering_mode "indicator".
func (r *DynamicSourceRegistry) AllIndicatorLayerTypes() []LayerType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []LayerType
	for lt, mode := range r.renderingModes {
		if mode == "indicator" {
			result = append(result, lt)
		}
	}
	return result
}

// AllCacheTTLs returns all registered cache TTLs.
func (r *DynamicSourceRegistry) AllCacheTTLs() map[LayerType]time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[LayerType]time.Duration, len(r.cacheTTLs))
	for lt, ttl := range r.cacheTTLs {
		result[lt] = ttl
	}
	return result
}

// AllLayerTypes returns all layer types that have at least one registered source.
func (r *DynamicSourceRegistry) AllLayerTypes() []LayerType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]LayerType, 0, len(r.layerToSources))
	for lt := range r.layerToSources {
		result = append(result, lt)
	}
	return result
}

// GetLayerStyle returns the LayerStyle for the given layer type, derived from
// the registered LayerDisplayConfig. Returns DefaultLayerStyle if no display
// config is registered or the config has no Style set.
func (r *DynamicSourceRegistry) GetLayerStyle(lt LayerType) LayerStyle {
	r.mu.RLock()
	dc, ok := r.displayConfigs[lt]
	r.mu.RUnlock()
	if ok && dc != nil && dc.Style != nil {
		style := DefaultLayerStyle
		if dc.Style.Color != "" {
			style.Color = dc.Style.Color
		}
		if dc.Style.PointSize > 0 {
			style.PointSize = dc.Style.PointSize
		}
		return style
	}
	return DefaultLayerStyle
}
