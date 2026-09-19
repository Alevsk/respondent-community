package parse

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Alevsk/respondent/internal/domain"
)

// ADSBLolResponse represents the adsb.lol v2 API response.
type ADSBLolResponse struct {
	AC    []ADSBLolAircraft `json:"ac"`
	Now   float64           `json:"now"`
	Total int               `json:"total"`
	Msg   string            `json:"msg"`
}

// ADSBLolAircraft represents a single aircraft in the adsb.lol v2 response.
// Note: alt_baro can be a number or the string "ground", so we use interface{}.
type ADSBLolAircraft struct {
	Hex     string      `json:"hex"`
	Flight  string      `json:"flight"`
	Lat     float64     `json:"lat"`
	Lon     float64     `json:"lon"`
	AltBaro interface{} `json:"alt_baro"`
	AltGeom interface{} `json:"alt_geom"`
	GS      float64     `json:"gs"`
	Track   float64     `json:"track"`
	Squawk  string      `json:"squawk"`
	Type    string      `json:"type"`
}

// AltBaroFeet returns the barometric altitude in feet, or 0 if unavailable/ground.
func (a ADSBLolAircraft) AltBaroFeet() float64 {
	switch v := a.AltBaro.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

// AltGeomFeet returns the GPS geometric altitude in feet, or 0 if unavailable.
func (a ADSBLolAircraft) AltGeomFeet() float64 {
	switch v := a.AltGeom.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

// AltFeet returns the best available altitude in feet: alt_baro if > 0, else alt_geom, else 0.
func (a ADSBLolAircraft) AltFeet() float64 {
	if alt := a.AltBaroFeet(); alt > 0 {
		return alt
	}
	return a.AltGeomFeet()
}

// ParseADSBLolResponse parses the adsb.lol v2 API JSON response into domain
// entities and observations. The layerType parameter determines the LayerType
// and entity ID prefix assigned to each entity.
func ParseADSBLolResponse(data []byte, layerType string) ([]*domain.Entity, []*domain.Observation, error) {
	var response ADSBLolResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, nil, err
	}

	entities := make([]*domain.Entity, 0, len(response.AC))
	observations := make([]*domain.Observation, 0, len(response.AC))

	for _, ac := range response.AC {
		hex := strings.ToLower(strings.TrimSpace(ac.Hex))
		if hex == "" {
			continue
		}

		if ac.Lat == 0 && ac.Lon == 0 {
			continue
		}

		externalID := hex
		entityID := domain.EntityID(domain.LayerType(layerType), externalID)

		callsign := strings.TrimSpace(ac.Flight)
		if callsign == "" {
			callsign = hex
		}

		altMeters := ac.AltFeet() * 0.3048

		entity := &domain.Entity{
			ID:         entityID,
			ExternalID: externalID,
			LayerType:  layerType,
			Name:       callsign,
			Metadata: map[string]string{
				"callsign": callsign,
				"icao24":   hex,
				"source":   "adsb_lol",
			},
		}

		observation := &domain.Observation{
			ID:        uuid.New().String(),
			EntityID:  entityID,
			Timestamp: time.Now(),
			Position: &domain.GeoPoint{
				Lat: ac.Lat,
				Lon: ac.Lon,
				Alt: altMeters,
			},
			AltitudeM: altMeters,
			Velocity:  make(map[string]float64),
		}

		if ac.GS > 0 {
			observation.Velocity["speed"] = ac.GS * 0.514444 // knots to m/s
		}
		if ac.Track > 0 {
			observation.Velocity["heading"] = ac.Track
		}

		entities = append(entities, entity)
		observations = append(observations, observation)
	}

	return entities, observations, nil
}
