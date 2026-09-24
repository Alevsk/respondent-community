package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// DB wraps SQLite with a read/write pool split. Under WAL, SQLite allows one
// writer plus many concurrent readers, so writes use a single serialized
// connection (no SQLITE_BUSY) and reads use a query-only pool — the analysis
// engine's long queries then run there instead of head-of-line-blocking
// ingestion and WebSocket reads on the writer. For :memory: (tests) each handle
// is a distinct database, so read aliases write.
type DB struct {
	write  *sql.DB
	read   *sql.DB // == write for :memory:; a separate query_only pool for file DBs
	logger zerolog.Logger
}

func isMemoryPath(path string) bool {
	return strings.Contains(path, ":memory:") || strings.Contains(path, "mode=memory")
}

// Open creates a new SQLite connection with WAL mode and foreign key enforcement.
// Use ":memory:" for in-memory databases (tests).
func Open(path string, logger zerolog.Logger) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// Enable incremental auto-vacuum BEFORE any table exists so freed pages can be
	// returned to the OS via PRAGMA incremental_vacuum (used by the size-cap
	// retention routine). auto_vacuum can only be set on an empty database without a
	// full VACUUM, so this must run before RunMigrations creates the schema. On a
	// pre-existing NONE database this is a silent no-op (community starts fresh).
	if _, err := db.Exec("PRAGMA auto_vacuum=INCREMENTAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable incremental auto_vacuum: %w", err)
	}

	// Enable WAL mode for concurrent read/write.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Bound the -wal sidecar so the on-disk footprint stays close to the page-count
	// size the retention routine measures: after each checkpoint the WAL is
	// truncated back down to this limit (~6 MB).
	if _, err := db.Exec("PRAGMA journal_size_limit=6144000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal size limit: %w", err)
	}

	// NORMAL is durable under WAL (only loses the last transaction on OS crash, not
	// corruption) and avoids an fsync per commit — cheaper for the single writer
	// shared by ingestion, enrichment, and the prune batches.
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set synchronous mode: %w", err)
	}

	// Enable foreign key constraints (off by default in SQLite).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Reasonable busy timeout for concurrent access.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// --- Read/write I/O tuning -------------------------------------------------
	// The defaults are tuned for tiny embedded databases, not a server holding a
	// large, write-heavy observations table queried continuously by WebSocket
	// subscribers and the analysis engine. Without these, every query over a DB
	// larger than the ~2 MB default page cache thrashes the cache to disk and the
	// snapshot/analysis ORDER BYs spill their temp B-trees to disk — the "loads
	// and returns data super slowly" symptom.

	// Page cache (negative = KiB) for the connection every repository uses.
	// Keeps hot index/leaf pages resident so repeated reads of the same
	// entities/indexes don't re-hit disk on every query. See pagecache.go for
	// why the two pools are sized separately.
	readConns := readPoolSize()
	if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size=-%d", writeCacheKiB)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set cache size: %w", err)
	}

	// Build temp B-trees (ORDER BY / GROUP BY / the snapshot+analysis sorts) in
	// memory instead of spilling them to on-disk temp files.
	if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set temp store: %w", err)
	}

	// Memory-map part of the database file so reads served from the mmap region
	// avoid a read() syscall + buffer copy per page. The mapping is virtual
	// address space rather than committed RAM, but cgroup v2 charges the pages
	// it faults in as file memory, so the window is sized for a small container
	// rather than left at a server-sized default. A no-op for :memory: databases.
	if _, err := db.Exec(fmt.Sprintf("PRAGMA mmap_size=%d", mmapBytes)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set mmap size: %w", err)
	}

	// Serialize all WRITES through a single connection. SQLite allows only one
	// writer at a time (even under WAL); a single connection lets Go's pool
	// serialize writers internally, eliminating SQLITE_BUSY. Reads are served by
	// the separate read pool below.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // local file — never expire (avoids re-running pragmas)

	wrapped := &DB{write: db, logger: logger}

	// In-memory databases cannot share state across separate pools (each handle
	// is its own database), so reads use the single write handle.
	if isMemoryPath(path) {
		wrapped.read = db
		return wrapped, nil
	}

	// Force the write connection to materialize now so the file exists and is in
	// WAL mode before the read pool opens against it.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping write pool: %w", err)
	}

	// --- Read pool (file-backed only) -----------------------------------------
	// A query-only pool over the same file. WAL lets these readers run
	// concurrently with the writer and with each other, so a long analysis query
	// no longer blocks ingestion or interactive reads. Per-connection PRAGMAs
	// must be supplied via the DSN here (Exec-set pragmas would only configure one
	// of the pool's connections).
	readPragmas := []string{
		"_pragma=query_only(1)", // defense-in-depth: this pool never writes
		"_pragma=busy_timeout(5000)",
		fmt.Sprintf("_pragma=cache_size(-%d)", readCacheKiB),
		"_pragma=temp_store(2)", // MEMORY
		fmt.Sprintf("_pragma=mmap_size(%d)", mmapBytes),
	}
	readDSN := "file:" + path + "?" + strings.Join(readPragmas, "&")
	rdb, err := sql.Open("sqlite", readDSN)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite read pool: %w", err)
	}
	rdb.SetMaxOpenConns(readConns)
	rdb.SetMaxIdleConns(readConns)
	rdb.SetConnMaxLifetime(0)
	if err := rdb.Ping(); err != nil {
		_ = rdb.Close()
		_ = db.Close()
		return nil, fmt.Errorf("ping read pool: %w", err)
	}
	wrapped.read = rdb

	return wrapped, nil
}

// SqlDB returns the write pool. All writers and the existing repositories use
// this handle; writes are serialized over its single connection.
func (d *DB) SqlDB() *sql.DB {
	return d.write
}

// ReadDB returns the concurrent read pool (a distinct query-only pool for file
// databases; the same handle as the write pool for :memory:). Use it for heavy
// read-only workloads — notably the analysis engine — so they do not contend
// with writes on the single writer connection.
func (d *DB) ReadDB() *sql.DB {
	return d.read
}

// OptimizeStartup runs `PRAGMA optimize` with the 0x10002 mask, which analyzes
// tables that lack statistics. SQLite ships with no planner statistics, so the
// cross-layer analysis queries (many possible plans) can pick poor plans; running
// this once after migrations gives the planner real stats. Cheap and bounded
// even on large databases (the mask self-limits the analysis).
func (d *DB) OptimizeStartup(ctx context.Context) error {
	_, err := d.write.ExecContext(ctx, "PRAGMA optimize=0x10002")
	return err
}

// Optimize runs `PRAGMA optimize`, refreshing planner statistics for tables whose
// contents have changed enough to matter since the last run. Safe to call
// periodically (e.g. hourly) on a long-lived connection.
func (d *DB) Optimize(ctx context.Context) error {
	_, err := d.write.ExecContext(ctx, "PRAGMA optimize")
	return err
}

// Close checkpoints the WAL into the main database file and truncates it, then
// closes both pools. The read pool is closed first so no readers hold the file
// during the writer's final checkpoint. The checkpoint ensures an abrupt later
// process exit cannot leave uncommitted WAL frames that corrupt the next open
// ("database disk image is malformed").
func (d *DB) Close() error {
	if d.read != nil && d.read != d.write {
		if err := d.read.Close(); err != nil {
			d.logger.Warn().Err(err).Msg("closing read pool failed")
		}
	}
	if _, err := d.write.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		d.logger.Warn().Err(err).Msg("wal checkpoint on close failed")
	}
	return d.write.Close()
}
