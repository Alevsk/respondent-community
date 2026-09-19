package parse

import (
	"encoding/json"
	"testing"
)

func TestParseADSBLolResponse_ValidData(t *testing.T) {
	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000), GS: 450, Track: 90},
			{Hex: "def456", Flight: "TEST02", Lat: 41.0, Lon: -73.0, AltBaro: float64(28000), GS: 400, Track: 180},
		},
		Total: 2,
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	entities, observations, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entities) != 2 {
		t.Errorf("expected 2 entities, got %d", len(entities))
	}
	if len(observations) != 2 {
		t.Errorf("expected 2 observations, got %d", len(observations))
	}

	// Verify entity fields
	e := entities[0]
	if e.ExternalID != "abc123" {
		t.Errorf("expected ExternalID abc123, got %s", e.ExternalID)
	}
	if e.LayerType != "flights_commercial" {
		t.Errorf("expected layer_type flights_commercial, got %s", e.LayerType)
	}
	if e.Name != "TEST01" {
		t.Errorf("expected name TEST01, got %s", e.Name)
	}
	if e.Metadata["source"] != "adsb_lol" {
		t.Errorf("expected source adsb_lol, got %s", e.Metadata["source"])
	}

	// Verify observation fields
	o := observations[0]
	if o.Position.Lat != 40.0 {
		t.Errorf("expected lat 40.0, got %f", o.Position.Lat)
	}
	if o.Velocity["speed"] == 0 {
		t.Error("expected non-zero speed")
	}
	if o.Velocity["heading"] != 90 {
		t.Errorf("expected heading 90, got %f", o.Velocity["heading"])
	}
}

func TestParseADSBLolResponse_SkipsInvalid(t *testing.T) {
	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "", Flight: "NOHEX", Lat: 40.0, Lon: -74.0}, // empty hex
			{Hex: "abc", Flight: "NOPOS", Lat: 0, Lon: 0},     // zero position
			{Hex: "def", Flight: "VALID", Lat: 40.0, Lon: -74.0, AltBaro: float64(10000)},
		},
	}

	data, _ := json.Marshal(response)
	entities, obs, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 valid entity, got %d", len(entities))
	}
	if len(obs) != 1 {
		t.Errorf("expected 1 valid observation, got %d", len(obs))
	}
}

func TestParseADSBLolResponse_InvalidJSON(t *testing.T) {
	_, _, err := ParseADSBLolResponse([]byte("not json"), "flights_commercial")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseADSBLolResponse_EmptyResponse(t *testing.T) {
	data, _ := json.Marshal(ADSBLolResponse{AC: nil})
	entities, obs, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 0 {
		t.Errorf("expected 0 entities for empty response, got %d", len(entities))
	}
	if len(obs) != 0 {
		t.Errorf("expected 0 observations for empty response, got %d", len(obs))
	}
}

func TestParseADSBLolResponse_LayerTypeAssignment(t *testing.T) {
	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "mil001", Flight: "MIL01", Lat: 40.0, Lon: -74.0, AltBaro: float64(20000)},
		},
	}

	data, _ := json.Marshal(response)
	entities, _, err := ParseADSBLolResponse(data, "flights_military")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if entities[0].LayerType != "flights_military" {
		t.Errorf("expected layer type flights_military, got %s", entities[0].LayerType)
	}
}

func TestParseADSBLolResponse_CallsignFallback(t *testing.T) {
	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "abc123", Flight: "", Lat: 40.0, Lon: -74.0},
		},
	}

	data, _ := json.Marshal(response)
	entities, _, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entities[0].Name != "abc123" {
		t.Errorf("expected callsign fallback to hex, got %s", entities[0].Name)
	}
}

