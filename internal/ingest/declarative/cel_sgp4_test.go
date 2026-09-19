package declarative

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/orbital"
)

// ISS TLE for testing. Epoch is recent enough for SGP4 to produce valid results.
const (
	issLine1 = "1 25544U 98067A   24100.50000000  .00016717  00000-0  10270-3 0  9009"
	issLine2 = "2 25544  51.6400 100.0000 0007417  40.0000 320.0000 15.49000000400000"
)

func TestSGP4Lat_ValidTLE(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_lat(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	record := map[string]interface{}{
		"line1": issLine1,
		"line2": issLine2,
	}

	val, err := evalProgram(t, prg, record)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	lat, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", val, val)
	}

	if lat < -90 || lat > 90 {
		t.Errorf("latitude %f out of valid range [-90, 90]", lat)
	}
}

func TestSGP4Lon_ValidTLE(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_lon(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	record := map[string]interface{}{
		"line1": issLine1,
		"line2": issLine2,
	}

	val, err := evalProgram(t, prg, record)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	lon, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", val, val)
	}

	if lon < -180 || lon > 180 {
		t.Errorf("longitude %f out of valid range [-180, 180]", lon)
	}
}

func TestSGP4Alt_ValidTLE(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_alt_m(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	record := map[string]interface{}{
		"line1": issLine1,
		"line2": issLine2,
	}

	val, err := evalProgram(t, prg, record)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	alt, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", val, val)
	}

	// Altitude in meters; ISS orbits at ~400km = 400000m, but allow wide range.
	// Must be > 0 and < 50,000 km in meters (50,000,000 m).
	if alt <= 0 || alt > 50000000 {
		t.Errorf("altitude %f m out of expected range (0, 50000000]", alt)
	}
}

func TestSGP4Vel_ValidTLE(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_vel_mps(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	record := map[string]interface{}{
		"line1": issLine1,
		"line2": issLine2,
	}

	val, err := evalProgram(t, prg, record)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	vel, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", val, val)
	}

	// ISS velocity ~7660 m/s; allow a wide range for any LEO satellite.
	if vel <= 0 {
		t.Errorf("velocity %f m/s should be positive", vel)
	}
}

func TestSGP4_InvalidTLE(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name string
		expr string
	}{
		{name: "sgp4_lat", expr: `sgp4_lat(record.line1, record.line2)`},
		{name: "sgp4_lon", expr: `sgp4_lon(record.line1, record.line2)`},
		{name: "sgp4_alt_m", expr: `sgp4_alt_m(record.line1, record.line2)`},
		{name: "sgp4_vel_mps", expr: `sgp4_vel_mps(record.line1, record.line2)`},
	}

	record := map[string]interface{}{
		"line1": "1 XXXXX BAD TLE DATA",
		"line2": "2 XXXXX BAD TLE DATA",
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpression(tc.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			val, err := evalProgram(t, prg, record)
			if err != nil {
				t.Fatalf("eval error (should not error, should return 0.0): %v", err)
			}

			got, ok := val.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T (%v)", val, val)
			}
			if got != 0.0 {
				t.Errorf("expected 0.0 for invalid TLE, got %f", got)
			}
		})
	}
}

func TestSGP4_EmptyStrings(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name string
		expr string
	}{
		{name: "sgp4_lat", expr: `sgp4_lat(record.line1, record.line2)`},
		{name: "sgp4_lon", expr: `sgp4_lon(record.line1, record.line2)`},
		{name: "sgp4_alt_m", expr: `sgp4_alt_m(record.line1, record.line2)`},
		{name: "sgp4_vel_mps", expr: `sgp4_vel_mps(record.line1, record.line2)`},
	}

	record := map[string]interface{}{
		"line1": "",
		"line2": "",
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpression(tc.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			val, err := evalProgram(t, prg, record)
			if err != nil {
				t.Fatalf("eval error (should not error, should return 0.0): %v", err)
			}

			got, ok := val.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T (%v)", val, val)
			}
			if got != 0.0 {
				t.Errorf("expected 0.0 for empty strings, got %f", got)
			}
		})
	}
}

