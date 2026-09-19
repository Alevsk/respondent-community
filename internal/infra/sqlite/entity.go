package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

type entityRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewEntityRepository(db *sql.DB, logger zerolog.Logger) domain.EntityRepository {
	return &entityRepository{db: db, logger: logger}
}

func (r *entityRepository) Create(ctx context.Context, entity *domain.Entity) error {
	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO entities (id, external_id, layer_type, name, metadata, ai_metadata, source, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		entity.ID, entity.ExternalID, entity.LayerType, entity.Name,
		marshalJSON(entity.Metadata), marshalJSON(entity.AIMetadata),
		entity.Source, formatTime(entity.CreatedAt),
	)
	return translateError("entity", err)
}

func (r *entityRepository) CreateBatch(ctx context.Context, entities []*domain.Entity) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translateError("entity", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO entities (id, external_id, layer_type, name, metadata, ai_metadata, source, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT (layer_type, external_id) DO UPDATE SET
             name = excluded.name,
             metadata = excluded.metadata`)
	if err != nil {
		return translateError("entity", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, e := range entities {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		if _, err := stmt.ExecContext(ctx,
			e.ID, e.ExternalID, e.LayerType, e.Name,
			marshalJSON(e.Metadata), marshalJSON(e.AIMetadata),
			e.Source, formatTime(e.CreatedAt),
		); err != nil {
			return translateError("entity", err)
		}
	}

	return translateError("entity", tx.Commit())
}

func (r *entityRepository) GetByID(ctx context.Context, id string) (*domain.Entity, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, external_id, layer_type, name, metadata, ai_metadata, source, created_at
         FROM entities WHERE id = ?`, id)
	return r.scanEntity(row)
}

