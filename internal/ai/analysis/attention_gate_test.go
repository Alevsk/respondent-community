package analysis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/llm"
)

func sp(s string) *string { return &s }

// TestEngine_attentionPasses covers the gate logic: per-op floor, engine default
// fallback, no-floor backward compatibility, and unclassified-as-info handling.
func TestEngine_attentionPasses(t *testing.T) {
	op := func(floor string) *aiconfig.OperationConfig {
		return &aiconfig.OperationConfig{Output: aiconfig.OutputConfig{MinAttention: floor}}
	}
	tests := []struct {
		name      string
		opFloor   string
		defFloor  string
		attention *string
		want      bool
	}{
		{"no floor passes a value", "", "", sp("info"), true},
		{"no floor passes nil", "", "", nil, true},
		{"op floor high: critical passes", "high", "", sp("critical"), true},
		{"op floor high: high passes", "high", "", sp("high"), true},
		{"op floor high: medium dropped", "high", "", sp("medium"), false},
		{"op floor high: info dropped", "high", "", sp("info"), false},
		{"op floor high: unclassified dropped", "high", "", nil, false},
		{"default low: info dropped", "", "low", sp("info"), false},
		{"default low: low passes", "", "low", sp("low"), true},
		{"default low: unclassified dropped", "", "low", nil, false},
		{"op override can lower below default", "info", "high", sp("info"), true},
		{"op override can raise above default", "critical", "low", sp("high"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{defaultMinAttention: tt.defFloor}
			assert.Equal(t, tt.want, e.attentionPasses(op(tt.opFloor), tt.attention))
		})
	}
}

// gateSchema is a permissive results-array schema with an attention field.
func gateSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"items"},
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"entity_id", "title", "attention"},
					"properties": map[string]any{
						"entity_id": map[string]any{"type": "string"},
						"title":     map[string]any{"type": "string"},
						"attention": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

// runGateAnalysis runs a results_path analysis whose LLM returns one item per
// attention level, with the given per-op floor and engine default, and returns
// the stored insights.
func runGateAnalysis(t *testing.T, opFloor, defaultFloor string) []*domain.AIInsight {
	t.Helper()
	resp := map[string]any{"items": []any{
		map[string]any{"entity_id": "ent-1", "title": "routine", "attention": "info"},
		map[string]any{"entity_id": "ent-1", "title": "minor", "attention": "low"},
		map[string]any{"entity_id": "ent-1", "title": "notable", "attention": "medium"},
		map[string]any{"entity_id": "ent-1", "title": "serious", "attention": "high"},
		map[string]any{"entity_id": "ent-1", "title": "urgent", "attention": "critical"},
	}}
	respJSON, _ := json.Marshal(resp)
	provider := &mockLLMProvider{response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"}}

	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)
	engine.defaultMinAttention = defaultFloor

	schemaMap := gateSchema()
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey("gate_test", "scan"), schemaMap))

	entityRepo.AddEntity(makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial"))
	now := engine.clock.Now()
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{
		makeTestObservation("obs-1", "ent-1", 40.7, -74.0, 10000, now.Add(-30*time.Minute)),
	})

	def := makeTestDefinition("gate_test")
	def.AI.Operations = []aiconfig.OperationConfig{{
		Name:         "scan",
		Prompt:       "Scan {{.RecordCount}}",
		OutputSchema: schemaMap,
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "test",
			ResultsPath:   "items",
			Retention:     "168h",
			MinAttention:  opFloor,
		},
	}}

	require.NoError(t, engine.RunAnalysis(context.Background(), def))
	return insightRepo.GetInsights()
}

func attentionsOf(insights []*domain.AIInsight) []string {
	out := make([]string, 0, len(insights))
	for _, in := range insights {
		if in.Attention != nil {
			out = append(out, *in.Attention)
		}
	}
	return out
}

// TestEngine_RunAnalysis_MinAttentionGate_PerOp proves a per-operation
// output.min_attention floor drops results below it (store + notify).
func TestEngine_RunAnalysis_MinAttentionGate_PerOp(t *testing.T) {
	insights := runGateAnalysis(t, "high", "")
	assert.Len(t, insights, 2, "only high+critical clear the high floor")
	assert.ElementsMatch(t, []string{"high", "critical"}, attentionsOf(insights))
}

// TestEngine_RunAnalysis_MinAttentionGate_GlobalDefault proves the engine-wide
// default applies when the operation sets no floor.
func TestEngine_RunAnalysis_MinAttentionGate_GlobalDefault(t *testing.T) {
	insights := runGateAnalysis(t, "", "medium")
	assert.Len(t, insights, 3, "medium+high+critical clear the medium default")
	assert.ElementsMatch(t, []string{"medium", "high", "critical"}, attentionsOf(insights))
}

// TestEngine_RunAnalysis_MinAttentionGate_PerOpOverridesDefault proves the
// per-op floor wins over the engine default (here, lowering it to keep all).
func TestEngine_RunAnalysis_MinAttentionGate_PerOpOverridesDefault(t *testing.T) {
	insights := runGateAnalysis(t, "info", "critical")
	assert.Len(t, insights, 5, "op floor=info overrides default=critical, keeping all")
}

// TestEngine_RunAnalysis_NoGate_StoresAll proves backward compatibility: with no
// floor configured anywhere, every result is stored.
func TestEngine_RunAnalysis_NoGate_StoresAll(t *testing.T) {
	insights := runGateAnalysis(t, "", "")
	assert.Len(t, insights, 5, "no floor keeps every result (backward compatible)")
}

// runEmptyArrayAnalysis runs a results_path analysis whose LLM returns an empty
// array, with the given floor, and returns the stored insight count.
func runEmptyArrayAnalysis(t *testing.T, opFloor string) int {
	t.Helper()
	respJSON, _ := json.Marshal(map[string]any{"items": []any{}})
	provider := &mockLLMProvider{response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"}}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)
	schemaMap := gateSchema()
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey("empty_test", "scan"), schemaMap))
	entityRepo.AddEntity(makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial"))
	now := engine.clock.Now()
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{
		makeTestObservation("obs-1", "ent-1", 40.7, -74.0, 10000, now.Add(-30*time.Minute)),
	})
	def := makeTestDefinition("empty_test")
	def.AI.Operations = []aiconfig.OperationConfig{{
		Name:         "scan",
		Prompt:       "Scan {{.RecordCount}}",
		OutputSchema: schemaMap,
		Output: aiconfig.OutputConfig{
			StoreInsights: true, InsightType: "test", ResultsPath: "items",
			Retention: "168h", MinAttention: opFloor,
		},
	}}
	require.NoError(t, engine.RunAnalysis(context.Background(), def))
	return insightRepo.InsightCount()
}

// TestEngine_RunAnalysis_EmptySummary_GatedWithFloor proves an "all clear" empty
// result is NOT persisted when a floor is set (no per-tick clutter), but IS
// stored when no floor is configured (legacy run-visibility behavior).
func TestEngine_RunAnalysis_EmptySummary_GatedWithFloor(t *testing.T) {
	assert.Equal(t, 0, runEmptyArrayAnalysis(t, "medium"), "all-clear summary gated under a floor")
	assert.Equal(t, 1, runEmptyArrayAnalysis(t, ""), "all-clear summary stored when no floor (legacy)")
}
