package declarative

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/declarative/parsers"
	"github.com/Alevsk/respondent/internal/logging"
)

// buildParserConfig constructs a ParserConfig from the source definition.
func (a *DeclarativeAdapter) buildParserConfig(def *SourceDefinition) parsers.ParserConfig {
	parserCfg := parsers.ParserConfig{
		RecordsPath:     def.Parser.RecordsPath,
		MaxRecords:      def.Parser.MaxRecords,
		ArrayColumns:    def.Parser.ArrayColumns,
		ObjectToRecords: def.Parser.ObjectToRecords,
		ObjectKeyField:  def.Parser.ObjectKeyField,
		ArrayOfArrays:   def.Parser.ArrayOfArrays,
	}
	if def.Parser.CSVOptions != nil {
		parserCfg.CSVOptions = &parsers.CSVOpts{
			Delimiter:          def.Parser.CSVOptions.Delimiter,
			HasHeader:          def.Parser.CSVOptions.HasHeader,
			SkipLines:          def.Parser.CSVOptions.SkipLines,
			CollapseWhitespace: def.Parser.CSVOptions.CollapseWhitespace,
			CommentPrefix:      def.Parser.CSVOptions.CommentPrefix,
		}
	}
	return parserCfg
}

// processRecords evaluates CEL expressions against parsed records to produce entities and observations.
// Entities are deduplicated by ID (last writer wins) since each entity should appear once.
// Observations are always appended — multiple records mapping to the same entity produce
// multiple observations (e.g., daily time-series data for the same country). When recording
// mode is upsert, the DB handles conflicts on (entity_id, ts).
func (a *DeclarativeAdapter) processRecords(cs *CompiledSource, records []map[string]interface{}) ([]*domain.Entity, []*domain.Observation) {
	// Dedup entities only: entityIndex tracks position in entities slice by entity ID.
	// Observations are appended unconditionally — each record produces one observation.
	entityIndex := make(map[string]int) // entity.ID -> index in entities slice
	var entities []*domain.Entity
	var observations []*domain.Observation

	loggedErrors := 0
	dupeCount := 0

	for _, record := range records {
		entity, obs, ok := a.mapRecord(cs, record, &loggedErrors)
		if !ok {
			continue
		}

		// Deduplicate entities: if this entity ID was already seen, overwrite the earlier entry.
		// Observations are always appended — multiple records for the same entity are expected
		// when the source provides time-series data (e.g., daily snapshots per country).
		if idx, exists := entityIndex[entity.ID]; exists {
			entities[idx] = entity
			dupeCount++
		} else {
			entityIndex[entity.ID] = len(entities)
			entities = append(entities, entity)
		}
		observations = append(observations, obs)
	}

	if dupeCount > 0 {
		a.logger.Debug("deduplicated entities within batch",
			logging.String("source_name", a.name),
			logging.Int("entity_duplicates_merged", dupeCount),
			logging.Int("observations_produced", len(observations)),
		)
	}

	return entities, observations
}

// mapRecord runs the filter → entity → observation transform for a single record.
// It returns ok=false when the record is filtered out or fails mapping (errors are
// logged, bounded by loggedErrors). Shared by processRecords and ParseOnDemand so the
// declarative transform is defined exactly once.
func (a *DeclarativeAdapter) mapRecord(cs *CompiledSource, record map[string]interface{}, loggedErrors *int) (*domain.Entity, *domain.Observation, bool) {
	activation := map[string]interface{}{"record": record}

	if cs.Filter() != nil {
		pass, err := evalBool(cs.Filter(), activation)
		if err != nil {
			if *loggedErrors < maxLoggedErrors {
				a.logger.Warn("filter eval error, skipping record",
					logging.String("source_name", a.name),
					logging.String("field", "filter"),
					logging.Err("error", err),
				)
				*loggedErrors++
			}
			return nil, nil, false
		}
		if !pass {
			return nil, nil, false
		}
	}

	entity, err := a.mapEntity(cs, activation)
	if err != nil {
		if *loggedErrors < maxLoggedErrors {
			a.logger.Warn("entity mapping error, skipping record",
				logging.String("source_name", a.name),
				logging.Err("error", err),
			)
			*loggedErrors++
		}
		return nil, nil, false
	}

	obs, err := a.mapObservation(cs, activation, entity.ID)
	if err != nil {
		if *loggedErrors < maxLoggedErrors {
			a.logger.Warn("observation mapping error, skipping record",
				logging.String("source_name", a.name),
				logging.Err("error", err),
			)
			*loggedErrors++
		}
		return nil, nil, false
	}

	return entity, obs, true
}

