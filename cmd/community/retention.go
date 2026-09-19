package main

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// runPeriodicOptimize refreshes SQLite's query-planner statistics (PRAGMA optimize)
// on a ticker until ctx is cancelled, so plans stay good as tables grow. Best-effort
// and serial, mirroring runStorageRetention.
func runPeriodicOptimize(ctx context.Context, db *sqlitedb.DB, interval time.Duration, logger zerolog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := db.Optimize(ctx); err != nil && ctx.Err() == nil {
				logger.Warn().Err(err).Msg("periodic PRAGMA optimize failed")
			}
		}
	}
}

// runStorageRetention enforces the SQLite size cap on a ticker until ctx is
// cancelled. It runs one pass immediately (to recover a database that grew while
// the process was down), then once per interval. Passes are serial — a single
// goroutine — so there is no overlap to guard against. The caller starts this only
// when retention is enabled (maxBytes > 0). It mirrors the lifecycle of the
// existing cleanup reapers (analysis engine, enrichment worker).
func runStorageRetention(ctx context.Context, repo domain.MaintenanceRepository, maxBytes int64, lowWaterRatio float64, interval time.Duration, logger zerolog.Logger) {
	enforce := func() {
		res, err := repo.EnforceSizeCap(ctx, maxBytes, lowWaterRatio)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down — not a real failure
			}
			logger.Warn().Err(err).Msg("storage retention pass failed")
			return
		}
		if res.DeletedObservations > 0 || res.DeletedOrphanRefs > 0 {
			logger.Info().
				Int64("deleted_observations", res.DeletedObservations).
				Int64("deleted_orphan_refs", res.DeletedOrphanRefs).
				Int64("bytes_before", res.BytesBefore).
				Int64("bytes_after", res.BytesAfter).
				Int("batches", res.BatchesRun).
				Msg("storage retention pruned database")
		}
		if res.BudgetExhausted {
			logger.Warn().
				Int64("bytes_after", res.BytesAfter).
				Msg("storage retention budget exhausted while still over cap; continuing next tick")
		}
	}

	enforce() // immediate pass at startup

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			enforce()
		}
	}
}
