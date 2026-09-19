// Package analyzer is a reserved placeholder for a future in-memory analysis
// scheduling service.
//
// The Community Edition currently runs cross-layer analysis through
// internal/ai/analysis (the analysis Engine), wired directly at the composition
// root (cmd/community). This package is kept as the future home for a
// standalone analyzer scheduler that orchestrates analysis runs on a cadence
// (e.g. periodic re-analysis, scheduled insights) without an external broker.
//
// When implemented, it must follow the app-layer rules in CLAUDE.md: depend on
// domain interfaces only and receive its dependencies via constructor
// injection.
package analyzer
