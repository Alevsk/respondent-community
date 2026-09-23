package declarative

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"

	"github.com/Alevsk/respondent/internal/orbital"
)

// maxExpressionSize is the maximum allowed CEL expression size in bytes.
// Mitigates ANTLR memory issues with adversarial inputs.
const maxExpressionSize = 4096

// celCostLimit is the runtime cost budget for CEL programs.
// Prevents ReDoS via matches() and expensive list operations.
const celCostLimit = 10000

// tleFormatValid performs minimal TLE format validation to prevent the go-satellite
// library from calling log.Fatal (os.Exit) on malformed input. The library panics
// on parse errors that recover() cannot catch, so we must reject obviously bad TLEs
// before calling PropagateTLE.
func tleFormatValid(line1, line2 string) bool {
	if len(line1) < 69 || len(line2) < 69 {
		return false
	}
	if line1[0] != '1' || line2[0] != '2' {
		return false
	}
	return true
}

// Clock provides time for testability.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// sgp4CacheEntry holds a cached SGP4 propagation result.
type sgp4CacheEntry struct {
	result orbital.PropagateResult
	err    error
}

// CELCompiler manages CEL environment creation and program compilation.
// It follows the Kubernetes pattern: parse+check at config time, evaluate at runtime.
// The baseEnv is safe for concurrent use; no mutex is needed.
//
// sgp4Cache eliminates redundant SGP4 propagation calls within a single evaluation
// cycle. For each satellite record, sgp4_lat/sgp4_lon/sgp4_alt_m/sgp4_vel_mps all
// call PropagateTLE with identical inputs. The cache deduplicates these to 1 call
// per unique (line1, line2, second) tuple. Entries naturally expire because the cache
// key includes the Unix timestamp truncated to the current second.
type CELCompiler struct {
	baseEnv   *cel.Env
	clock     Clock
	sgp4Cache sync.Map // key: "line1|line2|unixSec" -> *sgp4CacheEntry
}

// NewCELCompiler creates a compiler with the standard Respondent CEL environment.
// SECURITY: No env(), no I/O, no filesystem, no network access.
// The CEL environment ONLY sees the `record` variable (API response data).
func NewCELCompiler() (*CELCompiler, error) {
	return NewCELCompilerWithClock(realClock{})
}

