package grpctransport

import (
	"encoding/json"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	respondentv1 "github.com/Alevsk/respondent/gen/go"
	"github.com/Alevsk/respondent/internal/domain"
)

// domainLayerToProto converts a domain Layer to a proto Layer.
func domainLayerToProto(layer *domain.Layer) *respondentv1.Layer {
	if layer == nil {
		return nil
	}
	pl := &respondentv1.Layer{
		Id:            layer.ID,
		Name:          layer.Name,
		Type:          layer.Type,
		Enabled:       layer.Enabled,
		Mode:          respondentv1.LayerMode(domain.LayerModeFromString(layer.Mode)),
		Density:       layer.Density,
		Source:        layer.Source,
		LastUpdate:    layer.LastUpdate.UnixMilli(),
		Count:         layer.Count,
		Color:         layer.Color,
		PointSize:     layer.PointSize,
		RenderingMode: respondentv1.RenderingMode(domain.RenderingModeFromString(layer.RenderingMode)),
		FilteringMode: respondentv1.FilteringMode(domain.FilteringModeFromString(layer.FilteringMode)),
	}
	if dc := layer.DisplayConfig; dc != nil {
		pl.DisplayConfig = domainDisplayConfigToProto(dc)
	}
	if hc := layer.HistoryConfig; hc != nil {
		pl.HistoryConfig = &respondentv1.HistoryConfig{
			MaxLookbackHours:  hc.MaxLookbackHours,
			MaxRangeSpanHours: hc.MaxRangeSpanHours,
		}
	}
	return pl
}

// domainDisplayConfigToProto converts a domain LayerDisplayConfig to a proto LayerDisplayConfig.
func domainDisplayConfigToProto(dc *domain.LayerDisplayConfig) *respondentv1.LayerDisplayConfig {
	pdc := &respondentv1.LayerDisplayConfig{}
	if dc.Icon != nil {
		pdc.Icon = &respondentv1.IconConfig{
			Shape:         dc.Icon.Shape,
			Rotatable:     dc.Icon.Rotatable,
			Interpolation: dc.Icon.Interpolation,
			Scale:         dc.Icon.Scale,
		}
	}
	if dc.Trail != nil {
		pdc.Trail = &respondentv1.TrailConfig{
			Color:   dc.Trail.Color,
			Width:   dc.Trail.Width,
			Opacity: dc.Trail.Opacity,
		}
	}
	if dc.Style != nil {
		pdc.Style = &respondentv1.StyleConfig{
			Color:     dc.Style.Color,
			PointSize: dc.Style.PointSize,
		}
	}
	for _, fr := range dc.FieldRenderers {
		pfr := &respondentv1.FieldRendererConfig{
			Keys:     fr.Keys,
			Label:    fr.Label,
			Priority: fr.Priority,
			Format: &respondentv1.FieldFormat{
				Type:      respondentv1.FieldType(domain.FieldTypeFromString(fr.Format.Type)),
				Prefix:    fr.Format.Prefix,
				Suffix:    fr.Format.Suffix,
				Transform: respondentv1.FieldTransform(domain.FieldTransformFromString(fr.Format.Transform)),
			},
		}
		if fr.Format.Precision != nil {
			pfr.Format.Precision = fr.Format.Precision
		}
		pdc.FieldRenderers = append(pdc.FieldRenderers, pfr)
	}
	if dc.ColorBy != nil {
		values := make(map[string]string, len(dc.ColorBy.Values))
		for k, v := range dc.ColorBy.Values {
			values[k] = v
		}
		pdc.ColorBy = &respondentv1.ColorByConfig{
			Field:        dc.ColorBy.Field,
			Values:       values,
			DefaultColor: dc.ColorBy.DefaultColor,
		}
	}
	return pdc
}

// domainEntityToProto converts a domain Entity to a proto Entity.
func domainEntityToProto(entity *domain.Entity) *respondentv1.Entity {
	if entity == nil {
		return nil
	}
	pe := &respondentv1.Entity{
		Id:         entity.ID,
		ExternalId: entity.ExternalID,
		LayerType:  entity.LayerType,
		Name:       entity.Name,
		Metadata:   entity.Metadata,
	}
	if len(entity.AIMetadata) > 0 {
		if b, err := json.Marshal(entity.AIMetadata); err == nil {
			pe.AiMetadataJson = string(b)
		}
	}
	return pe
}

