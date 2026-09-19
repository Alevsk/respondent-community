// Package domain provides core domain types, constants, and utilities.
package domain

import "math"

// BBox represents a geographic bounding box for viewport queries.
type BBox struct {
	West  float64 `json:"west"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	North float64 `json:"north"`
}

// Contains checks if a lat/lon point falls within the bounding box.
// Handles antimeridian wrapping (west > east).
func (b *BBox) Contains(lat, lon float64) bool {
	if lat < b.South || lat > b.North {
		return false
	}
	if b.West <= b.East {
		// Normal case
		return lon >= b.West && lon <= b.East
	}
	// Antimeridian wrap: west > east (e.g., 170 to -170)
	return lon >= b.West || lon <= b.East
}

// WidthKm returns the approximate width of the bounding box in kilometers.
//
// Computed from the longitudinal span — NOT Haversine between corners.
// Haversine returns the shortest great-circle path between two points;
// for a near-global bbox like {West:-179, East:179} that's the 2°
// shortcut around the antimeridian (~222 km), not the 358° the bbox
// actually covers (~39800 km). Callers (Valkey GEOSEARCH) need the
// covered span so they search the whole bbox, not the shortcut.
//
// The longitudinal-degree → km factor uses the spherical-Earth equator
// constant (≈111.195 km/deg, matching HaversineDistanceKm's R=6371 km)
// scaled by cos(midLat) — the standard equirectangular approximation.
// Accurate to a few percent for bboxes narrower than a hemisphere;
// exact at the equator.
//
// Antimeridian-wrap convention (West > East): the bbox covers
// (360 - West + East) degrees going eastward through the antimeridian.
func (b *BBox) WidthKm() float64 {
	midLat := (b.North + b.South) / 2.0
	lonDegrees := b.East - b.West
	if lonDegrees < 0 {
		// Antimeridian wrap: e.g. West=170, East=-170 covers 20°.
		lonDegrees += 360
	}
	return lonDegrees * kmPerLonDegreeAt(midLat)
}

// HeightKm returns the approximate height of the bounding box in kilometers.
// Latitude doesn't wrap (no antimeridian equivalent), so a straight
// degree-difference × km-per-latitude-degree (≈111.32 km/deg) is exact
// to first order; we use Haversine here only because lat→km is
// uniform across longitudes (the meridian is a great circle).
func (b *BBox) HeightKm() float64 {
	midLon := (b.West + b.East) / 2.0
	return HaversineDistanceKm(b.South, midLon, b.North, midLon)
}

// kmPerLonDegreeAt returns the number of kilometers in one degree of
// longitude at the given latitude. At the equator this is the spherical
// Earth circumference (2πR with R=6371) / 360 ≈ 111.195 km, matching
// the constant used by HaversineDistanceKm. Decreases as cos(lat) —
// at 60° lat, one degree is ≈ 55.6 km.
//
// We use the spherical value, not the WGS84 equatorial value
// (≈111.32 km), so this function round-trips with HaversineDistanceKm
// for sub-hemisphere bboxes — keeping existing tests stable.
func kmPerLonDegreeAt(lat float64) float64 {
	const kmPerEquatorialDegree = 6371.0 * math.Pi / 180.0 // ≈ 111.195
	return kmPerEquatorialDegree * math.Cos(lat*math.Pi/180.0)
}

// HaversineDistanceKm computes the great-circle distance between two lat/lon points in km.
func HaversineDistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180.0)*math.Cos(lat2*math.Pi/180.0)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// BBoxCenter computes the center point of a bounding box, handling antimeridian wrapping.
func BBoxCenter(bbox BBox) (lat, lon float64) {
	lat = (bbox.North + bbox.South) / 2.0
	lon = (bbox.West + bbox.East) / 2.0
	if bbox.West > bbox.East {
		lon = bbox.West + (bbox.East+360-bbox.West)/2.0
		if lon > 180 {
			lon -= 360
		}
	}
	return lat, lon
}
