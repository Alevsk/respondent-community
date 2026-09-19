package enrichment

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inmem"
	"github.com/Alevsk/respondent/internal/infra/inproc"
	"github.com/Alevsk/respondent/internal/llm"
)

// ---------------------------------------------------------------------------
// Integration test helpers
// ---------------------------------------------------------------------------

// newInprocHarness creates a fresh inproc.Bus with a publisher and consumer
// for each test. This mirrors the community-edition wiring in serve.go.
func newInprocHarness(_ *testing.T) (domain.StreamConsumer, domain.MessagePublisher) {
	bus := inproc.NewBus(256)
	return inproc.NewConsumer(bus), inproc.NewPublisher(bus)
}

// trackingEntityRepo is a thread-safe mock that records PatchAIMetadata calls.
type trackingEntityRepo struct {
	mockEntityRepo
	mu      sync.Mutex
	patches []patchCall
}

func (r *trackingEntityRepo) PatchAIMetadata(_ context.Context, entityID string, metadata map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patches = append(r.patches, patchCall{EntityID: entityID, Metadata: metadata})
	return nil
}

func (r *trackingEntityRepo) getPatches() []patchCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]patchCall, len(r.patches))
	copy(cp, r.patches)
	return cp
}

// trackingAILogRepo is a thread-safe mock that records audit log calls.
type trackingAILogRepo struct {
	mu          sync.Mutex
	creates     []*domain.AIEnrichmentLog
	statusCalls []statusUpdateCall
}

func (r *trackingAILogRepo) Create(_ context.Context, log *domain.AIEnrichmentLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creates = append(r.creates, log)
	return nil
}

func (r *trackingAILogRepo) UpdateStatus(_ context.Context, id string, status string, result map[string]any, usage *domain.AIUsage, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statusCalls = append(r.statusCalls, statusUpdateCall{
		ID: id, Status: status, Result: result, Usage: usage, ErrMsg: errMsg,
	})
	return nil
}

func (r *trackingAILogRepo) GetByEntityAndOperation(_ context.Context, _, _ string) (*domain.AIEnrichmentLog, error) {
	return nil, nil
}

func (r *trackingAILogRepo) DeleteStrandedLogs(_ context.Context, _ time.Duration, _ []string) (int64, error) {
	return 0, nil
}

func (r *trackingAILogRepo) getCreates() []*domain.AIEnrichmentLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]*domain.AIEnrichmentLog, len(r.creates))
	copy(cp, r.creates)
	return cp
}

func (r *trackingAILogRepo) getStatusCalls() []statusUpdateCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]statusUpdateCall, len(r.statusCalls))
	copy(cp, r.statusCalls)
	return cp
}

// trackingLLMProvider is a thread-safe mock LLM that counts calls.
type trackingLLMProvider struct {
	mu       sync.Mutex
	name     string
	response *llm.CompletionResponse
	err      error
	calls    int
}

func (m *trackingLLMProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *trackingLLMProvider) Name() string { return m.name }
func (m *trackingLLMProvider) SupportsProvider(providerType string) bool {
	return providerType == m.name
}
func (m *trackingLLMProvider) HealthCheck(_ context.Context) error { return nil }

func (m *trackingLLMProvider) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// waitFor polls a condition up to a timeout.
func waitFor(t *testing.T, timeout time.Duration, interval time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("timed out waiting for: %s", msg)
}

// ---------------------------------------------------------------------------
// Integration tests — all run over the real inproc bus + memcache
// ---------------------------------------------------------------------------

