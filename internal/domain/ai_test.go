package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestAIEnrichmentLog_FieldsSetCorrectly(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(5 * time.Second)
	obsID := "obs-123"

	tests := []struct {
		name string
		log  domain.AIEnrichmentLog
	}{
		{
			name: "all fields populated",
			log: domain.AIEnrichmentLog{
				ID:               "log-1",
				EntityID:         "entity-1",
				ObservationID:    &obsID,
				SourceName:       "openai_enrichment",
				OperationName:    "classify_entity",
				PromptHash:       strPtr("abc123hash"),
				Status:           domain.AIStatusCompleted,
				Provider:         "openai",
				Model:            "gpt-4o",
				PromptTokens:     150,
				CompletionTokens: 50,
				LatencyMS:        1200,
				ErrorMessage:     "",
				Result:           map[string]any{"classification": "military", "confidence": 0.95},
				CreatedAt:        now,
				CompletedAt:      &completedAt,
			},
		},
		{
			name: "minimal fields",
			log: domain.AIEnrichmentLog{
				ID:            "log-2",
				EntityID:      "entity-2",
				OperationName: "enrich",
				Status:        domain.AIStatusPending,
				CreatedAt:     now,
			},
		},
		{
			name: "failed status with error message",
			log: domain.AIEnrichmentLog{
				ID:            "log-3",
				EntityID:      "entity-3",
				OperationName: "analyze",
				Status:        domain.AIStatusFailed,
				ErrorMessage:  "rate limit exceeded",
				CreatedAt:     now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := tt.log
			assert.NotEmpty(t, log.ID)
			assert.NotEmpty(t, log.EntityID)
			assert.NotEmpty(t, log.OperationName)
			assert.NotEmpty(t, log.Status)
			assert.False(t, log.CreatedAt.IsZero())

			if tt.name == "all fields populated" {
				require.NotNil(t, log.ObservationID)
				assert.Equal(t, "obs-123", *log.ObservationID)
				assert.Equal(t, "openai_enrichment", log.SourceName)
				require.NotNil(t, log.PromptHash)
				assert.Equal(t, "abc123hash", *log.PromptHash)
				assert.Equal(t, "openai", log.Provider)
				assert.Equal(t, "gpt-4o", log.Model)
				assert.Equal(t, 150, log.PromptTokens)
				assert.Equal(t, 50, log.CompletionTokens)
				assert.Equal(t, 1200, log.LatencyMS)
				assert.Empty(t, log.ErrorMessage)
				require.NotNil(t, log.Result)
				assert.Equal(t, "military", log.Result["classification"])
				assert.Equal(t, 0.95, log.Result["confidence"])
				require.NotNil(t, log.CompletedAt)
				assert.True(t, log.CompletedAt.After(log.CreatedAt))
			}

			if tt.name == "minimal fields" {
				assert.Nil(t, log.ObservationID)
				assert.Nil(t, log.CompletedAt)
				assert.Nil(t, log.Result)
				assert.Zero(t, log.PromptTokens)
				assert.Zero(t, log.CompletionTokens)
			}

			if tt.name == "failed status with error message" {
				assert.Equal(t, domain.AIStatusFailed, log.Status)
				assert.Equal(t, "rate limit exceeded", log.ErrorMessage)
			}
		})
	}
}

