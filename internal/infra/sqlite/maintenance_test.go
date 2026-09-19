package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// newFileTestDB opens a file-backed DB (not :memory:) so incremental_vacuum and
// wal_checkpoint(TRUNCATE) actually shrink an on-disk file we can measure.
func newFileTestDB(t *testing.T) *sqlitedb.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "maint.db")
	db, err := sqlitedb.Open(path, zerolog.Nop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.RunMigrations())
	return db
}

// seedObservations inserts one entity and n observations with strictly increasing
// ts (one second apart). Returns the ordered observation IDs (oldest first).
func seedObservations(t *testing.T, db *sqlitedb.DB, n int) []string {
	t.Helper()
	ctx := context.Background()
	entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), zerolog.Nop())
	obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), zerolog.Nop())

	entity := makeEntity("flights_commercial", "TEST", "Test")
	require.NoError(t, entityRepo.Create(ctx, entity))

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]string, 0, n)
	batch := make([]*domain.Observation, 0, n)
	for i := range n {
		obs := makeObservation(entity.ID, base.Add(time.Duration(i)*time.Second), 1.0, 2.0)
		obs.ContentHash = uuid.New().String() // unique to avoid (entity_id,ts) collisions
		// Pad metadata so rows have realistic byte weight and the file is reclaimable.
		obs.Metadata = map[string]string{"callsign": "PAD", "note": "padding to grow row byte size for reclaim test"}
		ids = append(ids, obs.ID)
		batch = append(batch, obs)
	}
	require.NoError(t, obsRepo.CreateBatch(ctx, batch))
	return ids
}

func countObservations(t *testing.T, db *sqlitedb.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.SqlDB().QueryRow("SELECT COUNT(*) FROM observations").Scan(&n))
	return n
}

func observationExists(t *testing.T, db *sqlitedb.DB, id string) bool {
	t.Helper()
	var x int
	err := db.SqlDB().QueryRow("SELECT 1 FROM observations WHERE id = ?", id).Scan(&x)
	return err == nil
}

// TestMaintenance_NoOpUnderCap: when the DB is under the high watermark, EnforceSizeCap
// deletes nothing and leaves the size unchanged.
func TestMaintenance_NoOpUnderCap(t *testing.T) {
	db := newFileTestDB(t)
	seedObservations(t, db, 1000)
	repo := sqlitedb.NewMaintenanceRepository(db.SqlDB(), zerolog.Nop(), 200, 100, 10000)
	ctx := context.Background()

	size, err := repo.DatabaseSizeBytes(ctx)
	require.NoError(t, err)
	require.Greater(t, size, int64(0))

	res, err := repo.EnforceSizeCap(ctx, size*10, 0.9) // cap far above current size
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.DeletedObservations)
	assert.Equal(t, 0, res.BatchesRun)
	assert.Equal(t, res.BytesBefore, res.BytesAfter)
	assert.Equal(t, 1000, countObservations(t, db))
}

// TestMaintenance_PrunesOldestAndShrinks: over the cap, EnforceSizeCap deletes the
// OLDEST observations first, stops near the low watermark, and the on-disk size drops.
func TestMaintenance_PrunesOldestAndShrinks(t *testing.T) {
	db := newFileTestDB(t)
	ids := seedObservations(t, db, 4000)
	repo := sqlitedb.NewMaintenanceRepository(db.SqlDB(), zerolog.Nop(), 500, 1000, 100000)
	ctx := context.Background()

	before, err := repo.DatabaseSizeBytes(ctx)
	require.NoError(t, err)

	// Cap to half the current size; prune down to 80% of that (low = 0.4*before).
	res, err := repo.EnforceSizeCap(ctx, before/2, 0.8)
	require.NoError(t, err)

	assert.Greater(t, res.DeletedObservations, int64(0), "should have pruned rows")
	assert.Less(t, res.BytesAfter, res.BytesBefore, "on-disk size must shrink after reclaim")
	assert.Less(t, countObservations(t, db), 4000)

	// Oldest-first: the very first seeded id must be gone; the last must survive.
	assert.False(t, observationExists(t, db, ids[0]), "oldest observation must be pruned")
	assert.True(t, observationExists(t, db, ids[len(ids)-1]), "newest observation must survive")
}

// TestMaintenance_OrphanedInsightRefsCleaned: pruning observations removes
// ai_insight_refs that point at deleted observations (the column has no FK), while
// refs to surviving observations remain.
func TestMaintenance_OrphanedInsightRefsCleaned(t *testing.T) {
	db := newFileTestDB(t)
	ids := seedObservations(t, db, 4000)
	ctx := context.Background()

	insightRepo := sqlitedb.NewAIInsightRepository(db.SqlDB(), zerolog.Nop())
	attention := "high"
	insightID, err := insightRepo.Create(ctx, &domain.AIInsight{
		InsightType: "test", SourceName: "t", OperationName: "op",
		Attention: &attention, Result: map[string]any{"title": "x"},
	})
	require.NoError(t, err)

	oldest, newest := ids[0], ids[len(ids)-1]
	require.NoError(t, insightRepo.CreateRef(ctx, insightID, nil, &oldest)) // will be pruned
	require.NoError(t, insightRepo.CreateRef(ctx, insightID, nil, &newest)) // will survive

	repo := sqlitedb.NewMaintenanceRepository(db.SqlDB(), zerolog.Nop(), 500, 1000, 100000)
	before, err := repo.DatabaseSizeBytes(ctx)
	require.NoError(t, err)
	res, err := repo.EnforceSizeCap(ctx, before/2, 0.8)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, res.DeletedOrphanRefs, int64(1), "orphaned ref to pruned observation must be deleted")

	refCount := func(obsID string) int {
		var n int
		require.NoError(t, db.SqlDB().QueryRow(
			"SELECT COUNT(*) FROM ai_insight_refs WHERE observation_id = ?", obsID).Scan(&n))
		return n
	}
	assert.Equal(t, 0, refCount(oldest), "ref to pruned observation must be gone")
	assert.Equal(t, 1, refCount(newest), "ref to surviving observation must remain")
}

// TestMaintenance_BatchBudgetBoundsCallAndResumes: a tiny per-call batch budget
// bounds one call (BudgetExhausted) and a second call continues toward the cap.
func TestMaintenance_BatchBudgetBoundsCallAndResumes(t *testing.T) {
	db := newFileTestDB(t)
	seedObservations(t, db, 4000)
	ctx := context.Background()

	// batchSize 100, only 1 batch per call → at most 100 deletions per call.
	repo := sqlitedb.NewMaintenanceRepository(db.SqlDB(), zerolog.Nop(), 100, 1, 100000)
	before, err := repo.DatabaseSizeBytes(ctx)
	require.NoError(t, err)

	res1, err := repo.EnforceSizeCap(ctx, before/2, 0.8)
	require.NoError(t, err)
	assert.Equal(t, 1, res1.BatchesRun)
	assert.LessOrEqual(t, res1.DeletedObservations, int64(100))
	assert.True(t, res1.BudgetExhausted, "still over low watermark after one bounded batch")

	res2, err := repo.EnforceSizeCap(ctx, before/2, 0.8)
	require.NoError(t, err)
	assert.Greater(t, res2.DeletedObservations, int64(0), "second call resumes pruning")
}
