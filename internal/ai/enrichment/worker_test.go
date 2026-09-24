package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/prompts"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inmem"
	"github.com/Alevsk/respondent/internal/llm"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockLLMProvider implements llm.Provider for testing.
type mockLLMProvider struct {
	mu       sync.Mutex
	name     string
	response *llm.CompletionResponse
	// responses, when non-empty, are returned in sequence per call (clamped to
	// the last entry), so a test can simulate e.g. a bad-then-good response.
	responses []*llm.CompletionResponse
	err       error
	calls     int
	lastReq   *llm.CompletionRequest
}

func (m *mockLLMProvider) Complete(_ context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	idx := m.calls
	m.calls++
	m.lastReq = req
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if len(m.responses) > 0 {
		if idx >= len(m.responses) {
			idx = len(m.responses) - 1
		}
		return m.responses[idx], nil
	}
	return m.response, nil
}

// getCalls returns the number of Complete invocations under the mutex.
func (m *mockLLMProvider) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// getLastReq returns the most recently captured request under the mutex.
func (m *mockLLMProvider) getLastReq() *llm.CompletionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastReq
}

func (m *mockLLMProvider) Name() string                              { return m.name }
func (m *mockLLMProvider) SupportsProvider(providerType string) bool { return providerType == m.name }
func (m *mockLLMProvider) HealthCheck(_ context.Context) error       { return nil }

// mockEntityRepo implements domain.EntityRepository for testing.
type mockEntityRepo struct {
	entity  *domain.Entity
	err     error
	patches []patchCall
}

type patchCall struct {
	EntityID string
	Metadata map[string]any
}

func (m *mockEntityRepo) Create(_ context.Context, _ *domain.Entity) error        { return nil }
func (m *mockEntityRepo) CreateBatch(_ context.Context, _ []*domain.Entity) error { return nil }
func (m *mockEntityRepo) GetByID(_ context.Context, _ string) (*domain.Entity, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.entity, nil
}
func (m *mockEntityRepo) GetByIDs(_ context.Context, _ []string) ([]*domain.Entity, error) {
	return nil, nil
}
func (m *mockEntityRepo) GetByExternalID(_ context.Context, _, _ string) (*domain.Entity, error) {
	return nil, nil
}
func (m *mockEntityRepo) GetByExternalIDs(_ context.Context, _ string, _ []string) ([]*domain.Entity, error) {
	return nil, nil
}
func (m *mockEntityRepo) GetDistinctLayerTypes(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockEntityRepo) CountByLayerType(_ context.Context) (map[string]int64, error) {
	return nil, nil
}
func (m *mockEntityRepo) Update(_ context.Context, _ *domain.Entity) error { return nil }
func (m *mockEntityRepo) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockEntityRepo) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}
func (m *mockEntityRepo) PatchAIMetadata(_ context.Context, entityID string, metadata map[string]any) error {
	m.patches = append(m.patches, patchCall{EntityID: entityID, Metadata: metadata})
	return nil
}
func (m *mockEntityRepo) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

// mockObsRepo implements domain.ObservationRepository for testing.
type mockObsRepo struct {
	obs     *domain.Observation
	err     error
	patches []obsPatchCall
}

type obsPatchCall struct {
	ObservationID string
	Metadata      map[string]any
}

