package domain

import "math"

// LatLonAltToWorld converts geodetic coordinates (lat/lon/alt) to a simplified
// world-space Cartesian representation (meters). This is a placeholder using
// equirectangular approximation; a production system would use proper geodetic
// transformations (e.g., ECEF).
func LatLonAltToWorld(lat, lon, alt float64) (x, y, z float64) {
	x = lon * 111319.488 // meters per degree at equator
	y = lat * 110574.0   // approximate meters per degree
	z = alt

	// Normalize to prevent extreme values
	x = math.Mod(x, 10000)
	y = math.Mod(y, 10000)

	return x, y, z
}

// ProjectPoint projects a geo point through a calibrated camera using a
// simplified pinhole camera model. It converts the target coordinates to world
// space, then applies FOV-based focal length projection with calibration offsets.
//
// Returns the projected 2D image coordinates (x, y).
func ProjectPoint(cal CameraCalibration, targetLat, targetLon, targetAlt float64) (x, y float64, err error) {
	worldX, worldY, worldZ := LatLonAltToWorld(targetLat, targetLon, targetAlt)

	// FOV is in degrees; convert to radians and compute focal length equivalent.
	focalLength := 1.0 / math.Tan(cal.FOV*math.Pi/360.0)

	x = (focalLength * worldX / worldZ) + cal.OffsetEastM
	y = (focalLength * worldY / worldZ) + cal.OffsetNorthM

	return x, y, nil
}

// CameraCalibration holds the subset of calibration parameters needed for
// point projection. This is a value object extracted from the full Calibration
// entity to keep the projection function independent of persistence concerns.
type CameraCalibration struct {
	FOV          float64
	OffsetNorthM float64
	OffsetEastM  float64
}

// CalibrationToProjection converts a persistence-layer Calibration to the
// domain value object used by ProjectPoint.
func CalibrationToProjection(cal *Calibration) CameraCalibration {
	return CameraCalibration{
		FOV:          cal.FOV,
		OffsetNorthM: cal.OffsetNorthM,
		OffsetEastM:  cal.OffsetEastM,
	}
}
