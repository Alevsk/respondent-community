package domain

import (
	"context"
	"time"
)

// EntityDetail combines an entity with its latest observation.
type EntityDetail struct {
	Entity            *Entity
	LatestObservation *Observation
}

// SnapshotResult contains the result of a layer snapshot query.
type SnapshotResult struct {
	Entities     []*Entity
	Observations []*Observation
	TotalCount   int64
	HasMore      bool
}

// NOTE: IndicatorValue and IndicatorSnapshot are NOT defined here —
// they already exist in domain/indicator.go.

// NLSearchResult represents a single result from a natural language search.
type NLSearchResult struct {
	EntityID   string            `json:"entity_id"`
	ExternalID string            `json:"external_id"`
	LayerType  string            `json:"layer_type"`
	Name       string            `json:"name"`
	Metadata   map[string]string `json:"metadata"`
}

// NLSearchResponse contains the results of a natural language search.
type NLSearchResponse struct {
	GeneratedSQL string            `json:"generated_sql"`
	Results      []*NLSearchResult `json:"results"`
	TotalCount   int               `json:"total_count"`
	Explanation  string            `json:"explanation"`
}

// AnalyzeEntityResult contains the result of an entity analysis.
type AnalyzeEntityResult struct {
	EntityID  string `json:"entity_id"`
	Analysis  string `json:"analysis"`
	InsightID string `json:"insight_id,omitempty"`
}

// AnalysisDefInfo is a summary of an analysis definition for API responses.
type AnalysisDefInfo struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Enabled      bool     `json:"enabled"`
	Schedule     string   `json:"schedule"`
	Layers       []string `json:"layers"`
	InsightTypes []string `json:"insight_types"`
}

// NotificationFilterOptions aggregates available filter dimensions.
type NotificationFilterOptions struct {
	InsightTypes    []InsightTypeOption
	AttentionLevels []string
	LayerTypes      []string
}

// NOTE: AnalysisDefinitionLoader and QueryExecutor interfaces stay in
// internal/app/ai/service.go (per errata E2) to avoid domain → ai dependency.

// EntityServicer abstracts the entity application service.
type EntityServicer interface {
	GetEntityDetail(ctx context.Context, entityID string) (*EntityDetail, error)
	GetObservationHistory(ctx context.Context, entityID string, limit int, before time.Time) ([]*Observation, bool, error)
	SearchEntities(ctx context.Context, query, layerType string, limit int) ([]*EntitySearchResult, int, error)
	GetEntitiesBatch(ctx context.Context, ids []string) ([]*EntityDetail, error)
}

// LayerServicer abstracts the layer application service.
type LayerServicer interface {
	GetLayers(ctx context.Context) ([]*Layer, error)
	ToggleLayer(ctx context.Context, toggle *LayerToggle) (*Layer, error)
	GetLayerSnapshot(ctx context.Context, layerID string, limit, offset int) (*SnapshotResult, error)
}

// IndicatorServicer abstracts the indicator application service.
type IndicatorServicer interface {
	GetGlobalIndicators(ctx context.Context, layerIDs []string) ([]IndicatorSnapshot, error)
}

// MediaServicer abstracts the media application service.
type MediaServicer interface {
	ReportPlayback(ctx context.Context, entityID, mediaID string) (bool, error)
}

// AIServicer abstracts the AI application service.
type AIServicer interface {
	NaturalLanguageSearch(ctx context.Context, query string, layerType string, limit int) (*NLSearchResponse, error)
	AnalyzeEntity(ctx context.Context, entityID string, obsLimit int) (*AnalyzeEntityResult, error)
	GetInsights(ctx context.Context, filter InsightFilter) ([]*AIInsight, int, error)
	ExplainQuery(ctx context.Context, sql string) (string, error)
	ListAnalysisDefinitions(ctx context.Context) []*AnalysisDefInfo
	GetNotificationFilterOptions(ctx context.Context) *NotificationFilterOptions
}
