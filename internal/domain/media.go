package domain

import "context"

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

// MediaPlaybackAction is a fully resolved, source-declared playback
// notification. It is built from a trusted source definition plus stored entity
// metadata — never from anything a caller supplied — and carries a relative
// path only, so the origin stays the source's own.
type MediaPlaybackAction struct {
	LayerType LayerType
	Name      string
	Method    string
	Path      string
}

// MediaActionResolver turns a layer's declared action name plus the entity's
// merged metadata into a concrete request. Implemented by the declarative
// ingest package and injected at the composition root.
type MediaActionResolver interface {
	ResolveMediaAction(layerType LayerType, name string, metadata map[string]string) (*MediaPlaybackAction, error)
}

// MediaActionExecutor performs a resolved notification against the owning
// source's origin. It reports success or failure and never returns upstream
// content, so no stream URL can reach the caller through this path.
type MediaActionExecutor interface {
	ExecuteMediaAction(ctx context.Context, action *MediaPlaybackAction) error
}
