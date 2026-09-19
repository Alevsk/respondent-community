package orbital

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropagateTLE_ISS(t *testing.T) {
	// ISS (ZARYA) TLE — epoch 2024
	line1 := "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009"
	line2 := "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011"

	result, err := PropagateTLE(line1, line2, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	// Latitude must be within ISS orbital inclination (~51.6°)
	assert.InDelta(t, 0, result.Lat, 90, "latitude should be between -90 and 90")
	assert.True(t, result.Lat >= -90 && result.Lat <= 90, "latitude out of range: %f", result.Lat)

	// Longitude must be between -180 and 180
	assert.True(t, result.Lon >= -180 && result.Lon <= 180, "longitude out of range: %f", result.Lon)

	// ISS altitude is roughly 400-430 km
	assert.True(t, result.AltKm > 100 && result.AltKm < 1000,
		"altitude should be 100-1000 km for ISS, got: %f km", result.AltKm)

	// ISS velocity is roughly 7.6-7.8 km/s
	assert.True(t, result.VelKmS > 5 && result.VelKmS < 10,
		"velocity should be 5-10 km/s for ISS, got: %f km/s", result.VelKmS)
}

func TestValidateTLE_ValidISS(t *testing.T) {
	line1 := "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009"
	line2 := "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011"
	require.NoError(t, ValidateTLE(line1, line2))
}

func TestValidateTLE_WrongLength(t *testing.T) {
	require.Error(t, ValidateTLE("short", "also short"))
}

func TestValidateTLE_WrongLinePrefix(t *testing.T) {
	line1 := "2 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9000"
	line2 := "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011"
	require.Error(t, ValidateTLE(line1, line2))
}

func TestValidateTLE_SatelliteNumberMismatch(t *testing.T) {
	line1 := "1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9009"
	line2 := "2 99999  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000016"
	require.Error(t, ValidateTLE(line1, line2))
	require.Contains(t, ValidateTLE(line1, line2).Error(), "satellite number mismatch")
}

func TestValidateTLE_EmptyStrings(t *testing.T) {
	require.Error(t, ValidateTLE("", ""))
}

func TestValidateTLE_BadEpoch(t *testing.T) {
	line1 := "1 25544U 98067A   XXXXX.XXXXXXXXX  .00016717  00000-0  10270-3 0  9007"
	line2 := "2 25544  51.6400 208.5000 0007417  35.0000 325.0000 15.49560000000011"
	require.Error(t, ValidateTLE(line1, line2))
}

func TestPropagateTLE_RejectsInvalidTLE(t *testing.T) {
	_, err := PropagateTLE("garbage", "data", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid TLE")
}