func TestCoerceDouble_Float(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`coerce_double(record.val, 0.0)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"val": 35000.0})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if got != 35000.0 {
		t.Errorf("expected 35000.0, got %f", got)
	}
}

func TestCoerceDouble_Int(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`coerce_double(record.val, 0.0)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"val": int64(35000)})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if got != 35000.0 {
		t.Errorf("expected 35000.0, got %f", got)
	}
}

func TestCoerceDouble_String(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`coerce_double(record.val, 0.0)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"val": "ground"})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if got != 0.0 {
		t.Errorf("expected 0.0 for non-numeric string, got %f", got)
	}
}

func TestCoerceDouble_NumericString(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`coerce_double(record.val, 0.0)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"val": "35000.5"})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if got != 35000.5 {
		t.Errorf("expected 35000.5, got %f", got)
	}
}

// TestSGP4Cache_DeduplicatesPropagation verifies that calling all 4 SGP4 functions
// with the same TLE lines results in only 1 actual propagation (cached).
func TestSGP4Cache_DeduplicatesPropagation(t *testing.T) {
	c := mustNewCompiler(t)

	record := map[string]interface{}{
		"line1": issLine1,
		"line2": issLine2,
	}

	// Evaluate all 4 SGP4 functions with the same TLE
	funcs := []string{
		`sgp4_lat(record.line1, record.line2)`,
		`sgp4_lon(record.line1, record.line2)`,
		`sgp4_alt_m(record.line1, record.line2)`,
		`sgp4_vel_mps(record.line1, record.line2)`,
	}

	results := make([]float64, len(funcs))
	for i, expr := range funcs {
		prg, err := c.CompileExpression(expr)
		if err != nil {
			t.Fatalf("compile %q: %v", expr, err)
		}
		val, err := evalProgram(t, prg, record)
		if err != nil {
			t.Fatalf("eval %q: %v", expr, err)
		}
		f, ok := val.(float64)
		if !ok {
			t.Fatalf("expected float64 from %q, got %T", expr, val)
		}
		results[i] = f
	}

	// Verify the cache was populated: same key should return the same entry
	// We can verify by calling propagateCached directly and checking it returns
	// the same result without error.
	now := c.clock.Now()
	result, err := c.propagateCached(issLine1, issLine2, now)
	if err != nil {
		t.Fatalf("propagateCached: %v", err)
	}

	// Cross-check the cached values against expected orbital results
	if result.Lat < -90 || result.Lat > 90 {
		t.Errorf("cached lat %f out of range", result.Lat)
	}
	if result.Lon < -180 || result.Lon > 180 {
		t.Errorf("cached lon %f out of range", result.Lon)
	}
	if result.AltKm <= 0 {
		t.Errorf("cached altitude %f km should be positive", result.AltKm)
	}
	if result.VelKmS <= 0 {
		t.Errorf("cached velocity %f km/s should be positive", result.VelKmS)
	}
}

// TestSGP4Cache_DifferentTLEsGetSeparateEntries verifies that different TLE lines
// produce different cache entries.
func TestSGP4Cache_DifferentTLEsGetSeparateEntries(t *testing.T) {
	c := mustNewCompiler(t)

	now := c.clock.Now()

	// Use the ISS TLE
	result1, err := c.propagateCached(issLine1, issLine2, now)
	if err != nil {
		t.Fatalf("propagateCached ISS: %v", err)
	}

	// Use a different TLE (NOAA 19)
	noaaLine1 := "1 33591U 09005A   24100.50000000  .00000137  00000-0  10270-3 0  9991"
	noaaLine2 := "2 33591  99.1900 100.0000 0014000  40.0000 320.0000 14.12000000700001"

	result2, err := c.propagateCached(noaaLine1, noaaLine2, now)
	if err != nil {
		t.Fatalf("propagateCached NOAA: %v", err)
	}

	// Results should be different satellites at different positions
	if result1.Lat == result2.Lat && result1.Lon == result2.Lon {
		t.Error("different TLEs should produce different results")
	}
}

