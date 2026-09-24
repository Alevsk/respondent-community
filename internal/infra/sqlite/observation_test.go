package sqlite_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

func makeObservation(entityID string, ts time.Time, lat, lon float64) *domain.Observation {
	return &domain.Observation{
		ID:          uuid.New().String(),
		EntityID:    entityID,
		Timestamp:   ts,
		Position:    &domain.GeoPoint{Lat: lat, Lon: lon},
		AltitudeM:   10000,
		Velocity:    map[string]float64{"speed": 500, "heading": 90},
		Metadata:    map[string]string{"callsign": "TEST"},
		AIMetadata:  map[string]any{},
		SourceType:  "adsb",
		ContentHash: "hash123",
		Source:      "test",
		CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
}

// setupEntityAndRepo creates entity + observation repos and inserts a test entity.
func setupEntityAndRepo(t *testing.T) (*sqlitedb.DB, domain.EntityRepository, domain.ObservationRepository, *domain.Entity) {
	t.Helper()
	db := newTestDB(t)
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	entity := makeEntity("flights", "ABC123", "Flight ABC123")
	require.NoError(t, entityRepo.Create(context.Background(), entity))
	return db, entityRepo, obsRepo, entity
}

func TestObservationRepository_CreateAndGetLatest(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	obs := makeObservation(entity.ID, time.Now().UTC(), 40.7128, -74.0060)
	require.NoError(t, obsRepo.Create(context.Background(), obs))

	got, err := obsRepo.GetLatest(context.Background(), entity.ID)
	require.NoError(t, err)
	assert.Equal(t, obs.ID, got.ID)
	assert.InDelta(t, 40.7128, got.Position.Lat, 0.0001)
	assert.InDelta(t, -74.0060, got.Position.Lon, 0.0001)
	assert.Equal(t, float64(500), got.Velocity["speed"])
}

func TestObservationRepository_CreateBatchUpsert(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	ts := time.Now().UTC().Truncate(time.Millisecond)
	obs1 := makeObservation(entity.ID, ts, 40.0, -74.0)
	obs1.ContentHash = "hash1"

	require.NoError(t, obsRepo.CreateBatchUpsert(context.Background(), []*domain.Observation{obs1}))

	// Upsert the same (entity_id, ts) with updated data.
	obs2 := makeObservation(entity.ID, ts, 41.0, -75.0)
	obs2.ContentHash = "hash2"
	require.NoError(t, obsRepo.CreateBatchUpsert(context.Background(), []*domain.Observation{obs2}))

	got, err := obsRepo.GetLatest(context.Background(), entity.ID)
	require.NoError(t, err)
	assert.Equal(t, "hash2", got.ContentHash)
	assert.InDelta(t, 41.0, got.Position.Lat, 0.0001)
}

func TestObservationRepository_GetByEntityID(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		obs := makeObservation(entity.ID, now.Add(time.Duration(-i)*time.Minute), 40.0, -74.0)
		require.NoError(t, obsRepo.Create(context.Background(), obs))
	}

	got, err := obsRepo.GetByEntityID(context.Background(), entity.ID, 3, now.Add(time.Second))
	require.NoError(t, err)
	assert.Len(t, got, 3)
	// Verify DESC order.
	assert.True(t, got[0].Timestamp.After(got[1].Timestamp))
}

func TestObservationRepository_GetLatestForEntityIDs(t *testing.T) {
	db := newTestDB(t)
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	e1 := makeEntity("flights", "F1", "Flight 1")
	e2 := makeEntity("flights", "F2", "Flight 2")
	require.NoError(t, entityRepo.Create(context.Background(), e1))
	require.NoError(t, entityRepo.Create(context.Background(), e2))

	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e1.ID, time.Now().UTC(), 40.0, -74.0)))
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e2.ID, time.Now().UTC(), 41.0, -75.0)))

	result, err := obsRepo.GetLatestForEntityIDs(context.Background(), []string{e1.ID, e2.ID})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.NotNil(t, result[e1.ID])
	assert.NotNil(t, result[e2.ID])
}