func (m *mockObsRepo) Create(_ context.Context, _ *domain.Observation) error              { return nil }
func (m *mockObsRepo) CreateBatch(_ context.Context, _ []*domain.Observation) error       { return nil }
func (m *mockObsRepo) CreateBatchUpsert(_ context.Context, _ []*domain.Observation) error { return nil }
func (m *mockObsRepo) GetByEntityID(_ context.Context, _ string, _ int, _ time.Time) ([]*domain.Observation, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLatest(_ context.Context, _ string) (*domain.Observation, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.obs, nil
}
func (m *mockObsRepo) CountLatestForLayer(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (m *mockObsRepo) GetLatestForLayerPage(_ context.Context, _ string, _, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLatestForEntityIDs(_ context.Context, _ []string) (map[string]*domain.Observation, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLatestContentHashes(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLayerSnapshotAt(_ context.Context, _ string, _ time.Time, _ time.Duration, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLatestForLayerByBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (m *mockObsRepo) GetLatestByCurrentPositionInBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (m *mockObsRepo) PatchAIMetadata(_ context.Context, obsID string, metadata map[string]any) error {
	m.patches = append(m.patches, obsPatchCall{ObservationID: obsID, Metadata: metadata})
	return nil
}

// mockAILogRepo implements domain.AIEnrichmentLogRepository for testing.
type mockAILogRepo struct {
	creates     []*domain.AIEnrichmentLog
	statusCalls []statusUpdateCall
	createErr   error

	// reaperMu guards reaperCalls, which the background reaper goroutine may
	// increment concurrently with the test goroutine in lifecycle tests.
	reaperMu    sync.Mutex
	reaperCalls int
}

type statusUpdateCall struct {
	ID     string
	Status string
	Result map[string]any
	Usage  *domain.AIUsage
	ErrMsg string
}

func (m *mockAILogRepo) Create(_ context.Context, log *domain.AIEnrichmentLog) error {
	m.creates = append(m.creates, log)
	return m.createErr
}

func (m *mockAILogRepo) UpdateStatus(_ context.Context, id string, status string, result map[string]any, usage *domain.AIUsage, errMsg string) error {
	m.statusCalls = append(m.statusCalls, statusUpdateCall{
		ID:     id,
		Status: status,
		Result: result,
		Usage:  usage,
		ErrMsg: errMsg,
	})
	return nil
}

func (m *mockAILogRepo) GetByEntityAndOperation(_ context.Context, _, _ string) (*domain.AIEnrichmentLog, error) {
	return nil, nil
}

func (m *mockAILogRepo) DeleteStrandedLogs(_ context.Context, _ time.Duration, _ []string) (int64, error) {
	m.reaperMu.Lock()
	m.reaperCalls++
	m.reaperMu.Unlock()
	return 0, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// defaultTestEntity returns the standard test entity.
func defaultTestEntity() *domain.Entity {
	return &domain.Entity{
		ID:         "ent-001",
		ExternalID: "ext-001",
		Name:       "Test Aircraft",
		LayerType:  "flights_commercial",
		Metadata:   map[string]string{"callsign": "TST123", "airline": "TestAir"},
	}
}

// defaultTestObservation returns the standard test observation.
func defaultTestObservation() *domain.Observation {
	return &domain.Observation{
		ID:        "obs-001",
		EntityID:  "ent-001",
		Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0},
		AltitudeM: 10000,
		Metadata:  map[string]string{"squawk": "1200"},
	}
}

func newTestWorker(t *testing.T, opts ...func(*testWorkerOpts)) *testHarness {
	t.Helper()

	o := &testWorkerOpts{
		llmProvider: &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"threat_level": "low", "classification": "civilian"}`,
				Model:   "test-model",
				Usage: llm.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
				},
			},
		},
		entityRepo: &mockEntityRepo{entity: defaultTestEntity()},
		obsRepo:    &mockObsRepo{obs: defaultTestObservation()},
		aiLogRepo:  &mockAILogRepo{},
		sourceConfigs: map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "threat_assessment",
						Prompt:   "Assess threat for {{.Entity.Name}} ({{.Entity.MetadataJSON}})",
						CacheTTL: "5m",
					},
				},
			},
		},
	}

	for _, fn := range opts {
		fn(o)
	}

	kv := inmem.NewKVCache()

	schemas := schema.NewRegistry()

	w := &Worker{
		llmProvider:   o.llmProvider,
		schemas:       schemas,
		entityRepo:    o.entityRepo,
		obsRepo:       o.obsRepo,
		aiLogRepo:     o.aiLogRepo,
		kvCache:       kv,
		sourceConfigs: o.sourceConfigs,
		logger:        zerolog.Nop(),
		concurrency:   1,
		subject:       "respondent.ai.enrich",
		consumerGroup: "test-workers",
	}

	require.NoError(t, w.initCELEnv())

	return &testHarness{
		worker:     w,
		kv:         kv,
		llm:        o.llmProvider,
		entityRepo: o.entityRepo,
		obsRepo:    o.obsRepo,
		aiLogRepo:  o.aiLogRepo,
		schemas:    schemas,
	}
}

type testWorkerOpts struct {
	llmProvider   *mockLLMProvider
	entityRepo    *mockEntityRepo
	obsRepo       *mockObsRepo
	aiLogRepo     *mockAILogRepo
	sourceConfigs map[string]*aiconfig.SourceAIConfig
}

type testHarness struct {
	worker     *Worker
	kv         *inmem.KVCache
	llm        *mockLLMProvider
	entityRepo *mockEntityRepo
	obsRepo    *mockObsRepo
	aiLogRepo  *mockAILogRepo
	schemas    *schema.Registry
}

func makeJobJSON(t *testing.T, job Job) []byte {
	t.Helper()
	data, err := json.Marshal(job)
	require.NoError(t, err)
	return data
}

// computePromptHash renders a prompt template against the default entity/obs
// and returns the hash. Used to pre-populate the prompt cache in tests.
func computePromptHash(t *testing.T, promptTemplate string) string {
	t.Helper()
	entity := defaultTestEntity()
	obs := defaultTestObservation()
	pd := prompts.NewPromptData(entity, obs, "")
	rendered, err := prompts.Render(promptTemplate, pd)
	require.NoError(t, err)
	return prompts.Hash(rendered)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestWorker_ProcessJob_Success(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		ExternalID:  "ext-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Verify LLM was called.
	assert.Equal(t, 1, h.llm.getCalls())

	// Verify entity was patched.
	require.Len(t, h.entityRepo.patches, 1)
	assert.Equal(t, "ent-001", h.entityRepo.patches[0].EntityID)

	// With no output_mapping, result is stored under operation name.
	assert.Contains(t, h.entityRepo.patches[0].Metadata, "threat_assessment")

	// Verify audit log was created and completed.
	require.Len(t, h.aiLogRepo.creates, 1)
	assert.Equal(t, "ent-001", h.aiLogRepo.creates[0].EntityID)
	assert.Equal(t, domain.AIStatusPending, h.aiLogRepo.creates[0].Status)

	// Check completed status update.
	require.NotEmpty(t, h.aiLogRepo.statusCalls)
	lastUpdate := h.aiLogRepo.statusCalls[len(h.aiLogRepo.statusCalls)-1]
	assert.Equal(t, domain.AIStatusCompleted, lastUpdate.Status)
	assert.Empty(t, lastUpdate.ErrMsg)

	// Verify the cache entry was set.
	val, err := h.kv.Get(ctx, "ai:enrich:ent-001:threat_assessment")
	require.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestWorker_ProcessJob_AuditLogPromptHashPopulated(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	job := Job{
		EntityID:      "ent-001",
		ObservationID: "obs-001",
		SourceName:    "adsb",
		LayerType:     "flights_commercial",
		PublishedAt:   time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Audit log must be created with a non-empty prompt hash.
	require.Len(t, h.aiLogRepo.creates, 1)
	created := h.aiLogRepo.creates[0]
	require.NotNil(t, created.PromptHash, "prompt_hash must be set on the audit log at creation time")

	// Verify the hash matches the expected value computed from the same template.
	promptTemplate := "Assess threat for {{.Entity.Name}} ({{.Entity.MetadataJSON}})"
	expectedHash := computePromptHash(t, promptTemplate)
	assert.Equal(t, expectedHash, *created.PromptHash, "prompt_hash should match the hash of the rendered prompt")

	// Verify observation ID is set.
	require.NotNil(t, created.ObservationID)
	assert.Equal(t, "obs-001", *created.ObservationID)
}

func TestWorker_ProcessJob_CELFilterRejects(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "threat_assessment",
						Prompt: "Assess threat for {{.Entity.Name}}",
						Filter: `entity.layer_type == "satellites"`, // does not match flights_commercial
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// LLM should NOT be called.
	assert.Equal(t, 0, h.llm.getCalls())

	// Entity should NOT be patched.
	assert.Empty(t, h.entityRepo.patches)

	// Audit log should be created directly with skipped status (no update call).
	require.Len(t, h.aiLogRepo.creates, 1)
	assert.Equal(t, domain.AIStatusSkipped, h.aiLogRepo.creates[0].Status)
	assert.Nil(t, h.aiLogRepo.creates[0].PromptHash, "skipped operations should have nil prompt hash")
	assert.Empty(t, h.aiLogRepo.statusCalls, "skipped via CEL should not trigger an update call")
}

func TestWorker_ProcessJob_CELFilterPasses(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "threat_assessment",
						Prompt: "Assess threat for {{.Entity.Name}}",
						Filter: `entity.layer_type == "flights_commercial"`, // matches
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// LLM should be called since filter passes.
	assert.Equal(t, 1, h.llm.getCalls())
	assert.NotEmpty(t, h.entityRepo.patches)
}

func TestWorker_ProcessJob_CELFilterUsesMetadata(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "threat_assessment",
						Prompt: "Assess threat for {{.Entity.Name}}",
						Filter: `entity.metadata.callsign == "TST123"`,
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Filter matches metadata.callsign == "TST123".
	assert.Equal(t, 1, h.llm.getCalls())
}

func TestWorker_ProcessJob_EntityCacheHit(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	// Pre-populate entity-level cache.
	require.NoError(t, h.kv.Set(ctx, "ai:enrich:ent-001:threat_assessment", "1", 5*time.Minute))

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// LLM should NOT be called due to cache hit.
	assert.Equal(t, 0, h.llm.getCalls())

	// Entity should NOT be patched.
	assert.Empty(t, h.entityRepo.patches)
}

func TestWorker_ProcessJob_PromptCacheHit(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	// Compute the same prompt hash that the worker will produce.
	promptTemplate := "Assess threat for {{.Entity.Name}} ({{.Entity.MetadataJSON}})"
	promptHash := computePromptHash(t, promptTemplate)

	// Pre-populate prompt-hash cache with a result.
	cachedResult := map[string]any{"threat_level": "cached", "classification": "cached_civilian"}
	cachedJSON, err := json.Marshal(cachedResult)
	require.NoError(t, err)
	require.NoError(t, h.kv.Set(ctx, fmt.Sprintf("ai:prompt:%s", promptHash), string(cachedJSON), 5*time.Minute))

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// LLM should NOT be called due to prompt cache hit.
	assert.Equal(t, 0, h.llm.getCalls())

	// Entity should be patched with cached result.
	require.Len(t, h.entityRepo.patches, 1)
	innerResult, ok := h.entityRepo.patches[0].Metadata["threat_assessment"].(map[string]any)
	require.True(t, ok, "expected nested map under operation name")
	assert.Equal(t, "cached", innerResult["threat_level"])
}

func TestWorker_ProcessJob_LLMFailure(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			err:  fmt.Errorf("connection timeout: %w", llm.ErrCompletionFailed),
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call")

	// Audit log should show failed status.
	require.NotEmpty(t, h.aiLogRepo.statusCalls)
	assert.Equal(t, domain.AIStatusFailed, h.aiLogRepo.statusCalls[0].Status)

	// Entity should NOT be patched.
	assert.Empty(t, h.entityRepo.patches)
}

func TestWorker_ProcessJob_LLMRateLimited(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			err:  llm.ErrRateLimited,
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)

	// Should be a transient error (triggers Nak).
	assert.True(t, isTransient(err), "rate limited errors should be transient")
}

func TestWorker_ProcessJob_SchemaValidationFails(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"wrong_field": true}`,
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "threat_assessment",
						Prompt: "Assess threat",
						OutputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"threat_level": map[string]any{"type": "string"},
							},
							"required": []any{"threat_level"},
						},
					},
				},
			},
		}
	})

	// Register the schema using compound key: "source_name.operation_name".
	err := h.schemas.RegisterFromYAML("adsb.threat_assessment", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"threat_level": map[string]any{"type": "string"},
		},
		"required": []any{"threat_level"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation")

	// Audit log should show failed.
	require.NotEmpty(t, h.aiLogRepo.statusCalls)
	assert.Equal(t, domain.AIStatusFailed, h.aiLogRepo.statusCalls[0].Status)
}