// ParseOnDemand runs the source's declarative parse + transform over a raw response
// body and returns index-aligned entity/observation pairs. It reuses the exact
// parseBody → filter/CEL/field-mapping pipeline used by the polling and streaming
// paths, so on-demand viewport fetches stay consistent with the source definition.
//
// Unlike processRecords (which dedups entities for time-series snapshots), this keeps
// one entity per observation so the caller can pair them by index. A parse error
// (e.g. malformed body) is returned; per-record mapping failures are skipped.
func (a *DeclarativeAdapter) ParseOnDemand(body []byte) ([]*domain.Entity, []*domain.Observation, error) {
	cs := a.compiled.Load()
	if cs == nil {
		return nil, nil, fmt.Errorf("no compiled source loaded for %q", a.name)
	}

	records, err := a.parseBody(cs.Definition(), body)
	if err != nil {
		return nil, nil, fmt.Errorf("parse on-demand body for %q: %w", a.name, err)
	}

	entities := make([]*domain.Entity, 0, len(records))
	observations := make([]*domain.Observation, 0, len(records))
	loggedErrors := 0
	for _, record := range records {
		entity, obs, ok := a.mapRecord(cs, record, &loggedErrors)
		if !ok {
			continue
		}
		entities = append(entities, entity)
		observations = append(observations, obs)
	}

	return entities, observations, nil
}

// mapEntity evaluates entity CEL programs and builds a domain.Entity.
func (a *DeclarativeAdapter) mapEntity(cs *CompiledSource, activation map[string]interface{}) (*domain.Entity, error) {
	def := cs.Definition()

	externalID, err := evalString(cs.EntityID(), activation, maxEntityIDLen)
	if err != nil {
		return nil, fmt.Errorf("entity.external_id: %w", err)
	}
	if externalID == "" {
		return nil, fmt.Errorf("entity.external_id: empty result")
	}

	name, err := evalString(cs.EntityName(), activation, maxEntityNameLen)
	if err != nil {
		return nil, fmt.Errorf("entity.name: %w", err)
	}

	entityID := domain.EntityID(domain.LayerType(def.LayerType), externalID)

	metadata := make(map[string]string, len(cs.EntityMeta()))
	metaCount := 0
	for k, prg := range cs.EntityMeta() {
		if metaCount >= maxMetadataKeys {
			break
		}
		val, err := evalString(prg, activation, maxMetadataValueLen)
		if err != nil {
			// Skip individual metadata fields on error
			a.logger.Debug("entity metadata eval error",
				logging.String("source_name", a.name),
				logging.String("key", k),
				logging.Err("error", err),
			)
			continue
		}
		metadata[k] = val
		metaCount++
	}

	for _, fm := range cs.FieldMappings() {
		if strings.HasPrefix(fm.Target, "entity.") {
			targetKey := strings.TrimPrefix(fm.Target, "entity.")
			val, err := evalFieldMappingToString(fm.Program, fm.Type, activation, maxMetadataValueLen)
			if err != nil {
				a.logger.Debug("entity field_mapping eval error",
					logging.String("source_name", a.name),
					logging.String("target", fm.Target),
					logging.Err("error", err),
				)
				continue
			}
			metadata[targetKey] = val
		}
	}

	return &domain.Entity{
		ID:         entityID,
		ExternalID: externalID,
		LayerType:  def.LayerType,
		Name:       name,
		Metadata:   metadata,
	}, nil
}