func TestObservationRepository_GetLatestContentHashes(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	obs := makeObservation(entity.ID, time.Now().UTC(), 40.0, -74.0)
	obs.ContentHash = "abc123"
	require.NoError(t, obsRepo.Create(context.Background(), obs))

	hashes, err := obsRepo.GetLatestContentHashes(context.Background(), []string{entity.ID})
	require.NoError(t, err)
	assert.Equal(t, "abc123", hashes[entity.ID])
}

func TestObservationRepository_GetLayerSnapshotAt(t *testing.T) {
	db := newTestDB(t)
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Flight 1")
	require.NoError(t, entityRepo.Create(context.Background(), e))

	now := time.Now().UTC()
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e.ID, now.Add(-30*time.Minute), 40.0, -74.0)))
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e.ID, now.Add(-10*time.Minute), 41.0, -75.0)))

	snapshots, err := obsRepo.GetLayerSnapshotAt(context.Background(), "flights", now, time.Hour, 1000)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.InDelta(t, 41.0, snapshots[0].Observation.Position.Lat, 0.0001)
}

func TestObservationRepository_GetLatestForLayerByBBox(t *testing.T) {
	db := newTestDB(t)
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	e1 := makeEntity("flights", "F1", "NYC Flight")
	e2 := makeEntity("flights", "F2", "London Flight")
	require.NoError(t, entityRepo.Create(context.Background(), e1))
	require.NoError(t, entityRepo.Create(context.Background(), e2))

	now := time.Now().UTC()
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e1.ID, now, 40.7, -74.0))) // NYC
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e2.ID, now, 51.5, -0.12))) // London

	// BBox around NYC only.
	snapshots, err := obsRepo.GetLatestForLayerByBBox(context.Background(), "flights",
		39.0, 42.0, -76.0, -72.0,
		now.Add(-time.Hour), now.Add(time.Second), 100)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.Equal(t, e1.ID, snapshots[0].Entity.ID)
}

func TestObservationRepository_BBox_Antimeridian(t *testing.T) {
	db := newTestDB(t)
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "PAC1", "Pacific Flight")
	require.NoError(t, entityRepo.Create(context.Background(), e))

	now := time.Now().UTC()
	require.NoError(t, obsRepo.Create(context.Background(), makeObservation(e.ID, now, 35.0, 175.0)))

	// Antimeridian bbox: west=170, east=-170 (wraps around 180°).
	snapshots, err := obsRepo.GetLatestForLayerByBBox(context.Background(), "flights",
		30.0, 40.0, 170.0, -170.0,
		now.Add(-time.Hour), now.Add(time.Second), 100)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
}

func TestObservationRepository_PatchAIMetadata(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	obs := makeObservation(entity.ID, time.Now().UTC(), 40.0, -74.0)
	require.NoError(t, obsRepo.Create(context.Background(), obs))

	require.NoError(t, obsRepo.PatchAIMetadata(context.Background(), obs.ID, map[string]any{"enriched": true}))

	got, err := obsRepo.GetLatest(context.Background(), entity.ID)
	require.NoError(t, err)
	assert.Equal(t, true, got.AIMetadata["enriched"])
}

func TestObservationRepository_PatchAIMetadata_Concurrent(t *testing.T) {
	_, _, obsRepo, entity := setupEntityAndRepo(t)

	obs := makeObservation(entity.ID, time.Now().UTC(), 40.0, -74.0)
	require.NoError(t, obsRepo.Create(context.Background(), obs))

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = obsRepo.PatchAIMetadata(context.Background(), obs.ID, map[string]any{fmt.Sprintf("k%d", i): i})
		}(i)
	}
	wg.Wait()

	got, err := obsRepo.GetLatest(context.Background(), entity.ID)
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		assert.Contains(t, got.AIMetadata, fmt.Sprintf("k%d", i))
	}
}
