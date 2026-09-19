// Package enrichment implements the AI enrichment pipeline.
package enrichment

import "time"

// Job represents an enrichment job published to NATS after entity persistence.
// The feeder publishes these unconditionally -- workers handle filtering and caching.
type Job struct {
	EntityID    string    `json:"entity_id"` // DB UUID of the entity (authoritative)
	ExternalID  string    `json:"external_id"`
	SourceName  string    `json:"source_name"`
	LayerType   string    `json:"layer_type"`
	PublishedAt time.Time `json:"published_at"`

	// ObservationID is the feeder-assigned UUID for the observation that
	// triggered this job. It is ADVISORY ONLY and must NOT be used for DB
	// operations: the feeder generates speculative UUIDs in buildObservationArrays
	// that may not exist in the DB (ON CONFLICT DO NOTHING skips the insert)
	// or may not match the actual row ID (ON CONFLICT DO UPDATE preserves
	// the original ID). The worker fetches the authoritative observation via
	// obsRepo.GetLatest and uses obs.ID for all database operations.
	ObservationID string `json:"observation_id,omitempty"`
}
