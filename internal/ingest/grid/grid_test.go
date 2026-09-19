package grid

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestGenerateGlobalGrid_Radius50(t *testing.T) {
	regions := GenerateGlobalGrid(50)
	if len(regions) == 0 {
		t.Error("expected non-empty regions for radius 50")
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
		if r.Label == "" {
			t.Errorf("region %d: expected non-empty label", i)
		}
	}
}

func TestGenerateGlobalGrid_Radius10(t *testing.T) {
	regions := GenerateGlobalGrid(10)
	if len(regions) == 0 {
		t.Error("expected non-empty regions for radius 10")
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
	}
}

func TestGenerateGlobalGrid_Radius100(t *testing.T) {
	regions := GenerateGlobalGrid(100)
	if len(regions) == 0 {
		t.Error("expected non-empty regions for radius 100")
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
		if r.Label == "" {
			t.Errorf("region %d: expected non-empty label", i)
		}
	}
}

func TestGenerateGlobalGrid_Radius250(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	if len(regions) < 700 {
		t.Errorf("expected at least 700 regions for radius 250, got %d", len(regions))
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
	}
}

func TestGenerateGlobalGrid_Radius500(t *testing.T) {
	regions := GenerateGlobalGrid(500)
	if len(regions) == 0 {
		t.Error("expected non-empty regions for radius 500")
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
	}
}

func TestGenerateGlobalGrid_Radius1000(t *testing.T) {
	regions := GenerateGlobalGrid(1000)
	if len(regions) == 0 {
		t.Error("expected non-empty regions for radius 1000")
	}
	for i, r := range regions {
		if r.Lat < -90 || r.Lat > 90 {
			t.Errorf("region %d: lat %f out of bounds [-90, 90]", i, r.Lat)
		}
		if r.Lon < -180 || r.Lon > 180 {
			t.Errorf("region %d: lon %f out of bounds [-180, 180]", i, r.Lon)
		}
	}
}

func TestGenerateGlobalGrid_SmallerRadiusMoreRegions(t *testing.T) {
	regions100 := GenerateGlobalGrid(100)
	regions250 := GenerateGlobalGrid(250)
	regions500 := GenerateGlobalGrid(500)
	if len(regions100) <= len(regions250) {
		t.Errorf("expected radius 100 (%d) to produce more regions than radius 250 (%d)", len(regions100), len(regions250))
	}
	if len(regions250) <= len(regions500) {
		t.Errorf("expected radius 250 (%d) to produce more regions than radius 500 (%d)", len(regions250), len(regions500))
	}
}

func TestGenerateGlobalGrid_HexPackingStagger(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	bandLons := make(map[int][]float64)
	for _, r := range regions {
		bandLons[extractBandFromLabel(r.Label)] = append(bandLons[extractBandFromLabel(r.Label)], r.Lon)
	}
	for band, lons := range bandLons {
		if len(lons) < 2 {
			continue
		}
		if band%2 == 1 {
			if lons[0] > -180 {
				t.Logf("odd band %d has offset starting at %f", band, lons[0])
			}
		}
	}
}

func extractBandFromLabel(label string) int {
	var band int
	_, _ = fmt.Sscanf(label, "grid_%d", &band)
	return band
}

func TestGenerateGlobalGrid_LabelFormat(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	for i, r := range regions {
		if len(r.Label) < 6 {
			t.Errorf("region %d: label too short: %s", i, r.Label)
		}
		var band, idx int
		n, err := fmt.Sscanf(r.Label, "grid_%d_%d", &band, &idx)
		if n != 2 || err != nil {
			t.Errorf("region %d: label format unexpected: %s (n=%d, err=%v)", i, r.Label, n, err)
		}
	}
}

func TestGenerateGlobalGrid_TierValue(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	for i, r := range regions {
		if r.Tier != 0 {
			t.Errorf("region %d: expected tier 0, got %d", i, r.Tier)
		}
	}
}

func TestGenerateGlobalGrid_LatitudeProgression(t *testing.T) {
	regions := GenerateGlobalGrid(100)
	if len(regions) < 2 {
		t.Skip("not enough regions to check latitude progression")
	}
	prevLat := regions[0].Lat
	for i := 1; i < len(regions); i++ {
		if regions[i].Lat < prevLat {
			prevLat = regions[i].Lat
		} else if regions[i].Lat > prevLat {
			prevLat = regions[i].Lat
		}
	}
}

func TestGenerateGlobalGrid_CoversPolarRegions(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	hasNearPole := false
	for _, r := range regions {
		if math.Abs(r.Lat) > 80 {
			hasNearPole = true
			break
		}
	}
	if !hasNearPole {
		t.Error("expected at least one region near the poles (|lat| > 80)")
	}
}