func TestAltFeet_Precedence(t *testing.T) {
	tests := []struct {
		name string
		ac   ADSBLolAircraft
		want float64
	}{
		{"baro preferred", ADSBLolAircraft{AltBaro: float64(35000), AltGeom: float64(34800)}, 35000},
		{"geom fallback", ADSBLolAircraft{AltBaro: "ground", AltGeom: float64(34800)}, 34800},
		{"both zero", ADSBLolAircraft{AltBaro: "ground", AltGeom: float64(0)}, 0},
		{"nil values", ADSBLolAircraft{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ac.AltFeet()
			if got != tt.want {
				t.Errorf("AltFeet() = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestAltBaroFeet_IntBranch covers the case int: branch in AltBaroFeet.
// When an ADSBLolAircraft is constructed directly in Go (not via JSON
// unmarshalling), the AltBaro interface{} field can hold a native int.
func TestAltBaroFeet_IntBranch(t *testing.T) {
	tests := []struct {
		name    string
		altBaro interface{}
		want    float64
	}{
		{
			name:    "int value converts to float64",
			altBaro: int(30000),
			want:    30000.0,
		},
		{
			name:    "int zero returns zero",
			altBaro: int(0),
			want:    0.0,
		},
		{
			name:    "negative int converts correctly",
			altBaro: int(-500),
			want:    -500.0,
		},
		{
			name:    "float64 branch still works",
			altBaro: float64(25000),
			want:    25000.0,
		},
		{
			name:    "string ground returns zero",
			altBaro: "ground",
			want:    0.0,
		},
		{
			name:    "nil interface returns zero",
			altBaro: nil,
			want:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := ADSBLolAircraft{AltBaro: tt.altBaro}
			got := ac.AltBaroFeet()
			if got != tt.want {
				t.Errorf("AltBaroFeet() = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestAltGeomFeet_IntBranch covers the case int: branch in AltGeomFeet.
func TestAltGeomFeet_IntBranch(t *testing.T) {
	tests := []struct {
		name    string
		altGeom interface{}
		want    float64
	}{
		{
			name:    "int value converts to float64",
			altGeom: int(34500),
			want:    34500.0,
		},
		{
			name:    "int zero returns zero",
			altGeom: int(0),
			want:    0.0,
		},
		{
			name:    "negative int converts correctly",
			altGeom: int(-200),
			want:    -200.0,
		},
		{
			name:    "float64 branch still works",
			altGeom: float64(34800),
			want:    34800.0,
		},
		{
			name:    "string value returns zero (default branch)",
			altGeom: "unknown",
			want:    0.0,
		},
		{
			name:    "nil interface returns zero",
			altGeom: nil,
			want:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := ADSBLolAircraft{AltGeom: tt.altGeom}
			got := ac.AltGeomFeet()
			if got != tt.want {
				t.Errorf("AltGeomFeet() = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestAltFeet_IntAltBaro verifies that AltFeet() correctly uses an int-typed
// AltBaro value as the primary altitude (baro > 0 takes precedence).
func TestAltFeet_IntAltBaro(t *testing.T) {
	tests := []struct {
		name    string
		altBaro interface{}
		altGeom interface{}
		want    float64
	}{
		{
			name:    "int baro preferred over float64 geom",
			altBaro: int(35000),
			altGeom: float64(34800),
			want:    35000.0,
		},
		{
			name:    "int baro zero falls back to int geom",
			altBaro: int(0),
			altGeom: int(34800),
			want:    34800.0,
		},
		{
			name:    "int baro positive returns baro",
			altBaro: int(10000),
			altGeom: int(9900),
			want:    10000.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := ADSBLolAircraft{AltBaro: tt.altBaro, AltGeom: tt.altGeom}
			got := ac.AltFeet()
			if got != tt.want {
				t.Errorf("AltFeet() = %f, want %f", got, tt.want)
			}
		})
	}
}

// TestParseADSBLolResponse_VelocityEdgeCases covers zero GS and zero Track branches,
// ensuring that velocity map entries are only added when values are positive.
func TestParseADSBLolResponse_VelocityEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		gs          float64
		track       float64
		wantSpeed   bool
		wantHeading bool
	}{
		{
			name:        "both gs and track zero — no velocity entries",
			gs:          0,
			track:       0,
			wantSpeed:   false,
			wantHeading: false,
		},
		{
			name:        "gs positive track zero — only speed",
			gs:          300,
			track:       0,
			wantSpeed:   true,
			wantHeading: false,
		},
		{
			name:        "gs zero track positive — only heading",
			gs:          0,
			track:       270,
			wantSpeed:   false,
			wantHeading: true,
		},
		{
			name:        "both positive — both present",
			gs:          450,
			track:       180,
			wantSpeed:   true,
			wantHeading: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := ADSBLolResponse{
				AC: []ADSBLolAircraft{
					{
						Hex:    "aabbcc",
						Flight: "VEL001",
						Lat:    52.0,
						Lon:    13.0,
						GS:     tt.gs,
						Track:  tt.track,
					},
				},
			}
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			_, obs, err := ParseADSBLolResponse(data, "flights_commercial")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(obs) != 1 {
				t.Fatalf("expected 1 observation, got %d", len(obs))
			}
			o := obs[0]

			_, hasSpeed := o.Velocity["speed"]
			_, hasHeading := o.Velocity["heading"]

			if hasSpeed != tt.wantSpeed {
				t.Errorf("speed present=%v, want %v", hasSpeed, tt.wantSpeed)
			}
			if hasHeading != tt.wantHeading {
				t.Errorf("heading present=%v, want %v", hasHeading, tt.wantHeading)
			}
		})
	}
}

// TestParseADSBLolResponse_HexNormalization verifies that hex codes are
// lower-cased and whitespace-trimmed before being used as the external ID.
func TestParseADSBLolResponse_HexNormalization(t *testing.T) {
	tests := []struct {
		name      string
		hex       string
		wantExtID string
	}{
		{
			name:      "uppercase hex is lowercased",
			hex:       "ABC123",
			wantExtID: "abc123",
		},
		{
			name:      "hex with leading/trailing spaces trimmed",
			hex:       "  def456  ",
			wantExtID: "def456",
		},
		{
			name:      "mixed case with spaces",
			hex:       "  A1B2C3  ",
			wantExtID: "a1b2c3",
		},
		{
			name:      "already lowercase is unchanged",
			hex:       "aabbcc",
			wantExtID: "aabbcc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := ADSBLolResponse{
				AC: []ADSBLolAircraft{
					{
						Hex:    tt.hex,
						Flight: "TEST",
						Lat:    40.0,
						Lon:    -74.0,
					},
				},
			}
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			entities, _, err := ParseADSBLolResponse(data, "flights_commercial")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(entities) != 1 {
				t.Fatalf("expected 1 entity, got %d", len(entities))
			}
			if entities[0].ExternalID != tt.wantExtID {
				t.Errorf("ExternalID=%q, want %q", entities[0].ExternalID, tt.wantExtID)
			}
		})
	}
}

// TestParseADSBLolResponse_MetadataFields checks that all metadata keys are
// populated correctly on parsed entities.
func TestParseADSBLolResponse_MetadataFields(t *testing.T) {
	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "ff1234", Flight: "ACME01", Lat: 48.0, Lon: 2.0, AltBaro: float64(15000)},
		},
	}

	data, _ := json.Marshal(response)
	entities, _, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	e := entities[0]
	if e.Metadata["callsign"] != "ACME01" {
		t.Errorf("metadata callsign=%q, want ACME01", e.Metadata["callsign"])
	}
	if e.Metadata["icao24"] != "ff1234" {
		t.Errorf("metadata icao24=%q, want ff1234", e.Metadata["icao24"])
	}
	if e.Metadata["source"] != "adsb_lol" {
		t.Errorf("metadata source=%q, want adsb_lol", e.Metadata["source"])
	}
}

// TestParseADSBLolResponse_AltitudeConversion verifies that feet-to-metres
// conversion is applied correctly to both the Position.Alt and AltitudeM fields.
func TestParseADSBLolResponse_AltitudeConversion(t *testing.T) {
	const altFeet = 10000.0
	const feetToMetres = 0.3048
	want := altFeet * feetToMetres

	response := ADSBLolResponse{
		AC: []ADSBLolAircraft{
			{Hex: "aa11bb", Flight: "ALT001", Lat: 51.5, Lon: -0.1, AltBaro: altFeet},
		},
	}

	data, _ := json.Marshal(response)
	_, obs, err := ParseADSBLolResponse(data, "flights_commercial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	o := obs[0]
	if o.AltitudeM != want {
		t.Errorf("AltitudeM=%f, want %f", o.AltitudeM, want)
	}
	if o.Position.Alt != want {
		t.Errorf("Position.Alt=%f, want %f", o.Position.Alt, want)
	}
}
