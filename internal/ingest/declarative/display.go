package declarative

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
	"gopkg.in/yaml.v3"
)

// DisplaySpecToDomain converts a declarative DisplaySpec to a domain LayerDisplayConfig.
// This is the single source of truth for this conversion, used by both the feeder
// (registerDeclarativeSources) and the server (RegisterDisplayConfigs).
func DisplaySpecToDomain(d *DisplaySpec) *domain.LayerDisplayConfig {
	if d == nil {
		return nil
	}
	dc := &domain.LayerDisplayConfig{
		Icon: &domain.IconConfig{
			Shape:         d.Icon.Shape,
			Rotatable:     d.Icon.Rotatable,
			Interpolation: d.Icon.Interpolation,
			Scale:         d.Icon.Scale,
		},
		Trail: &domain.TrailConfig{
			Color:   d.Trail.Color,
			Width:   d.Trail.Width,
			Opacity: d.Trail.Opacity,
		},
		Style: &domain.StyleConfig{
			Color:     d.Style.Color,
			PointSize: int32(d.Style.PointSize),
		},
	}
	for _, fr := range d.FieldRenderers {
		dfr := domain.FieldRendererConfig{
			Keys:     fr.Keys,
			Label:    fr.Label,
			Priority: int32(fr.Priority),
			Format: domain.FieldFormat{
				Type:      fr.Format.Type,
				Prefix:    fr.Format.Prefix,
				Suffix:    fr.Format.Suffix,
				Transform: fr.Format.Transform,
			},
		}
		if fr.Format.Precision != nil {
			p := int32(*fr.Format.Precision)
			dfr.Format.Precision = &p
		}
		dc.FieldRenderers = append(dc.FieldRenderers, dfr)
	}
	if d.ColorBy != nil {
		values := make(map[string]string, len(d.ColorBy.Values))
		for k, v := range d.ColorBy.Values {
			values[k] = v
		}
		dc.ColorBy = &domain.ColorByConfig{
			Field:        d.ColorBy.Field,
			Values:       values,
			DefaultColor: d.ColorBy.DefaultColor,
		}
	}
	return dc
}

// displayOnlyDef is a minimal struct for extracting display config and v2 metadata
// from YAML without requiring the full CEL compiler or validation pipeline.
type displayOnlyDef struct {
	SourceType string      `yaml:"source_type"`
	LayerType  string      `yaml:"layer_type"`
	Display    DisplaySpec `yaml:"display"`
	EntityType string      `yaml:"entity_type"`

	// v2 fields extracted for server-side wiring
	Filtering string            `yaml:"filtering"`
	Backfill  *displayBackfill  `yaml:"backfill"`
	GeoCache  *displayGeoCache  `yaml:"geo_cache"`
	Transport *displayTransport `yaml:"transport"`
	History   *HistorySpec      `yaml:"history"`
	Cache     *displayCache     `yaml:"cache"`
	Indicator *IndicatorSpec    `yaml:"indicator"`
}

// displayCache is a lightweight struct for extracting cache TTL.
type displayCache struct {
	TTL Duration `yaml:"ttl"`
}

// displayBackfill is a lightweight struct for extracting backfill config.
type displayBackfill struct {
	Threshold int `yaml:"threshold"`
}

// displayGeoCache is a lightweight struct for extracting geo cache config.
type displayGeoCache struct {
	OverfetchRatio      float64 `yaml:"overfetch_ratio"`
	AliveRatioThreshold float64 `yaml:"alive_ratio_threshold"`
	GCBatchSize         int     `yaml:"gc_batch_size"`
}

// displayTransport is a lightweight struct for extracting on_demand_url.
type displayTransport struct {
	OnDemandURL string `yaml:"on_demand_url"`
}

// SourceMetadata captures the fields needed to register a declarative source in
// the domain dynamic registry. Both the feeder (via SourceDefinition) and the
// server (via displayOnlyDef) construct this struct so that registration logic
// lives in a single place.
type SourceMetadata struct {
	SourceType        string
	LayerType         string
	LayerDisplayName  string // optional layer label override ("" = use FormatLayerName)
	Display           *DisplaySpec
	Filtering         string
	BackfillThreshold int // 0 means not set
	OnDemandURL       string
	History           *HistorySpec           // nil means use server defaults
	CacheTTL          time.Duration          // 0 means not set
	EntityType        string                 // "geo_entity" or "global_indicator"
	Indicator         *IndicatorSpec         // nil for geo_entity sources
	GeoCacheConfig    *domain.GeoCacheConfig // nil means not set
}

