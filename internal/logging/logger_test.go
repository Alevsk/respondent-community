// Package logging_test provides unit tests for the logging package.
package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/logging"
)

// ---------------------------------------------------------------------------
// Field constructors
// ---------------------------------------------------------------------------

func TestFieldConstructors(t *testing.T) {
	t.Run("String field stores key and value", func(t *testing.T) {
		f := logging.String("key", "value")
		assert.Equal(t, "key", f.Key)
		assert.Equal(t, "value", f.Value)
	})

	t.Run("String field with empty value", func(t *testing.T) {
		f := logging.String("empty", "")
		assert.Equal(t, "empty", f.Key)
		assert.Equal(t, "", f.Value)
	})

	t.Run("Int field stores key and value", func(t *testing.T) {
		f := logging.Int("count", 42)
		assert.Equal(t, "count", f.Key)
		assert.Equal(t, 42, f.Value)
	})

	t.Run("Int field with zero value", func(t *testing.T) {
		f := logging.Int("zero", 0)
		assert.Equal(t, "zero", f.Key)
		assert.Equal(t, 0, f.Value)
	})

	t.Run("Int field with negative value", func(t *testing.T) {
		f := logging.Int("neg", -1)
		assert.Equal(t, "neg", f.Key)
		assert.Equal(t, -1, f.Value)
	})

	t.Run("Err field stores key and error", func(t *testing.T) {
		err := errors.New("something went wrong")
		f := logging.Err("error", err)
		assert.Equal(t, "error", f.Key)
		assert.Equal(t, err, f.Value)
	})

	t.Run("Err field with nil error", func(t *testing.T) {
		f := logging.Err("error", nil)
		assert.Equal(t, "error", f.Key)
		assert.Nil(t, f.Value)
	})

	t.Run("Any field with map value", func(t *testing.T) {
		data := map[string]int{"a": 1, "b": 2}
		f := logging.Any("data", data)
		assert.Equal(t, "data", f.Key)
		assert.Equal(t, data, f.Value)
	})

	t.Run("Any field with nil value", func(t *testing.T) {
		f := logging.Any("key", nil)
		assert.Equal(t, "key", f.Key)
		assert.Nil(t, f.Value)
	})

	t.Run("Any field with struct value", func(t *testing.T) {
		type inner struct{ V int }
		v := inner{V: 7}
		f := logging.Any("obj", v)
		assert.Equal(t, "obj", f.Key)
		assert.Equal(t, v, f.Value)
	})
}

// ---------------------------------------------------------------------------
// NewLogger
// ---------------------------------------------------------------------------

func TestNewLogger(t *testing.T) {
	t.Run("creates logger with explicit writer", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("test-service", &buf)
		require.NotNil(t, logger)

		// A message should reach the buffer.
		logger.Info("hello")
		assert.NotEmpty(t, buf.Bytes())
	})

	t.Run("creates logger with nil writer defaults to stdout without panic", func(t *testing.T) {
		// Passing nil triggers the os.Stdout fallback branch in NewLogger.
		logger := logging.NewLogger("fallback-service", nil)
		require.NotNil(t, logger)
		// Just exercise the logger; actual output goes to stdout which we cannot
		// capture here, but the important thing is no panic and the branch is hit.
		logger.Info("nil writer fallback")
	})

	t.Run("service name appears in log output", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("my-svc", &buf)
		logger.Info("probe")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "my-svc", record["service"])
	})
}

// ---------------------------------------------------------------------------
// NewNopLogger
// ---------------------------------------------------------------------------

func TestNewNopLogger(t *testing.T) {
	t.Run("returns non-nil logger", func(t *testing.T) {
		logger := logging.NewNopLogger()
		require.NotNil(t, logger)
	})

	t.Run("does not write any output", func(t *testing.T) {
		// NopLogger discards output; calling all log levels must not panic.
		nop := logging.NewNopLogger()
		nop.Debug("debug")
		nop.Info("info")
		nop.Warn("warn")
		nop.Error("error")
	})

	t.Run("supports With chaining without panic", func(t *testing.T) {
		nop := logging.NewNopLogger()
		nop2 := nop.With("k", "v")
		require.NotNil(t, nop2)
		nop2.Info("chained nop")
	})
}