func TestAIInsight_FieldsSetCorrectly(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	layerType := "flights_commercial"

	tests := []struct {
		name    string
		insight domain.AIInsight
	}{
		{
			name: "all fields populated",
			insight: domain.AIInsight{
				ID:             "insight-1",
				InsightType:    "anomaly_detection",
				SourceName:     "openai_enrichment",
				OperationName:  "detect_anomalies",
				LayerType:      &layerType,
				Result:         map[string]any{"anomalies": []string{"speed_deviation"}},
				EntityIDs:      []string{"entity-1", "entity-2"},
				ObservationIDs: []string{"obs-1", "obs-2"},
				ExpiresAt:      &expiresAt,
				CreatedAt:      now,
			},
		},
		{
			name: "minimal fields",
			insight: domain.AIInsight{
				ID:            "insight-2",
				InsightType:   "summary",
				SourceName:    "local_model",
				OperationName: "summarize",
				CreatedAt:     now,
			},
		},
		{
			name: "no expiration",
			insight: domain.AIInsight{
				ID:            "insight-3",
				InsightType:   "classification",
				SourceName:    "enrichment_pipeline",
				OperationName: "classify",
				EntityIDs:     []string{"entity-1"},
				CreatedAt:     now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insight := tt.insight
			assert.NotEmpty(t, insight.ID)
			assert.NotEmpty(t, insight.InsightType)
			assert.NotEmpty(t, insight.SourceName)
			assert.NotEmpty(t, insight.OperationName)
			assert.False(t, insight.CreatedAt.IsZero())

			if tt.name == "all fields populated" {
				require.NotNil(t, insight.LayerType)
				assert.Equal(t, "flights_commercial", *insight.LayerType)
				require.NotNil(t, insight.Result)
				assert.Len(t, insight.EntityIDs, 2)
				assert.Len(t, insight.ObservationIDs, 2)
				require.NotNil(t, insight.ExpiresAt)
				assert.True(t, insight.ExpiresAt.After(insight.CreatedAt))
			}

			if tt.name == "minimal fields" {
				assert.Nil(t, insight.LayerType)
				assert.Nil(t, insight.Result)
				assert.Nil(t, insight.EntityIDs)
				assert.Nil(t, insight.ObservationIDs)
				assert.Nil(t, insight.ExpiresAt)
			}

			if tt.name == "no expiration" {
				assert.Nil(t, insight.ExpiresAt)
				assert.Len(t, insight.EntityIDs, 1)
			}
		})
	}
}

func TestInsightFilter_Defaults(t *testing.T) {
	tests := []struct {
		name   string
		filter domain.InsightFilter
	}{
		{
			name:   "zero value has empty strings and zero ints",
			filter: domain.InsightFilter{},
		},
		{
			name: "all fields set",
			filter: domain.InsightFilter{
				InsightType:   "anomaly_detection",
				LayerType:     "flights_commercial",
				EntityID:      "entity-1",
				ObservationID: "obs-1",
				Limit:         10,
				Offset:        20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.filter
			if tt.name == "zero value has empty strings and zero ints" {
				assert.Empty(t, f.InsightType)
				assert.Empty(t, f.LayerType)
				assert.Empty(t, f.EntityID)
				assert.Empty(t, f.ObservationID)
				assert.Zero(t, f.Limit)
				assert.Zero(t, f.Offset)
			} else {
				assert.Equal(t, "anomaly_detection", f.InsightType)
				assert.Equal(t, "flights_commercial", f.LayerType)
				assert.Equal(t, "entity-1", f.EntityID)
				assert.Equal(t, "obs-1", f.ObservationID)
				assert.Equal(t, 10, f.Limit)
				assert.Equal(t, 20, f.Offset)
			}
		})
	}
}