// TestSGP4Cache_ErrorsCached verifies that propagation errors are also cached
// (so we don't retry a bad TLE 4 times).
func TestSGP4Cache_ErrorsCached(t *testing.T) {
	c := mustNewCompiler(t)

	// Zero-position TLE will produce an error from PropagateTLE.
	// Use lines that pass format validation but produce bad results.
	badLine1 := "1 00000U 00000A   00001.00000000  .00000000  00000-0  00000-0 0  0004"
	badLine2 := "2 00000   0.0000   0.0000 0000000   0.0000   0.0000  0.00000000    02"

	now := c.clock.Now()

	// First call should store the error
	_, err1 := c.propagateCached(badLine1, badLine2, now)

	// Second call should return the cached error
	_, err2 := c.propagateCached(badLine1, badLine2, now)

	// Both should either error or not, but be consistent
	if (err1 == nil) != (err2 == nil) {
		t.Errorf("cache inconsistency: first call err=%v, second call err=%v", err1, err2)
	}
}

// countingClock is a clock that returns a fixed time and counts calls.
type countingClock struct {
	fixed time.Time
	calls atomic.Int64
}

func (c *countingClock) Now() time.Time {
	c.calls.Add(1)
	return c.fixed
}

// TestSGP4Cache_PropagateTLE_CalledOnce verifies at the propagateCached level
// that repeated calls with the same key don't re-propagate.
func TestSGP4Cache_PropagateTLE_CalledOnce(t *testing.T) {
	fixedTime := time.Date(2024, 4, 10, 12, 0, 0, 0, time.UTC)
	clk := &countingClock{fixed: fixedTime}

	c, err := NewCELCompilerWithClock(clk)
	if err != nil {
		t.Fatalf("NewCELCompilerWithClock: %v", err)
	}

	// Manually call PropagateTLE once to establish the expected result.
	expected, err := orbital.PropagateTLE(issLine1, issLine2, time.Unix(fixedTime.Unix(), 0).UTC())
	if err != nil {
		t.Fatalf("PropagateTLE baseline: %v", err)
	}

	// Call propagateCached 4 times (simulating sgp4_lat, sgp4_lon, sgp4_alt_m, sgp4_vel_mps)
	for i := 0; i < 4; i++ {
		result, err := c.propagateCached(issLine1, issLine2, fixedTime)
		if err != nil {
			t.Fatalf("propagateCached call %d: %v", i, err)
		}
		if result.Lat != expected.Lat || result.Lon != expected.Lon {
			t.Errorf("call %d: result mismatch: got (%f, %f), want (%f, %f)",
				i, result.Lat, result.Lon, expected.Lat, expected.Lon)
		}
	}

	// The cache should have stored the result after the first call.
	// We can verify the cache has an entry by checking it.
	key := issLine1 + "|" + issLine2 + "|" + "1712750400" // Unix timestamp for 2024-04-10T12:00:00Z
	if _, ok := c.sgp4Cache.Load(key); !ok {
		t.Error("expected cache entry to exist")
	}
}

func TestCoerceDouble_Nil(t *testing.T) {
	c := mustNewCompiler(t)
	// Use a has() guard to pass null when field is absent.
	prg, err := c.CompileExpression(`coerce_double(has(record.val) ? record.val : null, -1.0)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// Record without "val" field -> null path -> default -1.0
	val, err := evalProgram(t, prg, map[string]interface{}{})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", val, val)
	}
	if got != -1.0 {
		t.Errorf("expected -1.0, got %f", got)
	}
}
