package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// maintenanceRepository is the SQLite implementation of domain.MaintenanceRepository.
// It enforces an on-disk size cap by pruning the oldest observations and reclaiming
// freed pages with incremental_vacuum + a truncating WAL checkpoint.
//
// Tuning knobs (batchSize, maxBatchesPerCall, incrementalVacuumPages) are
// SQLite-specific and bound the lock contention the prune places on the single
// shared writer; they are adapter configuration, not part of the engine-neutral port.
type maintenanceRepository struct {
	db                     *sql.DB
	logger                 zerolog.Logger
	batchSize              int
	maxBatchesPerCall      int
	incrementalVacuumPages int
}

// NewMaintenanceRepository creates a SQLite-backed MaintenanceRepository.
func NewMaintenanceRepository(db *sql.DB, logger zerolog.Logger, batchSize, maxBatchesPerCall, incrementalVacuumPages int) domain.MaintenanceRepository {
	return &maintenanceRepository{
		db:                     db,
		logger:                 logger,
		batchSize:              batchSize,
		maxBatchesPerCall:      maxBatchesPerCall,
		incrementalVacuumPages: incrementalVacuumPages,
	}
}

// DatabaseSizeBytes returns the on-disk size of the main database file as
// page_count * page_size (an O(1) header read; no table scan).
func (r *maintenanceRepository) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	fileBytes, _, err := r.sizes(ctx)
	return fileBytes, err
}

// sizes returns the on-disk file size (page_count*page_size) and the live data size
// ((page_count-freelist_count)*page_size). DELETE moves pages to the freelist
// without shrinking the file, so the prune loop targets LIVE size; the file shrinks
// to ~live only after reclaim (incremental_vacuum + checkpoint).
func (r *maintenanceRepository) sizes(ctx context.Context) (fileBytes, liveBytes int64, err error) {
	var pageCount, pageSize, freelist int64
	if err = r.db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, 0, translateError("maintenance", err)
	}
	if err = r.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, 0, translateError("maintenance", err)
	}
	if err = r.db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelist); err != nil {
		return 0, 0, translateError("maintenance", err)
	}
	return pageCount * pageSize, (pageCount - freelist) * pageSize, nil
}

// EnforceSizeCap prunes oldest observations until the live data size is at or below
// maxBytes*lowWaterRatio, then reclaims freed pages so the on-disk file shrinks.
func (r *maintenanceRepository) EnforceSizeCap(ctx context.Context, maxBytes int64, lowWaterRatio float64) (domain.RetentionResult, error) {
	res := domain.RetentionResult{}

	fileBytes, _, err := r.sizes(ctx)
	if err != nil {
		return res, err
	}
	res.BytesBefore = fileBytes
	res.BytesAfter = fileBytes

	// Disabled, or under the HIGH watermark: nothing to do (hysteresis — we only act
	// once the file has crossed maxBytes, then prune down to the LOW watermark).
	if maxBytes <= 0 || fileBytes <= maxBytes {
		return res, nil
	}

	low := int64(float64(maxBytes) * lowWaterRatio)

	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		_, liveBytes, err := r.sizes(ctx)
		if err != nil {
			return res, err
		}
		if liveBytes <= low {
			break
		}
		if res.BatchesRun >= r.maxBatchesPerCall {
			res.BudgetExhausted = true
			break
		}
		deleted, err := r.deleteOldestObservations(ctx, r.batchSize)
		if err != nil {
			return res, err
		}
		res.BatchesRun++
		res.DeletedObservations += deleted
		if deleted == 0 {
			break // table drained
		}
	}

	// ai_insight_refs.observation_id has no foreign key, so pruning observations does
	// not cascade to it — remove the orphans explicitly.
	orphans, err := r.deleteOrphanedInsightRefs(ctx)
	if err != nil {
		return res, err
	}
	res.DeletedOrphanRefs = orphans

	// Return freed pages to the OS and truncate the WAL so the file (and -wal) shrink:
	// in WAL mode the shrink is otherwise only written into the WAL and never applied.
	if err := r.reclaim(ctx); err != nil {
		return res, err
	}

	after, _, err := r.sizes(ctx)
	if err != nil {
		return res, err
	}
	res.BytesAfter = after
	return res, nil
}

// deleteOldestObservations deletes up to batch oldest-by-ts observations in one
// transaction. The subquery form is required because the modernc.org/sqlite
// amalgamation omits SQLITE_ENABLE_UPDATE_DELETE_LIMIT (no DELETE ... LIMIT).
func (r *maintenanceRepository) deleteOldestObservations(ctx context.Context, batch int) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM observations
		 WHERE id IN (SELECT id FROM observations ORDER BY ts ASC LIMIT ?)`,
		batch,
	)
	if err != nil {
		return 0, translateError("maintenance", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// deleteOrphanedInsightRefs removes ai_insight_refs rows whose observation_id no
// longer matches a live observation. NOT EXISTS with the index on observations(id)
// keeps this an indexed lookup per (small) ref row rather than a scan.
func (r *maintenanceRepository) deleteOrphanedInsightRefs(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM ai_insight_refs
		 WHERE observation_id IS NOT NULL
		   AND NOT EXISTS (SELECT 1 FROM observations o WHERE o.id = ai_insight_refs.observation_id)`,
	)
	if err != nil {
		return 0, translateError("maintenance", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// reclaim returns up to incrementalVacuumPages freed pages to the OS, then truncates
// the WAL so the on-disk file actually shrinks. incrementalVacuumPages is a validated
// int from config (not user input), so interpolation is safe — PRAGMA arguments
// cannot be bound parameters.
func (r *maintenanceRepository) reclaim(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, fmt.Sprintf("PRAGMA incremental_vacuum(%d)", r.incrementalVacuumPages)); err != nil {
		return translateError("maintenance", err)
	}
	if _, err := r.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return translateError("maintenance", err)
	}
	return nil
}