// NewCELCompilerWithClock creates a compiler with a custom clock for testability.
func NewCELCompilerWithClock(clock Clock) (*CELCompiler, error) {
	if clock == nil {
		clock = realClock{}
	}

	// Create the compiler first so SGP4 closures can reference its cache.
	compiler := &CELCompiler{clock: clock}

	env, err := cel.NewEnv(
		// The record variable: a map representing one parsed record from the API response.
		// Using cel.DynType allows any JSON structure without pre-declaring the schema.
		// Tradeoff: field access errors surface at runtime, not compile time.
		cel.Variable("record", cel.DynType),

		// Standard extensions (no Encoders -- removes base64 covert exfil vector)
		ext.Strings(),
		ext.Math(),

		// Custom function: now() -> current timestamp
		cel.Function("now",
			cel.Overload("now_timestamp",
				[]*cel.Type{},
				cel.TimestampType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.Timestamp{Time: clock.Now()}
				}),
			),
		),

		// Custom function: unix_ms(int) -> timestamp
		// Most APIs return epoch milliseconds; this custom overload handles that.
		cel.Function("unix_ms",
			cel.Overload("unix_ms_to_timestamp",
				[]*cel.Type{cel.DynType},
				cel.TimestampType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					switch v := val.Value().(type) {
					case float64:
						return types.Timestamp{Time: time.UnixMilli(int64(v))}
					case int64:
						return types.Timestamp{Time: time.UnixMilli(v)}
					default:
						return types.NewErr("unix_ms: expected number, got %T", val.Value())
					}
				}),
			),
		),

		// Custom function: unix_s(int) -> timestamp (epoch seconds)
		cel.Function("unix_s",
			cel.Overload("unix_s_to_timestamp",
				[]*cel.Type{cel.DynType},
				cel.TimestampType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					switch v := val.Value().(type) {
					case float64:
						return types.Timestamp{Time: time.Unix(int64(v), 0)}
					case int64:
						return types.Timestamp{Time: time.Unix(v, 0)}
					default:
						return types.NewErr("unix_s: expected number, got %T", val.Value())
					}
				}),
			),
		),

		// Custom function: parse_rfc3339(string) -> timestamp
		cel.Function("parse_rfc3339",
			cel.Overload("parse_rfc3339_timestamp",
				[]*cel.Type{cel.StringType},
				cel.TimestampType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					s, ok := val.Value().(string)
					if !ok {
						return types.NewErr("parse_rfc3339: expected string, got %T", val.Value())
					}
					t, err := time.Parse(time.RFC3339, s)
					if err != nil {
						return types.NewErr("parse_rfc3339: %v", err)
					}
					return types.Timestamp{Time: t}
				}),
			),
		),

		// Custom function: sgp4_lat(line1, line2) -> double
		// Propagates a TLE to time.Now() using SGP4 and returns latitude in degrees.
		// Returns 0.0 on any error (malformed TLE, decayed orbit, etc.).
		// Results are cached per (line1, line2, second) to avoid redundant propagation.
		cel.Function("sgp4_lat",
			cel.Overload("sgp4_lat_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					line1, ok1 := lhs.Value().(string)
					line2, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 || !tleFormatValid(line1, line2) {
						return types.Double(0)
					}
					result, err := compiler.propagateCached(line1, line2, clock.Now())
					if err != nil {
						return types.Double(0)
					}
					return types.Double(result.Lat)
				}),
			),
		),

		// Custom function: sgp4_lon(line1, line2) -> double
		// Propagates a TLE to time.Now() using SGP4 and returns longitude in degrees.
		// Returns 0.0 on any error. Results are cached.
		cel.Function("sgp4_lon",
			cel.Overload("sgp4_lon_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					line1, ok1 := lhs.Value().(string)
					line2, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 || !tleFormatValid(line1, line2) {
						return types.Double(0)
					}
					result, err := compiler.propagateCached(line1, line2, clock.Now())
					if err != nil {
						return types.Double(0)
					}
					return types.Double(result.Lon)
				}),
			),
		),

		// Custom function: sgp4_alt_m(line1, line2) -> double
		// Propagates a TLE to time.Now() using SGP4 and returns altitude in meters.
		// Returns 0.0 on any error. Results are cached.
		cel.Function("sgp4_alt_m",
			cel.Overload("sgp4_alt_m_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					line1, ok1 := lhs.Value().(string)
					line2, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 || !tleFormatValid(line1, line2) {
						return types.Double(0)
					}
					result, err := compiler.propagateCached(line1, line2, clock.Now())
					if err != nil {
						return types.Double(0)
					}
					return types.Double(result.AltKm * 1000)
				}),
			),
		),

		// Custom function: sgp4_vel_mps(line1, line2) -> double
		// Propagates a TLE to time.Now() using SGP4 and returns velocity in m/s.
		// Returns 0.0 on any error. Results are cached.
		cel.Function("sgp4_vel_mps",
			cel.Overload("sgp4_vel_mps_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					line1, ok1 := lhs.Value().(string)
					line2, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 || !tleFormatValid(line1, line2) {
						return types.Double(0)
					}
					result, err := compiler.propagateCached(line1, line2, clock.Now())
					if err != nil {
						return types.Double(0)
					}
					return types.Double(result.VelKmS * 1000)
				}),
			),
		),

		// Custom function: parse_datetime(string, layout) -> timestamp
		// Uses Go's time.Parse(layout, value) under the hood.
		// The layout parameter accepts Go time layout strings (e.g., "2006-01-02T15:04:05").
		cel.Function("parse_datetime",
			cel.Overload("parse_datetime_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.TimestampType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					value, ok1 := lhs.Value().(string)
					layout, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 {
						return types.NewErr("parse_datetime: expected (string, string), got (%T, %T)", lhs.Value(), rhs.Value())
					}
					t, err := time.Parse(layout, value)
					if err != nil {
						return types.NewErr("parse_datetime: %v", err)
					}
					return types.Timestamp{Time: t}
				}),
			),
		),

		// Custom function: parse_iso8601(string) -> timestamp
		// Convenience wrapper for ISO 8601 strings that may lack a timezone suffix.
		// Appends "Z" (UTC) if no timezone info is present, then parses as RFC3339.
		// Use case: GDACS returns "2026-03-11T07:55:18" without timezone.
		cel.Function("parse_iso8601",
			cel.Overload("parse_iso8601_timestamp",
				[]*cel.Type{cel.StringType},
				cel.TimestampType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					s, ok := val.Value().(string)
					if !ok {
						return types.NewErr("parse_iso8601: expected string, got %T", val.Value())
					}
					// If the string has no timezone info (+, Z, or trailing offset), append "Z".
					if !strings.ContainsAny(s, "Zz+") || (strings.Count(s, "-") <= 2 && !strings.ContainsAny(s, "Zz+")) {
						// More precise check: if no "Z", "z", or "+" appears after the time portion,
						// and there's no negative offset after the time (e.g., "-05:00"),
						// then it lacks timezone info.
						hasTimezone := false
						for i := len(s) - 1; i >= 0 && i >= len(s)-6; i-- {
							if s[i] == 'Z' || s[i] == 'z' || s[i] == '+' {
								hasTimezone = true
								break
							}
							// A '-' after position 10 (past the date part) indicates a negative UTC offset.
							if s[i] == '-' && i > 10 {
								hasTimezone = true
								break
							}
						}
						if !hasTimezone {
							s += "Z"
						}
					}
					t, err := time.Parse(time.RFC3339, s)
					if err != nil {
						return types.NewErr("parse_iso8601: %v", err)
					}
					return types.Timestamp{Time: t}
				}),
			),
		),

		// Custom function: parse_rfc2822(string) -> timestamp
		// Parses RFC 2822 / email date strings used in RSS feeds (e.g., "Mon, 02 Jan 2006 15:04:05 -0700").
		// Tries RFC1123Z (numeric timezone offset) first, then RFC1123 (named timezone abbreviation).
		// Use case: RSS <pubDate> and <lastBuildDate> fields.
		cel.Function("parse_rfc2822",
			cel.Overload("parse_rfc2822_timestamp",
				[]*cel.Type{cel.StringType},
				cel.TimestampType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					s, ok := val.Value().(string)
					if !ok {
						return types.NewErr("parse_rfc2822: expected string, got %T", val.Value())
					}
					t, err := time.Parse(time.RFC1123Z, s)
					if err != nil {
						t, err = time.Parse(time.RFC1123, s)
					}
					if err != nil {
						return types.NewErr("parse_rfc2822: %v", err)
					}
					return types.Timestamp{Time: t}
				}),
			),
		),

		// Custom function: coerce_double(val, default) -> double
		// Safely coerces mixed-type fields (float64, int64, string, nil) to double.
		// Use case: APIs that return "ground" for altitude of grounded aircraft.
		cel.Function("coerce_double",
			cel.Overload("coerce_double_dyn_double",
				[]*cel.Type{cel.DynType, cel.DoubleType},
				cel.DoubleType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					defaultVal, ok := rhs.Value().(float64)
					if !ok {
						defaultVal = 0
					}

					val := lhs.Value()
					if val == nil {
						return types.Double(defaultVal)
					}

					switch v := val.(type) {
					case float64:
						return types.Double(v)
					case int64:
						return types.Double(float64(v))
					case string:
						f, err := strconv.ParseFloat(v, 64)
						if err != nil {
							return types.Double(defaultVal)
						}
						return types.Double(f)
					default:
						return types.Double(defaultVal)
					}
				}),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create CEL environment: %w", err)
	}

	compiler.baseEnv = env
	return compiler, nil
}

