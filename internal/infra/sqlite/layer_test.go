package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// insertLayer inserts a layer directly using SQL so tests are not coupled to a Create method.
func insertLayer(t *testing.T, db *sqlitedb.DB, layer *domain.Layer) {
	t.Helper()
	_, err := db.SqlDB().ExecContext(context.Background(),
		`INSERT INTO layers (id, name, type, enabled, mode, density, source, last_update, count,
		                     config, color, point_size, rendering_mode, filtering_mode)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		layer.ID, layer.Name, layer.Type, 1, layer.Mode,
		layer.Density, layer.Source, layer.LastUpdate.UTC().Format(time.RFC3339Nano), layer.Count,
		"{}", layer.Color, layer.PointSize, layer.RenderingMode, layer.FilteringMode,
	)
	require.NoError(t, err)
}

func makeLayer(layerType, name string) *domain.Layer {
	return &domain.Layer{
		ID:            uuid.New().String(),
		Name:          name,
		Type:          layerType,
		Enabled:       true,
		Mode:          "sparse",
		Density:       0,
		Source:        "test",
		LastUpdate:    time.Now().UTC().Truncate(time.Millisecond),
		Count:         0,
		Config:        map[string]any{},
		Color:         "#00ff00",
		PointSize:     2,
		RenderingMode: "map",
		FilteringMode: "",
	}
}

func TestLayerRepository_List(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	l1 := makeLayer("flights", "Flights")
	l2 := makeLayer("satellites", "Satellites")
	insertLayer(t, db, l1)
	insertLayer(t, db, l2)

	layers, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, layers, 2)
	// Ordered by name.
	assert.Equal(t, "Flights", layers[0].Name)
	assert.Equal(t, "Satellites", layers[1].Name)
}

func TestLayerRepository_GetByID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	l := makeLayer("flights", "Flights")
	insertLayer(t, db, l)

	got, err := repo.GetByID(context.Background(), l.ID)
	require.NoError(t, err)
	assert.Equal(t, l.ID, got.ID)
	assert.Equal(t, "flights", got.Type)
	assert.Equal(t, "Flights", got.Name)
	assert.True(t, got.Enabled)
	assert.Equal(t, "#00ff00", got.Color)
}

func TestLayerRepository_GetByID_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	_, err := repo.GetByID(context.Background(), uuid.New().String())
	assert.True(t, domain.IsNotFound(err))
}

func TestLayerRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	l := makeLayer("flights", "Flights")
	insertLayer(t, db, l)

	l.Name = "Updated Flights"
	l.Color = "#ff0000"
	l.Count = 42
	l.Config = map[string]any{"key": "value"}
	require.NoError(t, repo.Update(context.Background(), l))

	got, err := repo.GetByID(context.Background(), l.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Flights", got.Name)
	assert.Equal(t, "#ff0000", got.Color)
	assert.Equal(t, int64(42), got.Count)
	assert.Equal(t, "value", got.Config["key"])
}

func TestLayerRepository_Update_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	l := makeLayer("flights", "Flights")
	err := repo.Update(context.Background(), l)
	assert.True(t, domain.IsNotFound(err))
}

func TestLayerRepository_Update_WithDisplayConfig(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewLayerRepository(db.SqlDB(), testLogger())

	l := makeLayer("flights", "Flights")
	insertLayer(t, db, l)

	l.DisplayConfig = &domain.LayerDisplayConfig{
		Icon: &domain.IconConfig{Shape: "airplane", Rotatable: true, Scale: 1.5},
	}
	require.NoError(t, repo.Update(context.Background(), l))

	got, err := repo.GetByID(context.Background(), l.ID)
	require.NoError(t, err)
	require.NotNil(t, got.DisplayConfig)
	require.NotNil(t, got.DisplayConfig.Icon)
	assert.Equal(t, "airplane", got.DisplayConfig.Icon.Shape)
	assert.True(t, got.DisplayConfig.Icon.Rotatable)
	assert.InDelta(t, 1.5, got.DisplayConfig.Icon.Scale, 0.001)
}
