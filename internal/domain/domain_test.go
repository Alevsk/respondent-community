// Package domain_test provides unit tests for domain types.
package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestEntityID(t *testing.T) {
	tests := []struct {
		name       string
		layerType  string
		externalID string
		want       string
	}{
		{
			name:       "standard entity ID",
			layerType:  "flights_commercial",
			externalID: "ABC123",
			want:       "flights_commercial:ABC123",
		},
		{
			name:       "satellite entity ID",
			layerType:  "satellites",
			externalID: "25544",
			want:       "satellites:25544",
		},
		{
			name:       "empty external ID",
			layerType:  "flights_military",
			externalID: "",
			want:       "flights_military:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.EntityID(domain.LayerType(tt.layerType), tt.externalID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseEntityID(t *testing.T) {
	tests := []struct {
		name     string
		entityID string
		wantLT   domain.LayerType
		wantExt  string
	}{
		{
			name:     "standard entity ID",
			entityID: "flights_commercial:ABC123",
			wantLT:   domain.LayerType("flights_commercial"),
			wantExt:  "ABC123",
		},
		{
			name:     "satellite entity ID",
			entityID: "satellites:25544",
			wantLT:   domain.LayerType("satellites"),
			wantExt:  "25544",
		},
		{
			name:     "invalid format - no colon",
			entityID: "invalid",
			wantLT:   domain.LayerType(""),
			wantExt:  "",
		},
		{
			name:     "invalid format - multiple colons",
			entityID: "layer:type:id",
			wantLT:   domain.LayerType("layer"),
			wantExt:  "type:id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lt, ext := domain.ParseEntityID(tt.entityID)
			assert.Equal(t, tt.wantLT, lt)
			assert.Equal(t, tt.wantExt, ext)
		})
	}
}

func TestFormatLayerName(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		want       string
	}{
		{
			name:       "flights commercial",
			sourceType: "flights_commercial",
			want:       "Flights Commercial",
		},
		{
			name:       "satellites",
			sourceType: "celes_trak_satellites",
			want:       "Celes Trak Satellites",
		},
		{
			name:       "single word",
			sourceType: "satellites",
			want:       "Satellites",
		},
		{
			name:       "empty string",
			sourceType: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.FormatLayerName(tt.sourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseLayerName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Flights Commercial",
			want: "flights_commercial",
		},
		{
			name: "Satellites",
			want: "satellites",
		},
		{
			name: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ParseLayerName(tt.name)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRegistry_AllLayerTypes_Membership(t *testing.T) {
	// A layer type is "valid" when it has a registered source. This is resolved
	// from a registry instance — there is no global state.
	reg := domain.NewDynamicSourceRegistry()
	knownTypes := []struct{ src, layer string }{
		{"src_flights_commercial", "flights_commercial"},
		{"src_satellites", "satellites"},
		{"src_earthquakes", "earthquakes"},
	}
	for _, kt := range knownTypes {
		reg.Register(domain.SourceType(kt.src), domain.LayerType(kt.layer))
	}

	registered := make(map[domain.LayerType]bool)
	for _, lt := range reg.AllLayerTypes() {
		registered[lt] = true
	}

	assert.True(t, registered["flights_commercial"])
	assert.True(t, registered["satellites"])
	assert.True(t, registered["earthquakes"])
	assert.False(t, registered["invalid_type"])
}

func TestDomainError(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := domain.NewNotFoundError("entity not found", nil)
		assert.Contains(t, err.Error(), "NOT_FOUND")
		assert.Contains(t, err.Error(), "entity not found")
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := domain.NewInternalError("operation failed", cause)
		assert.Contains(t, err.Error(), "INTERNAL_ERROR")
		assert.Contains(t, err.Error(), "operation failed")
		assert.Contains(t, err.Error(), "underlying error")
	})

	t.Run("unwrap error", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := domain.NewInternalError("operation failed", cause)
		unwrapped := errors.Unwrap(err)
		require.NotNil(t, unwrapped)
		assert.Equal(t, cause, unwrapped)
	})

	t.Run("error with context", func(t *testing.T) {
		err := domain.NewNotFoundError("entity not found", nil).
			WithContext(map[string]interface{}{
				"entity_id":  "123",
				"layer_type": "flights_commercial",
			})
		assert.NotNil(t, err.Context)
		assert.Equal(t, "123", err.Context["entity_id"])
	})

	t.Run("is not found", func(t *testing.T) {
		err := domain.NewNotFoundError("not found", nil)
		assert.True(t, domain.IsNotFound(err))
		assert.False(t, domain.IsInternal(err))
	})

	t.Run("is internal", func(t *testing.T) {
		err := domain.NewInternalError("internal error", nil)
		assert.True(t, domain.IsInternal(err))
		assert.False(t, domain.IsNotFound(err))
	})

	t.Run("is invalid input", func(t *testing.T) {
		err := domain.NewInvalidInputError("invalid input", nil)
		assert.True(t, domain.IsInvalidInput(err))
		assert.False(t, domain.IsNotFound(err))
	})

	t.Run("non-domain error", func(t *testing.T) {
		err := errors.New("standard error")
		assert.False(t, domain.IsNotFound(err))
		assert.False(t, domain.IsInternal(err))
		assert.False(t, domain.IsInvalidInput(err))
	})
}

func TestEntity(t *testing.T) {
	now := time.Now()
	entity := &domain.Entity{
		ID:         "test-1",
		ExternalID: "ext-1",
		LayerType:  "flights_commercial",
		Name:       "Test Flight",
		Metadata: map[string]string{
			"icao24": "ABC123",
			"origin": "KJFK",
		},
		CreatedAt: now,
	}

	assert.Equal(t, "test-1", entity.ID)
	assert.Equal(t, "ext-1", entity.ExternalID)
	assert.Equal(t, "flights_commercial", entity.LayerType)
	assert.Equal(t, "Test Flight", entity.Name)
	assert.Equal(t, "ABC123", entity.Metadata["icao24"])
	assert.Equal(t, now, entity.CreatedAt)
}

func TestObservation(t *testing.T) {
	now := time.Now()
	obs := &domain.Observation{
		ID:        "obs-1",
		EntityID:  "entity-1",
		Timestamp: now,
		Position: &domain.GeoPoint{
			Lat: 37.7749,
			Lon: -122.4194,
			Alt: 10000,
		},
		AltitudeM: 10000,
		Velocity: map[string]float64{
			"speed":   250.5,
			"heading": 180.0,
		},
		Metadata: map[string]string{
			"source": "opensky",
		},
		CreatedAt: now,
	}

	assert.Equal(t, "obs-1", obs.ID)
	assert.Equal(t, "entity-1", obs.EntityID)
	assert.Equal(t, 37.7749, obs.Position.Lat)
	assert.Equal(t, -122.4194, obs.Position.Lon)
	assert.Equal(t, 10000.0, obs.Position.Alt)
	assert.Equal(t, 250.5, obs.Velocity["speed"])
}

func TestGeoPoint(t *testing.T) {
	point := domain.GeoPoint{
		Lat: 40.7128,
		Lon: -74.0060,
		Alt: 10.5,
	}

	assert.Equal(t, 40.7128, point.Lat)
	assert.Equal(t, -74.0060, point.Lon)
	assert.Equal(t, 10.5, point.Alt)
}

func TestGetLayerStyle(t *testing.T) {
	// Register display configs for the layer types under test.
	reg := domain.NewDynamicSourceRegistry()
	reg.RegisterWithDisplay("src_flights_commercial_style", "flights_commercial",
		&domain.LayerDisplayConfig{Style: &domain.StyleConfig{Color: "#00ff9d", PointSize: 8}})
	reg.RegisterWithDisplay("src_earthquakes_style", "earthquakes",
		&domain.LayerDisplayConfig{Style: &domain.StyleConfig{Color: "#ff006e", PointSize: 10}})
	reg.RegisterWithDisplay("src_satellites_style", "satellites",
		&domain.LayerDisplayConfig{Style: &domain.StyleConfig{Color: "#00ffff", PointSize: 6}})

	tests := []struct {
		name          string
		layerType     domain.LayerType
		wantColor     string
		wantPointSize int32
	}{
		{
			name:          "known layer type flights_commercial",
			layerType:     domain.LayerType("flights_commercial"),
			wantColor:     "#00ff9d",
			wantPointSize: 8,
		},
		{
			name:          "known layer type earthquakes",
			layerType:     domain.LayerType("earthquakes"),
			wantColor:     "#ff006e",
			wantPointSize: 10,
		},
		{
			name:          "known layer type satellites",
			layerType:     domain.LayerType("satellites"),
			wantColor:     "#00ffff",
			wantPointSize: 6,
		},
		{
			name:          "unknown layer type falls back to default",
			layerType:     domain.LayerType("disaster_alerts"),
			wantColor:     domain.DefaultLayerStyle.Color,
			wantPointSize: domain.DefaultLayerStyle.PointSize,
		},
		{
			name:          "empty layer type falls back to default",
			layerType:     domain.LayerType(""),
			wantColor:     domain.DefaultLayerStyle.Color,
			wantPointSize: domain.DefaultLayerStyle.PointSize,
		},
		{
			name:          "arbitrary unknown type falls back to default",
			layerType:     domain.LayerType("weather_alerts"),
			wantColor:     domain.DefaultLayerStyle.Color,
			wantPointSize: domain.DefaultLayerStyle.PointSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := reg.GetLayerStyle(tt.layerType)
			assert.Equal(t, tt.wantColor, style.Color)
			assert.Equal(t, tt.wantPointSize, style.PointSize)
		})
	}
}

