package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

type observationRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewObservationRepository creates a new SQLite-backed ObservationRepository.
func NewObservationRepository(db *sql.DB, logger zerolog.Logger) domain.ObservationRepository {
	return &observationRepository{db: db, logger: logger}
}

func (r *observationRepository) Create(ctx context.Context, obs *domain.Observation) error {
	if obs.ID == "" {
		obs.ID = uuid.New().String()
	}
	var lat, lon sql.NullFloat64
	if obs.Position != nil {
		lat = sql.NullFloat64{Float64: obs.Position.Lat, Valid: true}
		lon = sql.NullFloat64{Float64: obs.Position.Lon, Valid: true}
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO observations (id, entity_id, ts, event_time, event_end, lat, lon, altitude_m,
         velocity, metadata, ai_metadata, source_type, content_hash, source, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		obs.ID, obs.EntityID, formatTime(obs.Timestamp),
		nullTime(obs.EventTime), nullTime(obs.EventEnd),
		lat, lon, obs.AltitudeM,
		marshalJSON(obs.Velocity), marshalJSON(obs.Metadata), marshalJSON(obs.AIMetadata),
		obs.SourceType, obs.ContentHash, obs.Source, formatTime(obs.CreatedAt),
	)
	return translateError("observation", err)
}

func (r *observationRepository) CreateBatch(ctx context.Context, observations []*domain.Observation) error {
	if len(observations) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translateError("observation", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO observations (id, entity_id, ts, event_time, event_end, lat, lon, altitude_m,
         velocity, metadata, ai_metadata, source_type, content_hash, source, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT (entity_id, ts) DO NOTHING`)
	if err != nil {
		return translateError("observation", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, obs := range observations {
		if obs.ID == "" {
			obs.ID = uuid.New().String()
		}
		var lat, lon sql.NullFloat64
		if obs.Position != nil {
			lat = sql.NullFloat64{Float64: obs.Position.Lat, Valid: true}
			lon = sql.NullFloat64{Float64: obs.Position.Lon, Valid: true}
		}
		if _, err := stmt.ExecContext(ctx,
			obs.ID, obs.EntityID, formatTime(obs.Timestamp),
			nullTime(obs.EventTime), nullTime(obs.EventEnd),
			lat, lon, obs.AltitudeM,
			marshalJSON(obs.Velocity), marshalJSON(obs.Metadata), marshalJSON(obs.AIMetadata),
			obs.SourceType, obs.ContentHash, obs.Source, formatTime(obs.CreatedAt),
		); err != nil {
			return translateError("observation", err)
		}
	}
	return translateError("observation", tx.Commit())
}

func (r *observationRepository) CreateBatchUpsert(ctx context.Context, observations []*domain.Observation) error {
	if len(observations) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translateError("observation", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO observations (id, entity_id, ts, event_time, event_end, lat, lon, altitude_m,
         velocity, metadata, ai_metadata, source_type, content_hash, source, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT (entity_id, ts) DO UPDATE SET
             event_time = excluded.event_time,
             event_end = excluded.event_end,
             lat = excluded.lat,
             lon = excluded.lon,
             altitude_m = excluded.altitude_m,
             velocity = excluded.velocity,
             metadata = excluded.metadata,
             ai_metadata = excluded.ai_metadata,
             source_type = excluded.source_type,
             content_hash = excluded.content_hash,
             source = excluded.source`)
	if err != nil {
		return translateError("observation", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, obs := range observations {
		if obs.ID == "" {
			obs.ID = uuid.New().String()
		}
		var lat, lon sql.NullFloat64
		if obs.Position != nil {
			lat = sql.NullFloat64{Float64: obs.Position.Lat, Valid: true}
			lon = sql.NullFloat64{Float64: obs.Position.Lon, Valid: true}
		}
		if _, err := stmt.ExecContext(ctx,
			obs.ID, obs.EntityID, formatTime(obs.Timestamp),
			nullTime(obs.EventTime), nullTime(obs.EventEnd),
			lat, lon, obs.AltitudeM,
			marshalJSON(obs.Velocity), marshalJSON(obs.Metadata), marshalJSON(obs.AIMetadata),
			obs.SourceType, obs.ContentHash, obs.Source, formatTime(obs.CreatedAt),
		); err != nil {
			return translateError("observation", err)
		}
	}
	return translateError("observation", tx.Commit())
}

func (r *observationRepository) GetByEntityID(ctx context.Context, entityID string, limit int, before time.Time) ([]*domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, entity_id, ts, event_time, event_end, lat, lon, altitude_m,
         velocity, metadata, ai_metadata, source_type, content_hash, source, created_at
         FROM observations
         WHERE entity_id = ? AND ts < ?
         ORDER BY ts DESC
         LIMIT ?`, entityID, formatTime(before), limit)
	if err != nil {
		return nil, translateError("observation", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanObservations(rows)
}

func (r *observationRepository) GetLatest(ctx context.Context, entityID string) (*domain.Observation, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, entity_id, ts, event_time, event_end, lat, lon, altitude_m,
         velocity, metadata, ai_metadata, source_type, content_hash, source, created_at
         FROM observations
         WHERE entity_id = ?
         ORDER BY ts DESC
         LIMIT 1`, entityID)
	return r.scanObservation(row)
}

func (r *observationRepository) GetLatestForLayer(ctx context.Context, layerType string, limit int) ([]*domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
         o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
         FROM observations o
         JOIN entities e ON e.id = o.entity_id
         WHERE e.layer_type = ?
           AND o.ts = (SELECT MAX(o2.ts) FROM observations o2 WHERE o2.entity_id = o.entity_id)
         ORDER BY o.ts DESC
         LIMIT ?`, layerType, limit)
	if err != nil {
		return nil, translateError("observation", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanObservations(rows)
}

func (r *observationRepository) GetLatestForEntityIDs(ctx context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	// Chunk the IN(...) list to stay under SQLite's variable limit (see maxINParams).
	result := make(map[string]*domain.Observation, len(entityIDs))
	for start := 0; start < len(entityIDs); start += maxINParams {
		end := start + maxINParams
		if end > len(entityIDs) {
			end = len(entityIDs)
		}
		batch := entityIDs[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf(
			`SELECT o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
         o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
         FROM observations o
         WHERE o.entity_id IN (%s)
           AND o.ts = (SELECT MAX(o2.ts) FROM observations o2 WHERE o2.entity_id = o.entity_id)`,
			strings.Join(placeholders, ","))

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, translateError("observation", err)
		}
		for rows.Next() {
			obs, scanErr := r.scanObservationFromRows(rows)
			if scanErr != nil {
				_ = rows.Close()
				return nil, scanErr
			}
			result[obs.EntityID] = obs
		}
		cerr := rows.Err()
		_ = rows.Close()
		if cerr != nil {
			return nil, cerr
		}
	}
	return result, nil
}

func (r *observationRepository) GetLatestContentHashes(ctx context.Context, entityIDs []string) (map[string]string, error) {
	// Chunk the IN(...) list to stay under SQLite's variable limit (see maxINParams).
	result := make(map[string]string, len(entityIDs))
	for start := 0; start < len(entityIDs); start += maxINParams {
		end := start + maxINParams
		if end > len(entityIDs) {
			end = len(entityIDs)
		}
		batch := entityIDs[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf(
			`SELECT o.entity_id, o.content_hash
         FROM observations o
         WHERE o.entity_id IN (%s)
           AND o.ts = (SELECT MAX(o2.ts) FROM observations o2 WHERE o2.entity_id = o.entity_id)`,
			strings.Join(placeholders, ","))

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, translateError("observation", err)
		}
		for rows.Next() {
			var entityID, hash string
			if scanErr := rows.Scan(&entityID, &hash); scanErr != nil {
				_ = rows.Close()
				return nil, translateError("observation", scanErr)
			}
			result[entityID] = hash
		}
		cerr := rows.Err()
		_ = rows.Close()
		if cerr != nil {
			return nil, cerr
		}
	}
	return result, nil
}

func (r *observationRepository) GetLayerSnapshotAt(ctx context.Context, layerType string, asOf time.Time, window time.Duration) ([]*domain.EntitySnapshot, error) {
	from := asOf.Add(-window)
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.external_id, e.layer_type, e.name, e.metadata, e.ai_metadata, e.source, e.created_at,
                o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
                o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
         FROM entities e
         JOIN observations o ON o.entity_id = e.id
         WHERE e.layer_type = ?
           AND o.ts BETWEEN ? AND ?
           AND o.ts = (
               SELECT MAX(o2.ts) FROM observations o2
               WHERE o2.entity_id = e.id AND o2.ts BETWEEN ? AND ?
           )`,
		layerType, formatTime(from), formatTime(asOf), formatTime(from), formatTime(asOf))
	if err != nil {
		return nil, translateError("observation", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanSnapshots(rows)
}

func (r *observationRepository) GetLatestForLayerByBBox(
	ctx context.Context, layerType string,
	south, north, west, east float64,
	from, to time.Time, limit int,
) ([]*domain.EntitySnapshot, error) {
	return r.bboxQuery(ctx, layerType, south, north, west, east, from, to, limit, false)
}

func (r *observationRepository) GetLatestByCurrentPositionInBBox(
	ctx context.Context, layerType string,
	south, north, west, east float64,
	from, to time.Time, limit int,
) ([]*domain.EntitySnapshot, error) {
	return r.bboxQuery(ctx, layerType, south, north, west, east, from, to, limit, true)
}

// bboxQuery executes a bounding-box query with optional current-position filtering.
// When currentPosition is true, it finds each entity's latest observation in [from,to]
// then filters by whether that position falls in the bbox.
// When false, it finds observations within the bbox and time window directly.
// Antimeridian wrapping is handled when west > east.
func (r *observationRepository) bboxQuery(
	ctx context.Context, layerType string,
	south, north, west, east float64,
	from, to time.Time, limit int,
	currentPosition bool,
) ([]*domain.EntitySnapshot, error) {
	var lonCondition string
	if west <= east {
		lonCondition = "o.lon BETWEEN ? AND ?"
	} else {
		// Antimeridian wrap: e.g., west=170, east=-170 crosses the 180° line.
		lonCondition = "(o.lon >= ? OR o.lon <= ?)"
	}

	fromStr := formatTime(from)
	toStr := formatTime(to)

	var query string
	var args []any

	if currentPosition {
		query = fmt.Sprintf(`
            SELECT e.id, e.external_id, e.layer_type, e.name, e.metadata, e.ai_metadata, e.source, e.created_at,
                   o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
                   o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
            FROM entities e
            JOIN observations o ON o.entity_id = e.id
            WHERE e.layer_type = ?
              AND o.ts BETWEEN ? AND ?
              AND o.ts = (
                  SELECT MAX(o2.ts) FROM observations o2
                  WHERE o2.entity_id = e.id AND o2.ts BETWEEN ? AND ?
              )
              AND o.lat BETWEEN ? AND ?
              AND %s
            LIMIT ?`, lonCondition)
		args = []any{layerType, fromStr, toStr, fromStr, toStr, south, north, west, east, limit}
	} else {
		query = fmt.Sprintf(`
            SELECT e.id, e.external_id, e.layer_type, e.name, e.metadata, e.ai_metadata, e.source, e.created_at,
                   o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
                   o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
            FROM entities e
            JOIN observations o ON o.entity_id = e.id
            WHERE e.layer_type = ?
              AND o.ts BETWEEN ? AND ?
              AND o.lat BETWEEN ? AND ?
              AND %s
              AND o.ts = (
                  SELECT MAX(o2.ts) FROM observations o2
                  WHERE o2.entity_id = e.id
                    AND o2.ts BETWEEN ? AND ?
                    AND o2.lat BETWEEN ? AND ?
                    AND %s
              )
            LIMIT ?`, lonCondition, lonCondition)
		args = []any{
			layerType, fromStr, toStr, south, north, west, east,
			fromStr, toStr, south, north, west, east,
			limit,
		}
	}

	queryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, translateError("observation", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanSnapshots(rows)
}

func (r *observationRepository) PatchAIMetadata(ctx context.Context, observationID string, metadata map[string]any) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translateError("observation", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	if err := tx.QueryRowContext(ctx, "SELECT ai_metadata FROM observations WHERE id = ?", observationID).Scan(&current); err != nil {
		return translateError("observation", err)
	}
	merged := mergeJSONMaps(current, metadata)
	if _, err := tx.ExecContext(ctx, "UPDATE observations SET ai_metadata = ? WHERE id = ?", merged, observationID); err != nil {
		return translateError("observation", err)
	}
	return translateError("observation", tx.Commit())
}

// scanObservation scans a single observation from a *sql.Row.
func (r *observationRepository) scanObservation(row *sql.Row) (*domain.Observation, error) {
	var (
		obs                                     domain.Observation
		tsStr, createdAtStr                     string
		eventTime, eventEnd                     sql.NullString
		lat, lon, altitude                      sql.NullFloat64
		velocityStr, metadataStr, aiMetadataStr string
	)
	err := row.Scan(
		&obs.ID, &obs.EntityID, &tsStr, &eventTime, &eventEnd,
		&lat, &lon, &altitude,
		&velocityStr, &metadataStr, &aiMetadataStr,
		&obs.SourceType, &obs.ContentHash, &obs.Source, &createdAtStr,
	)
	if err != nil {
		return nil, translateError("observation", err)
	}
	obs.Timestamp, _ = parseTime(tsStr)
	obs.CreatedAt, _ = parseTime(createdAtStr)
	obs.EventTime = timePtr(eventTime)
	obs.EventEnd = timePtr(eventEnd)
	if lat.Valid && lon.Valid {
		obs.Position = &domain.GeoPoint{Lat: lat.Float64, Lon: lon.Float64}
		if altitude.Valid {
			obs.Position.Alt = altitude.Float64
		}
	}
	if altitude.Valid {
		obs.AltitudeM = altitude.Float64
	}
	obs.Velocity = unmarshalFloat64Map(velocityStr)
	obs.Metadata = unmarshalStringMap(metadataStr)
	obs.AIMetadata = unmarshalAnyMap(aiMetadataStr)
	return &obs, nil
}

// scanObservationFromRows scans a single observation from a *sql.Rows cursor.
func (r *observationRepository) scanObservationFromRows(rows *sql.Rows) (*domain.Observation, error) {
	var (
		obs                                     domain.Observation
		tsStr, createdAtStr                     string
		eventTime, eventEnd                     sql.NullString
		lat, lon, altitude                      sql.NullFloat64
		velocityStr, metadataStr, aiMetadataStr string
	)
	err := rows.Scan(
		&obs.ID, &obs.EntityID, &tsStr, &eventTime, &eventEnd,
		&lat, &lon, &altitude,
		&velocityStr, &metadataStr, &aiMetadataStr,
		&obs.SourceType, &obs.ContentHash, &obs.Source, &createdAtStr,
	)
	if err != nil {
		return nil, translateError("observation", err)
	}
	obs.Timestamp, _ = parseTime(tsStr)
	obs.CreatedAt, _ = parseTime(createdAtStr)
	obs.EventTime = timePtr(eventTime)
	obs.EventEnd = timePtr(eventEnd)
	if lat.Valid && lon.Valid {
		obs.Position = &domain.GeoPoint{Lat: lat.Float64, Lon: lon.Float64}
		if altitude.Valid {
			obs.Position.Alt = altitude.Float64
		}
	}
	if altitude.Valid {
		obs.AltitudeM = altitude.Float64
	}
	obs.Velocity = unmarshalFloat64Map(velocityStr)
	obs.Metadata = unmarshalStringMap(metadataStr)
	obs.AIMetadata = unmarshalAnyMap(aiMetadataStr)
	return &obs, nil
}

// scanObservations scans multiple observations from rows.
func (r *observationRepository) scanObservations(rows *sql.Rows) ([]*domain.Observation, error) {
	var observations []*domain.Observation
	for rows.Next() {
		obs, err := r.scanObservationFromRows(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obs)
	}
	return observations, rows.Err()
}

// scanSnapshots scans entity+observation pairs from rows.
func (r *observationRepository) scanSnapshots(rows *sql.Rows) ([]*domain.EntitySnapshot, error) {
	var snapshots []*domain.EntitySnapshot
	for rows.Next() {
		var (
			e                                  domain.Entity
			eMetadata, eAIMetadata, eCreatedAt string
			oID, oEntityID, oTs                string
			oEventTime, oEventEnd              sql.NullString
			oLat, oLon, oAltitude              sql.NullFloat64
			oVelocity, oMetadata, oAIMetadata  string
			oSourceType, oContentHash, oSource string
			oCreatedAt                         string
		)
		if err := rows.Scan(
			&e.ID, &e.ExternalID, &e.LayerType, &e.Name, &eMetadata, &eAIMetadata, &e.Source, &eCreatedAt,
			&oID, &oEntityID, &oTs, &oEventTime, &oEventEnd, &oLat, &oLon, &oAltitude,
			&oVelocity, &oMetadata, &oAIMetadata, &oSourceType, &oContentHash, &oSource, &oCreatedAt,
		); err != nil {
			return nil, translateError("observation", err)
		}

		e.Metadata = unmarshalStringMap(eMetadata)
		e.AIMetadata = unmarshalAnyMap(eAIMetadata)
		e.CreatedAt, _ = parseTime(eCreatedAt)

		obs := domain.Observation{
			ID:          oID,
			EntityID:    oEntityID,
			SourceType:  oSourceType,
			ContentHash: oContentHash,
			Source:      oSource,
		}
		obs.Timestamp, _ = parseTime(oTs)
		obs.CreatedAt, _ = parseTime(oCreatedAt)
		obs.EventTime = timePtr(oEventTime)
		obs.EventEnd = timePtr(oEventEnd)
		if oLat.Valid && oLon.Valid {
			obs.Position = &domain.GeoPoint{Lat: oLat.Float64, Lon: oLon.Float64}
			if oAltitude.Valid {
				obs.Position.Alt = oAltitude.Float64
			}
		}
		if oAltitude.Valid {
			obs.AltitudeM = oAltitude.Float64
		}
		obs.Velocity = unmarshalFloat64Map(oVelocity)
		obs.Metadata = unmarshalStringMap(oMetadata)
		obs.AIMetadata = unmarshalAnyMap(oAIMetadata)

		snapshots = append(snapshots, &domain.EntitySnapshot{Entity: e, Observation: obs})
	}
	return snapshots, rows.Err()
}
