package orbital

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownTLEs contains well-known satellite TLEs for testing.
// All epoch-based fields use 2024 to ensure consistent propagation.
var knownTLEs = struct {
	// ISS (ZARYA) — LEO, ~400 km, 51.6° inclination
	issLine1, issLine2 string
	// Hubble Space Telescope — LEO, ~540 km, 28.5° inclination
	hstLine1, hstLine2 string
	// GOES-16 — GEO, ~35786 km, near-equatorial
	goes16Line1, goes16Line2 string
}{
	issLine1: "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009",
	issLine2: "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011",

	hstLine1: "1 20580U 90037B   24001.50000000  .00001234  00000-0  60000-4 0  9996",
	hstLine2: "2 20580  28.4700 123.4500 0002345  90.1200 270.0000 15.09257900000017",

	goes16Line1: "1 41866U 16071A   24001.50000000 -.00000134  00000-0  00000+0 0  9990",
	goes16Line2: "2 41866   0.0531 106.3264 0001416  10.3847 266.5386  1.00270494 31437",
}

// TestPropagateTLE_ZeroPosition verifies that a TLE representing a decayed
// orbit (which causes the SGP4 propagator to return a zero position vector)
// is rejected with a descriptive error. The TLE below has eccentricity 0.9990000
// encoded in the TLE format, which causes the sgp4 semilatus-rectum to go
// negative (pl < 0), leaving position at zero.
func TestPropagateTLE_ZeroPosition(t *testing.T) {
	// eccentricity field "9990000" → 0.9990000, mean motion ~1 rev/day at 90° inclination.
	// The SGP4 semilatus rectum pl = am*(1-el2) goes negative at this eccentricity,
	// so the position assignment block is skipped and the vector stays at (0,0,0).
	line1 := "1 99999U 00000A   24001.00000000  .00000000  00000-0  00000-0 0  9992"
	line2 := "2 99999  90.0000   0.0000 9990000   0.0000   0.0000  1.00000000    15"

	_, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero position")
}

// TestPropagateTLE_UnrealisticAltitude_High verifies that the altitude sanity
// check rejects orbits with altitude > 50000 km. A stale LEO TLE propagated
// 100 years into the future diverges to ~65 million km, which is physically
// impossible for a near-Earth satellite.
func TestPropagateTLE_UnrealisticAltitude_High(t *testing.T) {
	// ISS TLE from 2024 propagated to 2124 — the orbit diverges massively.
	t100yr := time.Date(2124, 1, 1, 12, 0, 0, 0, time.UTC)
	_, err := PropagateTLE(knownTLEs.issLine1, knownTLEs.issLine2, t100yr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrealistic altitude")
}

// TestPropagateTLE_UnrealisticAltitude_HighEccentricity verifies that a
// TLE with extreme eccentricity (apogee >> 50000 km) is also rejected.
// The TLE below has e=0.9, giving an apogee of ~292000 km.
func TestPropagateTLE_UnrealisticAltitude_HighEccentricity(t *testing.T) {
	// e=0.9000000, n=0.135 rev/day, M=180° (satellite at apogee).
	// Apogee altitude ≈ a*(1+e) − Re ≈ 292000 km, well above the 50000 km limit.
	line1 := "1 99998U 00000A   24001.00000000  .00000000  00000-0  00000-0 0  9991"
	line2 := "2 99998   0.0000   0.0000 9000000   0.0000 180.0000  0.13500000    14"

	_, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrealistic altitude")
}

// TestPropagateTLE_ISS_MultipleEpochs tests ISS propagation across several
// distinct epochs to ensure consistent results and cover the happy path with
// different time inputs (year/month/day/hour/min/sec branching in Propagate).
func TestPropagateTLE_ISS_MultipleEpochs(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
	}{
		{"epoch_2024_jan_midnight", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"epoch_2024_jan_noon", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
		{"epoch_2024_jun_solstice", time.Date(2024, 6, 21, 6, 30, 0, 0, time.UTC)},
		{"epoch_2024_dec_solstice", time.Date(2024, 12, 21, 18, 45, 0, 0, time.UTC)},
		{"epoch_2024_leapday", time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := PropagateTLE(knownTLEs.issLine1, knownTLEs.issLine2, tc.t)
			require.NoError(t, err)

			// ISS stays within its orbital inclination of 51.6°
			assert.GreaterOrEqual(t, result.Lat, -90.0, "latitude below -90°")
			assert.LessOrEqual(t, result.Lat, 90.0, "latitude above 90°")
			assert.InDelta(t, 0.0, result.Lat, 52.0, "ISS lat should be within inclination ±52°")

			assert.GreaterOrEqual(t, result.Lon, -180.0, "longitude below -180°")
			assert.LessOrEqual(t, result.Lon, 180.0, "longitude above 180°")

			// LEO altitude range: 350–450 km for ISS
			assert.Greater(t, result.AltKm, 350.0, "ISS altitude too low: %f km", result.AltKm)
			assert.Less(t, result.AltKm, 450.0, "ISS altitude too high: %f km", result.AltKm)

			// ISS orbital speed ≈ 7.66 km/s
			assert.InDelta(t, 7.66, result.VelKmS, 0.2, "ISS velocity out of expected range: %f km/s", result.VelKmS)
		})
	}
}

