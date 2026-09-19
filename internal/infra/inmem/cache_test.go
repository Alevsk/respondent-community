package inmem_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inmem"
)

func makeTestEntity(layerType, externalID string) *domain.Entity {
	return &domain.Entity{
		ID:         uuid.New().String(),
		ExternalID: externalID,
		LayerType:  layerType,
		Name:       externalID,
		Metadata:   map[string]string{},
		AIMetadata: map[string]any{},
		Source:     "test",
		CreatedAt:  time.Now().UTC(),
	}
}

func makeTestObservation(entityID string, lat, lon float64) *domain.Observation {
	return &domain.Observation{
		ID:       uuid.New().String(),
		EntityID: entityID,
		Position: &domain.GeoPoint{Lat: lat, Lon: lon},
	}
}

func TestMemCache_SetAndGetEntity(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	entity := makeTestEntity("flights", "ABC")
	obs := makeTestObservation(entity.ID, 40.7, -74.0)

	require.NoError(t, c.SetEntity(context.Background(), entity, obs))

	gotE, gotO, err := c.GetEntity(context.Background(), "flights", "ABC")
	require.NoError(t, err)
	assert.Equal(t, entity.ID, gotE.ID)
	assert.InDelta(t, 40.7, gotO.Position.Lat, 0.0001)
}

func TestMemCache_GetEntity_NotFound(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	_, _, err := c.GetEntity(context.Background(), "flights", "MISSING")
	assert.True(t, domain.IsNotFound(err))
}

func TestMemCache_GetEntity_Expired(t *testing.T) {
	c := inmem.NewMemCache(50*time.Millisecond, time.Minute)
	defer func() { _ = c.Close() }()

	entity := makeTestEntity("flights", "X1")
	obs := makeTestObservation(entity.ID, 0, 0)
	require.NoError(t, c.SetEntity(context.Background(), entity, obs))

	time.Sleep(100 * time.Millisecond)

	_, _, err := c.GetEntity(context.Background(), "flights", "X1")
	assert.True(t, domain.IsNotFound(err))
}

func TestMemCache_GetLayerCount(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	for i := 0; i < 5; i++ {
		e := makeTestEntity("flights", uuid.New().String())
		require.NoError(t, c.SetEntity(context.Background(), e, makeTestObservation(e.ID, float64(i), 0)))
	}

	count, err := c.GetLayerCount(context.Background(), "flights")
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestMemCache_GetLayerEntities_Pagination(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	for i := 0; i < 10; i++ {
		e := makeTestEntity("flights", uuid.New().String())
		require.NoError(t, c.SetEntity(context.Background(), e, makeTestObservation(e.ID, float64(i), 0)))
	}

	entities, _, total, err := c.GetLayerEntities(context.Background(), "flights", 3, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, entities, 3)
}

func TestMemCache_ClearLayer(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	e := makeTestEntity("flights", "F1")
	require.NoError(t, c.SetEntity(context.Background(), e, makeTestObservation(e.ID, 0, 0)))

	require.NoError(t, c.ClearLayer(context.Background(), "flights"))

	count, err := c.GetLayerCount(context.Background(), "flights")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestMemCache_GetStats(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	e1 := makeTestEntity("flights", "F1")
	e2 := makeTestEntity("satellites", "S1")
	require.NoError(t, c.SetEntity(context.Background(), e1, makeTestObservation(e1.ID, 0, 0)))
	require.NoError(t, c.SetEntity(context.Background(), e2, makeTestObservation(e2.ID, 0, 0)))

	stats, err := c.GetStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, stats["total_entries"])
}

func TestMemCache_HealthCheck(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()
	assert.NoError(t, c.HealthCheck(context.Background()))
}

func TestMemCache_PerLayerTTL(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute) // default: no expiry
	defer func() { _ = c.Close() }()

	c.SetLayerTTL("flights", 50*time.Millisecond) // flights expire fast

	e1 := makeTestEntity("flights", "F1")
	e2 := makeTestEntity("satellites", "S1")
	require.NoError(t, c.SetEntity(context.Background(), e1, makeTestObservation(e1.ID, 0, 0)))
	require.NoError(t, c.SetEntity(context.Background(), e2, makeTestObservation(e2.ID, 0, 0)))

	time.Sleep(100 * time.Millisecond)

	// flights entry should have expired.
	_, _, err := c.GetEntity(context.Background(), "flights", "F1")
	assert.True(t, domain.IsNotFound(err))

	// satellites entry should still be alive (no TTL).
	_, _, err = c.GetEntity(context.Background(), "satellites", "S1")
	require.NoError(t, err)
}

func TestMemCache_GetLayerEntitiesByBBox(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	// NYC
	e1 := makeTestEntity("flights", "NYC")
	o1 := makeTestObservation(e1.ID, 40.7, -74.0)
	// London
	e2 := makeTestEntity("flights", "LON")
	o2 := makeTestObservation(e2.ID, 51.5, -0.12)

	require.NoError(t, c.SetEntity(context.Background(), e1, o1))
	require.NoError(t, c.SetEntity(context.Background(), e2, o2))

	bbox := domain.BBox{South: 39.0, North: 42.0, West: -76.0, East: -72.0}
	entities, obs, err := c.GetLayerEntitiesByBBox(context.Background(), "flights", bbox, 10)
	require.NoError(t, err)
	assert.Len(t, entities, 1)
	assert.Equal(t, "NYC", entities[0].ExternalID)
	assert.Len(t, obs, 1)
}

func TestMemCache_GetLayerEntitiesByBBox_Antimeridian(t *testing.T) {
	c := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = c.Close() }()

	// Pacific entity at lon=175 (east of dateline)
	e := makeTestEntity("flights", "PAC")
	o := makeTestObservation(e.ID, 35.0, 175.0)
	require.NoError(t, c.SetEntity(context.Background(), e, o))

	// Antimeridian bbox: west=170, east=-170 (crosses dateline)
	bbox := domain.BBox{South: 30.0, North: 40.0, West: 170.0, East: -170.0}
	entities, _, err := c.GetLayerEntitiesByBBox(context.Background(), "flights", bbox, 10)
	require.NoError(t, err)
	assert.Len(t, entities, 1)
}
