package sqlite_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

func makeEntity(layerType, externalID, name string) *domain.Entity {
	return &domain.Entity{
		ID:         uuid.New().String(),
		ExternalID: externalID,
		LayerType:  layerType,
		Name:       name,
		Metadata:   map[string]string{"key": "value"},
		AIMetadata: map[string]any{"score": 0.9},
		Source:     "test",
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
}

// TestEntityRepository_GetByExternalIDs_ChunksLargeBatch guards the regression where
// a batch larger than SQLite's variable limit overflowed it; chunking must return all rows.
func TestEntityRepository_GetByExternalIDs_ChunksLargeBatch(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	ctx := context.Background()

	const n = 2000 // exceeds the 900-param IN(...) chunk size → must paginate
	ents := make([]*domain.Entity, 0, n)
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		e := makeEntity("power_plants", fmt.Sprintf("pp-%d", i), fmt.Sprintf("Plant %d", i))
		ents = append(ents, e)
		ids = append(ids, e.ExternalID)
	}
	require.NoError(t, repo.CreateBatch(ctx, ents))

	got, err := repo.GetByExternalIDs(ctx, "power_plants", ids)
	require.NoError(t, err)
	require.Len(t, got, n, "all entities must be returned across chunks")
}

func TestEntityRepository_CreateAndGetByID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "ABC123", "Flight ABC123")
	require.NoError(t, repo.Create(context.Background(), e))

	got, err := repo.GetByID(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e.ID, got.ID)
	assert.Equal(t, e.ExternalID, got.ExternalID)
	assert.Equal(t, e.LayerType, got.LayerType)
	assert.Equal(t, e.Name, got.Name)
	assert.Equal(t, "value", got.Metadata["key"])
	assert.Equal(t, 0.9, got.AIMetadata["score"])
}

func TestEntityRepository_CreateBatch(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	entities := []*domain.Entity{
		makeEntity("flights", "A1", "Flight A1"),
		makeEntity("flights", "A2", "Flight A2"),
		makeEntity("flights", "A3", "Flight A3"),
	}
	require.NoError(t, repo.CreateBatch(context.Background(), entities))

	got, err := repo.GetByIDs(context.Background(), []string{entities[0].ID, entities[1].ID, entities[2].ID})
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestEntityRepository_GetByExternalID(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("satellites", "SAT-1", "Satellite 1")
	require.NoError(t, repo.Create(context.Background(), e))

	got, err := repo.GetByExternalID(context.Background(), "satellites", "SAT-1")
	require.NoError(t, err)
	assert.Equal(t, e.ID, got.ID)

	_, err = repo.GetByExternalID(context.Background(), "satellites", "MISSING")
	assert.True(t, domain.IsNotFound(err))
}

func TestEntityRepository_GetByExternalIDs(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e1 := makeEntity("flights", "F1", "Flight 1")
	e2 := makeEntity("flights", "F2", "Flight 2")
	require.NoError(t, repo.Create(context.Background(), e1))
	require.NoError(t, repo.Create(context.Background(), e2))

	got, err := repo.GetByExternalIDs(context.Background(), "flights", []string{"F1", "F2", "F3"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestEntityRepository_GetDistinctLayerTypes(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "F1", "F1")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("satellites", "S1", "S1")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "F2", "F2")))

	types, err := repo.GetDistinctLayerTypes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"flights", "satellites"}, types)
}

func TestEntityRepository_CountByLayerType(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "F1", "F1")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "F2", "F2")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "F3", "F3")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("satellites", "S1", "S1")))

	counts, err := repo.CountByLayerType(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"flights": 3, "satellites": 1}, counts)
}

func TestEntityRepository_CountByLayerType_Empty(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	counts, err := repo.CountByLayerType(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, counts)
	assert.Empty(t, counts)
}

func TestEntityRepository_CountByLayerType_DedupesReIngestedEntities(t *testing.T) {
	// The badge counts distinct entities (one row per layer_type+external_id), so a
	// source re-ingesting the same external_id must not inflate the count. CreateBatch
	// upserts via ON CONFLICT (layer_type, external_id), which is the real feeder
	// re-poll path — a fresh row id with the same identity collapses onto one row.
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	first := makeEntity("flights", "DUP", "Flight DUP")
	require.NoError(t, repo.CreateBatch(context.Background(), []*domain.Entity{first}))

	// Re-ingest: same (layer_type, external_id), different row id — as the feeder does.
	reingested := makeEntity("flights", "DUP", "Flight DUP (updated)")
	require.NoError(t, repo.CreateBatch(context.Background(), []*domain.Entity{reingested}))

	counts, err := repo.CountByLayerType(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["flights"])
}

func TestEntityRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Original")
	require.NoError(t, repo.Create(context.Background(), e))

	e.Name = "Updated"
	require.NoError(t, repo.Update(context.Background(), e))

	got, err := repo.GetByID(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
}

func TestEntityRepository_Update_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Original")
	e.ID = uuid.New().String()
	err := repo.Update(context.Background(), e)
	assert.True(t, domain.IsNotFound(err))
}

func TestEntityRepository_Delete(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Flight")
	require.NoError(t, repo.Create(context.Background(), e))
	require.NoError(t, repo.Delete(context.Background(), e.ID))

	_, err := repo.GetByID(context.Background(), e.ID)
	assert.True(t, domain.IsNotFound(err))
}

func TestEntityRepository_Delete_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	err := repo.Delete(context.Background(), uuid.New().String())
	assert.True(t, domain.IsNotFound(err))
}

func TestEntityRepository_SearchEntities(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "ABC123", "Flight ABC123")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "XYZ789", "Flight XYZ789")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("satellites", "ISS", "International Space Station")))

	results, total, err := repo.SearchEntities(context.Background(), "ABC", "", 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, "ABC123", results[0].Entity.ExternalID)
}

func TestEntityRepository_SearchEntities_WithLayerFilter(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	require.NoError(t, repo.Create(context.Background(), makeEntity("flights", "ABC", "ABC Flight")))
	require.NoError(t, repo.Create(context.Background(), makeEntity("satellites", "ABC", "ABC Satellite")))

	results, total, err := repo.SearchEntities(context.Background(), "ABC", "flights", 10)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, "flights", results[0].Entity.LayerType)
}

func TestEntityRepository_PatchAIMetadata(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Flight")
	require.NoError(t, repo.Create(context.Background(), e))

	require.NoError(t, repo.PatchAIMetadata(context.Background(), e.ID, map[string]any{"threat": "low"}))

	got, err := repo.GetByID(context.Background(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, "low", got.AIMetadata["threat"])
	assert.Equal(t, 0.9, got.AIMetadata["score"])
}

func TestEntityRepository_PatchAIMetadata_Concurrent(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())
	e := makeEntity("flights", "F1", "Flight")
	require.NoError(t, repo.Create(context.Background(), e))

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = repo.PatchAIMetadata(context.Background(), e.ID, map[string]any{fmt.Sprintf("k%d", i): i})
		}(i)
	}
	wg.Wait()

	got, err := repo.GetByID(context.Background(), e.ID)
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		assert.Contains(t, got.AIMetadata, fmt.Sprintf("k%d", i))
	}
}

func TestEntityRepository_CreateDuplicate(t *testing.T) {
	db := newTestDB(t)
	repo := sqlitedb.NewEntityRepository(db.SqlDB(), testLogger())

	e := makeEntity("flights", "F1", "Flight")
	require.NoError(t, repo.Create(context.Background(), e))

	e2 := makeEntity("flights", "F1", "Flight Updated")
	err := repo.Create(context.Background(), e2)
	assert.True(t, domain.IsConflict(err))
}