func TestIntegration_PublishAndConsume_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"threat_level": "low", "classification": "civilian"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
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
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   2,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-int-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerErr := make(chan error, 1)
	go func() {
		workerErr <- worker.Start(ctx)
	}()
	// Give worker time to register its subscription on the bus.
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:    "ent-001",
		ExternalID:  "ext-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}
	require.NoError(t, pub.Publish(ctx, job))

	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 1
	}, "entity to be patched")

	patches := entityRepo.getPatches()
	require.Len(t, patches, 1)
	assert.Equal(t, "ent-001", patches[0].EntityID)
	inner, ok := patches[0].Metadata["threat_assessment"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "low", inner["threat_level"])
	assert.Equal(t, "civilian", inner["classification"])

	assert.Equal(t, 1, llmProv.getCalls())

	waitFor(t, 2*time.Second, 50*time.Millisecond, func() bool {
		return len(aiLogRepo.getStatusCalls()) >= 1
	}, "audit log status update")

	creates := aiLogRepo.getCreates()
	require.NotEmpty(t, creates)
	assert.Equal(t, "ent-001", creates[0].EntityID)
	assert.Equal(t, domain.AIStatusPending, creates[0].Status)

	statuses := aiLogRepo.getStatusCalls()
	var completed *statusUpdateCall
	for i := range statuses {
		if statuses[i].Status == domain.AIStatusCompleted {
			completed = &statuses[i]
			break
		}
	}
	require.NotNil(t, completed)
	assert.Empty(t, completed.ErrMsg)
	require.NotNil(t, completed.Usage)
	assert.Equal(t, "test-provider", completed.Usage.Provider)
	assert.Equal(t, 100, completed.Usage.PromptTokens)

	worker.Stop()
}

func TestIntegration_PublishBatch_MultipleJobs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"status": "analyzed"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 80, CompletionTokens: 30},
		},
	}

	entity1 := &domain.Entity{ID: "ent-001", ExternalID: "ext-001", Name: "Aircraft A", LayerType: "flights", Metadata: map[string]string{}}
	entity2 := &domain.Entity{ID: "ent-002", ExternalID: "ext-002", Name: "Aircraft B", LayerType: "flights", Metadata: map[string]string{}}
	entity3 := &domain.Entity{ID: "ent-003", ExternalID: "ext-003", Name: "Aircraft C", LayerType: "flights", Metadata: map[string]string{}}

	entityLookup := map[string]*domain.Entity{
		"ent-001": entity1,
		"ent-002": entity2,
		"ent-003": entity3,
	}

	multiEntityRepo := &multiEntityMockRepo{entities: entityLookup}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:     "batch_analysis",
					Prompt:   "Analyze {{.Entity.Name}}",
					CacheTTL: "5m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    multiEntityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   4,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-batch-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	jobs := []Job{
		{EntityID: "ent-001", ExternalID: "ext-001", SourceName: "adsb", LayerType: "flights", PublishedAt: time.Now()},
		{EntityID: "ent-002", ExternalID: "ext-002", SourceName: "adsb", LayerType: "flights", PublishedAt: time.Now()},
		{EntityID: "ent-003", ExternalID: "ext-003", SourceName: "adsb", LayerType: "flights", PublishedAt: time.Now()},
	}
	require.NoError(t, pub.PublishBatch(ctx, jobs))

	waitFor(t, 10*time.Second, 100*time.Millisecond, func() bool {
		return multiEntityRepo.getPatchCount() >= 3
	}, "all 3 entities to be patched")

	patches := multiEntityRepo.getPatches()
	patchedIDs := make(map[string]bool)
	for _, p := range patches {
		patchedIDs[p.EntityID] = true
	}
	assert.True(t, patchedIDs["ent-001"])
	assert.True(t, patchedIDs["ent-002"])
	assert.True(t, patchedIDs["ent-003"])

	assert.Equal(t, 3, llmProv.getCalls())

	waitFor(t, 2*time.Second, 50*time.Millisecond, func() bool {
		return len(aiLogRepo.getCreates()) >= 3
	}, "3 audit logs created")

	worker.Stop()
}

func TestIntegration_CachePreventsRepeat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"result": "done"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 50, CompletionTokens: 20},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}
	kvCache := inmem.NewKVCache()

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:     "cached_op",
					Prompt:   "Assess {{.Entity.Name}}",
					CacheTTL: "10m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       kvCache,
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-cache-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:   "ent-001",
		SourceName: "adsb",
		LayerType:  "flights_commercial",
	}

	// Publish the same job twice.
	require.NoError(t, pub.Publish(ctx, job))
	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 1
	}, "first job to be processed")

	// Small delay to ensure cache is written before second publish.
	time.Sleep(100 * time.Millisecond)

	require.NoError(t, pub.Publish(ctx, job))
	// Give worker time to consume (and skip due to cache).
	time.Sleep(300 * time.Millisecond)

	// LLM should only be called once — second job hits entity cache.
	assert.Equal(t, 1, llmProv.getCalls())

	// Entity should only be patched once.
	assert.Len(t, entityRepo.getPatches(), 1)

	worker.Stop()
}

