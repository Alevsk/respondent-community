package geocoder

import "testing"

func TestDefaultCentroids_CoversCommonCountriesAndIsValid(t *testing.T) {
	c := DefaultCentroids()

	// Broad coverage: the curated humanitarian set was 40 entries; the offline
	// fallback must cover the long tail (US/UK/EU/etc.) the old list missed.
	if len(c) < 180 {
		t.Fatalf("expected comprehensive coverage (>=180 countries), got %d", len(c))
	}

	// Spot-check codes that the old 40-entry config list did NOT include — these
	// are exactly the ones that previously fell through to "all tiers failed".
	for _, iso3 := range []string{"USA", "GBR", "DEU", "FRA", "RUS", "CHN", "JPN", "BRA", "IND", "AUS"} {
		if _, ok := c[iso3]; !ok {
			t.Errorf("missing centroid for %s", iso3)
		}
	}

	// Every entry must be a valid WGS84 coordinate and not the null island.
	for iso3, ll := range c {
		lat, lon := ll[0], ll[1]
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			t.Errorf("%s: out-of-range coordinate (%v, %v)", iso3, lat, lon)
		}
		if lat == 0 && lon == 0 {
			t.Errorf("%s: null-island coordinate", iso3)
		}
		if len(iso3) != 3 {
			t.Errorf("key %q is not a 3-letter ISO code", iso3)
		}
	}
}

func TestDefaultCentroids_ReturnsIndependentCopies(t *testing.T) {
	a := DefaultCentroids()
	a["USA"] = [2]float64{0, 0}
	b := DefaultCentroids()
	if b["USA"] == [2]float64{0, 0} {
		t.Fatal("DefaultCentroids must return a fresh map; mutation leaked across calls")
	}
}
