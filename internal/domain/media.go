package domain

// MediaConfig describes explicitly declared entity media. Media bytes are never observations.
type MediaConfig struct {
	ID             string
	Kind           string
	Label          string
	URLKey         string
	AttributionKey string
	AllowedOrigins []string
	PlaybackAction string
	Snapshot       *SnapshotMediaConfig
}

type SnapshotMediaConfig struct {
	RefreshIntervalSeconds int32
	CacheBustParam         string
}