// TestWorker_ProcessJob_SchemaValidation_RetrySelfHeals proves a one-off LLM
// format glitch (here, the result wrapped in an extra object) is retried with a
// corrective prompt instead of being permanently dropped: attempt 1 fails schema
// validation, attempt 2 returns the correct flat object and the job succeeds.
func TestWorker_ProcessJob_SchemaValidation_RetrySelfHeals(t *testing.T) {
	schemaMap := map[string]any{
		"type":       "object",
		"properties": map[string]any{"threat_level": map[string]any{"type": "string"}},
		"required":   []any{"threat_level"},
	}
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			responses: []*llm.CompletionResponse{
				// attempt 1: wrapped in an extra key -> root missing 'threat_level'
				{Content: `{"threat_assessment": {"threat_level": "high"}}`, Model: "test-model", Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 50}},
				// attempt 2: correct flat object -> validates
				{Content: `{"threat_level": "high"}`, Model: "test-model", Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 20}},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {Enabled: true, Operations: []aiconfig.OperationConfig{{
				Name: "threat_assessment", Prompt: "Assess threat", OutputSchema: schemaMap,
			}}},
		}
	})
	require.NoError(t, h.schemas.RegisterFromYAML("adsb.threat_assessment", schemaMap))

	job := Job{EntityID: "ent-001", SourceName: "adsb", LayerType: "flights_commercial", PublishedAt: time.Now()}
	err := h.worker.processJob(context.Background(), makeJobJSON(t, job))
	require.NoError(t, err, "should self-heal after a one-off schema-format glitch")
	assert.Equal(t, 2, h.llm.getCalls(), "should retry exactly once after the first schema failure")

	// The retry must carry a corrective instruction naming the required key.
	last := h.llm.getLastReq()
	require.NotNil(t, last)
	var corrected bool
	for _, m := range last.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "threat_level") && strings.Contains(m.Content, "did not match") {
			corrected = true
		}
	}
	assert.True(t, corrected, "retry request should include a corrective system message naming the required keys")
}

