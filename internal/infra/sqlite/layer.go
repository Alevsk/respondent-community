package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

type layerRepository struct {
	db     *sql.DB
	logger zerolog.Logger
}

// NewLayerRepository creates a new SQLite-backed LayerRepository.
func NewLayerRepository(db *sql.DB, logger zerolog.Logger) domain.LayerRepository {
	return &layerRepository{db: db, logger: logger}
}

func (r *layerRepository) List(ctx context.Context) ([]*domain.Layer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, enabled, mode, density, source, last_update, count,
		        config, color, point_size, rendering_mode, filtering_mode, display_config, history_config
		 FROM layers ORDER BY name`)
	if err != nil {
		return nil, translateError("layer", err)
	}
	defer func() { _ = rows.Close() }()
	return r.scanLayers(rows)
}

func (r *layerRepository) GetByID(ctx context.Context, id string) (*domain.Layer, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, enabled, mode, density, source, last_update, count,
		        config, color, point_size, rendering_mode, filtering_mode, display_config, history_config
		 FROM layers WHERE id = ?`, id)
	return r.scanLayer(row)
}

func (r *layerRepository) Update(ctx context.Context, layer *domain.Layer) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE layers
		 SET name = ?, type = ?, enabled = ?, mode = ?, density = ?, source = ?,
		     last_update = ?, count = ?, config = ?, color = ?, point_size = ?,
		     rendering_mode = ?, filtering_mode = ?, display_config = ?, history_config = ?
		 WHERE id = ?`,
		layer.Name, layer.Type, boolToInt(layer.Enabled), layer.Mode,
		layer.Density, layer.Source, formatTime(layer.LastUpdate), layer.Count,
		marshalJSON(layer.Config), layer.Color, layer.PointSize,
		layer.RenderingMode, layer.FilteringMode,
		marshalOptionalJSON(layer.DisplayConfig),
		marshalOptionalJSON(layer.HistoryConfig),
		layer.ID,
	)
	if err != nil {
		return translateError("layer", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.NewNotFoundError("layer not found", nil)
	}
	return nil
}

// boolToInt converts bool to SQLite integer (0/1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// marshalOptionalJSON marshals v to JSON, returning NULL-able sql.NullString.
// Handles typed nil pointers (e.g., (*LayerDisplayConfig)(nil)) correctly.
func marshalOptionalJSON(v any) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	// reflect check handles typed nil pointers passed as interface{}.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return sql.NullString{}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

func (r *layerRepository) scanLayer(row *sql.Row) (*domain.Layer, error) {
	var (
		layer                              domain.Layer
		enabled                            int
		lastUpdateStr                      string
		configStr                          string
		displayConfigStr, historyConfigStr sql.NullString
	)
	err := row.Scan(
		&layer.ID, &layer.Name, &layer.Type, &enabled, &layer.Mode,
		&layer.Density, &layer.Source, &lastUpdateStr, &layer.Count,
		&configStr, &layer.Color, &layer.PointSize,
		&layer.RenderingMode, &layer.FilteringMode,
		&displayConfigStr, &historyConfigStr,
	)
	if err != nil {
		return nil, translateError("layer", err)
	}
	layer.Enabled = enabled != 0
	layer.LastUpdate, _ = parseTime(lastUpdateStr)
	layer.Config = unmarshalAnyMap(configStr)

	if displayConfigStr.Valid && displayConfigStr.String != "" {
		var dc domain.LayerDisplayConfig
		if err := json.Unmarshal([]byte(displayConfigStr.String), &dc); err == nil {
			layer.DisplayConfig = &dc
		}
	}
	if historyConfigStr.Valid && historyConfigStr.String != "" {
		var hc domain.HistoryConfig
		if err := json.Unmarshal([]byte(historyConfigStr.String), &hc); err == nil {
			layer.HistoryConfig = &hc
		}
	}
	return &layer, nil
}

func (r *layerRepository) scanLayers(rows *sql.Rows) ([]*domain.Layer, error) {
	var layers []*domain.Layer
	for rows.Next() {
		var (
			layer                              domain.Layer
			enabled                            int
			lastUpdateStr                      string
			configStr                          string
			displayConfigStr, historyConfigStr sql.NullString
		)
		err := rows.Scan(
			&layer.ID, &layer.Name, &layer.Type, &enabled, &layer.Mode,
			&layer.Density, &layer.Source, &lastUpdateStr, &layer.Count,
			&configStr, &layer.Color, &layer.PointSize,
			&layer.RenderingMode, &layer.FilteringMode,
			&displayConfigStr, &historyConfigStr,
		)
		if err != nil {
			return nil, translateError("layer", err)
		}
		layer.Enabled = enabled != 0
		layer.LastUpdate, _ = parseTime(lastUpdateStr)
		layer.Config = unmarshalAnyMap(configStr)

		if displayConfigStr.Valid && displayConfigStr.String != "" {
			var dc domain.LayerDisplayConfig
			if err := json.Unmarshal([]byte(displayConfigStr.String), &dc); err == nil {
				layer.DisplayConfig = &dc
			}
		}
		if historyConfigStr.Valid && historyConfigStr.String != "" {
			var hc domain.HistoryConfig
			if err := json.Unmarshal([]byte(historyConfigStr.String), &hc); err == nil {
				layer.HistoryConfig = &hc
			}
		}
		layers = append(layers, &layer)
	}
	return layers, rows.Err()
}
