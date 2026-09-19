package geocoder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNominatim_Geocode_Success(t *testing.T) {
	// Mock Nominatim /search endpoint returning a Kyiv result.
	kyivResult := []nominatimSearchResult{
		{
			Lat:         "50.4501",
			Lon:         "30.5234",
			DisplayName: "Kyiv, Ukraine",
			Importance:  0.9,
			Address: nominatimAddress{
				City:        "Kyiv",
				State:       "Kyiv City",
				CountryCode: "ua",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "json", r.URL.Query().Get("format"))
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		assert.Equal(t, "1", r.URL.Query().Get("addressdetails"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(kyivResult)
	}))
	defer srv.Close()

	n := NewNominatim(srv.URL, "test-agent/1.0")
	result, err := n.Geocode(context.Background(), "Kyiv")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.InDelta(t, 50.4501, result.Lat, 0.0001)
	assert.InDelta(t, 30.5234, result.Lon, 0.0001)
	assert.Equal(t, "Kyiv", result.City)
	assert.Equal(t, "nominatim", result.Source)
	assert.Equal(t, "UA", result.Country)
}

func TestNominatim_Geocode_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]nominatimSearchResult{})
	}))
	defer srv.Close()

	n := NewNominatim(srv.URL, "test-agent/1.0")
	result, err := n.Geocode(context.Background(), "NonExistentPlaceXYZ")

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestNominatim_Geocode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewNominatim(srv.URL, "test-agent/1.0")
	result, err := n.Geocode(context.Background(), "Anywhere")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestNominatim_Name(t *testing.T) {
	n := NewNominatim("", "test-agent/1.0")
	assert.Equal(t, "nominatim", n.Name())
}

func TestNominatim_ReverseGeocode_Success(t *testing.T) {
	reverseResult := nominatimReverseResult{
		Lat:         "50.4501",
		Lon:         "30.5234",
		DisplayName: "Kyiv, Ukraine",
		Address: nominatimAddress{
			City:        "Kyiv",
			State:       "Kyiv City",
			CountryCode: "ua",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/reverse", r.URL.Path)
		assert.Equal(t, "json", r.URL.Query().Get("format"))
		assert.Equal(t, "1", r.URL.Query().Get("addressdetails"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reverseResult)
	}))
	defer srv.Close()

	n := NewNominatim(srv.URL, "test-agent/1.0")
	result, err := n.ReverseGeocode(context.Background(), 50.4501, 30.5234)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "Kyiv", result.City)
	assert.Equal(t, "nominatim", result.Source)
}
