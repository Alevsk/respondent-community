// Package domain defines repository interfaces for dependency injection.
package domain

import (
	"context"
	"time"
)

// EntityRepository handles entity persistence
type EntityRepository interface {
	Create(ctx context.Context, entity *Entity) error
	CreateBatch(ctx context.Context, entities []*Entity) error
	GetByID(ctx context.Context, id string) (*Entity, error)
	GetByIDs(ctx context.Context, ids []string) ([]*Entity, error)
	GetByExternalID(ctx context.Context, layerType, externalID string) (*Entity, error)
	GetByExternalIDs(ctx context.Context, layerType string, externalIDs []string) ([]*Entity, error)
	GetDistinctLayerTypes(ctx context.Context) ([]string, error)
	// CountByLayerType returns the durable per-layer entity count keyed by
	// layer_type. It reflects the same persistent store used for layer discovery
	// (GetDistinctLayerTypes) and entity rendering, so it is the source of truth
	// for the per-layer "total available" badge.
	CountByLayerType(ctx context.Context) (map[string]int64, error)
	Update(ctx context.Context, entity *Entity) error
	Delete(ctx context.Context, id string) error
	// SearchEntities performs a full-text + prefix search across entities.
	// query is the search term (min 2 chars), layerType is an optional filter,
	// limit caps the result count (max 100).
	// Returns matching results, total count, and any error.
	SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*EntitySearchResult, int, error)
	PatchAIMetadata(ctx context.Context, entityID string, metadata map[string]any) error
	// UpdateCoordinates updates the latest observation's position for the given entity.
	UpdateCoordinates(ctx context.Context, entityID string, lat, lon float64) error
}

// ObservationRepository handles observation persistence
type ObservationRepository interface {
	Create(ctx context.Context, observation *Observation) error
	CreateBatch(ctx context.Context, observations []*Observation) error
	CreateBatchUpsert(ctx context.Context, observations []*Observation) error
	GetByEntityID(ctx context.Context, entityID string, limit int, before time.Time) ([]*Observation, error)
	GetLatest(ctx context.Context, entityID string) (*Observation, error)
	// GetLatestForLayerPage returns each entity's latest observation for a
	// layer, with its entity, ordered by external_id ascending, skipping offset
	// rows and returning at most limit.
	// Deterministic and index-served: pages neither overlap nor skip, and the
	// same request always returns the same rows in the same order.
	GetLatestForLayerPage(ctx context.Context, layerType string, limit, offset int) ([]*EntitySnapshot, error)
	GetLatestForEntityIDs(ctx context.Context, entityIDs []string) (map[string]*Observation, error)
	GetLatestContentHashes(ctx context.Context, entityIDs []string) (map[string]string, error)
	// GetLayerSnapshotAt returns the latest observation per entity for a layer
	// at a specific point in time, within the given lookback window.
	// This enables historical replay across all entity types.
	GetLayerSnapshotAt(ctx context.Context, layerType string, asOf time.Time, window time.Duration) ([]*EntitySnapshot, error)

	// GetLatestForLayerByBBox returns the latest observation per entity within
	// a geographic bounding box, limited to observations within the time window [from, to].
	// Handles antimeridian wrapping when west > east.
	// Used by the WebSocket server for Tier 2 backfill and time range queries.
	GetLatestForLayerByBBox(
		ctx context.Context,
		layerType string,
		south, north, west, east float64,
		from, to time.Time,
		limit int,
	) ([]*EntitySnapshot, error)

	// GetLatestByCurrentPositionInBBox finds each entity's truly-latest observation
	// within [from, to], then filters by whether that latest position falls inside
	// the bounding box. This prevents "jumping" artifacts for mobile entities where
	// older observations at different positions would otherwise match a viewport query.
	// Handles antimeridian wrapping when west > east.
	GetLatestByCurrentPositionInBBox(
		ctx context.Context,
		layerType string,
		south, north, west, east float64,
		from, to time.Time,
		limit int,
	) ([]*EntitySnapshot, error)
	PatchAIMetadata(ctx context.Context, observationID string, metadata map[string]any) error
}

// LayerRepository handles layer configuration
type LayerRepository interface {
	List(ctx context.Context) ([]*Layer, error)
	GetByID(ctx context.Context, id string) (*Layer, error)
	Update(ctx context.Context, layer *Layer) error
}