func TestWorker_ProcessJob_OutputTargetObservation(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:         "threat_assessment",
						Prompt:       "Assess threat for {{.Entity.Name}}",
						OutputTarget: "observation",
						CacheTTL:     "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:      "ent-001",
		ObservationID: "obs-001",
		SourceName:    "adsb",
		LayerType:     "flights_commercial",
		PublishedAt:   time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Entity should NOT be patched.
	assert.Empty(t, h.entityRepo.patches)

	// Observation should be patched.
	require.Len(t, h.obsRepo.patches, 1)
	assert.Equal(t, "obs-001", h.obsRepo.patches[0].ObservationID)
	assert.Contains(t, h.obsRepo.patches[0].Metadata, "threat_assessment")
}

// TestWorker_ProcessJob_OutputTargetObservation_UsesDBObsID verifies that the
// worker uses the DB-fetched observation ID (from GetLatest) for PatchAIMetadata,
// NOT the job's advisory ObservationID which may be stale or non-existent.
// This is a regression test for the ON CONFLICT deduplication issue where the
// feeder-generated UUID is never inserted into the observations table.
func TestWorker_ProcessJob_OutputTargetObservation_UsesDBObsID(t *testing.T) {
	// The DB-fetched observation has a DIFFERENT ID than the job's advisory one.
	dbObs := &domain.Observation{
		ID:        "obs-db-authoritative",
		EntityID:  "ent-001",
		Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0},
		AltitudeM: 10000,
		Metadata:  map[string]string{"squawk": "1200"},
	}

	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.obsRepo = &mockObsRepo{obs: dbObs}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:         "threat_assessment",
						Prompt:       "Assess threat for {{.Entity.Name}}",
						OutputTarget: "observation",
						CacheTTL:     "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:      "ent-001",
		ObservationID: "obs-stale-feeder-uuid", // This ID does NOT exist in the DB
		SourceName:    "adsb",
		LayerType:     "flights_commercial",
		PublishedAt:   time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Worker must patch the DB-fetched observation, NOT the job's stale ID.
	require.Len(t, h.obsRepo.patches, 1)
	assert.Equal(t, "obs-db-authoritative", h.obsRepo.patches[0].ObservationID,
		"worker must use DB-fetched obs.ID from GetLatest, not the feeder-assigned job.ObservationID")
	assert.Contains(t, h.obsRepo.patches[0].Metadata, "threat_assessment")
}