// SourceMetadataFromDefinition builds a SourceMetadata from a full SourceDefinition.
func SourceMetadataFromDefinition(def *SourceDefinition) SourceMetadata {
	sm := SourceMetadata{
		SourceType:       def.SourceType,
		LayerType:        def.LayerType,
		LayerDisplayName: def.LayerDisplayName,
		Display:          &def.Display,
		Filtering:        def.Filtering,
		OnDemandURL:      def.Transport.OnDemandURL,
		CacheTTL:         def.Cache.TTL.Duration,
		EntityType:       def.EntityType,
	}
	if def.Backfill != nil {
		sm.BackfillThreshold = def.Backfill.Threshold
	}
	if def.GeoCache != nil {
		defaults := domain.DefaultGeoCacheConfig()
		cfg := domain.GeoCacheConfig{
			OverfetchRatio:      defaults.OverfetchRatio,
			AliveRatioThreshold: defaults.AliveRatioThreshold,
			GCBatchSize:         defaults.GCBatchSize,
		}
		if def.GeoCache.OverfetchRatio > 0 {
			cfg.OverfetchRatio = def.GeoCache.OverfetchRatio
		}
		if def.GeoCache.AliveRatioThreshold > 0 {
			cfg.AliveRatioThreshold = def.GeoCache.AliveRatioThreshold
		}
		if def.GeoCache.GCBatchSize > 0 {
			cfg.GCBatchSize = def.GeoCache.GCBatchSize
		}
		sm.GeoCacheConfig = &cfg
	}
	sm.History = def.History
	sm.Indicator = def.Indicator
	return sm
}

// RegisterSourceMetadata registers a single declarative source's display config
// and v2 metadata (filtering mode, backfill threshold, on-demand URL) into the
// given dynamic registry. This is the single source of truth for metadata
// registration, used by both the feeder and the server processes.
// Returns an error if indicator CEL expressions fail to compile.
func RegisterSourceMetadata(sm SourceMetadata, dynReg *domain.DynamicSourceRegistry) error {
	lt := domain.LayerType(sm.LayerType)
	dc := DisplaySpecToDomain(sm.Display)

	dynReg.RegisterWithDisplay(
		domain.SourceType(sm.SourceType),
		lt,
		dc,
	)

	if sm.LayerDisplayName != "" {
		dynReg.SetLayerDisplayName(lt, sm.LayerDisplayName)
	}
	if sm.Filtering != "" {
		dynReg.SetFilteringMode(lt, sm.Filtering)
	}
	if sm.BackfillThreshold > 0 {
		dynReg.SetBackfillThreshold(lt, sm.BackfillThreshold)
	}
	if sm.OnDemandURL != "" {
		dynReg.SetOnDemandURL(lt, sm.OnDemandURL)
	}
	if sm.CacheTTL > 0 {
		dynReg.SetCacheTTL(lt, sm.CacheTTL)
	}
	if sm.GeoCacheConfig != nil {
		dynReg.SetGeoCacheConfig(lt, *sm.GeoCacheConfig)
	}
	if sm.History != nil {
		hc := &domain.HistoryConfig{
			MaxLookbackHours:  int32(sm.History.MaxLookback.Hours()),
			MaxRangeSpanHours: int32(sm.History.MaxRangeSpan.Hours()),
		}
		dynReg.SetHistoryConfig(lt, hc)
	}

	// Set rendering mode based on entity type.
	if sm.EntityType == string(domain.EntityTypeIndicator) {
		dynReg.SetRenderingMode(lt, "indicator")
	}

	// Register indicator spec if present. CEL expressions are compiled here
	// (control plane) so failures are caught at startup, not at runtime.
	if sm.Indicator != nil {
		domainSpec, err := indicatorSpecToDomain(sm.Indicator)
		if err != nil {
			return fmt.Errorf("indicator spec for %s: %w", sm.LayerType, err)
		}
		dynReg.SetIndicatorSpec(lt, domainSpec)
	}

	return nil
}

