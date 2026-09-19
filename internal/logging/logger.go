// Package logging provides structured, context-aware logging.
package logging

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"
)

// CtxKey type for context keys
type CtxKey string

const (
	ctxKeyRequestID CtxKey = "request_id"
	ctxKeyTraceID   CtxKey = "trace_id"
	ctxKeyLayerType CtxKey = "layer_type"
	ctxKeySource    CtxKey = "source"
)

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(key string, value error) Field {
	return Field{Key: key, Value: value}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Logger wraps zerolog with context enrichment
type Logger struct {
	zl      zerolog.Logger
	service string
}

// NewLogger creates a new logger for the service
func NewLogger(service string, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}

	zl := zerolog.New(output).With().Timestamp().Str("service", service).Logger()

	return &Logger{
		zl:      zl,
		service: service,
	}
}

// NewNopLogger creates a logger that discards output
func NewNopLogger() *Logger {
	return &Logger{
		zl: zerolog.New(io.Discard),
	}
}

// With returns a logger with a field set
func (l *Logger) With(key string, value interface{}) *Logger {
	return &Logger{
		zl:      l.zl.With().Interface(key, value).Logger(),
		service: l.service,
	}
}

// WithRequestID returns a logger with request ID context
func (l *Logger) WithRequestID(id string) *Logger {
	return l.With(string(ctxKeyRequestID), id)
}

// WithLayerType returns a logger with layer type context
func (l *Logger) WithLayerType(layerType string) *Logger {
	return l.With(string(ctxKeyLayerType), layerType)
}

// WithSource returns a logger with source context
func (l *Logger) WithSource(source string) *Logger {
	return l.With(string(ctxKeySource), source)
}

// WithTraceID returns a logger with trace_id context
func (l *Logger) WithTraceID(traceID string) *Logger {
	return l.With(string(ctxKeyTraceID), traceID)
}

// WithFields returns a logger with multiple fields
func (l *Logger) WithFields(fields ...Field) *Logger {
	ctx := l.zl.With()
	for _, f := range fields {
		ctx = ctx.Interface(f.Key, f.Value)
	}
	return &Logger{
		zl:      ctx.Logger(),
		service: l.service,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(zerolog.DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Field) {
	l.log(zerolog.InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(zerolog.WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...Field) {
	l.log(zerolog.ErrorLevel, msg, fields...)
}

// log is the internal method that logs at a level with fields
func (l *Logger) log(level zerolog.Level, msg string, fields ...Field) {
	event := l.zl.WithLevel(level)
	if event == nil {
		return
	}
	for _, f := range fields {
		event = event.Interface(f.Key, f.Value)
	}
	event.Msg(msg)
}

// ContextLogger provides context-aware logging
type ContextLogger struct {
	logger *Logger
	ctx    context.Context
}

// WithContext creates a context logger
func (l *Logger) WithContext(ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: l,
		ctx:    ctx,
	}
}

// enrichFromContext adds context values to the logger
func (cl *ContextLogger) enrichFromContext() *Logger {
	l := cl.logger
	if cl.ctx == nil {
		return l
	}

	// Extract values from context if present
	if requestID := contextValue(cl.ctx, ctxKeyRequestID); requestID != nil {
		l = l.WithRequestID(requestID.(string))
	}
	if traceID := contextValue(cl.ctx, ctxKeyTraceID); traceID != nil {
		l = l.WithTraceID(traceID.(string))
	}
	if layerType := contextValue(cl.ctx, ctxKeyLayerType); layerType != nil {
		l = l.WithLayerType(layerType.(string))
	}
	if source := contextValue(cl.ctx, ctxKeySource); source != nil {
		l = l.WithSource(source.(string))
	}

	return l
}

// contextValue safely extracts a value from context
func contextValue(ctx context.Context, key CtxKey) interface{} {
	if ctx == nil {
		return nil
	}
	return ctx.Value(key)
}

// Debug logs a debug message with context enrichment
func (cl *ContextLogger) Debug(msg string, fields ...Field) {
	cl.enrichFromContext().Debug(msg, fields...)
}

// Info logs an info message with context enrichment
func (cl *ContextLogger) Info(msg string, fields ...Field) {
	cl.enrichFromContext().Info(msg, fields...)
}

// Warn logs a warning message with context enrichment
func (cl *ContextLogger) Warn(msg string, fields ...Field) {
	cl.enrichFromContext().Warn(msg, fields...)
}

// Error logs an error message with context enrichment
func (cl *ContextLogger) Error(msg string, fields ...Field) {
	cl.enrichFromContext().Error(msg, fields...)
}

// With returns a new context logger with additional fields
func (cl *ContextLogger) With(key string, value interface{}) *ContextLogger {
	return &ContextLogger{
		logger: cl.logger.With(key, value),
		ctx:    cl.ctx,
	}
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, requestID)
}

// WithLayerType adds layer type to context
func WithLayerType(ctx context.Context, layerType string) context.Context {
	return context.WithValue(ctx, ctxKeyLayerType, layerType)
}

// WithSource adds source to context
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, ctxKeySource, source)
}

// WithTraceID adds trace_id to context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID, traceID)
}
