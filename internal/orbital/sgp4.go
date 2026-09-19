package orbital

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	satellite "github.com/joshuaferrara/go-satellite"
)

// PropagateResult holds the propagated position and velocity of a satellite.
type PropagateResult struct {
	Lat    float64 // degrees
	Lon    float64 // degrees
	AltKm  float64 // altitude in kilometers
	VelKmS float64 // velocity magnitude in km/s
}

// ValidateTLE performs syntactic validation on TLE line 1 and line 2 before
// passing them to the go-satellite library. The go-satellite library calls
// log.Fatal (os.Exit) on malformed TLE input, which kills the process
// without any chance for graceful shutdown. This function catches format
// errors that would trigger that behavior.
//
// TLE format reference: https://celestrak.org/columns/v04n03/
func ValidateTLE(line1, line2 string) error {
	if err := validateTLELine(line1, '1'); err != nil {
		return fmt.Errorf("line 1: %w", err)
	}
	if err := validateTLELine(line2, '2'); err != nil {
		return fmt.Errorf("line 2: %w", err)
	}

	satNum1 := strings.TrimSpace(line1[2:7])
	satNum2 := strings.TrimSpace(line2[2:7])
	if satNum1 != satNum2 {
		return fmt.Errorf("satellite number mismatch: line1=%q line2=%q", satNum1, satNum2)
	}

	return nil
}

func validateTLELine(line string, expectedLineNum byte) error {
	if len(line) != 69 {
		return fmt.Errorf("invalid length %d (must be 69)", len(line))
	}
	if line[0] != expectedLineNum {
		return fmt.Errorf("invalid line number %q (must be %c)", line[0], expectedLineNum)
	}
	if line[1] != ' ' {
		return fmt.Errorf("expected space at position 1, got %q", line[1])
	}

	classification := line[7]
	switch classification {
	case 'U', 'C', 'S', 'T', 'B', 'D', '?', ' ':
	default:
		return fmt.Errorf("invalid classification %q", classification)
	}

	if expectedLineNum == '1' {
		epochStr := line[18:32]
		if _, err := strconv.ParseFloat(epochStr, 64); err != nil {
			return fmt.Errorf("invalid epoch %q: %w", epochStr, err)
		}
	}

	// Checksum: modulo-10 of all numeric digits (treating minus signs as 1, all others as 0)
	expectedChecksum := line[68] - '0'
	if expectedChecksum > 9 {
		return fmt.Errorf("invalid checksum character %q", line[68])
	}
	computed := computeTLEChecksum(line[:68])
	if computed != int(expectedChecksum) {
		return fmt.Errorf("checksum mismatch: computed=%d expected=%d", computed, expectedChecksum)
	}

	if expectedLineNum == '2' {
		meanMotionStr := strings.TrimSpace(line[52:63])
		if meanMotion, err := strconv.ParseFloat(meanMotionStr, 64); err != nil {
			return fmt.Errorf("invalid mean motion %q: %w", meanMotionStr, err)
		} else if meanMotion <= 0 {
			return fmt.Errorf("mean motion must be positive, got %f", meanMotion)
		}
	}

	return nil
}

func computeTLEChecksum(line string) int {
	sum := 0
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] >= '0' && line[i] <= '9':
			sum += int(line[i] - '0')
		case line[i] == '-':
			sum++
		}
	}
	return sum % 10
}

// PropagateTLE propagates a TLE (Two-Line Element) to the given time using SGP4
// and returns the geographic position and velocity magnitude.
//
// ValidateTLE is called internally to catch malformed input before it reaches the
// go-satellite library, which calls log.Fatal (os.Exit) on invalid TLE data.
// The recover() block catches any remaining panics from propagation failures
// (e.g., decayed orbits, math errors).
func PropagateTLE(line1, line2 string, t time.Time) (result PropagateResult, err error) {
	if err := ValidateTLE(line1, line2); err != nil {
		return result, fmt.Errorf("invalid TLE: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("sgp4 propagation failed: %v", r)
		}
	}()

	sat := satellite.TLEToSat(line1, line2, satellite.GravityWGS84)

	year, month, day := t.Date()
	hour, min, sec := t.Clock()

	position, velocity := satellite.Propagate(sat, year, int(month), day, hour, min, sec)

	// Zero position vector indicates decayed orbit or invalid TLE
	if position.X == 0 && position.Y == 0 && position.Z == 0 {
		return result, fmt.Errorf("sgp4 returned zero position (decayed orbit or invalid TLE)")
	}

	gmst := satellite.GSTimeFromDate(year, int(month), day, hour, min, sec)

	altitude, _, latlong := satellite.ECIToLLA(position, gmst)
	latlongDeg := satellite.LatLongDeg(latlong)

	velMag := math.Sqrt(velocity.X*velocity.X + velocity.Y*velocity.Y + velocity.Z*velocity.Z)

	result = PropagateResult{
		Lat:    latlongDeg.Latitude,
		Lon:    latlongDeg.Longitude,
		AltKm:  altitude,
		VelKmS: velMag,
	}

	// Sanity check: reject results from stale/diverged TLEs.
	// LEO: ~160-2000 km, MEO: ~2000-35786 km, GEO: ~35786 km, HEO up to ~40000 km.
	// Negative altitude or >50000 km means the propagation diverged.
	if result.AltKm < 0 || result.AltKm > 50000 {
		return result, fmt.Errorf("unrealistic altitude %.0f km (stale TLE or decayed orbit)", result.AltKm)
	}

	return result, nil
}
