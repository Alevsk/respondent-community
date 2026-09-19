package domain

import (
	"strconv"
	"time"
)

// IndicatorValue represents a single global indicator reading.
type IndicatorValue struct {
	Key       string
	Value     string
	Label     string
	Unit      string
	Level     int32
	ChangePct float64
	ChangeAbs float64
	Format    string
	Precision int32
	Prefix    string
}

// IndicatorSnapshot is the current state of a global indicator layer.
type IndicatorSnapshot struct {
	LayerID      string
	LayerName    string
	TimestampMs  int64
	Values       []IndicatorValue
	Summary      string
	OverallLevel int32
}

// BuildIndicatorSnapshot merges observation metadata, computes levels from the
// spec, generates a summary, and returns the complete IndicatorSnapshot.
// This is pure domain logic with no IO dependencies.
func BuildIndicatorSnapshot(layerID, layerName string, observations []*Observation, spec *IndicatorSpec) *IndicatorSnapshot {
	if len(observations) == 0 || spec == nil {
		return nil
	}

	// Merge metadata from all observations in this layer into a single map.
	// Multiple indicator entities (e.g., noaa_space_weather + noaa_kp_index)
	// may feed the same layer. We merge their metadata.
	merged := make(map[string]string)
	var latestTS time.Time
	for _, obs := range observations {
		for k, v := range obs.Metadata {
			merged[k] = v
		}
		if obs.Timestamp.After(latestTS) {
			latestTS = obs.Timestamp
		}
	}

	// Map metadata to IndicatorValues using the spec.
	var values []IndicatorValue
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

		values = append(values, IndicatorValue{
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

	return &IndicatorSnapshot{
		LayerID:      layerID,
		LayerName:    layerName,
		TimestampMs:  latestTS.UnixMilli(),
		Values:       values,
		Summary:      summary,
		OverallLevel: maxLevel,
	}
}