// SceneRepository handles scene persistence
type SceneRepository interface {
	Create(ctx context.Context, scene *Scene) error
	GetByID(ctx context.Context, id string) (*Scene, error)
	List(ctx context.Context) ([]*Scene, error)
	Update(ctx context.Context, scene *Scene) error
	Delete(ctx context.Context, id string) error
}

// LocationRepository handles location persistence.
// TODO: implement when scenes location feature is built.
type LocationRepository interface {
	List(ctx context.Context, locationType string, cityID string) ([]*Location, error)
	GetByID(ctx context.Context, id string) (*Location, error)
	GetNearest(ctx context.Context, position GeoPoint) (*Location, error)
}

// FilterPresetRepository handles filter preset persistence
type FilterPresetRepository interface {
	Create(ctx context.Context, preset *FilterPreset) error
	GetByID(ctx context.Context, id string) (*FilterPreset, error)
	List(ctx context.Context) ([]*FilterPreset, error)
}

// CameraFeedRepository handles camera feed persistence
type CameraFeedRepository interface {
	Create(ctx context.Context, feed *CameraFeed) error
	GetByID(ctx context.Context, id string) (*CameraFeed, error)
	List(ctx context.Context, city string) ([]*CameraFeed, error)
	GetNearest(ctx context.Context, position GeoPoint, city string) (*CameraFeed, error)
}

// CalibrationRepository handles calibration persistence
type CalibrationRepository interface {
	GetByCameraFeedID(ctx context.Context, cameraFeedID string) (*Calibration, error)
	Update(ctx context.Context, cal *Calibration) error
	Delete(ctx context.Context, id string) error
}

// KeyValueCache defines a minimal key-value cache interface.
// Used by AI enrichment workers for dedup caching without coupling to a concrete store.
type KeyValueCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
}

// AIEnrichmentLogRepository handles AI enrichment audit logging.
type AIEnrichmentLogRepository interface {
	Create(ctx context.Context, log *AIEnrichmentLog) error
	UpdateStatus(ctx context.Context, id string, status string, result map[string]any, usage *AIUsage, errMsg string) error
	GetByEntityAndOperation(ctx context.Context, entityID, operationName string) (*AIEnrichmentLog, error)
	// DeleteStrandedLogs marks/removes audit rows left in a non-terminal status
	// older than olderThan. Returns the number affected.
	DeleteStrandedLogs(ctx context.Context, olderThan time.Duration, statuses []string) (int64, error)
}

// AIInsightRepository handles AI insight persistence.
type AIInsightRepository interface {
	Create(ctx context.Context, insight *AIInsight) (string, error)
	CreateRef(ctx context.Context, insightID string, entityID, observationID *string) error
	GetByID(ctx context.Context, id string) (*AIInsight, error)
	List(ctx context.Context, filter InsightFilter) ([]*AIInsight, int, error)
	DeleteExpired(ctx context.Context) (int64, error)
	GetRecentDedupKeys(ctx context.Context, sourceName, operationName string, since time.Time) (map[string]bool, error)
}

// AIQueryLogRepository handles NL search audit logging.
type AIQueryLogRepository interface {
	Create(ctx context.Context, log *AIQueryLog) error
}

// MaintenanceRepository handles database-engine-specific storage maintenance:
// measuring the on-disk size and enforcing a size cap by pruning oldest data and
// reclaiming space. Each engine adapter implements its own mechanism behind this
// port — the SQLite adapter prunes oldest observations and runs incremental_vacuum;
// a future Postgres adapter would DELETE and rely on its own autovacuum. The
// retention driver depends only on this port.
type MaintenanceRepository interface {
	// DatabaseSizeBytes returns the current on-disk size of the main database.
	DatabaseSizeBytes(ctx context.Context) (int64, error)
	// EnforceSizeCap prunes oldest observations (and orphaned insight refs) until the
	// database size is at or below maxBytes*lowWaterRatio, then reclaims freed space.
	// It is a no-op when the size is at or below maxBytes (the high watermark), or
	// when maxBytes <= 0. It is bounded per call and idempotent across calls: if the
	// per-call batch budget is exhausted while still over the low watermark, the next
	// call continues.
	EnforceSizeCap(ctx context.Context, maxBytes int64, lowWaterRatio float64) (RetentionResult, error)
}

// RetentionResult summarizes one EnforceSizeCap call.
type RetentionResult struct {
	BytesBefore         int64 // on-disk size before the call
	BytesAfter          int64 // on-disk size after pruning + reclaim
	DeletedObservations int64
	DeletedOrphanRefs   int64
	BatchesRun          int
	BudgetExhausted     bool // still over the low watermark when the batch budget ran out
}
