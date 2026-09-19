package domain

import "time"

// FilterPreset represents a shader configuration
type FilterPreset struct {
	ID        string
	Name      string
	Style     string // CRT/NVG/FLIR/NORMAL/Anime/Noir/Snow/AI
	Params    map[string]float64
	CreatedAt time.Time
}