// mapObservation evaluates observation CEL programs and builds a domain.Observation.
func (a *DeclarativeAdapter) mapObservation(cs *CompiledSource, activation map[string]interface{}, entityID string) (*domain.Observation, error) {
	def := cs.Definition()
	isIndicator := def.EntityType == string(domain.EntityTypeIndicator)

	var lat, lon float64
	var err error

	if isIndicator {
		// Global indicator entities have no geographic position — skip lat/lon evaluation.
	} else {
		lat, err = evalFloat(cs.ObservationLat(), activation)
		if err != nil {
			return nil, fmt.Errorf("observation.latitude: %w", err)
		}

		lon, err = evalFloat(cs.ObservationLon(), activation)
		if err != nil {
			return nil, fmt.Errorf("observation.longitude: %w", err)
		}
	}

	var alt float64
	if cs.ObservationAlt() != nil {
		alt, err = evalFloat(cs.ObservationAlt(), activation)
		if err != nil {
			// Altitude is optional; default to 0 on error.
			alt = 0
		}
	}

	ts, err := evalTimestamp(cs.ObservationTS(), activation)
	if err != nil {
		return nil, fmt.Errorf("observation.timestamp: %w", err)
	}

	velocity := make(map[string]float64)
	for k, prg := range cs.ObservationVelocity() {
		val, err := evalFloat(prg, activation)
		if err != nil {
			continue // Skip individual velocity fields on error
		}
		velocity[k] = val
	}

	metadata := make(map[string]string)
	metaCount := 0
	for k, prg := range cs.ObservationMeta() {
		if metaCount >= maxMetadataKeys {
			break
		}
		val, err := evalString(prg, activation, maxMetadataValueLen)
		if err != nil {
			continue
		}
		metadata[k] = val
		metaCount++
	}

	obs := &domain.Observation{
		ID:        uuid.New().String(),
		EntityID:  entityID,
		Timestamp: ts,
		AltitudeM: alt,
		Velocity:  velocity,
		Metadata:  metadata,
	}

	if cs.ObservationEventTime() != nil {
		eventTime, err := evalTimestamp(cs.ObservationEventTime(), activation)
		if err == nil && !eventTime.IsZero() {
			obs.EventTime = &eventTime
		} else if err != nil {
			a.logger.Debug("observation.event_time eval error",
				logging.String("source_name", a.name),
				logging.Err("error", err),
			)
		}
	}

	if cs.ObservationEventEnd() != nil {
		eventEnd, err := evalTimestamp(cs.ObservationEventEnd(), activation)
		if err == nil && !eventEnd.IsZero() {
			obs.EventEnd = &eventEnd
		} else if err != nil {
			a.logger.Debug("observation.event_end eval error",
				logging.String("source_name", a.name),
				logging.Err("error", err),
			)
		}
	}

	for _, fm := range cs.FieldMappings() {
		if strings.HasPrefix(fm.Target, "observation.") {
			targetKey := strings.TrimPrefix(fm.Target, "observation.")

			if targetKey == "event_time" {
				ts, err := evalTimestamp(fm.Program, activation)
				if err == nil && !ts.IsZero() {
					obs.EventTime = &ts
				}
				continue
			}
			if targetKey == "event_end" {
				ts, err := evalTimestamp(fm.Program, activation)
				if err == nil && !ts.IsZero() {
					obs.EventEnd = &ts
				}
				continue
			}

			val, err := evalFieldMappingToString(fm.Program, fm.Type, activation, maxMetadataValueLen)
			if err != nil {
				a.logger.Debug("observation field_mapping eval error",
					logging.String("source_name", a.name),
					logging.String("target", fm.Target),
					logging.Err("error", err),
				)
				continue
			}
			metadata[targetKey] = val
		}
	}

	// Default event_time to observation timestamp if no CEL override produced a value.
	// This ensures every observation has a meaningful event_time for temporal queries.
	if obs.EventTime == nil {
		obs.EventTime = &ts
	}

	if !isIndicator {
		obs.Position = &domain.GeoPoint{
			Lat: lat,
			Lon: lon,
			Alt: alt,
		}
	}

	// Compute content hash if defined
	if cs.ContentHash() != nil {
		hash, err := evalString(cs.ContentHash(), activation, maxMetadataValueLen)
		if err == nil {
			obs.ContentHash = hash
		}
	}

	return obs, nil
}
