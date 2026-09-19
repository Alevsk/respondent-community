package realtime

import (
	"strconv"

	"github.com/Alevsk/respondent/internal/domain"
)

// IndicatorUpdateData is the data payload for "indicator.update" WS messages.
type IndicatorUpdateData struct {
	TimestampMs  int64              `json:"timestamp_ms"`
	Values       []IndicatorValueWS `json:"values"`
	Summary      string             `json:"summary"`
	OverallLevel int32              `json:"overall_level"`
}

// IndicatorValueWS represents a single indicator value in a WS message.
type IndicatorValueWS struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	Label     string  `json:"label"`
	Unit      string  `json:"unit"`
	Level     int32   `json:"level"`
	ChangePct float64 `json:"change_pct"`
	ChangeAbs float64 `json:"change_abs"`
	Format    string  `json:"format,omitempty"`
	Precision int32   `json:"precision,omitempty"`
	Prefix    string  `json:"prefix,omitempty"`
}

func isIndicatorLayer(dynReg LayerRegistry, layerType string) bool {
	mode, ok := dynReg.LookupRenderingMode(domain.LayerType(layerType))
	return ok && mode == "indicator"
}

func buildIndicatorUpdate(dynReg LayerRegistry, layerType string, obs *domain.Observation) *IndicatorUpdateData {
	if obs == nil || obs.Metadata == nil {
		return nil
	}

	spec, ok := dynReg.LookupIndicatorSpec(domain.LayerType(layerType))
	if !ok || spec == nil {
		return nil
	}

	var values []IndicatorValueWS
	var maxLevel int32

	for _, vs := range spec.Values {
		rawVal, exists := obs.Metadata[vs.SourceField]
		if !exists {
			rawVal = "0"
		}

		var changePctStr string
		var changePct, changeAbs float64
		if vs.ChangeSourceField != "" {
			if raw, ok := obs.Metadata[vs.ChangeSourceField]; ok {
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

		values = append(values, IndicatorValueWS{
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

	summary := spec.ComputeSummary(obs.Metadata, maxLevel)

	return &IndicatorUpdateData{
		TimestampMs:  obs.Timestamp.UnixMilli(),
		Values:       values,
		Summary:      summary,
		OverallLevel: maxLevel,
	}
}