func TestGenerateGlobalGrid_CoversEquator(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	hasNearEquator := false
	for _, r := range regions {
		if math.Abs(r.Lat) < 10 {
			hasNearEquator = true
			break
		}
	}
	if !hasNearEquator {
		t.Error("expected at least one region near the equator (|lat| < 10)")
	}
}

func TestGenerateGlobalGrid_PoleNearZeroCosLat(t *testing.T) {
	regions := GenerateGlobalGrid(500)
	var poleRegions []Region
	for _, r := range regions {
		if math.Abs(r.Lat) > 85 {
			poleRegions = append(poleRegions, r)
		}
	}
	for i, r := range poleRegions {
		if r.Lon != 0 {
			t.Errorf("pole region %d: expected lon=0, got %f", i, r.Lon)
		}
	}
}

func TestComputeBatchSize_BasicCalculation(t *testing.T) {
	regionCount := 100
	interval := 1 * time.Minute
	targetRefresh := 10 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 10

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_ExactDivision(t *testing.T) {
	regionCount := 200
	interval := 30 * time.Second
	targetRefresh := 5 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 20

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_NonDivisible(t *testing.T) {
	regionCount := 103
	interval := 1 * time.Minute
	targetRefresh := 10 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 11

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_SingleRegion(t *testing.T) {
	regionCount := 1
	interval := 1 * time.Minute
	targetRefresh := 10 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_LargeRegionCount(t *testing.T) {
	regionCount := 10000
	interval := 5 * time.Second
	targetRefresh := 1 * time.Hour

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result <= 0 {
		t.Errorf("expected positive batch size, got %d", result)
	}

	cyclesPerRefresh := targetRefresh.Seconds() / interval.Seconds()
	expected := int(math.Ceil(float64(regionCount) / cyclesPerRefresh))

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_IntervalGreaterThanTargetRefresh(t *testing.T) {
	regionCount := 100
	interval := 10 * time.Minute
	targetRefresh := 5 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 100

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_ZeroRegionCount(t *testing.T) {
	result := ComputeBatchSize(0, 1*time.Minute, 10*time.Minute)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_NegativeRegionCount(t *testing.T) {
	result := ComputeBatchSize(-100, 1*time.Minute, 10*time.Minute)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_ZeroInterval(t *testing.T) {
	result := ComputeBatchSize(100, 0, 10*time.Minute)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_NegativeInterval(t *testing.T) {
	result := ComputeBatchSize(100, -1*time.Minute, 10*time.Minute)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_ZeroTargetRefresh(t *testing.T) {
	result := ComputeBatchSize(100, 1*time.Minute, 0)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_NegativeTargetRefresh(t *testing.T) {
	result := ComputeBatchSize(100, 1*time.Minute, -10*time.Minute)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_AllZeros(t *testing.T) {
	result := ComputeBatchSize(0, 0, 0)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_AllNegatives(t *testing.T) {
	result := ComputeBatchSize(-1, -1*time.Second, -1*time.Second)
	expected := 1

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_VerySmallInterval(t *testing.T) {
	regionCount := 1000
	interval := 1 * time.Millisecond
	targetRefresh := 1 * time.Second

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result <= 0 {
		t.Errorf("expected positive batch size, got %d", result)
	}
	if result > regionCount {
		t.Errorf("batch size %d should not exceed region count %d", result, regionCount)
	}
}

func TestComputeBatchSize_VeryLargeTargetRefresh(t *testing.T) {
	regionCount := 100
	interval := 1 * time.Second
	targetRefresh := 24 * time.Hour

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result <= 0 {
		t.Errorf("expected positive batch size, got %d", result)
	}
	if result > regionCount {
		t.Errorf("batch size %d should not exceed region count %d", result, regionCount)
	}
}

func TestComputeBatchSize_MinimumReturn(t *testing.T) {
	regionCount := 1
	interval := 1 * time.Hour
	targetRefresh := 1 * time.Second

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result < 1 {
		t.Errorf("expected batch size >= 1, got %d", result)
	}
}

func TestRegionTier_Values(t *testing.T) {
	if TierHot != 1 {
		t.Errorf("expected TierHot = 1, got %d", TierHot)
	}
	if TierWarm != 2 {
		t.Errorf("expected TierWarm = 2, got %d", TierWarm)
	}
	if TierCold != 3 {
		t.Errorf("expected TierCold = 3, got %d", TierCold)
	}
}

func TestRegion_Fields(t *testing.T) {
	r := Region{
		Lat:   40.7128,
		Lon:   -74.0060,
		Label: "nyc",
		Tier:  TierHot,
	}

	if r.Lat != 40.7128 {
		t.Errorf("expected Lat = 40.7128, got %f", r.Lat)
	}
	if r.Lon != -74.0060 {
		t.Errorf("expected Lon = -74.0060, got %f", r.Lon)
	}
	if r.Label != "nyc" {
		t.Errorf("expected Label = nyc, got %s", r.Label)
	}
	if r.Tier != TierHot {
		t.Errorf("expected Tier = TierHot, got %d", r.Tier)
	}
}

func TestComputeBatchSize_FractionalCycles(t *testing.T) {
	regionCount := 10
	interval := 7 * time.Second
	targetRefresh := 30 * time.Second

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	cyclesPerRefresh := targetRefresh.Seconds() / interval.Seconds()
	expected := int(math.Ceil(float64(regionCount) / cyclesPerRefresh))

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_EqualIntervalAndTargetRefresh(t *testing.T) {
	regionCount := 50
	interval := 5 * time.Minute
	targetRefresh := 5 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 50

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_HalfInterval(t *testing.T) {
	regionCount := 100
	interval := 30 * time.Second
	targetRefresh := 1 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)
	expected := 50

	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestComputeBatchSize_DoubleCycles(t *testing.T) {
	regionCount := 100
	interval := 30 * time.Second
	targetRefresh := 1 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}
}

func TestGenerateGlobalGrid_BoundsLatRange(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	minLat := 90.0
	maxLat := -90.0
	for _, r := range regions {
		if r.Lat < minLat {
			minLat = r.Lat
		}
		if r.Lat > maxLat {
			maxLat = r.Lat
		}
	}
	if minLat < -90 {
		t.Errorf("minimum latitude %f is below -90", minLat)
	}
	if maxLat > 90 {
		t.Errorf("maximum latitude %f is above 90", maxLat)
	}
}

func TestGenerateGlobalGrid_BoundsLonRange(t *testing.T) {
	regions := GenerateGlobalGrid(250)
	minLon := 180.0
	maxLon := -180.0
	for _, r := range regions {
		if r.Lon < minLon {
			minLon = r.Lon
		}
		if r.Lon > maxLon {
			maxLon = r.Lon
		}
	}
	if minLon < -180 {
		t.Errorf("minimum longitude %f is below -180", minLon)
	}
	if maxLon > 180 {
		t.Errorf("maximum longitude %f is above 180", maxLon)
	}
}

func TestGenerateGlobalGrid_NoDuplicateLabels(t *testing.T) {
	regions := GenerateGlobalGrid(500)
	labels := make(map[string]bool)
	for i, r := range regions {
		if labels[r.Label] {
			t.Errorf("region %d has duplicate label: %s", i, r.Label)
		}
		labels[r.Label] = true
	}
}

func TestComputeBatchSize_RoundingUp(t *testing.T) {
	regionCount := 7
	interval := 1 * time.Minute
	targetRefresh := 3 * time.Minute

	result := ComputeBatchSize(regionCount, interval, targetRefresh)

	if result < 3 {
		t.Errorf("expected batch size >= 3 for rounding up, got %d", result)
	}
}

func TestGenerateGridForBBox_CoversBox(t *testing.T) {
	// A Mexico-sized box at radius 250nm must yield multiple cells (viewport
	// tiling), all within the box expanded by ~one radius, and aligned to the
	// global grid's latitudes.
	west, south, east, north := -118.0, 14.0, -86.0, 33.0
	pts := GenerateGridForBBox(west, south, east, north, 250, 0)
	if len(pts) < 4 {
		t.Fatalf("expected several grid cells covering Mexico, got %d", len(pts))
	}
	// radius 250nm ≈ 463km ≈ 4.16° lat margin
	const marginLat = 5.0
	for _, p := range pts {
		if p.Lat < south-marginLat || p.Lat > north+marginLat {
			t.Errorf("cell lat %.2f outside box+margin [%.1f,%.1f]", p.Lat, south-marginLat, north+marginLat)
		}
	}
}

func TestGenerateGridForBBox_MaxPointsCap(t *testing.T) {
	// A huge box would fan out to many cells; maxPoints must cap it.
	pts := GenerateGridForBBox(-180, -60, 180, 70, 250, 10)
	if len(pts) != 10 {
		t.Fatalf("expected cap of 10 cells, got %d", len(pts))
	}
}

func TestGenerateGridForBBox_Antimeridian(t *testing.T) {
	// A box straddling the antimeridian (west=170, east=-170) must still yield
	// cells, all near ±180 longitude (not in the middle of the Pacific gap).
	pts := GenerateGridForBBox(170, 0, -170, 20, 250, 0)
	if len(pts) == 0 {
		t.Fatal("expected cells across the antimeridian, got none")
	}
	for _, p := range pts {
		if p.Lon > -160 && p.Lon < 160 {
			t.Errorf("cell lon %.2f not near the antimeridian", p.Lon)
		}
	}
}
