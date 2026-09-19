package prompts

import (
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEntity() *domain.Entity {
	return &domain.Entity{
		ID:         "ent-001",
		ExternalID: "ext-abc123",
		Name:       "Boeing 747",
		LayerType:  "flights_commercial",
		Metadata: map[string]string{
			"airline":  "United",
			"callsign": "UAL123",
			"squawk":   "1200",
		},
	}
}

func testObservation() *domain.Observation {
	return &domain.Observation{
		Position: &domain.GeoPoint{
			Lat: 40.7128,
			Lon: -74.0060,
		},
		AltitudeM: 10000,
		Metadata: map[string]string{
			"speed":   "450",
			"heading": "270",
		},
	}
}

func TestRender_SimpleEntityFields(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "")

	tmpl := "Entity: {{.Entity.Name}} ({{.Entity.ID}}), Layer: {{.Entity.LayerType}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Equal(t, "Entity: Boeing 747 (ent-001), Layer: flights_commercial", result)
}

func TestRender_ObservationMetadata(t *testing.T) {
	ent := testEntity()
	obs := testObservation()
	pd := NewPromptData(ent, obs, "")

	tmpl := "Position: {{.Observation.Lat}}, {{.Observation.Lon}}, Alt: {{.Observation.Altitude}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Contains(t, result, "40.7128")
	assert.Contains(t, result, "-74.006")
	assert.Contains(t, result, "10000")
}

func TestRender_OutputSchemaInjection(t *testing.T) {
	ent := testEntity()
	schemaJSON := `{"type":"object","properties":{"category":{"type":"string"}}}`
	pd := NewPromptData(ent, nil, schemaJSON)

	tmpl := "Respond with JSON matching: {{.OutputSchema}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Contains(t, result, `"type":"object"`)
}

func TestRender_MetadataKeys(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "")

	tmpl := "Available keys: {{.Entity.MetadataKeys}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	// Keys are sorted alphabetically.
	assert.Equal(t, "Available keys: airline, callsign, squawk", result)
}

func TestRender_MetadataJSON(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "")

	tmpl := "Metadata: {{.Entity.MetadataJSON}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Contains(t, result, `"airline":"United"`)
	assert.Contains(t, result, `"callsign":"UAL123"`)
}

func TestRender_InvalidTemplate(t *testing.T) {
	pd := PromptData{}
	_, err := Render("{{.Invalid.Template", pd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse prompt template")
}

func TestRender_TemplateExecutionError(t *testing.T) {
	pd := PromptData{}
	// Calling "call" on a non-function value triggers an execution error.
	_, err := Render("{{call .Entity.Name}}", pd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute prompt template")
}

func TestHash_ConsistentForSameInput(t *testing.T) {
	input := "Classify this entity based on metadata"
	h1 := Hash(input)
	h2 := Hash(input)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64) // SHA256 hex is 64 chars.
}

func TestHash_DifferentForDifferentInput(t *testing.T) {
	h1 := Hash("prompt A")
	h2 := Hash("prompt B")
	assert.NotEqual(t, h1, h2)
}

func TestNewPromptData_NilObservation(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "test-schema")

	assert.Equal(t, "ent-001", pd.Entity.ID)
	assert.Equal(t, "test-schema", pd.OutputSchema)
	assert.False(t, pd.HasCoordinates)
	assert.False(t, pd.HasAltitude)
	assert.Equal(t, float64(0), pd.Observation.Lat)
	assert.Equal(t, float64(0), pd.Observation.Lon)
	assert.Nil(t, pd.Observation.Metadata)
}

func TestNewPromptData_ObservationWithPosition(t *testing.T) {
	ent := testEntity()
	obs := testObservation()
	pd := NewPromptData(ent, obs, "")

	assert.True(t, pd.HasCoordinates)
	assert.True(t, pd.HasAltitude)
	assert.Equal(t, 40.7128, pd.Observation.Lat)
	assert.Equal(t, -74.0060, pd.Observation.Lon)
	assert.Equal(t, float64(10000), pd.Observation.Altitude)
}

func TestNewPromptData_ObservationWithoutPosition(t *testing.T) {
	ent := testEntity()
	obs := &domain.Observation{
		AltitudeM: 5000,
		Metadata:  map[string]string{"key": "val"},
	}
	pd := NewPromptData(ent, obs, "")

	assert.False(t, pd.HasCoordinates)
	assert.True(t, pd.HasAltitude)
	assert.Equal(t, float64(0), pd.Observation.Lat)
	assert.Equal(t, float64(0), pd.Observation.Lon)
}

func TestNewPromptData_ObservationZeroAltitude(t *testing.T) {
	ent := testEntity()
	obs := &domain.Observation{
		Position: &domain.GeoPoint{Lat: 1.0, Lon: 2.0},
	}
	pd := NewPromptData(ent, obs, "")

	assert.True(t, pd.HasCoordinates)
	assert.False(t, pd.HasAltitude)
}

func TestNewPromptData_EntityWithEmptyMetadata(t *testing.T) {
	ent := &domain.Entity{
		ID:       "ent-empty",
		Metadata: map[string]string{},
	}
	pd := NewPromptData(ent, nil, "")

	assert.Equal(t, "", pd.Entity.MetadataKeys)
	assert.Equal(t, "{}", pd.Entity.MetadataJSON)
}

func TestNewPromptData_LayerTypeSet(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "")

	assert.Equal(t, "flights_commercial", pd.LayerType)
	assert.Equal(t, "flights_commercial", pd.Entity.LayerType)
}

func TestRender_HasCoordinatesConditional(t *testing.T) {
	ent := testEntity()
	obs := testObservation()
	pd := NewPromptData(ent, obs, "")

	tmpl := "{{if .HasCoordinates}}Located at {{.Observation.Lat}},{{.Observation.Lon}}{{else}}No position{{end}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Contains(t, result, "Located at")
}

func TestRender_HasCoordinatesFalseConditional(t *testing.T) {
	ent := testEntity()
	pd := NewPromptData(ent, nil, "")

	tmpl := "{{if .HasCoordinates}}Located{{else}}No position{{end}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Equal(t, "No position", result)
}

func TestRender_ObservationMetadataAccess(t *testing.T) {
	ent := testEntity()
	obs := testObservation()
	pd := NewPromptData(ent, obs, "")

	tmpl := "Speed: {{index .Observation.Metadata \"speed\"}}"
	result, err := Render(tmpl, pd)
	require.NoError(t, err)
	assert.Equal(t, "Speed: 450", result)
}