func TestDefaultLayerStyle_NonZero(t *testing.T) {
	// DefaultLayerStyle must have non-zero PointSize to avoid invisible rendering.
	assert.NotEmpty(t, domain.DefaultLayerStyle.Color, "DefaultLayerStyle.Color must not be empty")
	assert.Greater(t, domain.DefaultLayerStyle.PointSize, int32(0), "DefaultLayerStyle.PointSize must be > 0")
}

func TestEntityUpdate(t *testing.T) {
	entity := &domain.Entity{ID: "1"}
	observation := &domain.Observation{ID: "obs-1"}

	update := &domain.EntityUpdate{
		Type:        domain.EntityUpdateCreate,
		Entity:      entity,
		Observation: observation,
	}

	assert.Equal(t, domain.EntityUpdateCreate, update.Type)
	assert.Equal(t, entity, update.Entity)
	assert.Equal(t, observation, update.Observation)
}

func TestObservation_ComputeContentHash(t *testing.T) {
	t.Run("with position", func(t *testing.T) {
		now := time.Now()
		obs := &domain.Observation{
			ID:        "obs-1",
			EntityID:  "entity-1",
			Timestamp: now,
			Position: &domain.GeoPoint{
				Lat: 37.7749,
				Lon: -122.4194,
				Alt: 10000,
			},
			AltitudeM: 10000,
			Velocity: map[string]float64{
				"speed":   250.5,
				"heading": 180.0,
			},
			Metadata: map[string]string{
				"source": "opensky",
			},
		}
		hash1, err := obs.ComputeContentHash()
		assert.NoError(t, err)
		assert.Len(t, hash1, 64)
		hash2, err := obs.ComputeContentHash()
		assert.NoError(t, err)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("nil position", func(t *testing.T) {
		now := time.Now()
		obs := &domain.Observation{
			ID:        "obs-2",
			EntityID:  "entity-2",
			Timestamp: now,
			Position:  nil,
			AltitudeM: 0,
			Velocity:  nil,
			Metadata:  nil,
		}
		hash, err := obs.ComputeContentHash()
		assert.NoError(t, err)
		assert.Len(t, hash, 64)
	})

	t.Run("different observations produce different hashes", func(t *testing.T) {
		now := time.Now()
		obs1 := &domain.Observation{
			Timestamp: now,
			Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0, Alt: 0},
			AltitudeM: 1000,
		}
		obs2 := &domain.Observation{
			Timestamp: now,
			Position:  &domain.GeoPoint{Lat: 41.0, Lon: -74.0, Alt: 0},
			AltitudeM: 1000,
		}
		hash1, err := obs1.ComputeContentHash()
		assert.NoError(t, err)
		hash2, err := obs2.ComputeContentHash()
		assert.NoError(t, err)
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestDefaultComputeLevel(t *testing.T) {
	t.Run("no thresholds uses raw value", func(t *testing.T) {
		fn := domain.DefaultComputeLevel(nil, 5)
		assert.Equal(t, int32(3), fn("3", "0"))
		assert.Equal(t, int32(0), fn("0", "0"))
		assert.Equal(t, int32(5), fn("5", "0"))
	})

	t.Run("clamps to max level", func(t *testing.T) {
		fn := domainComputeLevel(nil, 3)
		assert.Equal(t, int32(3), fn("10", "0"))
		assert.Equal(t, int32(3), fn("100", "0"))
	})

	t.Run("clamps negative to zero", func(t *testing.T) {
		fn := domain.DefaultComputeLevel(nil, 5)
		assert.Equal(t, int32(0), fn("-5", "0"))
	})

	t.Run("invalid parse returns zero", func(t *testing.T) {
		fn := domain.DefaultComputeLevel(nil, 5)
		assert.Equal(t, int32(0), fn("invalid", "0"))
	})

	t.Run("with thresholds", func(t *testing.T) {
		thresholds := []float64{1.0, 2.0, 3.0}
		fn := domain.DefaultComputeLevel(thresholds, 5)
		assert.Equal(t, int32(0), fn("0.5", "0"))
		assert.Equal(t, int32(0), fn("1.0", "0"))
		assert.Equal(t, int32(1), fn("2.0", "0"))
		assert.Equal(t, int32(2), fn("3.0", "0"))
		assert.Equal(t, int32(2), fn("4.0", "0"))
	})

	t.Run("thresholds clamp to max level", func(t *testing.T) {
		thresholds := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
		fn := domain.DefaultComputeLevel(thresholds, 3)
		assert.Equal(t, int32(3), fn("10.0", "0"))
	})

	t.Run("zero or negative max level defaults to 5", func(t *testing.T) {
		fn := domain.DefaultComputeLevel(nil, 0)
		assert.Equal(t, int32(5), fn("5", "0"))
		fn2 := domain.DefaultComputeLevel(nil, -1)
		assert.Equal(t, int32(5), fn2("5", "0"))
	})

	t.Run("thresholds with invalid parse returns zero", func(t *testing.T) {
		thresholds := []float64{1.0, 2.0}
		fn := domain.DefaultComputeLevel(thresholds, 5)
		assert.Equal(t, int32(0), fn("bad", "0"))
	})
}

func domainComputeLevel(thresholds []float64, maxLevel int) func(string, string) int32 {
	return domain.DefaultComputeLevel(thresholds, maxLevel)
}

func TestDefaultComputeSummary(t *testing.T) {
	fn := domain.DefaultComputeSummary()
	assert.Equal(t, "Quiet", fn(nil, 0))
	assert.Equal(t, "Minor Activity", fn(nil, 1))
	assert.Equal(t, "Moderate", fn(nil, 2))
	assert.Equal(t, "Strong", fn(nil, 3))
	assert.Equal(t, "Severe", fn(nil, 4))
	assert.Equal(t, "Extreme", fn(nil, 5))
	assert.Equal(t, "Extreme", fn(nil, 10))
	assert.Equal(t, "Unknown", fn(nil, -1))
}

func TestNewUnavailableError(t *testing.T) {
	err := domain.NewUnavailableError("resource unavailable", nil)
	assert.Contains(t, err.Error(), "UNAVAILABLE")
	assert.Contains(t, err.Error(), "resource unavailable")
	assert.True(t, domain.IsUnavailable(err))
	assert.False(t, domain.IsNotFound(err))
}

func TestNewTimeoutError(t *testing.T) {
	cause := errors.New("timeout cause")
	err := domain.NewTimeoutError("operation timed out", cause)
	assert.Contains(t, err.Error(), "TIMEOUT")
	assert.Contains(t, err.Error(), "operation timed out")
	assert.Contains(t, err.Error(), "timeout cause")
	assert.True(t, domain.IsTimeout(err))
	assert.False(t, domain.IsNotFound(err))
}

func TestNewConflictError(t *testing.T) {
	err := domain.NewConflictError("resource conflict", nil)
	assert.Contains(t, err.Error(), "CONFLICT")
	assert.Contains(t, err.Error(), "resource conflict")
	assert.True(t, domain.IsConflict(err))
	assert.False(t, domain.IsNotFound(err))
}

func TestIsUnavailable(t *testing.T) {
	err := domain.NewUnavailableError("unavailable", nil)
	assert.True(t, domain.IsUnavailable(err))
	assert.False(t, domain.IsUnavailable(errors.New("standard error")))
}

func TestIsTimeout(t *testing.T) {
	err := domain.NewTimeoutError("timeout", nil)
	assert.True(t, domain.IsTimeout(err))
	assert.False(t, domain.IsTimeout(errors.New("standard error")))
}

func TestIsConflict(t *testing.T) {
	err := domain.NewConflictError("conflict", nil)
	assert.True(t, domain.IsConflict(err))
	assert.False(t, domain.IsConflict(errors.New("standard error")))
}

func TestGetDomainError(t *testing.T) {
	t.Run("extracts domain error", func(t *testing.T) {
		err := domain.NewNotFoundError("not found", nil)
		domainErr := domain.GetDomainError(err)
		require.NotNil(t, domainErr)
		assert.Equal(t, domain.ErrCodeNotFound, domainErr.Code)
	})

	t.Run("returns nil for non-domain error", func(t *testing.T) {
		err := errors.New("standard error")
		domainErr := domain.GetDomainError(err)
		assert.Nil(t, domainErr)
	})
}

func TestSourceType_String(t *testing.T) {
	st := domain.SourceType("test_source")
	assert.Equal(t, "test_source", st.String())
}

func TestLayerType_String(t *testing.T) {
	lt := domain.LayerType("test_layer")
	assert.Equal(t, "test_layer", lt.String())
}

func TestLayerType_SourceTypes(t *testing.T) {
	reg := domain.NewDynamicSourceRegistry()
	lt := domain.LayerType("test_layer_sourcetypes")
	reg.Register(domain.SourceType("source_one"), lt)
	reg.Register(domain.SourceType("source_two"), lt)

	sources := reg.LookupSourceTypes(lt)
	assert.Len(t, sources, 2)
}

func TestAllLayerTypes(t *testing.T) {
	reg := domain.NewDynamicSourceRegistry()
	reg.Register(domain.SourceType("all_lt_test_src"), domain.LayerType("all_lt_test_layer"))

	layerTypes := reg.AllLayerTypes()
	assert.Equal(t, []domain.LayerType{"all_lt_test_layer"}, layerTypes)
}

func TestLookupLayerType_Registered(t *testing.T) {
	reg := domain.NewDynamicSourceRegistry()
	st := domain.SourceType("srclayer_test_src")
	lt := domain.LayerType("srclayer_test_layer")
	reg.Register(st, lt)

	got, ok := reg.LookupLayerType(st)
	assert.True(t, ok)
	assert.Equal(t, lt, got)
}

func TestLookupLayerType_NotRegistered(t *testing.T) {
	reg := domain.NewDynamicSourceRegistry()
	_, ok := reg.LookupLayerType(domain.SourceType("unregistered_source"))
	assert.False(t, ok)
}

func TestIndicatorValueSpec(t *testing.T) {
	spec := domain.IndicatorValueSpec{
		Key:             "magnitude",
		Label:           "Magnitude",
		SourceField:     "mag",
		Unit:            "",
		MaxLevel:        5,
		LevelThresholds: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
	}
	assert.Equal(t, "magnitude", spec.Key)
	assert.Equal(t, "Magnitude", spec.Label)
	assert.Equal(t, "mag", spec.SourceField)
	assert.Equal(t, 5, spec.MaxLevel)
	assert.Len(t, spec.LevelThresholds, 5)
}

func TestIndicatorSpec(t *testing.T) {
	spec := domain.IndicatorSpec{
		Values: []domain.IndicatorValueSpec{
			{Key: "value1", Label: "Value 1"},
			{Key: "value2", Label: "Value 2"},
		},
		ComputeSummary: func(m map[string]string, level int32) string {
			return "summary"
		},
	}
	assert.Len(t, spec.Values, 2)
	assert.NotNil(t, spec.ComputeSummary)
	assert.Equal(t, "summary", spec.ComputeSummary(nil, 0))
}

func TestEntityUpdateType(t *testing.T) {
	assert.Equal(t, domain.EntityUpdateType(0), domain.EntityUpdateUnknown)
	assert.Equal(t, domain.EntityUpdateType(1), domain.EntityUpdateCreate)
	assert.Equal(t, domain.EntityUpdateType(2), domain.EntityUpdateUpdate)
	assert.Equal(t, domain.EntityUpdateType(3), domain.EntityUpdateDelete)
}

func TestEntitySnapshot(t *testing.T) {
	entity := domain.Entity{ID: "e1", LayerType: "test"}
	obs := domain.Observation{ID: "o1", EntityID: "e1"}
	snapshot := domain.EntitySnapshot{Entity: entity, Observation: obs}
	assert.Equal(t, "e1", snapshot.Entity.ID)
	assert.Equal(t, "o1", snapshot.Observation.ID)
}

func TestEntitySearchResult(t *testing.T) {
	entity := domain.Entity{ID: "e1", LayerType: "test"}
	obs := domain.Observation{ID: "o1", EntityID: "e1"}
	result := domain.EntitySearchResult{Entity: entity, LatestObservation: &obs}
	assert.Equal(t, "e1", result.Entity.ID)
	assert.Equal(t, "o1", result.LatestObservation.ID)

	resultNil := domain.EntitySearchResult{Entity: entity, LatestObservation: nil}
	assert.Nil(t, resultNil.LatestObservation)
}

func TestEntityType(t *testing.T) {
	assert.Equal(t, domain.EntityType("geo_entity"), domain.EntityTypeGeo)
	assert.Equal(t, domain.EntityType("global_indicator"), domain.EntityTypeIndicator)
}

func TestSourceConfig(t *testing.T) {
	config := domain.SourceConfig{
		Type:     domain.SourceType("test_source"),
		APIURL:   "https://api.example.com",
		Interval: 60,
		Timeout:  30,
		Options:  map[string]interface{}{"key": "value"},
	}
	assert.Equal(t, domain.SourceType("test_source"), config.Type)
	assert.Equal(t, "https://api.example.com", config.APIURL)
	assert.Equal(t, 60, config.Interval)
	assert.Equal(t, 30, config.Timeout)
	assert.Equal(t, "value", config.Options["key"])
}