// Clock returns the clock used by this compiler's CEL environment.
func (c *CELCompiler) Clock() Clock {
	return c.clock
}

// propagateCached returns a cached SGP4 propagation result for the given TLE lines
// and time. The cache key includes the Unix second so entries are valid for at most
// one second. This eliminates 4x redundant propagation per satellite record when
// sgp4_lat, sgp4_lon, sgp4_alt_m, and sgp4_vel_mps are all evaluated.
func (c *CELCompiler) propagateCached(line1, line2 string, t time.Time) (orbital.PropagateResult, error) {
	unixSec := t.Unix()
	key := line1 + "|" + line2 + "|" + strconv.FormatInt(unixSec, 10)

	if cached, ok := c.sgp4Cache.Load(key); ok {
		entry := cached.(*sgp4CacheEntry)
		return entry.result, entry.err
	}

	// Truncate to the second boundary for determinism within the cache window.
	tTrunc := time.Unix(unixSec, 0).UTC()
	result, err := orbital.PropagateTLE(line1, line2, tTrunc)

	entry := &sgp4CacheEntry{result: result, err: err}
	c.sgp4Cache.Store(key, entry)
	return result, err
}

// CompileExpression parses and type-checks a CEL expression.
// Returns a compiled Program ready for repeated evaluation.
// This MUST be called at config load time (control plane), not at runtime.
//
// Security: enforces a 4KB expression size limit and a runtime cost budget.
func (c *CELCompiler) CompileExpression(expr string) (cel.Program, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("CEL expression exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	ast, issues := c.baseEnv.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compile error: %w", issues.Err())
	}

	// Runtime cost limit prevents ReDoS via matches() and expensive list operations.
	prg, err := c.baseEnv.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(celCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL program error: %w", err)
	}
	return prg, nil
}