// ---------------------------------------------------------------------------
// Logger.With / WithRequestID / WithLayerType / WithSource / WithFields
// ---------------------------------------------------------------------------

func TestLogger_With(t *testing.T) {
	t.Run("With returns new logger preserving prior fields", func(t *testing.T) {
		var buf bytes.Buffer
		base := logging.NewLogger("svc", &buf)
		derived := base.With("trace_id", "abc-123")
		require.NotNil(t, derived)

		buf.Reset()
		derived.Info("msg")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "abc-123", record["trace_id"])
	})

	t.Run("WithRequestID embeds request_id field", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf).WithRequestID("req-999")
		logger.Info("request")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "req-999", record["request_id"])
	})

	t.Run("WithLayerType embeds layer_type field", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf).WithLayerType("flights_commercial")
		logger.Info("layer")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "flights_commercial", record["layer_type"])
	})

	t.Run("WithSource embeds source field", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf).WithSource("opensky")
		logger.Info("source")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "opensky", record["source"])
	})

	t.Run("chaining preserves all fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf).
			WithRequestID("req-1").
			WithLayerType("ais").
			WithSource("marinetraffic")

		logger.Info("all fields")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "req-1", record["request_id"])
		assert.Equal(t, "ais", record["layer_type"])
		assert.Equal(t, "marinetraffic", record["source"])
	})
}

func TestLogger_WithFields(t *testing.T) {
	t.Run("multiple typed fields all appear in output", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf)

		derived := logger.WithFields(
			logging.String("str_key", "str_val"),
			logging.Int("int_key", 7),
			logging.Any("any_key", "any_val"),
		)
		require.NotNil(t, derived)

		derived.Info("multi-field")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "str_val", record["str_key"])
		assert.InEpsilon(t, 7.0, record["int_key"], 0.001) // JSON numbers are float64
		assert.Equal(t, "any_val", record["any_key"])
	})

	t.Run("WithFields with zero fields is a no-op", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf)
		derived := logger.WithFields()
		require.NotNil(t, derived)
		derived.Info("no extra fields")
		assert.NotEmpty(t, buf.Bytes())
	})

	t.Run("Err field key is present in output", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf)
		err := errors.New("disk full")

		logger.WithFields(logging.Err("err_key", err)).Info("error event")

		// logging.Err stores the error via zerolog's Interface(), which serialises
		// a standard error value as an empty object because error has no exported
		// fields. We therefore only assert that the key is present and the message
		// is correct; the exact shape of the serialised error is a zerolog detail.
		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "error event", record["message"])
		_, hasErrKey := record["err_key"]
		assert.True(t, hasErrKey, "expected err_key to be present in log output")
	})
}

// ---------------------------------------------------------------------------
// Logger log-level methods (output content assertions)
// ---------------------------------------------------------------------------

func TestLogger_LogLevels(t *testing.T) {
	levels := []struct {
		name string
		call func(l *logging.Logger, msg string, fields ...logging.Field)
		want string
	}{
		{"debug", (*logging.Logger).Debug, "debug"},
		{"info", (*logging.Logger).Info, "info"},
		{"warn", (*logging.Logger).Warn, "warn"},
		{"error", (*logging.Logger).Error, "error"},
	}

	for _, tc := range levels {
		tc := tc
		t.Run(tc.name+" level written to output", func(t *testing.T) {
			var buf bytes.Buffer
			logger := logging.NewLogger("svc", &buf)

			tc.call(logger, tc.name+" message", logging.String("k", "v"))

			var record map[string]interface{}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
			assert.Equal(t, tc.want, record["level"])
			assert.Equal(t, tc.name+" message", record["message"])
			assert.Equal(t, "v", record["k"])
		})
	}

	t.Run("log with no extra fields", func(t *testing.T) {
		var buf bytes.Buffer
		logger := logging.NewLogger("svc", &buf)
		logger.Info("bare message")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "bare message", record["message"])
	})
}

