package domain

import "time"

// CameraFeed represents a CCTV camera
type CameraFeed struct {
	ID                    string
	Name                  string
	Provider              string
	Location              GeoPoint
	StreamURL             string
	UpdateIntervalSeconds int32
	Metadata              map[string]string
	CreatedAt             time.Time
	Calibration           *Calibration
}

// Calibration represents camera projection parameters
type Calibration struct {
	ID           string
	CameraFeedID string
	Heading      float64
	Pitch        float64
	Roll         float64
	FOV          float64
	RangeM       float64
	HeightM      float64
	OffsetNorthM float64
	OffsetEastM  float64
	UpdatedAt    time.Time
}