// CompileExpressionWithLookups compiles a CEL expression in an environment
// that includes lookup functions bound to the given tables.
// Used for sources that define lookup_tables; creates a per-source extended environment.
func (c *CELCompiler) CompileExpressionWithLookups(expr string, tables map[string]*LookupTable) (cel.Program, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("CEL expression exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	// Extend the base environment with lookup functions bound to this source's tables.
	env, err := c.baseEnv.Extend(
		cel.Function("lookup",
			cel.Overload("lookup_string_string_string",
				[]*cel.Type{cel.StringType, cel.StringType, cel.StringType},
				cel.DynType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					tableName, ok1 := args[0].Value().(string)
					key, ok2 := args[1].Value().(string)
					field, ok3 := args[2].Value().(string)
					if !ok1 || !ok2 || !ok3 {
						return types.NewErr("lookup: expected (string, string, string), got (%T, %T, %T)", args[0].Value(), args[1].Value(), args[2].Value())
					}
					return lookupValue(tables, tableName, key, field)
				}),
			),
		),
		cel.Function("has_lookup",
			cel.Overload("has_lookup_string_string",
				[]*cel.Type{cel.StringType, cel.StringType},
				cel.BoolType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					tableName, ok1 := lhs.Value().(string)
					key, ok2 := rhs.Value().(string)
					if !ok1 || !ok2 {
						return types.Bool(false)
					}
					return hasLookupValue(tables, tableName, key)
				}),
			),
		),
		cel.Function("lookup_or",
			cel.Overload("lookup_or_string_string_string_dyn",
				[]*cel.Type{cel.StringType, cel.StringType, cel.StringType, cel.DynType},
				cel.DynType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					tableName, ok1 := args[0].Value().(string)
					key, ok2 := args[1].Value().(string)
					field, ok3 := args[2].Value().(string)
					defaultVal := args[3]
					if !ok1 || !ok2 || !ok3 {
						return defaultVal
					}
					return lookupValueOr(tables, tableName, key, field, defaultVal)
				}),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("extend CEL environment with lookups: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compile error: %w", issues.Err())
	}

	prg, err := env.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(celCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL program error: %w", err)
	}
	return prg, nil
}

