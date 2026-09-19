package declarative

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

// sciNotationRe matches scientific notation numbers that may be embedded in a
// larger string. CEL's builtin string() uses Go's %g format which always
// includes an explicit sign after the exponent (e.g. "2.58187e+08", "3e+08").
// Requiring [+-] after e/E avoids false positives on hex-like tokens.
var sciNotationRe = regexp.MustCompile(`-?\d+(?:\.\d+)?[eE][+-]\d+`)

// normalizeScientificNotation finds scientific notation numbers anywhere in the
// string and converts whole-number ones to integer form. This handles both
// pure scientific notation ("2.58187e+08" → "258187000") and embedded cases
// produced by CEL string concatenation ("bolt_1.77e+18" → "bolt_1775111450049258240").
func normalizeScientificNotation(s string) string {
	if !strings.ContainsAny(s, "eE") {
		return s
	}
	return sciNotationRe.ReplaceAllStringFunc(s, func(match string) string {
		f, err := strconv.ParseFloat(match, 64)
		if err != nil {
			return match
		}
		if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	})
}

// formatFloat converts a float64 to its cleanest string representation.
// Whole numbers are formatted as integers (e.g. 258187000 → "258187000")
// instead of scientific notation (which %g would produce: "2.58187e+08").
// Fractional values use fixed-point with no trailing zeros.
func formatFloat(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// evalString evaluates a compiled CEL program and returns a string result.
// Enforces maxLen by truncating. Returns a descriptive error on type mismatch.
func evalString(prg cel.Program, activation map[string]interface{}, maxLen int) (string, error) {
	out, _, err := prg.Eval(activation)
	if err != nil {
		return "", fmt.Errorf("CEL eval error: %w", err)
	}

	var s string
	switch v := out.Value().(type) {
	case string:
		// CEL's builtin string() uses %g for doubles, producing scientific
		// notation for large whole numbers (e.g. 258187000 → "2.58187e+08").
		// Normalize these back to integer form since they're typically IDs
		// (MMSI, sensor_index) that must be stored as clean integers.
		s = normalizeScientificNotation(v)
	case int64:
		s = fmt.Sprintf("%d", v)
	case float64:
		s = formatFloat(v)
	case bool:
		s = fmt.Sprintf("%t", v)
	default:
		// Try the CEL ConvertToType path for other types.
		converted := out.ConvertToType(types.StringType)
		if types.IsError(converted) {
			return "", fmt.Errorf("CEL eval: cannot convert %T to string", out.Value())
		}
		s = converted.Value().(string)
	}

	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
	}
	return s, nil
}

// evalFloat evaluates a compiled CEL program and returns a float64 result.
// Handles int64 -> float64 coercion (common with CEL DynType).
func evalFloat(prg cel.Program, activation map[string]interface{}) (float64, error) {
	out, _, err := prg.Eval(activation)
	if err != nil {
		return 0, fmt.Errorf("CEL eval error: %w", err)
	}

	switch v := out.Value().(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case string:
		// Some APIs return numeric strings; attempt parse.
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
			return 0, fmt.Errorf("CEL eval: cannot parse string %q as float64", v)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("CEL eval: expected numeric type, got %T", out.Value())
	}
}

// evalBool evaluates a compiled CEL program and returns a bool result.
func evalBool(prg cel.Program, activation map[string]interface{}) (bool, error) {
	out, _, err := prg.Eval(activation)
	if err != nil {
		return false, fmt.Errorf("CEL eval error: %w", err)
	}

	b, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL eval: expected bool, got %T", out.Value())
	}
	return b, nil
}

// evalTimestamp evaluates a compiled CEL program and returns a time.Time result.
// Handles both native CEL Timestamp types and string-based timestamp results.
func evalTimestamp(prg cel.Program, activation map[string]interface{}) (time.Time, error) {
	out, _, err := prg.Eval(activation)
	if err != nil {
		return time.Time{}, fmt.Errorf("CEL eval error: %w", err)
	}

	switch v := out.Value().(type) {
	case time.Time:
		return v, nil
	case string:
		// Try RFC3339 parse as a fallback.
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, fmt.Errorf("CEL eval: cannot parse string %q as timestamp: %w", v, err)
		}
		return t, nil
	case int64:
		// Treat as epoch milliseconds.
		return time.UnixMilli(v), nil
	case float64:
		// Treat as epoch milliseconds.
		return time.UnixMilli(int64(v)), nil
	default:
		return time.Time{}, fmt.Errorf("CEL eval: expected timestamp, got %T", out.Value())
	}
}

// evalFieldMappingToString evaluates a CEL program and converts the result to a string
// based on the expected type in the FieldMapping.
func evalFieldMappingToString(prg cel.Program, typ string, activation map[string]interface{}, maxLen int) (string, error) {
	switch typ {
	case "timestamp":
		ts, err := evalTimestamp(prg, activation)
		if err != nil {
			return "", err
		}
		return ts.Format(time.RFC3339), nil
	case "integer":
		val, err := evalFloat(prg, activation)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", int64(val)), nil
	case "float":
		val, err := evalFloat(prg, activation)
		if err != nil {
			return "", err
		}
		return formatFloat(val), nil
	case "boolean":
		val, err := evalBool(prg, activation)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%t", val), nil
	default:
		// "string" or unknown
		return evalString(prg, activation, maxLen)
	}
}