// indicatorSpecToDomain converts a declarative IndicatorSpec to a domain IndicatorSpec.
// CEL expressions (level_expr, summary_expr) are compiled into closures here at
// config load time (control plane). When no CEL expression is provided, default
// closures are built from LevelThresholds/MaxLevel data fields.
func indicatorSpecToDomain(spec *IndicatorSpec) (*domain.IndicatorSpec, error) {
	if spec == nil {
		return nil, nil
	}

	domainSpec := &domain.IndicatorSpec{
		Values: make([]domain.IndicatorValueSpec, len(spec.Values)),
	}

	for i, v := range spec.Values {
		dv := domain.IndicatorValueSpec{
			Key:               v.Key,
			Label:             v.Label,
			SourceField:       v.SourceField,
			Unit:              v.Unit,
			MaxLevel:          v.MaxLevel,
			LevelThresholds:   v.LevelThresholds,
			ChangeSourceField: v.ChangeSourceField,
			Format:            v.Format,
			Precision:         v.Precision,
			Prefix:            v.Prefix,
		}

		if v.LevelExpr != "" {
			fn, err := compileIndicatorLevelExpr(v.LevelExpr)
			if err != nil {
				return nil, fmt.Errorf("value %q: %w", v.Key, err)
			}
			dv.ComputeLevel = fn
		} else {
			dv.ComputeLevel = domain.DefaultComputeLevel(v.LevelThresholds, v.MaxLevel)
		}

		domainSpec.Values[i] = dv
	}

	if spec.SummaryExpr != "" {
		fn, err := compileIndicatorSummaryExpr(spec.SummaryExpr)
		if err != nil {
			return nil, err
		}
		domainSpec.ComputeSummary = fn
	} else {
		domainSpec.ComputeSummary = domain.DefaultComputeSummary()
	}

	return domainSpec, nil
}

// RegisterDisplayConfigs loads YAML source definitions from dir and registers
// their display configs in the domain dynamic registry. This is a lightweight
// alternative to full Loader.LoadDir for processes that only need display metadata
// (e.g. the respondent server, which serves GetLayers but doesn't run ingestion).
// A logger should be provided for diagnostic output; if nil, warnings are silently dropped.
func RegisterDisplayConfigs(dir string, dynReg *domain.DynamicSourceRegistry, log *logging.Logger) error {
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			if log != nil {
				log.Warn("failed to read display config file",
					logging.String("file", filePath),
					logging.String("error", err.Error()),
				)
			}
			continue
		}

		var def displayOnlyDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			if log != nil {
				log.Warn("failed to parse display config YAML",
					logging.String("file", filePath),
					logging.String("error", err.Error()),
				)
			}
			continue
		}

		if def.LayerType == "" || def.Display.Icon.Shape == "" {
			continue
		}

		sm := SourceMetadata{
			SourceType: def.SourceType,
			LayerType:  def.LayerType,
			Display:    &def.Display,
			Filtering:  def.Filtering,
			EntityType: def.EntityType,
		}
		if def.Backfill != nil {
			sm.BackfillThreshold = def.Backfill.Threshold
		}
		if def.Transport != nil {
			sm.OnDemandURL = def.Transport.OnDemandURL
		}
		if def.Cache != nil {
			sm.CacheTTL = def.Cache.TTL.Duration
		}
		if def.GeoCache != nil {
			defaults := domain.DefaultGeoCacheConfig()
			cfg := domain.GeoCacheConfig{
				OverfetchRatio:      defaults.OverfetchRatio,
				AliveRatioThreshold: defaults.AliveRatioThreshold,
				GCBatchSize:         defaults.GCBatchSize,
			}
			if def.GeoCache.OverfetchRatio > 0 {
				cfg.OverfetchRatio = def.GeoCache.OverfetchRatio
			}
			if def.GeoCache.AliveRatioThreshold > 0 {
				cfg.AliveRatioThreshold = def.GeoCache.AliveRatioThreshold
			}
			if def.GeoCache.GCBatchSize > 0 {
				cfg.GCBatchSize = def.GeoCache.GCBatchSize
			}
			sm.GeoCacheConfig = &cfg
		}
		sm.History = def.History
		sm.Indicator = def.Indicator

		if err := RegisterSourceMetadata(sm, dynReg); err != nil {
			if log != nil {
				log.Warn("failed to register source metadata",
					logging.String("file", filePath),
					logging.String("error", err.Error()),
				)
			}
			continue
		}
	}

	return nil
}