func TestIntegration_CELFilterSkipsNonMatching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"result": "ok"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 50, CompletionTokens: 20},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:   "filtered_op",
					Prompt: "Analyze {{.Entity.Name}}",
					Filter: `entity.layer_type == "satellites"`, // does NOT match "flights_commercial"
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-filter-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:   "ent-001",
		SourceName: "adsb",
		LayerType:  "flights_commercial",
	}
	require.NoError(t, pub.Publish(ctx, job))

	// Wait for audit log to be created (CEL filter skip creates it directly with status=skipped).
	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(aiLogRepo.getCreates()) >= 1
	}, "audit log created")

	assert.Equal(t, 0, llmProv.getCalls())
	assert.Empty(t, entityRepo.getPatches())

	creates := aiLogRepo.getCreates()
	assert.Equal(t, domain.AIStatusSkipped, creates[0].Status)

	worker.Stop()
}

func TestIntegration_SchemaValidation_RejectsInvalid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	// LLM returns response missing required "threat_level" field.
	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"wrong_field": "oops"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 80, CompletionTokens: 40},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	schemaMap := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"threat_level": map[string]any{"type": "string"},
		},
		"required": []any{"threat_level"},
	}

	schemaReg := schema.NewRegistry()
	require.NoError(t, schemaReg.RegisterFromYAML("adsb.strict_op", schemaMap))

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:         "strict_op",
					Prompt:       "Check {{.Entity.Name}}",
					OutputSchema: schemaMap,
					CacheTTL:     "5m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schemaReg,
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-schema-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:   "ent-001",
		SourceName: "adsb",
		LayerType:  "flights_commercial",
	}
	require.NoError(t, pub.Publish(ctx, job))

	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		statuses := aiLogRepo.getStatusCalls()
		for _, s := range statuses {
			if s.Status == domain.AIStatusFailed {
				return true
			}
		}
		return false
	}, "schema validation failure in audit log")

	assert.Empty(t, entityRepo.getPatches())

	statuses := aiLogRepo.getStatusCalls()
	var failed *statusUpdateCall
	for i := range statuses {
		if statuses[i].Status == domain.AIStatusFailed {
			failed = &statuses[i]
			break
		}
	}
	require.NotNil(t, failed)
	assert.Contains(t, failed.ErrMsg, "schema validation")

	worker.Stop()
}

func TestIntegration_OutputMapping_Allowlist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"threat_level": "high", "classification": "military", "secret_data": "should_not_persist"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:   "mapped_op",
					Prompt: "Assess {{.Entity.Name}}",
					OutputMapping: map[string]string{
						"threat_level":   "ai.threat",
						"classification": "ai.class",
					},
					CacheTTL: "5m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-map-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:   "ent-001",
		SourceName: "adsb",
		LayerType:  "flights_commercial",
	}
	require.NoError(t, pub.Publish(ctx, job))

	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 1
	}, "entity to be patched with mapped fields")

	patches := entityRepo.getPatches()
	require.Len(t, patches, 1)
	patch := patches[0].Metadata

	assert.Equal(t, "high", patch["ai.threat"])
	assert.Equal(t, "military", patch["ai.class"])

	_, hasSecret := patch["secret_data"]
	assert.False(t, hasSecret, "unmapped fields should not be persisted")

	worker.Stop()
}