func (r *entityRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Entity, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		`SELECT id, external_id, layer_type, name, metadata, ai_metadata, source, created_at
         FROM entities WHERE id IN (%s)`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, translateError("entity", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanEntities(rows)
}

func (r *entityRepository) GetByExternalID(ctx context.Context, layerType, externalID string) (*domain.Entity, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, external_id, layer_type, name, metadata, ai_metadata, source, created_at
         FROM entities WHERE layer_type = ? AND external_id = ?`, layerType, externalID)
	return r.scanEntity(row)
}

// maxINParams caps placeholders in one IN(...) statement, below SQLite's
// SQLITE_MAX_VARIABLE_NUMBER (999 on old builds). Larger id lists must be chunked:
// an un-chunked batch (e.g. power_plants ~35k entities) overflows the limit with
// "too many SQL variables", failing the lookup so the layer's data never persists.
const maxINParams = 900

func (r *entityRepository) GetByExternalIDs(ctx context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	if len(externalIDs) == 0 {
		return nil, nil
	}
	const chunk = maxINParams - 1 // reserve one parameter for layer_type
	result := make([]*domain.Entity, 0, len(externalIDs))
	for start := 0; start < len(externalIDs); start += chunk {
		end := start + chunk
		if end > len(externalIDs) {
			end = len(externalIDs)
		}
		batch := externalIDs[start:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch)+1)
		args[0] = layerType
		for i, eid := range batch {
			placeholders[i] = "?"
			args[i+1] = eid
		}
		query := fmt.Sprintf(
			`SELECT id, external_id, layer_type, name, metadata, ai_metadata, source, created_at
         FROM entities WHERE layer_type = ? AND external_id IN (%s)`,
			strings.Join(placeholders, ","))

		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, translateError("entity", err)
		}
		entities, scanErr := r.scanEntities(rows)
		_ = rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, entities...)
	}
	return result, nil
}

func (r *entityRepository) GetDistinctLayerTypes(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT DISTINCT layer_type FROM entities ORDER BY layer_type")
	if err != nil {
		return nil, translateError("entity", err)
	}
	defer func() { _ = rows.Close() }()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, translateError("entity", err)
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

func (r *entityRepository) CountByLayerType(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT layer_type, COUNT(*) FROM entities GROUP BY layer_type")
	if err != nil {
		return nil, translateError("entity", err)
	}
	defer func() { _ = rows.Close() }()

	counts := make(map[string]int64)
	for rows.Next() {
		var (
			layerType string
			count     int64
		)
		if err := rows.Scan(&layerType, &count); err != nil {
			return nil, translateError("entity", err)
		}
		counts[layerType] = count
	}
	return counts, rows.Err()
}

func (r *entityRepository) Update(ctx context.Context, entity *domain.Entity) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE entities SET external_id = ?, layer_type = ?, name = ?,
         metadata = ?, ai_metadata = ?, source = ? WHERE id = ?`,
		entity.ExternalID, entity.LayerType, entity.Name,
		marshalJSON(entity.Metadata), marshalJSON(entity.AIMetadata),
		entity.Source, entity.ID)
	if err != nil {
		return translateError("entity", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.NewNotFoundError("entity not found", nil)
	}
	return nil
}

func (r *entityRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM entities WHERE id = ?", id)
	if err != nil {
		return translateError("entity", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.NewNotFoundError("entity not found", nil)
	}
	return nil
}

func (r *entityRepository) SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*domain.EntitySearchResult, int, error) {
	if len(query) < 2 {
		return nil, 0, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}

	escaped := escapeLIKE(query)
	wildcardQuery := "%" + escaped + "%"
	prefixQuery := escaped + "%"

	var layerClause string
	var countArgs, searchArgs []any
	if layerType != "" {
		layerClause = "AND layer_type = ?"
		countArgs = []any{wildcardQuery, layerType, wildcardQuery, layerType}
		searchArgs = []any{prefixQuery, wildcardQuery, layerType, wildcardQuery, layerType, limit}
	} else {
		countArgs = []any{wildcardQuery, wildcardQuery}
		searchArgs = []any{prefixQuery, wildcardQuery, wildcardQuery, limit}
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM (
        SELECT id FROM entities WHERE name LIKE ? ESCAPE '\' %s
        UNION
        SELECT id FROM entities WHERE external_id LIKE ? ESCAPE '\' %s
    )`, layerClause, layerClause)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, translateError("entity", err)
	}

	searchQuery := fmt.Sprintf(`
        SELECT e.id, e.external_id, e.layer_type, e.name, e.metadata, e.ai_metadata, e.source, e.created_at,
               o.id, o.entity_id, o.ts, o.event_time, o.event_end, o.lat, o.lon, o.altitude_m,
               o.velocity, o.metadata, o.ai_metadata, o.source_type, o.content_hash, o.source, o.created_at
        FROM (
            SELECT id, name, MIN(rank) AS rank FROM (
                SELECT id, name, CASE WHEN name LIKE ? ESCAPE '\' THEN 0 ELSE 1 END AS rank
                FROM entities WHERE name LIKE ? ESCAPE '\' %s
                UNION ALL
                SELECT id, name, 1 AS rank
                FROM entities WHERE external_id LIKE ? ESCAPE '\' %s
            ) GROUP BY id
        ) AS candidates
        JOIN entities e ON e.id = candidates.id
        LEFT JOIN observations o ON o.entity_id = e.id
            AND o.ts = (SELECT MAX(ts) FROM observations WHERE entity_id = e.id)
        ORDER BY candidates.rank, candidates.name
        LIMIT ?`, layerClause, layerClause)

	rows, err := r.db.QueryContext(ctx, searchQuery, searchArgs...)
	if err != nil {
		return nil, 0, translateError("entity", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*domain.EntitySearchResult
	for rows.Next() {
		var (
			e                                                       domain.Entity
			metadataStr, aiMetadataStr, createdAtStr                string
			oID, oEntityID, oTs, oSourceType, oContentHash, oSource sql.NullString
			oEventTime, oEventEnd                                   sql.NullString
			oLat, oLon                                              sql.NullFloat64
			oAltitude                                               sql.NullFloat64
			oVelocity, oMetadata, oAIMetadata, oCreatedAt           sql.NullString
		)
		if err := rows.Scan(
			&e.ID, &e.ExternalID, &e.LayerType, &e.Name, &metadataStr, &aiMetadataStr, &e.Source, &createdAtStr,
			&oID, &oEntityID, &oTs, &oEventTime, &oEventEnd, &oLat, &oLon, &oAltitude,
			&oVelocity, &oMetadata, &oAIMetadata, &oSourceType, &oContentHash, &oSource, &oCreatedAt,
		); err != nil {
			return nil, 0, translateError("entity", err)
		}
		e.Metadata = unmarshalStringMap(metadataStr)
		e.AIMetadata = unmarshalAnyMap(aiMetadataStr)
		e.CreatedAt, _ = parseTime(createdAtStr)

		result := &domain.EntitySearchResult{Entity: e}
		if oID.Valid {
			obs := buildObservation(oID, oEntityID, oTs, oEventTime, oEventEnd,
				oLat, oLon, oAltitude, oVelocity, oMetadata, oAIMetadata,
				oSourceType, oContentHash, oSource, oCreatedAt)
			result.LatestObservation = obs
		}
		results = append(results, result)
	}

	return results, total, rows.Err()
}

func (r *entityRepository) PatchAIMetadata(ctx context.Context, entityID string, metadata map[string]any) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translateError("entity", err)
	}
	defer func() { _ = tx.Rollback() }()

	var current string
	if err := tx.QueryRowContext(ctx, "SELECT ai_metadata FROM entities WHERE id = ?", entityID).Scan(&current); err != nil {
		return translateError("entity", err)
	}
	merged := mergeJSONMaps(current, metadata)
	if _, err := tx.ExecContext(ctx, "UPDATE entities SET ai_metadata = ? WHERE id = ?", merged, entityID); err != nil {
		return translateError("entity", err)
	}
	return translateError("entity", tx.Commit())
}

func (r *entityRepository) UpdateCoordinates(ctx context.Context, entityID string, lat, lon float64) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE observations SET lat = ?, lon = ?
         WHERE entity_id = ? AND ts = (SELECT MAX(ts) FROM observations WHERE entity_id = ?)`,
		lat, lon, entityID, entityID)
	if err != nil {
		return translateError("observation", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return domain.NewNotFoundError("observation not found", nil)
	}
	return nil
}

func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (r *entityRepository) scanEntity(row *sql.Row) (*domain.Entity, error) {
	var e domain.Entity
	var metadataStr, aiMetadataStr, createdAtStr string
	err := row.Scan(&e.ID, &e.ExternalID, &e.LayerType, &e.Name,
		&metadataStr, &aiMetadataStr, &e.Source, &createdAtStr)
	if err != nil {
		return nil, translateError("entity", err)
	}
	e.Metadata = unmarshalStringMap(metadataStr)
	e.AIMetadata = unmarshalAnyMap(aiMetadataStr)
	e.CreatedAt, _ = parseTime(createdAtStr)
	return &e, nil
}

func (r *entityRepository) scanEntities(rows *sql.Rows) ([]*domain.Entity, error) {
	var entities []*domain.Entity
	for rows.Next() {
		var e domain.Entity
		var metadataStr, aiMetadataStr, createdAtStr string
		if err := rows.Scan(&e.ID, &e.ExternalID, &e.LayerType, &e.Name,
			&metadataStr, &aiMetadataStr, &e.Source, &createdAtStr); err != nil {
			return nil, translateError("entity", err)
		}
		e.Metadata = unmarshalStringMap(metadataStr)
		e.AIMetadata = unmarshalAnyMap(aiMetadataStr)
		e.CreatedAt, _ = parseTime(createdAtStr)
		entities = append(entities, &e)
	}
	return entities, rows.Err()
}

func buildObservation(
	oID, oEntityID, oTs, oEventTime, oEventEnd sql.NullString,
	oLat, oLon, oAltitude sql.NullFloat64,
	oVelocity, oMetadata, oAIMetadata sql.NullString,
	oSourceType, oContentHash, oSource, oCreatedAt sql.NullString,
) *domain.Observation {
	obs := &domain.Observation{
		ID:          oID.String,
		EntityID:    oEntityID.String,
		SourceType:  oSourceType.String,
		ContentHash: oContentHash.String,
		Source:      oSource.String,
	}
	if oTs.Valid {
		obs.Timestamp, _ = parseTime(oTs.String)
	}
	obs.EventTime = timePtr(oEventTime)
	obs.EventEnd = timePtr(oEventEnd)
	if oLat.Valid && oLon.Valid {
		obs.Position = &domain.GeoPoint{Lat: oLat.Float64, Lon: oLon.Float64}
	}
	if oAltitude.Valid {
		obs.AltitudeM = oAltitude.Float64
	}
	obs.Velocity = unmarshalFloat64Map(oVelocity.String)
	obs.Metadata = unmarshalStringMap(oMetadata.String)
	obs.AIMetadata = unmarshalAnyMap(oAIMetadata.String)
	if oCreatedAt.Valid {
		obs.CreatedAt, _ = parseTime(oCreatedAt.String)
	}
	return obs
}
