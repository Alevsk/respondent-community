package analysis

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/llm"
)

// sequencedProvider returns a different response per call, so a test can simulate
// a first-attempt schema failure that a later attempt corrects. After the queue
// is exhausted it keeps returning the last entry.
type sequencedProvider struct {
	mu        sync.Mutex
	responses []string
	calls     int
}

func (p *sequencedProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.calls
	if i >= len(p.responses) {
		i = len(p.responses) - 1
	}
	p.calls++
	return &llm.CompletionResponse{Content: p.responses[i], Model: "test"}, nil
}

func (p *sequencedProvider) Name() string                        { return "mock" }
func (p *sequencedProvider) SupportsProvider(pt string) bool     { return pt == "mock" }
func (p *sequencedProvider) HealthCheck(_ context.Context) error { return nil }

func (p *sequencedProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func newTestEngineWith(t *testing.T, provider llm.Provider, logger zerolog.Logger) (*Engine, *mockEntityRepository, *mockObservationRepository, *mockInsightRepository) {
	t.Helper()
	entityRepo := newMockEntityRepo()
	obsRepo := newMockObsRepo()
	insightRepo := newMockInsightRepo()
	engine, err := NewEngine(EngineConfig{
		LLMRegistry: &mockLLMRegistry{provider: provider},
		EntityRepo:  entityRepo,
		ObsRepo:     obsRepo,
		InsightRepo: insightRepo,
		Schemas:     schema.NewRegistry(),
		Logger:      logger,
		Clock:       &mockClock{now: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)},
	})
	require.NoError(t, err)
	return engine, entityRepo, obsRepo, insightRepo
}

// schemaMap requiring a single top-level "summary" string.
func summarySchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []any{"summary"},
		"properties": map[string]any{"summary": map[string]any{"type": "string"}},
	}
}

func summaryDef(t *testing.T, engine *Engine, name string) *AnalysisDefinition {
	t.Helper()
	sm := summarySchema()
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey(name, "op"), sm))
	def := makeTestDefinition(name)
	def.AI.Operations = []aiconfig.OperationConfig{
		{
			Name:         "op",
			Prompt:       "Analyze {{.RecordCount}} records",
			OutputSchema: sm,
			Output:       aiconfig.OutputConfig{StoreInsights: true, InsightType: "test", Retention: "168h"},
		},
	}
	return def
}

func seedOneRecord(engine *Engine, entityRepo *mockEntityRepository, obsRepo *mockObservationRepository) {
	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, engine.clock.Now().Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})
}

func TestEngine_RunAnalysis_SchemaRetrySelfHeals(t *testing.T) {
	// First response is missing the required "summary" (the real-world failure:
	// the model returned a malformed/partial object); the retry corrects it.
	provider := &sequencedProvider{responses: []string{
		`{"not_summary": "oops"}`,
		`{"summary": "all clear"}`,
	}}
	engine, entityRepo, obsRepo, insightRepo := newTestEngineWith(t, provider, zerolog.Nop())
	seedOneRecord(engine, entityRepo, obsRepo)

	def := summaryDef(t, engine, "retry_heals")
	err := engine.RunAnalysis(context.Background(), def)

	require.NoError(t, err)
	assert.Equal(t, 2, provider.callCount(), "should retry once after the first invalid response")
	assert.Equal(t, 1, insightRepo.InsightCount(), "the corrected response should produce an insight")
}

func TestEngine_RunAnalysis_SchemaRetryGivesUpAfterBound(t *testing.T) {
	// Every response is invalid. The operation gives up after the bounded attempts
	// (rather than looping forever) and the run continues — RunAnalysis logs the
	// failure rather than aborting, so we assert on the observable effects + log.
	provider := &sequencedProvider{responses: []string{`{"bad": 1}`}}
	var logBuf bytes.Buffer
	engine, entityRepo, obsRepo, insightRepo := newTestEngineWith(t, provider, zerolog.New(&logBuf))
	seedOneRecord(engine, entityRepo, obsRepo)

	def := summaryDef(t, engine, "retry_gives_up")
	err := engine.RunAnalysis(context.Background(), def)

	require.NoError(t, err) // per-operation failures are logged, not returned
	assert.Equal(t, 3, provider.callCount(), "should make exactly maxSchemaAttempts calls")
	assert.Equal(t, 0, insightRepo.InsightCount(), "no insight stored for an unrecoverable response")
	assert.Contains(t, logBuf.String(), "after 3 attempts", "the bounded give-up reason should be logged")
}