func TestIntegration_PromptHashCache_SharesAcrossEntities(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	consumer, msgPublisher := newInprocHarness(t)

	llmProv := &trackingLLMProvider{
		name: "test-provider",
		response: &llm.CompletionResponse{
			Content: `{"analysis": "identical"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 60, CompletionTokens: 25},
		},
	}

	// Two entities with the same name → same rendered prompt → prompt cache hit.
	sharedEntity := &domain.Entity{
		ID:         "ent-001",
		ExternalID: "ext-001",
		Name:       "Shared Name",
		LayerType:  "flights",
		Metadata:   map[string]string{},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: sharedEntity},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:     "prompt_hash_op",
					Prompt:   "Analyze entity Shared Name",
					CacheTTL: "10m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "test-phash-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = worker.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)

	// First job: entity ent-001 (calls LLM, populates prompt cache).
	job1 := Job{EntityID: "ent-001", SourceName: "adsb", LayerType: "flights"}
	require.NoError(t, pub.Publish(ctx, job1))

	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 1
	}, "first job processed")

	// Second job: different entity ID (ent-002) but same prompt.
	// Entity cache won't match (different entity ID), but prompt cache will.
	sharedEntity2 := &domain.Entity{
		ID: "ent-002", ExternalID: "ext-002", Name: "Shared Name",
		LayerType: "flights", Metadata: map[string]string{},
	}
	entityRepo.mu.Lock()
	entityRepo.entity = sharedEntity2
	entityRepo.mu.Unlock()

	job2 := Job{EntityID: "ent-002", SourceName: "adsb", LayerType: "flights"}
	require.NoError(t, pub.Publish(ctx, job2))

	waitFor(t, 5*time.Second, 50*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 2
	}, "second job processed")

	// LLM should only be called once — second job used prompt cache.
	assert.Equal(t, 1, llmProv.getCalls())

	patches := entityRepo.getPatches()
	require.Len(t, patches, 2)
	for _, p := range patches {
		inner, ok := p.Metadata["prompt_hash_op"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "identical", inner["analysis"])
	}

	worker.Stop()
}

// ---------------------------------------------------------------------------
// multiEntityMockRepo supports lookup by entity ID for batch tests.
// ---------------------------------------------------------------------------

type multiEntityMockRepo struct {
	mockEntityRepo
	mu       sync.Mutex
	entities map[string]*domain.Entity
	patches  []patchCall
}

func (r *multiEntityMockRepo) GetByID(_ context.Context, id string) (*domain.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entities[id]
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}
	return e, nil
}

func (r *multiEntityMockRepo) PatchAIMetadata(_ context.Context, entityID string, metadata map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patches = append(r.patches, patchCall{EntityID: entityID, Metadata: metadata})
	return nil
}

func (r *multiEntityMockRepo) getPatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.patches)
}

func (r *multiEntityMockRepo) getPatches() []patchCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]patchCall, len(r.patches))
	copy(cp, r.patches)
	return cp
}

// ---------------------------------------------------------------------------
// Real in-process bus end-to-end test (Task 11 regression guard)
//
// This test wires the REAL inproc bus + the real enrichment.Publisher (rooted
// at inproc.EnrichJobRoot) + the real enrichment.Worker subscribed via its
// production "subject + .>" filter, publishes one job, and asserts the full
// terminal outcome: (a) the worker's handler ran (LLM stub fired), (b) a
// completed ai_enrichment_log row exists, (c) ai_metadata was patched. It would
// FAIL against a pre-Task-1 exact-match bus.
// ---------------------------------------------------------------------------

// signallingLLMProvider is a stub llm.Provider that returns canned schema-valid
// JSON and closes a channel the first time it is called, enabling a bounded wait
// on a signal rather than an arbitrary sleep.
type signallingLLMProvider struct {
	mu       sync.Mutex
	name     string
	response *llm.CompletionResponse
	calls    int
	called   chan struct{}
	once     sync.Once
}

func (m *signallingLLMProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	m.once.Do(func() { close(m.called) })
	return m.response, nil
}

func (m *signallingLLMProvider) Name() string { return m.name }
func (m *signallingLLMProvider) SupportsProvider(providerType string) bool {
	return providerType == m.name
}
func (m *signallingLLMProvider) HealthCheck(_ context.Context) error { return nil }

func (m *signallingLLMProvider) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func TestIntegration_InprocBus_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// --- Real in-process bus + production wiring --------------------------
	bus := inproc.NewBus(16)
	msgPublisher := inproc.NewPublisher(bus)
	consumer := inproc.NewConsumer(bus)

	llmProv := &signallingLLMProvider{
		name:   "test-provider",
		called: make(chan struct{}),
		response: &llm.CompletionResponse{
			Content: `{"threat_level": "low", "classification": "civilian"}`,
			Model:   "test-model",
			Usage:   llm.Usage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150},
		},
	}

	entityRepo := &trackingEntityRepo{
		mockEntityRepo: mockEntityRepo{entity: defaultTestEntity()},
	}
	obsRepo := &mockObsRepo{obs: defaultTestObservation()}
	aiLogRepo := &trackingAILogRepo{}

	sourceConfigs := map[string]*aiconfig.SourceAIConfig{
		"adsb": {
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:     "threat_assessment",
					Prompt:   "Assess threat for {{.Entity.Name}}",
					CacheTTL: "5m",
				},
			},
		},
	}

	worker := NewWorker(WorkerConfig{
		Consumer:      consumer,
		LLMProvider:   llmProv,
		Schemas:       schema.NewRegistry(),
		EntityRepo:    entityRepo,
		ObsRepo:       obsRepo,
		AILogRepo:     aiLogRepo,
		KVCache:       inmem.NewKVCache(),
		SourceConfigs: sourceConfigs,
		Logger:        zerolog.Nop(),
		Concurrency:   1,
		// Subject mirrors the production default. The worker subscribes
		// Subject+".>", and the enrichment.Publisher publishes
		// EnrichJobRoot+".<source>" — exercising the wildcard match.
		Subject:       inproc.EnrichJobRoot,
		ConsumerGroup: "community-ai-workers",
		StreamName:    "AI_ENRICH",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerErr := make(chan error, 1)
	go func() { workerErr <- worker.Start(ctx) }()

	// Mirror bus_test.go's subscribe-settle window: give the worker's Consume
	// goroutine time to register its wildcard subscription on the bus before we
	// publish. Without an active subscriber the bus drops the message.
	time.Sleep(50 * time.Millisecond)

	// --- Publish one job through the real enrichment.Publisher ------------
	pub := NewPublisher(msgPublisher, inproc.EnrichJobRoot)
	job := Job{
		EntityID:    "ent-001",
		ExternalID:  "ext-001",
		SourceName:  "adsb",
		LayerType:   "flights_commercial",
		PublishedAt: time.Now(),
	}
	require.NoError(t, pub.Publish(ctx, job))

	// (a) The worker's handler ran: bounded wait on the LLM-called signal.
	select {
	case <-llmProv.called:
	case <-time.After(5 * time.Second):
		t.Fatal("worker handler never ran: LLM stub was not called (silent-drop regression)")
	}

	// (b) A completed ai_enrichment_log row exists.
	waitFor(t, 2*time.Second, 25*time.Millisecond, func() bool {
		for _, s := range aiLogRepo.getStatusCalls() {
			if s.Status == domain.AIStatusCompleted {
				return true
			}
		}
		return false
	}, "completed ai_enrichment_log row")

	creates := aiLogRepo.getCreates()
	require.NotEmpty(t, creates, "a pending audit row must be created")
	assert.Equal(t, "ent-001", creates[0].EntityID)
	assert.Equal(t, domain.AIStatusPending, creates[0].Status)

	var completed *statusUpdateCall
	statuses := aiLogRepo.getStatusCalls()
	for i := range statuses {
		if statuses[i].Status == domain.AIStatusCompleted {
			completed = &statuses[i]
			break
		}
	}
	require.NotNil(t, completed, "expected a completed status update")
	assert.Empty(t, completed.ErrMsg)
	require.NotNil(t, completed.Usage)
	assert.Equal(t, "test-provider", completed.Usage.Provider)

	// (c) ai_metadata was patched with the operation result.
	waitFor(t, 2*time.Second, 25*time.Millisecond, func() bool {
		return len(entityRepo.getPatches()) >= 1
	}, "entity ai_metadata to be patched")

	patches := entityRepo.getPatches()
	require.Len(t, patches, 1)
	assert.Equal(t, "ent-001", patches[0].EntityID)
	inner, ok := patches[0].Metadata["threat_assessment"].(map[string]any)
	require.True(t, ok, "patched metadata must nest the operation result under its name")
	assert.Equal(t, "low", inner["threat_level"])
	assert.Equal(t, "civilian", inner["classification"])

	// LLM stub fired exactly once for the single job.
	assert.Equal(t, 1, llmProv.getCalls())

	// Clean shutdown; Start should return without error after context cancel.
	worker.Stop()
	cancel()
	select {
	case err := <-workerErr:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("worker.Start did not return after shutdown")
	}
}