// domainObservationToProto converts a domain Observation to a proto Observation.
func domainObservationToProto(obs *domain.Observation) *respondentv1.Observation {
	if obs == nil {
		return nil
	}
	proto := &respondentv1.Observation{
		EntityId:  obs.EntityID,
		AltitudeM: obs.AltitudeM,
		Ts:        obs.Timestamp.UnixMilli(),
		Velocity:  obs.Velocity,
		Metadata:  obs.Metadata,
	}
	if obs.Position != nil {
		proto.Position = &respondentv1.GeoPoint{
			Lat:  obs.Position.Lat,
			Lon:  obs.Position.Lon,
			AltM: obs.Position.Alt,
		}
	}
	if obs.EventTime != nil {
		proto.EventTimeMs = obs.EventTime.UnixMilli()
	}
	if obs.EventEnd != nil {
		proto.EventEndMs = obs.EventEnd.UnixMilli()
	}
	return proto
}

// ── Proto conversion helpers ────────────────────────────────────────

// domainInsightToProto converts a domain AIInsight to a proto AIInsight.
func domainInsightToProto(insight *domain.AIInsight, logger ...zerolog.Logger) *respondentv1.AIInsight {
	if insight == nil {
		return nil
	}

	proto := &respondentv1.AIInsight{
		Id:             insight.ID,
		InsightType:    insight.InsightType,
		SourceName:     insight.SourceName,
		OperationName:  insight.OperationName,
		EntityIds:      insight.EntityIDs,
		ObservationIds: insight.ObservationIDs,
		CreatedAt:      timestamppb.New(insight.CreatedAt),
	}

	if len(insight.Entities) > 0 {
		proto.Entities = make([]*respondentv1.InsightEntityRef, len(insight.Entities))
		for i, e := range insight.Entities {
			proto.Entities[i] = &respondentv1.InsightEntityRef{
				Id:         e.ID,
				ExternalId: e.ExternalID,
				Name:       e.Name,
				LayerType:  e.LayerType,
			}
		}
	}

	if insight.LayerType != nil {
		proto.LayerType = *insight.LayerType
	}

	if insight.Attention != nil {
		proto.Attention = respondentv1.AttentionLevel(domain.AttentionLevelFromString(*insight.Attention))
	}
	if insight.AttentionRank != nil {
		proto.AttentionRank = int32(*insight.AttentionRank)
	}

	if insight.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*insight.ExpiresAt)
	}

	if insight.Result != nil {
		resultStruct, err := structpb.NewStruct(insight.Result)
		if err != nil {
			l := zerolog.Nop()
			if len(logger) > 0 {
				l = logger[0]
			}
			l.Warn().Err(err).Str("insight_id", insight.ID).Msg("failed to convert insight result to proto Struct")
		} else {
			proto.Result = resultStruct
		}
	}

	return proto
}

// domainIndicatorValueToProto converts a domain IndicatorValue to a proto IndicatorValue.
func domainIndicatorValueToProto(val domain.IndicatorValue) *respondentv1.IndicatorValue {
	return &respondentv1.IndicatorValue{
		Key:       val.Key,
		Value:     val.Value,
		Label:     val.Label,
		Unit:      val.Unit,
		Level:     val.Level,
		ChangePct: val.ChangePct,
		ChangeAbs: val.ChangeAbs,
		Format:    val.Format,
		Precision: val.Precision,
		Prefix:    val.Prefix,
	}
}

// domainIndicatorSnapshotToProto converts a domain IndicatorSnapshot to a proto IndicatorSnapshot.
func domainIndicatorSnapshotToProto(snap domain.IndicatorSnapshot) *respondentv1.IndicatorSnapshot {
	protoValues := make([]*respondentv1.IndicatorValue, len(snap.Values))
	for i, val := range snap.Values {
		protoValues[i] = domainIndicatorValueToProto(val)
	}
	return &respondentv1.IndicatorSnapshot{
		LayerId:      snap.LayerID,
		LayerName:    snap.LayerName,
		TimestampMs:  snap.TimestampMs,
		Values:       protoValues,
		Summary:      snap.Summary,
		OverallLevel: snap.OverallLevel,
	}
}
