package domain_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestBBox_Contains_NormalCase(t *testing.T) {
	tests := []struct {
		name string
		bbox domain.BBox
		lat  float64
		lon  float64
		want bool
	}{
		{
			name: "point inside box",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  0, lon: 0,
			want: true,
		},
		{
			name: "point on west edge",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  0, lon: -10,
			want: true,
		},
		{
			name: "point on east edge",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  0, lon: 10,
			want: true,
		},
		{
			name: "point on south edge",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  -10, lon: 0,
			want: true,
		},
		{
			name: "point on north edge",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  10, lon: 0,
			want: true,
		},
		{
			name: "point too far south",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  -11, lon: 0,
			want: false,
		},
		{
			name: "point too far north",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  11, lon: 0,
			want: false,
		},
		{
			name: "point too far west",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  0, lon: -11,
			want: false,
		},
		{
			name: "point too far east",
			bbox: domain.BBox{West: -10, South: -10, East: 10, North: 10},
			lat:  0, lon: 11,
			want: false,
		},
		{
			name: "real-world box around NYC",
			bbox: domain.BBox{West: -74.5, South: 40.5, East: -73.5, North: 41.0},
			lat:  40.7128, lon: -74.006,
			want: true,
		},
		{
			name: "point outside NYC box",
			bbox: domain.BBox{West: -74.5, South: 40.5, East: -73.5, North: 41.0},
			lat:  51.5, lon: -0.1,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bbox.Contains(tt.lat, tt.lon)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBBox_Contains_AntimeridianWrap(t *testing.T) {
	// Antimeridian case: west > east (e.g., Japan to Alaska crossing 180°)
	bbox := domain.BBox{West: 170, South: -10, East: -170, North: 10}

	tests := []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{
			name: "point east of antimeridian (in box)",
			lat:  0, lon: 175,
			want: true,
		},
		{
			name: "point west of antimeridian (in box)",
			lat:  0, lon: -175,
			want: true,
		},
		{
			name: "point on west boundary",
			lat:  0, lon: 170,
			want: true,
		},
		{
			name: "point on east boundary",
			lat:  0, lon: -170,
			want: true,
		},
		{
			name: "point in middle (outside wrap box)",
			lat:  0, lon: 0,
			want: false,
		},
		{
			name: "point out of latitude range",
			lat:  20, lon: 175,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bbox.Contains(tt.lat, tt.lon)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBBox_WidthKm(t *testing.T) {
	tests := []struct {
		name      string
		bbox      domain.BBox
		wantRange [2]float64 // [min, max] expected km
	}{
		{
			name: "10 degree wide box at equator (~1110 km)",
			bbox: domain.BBox{West: 0, South: -5, East: 10, North: 5},
			// At equator, 10 degrees of longitude ≈ 1111 km
			wantRange: [2]float64{1100, 1120},
		},
		{
			name: "zero width box",
			bbox: domain.BBox{West: 10, South: -5, East: 10, North: 5},
			// Same west and east means zero width
			wantRange: [2]float64{0, 0},
		},
		{
			name: "small box in mid-latitudes",
			bbox: domain.BBox{West: -5, South: 45, East: 5, North: 55},
			// At ~50° lat, 1 degree of longitude ≈ 71.4 km, so 10 degrees ≈ 714 km
			wantRange: [2]float64{600, 800},
		},
		{
			// Regression: the prior Haversine implementation returned ~222 km
			// here (the great-circle short path going west through the
			// antimeridian, only 2°) instead of the bbox's actual 358° coverage.
			// Valkey GEOSEARCH then only found entities within 222 km of (0,0).
			name: "near-global bbox covers the whole equator (~39800 km)",
			bbox: domain.BBox{West: -179, South: -85, East: 179, North: 85},
			// 358° * 111.195 km/deg = 39808 km
			wantRange: [2]float64{39750, 39850},
		},
		{
			// Bog-standard full globe.
			name: "full globe bbox covers the whole equator (~40030 km)",
			bbox: domain.BBox{West: -180, South: -90, East: 180, North: 90},
			// 360° * 111.195 km/deg = 40030 km (spherical Earth, R=6371)
			wantRange: [2]float64{40000, 40050},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bbox.WidthKm()
			assert.GreaterOrEqual(t, got, tt.wantRange[0], "WidthKm too small")
			assert.LessOrEqual(t, got, tt.wantRange[1], "WidthKm too large")
		})
	}
}

func TestBBox_HeightKm(t *testing.T) {
	tests := []struct {
		name      string
		bbox      domain.BBox
		wantRange [2]float64 // [min, max] expected km
	}{
		{
			name: "10 degree tall box (~1106 km)",
			bbox: domain.BBox{West: -5, South: 0, East: 5, North: 10},
			// 10 degrees of latitude ≈ 1105-1110 km
			wantRange: [2]float64{1100, 1115},
		},
		{
			name:      "zero height box",
			bbox:      domain.BBox{West: -5, South: 10, East: 5, North: 10},
			wantRange: [2]float64{0, 0},
		},
		{
			name: "1 degree tall box (~111 km)",
			bbox: domain.BBox{West: -0.5, South: 40, East: 0.5, North: 41},
			// 1 degree of latitude ≈ 110-111 km
			wantRange: [2]float64{109, 112},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.bbox.HeightKm()
			assert.GreaterOrEqual(t, got, tt.wantRange[0], "HeightKm too small")
			assert.LessOrEqual(t, got, tt.wantRange[1], "HeightKm too large")
		})
	}
}

func TestHaversineDistanceKm(t *testing.T) {
	tests := []struct {
		name             string
		lat1, lon1       float64
		lat2, lon2       float64
		wantApprox       float64
		wantTolerancePct float64
	}{
		{
			name: "same point is zero distance",
			lat1: 40.0, lon1: -74.0,
			lat2: 40.0, lon2: -74.0,
			wantApprox:       0.0,
			wantTolerancePct: 0.0,
		},
		{
			name: "NYC to London (~5570 km)",
			lat1: 40.7128, lon1: -74.006,
			lat2: 51.5074, lon2: -0.1278,
			wantApprox:       5570.0,
			wantTolerancePct: 1.0,
		},
		{
			name: "equator 10 degrees longitude (~1111 km)",
			lat1: 0.0, lon1: 0.0,
			lat2: 0.0, lon2: 10.0,
			wantApprox:       1111.9,
			wantTolerancePct: 0.5,
		},
		{
			name: "poles distance (~20004 km)",
			lat1: 90.0, lon1: 0.0,
			lat2: -90.0, lon2: 0.0,
			wantApprox:       20004.0,
			wantTolerancePct: 0.5,
		},
		{
			name: "SF to LA (~559 km)",
			lat1: 37.7749, lon1: -122.4194,
			lat2: 34.0522, lon2: -118.2437,
			wantApprox:       559.0,
			wantTolerancePct: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.HaversineDistanceKm(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			assert.False(t, math.IsNaN(got), "result should not be NaN")
			assert.False(t, math.IsInf(got, 0), "result should not be Inf")
			assert.GreaterOrEqual(t, got, 0.0, "distance should be non-negative")

			if tt.wantApprox > 0 {
				tolerance := tt.wantApprox * tt.wantTolerancePct / 100.0
				assert.InDelta(t, tt.wantApprox, got, tolerance,
					"distance should be within %.1f%% of expected", tt.wantTolerancePct)
			} else {
				assert.InDelta(t, 0.0, got, 0.001, "same-point distance should be near zero")
			}
		})
	}
}

func TestBBoxCenter(t *testing.T) {
	tests := []struct {
		name    string
		bbox    domain.BBox
		wantLat float64
		wantLon float64
	}{
		{
			name:    "simple box centered at origin",
			bbox:    domain.BBox{West: -10, South: -10, East: 10, North: 10},
			wantLat: 0.0,
			wantLon: 0.0,
		},
		{
			name:    "box centered at 45N 45E",
			bbox:    domain.BBox{West: 40, South: 40, East: 50, North: 50},
			wantLat: 45.0,
			wantLon: 45.0,
		},
		{
			name:    "box spanning 180th meridian (antimeridian wrap)",
			bbox:    domain.BBox{West: 170, South: -5, East: -170, North: 5},
			wantLat: 0.0,
			wantLon: 180.0,
		},
		{
			name:    "antimeridian box where center is exactly at 180",
			bbox:    domain.BBox{West: 160, South: -5, East: -160, North: 5},
			wantLat: 0.0,
			wantLon: 180.0,
		},
		{
			name:    "NYC area box",
			bbox:    domain.BBox{West: -74.5, South: 40.5, East: -73.5, North: 41.0},
			wantLat: 40.75,
			wantLon: -74.0,
		},
		{
			// West=170, East=10: lon = 170 + (10+360-170)/2 = 270 > 180, normalises to -90
			name:    "antimeridian box where lon normalizes past 180 to negative",
			bbox:    domain.BBox{West: 170, South: -5, East: 10, North: 5},
			wantLat: 0.0,
			wantLon: -90.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon := domain.BBoxCenter(tt.bbox)
			assert.InDelta(t, tt.wantLat, lat, 0.0001)
			assert.InDelta(t, tt.wantLon, lon, 0.0001)
		})
	}
}