// lookupValue retrieves a field from a lookup table entry.
func lookupValue(tables map[string]*LookupTable, tableName, key, field string) ref.Val {
	table, ok := tables[tableName]
	if !ok {
		return types.NewErr("lookup: unknown table %q", tableName)
	}
	entry, ok := table.Data[key]
	if !ok {
		return types.NewErr("lookup: key %q not found in table %q", key, tableName)
	}
	val, ok := entry[field]
	if !ok {
		return types.NewErr("lookup: field %q not found in table %q for key %q", field, tableName, key)
	}
	return types.DefaultTypeAdapter.NativeToValue(val)
}

// hasLookupValue checks if a key exists in a lookup table.
func hasLookupValue(tables map[string]*LookupTable, tableName, key string) ref.Val {
	table, ok := tables[tableName]
	if !ok {
		return types.Bool(false)
	}
	_, ok = table.Data[key]
	return types.Bool(ok)
}

// lookupValueOr retrieves a field, returning a default if the key is not found.
func lookupValueOr(tables map[string]*LookupTable, tableName, key, field string, defaultVal ref.Val) ref.Val {
	table, ok := tables[tableName]
	if !ok {
		return defaultVal
	}
	entry, ok := table.Data[key]
	if !ok {
		return defaultVal
	}
	val, ok := entry[field]
	if !ok {
		return defaultVal
	}
	return types.DefaultTypeAdapter.NativeToValue(val)
}

// CompileMediaActionPath compiles a media_actions path expression. It uses a
// dedicated environment exposing only `metadata` (the entity's merged string
// map), so an action path can never read a record, a header or a secret.
func (c *CELCompiler) CompileMediaActionPath(expr string) (cel.Program, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("CEL expression exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	env, err := cel.NewEnv(
		cel.Variable("metadata", cel.MapType(cel.StringType, cel.StringType)),
		ext.Strings(),
	)
	if err != nil {
		return nil, fmt.Errorf("create media action CEL environment: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compile error (media action path): %w", issues.Err())
	}
	if ast.OutputType() != cel.StringType {
		return nil, fmt.Errorf("media action path must evaluate to a string, got %s", ast.OutputType())
	}

	prg, err := env.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(celCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL program error (media action path): %w", err)
	}
	return prg, nil
}

// CompileStopWhen compiles a CEL expression for pagination stop_when evaluation.
// Unlike CompileExpression, this uses a separate environment with `records` (list)
// variable instead of `record` (map).
func (c *CELCompiler) CompileStopWhen(expr string) (cel.Program, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("CEL expression exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	// Create a separate environment with `records` as a list variable.
	env, err := cel.NewEnv(
		cel.Variable("records", cel.ListType(cel.DynType)),
		ext.Strings(),
		ext.Math(),
	)
	if err != nil {
		return nil, fmt.Errorf("create stop_when CEL environment: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compile error (stop_when): %w", issues.Err())
	}

	prg, err := env.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(celCostLimit),
	)
	if err != nil {
		return nil, fmt.Errorf("CEL program error (stop_when): %w", err)
	}
	return prg, nil
}

type CompiledFieldMapping struct {
	Program cel.Program
	Target  string
	Type    string
}

// CompiledSource holds all pre-compiled CEL programs for a source definition.
// Fields are unexported; access via methods. This struct is immutable after creation.
type CompiledSource struct {
	clock                Clock
	definition           *SourceDefinition
	filter               cel.Program
	entityID             cel.Program
	entityName           cel.Program
	entityMeta           map[string]cel.Program
	observationLat       cel.Program
	observationLon       cel.Program
	observationAlt       cel.Program
	observationTS        cel.Program
	observationEventTime cel.Program
	observationEventEnd  cel.Program
	observationVelocity  map[string]cel.Program
	observationMeta      map[string]cel.Program
	contentHash          cel.Program
	stopWhen             cel.Program            // Pagination stop_when expression (uses `records` variable)
	mediaActions         map[string]cel.Program // media_actions path expressions (use `metadata` map)
	entityCacheKey       cel.Program            // CEL program for entity cache key extraction
	fieldMappings        []CompiledFieldMapping

	// Lookup tables loaded from YAML or file, indexed by table name.
	lookupTables map[string]*LookupTable

	// Resolved secrets (opaque, never passed to CEL activation)
	resolvedHeaders map[string]string

	// tokenProvider handles dynamic auth (OAuth2) or static auth (bearer/api_key).
	tokenProvider TokenProvider

	// envResolve resolves environment variable names to values.
	// Stored for transport factory construction.
	envResolve EnvResolver
}

