// Package tasker is a reserved placeholder for a future in-memory task
// scheduling service.
//
// The Community Edition is a single binary with no external job queue, so
// background work currently runs as goroutines wired at the composition root
// (cmd/community): the feeder, the enrichment worker, and the analysis engine.
// This package is kept as the future home for a unified in-memory task
// scheduler (cron-style jobs, retries, backfill tasks, harvest tasks) that
// coordinates those workers without introducing an external dependency.
//
// When implemented, it must follow the app-layer rules in CLAUDE.md: depend on
// domain interfaces only and receive its dependencies via constructor
// injection.
package tasker