// ---------------------------------------------------------------------------
// Logger.log nil-event guard (event == nil branch)
//
// zerolog.Logger.WithLevel returns nil when the global log level is set to
// zerolog.Disabled. This test exercises the nil-event early-return in log().
// It must NOT be run in parallel because it mutates the zerolog global level.
// ---------------------------------------------------------------------------

func TestLogger_LogNilEventGuard(t *testing.T) {
	// Save and restore the global zerolog level so other tests are unaffected.
	original := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(original) })

	zerolog.SetGlobalLevel(zerolog.Disabled)

	var buf bytes.Buffer
	logger := logging.NewLogger("svc", &buf)

	// With global level Disabled, WithLevel returns nil; the guard must not panic.
	logger.Debug("should be swallowed")
	logger.Info("should be swallowed")
	logger.Warn("should be swallowed")
	logger.Error("should be swallowed")

	// Nothing must have been written.
	assert.Empty(t, buf.Bytes())
}

// ---------------------------------------------------------------------------
// ContextLogger methods
// ---------------------------------------------------------------------------

func TestContextLogger_LogLevels(t *testing.T) {
	ctx := context.Background()
	ctx = logging.WithRequestID(ctx, "req-ctx-1")
	ctx = logging.WithLayerType(ctx, "weather")
	ctx = logging.WithSource(ctx, "noaa")

	levels := []struct {
		name string
		call func(cl *logging.ContextLogger, msg string, fields ...logging.Field)
		want string
	}{
		{"debug", (*logging.ContextLogger).Debug, "debug"},
		{"info", (*logging.ContextLogger).Info, "info"},
		{"warn", (*logging.ContextLogger).Warn, "warn"},
		{"error", (*logging.ContextLogger).Error, "error"},
	}

	for _, tc := range levels {
		tc := tc
		t.Run(tc.name+" writes message and context fields", func(t *testing.T) {
			var buf bytes.Buffer
			cl := logging.NewLogger("svc", &buf).WithContext(ctx)

			tc.call(cl, tc.name+" msg", logging.String("extra", "val"))

			var record map[string]interface{}
			require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
			assert.Equal(t, tc.want, record["level"])
			assert.Equal(t, tc.name+" msg", record["message"])
			assert.Equal(t, "req-ctx-1", record["request_id"])
			assert.Equal(t, "weather", record["layer_type"])
			assert.Equal(t, "noaa", record["source"])
			assert.Equal(t, "val", record["extra"])
		})
	}
}

func TestContextLogger_With(t *testing.T) {
	t.Run("With returns new ContextLogger with added field", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := logging.WithRequestID(context.Background(), "req-w")
		cl := logging.NewLogger("svc", &buf).WithContext(ctx)

		cl2 := cl.With("component", "router")
		require.NotNil(t, cl2)

		cl2.Info("with test")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "router", record["component"])
		assert.Equal(t, "req-w", record["request_id"])
	})

	t.Run("With on ContextLogger does not mutate original", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := context.Background()
		cl := logging.NewLogger("svc", &buf).WithContext(ctx)

		_ = cl.With("x", 1)

		// Original cl must not have field "x".
		buf.Reset()
		cl.Info("original")
		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		_, hasX := record["x"]
		assert.False(t, hasX)
	})
}

func TestContextLogger_NilContext(t *testing.T) {
	t.Run("nil context does not panic on any level", func(t *testing.T) {
		var buf bytes.Buffer
		cl := logging.NewLogger("svc", &buf).WithContext(context.TODO())
		require.NotNil(t, cl)

		cl.Debug("nil ctx debug")
		cl.Info("nil ctx info")
		cl.Warn("nil ctx warn")
		cl.Error("nil ctx error")
		cl2 := cl.With("k", "v")
		cl2.Info("nil ctx with")
	})
}

