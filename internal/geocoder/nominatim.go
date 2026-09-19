package geocoder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	nominatimDefaultBaseURL = "https://nominatim.openstreetmap.org"
	nominatimProviderName   = "nominatim"
)

// nominatimAddress holds the address fields returned by Nominatim.
type nominatimAddress struct {
	City        string `json:"city"`
	Town        string `json:"town"`
	Village     string `json:"village"`
	State       string `json:"state"`
	CountryCode string `json:"country_code"`
}

// nominatimSearchResult represents a single entry in the /search response array.
type nominatimSearchResult struct {
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	DisplayName string           `json:"display_name"`
	Importance  float64          `json:"importance"`
	Address     nominatimAddress `json:"address"`
}

// nominatimReverseResult represents the /reverse response object.
type nominatimReverseResult struct {
	Lat         string           `json:"lat"`
	Lon         string           `json:"lon"`
	DisplayName string           `json:"display_name"`
	Address     nominatimAddress `json:"address"`
}

// Nominatim is a Geocoder backed by the Nominatim / OpenStreetMap API.
type Nominatim struct {
	baseURL   string
	userAgent string
	client    *http.Client
}

// NewNominatim creates a new Nominatim geocoder. If baseURL is empty, the public
// OpenStreetMap Nominatim endpoint is used. userAgent must comply with Nominatim's
// usage policy (https://operations.osmfoundation.org/policies/nominatim/).
func NewNominatim(baseURL, userAgent string) *Nominatim {
	if baseURL == "" {
		baseURL = nominatimDefaultBaseURL
	}
	return &Nominatim{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: userAgent,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Name implements Geocoder.
func (n *Nominatim) Name() string {
	return nominatimProviderName
}

// Geocode implements Geocoder. Returns nil, nil when no results are found.
func (n *Nominatim) Geocode(ctx context.Context, query string) (*Result, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("limit", "1")
	params.Set("addressdetails", "1")

	endpoint := fmt.Sprintf("%s/search?%s", n.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("nominatim: build request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim: search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim: search returned status %d", resp.StatusCode)
	}

	var results []nominatimSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("nominatim: decode search response: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	r := results[0]
	return n.searchToResult(r)
}

// ReverseGeocode implements Geocoder. Returns nil, nil when no result is found.
func (n *Nominatim) ReverseGeocode(ctx context.Context, lat, lon float64) (*Result, error) {
	params := url.Values{}
	params.Set("lat", strconv.FormatFloat(lat, 'f', -1, 64))
	params.Set("lon", strconv.FormatFloat(lon, 'f', -1, 64))
	params.Set("format", "json")
	params.Set("addressdetails", "1")

	endpoint := fmt.Sprintf("%s/reverse?%s", n.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("nominatim: build reverse request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nominatim: reverse request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim: reverse returned status %d", resp.StatusCode)
	}

	var result nominatimReverseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("nominatim: decode reverse response: %w", err)
	}

	return n.reverseToResult(result)
}

// searchToResult converts a nominatimSearchResult to a geocoder Result.
func (n *Nominatim) searchToResult(r nominatimSearchResult) (*Result, error) {
	lat, err := strconv.ParseFloat(r.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("nominatim: parse lat: %w", err)
	}
	lon, err := strconv.ParseFloat(r.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("nominatim: parse lon: %w", err)
	}
	return &Result{
		Lat:        lat,
		Lon:        lon,
		Country:    strings.ToUpper(r.Address.CountryCode),
		Region:     r.Address.State,
		City:       resolveCity(r.Address),
		Confidence: r.Importance,
		Source:     nominatimProviderName,
	}, nil
}

// reverseToResult converts a nominatimReverseResult to a geocoder Result.
func (n *Nominatim) reverseToResult(r nominatimReverseResult) (*Result, error) {
	lat, err := strconv.ParseFloat(r.Lat, 64)
	if err != nil {
		return nil, fmt.Errorf("nominatim: parse lat: %w", err)
	}
	lon, err := strconv.ParseFloat(r.Lon, 64)
	if err != nil {
		return nil, fmt.Errorf("nominatim: parse lon: %w", err)
	}
	return &Result{
		Lat:     lat,
		Lon:     lon,
		Country: strings.ToUpper(r.Address.CountryCode),
		Region:  r.Address.State,
		City:    resolveCity(r.Address),
		Source:  nominatimProviderName,
	}, nil
}

// resolveCity returns the most specific populated-place name from an address,
// preferring city > town > village.
func resolveCity(addr nominatimAddress) string {
	if addr.City != "" {
		return addr.City
	}
	if addr.Town != "" {
		return addr.Town
	}
	return addr.Village
}