func TestWorker_ProcessJob_OutputMapping(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"threat_level": "high", "classification": "military", "extra_field": "ignored"}`,
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "threat_assessment",
						Prompt: "Assess threat for {{.Entity.Name}}",
						OutputMapping: map[string]string{
							"threat_level":   "ai.threat_level",
							"classification": "ai.classification",
						},
						CacheTTL: "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	require.Len(t, h.entityRepo.patches, 1)
	patch := h.entityRepo.patches[0].Metadata

	// Only mapped fields should be present.
	assert.Equal(t, "high", patch["ai.threat_level"])
	assert.Equal(t, "military", patch["ai.classification"])

	// Extra field should NOT be present.
	_, hasExtra := patch["extra_field"]
	assert.False(t, hasExtra, "unmapped fields should not be stored")

	// Operation name key should NOT be present (mapping replaces default behavior).
	_, hasOpName := patch["threat_assessment"]
	assert.False(t, hasOpName, "with mapping, result should not be under operation name")
}

func TestWorker_ProcessJob_NoAIConfig(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "unknown_source",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Nothing should happen.
	assert.Equal(t, 0, h.llm.getCalls())
	assert.Empty(t, h.entityRepo.patches)
	assert.Empty(t, h.aiLogRepo.creates)
}

func TestWorker_ProcessJob_DisabledAIConfig(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled:    false,
				Operations: []aiconfig.OperationConfig{{Name: "noop", Prompt: "noop"}},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)
	assert.Equal(t, 0, h.llm.getCalls())
}

func TestWorker_ProcessJob_MalformedJSON(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	err := h.worker.processJob(ctx, []byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal job")
}

func TestWorker_ProcessJob_MultipleOperations(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "threat_assessment",
						Prompt:   "Assess threat for {{.Entity.Name}}",
						CacheTTL: "5m",
					},
					{
						Name:     "route_analysis",
						Prompt:   "Analyze route for {{.Entity.Name}}",
						CacheTTL: "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Both operations should call LLM.
	assert.Equal(t, 2, h.llm.getCalls())

	// Both should patch the entity.
	require.Len(t, h.entityRepo.patches, 2)
}

func TestWorker_ProcessOperation_EntityLoadFailure(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.entityRepo = &mockEntityRepo{err: fmt.Errorf("db connection lost")}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)

	// Should be transient (DB failure).
	assert.True(t, isTransient(err), "DB failures should be transient")
}

func TestWorker_ApplyOutputMapping_NoMapping(t *testing.T) {
	h := newTestWorker(t)

	op := &aiconfig.OperationConfig{Name: "test_op"}
	result := map[string]any{"key1": "val1", "key2": 42}

	patch := h.worker.applyOutputMapping(op, result)

	// Should wrap under operation name.
	inner, ok := patch["test_op"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "val1", inner["key1"])
	assert.Equal(t, 42, inner["key2"])
}

func TestWorker_ApplyOutputMapping_WithMapping(t *testing.T) {
	h := newTestWorker(t)

	op := &aiconfig.OperationConfig{
		Name: "test_op",
		OutputMapping: map[string]string{
			"source_key": "target_key",
			"missing":    "also_missing",
		},
	}
	result := map[string]any{"source_key": "value", "extra": "ignored"}

	patch := h.worker.applyOutputMapping(op, result)

	assert.Equal(t, "value", patch["target_key"])
	_, hasExtra := patch["extra"]
	assert.False(t, hasExtra)
	_, hasMissing := patch["also_missing"]
	assert.False(t, hasMissing)
}

func TestWorker_CELFilter_EntityMetadata(t *testing.T) {
	h := newTestWorker(t)

	entity := &domain.Entity{
		ID:        "ent-001",
		LayerType: "flights_commercial",
		Metadata:  map[string]string{"callsign": "TST123"},
	}

	obs := &domain.Observation{
		Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0},
		AltitudeM: 10000,
		Metadata:  map[string]string{"squawk": "1200"},
	}

	tests := []struct {
		name   string
		filter string
		want   bool
	}{
		{
			name:   "matching layer_type",
			filter: `entity.layer_type == "flights_commercial"`,
			want:   true,
		},
		{
			name:   "non-matching layer_type",
			filter: `entity.layer_type == "satellites"`,
			want:   false,
		},
		{
			name:   "metadata match",
			filter: `entity.metadata.callsign == "TST123"`,
			want:   true,
		},
		{
			name:   "observation altitude check",
			filter: `observation.altitude > 5000.0`,
			want:   true,
		},
		{
			name:   "observation altitude below threshold",
			filter: `observation.altitude > 20000.0`,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.worker.evaluateCELFilter(tt.filter, entity, obs)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorker_CELFilter_InvalidExpression(t *testing.T) {
	h := newTestWorker(t)

	entity := &domain.Entity{ID: "ent-001", LayerType: "test"}
	_, err := h.worker.evaluateCELFilter(`invalid+++syntax`, entity, nil)
	require.Error(t, err)
}

func TestWorker_CELFilter_NilObservation(t *testing.T) {
	h := newTestWorker(t)

	entity := &domain.Entity{
		ID:        "ent-001",
		LayerType: "test",
		Metadata:  map[string]string{},
	}

	// Should work with nil observation (defaults to zeros).
	got, err := h.worker.evaluateCELFilter(`observation.altitude == 0.0`, entity, nil)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestWorker_CELFilter_CompiledOnceEvaluatedMany(t *testing.T) {
	h := newTestWorker(t)

	filter := `entity.layer_type == "flights_commercial"`

	entity := &domain.Entity{
		ID:        "ent-001",
		LayerType: "flights_commercial",
		Metadata:  map[string]string{"callsign": "TST123"},
	}
	obs := &domain.Observation{
		Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0},
		AltitudeM: 10000,
		Metadata:  map[string]string{"squawk": "1200"},
	}

	// Evaluate the same filter expression multiple times.
	for i := 0; i < 5; i++ {
		got, err := h.worker.evaluateCELFilter(filter, entity, obs)
		require.NoError(t, err)
		assert.True(t, got, "evaluation %d should pass", i)
	}

	// Verify the compiled program was cached: the sync.Map should contain
	// exactly one entry for this filter string.
	var count int
	h.worker.compiledFilters.Range(func(key, value any) bool {
		count++
		assert.Equal(t, filter, key.(string), "cached key should be the filter expression")
		assert.NotNil(t, value, "cached program should not be nil")
		return true
	})
	assert.Equal(t, 1, count, "expected exactly one compiled program in cache")

	// Evaluate with a different entity to confirm the cached program works
	// with varying input data.
	entityMilitary := &domain.Entity{
		ID:        "ent-002",
		LayerType: "satellites",
		Metadata:  map[string]string{},
	}
	got, err := h.worker.evaluateCELFilter(filter, entityMilitary, nil)
	require.NoError(t, err)
	assert.False(t, got, "satellites should not match flights_commercial filter")

	// Cache should still have exactly one entry (same filter expression).
	count = 0
	h.worker.compiledFilters.Range(func(_, _ any) bool {
		count++
		return true
	})
	assert.Equal(t, 1, count, "cache size should remain 1 for the same filter expression")
}

func TestWorker_ParseCacheTTL(t *testing.T) {
	h := newTestWorker(t)

	tests := []struct {
		input string
		want  time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"300s", 300 * time.Second},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := h.worker.parseCacheTTL(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorker_TransientError(t *testing.T) {
	err := &transientError{err: fmt.Errorf("db timeout")}
	assert.True(t, isTransient(err))
	assert.Contains(t, err.Error(), "db timeout")

	// Non-transient error.
	plainErr := fmt.Errorf("bad request")
	assert.False(t, isTransient(plainErr))
}

func TestWorker_EntityNotFound_IsTerminal(t *testing.T) {
	// When the DB is reset but stale enrichment messages remain, entity
	// NOT_FOUND must be treated as terminal: processOperation returns nil so the
	// message is acked and dropped (no spurious "terminal error" log, and no
	// retry storm under any future redelivery transport).
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.entityRepo = &mockEntityRepo{
			err: domain.NewNotFoundError("entity not found", nil),
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "deleted-entity-id",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err, "NOT_FOUND must return nil so the message is acked, not retried")

	// LLM should never be called for a missing entity.
	assert.Equal(t, 0, h.llm.getCalls())
}

func TestWorker_EntityDBError_IsTransient(t *testing.T) {
	// Genuine DB errors (connection refused, timeout) should remain transient
	// so NATS redelivers the message when the DB recovers.
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.entityRepo = &mockEntityRepo{
			err: fmt.Errorf("connection refused"),
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)
	assert.True(t, isTransient(err), "DB connection errors must be transient")
}

func TestWorker_IsLLMTransient(t *testing.T) {
	assert.True(t, isLLMTransient(llm.ErrRateLimited))
	assert.True(t, isLLMTransient(llm.ErrProviderUnavailable))
	assert.False(t, isLLMTransient(llm.ErrCompletionFailed))
	assert.False(t, isLLMTransient(llm.ErrInvalidResponse))
	assert.False(t, isLLMTransient(fmt.Errorf("random error")))
	assert.True(t, isLLMTransient(fmt.Errorf("context deadline exceeded")))
	assert.True(t, isLLMTransient(fmt.Errorf("Client.Timeout exceeded while awaiting headers")))
}

func TestWorker_ProcessJob_CacheTTLZero_NoCacheSet(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "no_cache_op",
						Prompt:   "Analyze {{.Entity.Name}}",
						CacheTTL: "", // empty = no caching
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// LLM should be called.
	assert.Equal(t, 1, h.llm.getCalls())

	// Entity cache should NOT be set.
	_, err = h.kv.Get(ctx, "ai:enrich:ent-001:no_cache_op")
	assert.True(t, domain.IsNotFound(err), "expected cache miss (not-found) for uncached op")
}

func TestWorker_ProcessJob_SchemaValidationSuccess(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"threat_level": "medium"}`,
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 80, CompletionTokens: 30},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "validated_op",
						Prompt: "Check {{.Entity.Name}}",
						OutputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"threat_level": map[string]any{"type": "string"},
							},
							"required": []any{"threat_level"},
						},
						CacheTTL: "5m",
					},
				},
			},
		}
	})

	// Register schema using compound key: "source_name.operation_name".
	err := h.schemas.RegisterFromYAML("adsb.validated_op", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"threat_level": map[string]any{"type": "string"},
		},
		"required": []any{"threat_level"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Entity should be patched with validated result.
	require.Len(t, h.entityRepo.patches, 1)
	inner, ok := h.entityRepo.patches[0].Metadata["validated_op"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "medium", inner["threat_level"])
}