func TestAIUsage_FieldsSetCorrectly(t *testing.T) {
	tests := []struct {
		name  string
		usage domain.AIUsage
	}{
		{
			name: "all fields populated",
			usage: domain.AIUsage{
				Provider:         "openai",
				Model:            "gpt-4o",
				PromptTokens:     200,
				CompletionTokens: 100,
				LatencyMS:        850,
			},
		},
		{
			name:  "zero value",
			usage: domain.AIUsage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tt.usage
			if tt.name == "all fields populated" {
				assert.Equal(t, "openai", u.Provider)
				assert.Equal(t, "gpt-4o", u.Model)
				assert.Equal(t, 200, u.PromptTokens)
				assert.Equal(t, 100, u.CompletionTokens)
				assert.Equal(t, 850, u.LatencyMS)
			} else {
				assert.Empty(t, u.Provider)
				assert.Empty(t, u.Model)
				assert.Zero(t, u.PromptTokens)
				assert.Zero(t, u.CompletionTokens)
				assert.Zero(t, u.LatencyMS)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestAttentionRank_AllLevels(t *testing.T) {
	tests := []struct {
		level string
		want  int
	}{
		{level: "info", want: 0},
		{level: "low", want: 1},
		{level: "medium", want: 2},
		{level: "high", want: 3},
		{level: "critical", want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			got := domain.AttentionRank(tt.level)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAttentionRank_Unknown(t *testing.T) {
	unknowns := []string{"", "extreme", "INFO", "CRITICAL", "randomstring", "Med"}

	for _, level := range unknowns {
		t.Run(level, func(t *testing.T) {
			got := domain.AttentionRank(level)
			assert.Equal(t, -1, got, "expected -1 for unknown level %q", level)
		})
	}
}

func TestAttentionLevelsAtOrAbove_AllLevels(t *testing.T) {
	tests := []struct {
		minLevel string
		wantLen  int
		wantAll  []string
	}{
		{
			minLevel: "info",
			wantLen:  5,
			wantAll:  []string{"info", "low", "medium", "high", "critical"},
		},
		{
			minLevel: "low",
			wantLen:  4,
			wantAll:  []string{"low", "medium", "high", "critical"},
		},
		{
			minLevel: "medium",
			wantLen:  3,
			wantAll:  []string{"medium", "high", "critical"},
		},
		{
			minLevel: "high",
			wantLen:  2,
			wantAll:  []string{"high", "critical"},
		},
		{
			minLevel: "critical",
			wantLen:  1,
			wantAll:  []string{"critical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.minLevel, func(t *testing.T) {
			got := domain.AttentionLevelsAtOrAbove(tt.minLevel)
			require.NotNil(t, got)
			assert.Len(t, got, tt.wantLen)
			assert.Equal(t, tt.wantAll, got)
		})
	}
}

func TestAttentionLevelsAtOrAbove_Invalid(t *testing.T) {
	invalids := []string{"", "unknown", "INFO"}

	for _, level := range invalids {
		t.Run(level, func(t *testing.T) {
			got := domain.AttentionLevelsAtOrAbove(level)
			assert.Nil(t, got, "expected nil for invalid level %q", level)
		})
	}
}

func TestAttentionLevelsAtOrAbove_Order(t *testing.T) {
	got := domain.AttentionLevelsAtOrAbove("info")
	require.NotNil(t, got)
	require.Len(t, got, 5)

	expected := []string{"info", "low", "medium", "high", "critical"}
	for i, level := range expected {
		assert.Equal(t, level, got[i], "level at index %d should be %q, got %q", i, level, got[i])
	}

	// Verify the ranks are strictly ascending across the returned slice.
	for i := 1; i < len(got); i++ {
		prevRank := domain.AttentionRank(got[i-1])
		currRank := domain.AttentionRank(got[i])
		assert.Greater(t, currRank, prevRank,
			"rank of %q (%d) should be greater than rank of %q (%d)",
			got[i], currRank, got[i-1], prevRank,
		)
	}
}

func TestValidAttentionLevels_AllPresent(t *testing.T) {
	expected := []string{"info", "low", "medium", "high", "critical"}

	assert.Len(t, domain.ValidAttentionLevels, 5, "ValidAttentionLevels should have exactly 5 entries")

	for _, level := range expected {
		t.Run(level, func(t *testing.T) {
			val, ok := domain.ValidAttentionLevels[level]
			assert.True(t, ok, "key %q should be present in ValidAttentionLevels", level)
			assert.True(t, val, "value for key %q should be true", level)
		})
	}
}

func TestValidAttentionLevels_InvalidKeys(t *testing.T) {
	invalidKeys := []string{"extreme", "", "INFO", "Critical"}

	for _, key := range invalidKeys {
		t.Run(key, func(t *testing.T) {
			val := domain.ValidAttentionLevels[key]
			assert.False(t, val, "key %q should not be present (or should be false) in ValidAttentionLevels", key)
		})
	}
}

func TestInsightFilter_AttentionFields(t *testing.T) {
	tests := []struct {
		name       string
		filter     domain.InsightFilter
		wantAtt    string
		wantMinAtt string
	}{
		{
			name:       "zero value has empty attention fields",
			filter:     domain.InsightFilter{},
			wantAtt:    "",
			wantMinAtt: "",
		},
		{
			name: "attention field set to high",
			filter: domain.InsightFilter{
				Attention: "high",
			},
			wantAtt:    "high",
			wantMinAtt: "",
		},
		{
			name: "min attention field set to medium",
			filter: domain.InsightFilter{
				MinAttention: "medium",
			},
			wantAtt:    "",
			wantMinAtt: "medium",
		},
		{
			name: "both attention fields set",
			filter: domain.InsightFilter{
				Attention:    "critical",
				MinAttention: "low",
			},
			wantAtt:    "critical",
			wantMinAtt: "low",
		},
		{
			name: "all fields set including attention",
			filter: domain.InsightFilter{
				InsightType:   "anomaly_detection",
				LayerType:     "flights_commercial",
				EntityID:      "entity-1",
				ObservationID: "obs-1",
				Attention:     "high",
				MinAttention:  "medium",
				Limit:         10,
				Offset:        5,
			},
			wantAtt:    "high",
			wantMinAtt: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantAtt, tt.filter.Attention)
			assert.Equal(t, tt.wantMinAtt, tt.filter.MinAttention)
		})
	}
}

func TestAIInsight_WithAttention(t *testing.T) {
	now := time.Now()
	highLevel := "high"

	tests := []struct {
		name          string
		insight       domain.AIInsight
		wantAttention *string
	}{
		{
			name: "insight with non-nil attention",
			insight: domain.AIInsight{
				ID:            "insight-att-1",
				InsightType:   "threat_assessment",
				SourceName:    "openai_enrichment",
				OperationName: "assess_threat",
				Attention:     &highLevel,
				CreatedAt:     now,
			},
			wantAttention: &highLevel,
		},
		{
			name: "insight with nil attention",
			insight: domain.AIInsight{
				ID:            "insight-att-2",
				InsightType:   "summary",
				SourceName:    "local_model",
				OperationName: "summarize",
				Attention:     nil,
				CreatedAt:     now,
			},
			wantAttention: nil,
		},
		{
			name: "insight with each valid attention level",
			insight: domain.AIInsight{
				ID:            "insight-att-3",
				InsightType:   "alert",
				SourceName:    "enrichment_pipeline",
				OperationName: "alert",
				Attention:     strPtr("critical"),
				CreatedAt:     now,
			},
			wantAttention: strPtr("critical"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantAttention == nil {
				assert.Nil(t, tt.insight.Attention)
			} else {
				require.NotNil(t, tt.insight.Attention)
				assert.Equal(t, *tt.wantAttention, *tt.insight.Attention)
				// Verify the attention value is a recognised valid level.
				assert.True(t, domain.ValidAttentionLevels[*tt.insight.Attention],
					"attention value %q should be in ValidAttentionLevels", *tt.insight.Attention)
			}
		})
	}
}

func TestAIStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		want     string
	}{
		{name: "pending", constant: domain.AIStatusPending, want: "pending"},
		{name: "processing", constant: domain.AIStatusProcessing, want: "processing"},
		{name: "completed", constant: domain.AIStatusCompleted, want: "completed"},
		{name: "failed", constant: domain.AIStatusFailed, want: "failed"},
		{name: "skipped", constant: domain.AIStatusSkipped, want: "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.constant)
		})
	}
}

func TestValidAIStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{domain.AIStatusPending, true},
		{domain.AIStatusProcessing, true},
		{domain.AIStatusCompleted, true},
		{domain.AIStatusFailed, true},
		{domain.AIStatusSkipped, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := domain.ValidAIStatus(tt.status); got != tt.valid {
				t.Errorf("ValidAIStatus(%q) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestAIStatusConstants_AreDistinct(t *testing.T) {
	statuses := []string{
		domain.AIStatusPending,
		domain.AIStatusProcessing,
		domain.AIStatusCompleted,
		domain.AIStatusFailed,
		domain.AIStatusSkipped,
	}

	seen := make(map[string]bool)
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate status constant: %s", s)
		seen[s] = true
	}
	assert.Len(t, seen, 5)
}
