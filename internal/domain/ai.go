package domain

import "time"

// AIEnrichmentLog tracks an enrichment job for auditing and idempotency.
type AIEnrichmentLog struct {
	ID               string
	EntityID         string
	ObservationID    *string
	SourceName       string
	OperationName    string
	PromptHash       *string
	Status           string // pending, processing, completed, failed, skipped
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int
	ErrorMessage     string
	Result           map[string]any
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

// AIEnrichmentStatus constants.
const (
	AIStatusPending    = "pending"
	AIStatusProcessing = "processing"
	AIStatusCompleted  = "completed"
	AIStatusFailed     = "failed"
	AIStatusSkipped    = "skipped"
)

// ValidAIStatus returns true if the given status is a recognized AI enrichment status.
func ValidAIStatus(status string) bool {
	switch status {
	case AIStatusPending, AIStatusProcessing, AIStatusCompleted, AIStatusFailed, AIStatusSkipped:
		return true
	default:
		return false
	}
}

// AIUsage tracks token usage from an LLM call.
type AIUsage struct {
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int
}

// InsightEntityRef is a lightweight entity reference attached to an AI insight.
type InsightEntityRef struct {
	ID         string
	ExternalID string
	Name       string
	LayerType  string
}

// AIInsight represents a stored AI-generated insight.
type AIInsight struct {
	ID             string
	InsightType    string
	SourceName     string
	OperationName  string
	LayerType      *string
	Attention      *string // base schema field: info, low, medium, high, critical
	AttentionRank  *int    // computed: 0=info,1=low,2=medium,3=high,4=critical
	DedupKey       *string // composite hash of key fields; set by analysis engine
	Result         map[string]any
	EntityIDs      []string
	Entities       []InsightEntityRef
	ObservationIDs []string
	ExpiresAt      *time.Time
	CreatedAt      time.Time
}

// ValidAttentionLevels defines the allowed values for the attention base schema field.
var ValidAttentionLevels = map[string]bool{
	"info": true, "low": true, "medium": true, "high": true, "critical": true,
}

// InsightFilter defines query filters for listing insights.
type InsightFilter struct {
	InsightType   string
	LayerType     string
	EntityID      string
	ObservationID string
	Attention     string // filter by attention level (info, low, medium, high, critical)
	MinAttention  string // filter by minimum attention level (inclusive)
	Limit         int
	Offset        int
}

// AttentionRank returns the numeric rank for an attention level (for >= filtering).
// Returns -1 for unknown levels.
func AttentionRank(level string) int {
	switch level {
	case "info":
		return 0
	case "low":
		return 1
	case "medium":
		return 2
	case "high":
		return 3
	case "critical":
		return 4
	default:
		return -1
	}
}

// AttentionLevelsAtOrAbove returns all attention levels at or above the given minimum.
func AttentionLevelsAtOrAbove(minLevel string) []string {
	rank := AttentionRank(minLevel)
	if rank < 0 {
		return nil
	}
	all := []string{"info", "low", "medium", "high", "critical"}
	return all[rank:]
}

// InsightTypeOption describes one available insight type for the filter UI.
type InsightTypeOption struct {
	Value       string
	DisplayName string
	SourceName  string
}

// AIQueryLog records an NL search request for auditing.
type AIQueryLog struct {
	ID               string
	UserQuery        string
	GeneratedSQL     string
	Explanation      string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int
	ResultCount      int
	ErrorMessage     string
	CreatedAt        time.Time
}