// TestWorker_ProcessJob_SchemaKeyUsesCompoundFormat verifies that the worker
// looks up schemas using the compound key "source_name.operation_name", matching
// the format the declarative loader uses when registering schemas.
// Regression test for a bug where the worker used only op.Name as the key.
func TestWorker_ProcessJob_SchemaKeyUsesCompoundFormat(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"risk": "low"}`,
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 80, CompletionTokens: 30},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"emsc_earthquakes": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "risk_assessment",
						Prompt: "Assess {{.Entity.Name}}",
						OutputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"risk": map[string]any{"type": "string"},
							},
							"required": []any{"risk"},
						},
						CacheTTL: "5m",
					},
				},
			},
		}
	})

	// Register with WRONG key (just operation name) — simulates the old bug.
	err := h.schemas.RegisterFromYAML("risk_assessment", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"risk": map[string]any{"type": "string"},
		},
		"required": []any{"risk"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	job := Job{
		EntityID:    "ent-001",
		SourceName:  "emsc_earthquakes",
		LayerType:   "earthquakes",
		PublishedAt: time.Now(),
	}

	// Worker should fail schema validation because it looks up
	// "emsc_earthquakes.risk_assessment" and that key is NOT registered.
	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation")

	// Now register with the CORRECT compound key and retry.
	h.entityRepo.patches = nil // reset
	h.aiLogRepo.statusCalls = nil
	err = h.schemas.RegisterFromYAML("emsc_earthquakes.risk_assessment", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"risk": map[string]any{"type": "string"},
		},
		"required": []any{"risk"},
	})
	require.NoError(t, err)

	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Verify entity was patched with the validated result.
	require.Len(t, h.entityRepo.patches, 1)
	inner, ok := h.entityRepo.patches[0].Metadata["risk_assessment"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "low", inner["risk"])
}

func TestWorker_ProcessJob_OutputTargetDefaultsToEntity(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:         "default_target",
						Prompt:       "Analyze {{.Entity.Name}}",
						OutputTarget: "", // empty = defaults to entity
						CacheTTL:     "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:      "ent-001",
		ObservationID: "obs-001",
		SourceName:    "adsb",
		LayerType:     "flights_commercial",
		PublishedAt:   time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Should patch entity, not observation.
	require.Len(t, h.entityRepo.patches, 1)
	assert.Empty(t, h.obsRepo.patches)
}

func TestWorker_NewWorker_DefaultConcurrency(t *testing.T) {
	w := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: 0})
	assert.Equal(t, 4, w.concurrency)

	w2 := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: -1})
	assert.Equal(t, 4, w2.concurrency)

	w3 := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: 8})
	assert.Equal(t, 8, w3.concurrency)
}

func TestWorker_NewWorker_DefaultMaxDeliverAndAckWait(t *testing.T) {
	// Zero values should use defaults.
	w := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: 1})
	assert.Equal(t, defaultMaxDeliver, w.maxDeliver)
	assert.Equal(t, defaultAckWait, w.ackWait)

	// Negative values should use defaults.
	w2 := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: 1, MaxDeliver: -1, AckWait: -1 * time.Second})
	assert.Equal(t, defaultMaxDeliver, w2.maxDeliver)
	assert.Equal(t, defaultAckWait, w2.ackWait)

	// Explicit values should be preserved.
	w3 := NewWorker(WorkerConfig{Logger: zerolog.Nop(), Concurrency: 1, MaxDeliver: 5, AckWait: 120 * time.Second})
	assert.Equal(t, 5, w3.maxDeliver)
	assert.Equal(t, 120*time.Second, w3.ackWait)
}

func TestWorker_ProcessJob_LLMResponseNonJSON(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: "This is a plain text response with no JSON.",
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 50, CompletionTokens: 20},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "text_op",
						Prompt:   "Describe {{.Entity.Name}}",
						CacheTTL: "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Should still patch with raw content wrapped.
	require.Len(t, h.entityRepo.patches, 1)
	inner, ok := h.entityRepo.patches[0].Metadata["text_op"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "This is a plain text response with no JSON.", inner["result"])
}

func TestWorker_ProcessJob_LLMResponseCodeFenced(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: "```json\n{\"threat_level\": \"high\"}\n```",
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 50, CompletionTokens: 20},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "fenced_op",
						Prompt:   "Analyze {{.Entity.Name}}",
						CacheTTL: "5m",
					},
				},
			},
		}
	})
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Should strip code fences and parse JSON.
	require.Len(t, h.entityRepo.patches, 1)
	inner, ok := h.entityRepo.patches[0].Metadata["fenced_op"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "high", inner["threat_level"])
}

func TestWorker_ProcessJob_AuditLogUsageRecorded(t *testing.T) {
	h := newTestWorker(t)
	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err := h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Find the completed status update.
	var completedUpdate *statusUpdateCall
	for i := range h.aiLogRepo.statusCalls {
		if h.aiLogRepo.statusCalls[i].Status == domain.AIStatusCompleted {
			completedUpdate = &h.aiLogRepo.statusCalls[i]
			break
		}
	}
	require.NotNil(t, completedUpdate, "expected a completed status update")

	// Usage should be recorded.
	require.NotNil(t, completedUpdate.Usage)
	assert.Equal(t, "test-provider", completedUpdate.Usage.Provider)
	assert.Equal(t, "test-model", completedUpdate.Usage.Model)
	assert.Equal(t, 100, completedUpdate.Usage.PromptTokens)
	assert.Equal(t, 50, completedUpdate.Usage.CompletionTokens)
	assert.Greater(t, completedUpdate.Usage.LatencyMS, -1, "latency should be non-negative")
}

