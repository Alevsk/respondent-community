package domain

import "time"

// Camera represents a 3D camera state
type Camera struct {
	Position GeoPoint
	Heading  float64
	Pitch    float64
	Roll     float64
	Range    float64
}

// Scene represents a saved camera state, layers, and filters
type Scene struct {
	ID             string
	Name           string
	Camera         Camera
	LayerToggles   []LayerToggle
	FilterPresetID string
	CreatedAt      time.Time
}

// Location represents a saved location (city or landmark)
type Location struct {
	ID       string
	Name     string
	Type     string // city, landmark
	CityID   string // Parent city for landmarks
	Position GeoPoint
	Region   string // US, Europe, Asia, etc.
}
