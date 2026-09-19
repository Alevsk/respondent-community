package enrichment

import "testing"

func TestSchemaKey(t *testing.T) {
	if got := SchemaKey("adsb_military", "military_aircraft_classification"); got != "adsb_military.military_aircraft_classification" {
		t.Errorf("SchemaKey = %q, want adsb_military.military_aircraft_classification", got)
	}
}
