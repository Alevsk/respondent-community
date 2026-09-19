// Package logging provides structured, context-aware logging.
package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
)

// SetupLogger creates a zerolog.Logger with the specified service name, level,
// and format. This is the single source of truth for CLI/service bootstrap
// logging, ensuring consistent defaults across all Respondent binaries.
//
// Supported levels: debug, info, warn, error. Unknown values default to info.
// Supported formats: "console" (human-readable), anything else yields JSON.
func SetupLogger(service, level, format string) zerolog.Logger {
	var l zerolog.Logger

	if format == "console" {
		l = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
			With().Timestamp().Str("service", service).Logger()
	} else {
		l = zerolog.New(os.Stderr).
			With().Timestamp().Str("service", service).Logger()
	}

	switch level {
	case "debug":
		l = l.Level(zerolog.DebugLevel)
	case "info":
		l = l.Level(zerolog.InfoLevel)
	case "warn":
		l = l.Level(zerolog.WarnLevel)
	case "error":
		l = l.Level(zerolog.ErrorLevel)
	default:
		l = l.Level(zerolog.InfoLevel)
	}

	return l
}

// TraceIDFromContext extracts a trace_id from context if one was stored via
// WithTraceID, and returns an enriched logger. If no trace_id is present the
// logger is returned unchanged.
func TraceIDFromContext(ctx interface{ Value(any) any }, l zerolog.Logger) zerolog.Logger {
	if ctx == nil {
		return l
	}
	if traceID, ok := ctx.Value(ctxKeyTraceID).(string); ok && traceID != "" {
		return l.With().Str("trace_id", traceID).Logger()
	}
	return l
}
