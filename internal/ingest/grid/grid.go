// Package grid provides hex-packed grid generation and batch size computation
// for spatial coverage crawling. Shared by both imperative and declarative adapters.
package grid

import (
	"fmt"
	"math"
	"time"
)

const (
	nmToKm      = 1.852
	kmPerDegLat = 111.32 // approximate km per degree latitude
)

// Region represents a geographic center point for spatial data fetching.
type Region struct {
	Lat   float64
	Lon   float64
	Label string
	Tier  RegionTier
}

// RegionTier controls polling priority. Lower tiers are polled more frequently.
type RegionTier int

const (
	// TierHot -- major global hubs, polled every cycle.
	TierHot RegionTier = 1
	// TierWarm -- secondary corridors, polled every 2nd cycle.
	TierWarm RegionTier = 2
	// TierCold -- sparse coverage areas, polled every 4th cycle.
	TierCold RegionTier = 3
)

// GenerateGlobalGrid produces a set of hex-packed grid points that cover the
// entire Earth surface. Each point represents the center of a circle with the
// given radius (in nautical miles). Hex packing ensures full coverage with
// minimal overlap.
//
// Hex geometry for covering circles of radius R:
//   - Horizontal spacing (within a row): sqrt(3) * R
//   - Vertical spacing (between rows):   1.5 * R
//   - Odd rows offset by half the horizontal spacing
//
// The farthest point from any center is the circumradius of the equilateral
// triangle formed by three adjacent centers, which equals exactly R when
// horizontal = sqrt(3)*R and vertical = 1.5*R.
//
// For radiusNM=250 this produces ~800 points.
func GenerateGlobalGrid(radiusNM float64) []Region {
	radiusKm := radiusNM * nmToKm

	// Hex packing: vertical = 1.5R, horizontal = sqrt(3)*R
	dLatKm := radiusKm * 1.5
	dLatDeg := dLatKm / kmPerDegLat
	dLonKmEquator := radiusKm * math.Sqrt(3)

	var regions []Region
	band := 0

	for lat := -90.0 + dLatDeg/2; lat < 90.0; lat += dLatDeg {
		latRad := lat * math.Pi / 180.0
		cosLat := math.Cos(latRad)

		// Near poles: single point per band
		if cosLat < 0.01 {
			regions = append(regions, Region{
				Lat:   lat,
				Lon:   0,
				Label: fmt.Sprintf("grid_%d_0", band),
				Tier:  0,
			})
			band++
			continue
		}

		// Horizontal spacing adjusted for latitude convergence
		dLonDeg := dLonKmEquator / (kmPerDegLat * cosLat)

		// Hex stagger: odd bands offset by half
		offset := 0.0
		if band%2 == 1 {
			offset = dLonDeg / 2
		}

		idx := 0
		for lon := -180.0 + offset; lon < 180.0; lon += dLonDeg {
			regions = append(regions, Region{
				Lat:   lat,
				Lon:   lon,
				Label: fmt.Sprintf("grid_%d_%d", band, idx),
				Tier:  0,
			})
			idx++
		}
		band++
	}

	return regions
}

// GenerateGridForBBox returns the subset of the global hex grid (for circles of
// radiusNM) whose center points cover the given bounding box, expanded by one
// radius so circles at the edges still cover the box corners. maxPoints caps the
// result (0 = uncapped) to bound upstream load for very large viewports.
// Antimeridian (west > east) is handled.
//
// It reuses GenerateGlobalGrid so on-demand viewport fills land on the SAME hex
// cells the global crawl uses — keeping fetched cells aligned and cache-reusable
// as the viewport pans.
func GenerateGridForBBox(west, south, east, north, radiusNM float64, maxPoints int) []Region {
	radiusKm := radiusNM * nmToKm
	marginLat := radiusKm / kmPerDegLat
	latMin := south - marginLat
	latMax := north + marginLat

	var out []Region
	for _, r := range GenerateGlobalGrid(radiusNM) {
		if r.Lat < latMin || r.Lat > latMax {
			continue
		}
		cosLat := math.Cos(r.Lat * math.Pi / 180.0)
		if cosLat < 0.01 {
			cosLat = 0.01
		}
		marginLon := radiusKm / (kmPerDegLat * cosLat)
		if !lonWithin(r.Lon, west, east, marginLon) {
			continue
		}
		out = append(out, r)
		if maxPoints > 0 && len(out) >= maxPoints {
			break
		}
	}
	return out
}

// lonWithin reports whether lon falls inside [west, east] expanded by margin,
// handling the antimeridian case where west > east (the box wraps ±180°).
func lonWithin(lon, west, east, margin float64) bool {
	w := west - margin
	e := east + margin
	if west <= east {
		return lon >= w && lon <= e
	}
	return lon >= w || lon <= e
}

// ComputeBatchSize calculates how many regions to fetch per polling interval
// so that all regions are visited within the target refresh window.
//
//	batch_size = ceil(regionCount / (targetRefresh / interval))
func ComputeBatchSize(regionCount int, interval, targetRefresh time.Duration) int {
	if regionCount <= 0 || interval <= 0 || targetRefresh <= 0 {
		return 1
	}

	cyclesPerRefresh := targetRefresh.Seconds() / interval.Seconds()
	if cyclesPerRefresh < 1 {
		cyclesPerRefresh = 1
	}

	return max(1, int(math.Ceil(float64(regionCount)/cyclesPerRefresh)))
}
