package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

func newTestDB(t *testing.T) *sqlitedb.DB {
	t.Helper()
	db, err := sqlitedb.Open(":memory:", zerolog.Nop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.RunMigrations())
	return db
}

func TestOpen_InMemory(t *testing.T) {
	db, err := sqlitedb.Open(":memory:", zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Verify WAL mode is enabled.
	var mode string
	err = db.SqlDB().QueryRow("PRAGMA journal_mode").Scan(&mode)
	require.NoError(t, err)
	// In-memory databases may return "memory" instead of "wal".
	assert.Contains(t, []string{"wal", "memory"}, mode)

	// Verify foreign keys are enabled.
	var fk int
	err = db.SqlDB().QueryRow("PRAGMA foreign_keys").Scan(&fk)
	require.NoError(t, err)
	assert.Equal(t, 1, fk)
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := newTestDB(t)

	// Running migrations again should not fail.
	err := db.RunMigrations()
	require.NoError(t, err)

	// Verify tracking table has entries.
	var count int
	err = db.SqlDB().QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
}

// TestDB_CloseCheckpointsWAL verifies Close() checkpoints+truncates the WAL so
// an abrupt next-open cannot find uncommitted WAL frames. After Close on a
// file-backed DB with prior writes, the -wal sidecar must be absent or empty.
func TestDB_CloseCheckpointsWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.db")

	db, err := sqlitedb.Open(path, zerolog.Nop())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.SqlDB().Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY); INSERT INTO t DEFAULT VALUES;`); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(path + "-wal")
	if err == nil && info.Size() > 0 {
		t.Fatalf("WAL not checkpointed on Close: -wal size = %d", info.Size())
	}
	// (os.IsNotExist(err) — WAL removed — is also a pass.)
}

// TestOpen_SizeCapPragmas verifies Open() configures the fresh database for
// size-cap retention: auto_vacuum=INCREMENTAL (so incremental_vacuum can reclaim
// pages), a bounded journal_size_limit (so the WAL footprint stays near the
// page-count measure), and synchronous=NORMAL (safe under WAL, cheaper fsync).
func TestOpen_SizeCapPragmas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pragmas.db")

	db, err := sqlitedb.Open(path, zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var autoVacuum int
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA auto_vacuum").Scan(&autoVacuum))
	assert.Equal(t, 2, autoVacuum, "auto_vacuum must be INCREMENTAL (2) on a fresh DB")

	var journalSizeLimit int64
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA journal_size_limit").Scan(&journalSizeLimit))
	assert.Equal(t, int64(6144000), journalSizeLimit)

	var synchronous int
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA synchronous").Scan(&synchronous))
	assert.Equal(t, 1, synchronous, "synchronous must be NORMAL (1)")
}

// TestOpen_IOTuningPragmas verifies Open() applies the read/write I/O tuning that
// keeps a large, write-heavy DB off the disk-thrash path: a 64 MB page cache,
// in-memory temp B-trees, and 256 MB of memory-mapped I/O.
func TestOpen_IOTuningPragmas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iotuning.db")

	db, err := sqlitedb.Open(path, zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// cache_size is reported in pages when positive, or as negative KiB when set
	// that way. One budget is split across the whole pool, so the per-connection
	// value depends on how many readers this host runs.
	var cacheSize int
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA cache_size").Scan(&cacheSize))
	// The exact value depends on how many readers this host runs, since one
	// budget is divided across the pool (pagecache.go). What must hold
	// everywhere: it is expressed in KiB, and no single connection may reserve
	// the 64MiB that used to be hardcoded per connection.
	assert.Negative(t, cacheSize, "cache_size must be set in KiB, not pages")
	assert.Less(t, -cacheSize, 65536, "no connection may reserve the old 64MiB")

	// temp_store: 0=default, 1=FILE, 2=MEMORY.
	var tempStore int
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA temp_store").Scan(&tempStore))
	assert.Equal(t, 2, tempStore, "temp_store must be MEMORY (2)")

	var mmapSize int64
	require.NoError(t, db.SqlDB().QueryRow("PRAGMA mmap_size").Scan(&mmapSize))
	assert.Equal(t, int64(64*1024*1024), mmapSize, "mmap_size must be the container-sized 64MiB window")
}

// TestOpen_ReadWritePools verifies the read/write pool split for file databases:
// ReadDB() is a distinct query-only pool that sees committed writes, and a read
// held open on it does not block a write on the write pool (WAL concurrency).
func TestOpen_ReadWritePools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools.db")
	ctx := context.Background()

	db, err := sqlitedb.Open(path, zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, db.RunMigrations())

	// Distinct pools for a file DB.
	assert.NotSame(t, db.SqlDB(), db.ReadDB(), "file DB must have a separate read pool")

	// Write on the write pool; read sees it after commit.
	_, err = db.SqlDB().ExecContext(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v INTEGER)")
	require.NoError(t, err)
	_, err = db.SqlDB().ExecContext(ctx, "INSERT INTO t (v) VALUES (1)")
	require.NoError(t, err)
	var v int
	require.NoError(t, db.ReadDB().QueryRowContext(ctx, "SELECT v FROM t LIMIT 1").Scan(&v))
	assert.Equal(t, 1, v)

	// The read pool is query-only: writes must be rejected.
	_, err = db.ReadDB().ExecContext(ctx, "INSERT INTO t (v) VALUES (2)")
	require.Error(t, err, "read pool must reject writes (query_only)")

	// A read transaction held open on the read pool must NOT block a write on the
	// write pool — this is the whole point of the split (WAL reader + writer).
	rtx, err := db.ReadDB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	require.NoError(t, err)
	var n int
	require.NoError(t, rtx.QueryRowContext(ctx, "SELECT COUNT(*) FROM t").Scan(&n))
	_, err = db.SqlDB().ExecContext(ctx, "INSERT INTO t (v) VALUES (3)")
	require.NoError(t, err, "write must proceed while a read transaction is open on the read pool")
	require.NoError(t, rtx.Rollback())
}

// TestOpen_MemoryReadAliasesWrite verifies in-memory databases reuse the single
// write handle for reads (each :memory: handle is a distinct database, so a
// separate pool would be empty).
func TestOpen_MemoryReadAliasesWrite(t *testing.T) {
	db, err := sqlitedb.Open(":memory:", zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	assert.Same(t, db.SqlDB(), db.ReadDB(), ":memory: must alias read pool to the write handle")
}

// TestOptimize verifies the PRAGMA optimize wrappers run without error on both a
// fresh and a migrated database.
func TestOptimize(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.OptimizeStartup(ctx))
	require.NoError(t, db.Optimize(ctx))
}

// TestRunMigrations_ObservationsTsIndex verifies migration 003 creates the global
// timestamp index that makes oldest-first pruning an index range scan.
func TestRunMigrations_ObservationsTsIndex(t *testing.T) {
	db := newTestDB(t)

	var name string
	err := db.SqlDB().QueryRow(
		"SELECT name FROM sqlite_master WHERE type='index' AND name=?",
		"idx_observations_ts",
	).Scan(&name)
	require.NoError(t, err, "idx_observations_ts should exist after migrations")
	assert.Equal(t, "idx_observations_ts", name)
}

func TestRunMigrations_CreatesSchema(t *testing.T) {
	db := newTestDB(t)

	// Verify core tables exist by querying sqlite_master.
	tables := []string{"entities", "observations", "ai_insights", "ai_insight_refs", "ai_enrichment_log", "layers"}
	for _, table := range tables {
		var name string
		err := db.SqlDB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}
