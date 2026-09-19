package domain

import (
	"math"
	"testing"
)

func TestLatLonAltToWorld(t *testing.T) {
	tests := []struct {
		name          string
		lat, lon, alt float64
	}{
		{"equator prime meridian", 0, 0, 0},
		{"SF coordinates", 37.77, -122.42, 100},
		{"london coordinates", 51.5, -0.1, 50},
		{"negative altitude", 0, 0, -100},
		{"high altitude", 0, 0, 10000},
		{"extreme longitude", 0, 180, 0},
		{"extreme latitude", 90, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, z := LatLonAltToWorld(tt.lat, tt.lon, tt.alt)
			if math.IsNaN(x) || math.IsNaN(y) || math.IsNaN(z) {
				t.Errorf("got NaN: x=%f, y=%f, z=%f", x, y, z)
			}
			if math.IsInf(x, 0) || math.IsInf(y, 0) || math.IsInf(z, 0) {
				t.Errorf("got Inf: x=%f, y=%f, z=%f", x, y, z)
			}
			if z != tt.alt {
				t.Errorf("expected z=%f, got %f", tt.alt, z)
			}
		})
	}
}

func TestLatLonAltToWorld_Normalization(t *testing.T) {
	x1, y1, _ := LatLonAltToWorld(0, 100, 0)
	x2, _, _ := LatLonAltToWorld(0, 260, 0)
	if x1 == x2 {
		t.Error("expected different x values for different longitudes")
	}
	if math.Abs(x1) > 10000 || math.Abs(y1) > 10000 {
		t.Errorf("values should be normalized to < 10000: x=%f, y=%f", x1, y1)
	}
}

func TestProjectPoint(t *testing.T) {
	tests := []struct {
		name       string
		cal        CameraCalibration
		lat, lon   float64
		alt        float64
		wantFinite bool
	}{
		{
			name:       "standard FOV 90",
			cal:        CameraCalibration{FOV: 90, OffsetNorthM: 0, OffsetEastM: 0},
			lat:        37.77,
			lon:        -122.42,
			alt:        100,
			wantFinite: true,
		},
		{
			name:       "narrow FOV 30",
			cal:        CameraCalibration{FOV: 30, OffsetNorthM: 0, OffsetEastM: 0},
			lat:        37.77,
			lon:        -122.42,
			alt:        100,
			wantFinite: true,
		},
		{
			name:       "with offsets",
			cal:        CameraCalibration{FOV: 90, OffsetNorthM: 100, OffsetEastM: 50},
			lat:        37.77,
			lon:        -122.42,
			alt:        100,
			wantFinite: true,
		},
		{
			name:       "wide FOV 120",
			cal:        CameraCalibration{FOV: 120, OffsetNorthM: 0, OffsetEastM: 0},
			lat:        51.5,
			lon:        -0.1,
			alt:        50,
			wantFinite: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, err := ProjectPoint(tt.cal, tt.lat, tt.lon, tt.alt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantFinite {
				if math.IsNaN(x) || math.IsNaN(y) {
					t.Errorf("expected finite values, got NaN: x=%f, y=%f", x, y)
				}
				if math.IsInf(x, 0) || math.IsInf(y, 0) {
					t.Errorf("expected finite values, got Inf: x=%f, y=%f", x, y)
				}
			}
		})
	}
}

func TestCalibrationToProjection(t *testing.T) {
	cal := &Calibration{
		ID:           "cal-1",
		CameraFeedID: "feed-1",
		Heading:      45,
		Pitch:        -30,
		Roll:         5,
		FOV:          90,
		RangeM:       1000,
		HeightM:      10,
		OffsetNorthM: 15.5,
		OffsetEastM:  -20.3,
	}

	proj := CalibrationToProjection(cal)
	if proj.FOV != 90 {
		t.Errorf("FOV = %f, want 90", proj.FOV)
	}
	if proj.OffsetNorthM != 15.5 {
		t.Errorf("OffsetNorthM = %f, want 15.5", proj.OffsetNorthM)
	}
	if proj.OffsetEastM != -20.3 {
		t.Errorf("OffsetEastM = %f, want -20.3", proj.OffsetEastM)
	}
}

func TestProjectPoint_OffsetsAffectResult(t *testing.T) {
	calNoOffset := CameraCalibration{FOV: 90, OffsetNorthM: 0, OffsetEastM: 0}
	calOffset := CameraCalibration{FOV: 90, OffsetNorthM: 100, OffsetEastM: 50}

	x1, y1, _ := ProjectPoint(calNoOffset, 37.77, -122.42, 100)
	x2, y2, _ := ProjectPoint(calOffset, 37.77, -122.42, 100)

	if x1 == x2 {
		t.Error("expected different x for different offsets")
	}
	if y1 == y2 {
		t.Error("expected different y for different offsets")
	}
}