// TestPropagateTLE_HST_HubbleSpaceTelescope verifies propagation of the Hubble
// Space Telescope, which orbits at ~540 km altitude and 28.5° inclination.
func TestPropagateTLE_HST_HubbleSpaceTelescope(t *testing.T) {
	result, err := PropagateTLE(
		knownTLEs.hstLine1,
		knownTLEs.hstLine2,
		time.Date(2024, 3, 15, 9, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	// HST inclination is 28.5°, so latitude is bounded by ±28.5°
	assert.InDelta(t, 0.0, result.Lat, 29.0, "HST latitude outside inclination band")
	assert.GreaterOrEqual(t, result.Lon, -180.0)
	assert.LessOrEqual(t, result.Lon, 180.0)

	// HST altitude: ~530–550 km
	assert.Greater(t, result.AltKm, 100.0, "HST altitude too low")
	assert.Less(t, result.AltKm, 1000.0, "HST altitude too high")

	// HST orbital speed ≈ 7.59 km/s
	assert.Greater(t, result.VelKmS, 5.0)
	assert.Less(t, result.VelKmS, 10.0)
}

// TestPropagateTLE_GEO_GOES16 verifies propagation of a geostationary satellite
// (GOES-16 class). GEO satellites sit at ~35786 km altitude with near-zero
// inclination and near-zero velocity relative to Earth's surface.
func TestPropagateTLE_GEO_GOES16(t *testing.T) {
	result, err := PropagateTLE(
		knownTLEs.goes16Line1,
		knownTLEs.goes16Line2,
		time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	// GEO satellites are near the equator
	assert.InDelta(t, 0.0, result.Lat, 5.0, "GEO satellite should be near equator, got %f°", result.Lat)

	// GEO altitude: ~35786 km
	assert.InDelta(t, 35786.0, result.AltKm, 500.0, "GEO altitude should be ~35786 km, got %f km", result.AltKm)

	// GEO orbital velocity ≈ 3.07 km/s
	assert.Greater(t, result.VelKmS, 1.0, "GEO velocity too low: %f km/s", result.VelKmS)
	assert.Less(t, result.VelKmS, 6.0, "GEO velocity too high: %f km/s", result.VelKmS)
}

// TestPropagateTLE_PolarOrbit verifies propagation of a satellite in a
// polar orbit (90° inclination). Polar satellites can reach any latitude,
// so the latitude check is just within ±90°.
func TestPropagateTLE_PolarOrbit(t *testing.T) {
	// ISS orbital elements with inclination changed to 90° (polar)
	line1 := "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009"
	line2 := "2 25544  90.0000 208.5000 0007417  35.0000 325.0000 15.49560000000014"

	result, err := PropagateTLE(line1, line2, time.Date(2024, 4, 10, 6, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.GreaterOrEqual(t, result.Lat, -90.0)
	assert.LessOrEqual(t, result.Lat, 90.0)
	assert.GreaterOrEqual(t, result.Lon, -180.0)
	assert.LessOrEqual(t, result.Lon, 180.0)
	assert.Greater(t, result.AltKm, 200.0)
	assert.Less(t, result.AltKm, 1000.0)
	assert.Greater(t, result.VelKmS, 5.0)
	assert.Less(t, result.VelKmS, 10.0)
}

// TestPropagateTLE_EquatorialOrbit verifies propagation of a satellite in an
// equatorial orbit (0° inclination). The latitude should be very close to 0°.
func TestPropagateTLE_EquatorialOrbit(t *testing.T) {
	// ISS elements with inclination set to 0°
	line1 := "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009"
	line2 := "2 25544   0.0000 208.5000 0007417  35.0000 325.0000 15.49560000000015"

	result, err := PropagateTLE(line1, line2, time.Date(2024, 4, 10, 6, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	// Equatorial orbit stays at ~0° latitude
	assert.InDelta(t, 0.0, result.Lat, 1.0, "equatorial orbit latitude should be ~0°, got %f°", result.Lat)
	assert.GreaterOrEqual(t, result.Lon, -180.0)
	assert.LessOrEqual(t, result.Lon, 180.0)
	assert.Greater(t, result.AltKm, 200.0)
	assert.Less(t, result.AltKm, 1000.0)
}

// TestPropagateTLE_PropagateResultFields verifies that all fields of
// PropagateResult are populated with finite, non-NaN values for a valid TLE.
func TestPropagateTLE_PropagateResultFields(t *testing.T) {
	result, err := PropagateTLE(
		knownTLEs.issLine1,
		knownTLEs.issLine2,
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	assert.False(t, math.IsNaN(result.Lat), "Lat should not be NaN")
	assert.False(t, math.IsNaN(result.Lon), "Lon should not be NaN")
	assert.False(t, math.IsNaN(result.AltKm), "AltKm should not be NaN")
	assert.False(t, math.IsNaN(result.VelKmS), "VelKmS should not be NaN")

	assert.False(t, math.IsInf(result.Lat, 0), "Lat should not be Inf")
	assert.False(t, math.IsInf(result.Lon, 0), "Lon should not be Inf")
	assert.False(t, math.IsInf(result.AltKm, 0), "AltKm should not be Inf")
	assert.False(t, math.IsInf(result.VelKmS, 0), "VelKmS should not be Inf")
}

// TestPropagateTLE_ErrorMessages verifies the exact wording of error messages
// to ensure callers can rely on them for monitoring and alerting.
func TestPropagateTLE_ErrorMessages(t *testing.T) {
	t.Run("zero_position_error_text", func(t *testing.T) {
		line1 := "1 99999U 00000A   24001.00000000  .00000000  00000-0  00000-0 0  9992"
		line2 := "2 99999  90.0000   0.0000 9990000   0.0000   0.0000  1.00000000    15"

		_, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "zero position")
	})

	t.Run("unrealistic_altitude_error_text", func(t *testing.T) {
		// Stale LEO TLE propagated 100 years into the future
		_, err := PropagateTLE(
			knownTLEs.issLine1,
			knownTLEs.issLine2,
			time.Date(2124, 1, 1, 12, 0, 0, 0, time.UTC),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unrealistic altitude")
		assert.Contains(t, err.Error(), "stale TLE or decayed orbit")
	})
}

// TestPropagateTLE_ResultReturnedOnAltitudeError verifies that when the
// altitude sanity check fires, the partially computed result is still returned
// alongside the error, so callers can inspect raw values for diagnostics.
func TestPropagateTLE_ResultReturnedOnAltitudeError(t *testing.T) {
	// Stale ISS TLE propagated 100 years into the future — diverged orbit.
	result, err := PropagateTLE(
		knownTLEs.issLine1,
		knownTLEs.issLine2,
		time.Date(2124, 1, 1, 12, 0, 0, 0, time.UTC),
	)
	require.Error(t, err)

	// The function returns the computed (diverged) result alongside the error.
	// The altitude should be far above 50000 km.
	assert.Greater(t, result.AltKm, 50000.0, "diverged altitude should be > 50000 km, got %f", result.AltKm)
}

// TestPropagateTLE_ZeroPositionReturnsZeroResult verifies that the zero
// position error path returns a zero-value PropagateResult.
func TestPropagateTLE_ZeroPositionReturnsZeroResult(t *testing.T) {
	line1 := "1 99999U 00000A   24001.00000000  .00000000  00000-0  00000-0 0  9992"
	line2 := "2 99999  90.0000   0.0000 9990000   0.0000   0.0000  1.00000000    15"

	result, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err)

	// Zero result is returned when position is zero
	assert.Equal(t, 0.0, result.Lat)
	assert.Equal(t, 0.0, result.Lon)
	assert.Equal(t, 0.0, result.AltKm)
	assert.Equal(t, 0.0, result.VelKmS)
}

// TestPropagateTLE_PropagateResult_VelocityCalculation verifies that the
// velocity magnitude is correctly computed as the Euclidean norm of the
// velocity vector. Uses a known ISS propagation and checks internal consistency.
func TestPropagateTLE_PropagateResult_VelocityCalculation(t *testing.T) {
	tests := []struct {
		name        string
		propagateAt time.Time
		minVelKmS   float64
		maxVelKmS   float64
	}{
		{
			name:        "ISS_LEO_velocity",
			propagateAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			minVelKmS:   7.0,
			maxVelKmS:   8.5,
		},
		{
			name:        "GEO_velocity",
			propagateAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			minVelKmS:   2.5,
			maxVelKmS:   4.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var line1, line2 string
			if tc.name == "GEO_velocity" {
				line1 = knownTLEs.goes16Line1
				line2 = knownTLEs.goes16Line2
			} else {
				line1 = knownTLEs.issLine1
				line2 = knownTLEs.issLine2
			}

			result, err := PropagateTLE(line1, line2, tc.propagateAt)
			require.NoError(t, err)

			assert.Greater(t, result.VelKmS, tc.minVelKmS,
				"velocity %f km/s below minimum %f km/s", result.VelKmS, tc.minVelKmS)
			assert.Less(t, result.VelKmS, tc.maxVelKmS,
				"velocity %f km/s above maximum %f km/s", result.VelKmS, tc.maxVelKmS)
		})
	}
}

// TestPropagateTLE_AltitudeSanityBoundary exercises the boundary of the
// altitude sanity check. Altitudes up to 50000 km (HEO) should pass;
// beyond that they are rejected as diverged.
func TestPropagateTLE_AltitudeSanityBoundary(t *testing.T) {
	// A Molniya-class highly eccentric orbit at ~11000 km altitude should pass.
	// MOLNIYA-type: e=0.72, n=2 rev/day, i=62.8°
	line1 := "1 88888U 00000A   24001.50000000  .00000000  00000-0  00000-0 0  9992"
	line2 := "2 88888  62.8000 270.0000 7200000 270.0000   0.0000  2.00634480    13"

	result, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err, "Molniya orbit at apogee should pass altitude sanity check, got: %v", err)

	// Molniya at apogee: altitude roughly 200–40000 km depending on position in orbit
	assert.Greater(t, result.AltKm, 0.0, "altitude should be positive")
	assert.Less(t, result.AltKm, 50000.0, "altitude should be within sanity limit")
}

// TestPropagateTLE_ConcurrentSafety verifies that PropagateTLE is safe to
// call concurrently from multiple goroutines (no data races). This exercises
// the -race detector path.
func TestPropagateTLE_ConcurrentSafety(t *testing.T) {
	propagateAt := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	const goroutines = 10
	results := make(chan error, goroutines)

	for i := range goroutines {
		go func(offset int) {
			propagateTime := propagateAt.Add(time.Duration(offset) * time.Minute)
			_, err := PropagateTLE(knownTLEs.issLine1, knownTLEs.issLine2, propagateTime)
			results <- err
		}(i)
	}

	for range goroutines {
		err := <-results
		assert.NoError(t, err)
	}
}

// TestPropagateTLE_DifferentUTCTimes ensures that time components (year, month,
// day, hour, minute, second) are correctly passed to the underlying SGP4
// propagator. Different times should yield different positions for a LEO sat.
func TestPropagateTLE_DifferentUTCTimes(t *testing.T) {
	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 0, 46, 0, 0, time.UTC) // ~half ISS orbit period (~92 min)

	r1, err1 := PropagateTLE(knownTLEs.issLine1, knownTLEs.issLine2, t1)
	require.NoError(t, err1)

	r2, err2 := PropagateTLE(knownTLEs.issLine1, knownTLEs.issLine2, t2)
	require.NoError(t, err2)

	// After half an orbit, the satellite should be at a noticeably different position
	posDiff := math.Abs(r1.Lat-r2.Lat) + math.Abs(r1.Lon-r2.Lon)
	assert.Greater(t, posDiff, 1.0, "position should differ significantly after ~half orbit period")
}

// TestPropagateTLE_PanicRecovery verifies that the defer/recover block in
// PropagateTLE catches runtime panics that originate inside the go-satellite
// library and converts them into regular Go errors.
//
// The go-satellite days2mdhms function accesses a 12-element lmonth array with
// an index that can reach 13 when epochdays > 365. A TLE with epochdays=400
// triggers this runtime panic, which PropagateTLE's recover() must catch.
func TestPropagateTLE_PanicRecovery(t *testing.T) {
	// epochdays "400.50000000" is invalid (> 366); it triggers an index-out-of-range
	// panic inside satellite.TLEToSat → days2mdhms → lmonth[12] (out of bounds).
	line1 := "1 25544U 98067A   24400.50000000  .00016717  00000-0  10270-3 0  9002"
	line2 := "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011"

	_, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err, "invalid epochdays TLE should return an error via panic recovery")
	assert.Contains(t, err.Error(), "sgp4 propagation failed",
		"recovered panic should produce the standard error prefix")
}

// TestPropagateResult_StructFields verifies that PropagateResult holds all
// expected fields with their documented semantics.
func TestPropagateResult_StructFields(t *testing.T) {
	result, err := PropagateTLE(
		knownTLEs.issLine1,
		knownTLEs.issLine2,
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)

	// Lat: geographic latitude in degrees
	assert.IsType(t, float64(0), result.Lat)
	assert.GreaterOrEqual(t, result.Lat, -90.0)
	assert.LessOrEqual(t, result.Lat, 90.0)

	// Lon: geographic longitude in degrees
	assert.IsType(t, float64(0), result.Lon)
	assert.GreaterOrEqual(t, result.Lon, -180.0)
	assert.LessOrEqual(t, result.Lon, 180.0)

	// AltKm: altitude in kilometers (positive for orbiting satellites)
	assert.IsType(t, float64(0), result.AltKm)
	assert.Greater(t, result.AltKm, 0.0)

	// VelKmS: velocity magnitude in km/s (always positive)
	assert.IsType(t, float64(0), result.VelKmS)
	assert.Greater(t, result.VelKmS, 0.0)
}