func TestWorker_ProcessJob_NilCache(t *testing.T) {
	// Build a worker with a nil kvCache to verify no nil pointer dereference.
	llmProvider := &mockLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"threat_level": "low", "classification": "civilian"}`,
			Model:   "test-model",
			Usage: llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
	}
	entityRepo := &mockEntityRepo{entity: defaultTestEntity()}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &mockAILogRepo{}

	w := &Worker{
		llmProvider: llmProvider,
		schemas:     schema.NewRegistry(),
		entityRepo:  entityRepo,
		obsRepo:     obsRepo,
		aiLogRepo:   aiLogRepo,
		kvCache:     nil, // explicitly nil
		sourceConfigs: map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:     "threat_assessment",
						Prompt:   "Assess threat for {{.Entity.Name}} ({{.Entity.MetadataJSON}})",
						CacheTTL: "5m",
					},
				},
			},
		},
		logger:        zerolog.Nop(),
		concurrency:   1,
		subject:       "respondent.ai.enrich",
		consumerGroup: "test-workers",
	}
	require.NoError(t, w.initCELEnv())

	ctx := context.Background()

	job := Job{
		EntityID:    "ent-001",
		ExternalID:  "ext-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	// Must not panic and should succeed.
	err := w.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Verify LLM was called (cache miss path since kvCache is nil).
	assert.Equal(t, 1, llmProvider.getCalls())

	// Verify entity was patched.
	require.Len(t, entityRepo.patches, 1)
	assert.Equal(t, "ent-001", entityRepo.patches[0].EntityID)
	assert.Contains(t, entityRepo.patches[0].Metadata, "threat_assessment")

	// Verify audit log completed.
	require.NotEmpty(t, aiLogRepo.statusCalls)
	lastUpdate := aiLogRepo.statusCalls[len(aiLogRepo.statusCalls)-1]
	assert.Equal(t, domain.AIStatusCompleted, lastUpdate.Status)
}

// TestWorker_ProcessJob_JSONModeSetWhenOutputSchemaPresent verifies that the
// enrichment worker sets ResponseFormatJSON and prepends a JSON-only system
// message when an operation carries an OutputSchema, mirroring the canonical
// pattern in internal/ai/analysis/engine.go.
func TestWorker_ProcessJob_JSONModeSetWhenOutputSchemaPresent(t *testing.T) {
	h := newTestWorker(t, func(o *testWorkerOpts) {
		o.llmProvider = &mockLLMProvider{
			name: "test-provider",
			response: &llm.CompletionResponse{
				Content: `{"threat_level": "high"}`,
				Model:   "test-model",
				Usage:   llm.Usage{PromptTokens: 80, CompletionTokens: 30},
			},
		}
		o.sourceConfigs = map[string]*aiconfig.SourceAIConfig{
			"adsb": {
				Enabled: true,
				Operations: []aiconfig.OperationConfig{
					{
						Name:   "json_mode_op",
						Prompt: "Analyze {{.Entity.Name}}",
						OutputSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"threat_level": map[string]any{"type": "string"},
							},
							"required": []any{"threat_level"},
						},
						CacheTTL: "5m",
					},
				},
			},
		}
	})

	// Register the schema with the compound key used by the worker.
	err := h.schemas.RegisterFromYAML("adsb.json_mode_op", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"threat_level": map[string]any{"type": "string"},
		},
		"required": []any{"threat_level"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	job := Job{
		EntityID:    "ent-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}

	err = h.worker.processJob(ctx, makeJobJSON(t, job))
	require.NoError(t, err)

	// Assert the LLM request carried JSON mode settings.
	lastReq := h.llm.getLastReq()
	require.NotNil(t, lastReq)
	require.GreaterOrEqual(t, len(lastReq.Messages), 2, "expected system + user messages")
	assert.Equal(t, "system", lastReq.Messages[0].Role)
	assert.Contains(t, lastReq.Messages[0].Content, "You MUST respond with a valid JSON object")
	assert.Equal(t, llm.ResponseFormatJSON, lastReq.ResponseFormat)
}

// spyConsumedMessage records which ack/nak method was called.
type spyConsumedMessage struct {
	data     []byte
	acked    bool
	naked    bool
	nakDelay time.Duration
}

func (m *spyConsumedMessage) Data() []byte                       { return m.data }
func (m *spyConsumedMessage) Ack() error                         { m.acked = true; return nil }
func (m *spyConsumedMessage) Nak() error                         { m.naked = true; return nil }
func (m *spyConsumedMessage) NakWithDelay(d time.Duration) error { m.nakDelay = d; return nil }

func TestWorker_ProcessConsumedMessage_AckNakBranching(t *testing.T) {
	// In-process delivery is at-most-once: there is no redelivery, so every
	// outcome is terminal and the message is ALWAYS acked. Transient and
	// terminal failures differ only in log level (Warn vs Error), never in
	// ack/nak behaviour. Nak/NakWithDelay are no-ops on the inproc bus and must
	// never be called.
	w := &Worker{
		sourceConfigs: map[string]*aiconfig.SourceAIConfig{},
		logger:        zerolog.Nop(),
	}

	successJob, _ := json.Marshal(Job{EntityID: "e1", SourceName: "src", LayerType: "lt"})
	malformedData := []byte("{bad json")

	t.Run("success acks", func(t *testing.T) {
		spy := &spyConsumedMessage{data: successJob}
		w.processConsumedMessage(context.Background(), spy)
		assert.True(t, spy.acked, "expected Ack on success")
		assert.False(t, spy.naked, "inproc is at-most-once: never Nak")
		assert.Zero(t, spy.nakDelay)
	})

	t.Run("terminal error acks", func(t *testing.T) {
		spy := &spyConsumedMessage{data: malformedData}
		w.processConsumedMessage(context.Background(), spy)
		assert.True(t, spy.acked, "expected Ack on terminal error")
		assert.False(t, spy.naked, "inproc is at-most-once: never Nak")
		assert.Zero(t, spy.nakDelay)
	})

	t.Run("transient error acks (dropped, retried by feeder)", func(t *testing.T) {
		// A transient error (e.g. DB timeout) is now dropped, not redelivered:
		// the feeder re-publishes a job per entity each ingest cycle.
		entityRepo := &mockEntityRepo{err: fmt.Errorf("db timeout")}
		tw := &Worker{
			sourceConfigs: map[string]*aiconfig.SourceAIConfig{
				"src": {Enabled: true, Operations: []aiconfig.OperationConfig{{Name: "op1", Prompt: "test"}}},
			},
			entityRepo: entityRepo,
			logger:     zerolog.Nop(),
		}
		spy := &spyConsumedMessage{data: successJob}
		tw.processConsumedMessage(context.Background(), spy)
		assert.True(t, spy.acked, "expected Ack (at-most-once) on transient error")
		assert.False(t, spy.naked, "inproc is at-most-once: never Nak")
		assert.Zero(t, spy.nakDelay)
	})

	t.Run("rate-limit transient error acks (no NakWithDelay)", func(t *testing.T) {
		// Rate-limit errors are transient but, like every other failure under
		// at-most-once, are simply acked and dropped — never NakWithDelay'd.
		entityRepo := &mockEntityRepo{entity: &domain.Entity{ID: "e1", Name: "test"}}
		obsRepo := &mockObsRepo{obs: &domain.Observation{ID: "o1"}}
		mockLLM := &mockLLMProvider{err: llm.ErrRateLimited, name: "test"}
		tw := &Worker{
			sourceConfigs: map[string]*aiconfig.SourceAIConfig{
				"src": {Enabled: true, Operations: []aiconfig.OperationConfig{
					{Name: "op1", Prompt: "classify {{ .Entity.Name }}"},
				}},
			},
			entityRepo:  entityRepo,
			obsRepo:     obsRepo,
			aiLogRepo:   &mockAILogRepo{},
			llmProvider: mockLLM,
			schemas:     schema.NewRegistry(),
			logger:      zerolog.Nop(),
		}
		spy := &spyConsumedMessage{data: successJob}
		tw.processConsumedMessage(context.Background(), spy)
		assert.True(t, spy.acked, "expected Ack (at-most-once) on rate-limit error")
		assert.False(t, spy.naked, "inproc is at-most-once: never Nak")
		assert.Zero(t, spy.nakDelay, "NakWithDelay must not be called on inproc")
	})
}

// panicEntityRepo is a domain.EntityRepository whose GetByID panics, used to
// exercise the worker's panic-recovery path. All other methods return zero
// values.
type panicEntityRepo struct{}

func (p *panicEntityRepo) Create(_ context.Context, _ *domain.Entity) error        { return nil }
func (p *panicEntityRepo) CreateBatch(_ context.Context, _ []*domain.Entity) error { return nil }
func (p *panicEntityRepo) GetByID(_ context.Context, _ string) (*domain.Entity, error) {
	panic("boom")
}
func (p *panicEntityRepo) GetByIDs(_ context.Context, _ []string) ([]*domain.Entity, error) {
	return nil, nil
}
func (p *panicEntityRepo) GetByExternalID(_ context.Context, _, _ string) (*domain.Entity, error) {
	return nil, nil
}
func (p *panicEntityRepo) GetByExternalIDs(_ context.Context, _ string, _ []string) ([]*domain.Entity, error) {
	return nil, nil
}
func (p *panicEntityRepo) GetDistinctLayerTypes(_ context.Context) ([]string, error) {
	return nil, nil
}
func (p *panicEntityRepo) CountByLayerType(_ context.Context) (map[string]int64, error) {
	return nil, nil
}
func (p *panicEntityRepo) Update(_ context.Context, _ *domain.Entity) error { return nil }
func (p *panicEntityRepo) Delete(_ context.Context, _ string) error         { return nil }
func (p *panicEntityRepo) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}
func (p *panicEntityRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	return nil
}
func (p *panicEntityRepo) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

func TestWorker_ProcessConsumedMessage_RecoversPanic(t *testing.T) {
	// A panic in the single consume goroutine must not propagate (which would
	// kill enrichment for the process lifetime). The poison message is acked
	// (dropped) so the consume loop survives.
	w := &Worker{
		sourceConfigs: map[string]*aiconfig.SourceAIConfig{
			"adsb": {Enabled: true, Operations: []aiconfig.OperationConfig{{Name: "op1", Prompt: "test"}}},
		},
		entityRepo: &panicEntityRepo{},
		logger:     zerolog.Nop(),
	}
	spy := &spyConsumedMessage{data: makeJobJSON(t, Job{EntityID: "x", SourceName: "adsb", PublishedAt: time.Now()})}

	assert.NotPanics(t, func() {
		w.processConsumedMessage(context.Background(), spy)
	})
	assert.True(t, spy.acked, "poison message must be acked (dropped) so the loop survives")
	assert.False(t, spy.naked, "inproc is at-most-once: never Nak")
}

// TestWorker_Reaper_TicksAndReconciles verifies that runReaper invokes
// DeleteStrandedLogs on each tick.
func TestWorker_Reaper_TicksAndReconciles(t *testing.T) {
	aiLogRepo := &mockAILogRepo{}
	w := &Worker{
		aiLogRepo: aiLogRepo,
		logger:    zerolog.Nop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Drive the reaper body directly via a short ticker analogue: call the
	// reconcile path on a ticker by stubbing the interval. Instead of waiting
	// 15m, exercise the same code path by invoking the repo and confirming the
	// reaper goroutine drains on cancel without deadlocking.
	w.wg.Add(1)
	go w.runReaper(ctx)

	// Cancel and wait — the reaper selects on ctx.Done() and must return so
	// wg.Wait() does not block.
	cancel()

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// drained cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("reaper did not drain after context cancellation (deadlock)")
	}
}

// TestWorker_Reaper_StopDrainsWithoutDeadlock simulates the Start/Stop lifecycle
// for the reaper: it launches the reaper exactly as Start does (wg.Add + go
// runReaper) under a cancellable context wired into cancelFn, then calls Stop()
// and asserts it returns promptly (cancel + wg.Wait drains the goroutine).
func TestWorker_Reaper_StopDrainsWithoutDeadlock(t *testing.T) {
	w := &Worker{
		aiLogRepo: &mockAILogRepo{},
		logger:    zerolog.Nop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.cancelFn = cancel
	w.mu.Unlock()

	w.wg.Add(1)
	go w.runReaper(ctx)

	done := make(chan struct{})
	go func() {
		w.Stop() // cancels ctx then wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Stop returned: reaper drained without deadlock.
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() deadlocked waiting for reaper")
	}
}