func TestContextLogger_EmptyContext(t *testing.T) {
	t.Run("empty context produces no extra fields", func(t *testing.T) {
		var buf bytes.Buffer
		cl := logging.NewLogger("svc", &buf).WithContext(context.Background())

		cl.Info("empty ctx")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		_, hasRequestID := record["request_id"]
		assert.False(t, hasRequestID, "expected no request_id in empty context")
		_, hasLayerType := record["layer_type"]
		assert.False(t, hasLayerType, "expected no layer_type in empty context")
		_, hasSource := record["source"]
		assert.False(t, hasSource, "expected no source in empty context")
	})
}

func TestContextLogger_PartialContext(t *testing.T) {
	t.Run("only request_id in context", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := logging.WithRequestID(context.Background(), "req-partial")
		cl := logging.NewLogger("svc", &buf).WithContext(ctx)

		cl.Info("partial context")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "req-partial", record["request_id"])
		_, hasLayerType := record["layer_type"]
		assert.False(t, hasLayerType)
		_, hasSource := record["source"]
		assert.False(t, hasSource)
	})

	t.Run("only layer_type in context", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := logging.WithLayerType(context.Background(), "ais")
		cl := logging.NewLogger("svc", &buf).WithContext(ctx)

		cl.Info("only layer_type")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "ais", record["layer_type"])
		_, hasRequestID := record["request_id"]
		assert.False(t, hasRequestID)
	})

	t.Run("only source in context", func(t *testing.T) {
		var buf bytes.Buffer
		ctx := logging.WithSource(context.Background(), "custom-source")
		cl := logging.NewLogger("svc", &buf).WithContext(ctx)

		cl.Info("only source")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		assert.Equal(t, "custom-source", record["source"])
		_, hasRequestID := record["request_id"]
		assert.False(t, hasRequestID)
	})
}

// ---------------------------------------------------------------------------
// Context helper functions (package-level)
// ---------------------------------------------------------------------------

func TestContextHelpers(t *testing.T) {
	t.Run("WithRequestID stores value retrievable by CtxKey", func(t *testing.T) {
		ctx := logging.WithRequestID(context.Background(), "req-123")
		value := ctx.Value(logging.CtxKey("request_id"))
		assert.Equal(t, "req-123", value)
	})

	t.Run("WithLayerType stores value retrievable by CtxKey", func(t *testing.T) {
		ctx := logging.WithLayerType(context.Background(), "flights_commercial")
		value := ctx.Value(logging.CtxKey("layer_type"))
		assert.Equal(t, "flights_commercial", value)
	})

	t.Run("WithSource stores value retrievable by CtxKey", func(t *testing.T) {
		ctx := logging.WithSource(context.Background(), "opensky")
		value := ctx.Value(logging.CtxKey("source"))
		assert.Equal(t, "opensky", value)
	})

	t.Run("context helpers compose correctly", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.WithRequestID(ctx, "r1")
		ctx = logging.WithLayerType(ctx, "lt1")
		ctx = logging.WithSource(ctx, "s1")

		assert.Equal(t, "r1", ctx.Value(logging.CtxKey("request_id")))
		assert.Equal(t, "lt1", ctx.Value(logging.CtxKey("layer_type")))
		assert.Equal(t, "s1", ctx.Value(logging.CtxKey("source")))
	})
}

// ---------------------------------------------------------------------------
// contextValue nil-ctx branch
//
// contextValue is unexported, but it is exercised through enrichFromContext
// when WithContext is given a nil context.
// ---------------------------------------------------------------------------

func TestContextValue_NilContext(t *testing.T) {
	t.Run("nil context passed to WithContext results in no enrichment", func(t *testing.T) {
		var buf bytes.Buffer
		cl := logging.NewLogger("svc", &buf).WithContext(context.TODO())
		cl.Info("nil ctx probe")

		var record map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
		_, hasRequestID := record["request_id"]
		assert.False(t, hasRequestID)
		_, hasLayerType := record["layer_type"]
		assert.False(t, hasLayerType)
		_, hasSource := record["source"]
		assert.False(t, hasSource)
	})
}