// SourceClock returns the clock associated with this compiled source.
func (cs *CompiledSource) SourceClock() Clock {
	if cs.clock == nil {
		return realClock{}
	}
	return cs.clock
}

// Definition returns the source definition.
func (cs *CompiledSource) Definition() *SourceDefinition {
	return cs.definition
}

// Filter returns the compiled filter CEL program, or nil if no filter is defined.
func (cs *CompiledSource) Filter() cel.Program {
	return cs.filter
}

// EntityID returns the compiled entity external_id CEL program.
func (cs *CompiledSource) EntityID() cel.Program {
	return cs.entityID
}

// EntityName returns the compiled entity name CEL program.
func (cs *CompiledSource) EntityName() cel.Program {
	return cs.entityName
}

// EntityMeta returns the compiled entity metadata CEL programs.
func (cs *CompiledSource) EntityMeta() map[string]cel.Program {
	return cs.entityMeta
}

// ObservationLat returns the compiled latitude CEL program.
func (cs *CompiledSource) ObservationLat() cel.Program {
	return cs.observationLat
}

// ObservationLon returns the compiled longitude CEL program.
func (cs *CompiledSource) ObservationLon() cel.Program {
	return cs.observationLon
}

// ObservationAlt returns the compiled altitude CEL program, or nil if not defined.
func (cs *CompiledSource) ObservationAlt() cel.Program {
	return cs.observationAlt
}

// ObservationTS returns the compiled timestamp CEL program.
func (cs *CompiledSource) ObservationTS() cel.Program {
	return cs.observationTS
}

// ObservationEventTime returns the compiled event_time CEL program.
func (cs *CompiledSource) ObservationEventTime() cel.Program {
	return cs.observationEventTime
}

// ObservationEventEnd returns the compiled event_end CEL program.
func (cs *CompiledSource) ObservationEventEnd() cel.Program {
	return cs.observationEventEnd
}

// ObservationVelocity returns the compiled velocity mapping CEL programs.
func (cs *CompiledSource) ObservationVelocity() map[string]cel.Program {
	return cs.observationVelocity
}

// FieldMappings returns the compiled field mappings.
func (cs *CompiledSource) FieldMappings() []CompiledFieldMapping {
	return cs.fieldMappings
}

// ObservationMeta returns the compiled observation metadata CEL programs.
func (cs *CompiledSource) ObservationMeta() map[string]cel.Program {
	return cs.observationMeta
}

// ContentHash returns the compiled content hash CEL program, or nil if not defined.
func (cs *CompiledSource) ContentHash() cel.Program {
	return cs.contentHash
}

// StopWhen returns the compiled stop_when CEL program for pagination, or nil if not defined.
func (cs *CompiledSource) StopWhen() cel.Program {
	return cs.stopWhen
}

// MediaActions returns the compiled media_actions path expressions, keyed by
// action name. Path expressions are compiled at source load so a malformed one
// fails the source rather than a user's playback notification.
func (cs *CompiledSource) MediaActions() map[string]cel.Program {
	return cs.mediaActions
}

// LookupTables returns the compiled lookup tables indexed by name.
func (cs *CompiledSource) LookupTables() map[string]*LookupTable {
	return cs.lookupTables
}

// ResolvedHeaders returns the resolved static headers (non-auth).
// These are resolved at control-plane time and never passed to CEL evaluation.
func (cs *CompiledSource) ResolvedHeaders() map[string]string {
	return cs.resolvedHeaders
}

// TokenProvider returns the token provider for dynamic auth, or nil if static/no auth.
func (cs *CompiledSource) TokenProvider() TokenProvider {
	return cs.tokenProvider
}

// EnvResolve returns the environment variable resolver stored on this compiled source.
func (cs *CompiledSource) EnvResolve() EnvResolver {
	return cs.envResolve
}
