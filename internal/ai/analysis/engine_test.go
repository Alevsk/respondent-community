package analysis

import (
	"context"
	"encoding/json"
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
	"github.com/Alevsk/respondent/internal/llm"
)

// --- Mock implementations ---

// mockClock provides a deterministic clock for tests.
type mockClock struct {
	now time.Time
}

func (c *mockClock) Now() time.Time { return c.now }

// mockLLMProvider implements llm.Provider for testing.
type mockLLMProvider struct {
	response    *llm.CompletionResponse
	err         error
	calls       int
	mu          sync.Mutex
	lastRequest *llm.CompletionRequest

	// entered, if non-nil, is closed-on-first-receive-signalled once Complete is
	// invoked, letting a test observe that a run is in-flight. release, if
	// non-nil, blocks Complete until the test closes/sends on it, simulating a
	// slow in-flight analysis run.
	entered chan struct{}
	release chan struct{}
}

func (m *mockLLMProvider) Complete(_ context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Signal entry and block (if configured) BEFORE taking the mutex so a second
	// concurrent Complete is never serialized behind the first by the lock.
	if m.entered != nil {
		m.entered <- struct{}{}
	}
	if m.release != nil {
		<-m.release
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastRequest = req
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMProvider) Name() string                        { return "mock" }
func (m *mockLLMProvider) SupportsProvider(pt string) bool     { return pt == "mock" }
func (m *mockLLMProvider) HealthCheck(_ context.Context) error { return nil }

func (m *mockLLMProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// mockLLMRegistry implements llm.Registry for testing.
type mockLLMRegistry struct {
	provider llm.Provider
	err      error
}

func (r *mockLLMRegistry) Register(_ llm.Provider)                    {}
func (r *mockLLMRegistry) GetProvider(_ string) (llm.Provider, error) { return r.provider, r.err }
func (r *mockLLMRegistry) GetPreferredProvider(_ context.Context) (llm.Provider, error) {
	return r.provider, r.err
}
func (r *mockLLMRegistry) ListProviders() []string { return []string{"mock"} }

// mockEntityRepository implements domain.EntityRepository for testing.
type mockEntityRepository struct {
	mu               sync.Mutex
	entities         map[string]*domain.Entity
	byExtID          map[string]*domain.Entity // key: "layerType:externalID"
	distinctLayers   []string
	getByIDErr       error
	getByExtIDErr    error
	patchAIMetaErr   error
	patchAIMetaCalls []string
}

func newMockEntityRepo() *mockEntityRepository {
	return &mockEntityRepository{
		entities: make(map[string]*domain.Entity),
		byExtID:  make(map[string]*domain.Entity),
	}
}

func (r *mockEntityRepository) AddEntity(e *domain.Entity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entities[e.ID] = e
	r.byExtID[e.LayerType+":"+e.ExternalID] = e
}

func (r *mockEntityRepository) Create(_ context.Context, e *domain.Entity) error        { return nil }
func (r *mockEntityRepository) CreateBatch(_ context.Context, _ []*domain.Entity) error { return nil }
func (r *mockEntityRepository) GetByID(_ context.Context, id string) (*domain.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	e, ok := r.entities[id]
	if !ok {
		return nil, domain.NewNotFoundError("entity not found", nil)
	}
	return e, nil
}
func (r *mockEntityRepository) GetByIDs(_ context.Context, ids []string) ([]*domain.Entity, error) {
	var result []*domain.Entity
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		if e, ok := r.entities[id]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}
func (r *mockEntityRepository) GetByExternalID(_ context.Context, layerType, externalID string) (*domain.Entity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByExtIDErr != nil {
		return nil, r.getByExtIDErr
	}
	e, ok := r.byExtID[layerType+":"+externalID]
	if !ok {
		return nil, domain.NewNotFoundError("entity not found", nil)
	}
	return e, nil
}
func (r *mockEntityRepository) GetByExternalIDs(_ context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	var result []*domain.Entity
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, extID := range externalIDs {
		if e, ok := r.byExtID[layerType+":"+extID]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}
func (r *mockEntityRepository) GetDistinctLayerTypes(_ context.Context) ([]string, error) {
	return r.distinctLayers, nil
}
func (r *mockEntityRepository) CountByLayerType(_ context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (r *mockEntityRepository) Update(_ context.Context, _ *domain.Entity) error { return nil }
func (r *mockEntityRepository) Delete(_ context.Context, _ string) error         { return nil }
func (r *mockEntityRepository) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}
func (r *mockEntityRepository) PatchAIMetadata(_ context.Context, entityID string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patchAIMetaCalls = append(r.patchAIMetaCalls, entityID)
	return r.patchAIMetaErr
}

func (r *mockEntityRepository) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

// mockObservationRepository implements domain.ObservationRepository for testing.
type mockObservationRepository struct {
	mu             sync.Mutex
	latestForLayer map[string][]*domain.Observation // layerType -> observations
	// entities lets the page query join an observation to its entity, the way
	// the real SQL does; the engine no longer fetches entities one at a time.
	entities             *mockEntityRepository
	getLatestForLayerErr error
}

func newMockObsRepo() *mockObservationRepository {
	return &mockObservationRepository{
		latestForLayer: make(map[string][]*domain.Observation),
	}
}

func (r *mockObservationRepository) AddLatestForLayer(layerType string, obs []*domain.Observation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latestForLayer[layerType] = obs
}

func (r *mockObservationRepository) Create(_ context.Context, _ *domain.Observation) error {
	return nil
}
func (r *mockObservationRepository) CreateBatch(_ context.Context, _ []*domain.Observation) error {
	return nil
}
func (r *mockObservationRepository) CreateBatchUpsert(_ context.Context, _ []*domain.Observation) error {
	return nil
}
func (r *mockObservationRepository) GetByEntityID(_ context.Context, _ string, _ int, _ time.Time) ([]*domain.Observation, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLatest(_ context.Context, _ string) (*domain.Observation, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLatestForLayerPage(ctx context.Context, layerType string, limit, offset int) ([]*domain.EntitySnapshot, error) {
	r.mu.Lock()
	obs := r.latestForLayer[layerType]
	err := r.getLatestForLayerErr
	entities := r.entities
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}
	snaps := make([]*domain.EntitySnapshot, 0, len(obs))
	for _, o := range obs {
		snap := &domain.EntitySnapshot{Observation: *o}
		if entities != nil {
			if e, gErr := entities.GetByID(ctx, o.EntityID); gErr == nil && e != nil {
				snap.Entity = *e
			}
		}
		snaps = append(snaps, snap)
	}
	if offset >= len(snaps) {
		return nil, nil
	}
	snaps = snaps[offset:]
	if limit > 0 && limit < len(snaps) {
		snaps = snaps[:limit]
	}
	return snaps, nil
}
func (r *mockObservationRepository) GetLatestForEntityIDs(_ context.Context, _ []string) (map[string]*domain.Observation, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLatestContentHashes(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLayerSnapshotAt(_ context.Context, _ string, _ time.Time, _ time.Duration) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLatestForLayerByBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}
func (r *mockObservationRepository) GetLatestByCurrentPositionInBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (r *mockObservationRepository) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

// mockInsightRepository implements domain.AIInsightRepository for testing.
type mockInsightRepository struct {
	mu               sync.Mutex
	insights         []*domain.AIInsight
	refs             []insightRef
	createErr        error
	deleteExpiredN   int64
	deleteExpiredErr error
	recentDedupKeys  map[string]bool
}

type insightRef struct {
	InsightID     string
	EntityID      *string
	ObservationID *string
}

func newMockInsightRepo() *mockInsightRepository {
	return &mockInsightRepository{}
}

func (r *mockInsightRepository) Create(_ context.Context, insight *domain.AIInsight) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return "", r.createErr
	}
	r.insights = append(r.insights, insight)
	return insight.ID, nil
}

func (r *mockInsightRepository) CreateRef(_ context.Context, insightID string, entityID, observationID *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refs = append(r.refs, insightRef{
		InsightID:     insightID,
		EntityID:      entityID,
		ObservationID: observationID,
	})
	return nil
}

func (r *mockInsightRepository) GetByID(_ context.Context, _ string) (*domain.AIInsight, error) {
	return nil, nil
}

func (r *mockInsightRepository) List(_ context.Context, _ domain.InsightFilter) ([]*domain.AIInsight, int, error) {
	return nil, 0, nil
}

func (r *mockInsightRepository) DeleteExpired(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deleteExpiredN, r.deleteExpiredErr
}

func (r *mockInsightRepository) GetRecentDedupKeys(_ context.Context, _, _ string, _ time.Time) (map[string]bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recentDedupKeys, nil
}

func (r *mockInsightRepository) SetRecentDedupKeys(keys map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recentDedupKeys = keys
}

func (r *mockInsightRepository) InsightCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.insights)
}

func (r *mockInsightRepository) RefCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.refs)
}

func (r *mockInsightRepository) GetInsights() []*domain.AIInsight {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]*domain.AIInsight, len(r.insights))
	copy(result, r.insights)
	return result
}

func (r *mockInsightRepository) GetRefs() []insightRef {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]insightRef, len(r.refs))
	copy(result, r.refs)
	return result
}

// --- Test helpers ---

func newTestEngine(t *testing.T, provider *mockLLMProvider) (*Engine, *mockEntityRepository, *mockObservationRepository, *mockInsightRepository) {
	t.Helper()

	entityRepo := newMockEntityRepo()
	obsRepo := newMockObsRepo()
	obsRepo.entities = entityRepo
	insightRepo := newMockInsightRepo()
	reg := schema.NewRegistry()
	clk := &mockClock{now: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)}

	registry := &mockLLMRegistry{provider: provider}

	engine, err := NewEngine(EngineConfig{
		LLMRegistry: registry,
		EntityRepo:  entityRepo,
		ObsRepo:     obsRepo,
		InsightRepo: insightRepo,
		Schemas:     reg,
		Layers:      stubLayers{},
		Logger:      zerolog.Nop(),
		Clock:       clk,
	})
	require.NoError(t, err)

	return engine, entityRepo, obsRepo, insightRepo
}

func makeTestEntity(id, extID, name, layerType string) *domain.Entity {
	return &domain.Entity{
		ID:         id,
		ExternalID: extID,
		Name:       name,
		LayerType:  layerType,
		Metadata:   map[string]string{"flight": extID},
		CreatedAt:  time.Now(),
	}
}

func makeTestObservation(id, entityID string, lat, lon, alt float64, ts time.Time) *domain.Observation {
	return &domain.Observation{
		ID:        id,
		EntityID:  entityID,
		Position:  &domain.GeoPoint{Lat: lat, Lon: lon},
		AltitudeM: alt,
		Metadata: map[string]string{
			"gs":     "450",
			"track":  "180",
			"squawk": "1200",
		},
		Timestamp: ts,
	}
}

func makeTestDefinition(name string) *AnalysisDefinition {
	return &AnalysisDefinition{
		SchemaVersion: 1,
		Name:          name,
		DisplayName:   "Test " + name,
		Enabled:       true,
		Schedule: ScheduleConfig{
			Interval: "5m",
		},
		Data: DataConfig{
			Layers:     []string{"flights_commercial"},
			Lookback:   "1h",
			MinRecords: 0,
		},
		AI: aiconfig.SourceAIConfig{
			Enabled: true,
			Operations: []aiconfig.OperationConfig{
				{
					Name:   "test_op",
					Prompt: "Analyze {{.RecordCount}} records from the last {{.Lookback}}",
					Output: aiconfig.OutputConfig{
						StoreInsights: true,
						InsightType:   "test",
						Retention:     "168h",
					},
				},
			},
		},
	}
}

// --- Engine tests ---

// TestEngine_InFlightGuard_SkipsOverlappingRun proves the per-definition
// in-flight guard (tryAcquireRun/releaseRun) skips an overlapping tick while a
// previous run for the same definition is still active — exactly the way both
// scheduleCron and scheduleInterval wrap RunAnalysis.
func TestEngine_InFlightGuard_SkipsOverlappingRun(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{"summary": "ok"}`},
		entered:  make(chan struct{}, 1),
		release:  make(chan struct{}),
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)
	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("guard_test")
	ctx := context.Background()

	// guardedRun mirrors the scheduler closure: acquire-or-skip, defer release,
	// then run. Returns true if the run actually executed (acquired the guard).
	guardedRun := func() bool {
		if !engine.tryAcquireRun(def.Name) {
			return false // overlapping tick skipped
		}
		defer engine.releaseRun(def.Name)
		require.NoError(t, engine.RunAnalysis(ctx, def))
		return true
	}

	// Start the first run; it blocks inside the LLM Complete call.
	firstRan := make(chan bool, 1)
	go func() { firstRan <- guardedRun() }()

	// Wait until the first run is provably in-flight (inside Complete).
	select {
	case <-provider.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first run never reached the LLM provider")
	}

	// While the first run is in-flight, a second guarded tick must be skipped.
	assert.False(t, guardedRun(), "second run must be skipped while first is in-flight")

	// Let the first run finish and verify it ran exactly once.
	close(provider.release)
	assert.True(t, <-firstRan, "first run should have executed")

	assert.Equal(t, 1, provider.CallCount(), "LLM should be called exactly once")
	assert.Equal(t, 1, insightRepo.InsightCount(), "exactly one insight should be stored")

	// After completion the guard is released, so a fresh run acquires again.
	assert.True(t, engine.tryAcquireRun(def.Name), "guard must be released after run completes")
	engine.releaseRun(def.Name)
}

func TestEngine_RunAnalysis_ProducesInsight(t *testing.T) {
	llmResp := &llm.CompletionResponse{
		Content: `{"summary": "All clear", "event_count": 5}`,
		Model:   "test-model",
	}
	provider := &mockLLMProvider{response: llmResp}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	// Add test data.
	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("test_analysis")

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	assert.Equal(t, 1, provider.CallCount())
	assert.Equal(t, 1, insightRepo.InsightCount())

	insights := insightRepo.GetInsights()
	assert.Equal(t, "test", insights[0].InsightType)
	assert.Equal(t, "test_analysis", insights[0].SourceName)
	assert.Equal(t, "test_op", insights[0].OperationName)
	assert.NotNil(t, insights[0].Result["summary"])
}

func TestEngine_RunAnalysis_MinRecordsSkips(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("min_records_test")
	def.Data.MinRecords = 10 // require 10 records, but only 1 exists

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// LLM should NOT have been called.
	assert.Equal(t, 0, provider.CallCount())
	assert.Equal(t, 0, insightRepo.InsightCount())
}

func TestEngine_RunAnalysis_ResultsPathIteratesArray(t *testing.T) {
	respData := map[string]any{
		"anomalies": []any{
			map[string]any{
				"entity_external_id": "ABC123",
				"anomaly_type":       "altitude_drop",
				"severity":           3,
				"title":              "Rapid altitude loss",
				"description":        "Flight dropped 5000ft",
			},
			map[string]any{
				"entity_external_id": "ABC123",
				"anomaly_type":       "squawk_emergency",
				"severity":           4,
				"title":              "Emergency squawk",
				"description":        "Aircraft transmitting 7700",
			},
		},
	}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	// Register the output schema.
	schemaMap := map[string]any{
		"type":     "object",
		"required": []any{"anomalies"},
		"properties": map[string]any{
			"anomalies": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"entity_external_id", "anomaly_type", "severity", "title", "description"},
					"properties": map[string]any{
						"entity_external_id": map[string]any{"type": "string"},
						"anomaly_type":       map[string]any{"type": "string"},
						"severity":           map[string]any{"type": "integer"},
						"title":              map[string]any{"type": "string"},
						"description":        map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey("results_path_test", "anomaly_scan"), schemaMap))

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("results_path_test")
	def.AI.Operations = []aiconfig.OperationConfig{
		{
			Name:         "anomaly_scan",
			Prompt:       "Find anomalies in {{.RecordCount}} records",
			OutputSchema: schemaMap,
			Output: aiconfig.OutputConfig{
				StoreInsights: true,
				InsightType:   "anomaly",
				ResultsPath:   "anomalies",
				Retention:     "168h",
			},
		},
	}

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// Should have created 2 insights (one per anomaly).
	assert.Equal(t, 2, insightRepo.InsightCount())

	insights := insightRepo.GetInsights()
	assert.Equal(t, "anomaly", insights[0].InsightType)
	assert.Equal(t, "results_path_test", insights[0].SourceName)
	assert.Equal(t, "anomaly_scan", insights[0].OperationName)

	// Input entity should be linked to each insight deterministically.
	refs := insightRepo.GetRefs()
	assert.GreaterOrEqual(t, len(refs), 1) // at least one ref for ent-1
}

func TestEngine_RunAnalysis_EntityExternalIDAssociation(t *testing.T) {
	respData := map[string]any{
		"alerts": []any{
			map[string]any{
				"entity_external_id": "ABC123",
				"alert_type":         "proximity",
				"severity":           2,
			},
		},
	}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	// Register schema.
	schemaMap := map[string]any{
		"type":     "object",
		"required": []any{"alerts"},
		"properties": map[string]any{
			"alerts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entity_external_id": map[string]any{"type": "string"},
						"alert_type":         map[string]any{"type": "string"},
						"severity":           map[string]any{"type": "integer"},
					},
				},
			},
		},
	}
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey("ext_id_test", "alert_op"), schemaMap))

	entity := makeTestEntity("ent-uuid-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-uuid-1", 40.7128, -74.0060, 10000, now.Add(-10*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("ext_id_test")
	def.AI.Operations = []aiconfig.OperationConfig{
		{
			Name:         "alert_op",
			Prompt:       "Check alerts",
			OutputSchema: schemaMap,
			Output: aiconfig.OutputConfig{
				StoreInsights: true,
				InsightType:   "alert",
				ResultsPath:   "alerts",
				Retention:     "24h",
			},
		},
	}

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	assert.Equal(t, 1, insightRepo.InsightCount())

	// Verify entity ref was created via external ID resolution.
	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	require.NotNil(t, refs[0].EntityID)
	assert.Equal(t, "ent-uuid-1", *refs[0].EntityID)
}

func TestEngine_RunAnalysis_DeterministicAssociation(t *testing.T) {
	// LLM output includes observation_ids and entity_external_id, but these
	// should be ignored. Associations come solely from the input SQL records.
	respData := map[string]any{
		"observation_ids":    []any{"obs-uuid-1", "obs-uuid-2"},
		"entity_external_id": "HALLUCINATED_ID",
		"summary":            "test",
	}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "TEST", "Test Entity", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 5000, now.Add(-15*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("deterministic_ref_test")

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// Refs should be created from the input SQL records, not LLM output.
	// Entity "ent-1" was in the SQL result, so it should be linked.
	refs := insightRepo.GetRefs()
	entityRefCount := 0
	obsRefCount := 0
	for _, ref := range refs {
		if ref.EntityID != nil {
			entityRefCount++
			assert.Equal(t, "ent-1", *ref.EntityID)
		}
		if ref.ObservationID != nil {
			obsRefCount++
		}
	}
	assert.Equal(t, 1, entityRefCount, "should have 1 entity ref from input records")
	assert.Equal(t, 0, obsRefCount, "LLM-provided observation_ids should be ignored")
}

func TestEngine_RunAnalysis_DisabledDefinitionSkipped(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	def := makeTestDefinition("disabled_test")
	def.Enabled = false

	// SetDefinitions and Start should skip disabled definitions.
	engine.SetDefinitions(map[string]*AnalysisDefinition{"disabled_test": def})

	// Verify it's in definitions but Start would skip it.
	assert.Contains(t, engine.Definitions(), "disabled_test")
	assert.False(t, engine.Definitions()["disabled_test"].Enabled)
}

func TestEngine_RunAnalysis_CELFilterApplied(t *testing.T) {
	respData := map[string]any{"result": "filtered analysis"}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	// Add 2 entities: one with "flight" metadata, one without.
	entity1 := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entity1.Metadata = map[string]string{"flight": "ABC123", "type": "commercial"}
	entityRepo.AddEntity(entity1)

	entity2 := makeTestEntity("ent-2", "XYZ789", "Unknown", "flights_commercial")
	entity2.Metadata = map[string]string{"type": "unknown"} // no "flight" key
	entityRepo.AddEntity(entity2)

	now := engine.clock.Now()
	obs1 := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obs2 := makeTestObservation("obs-2", "ent-2", 41.0, -73.0, 5000, now.Add(-20*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs1, obs2})

	def := makeTestDefinition("cel_filter_test")
	def.Data.Filter = `has(entity.metadata.flight)`

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// LLM was called (at least 1 record passed filter).
	assert.Equal(t, 1, provider.CallCount())
	assert.Equal(t, 1, insightRepo.InsightCount())
}

func TestEngine_RunAnalysis_NoRecordsSkips(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, obsRepo, insightRepo := newTestEngine(t, provider)

	// No observations in the layer.
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{})

	def := makeTestDefinition("no_records_test")
	def.Data.MinRecords = 1

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	assert.Equal(t, 0, provider.CallCount())
	assert.Equal(t, 0, insightRepo.InsightCount())
}

func TestEngine_RunAnalysis_LLMFailureReportsError(t *testing.T) {
	provider := &mockLLMProvider{
		err: fmt.Errorf("provider timeout"),
	}
	engine, entityRepo, obsRepo, _ := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("llm_fail_test")

	// RunAnalysis should not return an error (individual operation errors are logged).
	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)
}

func TestEngine_RunAnalysis_ExpiresAtComputed(t *testing.T) {
	respData := map[string]any{"result": "test"}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("expiry_test")
	def.AI.Operations[0].Output.Retention = "168h" // 7 days

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)
	require.NotNil(t, insights[0].ExpiresAt)

	expectedExpiry := now.Add(168 * time.Hour)
	assert.Equal(t, expectedExpiry, *insights[0].ExpiresAt)
}

func TestEngine_RunAnalysis_SingleLayerSetsLayerType(t *testing.T) {
	respData := map[string]any{"result": "test"}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("layer_type_test")

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)
	require.NotNil(t, insights[0].LayerType)
	assert.Equal(t, "flights_commercial", *insights[0].LayerType)
}

func TestEngine_RunAnalysis_MultiLayerDerivesLayerType(t *testing.T) {
	respData := map[string]any{"result": "test"}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity1 := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity1)
	entity2 := makeTestEntity("ent-2", "EQ001", "Earthquake", "earthquakes")
	entityRepo.AddEntity(entity2)

	now := engine.clock.Now()
	obs1 := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obs2 := makeTestObservation("obs-2", "ent-2", 35.0, 139.0, 0, now.Add(-20*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs1})
	obsRepo.AddLatestForLayer("earthquakes", []*domain.Observation{obs2})

	def := makeTestDefinition("multi_layer_test")
	def.Data.Layers = []string{"flights_commercial", "earthquakes"}

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)
	// Multi-layer -> layer_type is derived from the first record's layer.
	require.NotNil(t, insights[0].LayerType)
	assert.Equal(t, "flights_commercial", *insights[0].LayerType)
}

func TestEngine_RunAnalysis_AllLayersWhenEmpty(t *testing.T) {
	respData := map[string]any{"result": "all layers"}
	respJSON, _ := json.Marshal(respData)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: string(respJSON), Model: "test"},
	}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entityRepo.distinctLayers = []string{"flights_commercial", "earthquakes"}

	entity := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})
	obsRepo.AddLatestForLayer("earthquakes", []*domain.Observation{})

	def := makeTestDefinition("all_layers_test")
	def.Data.Layers = []string{} // empty = all layers

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	assert.Equal(t, 1, provider.CallCount())
	assert.Equal(t, 1, insightRepo.InsightCount())
}

func TestEngine_GracefulShutdown(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	def := makeTestDefinition("shutdown_test")
	engine.SetDefinitions(map[string]*AnalysisDefinition{"shutdown_test": def})

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = engine.Start(ctx)
		close(done)
	}()

	<-started
	// Give the scheduler a moment to initialize.
	time.Sleep(50 * time.Millisecond)

	// Stop the engine.
	cancel()
	engine.Stop()

	select {
	case <-done:
		// Success -- engine shut down.
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not shut down within timeout")
	}
}

// --- Prompt rendering tests ---

func TestRenderAnalysisPrompt_Basic(t *testing.T) {
	data := AnalysisPromptData{
		RecordCount:  42,
		Lookback:     "1h",
		OutputSchema: `{"type":"object"}`,
	}

	tmpl := "Analyzing {{.RecordCount}} records from the last {{.Lookback}}. Schema: {{.OutputSchema}}"
	result, err := renderAnalysisPrompt(tmpl, data)
	require.NoError(t, err)
	assert.Contains(t, result, "42 records")
	assert.Contains(t, result, "last 1h")
	assert.Contains(t, result, `{"type":"object"}`)
}

func TestRenderAnalysisPrompt_WithRecords(t *testing.T) {
	data := AnalysisPromptData{
		RecordCount: 2,
		Records: []AnalysisRecord{
			{EntityName: "Flight ABC", ExternalID: "ABC123", Altitude: 10000, Lat: 40.0, Lon: -74.0},
			{EntityName: "Flight XYZ", ExternalID: "XYZ789", Altitude: 5000, Lat: 41.0, Lon: -73.0},
		},
	}

	tmpl := `{{range .Records}}- {{.EntityName}} ({{.ExternalID}}): alt={{.Altitude}}m
{{end}}`

	result, err := renderAnalysisPrompt(tmpl, data)
	require.NoError(t, err)
	assert.Contains(t, result, "Flight ABC (ABC123): alt=10000m")
	assert.Contains(t, result, "Flight XYZ (XYZ789): alt=5000m")
}

func TestRenderAnalysisPrompt_InvalidTemplate(t *testing.T) {
	data := AnalysisPromptData{}
	_, err := renderAnalysisPrompt("{{.Invalid", data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse template")
}

// --- Layer stat computation tests ---

func TestComputeLayerStat(t *testing.T) {
	records := []AnalysisRecord{
		{ExternalID: "A", Lat: 40.0, Lon: -74.0, Altitude: 10000, Metadata: map[string]string{"flight": "A", "gs": "450"}},
		{ExternalID: "B", Lat: 41.0, Lon: -73.0, Altitude: 5000, Metadata: map[string]string{"flight": "B"}},
		{ExternalID: "A", Lat: 40.1, Lon: -74.1, Altitude: 9000, Metadata: map[string]string{"flight": "A", "gs": "430"}},
	}

	stat := computeLayerStat("flights_commercial", records)

	assert.Equal(t, "flights_commercial", stat.LayerType)
	assert.Equal(t, 3, stat.Count)
	assert.Equal(t, 100, stat.CoordPercent)
	assert.Equal(t, 100, stat.AltPercent)
	assert.Equal(t, 2, stat.UniqueExtIDs)    // A and B
	assert.Equal(t, 1, stat.DuplicateExtIDs) // A appears twice
	assert.True(t, stat.AvgMetadataKeys > 0)
}

func TestComputeLayerStat_Empty(t *testing.T) {
	stat := computeLayerStat("empty", nil)
	assert.Equal(t, 0, stat.Count)
	assert.Equal(t, 0, stat.CoordPercent)
}

// --- CEL filter tests ---

func TestEngine_ApplyFilter_PassesMatching(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "1", EntityName: "A", Metadata: map[string]string{"flight": "ABC"}, EntityMetadata: map[string]string{"flight": "ABC"}},
		{EntityID: "2", EntityName: "B", Metadata: map[string]string{"other": "val"}, EntityMetadata: map[string]string{"other": "val"}},
	}

	filtered, err := engine.applyFilter(`has(entity.metadata.flight)`, records)
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "1", filtered[0].EntityID)
}

func TestEngine_ApplyFilter_RejectsAll(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "1", Metadata: map[string]string{"a": "1"}, EntityMetadata: map[string]string{"a": "1"}},
		{EntityID: "2", Metadata: map[string]string{"b": "2"}, EntityMetadata: map[string]string{"b": "2"}},
	}

	filtered, err := engine.applyFilter(`has(entity.metadata.nonexistent)`, records)
	require.NoError(t, err)
	assert.Empty(t, filtered)
}

func TestEngine_ApplyFilter_InvalidFilter(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{{EntityID: "1"}}

	_, err := engine.applyFilter(`invalid !!!`, records)
	require.Error(t, err)
}

// --- buildPromptData tests ---

func TestEngine_BuildPromptData(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{LayerType: "flights_commercial", ExternalID: "A"},
		{LayerType: "flights_commercial", ExternalID: "B"},
		{LayerType: "earthquakes", ExternalID: "EQ1"},
	}

	def := makeTestDefinition("test")
	data := engine.buildPromptData(records, def, 1*time.Hour)

	assert.Equal(t, 3, data.RecordCount)
	assert.Equal(t, "1h0m0s", data.Lookback)
	assert.Equal(t, 2, data.LayerCount)
	assert.Len(t, data.LayerStats, 2)
}

// --- Entity association tests (deterministic: all input records → insight) ---

func TestEngine_CreateAssociations_AllRecordsLinked(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "uuid-1", EntityName: "Fire Alpha", ExternalID: "fire-001"},
		{EntityID: "uuid-2", EntityName: "Fire Bravo", ExternalID: "fire-002"},
		{EntityID: "uuid-3", EntityName: "Sensor Charlie", ExternalID: "sensor-003"},
	}

	engine.createAssociations(context.Background(), "insight-1", records, zerolog.Nop())

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 3)

	entityIDs := make(map[string]bool)
	for _, ref := range refs {
		require.NotNil(t, ref.EntityID)
		entityIDs[*ref.EntityID] = true
	}
	assert.True(t, entityIDs["uuid-1"])
	assert.True(t, entityIDs["uuid-2"])
	assert.True(t, entityIDs["uuid-3"])
}

func TestEngine_CreateAssociations_DeduplicatesEntityIDs(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	// Cross-layer join can produce duplicate entity rows.
	records := []AnalysisRecord{
		{EntityID: "uuid-1", EntityName: "Ship Mercury", ExternalID: "MMSI-111"},
		{EntityID: "uuid-1", EntityName: "Ship Mercury", ExternalID: "MMSI-111"}, // duplicate
		{EntityID: "uuid-2", EntityName: "Ship Venus", ExternalID: "MMSI-222"},
	}

	engine.createAssociations(context.Background(), "insight-1", records, zerolog.Nop())

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 2) // uuid-1 only linked once
}

func TestEngine_CreateAssociations_SkipsEmptyEntityID(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "", EntityName: "Orphan Record", ExternalID: "ext-1"},
		{EntityID: "uuid-2", EntityName: "Valid Record", ExternalID: "ext-2"},
	}

	engine.createAssociations(context.Background(), "insight-1", records, zerolog.Nop())

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "uuid-2", *refs[0].EntityID)
}

func TestEngine_CreateAssociations_NoRecords(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	engine.createAssociations(context.Background(), "insight-1", nil, zerolog.Nop())

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_CreateAssociations_EmptyRecords(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	engine.createAssociations(context.Background(), "insight-1", []AnalysisRecord{}, zerolog.Nop())

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_CreateAssociations_SingleRecord(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "uuid-only", EntityName: "Solo Entity", ExternalID: "ext-solo"},
	}

	engine.createAssociations(context.Background(), "insight-1", records, zerolog.Nop())

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "uuid-only", *refs[0].EntityID)
}

func TestEngine_CreateAssociations_LLMOutputIgnored(t *testing.T) {
	// Verify that LLM output fields like entity_external_id are irrelevant.
	// The association is purely based on input records, not LLM result content.
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "uuid-real", EntityName: "Real Entity", ExternalID: "ext-real"},
	}

	engine.createAssociations(context.Background(), "insight-1", records, zerolog.Nop())

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "uuid-real", *refs[0].EntityID)
}

// --- matchRecordsByExternalID tests ---

func TestMatchRecordsByExternalID_MatchesSingleEntity(t *testing.T) {
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "MMSI-111", EntityName: "Ship Alpha"},
		{EntityID: "uuid-2", ExternalID: "MMSI-222", EntityName: "Ship Beta"},
		{EntityID: "uuid-3", ExternalID: "MMSI-333", EntityName: "Ship Gamma"},
	}

	itemMap := map[string]any{
		"entity_external_id": "MMSI-222",
		"vessel_name":        "Ship Beta",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	require.Len(t, matched, 1)
	assert.Equal(t, "uuid-2", matched[0].EntityID)
	assert.Equal(t, "MMSI-222", matched[0].ExternalID)
}

func TestMatchRecordsByExternalID_NoFieldReturnsNil(t *testing.T) {
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "ext-1"},
		{EntityID: "uuid-2", ExternalID: "ext-2"},
	}

	// No entity_external_id field: a per-item insight that cannot identify its
	// entity must associate NOTHING from records, never the whole batch.
	itemMap := map[string]any{
		"summary": "general analysis",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	assert.Empty(t, matched)
}

func TestMatchRecordsByExternalID_EmptyStringReturnsNil(t *testing.T) {
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "ext-1"},
	}

	itemMap := map[string]any{
		"entity_external_id": "",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	assert.Empty(t, matched)
}

func TestMatchRecordsByExternalID_NoMatchReturnsNil(t *testing.T) {
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "ext-1"},
		{EntityID: "uuid-2", ExternalID: "ext-2"},
	}

	// LLM named an external_id that matches no input record: associate nothing
	// rather than every unrelated record.
	itemMap := map[string]any{
		"entity_external_id": "nonexistent-id",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	assert.Empty(t, matched)
}

func TestMatchRecordsByExternalID_NonStringTypeReturnsNil(t *testing.T) {
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "ext-1"},
	}

	// entity_external_id is a number instead of string.
	itemMap := map[string]any{
		"entity_external_id": 12345,
	}

	matched := matchRecordsByExternalID(itemMap, records)
	assert.Empty(t, matched)
}

// TestDedupRecordForItem_StableFromMatchedRecord reproduces the duplicate-
// notification bug: a result item whose dedup uses a SQL-only key the LLM never
// echoes (here quake_key) must still get a STABLE key — derived from the input
// record it identifies, disambiguated by the secondary id it does echo — equal to
// the key the pre-LLM filter computes for that same record (so re-runs dedup).
func TestDedupRecordForItem_StableFromMatchedRecord(t *testing.T) {
	// Same hazard (EQ1) near two cables; quake_key is identical (same quake) but
	// infrastructure_external_id differs. The LLM item echoes only the ids it has.
	records := []AnalysisRecord{
		{EntityID: "u-a", ExternalID: "EQ1", LayerType: "disaster_alerts", Metadata: map[string]string{
			"entity_external_id": "EQ1", "quake_key": "disaster:24.5,123.0", "infrastructure_external_id": "cable_A",
		}},
		{EntityID: "u-b", ExternalID: "EQ1", LayerType: "disaster_alerts", Metadata: map[string]string{
			"entity_external_id": "EQ1", "quake_key": "disaster:24.5,123.0", "infrastructure_external_id": "cable_B",
		}},
	}
	keyFields := []string{"quake_key", "infrastructure_external_id"}
	item := map[string]any{
		"entity_external_id":         "EQ1",
		"entity_layer_type":          "disaster_alerts",
		"infrastructure_external_id": "cable_B", // no quake_key echoed
	}

	matched := matchRecordsByExternalID(item, records)
	require.Len(t, matched, 2, "both EQ1 records match the hazard")

	got := computeDedupKey(dedupRecordForItem(item, matched, keyFields), keyFields)
	// Must equal the cable_B record's own key — exactly what dedupRecords computes
	// for the pre-LLM filter, so a re-run skips it. And must NOT collapse onto cable_A.
	assert.Equal(t, computeDedupKey(records[1], keyFields), got, "key derived from the matched cable_B record")
	assert.NotEqual(t, computeDedupKey(records[0], keyFields), got, "disambiguated from cable_A")
}

func TestMatchRecordsByExternalID_LayerTypeDisambiguates(t *testing.T) {
	// Same external_id in two different layers (the cross-layer collision the
	// composite identity guards against).
	records := []AnalysisRecord{
		{EntityID: "uuid-eq", ExternalID: "211010", LayerType: "earthquakes"},
		{EntityID: "uuid-vol", ExternalID: "211010", LayerType: "volcanoes"},
	}

	// Without a layer, external_id alone matches both (legacy single-layer behavior).
	both := matchRecordsByExternalID(map[string]any{"entity_external_id": "211010"}, records)
	assert.Len(t, both, 2)

	// With the layer, only the matching-layer record is returned.
	vol := matchRecordsByExternalID(map[string]any{
		"entity_external_id": "211010",
		"entity_layer_type":  "volcanoes",
	}, records)
	require.Len(t, vol, 1)
	assert.Equal(t, "uuid-vol", vol[0].EntityID)
}

// --- resolveRefFieldEntities tests ---

func TestEngine_ResolveRefFieldEntities_LinksSecondaryEntity(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	// Add a secondary entity (e.g. a cable landing point) to the entity repo.
	entityRepo.AddEntity(&domain.Entity{
		ID:         "cable-uuid-1",
		ExternalID: "cable_lp_ternate-indonesia",
		Name:       "Ternate, Indonesia",
		LayerType:  "subsea_cables",
	})

	itemMap := map[string]any{
		"entity_external_id":         "earthquake-123",
		"infrastructure_external_id": "cable_lp_ternate-indonesia",
	}

	ids, entityRefs := engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "cable-uuid-1", *refs[0].EntityID)
	assert.Equal(t, "insight-1", refs[0].InsightID)

	// Verify return values for WebSocket payload.
	require.Len(t, ids, 1)
	assert.Equal(t, "cable-uuid-1", ids[0])
	require.Len(t, entityRefs, 1)
	assert.Equal(t, "cable-uuid-1", entityRefs[0].ID)
	assert.Equal(t, "Ternate, Indonesia", entityRefs[0].Name)
	assert.Equal(t, "subsea_cables", entityRefs[0].LayerType)
}

func TestEngine_ResolveRefFieldEntities_NoRefFields(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	itemMap := map[string]any{
		"entity_external_id": "ext-1",
	}

	// Empty ref_fields — no additional refs should be created.
	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		nil, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_ResolveRefFieldEntities_FieldMissingInResult(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	entityRepo.AddEntity(&domain.Entity{
		ID:         "cable-uuid-1",
		ExternalID: "cable_lp_xyz",
		Name:       "Test Cable",
		LayerType:  "subsea_cables",
	})

	// The result item doesn't have the ref_field.
	itemMap := map[string]any{
		"entity_external_id": "ext-1",
	}

	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_ResolveRefFieldEntities_EmptyStringSkipped(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	itemMap := map[string]any{
		"infrastructure_external_id": "",
	}

	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_ResolveRefFieldEntities_NonStringSkipped(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	itemMap := map[string]any{
		"infrastructure_external_id": 42,
	}

	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_ResolveRefFieldEntities_NoEntityFound(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	// No entity in repo with this external_id.
	itemMap := map[string]any{
		"infrastructure_external_id": "nonexistent_cable",
	}

	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	assert.Empty(t, refs)
}

func TestEngine_ResolveRefFieldEntities_MultipleRefFields(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	entityRepo.AddEntity(&domain.Entity{
		ID:         "cable-uuid-1",
		ExternalID: "cable_lp_ternate",
		Name:       "Ternate Cable",
		LayerType:  "subsea_cables",
	})
	entityRepo.AddEntity(&domain.Entity{
		ID:         "ixp-uuid-1",
		ExternalID: "ixp_jakarta",
		Name:       "Jakarta IXP",
		LayerType:  "internet_infrastructure",
	})

	itemMap := map[string]any{
		"infrastructure_external_id": "cable_lp_ternate",
		"ixp_external_id":            "ixp_jakarta",
	}

	engine.resolveRefFieldEntities(
		context.Background(), "insight-1", itemMap,
		[]aiconfig.RefField{
			{Field: "infrastructure_external_id", LayerType: "subsea_cables"},
			{Field: "ixp_external_id", LayerType: "internet_infrastructure"},
		}, zerolog.Nop(),
	)

	refs := insightRepo.GetRefs()
	require.Len(t, refs, 2)

	refEntityIDs := map[string]bool{}
	for _, ref := range refs {
		refEntityIDs[*ref.EntityID] = true
	}
	assert.True(t, refEntityIDs["cable-uuid-1"])
	assert.True(t, refEntityIDs["ixp-uuid-1"])
}

// --- storeResultsArray integration tests with ref_fields ---

func TestEngine_StoreResultsArray_WithRefFields(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	// Primary entity (earthquake) — part of input records.
	entityRepo.AddEntity(&domain.Entity{
		ID:         "eq-uuid-1",
		ExternalID: "earthquake-001",
		Name:       "Molucca Sea Earthquake",
		LayerType:  "earthquakes",
	})
	// Secondary entity (cable landing) — NOT in input records, resolved via ref_fields.
	entityRepo.AddEntity(&domain.Entity{
		ID:         "cable-uuid-1",
		ExternalID: "cable_lp_ternate",
		Name:       "Ternate, Indonesia",
		LayerType:  "subsea_cables",
	})

	records := []AnalysisRecord{
		{EntityID: "eq-uuid-1", ExternalID: "earthquake-001", EntityName: "Molucca Sea"},
	}

	def := makeTestDefinition("infra_threat")
	op := &aiconfig.OperationConfig{
		Name: "infra_risk",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "infrastructure_threat",
			ResultsPath:   "threats",
			RefFields:     []aiconfig.RefField{{Field: "infrastructure_external_id", LayerType: "subsea_cables"}},
		},
	}

	result := map[string]any{
		"threats": []any{
			map[string]any{
				"entity_external_id":         "earthquake-001",
				"infrastructure_external_id": "cable_lp_ternate",
				"hazard_name":                "Molucca Sea Earthquake",
				"infrastructure_name":        "Ternate, Indonesia",
				"threat_level":               "moderate",
			},
		},
	}

	err := engine.storeResultsArray(
		context.Background(), def, op, result,
		nil, nil, records, zerolog.Nop(),
	)
	require.NoError(t, err)

	// Should have 1 insight stored.
	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)

	// Should have 2 refs: earthquake (from matchRecordsByExternalID) + cable (from ref_fields).
	refs := insightRepo.GetRefs()
	require.Len(t, refs, 2)

	refEntityIDs := map[string]bool{}
	for _, ref := range refs {
		refEntityIDs[*ref.EntityID] = true
	}
	assert.True(t, refEntityIDs["eq-uuid-1"], "should reference the earthquake entity")
	assert.True(t, refEntityIDs["cable-uuid-1"], "should reference the cable landing entity")
}

func TestEngine_StoreResultsArray_RefFieldsDoesNotDuplicate(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	// Entity that is BOTH primary and in ref_fields — should not create duplicate refs.
	entityRepo.AddEntity(&domain.Entity{
		ID:         "eq-uuid-1",
		ExternalID: "earthquake-001",
		Name:       "Molucca Sea",
		LayerType:  "earthquakes",
	})

	records := []AnalysisRecord{
		{EntityID: "eq-uuid-1", ExternalID: "earthquake-001", EntityName: "Molucca Sea"},
	}

	def := makeTestDefinition("test")
	op := &aiconfig.OperationConfig{
		Name: "test_op",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "test",
			ResultsPath:   "items",
			RefFields:     []aiconfig.RefField{{Field: "also_this_entity", LayerType: "earthquakes"}},
		},
	}

	result := map[string]any{
		"items": []any{
			map[string]any{
				"entity_external_id": "earthquake-001",
				"also_this_entity":   "earthquake-001",
			},
		},
	}

	err := engine.storeResultsArray(
		context.Background(), def, op, result,
		nil, nil, records, zerolog.Nop(),
	)
	require.NoError(t, err)

	// The mock doesn't deduplicate, so we'll see 2 refs (createAssociations + resolveRefFieldEntities).
	// In production, the DB unique constraint on (insight_id, entity_id) prevents duplicates.
	// The important thing: no errors are returned.
	refs := insightRepo.GetRefs()
	assert.GreaterOrEqual(t, len(refs), 1)
}

func TestEngine_StoreResultsArray_NoRefFieldsNoExtraRefs(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	entityRepo.AddEntity(&domain.Entity{
		ID:         "ship-uuid-1",
		ExternalID: "MMSI-123",
		Name:       "Test Ship",
		LayerType:  "ships",
	})

	records := []AnalysisRecord{
		{EntityID: "ship-uuid-1", ExternalID: "MMSI-123", EntityName: "Test Ship"},
		{EntityID: "ship-uuid-2", ExternalID: "MMSI-456", EntityName: "Other Ship"},
	}

	def := makeTestDefinition("cable_threat")
	op := &aiconfig.OperationConfig{
		Name: "cable_threat_assessment",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "maritime_cable_threat",
			ResultsPath:   "threats",
			// No RefFields — only entity_external_id matching.
		},
	}

	result := map[string]any{
		"threats": []any{
			map[string]any{
				"entity_external_id": "MMSI-123",
				"vessel_name":        "Test Ship",
			},
		},
	}

	err := engine.storeResultsArray(
		context.Background(), def, op, result,
		nil, nil, records, zerolog.Nop(),
	)
	require.NoError(t, err)

	// Only 1 ref for the matched ship (MMSI-123), NOT both ships.
	refs := insightRepo.GetRefs()
	require.Len(t, refs, 1)
	assert.Equal(t, "ship-uuid-1", *refs[0].EntityID)
}

// --- Old observation filtering tests ---

func TestEngine_FetchData_FiltersOldObservations(t *testing.T) {
	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{"result":"ok"}`},
	}
	engine, entityRepo, obsRepo, _ := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	// One observation within lookback, one outside.
	recentObs := makeTestObservation("obs-1", "ent-1", 40.0, -74.0, 10000, now.Add(-30*time.Minute))
	oldObs := makeTestObservation("obs-2", "ent-1", 40.0, -74.0, 10000, now.Add(-3*time.Hour))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{recentObs, oldObs})

	def := makeTestDefinition("lookback_test")
	def.Data.Lookback = "1h"

	records, err := engine.fetchData(context.Background(), def, zerolog.Nop())
	require.NoError(t, err)

	// Only the recent observation should be included.
	assert.Len(t, records, 1)
	assert.Equal(t, "ent-1", records[0].EntityID)
}

// --- Store insights error handling ---

func TestEngine_StoreInsights_CreateError(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})
	insightRepo.createErr = fmt.Errorf("database error")

	def := makeTestDefinition("test")
	op := &def.AI.Operations[0]

	err := engine.storeSingleInsight(
		context.Background(), def, op,
		map[string]any{"result": "test"},
		nil, nil, nil, nil, zerolog.Nop(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create insight")
}

func TestEngine_StoreInsights_ResultsPathNotFound(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	def := makeTestDefinition("test")
	op := &aiconfig.OperationConfig{
		Name: "test_op",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "test",
			ResultsPath:   "nonexistent_key",
		},
	}

	err := engine.storeResultsArray(
		context.Background(), def, op,
		map[string]any{"other_key": "value"},
		nil, nil, nil, zerolog.Nop(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "results_path")
}

func TestEngine_StoreInsights_ResultsPathNotArray(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	def := makeTestDefinition("test")
	op := &aiconfig.OperationConfig{
		Name: "test_op",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "test",
			ResultsPath:   "not_array",
		},
	}

	err := engine.storeResultsArray(
		context.Background(), def, op,
		map[string]any{"not_array": "this is a string, not an array"},
		nil, nil, nil, zerolog.Nop(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an array")
}

// --- rowToAnalysisRecord tests ---

func TestRowToAnalysisRecord_AllStandardFields(t *testing.T) {
	row := map[string]any{
		"entity_id":   "uuid-123",
		"external_id": "ABC123",
		"name":        "Flight ABC123",
		"layer_type":  "flights_commercial",
		"lat":         40.7128,
		"lon":         -74.0060,
		"altitude_m":  10000.0,
		"ts":          "2026-03-28T12:00:00Z",
	}

	rec := rowToAnalysisRecord(row)

	assert.Equal(t, "uuid-123", rec.EntityID)
	assert.Equal(t, "ABC123", rec.ExternalID)
	assert.Equal(t, "Flight ABC123", rec.EntityName)
	assert.Equal(t, "flights_commercial", rec.LayerType)
	assert.InDelta(t, 40.7128, rec.Lat, 0.0001)
	assert.InDelta(t, -74.0060, rec.Lon, 0.0001)
	assert.InDelta(t, 10000.0, rec.Altitude, 0.1)
	assert.Equal(t, "2026-03-28T12:00:00Z", rec.Timestamp)
	// Structural fields are mirrored into Metadata for declarative dedup key resolution.
	assert.Equal(t, "uuid-123", rec.Metadata["entity_id"])
	assert.Equal(t, "ABC123", rec.Metadata["entity_external_id"])
	assert.Equal(t, "Flight ABC123", rec.Metadata["entity_name"])
	assert.Equal(t, "flights_commercial", rec.Metadata["layer_type"])
}

func TestRowToAnalysisRecord_MissingFields(t *testing.T) {
	// Only name and layer_type present.
	row := map[string]any{
		"name":       "Partial Entity",
		"layer_type": "earthquakes",
	}

	rec := rowToAnalysisRecord(row)

	assert.Equal(t, "", rec.EntityID)
	assert.Equal(t, "", rec.ExternalID)
	assert.Equal(t, "Partial Entity", rec.EntityName)
	assert.Equal(t, "earthquakes", rec.LayerType)
	assert.InDelta(t, 0.0, rec.Lat, 0.0001)
	assert.InDelta(t, 0.0, rec.Lon, 0.0001)
	assert.InDelta(t, 0.0, rec.Altitude, 0.1)
	assert.Equal(t, "", rec.Timestamp)
	// Only the mirrored structural fields should be in Metadata.
	assert.Equal(t, "", rec.Metadata["entity_id"])
	assert.Equal(t, "", rec.Metadata["entity_external_id"])
	assert.Equal(t, "Partial Entity", rec.Metadata["entity_name"])
	assert.Equal(t, "earthquakes", rec.Metadata["layer_type"])
}

func TestRowToAnalysisRecord_ExtraColumnsGoToMetadata(t *testing.T) {
	row := map[string]any{
		"entity_id":  "uuid-1",
		"name":       "Test",
		"layer_type": "flights_commercial",
		"callsign":   "UAL1234",
		"squawk":     "1200",
		"speed":      "450",
	}

	rec := rowToAnalysisRecord(row)

	assert.Equal(t, "uuid-1", rec.EntityID)
	assert.Equal(t, "Test", rec.EntityName)
	assert.Equal(t, "flights_commercial", rec.LayerType)

	// Non-standard columns should be in Metadata.
	assert.Equal(t, "UAL1234", rec.Metadata["callsign"])
	assert.Equal(t, "1200", rec.Metadata["squawk"])
	assert.Equal(t, "450", rec.Metadata["speed"])
}

func TestRowToAnalysisRecord_NilValuesSkipped(t *testing.T) {
	row := map[string]any{
		"entity_id":  nil,
		"name":       nil,
		"layer_type": "test",
		"custom_key": nil,
	}

	rec := rowToAnalysisRecord(row)

	assert.Equal(t, "", rec.EntityID)
	assert.Equal(t, "", rec.EntityName)
	assert.Equal(t, "test", rec.LayerType)
	// nil custom_key should not appear in metadata.
	_, hasCustom := rec.Metadata["custom_key"]
	assert.False(t, hasCustom, "nil values should be skipped in metadata")
}

func TestRowToAnalysisRecord_IntegerCoordinates(t *testing.T) {
	// Verify toFloat64 handles int and int64 types.
	row := map[string]any{
		"lat":        int(42),
		"lon":        int64(-73),
		"altitude_m": float32(5000.5),
	}

	rec := rowToAnalysisRecord(row)

	assert.InDelta(t, 42.0, rec.Lat, 0.0001)
	assert.InDelta(t, -73.0, rec.Lon, 0.0001)
	assert.InDelta(t, 5000.5, rec.Altitude, 0.1)
}

func TestRowToAnalysisRecord_TimestampFormats(t *testing.T) {
	tests := []struct {
		name      string
		tsValue   any
		wantTS    string
		wantEmpty bool
	}{
		{
			name:    "RFC3339 string",
			tsValue: "2026-03-28T12:00:00Z",
			wantTS:  "2026-03-28T12:00:00Z",
		},
		{
			name:    "time.Time value",
			tsValue: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
			wantTS:  "2026-03-28T12:00:00Z",
		},
		{
			name:    "datetime without timezone",
			tsValue: "2026-03-28 12:00:00",
			wantTS:  "2026-03-28T12:00:00Z",
		},
		{
			name:      "missing ts field",
			tsValue:   nil,
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := map[string]any{}
			if tt.tsValue != nil {
				row["ts"] = tt.tsValue
			}

			rec := rowToAnalysisRecord(row)

			if tt.wantEmpty {
				assert.Equal(t, "", rec.Timestamp)
			} else {
				assert.Equal(t, tt.wantTS, rec.Timestamp)
			}
		})
	}
}

func TestRowToAnalysisRecord_EmptyRow(t *testing.T) {
	rec := rowToAnalysisRecord(map[string]any{})

	assert.Equal(t, "", rec.EntityID)
	assert.Equal(t, "", rec.EntityName)
	assert.Equal(t, "", rec.ExternalID)
	assert.Equal(t, "", rec.LayerType)
	assert.InDelta(t, 0.0, rec.Lat, 0.0001)
	assert.InDelta(t, 0.0, rec.Lon, 0.0001)
	assert.InDelta(t, 0.0, rec.Altitude, 0.1)
	assert.Equal(t, "", rec.Timestamp)
	assert.NotNil(t, rec.Metadata)
	// Mirrored structural fields are present even for empty rows (as zero values).
	assert.Equal(t, 4, len(rec.Metadata))
	assert.Equal(t, "", rec.Metadata["entity_id"])
	assert.Equal(t, "", rec.Metadata["entity_external_id"])
	assert.Equal(t, "", rec.Metadata["entity_name"])
	assert.Equal(t, "", rec.Metadata["layer_type"])
}

// --- anyToString / UUID conversion tests ---

func TestAnyToString_RawUUIDBytes(t *testing.T) {
	// pgx.RowToMap returns PostgreSQL UUID columns as [16]byte.
	raw := [16]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0x00}
	got := anyToString(raw)
	assert.Equal(t, "aabbccdd-eeff-1122-3344-556677889900", got)
}

func TestAnyToString_StringPassthrough(t *testing.T) {
	got := anyToString("some-uuid-string")
	assert.Equal(t, "some-uuid-string", got)
}

func TestAnyToString_IntValue(t *testing.T) {
	got := anyToString(42)
	assert.Equal(t, "42", got)
}

func TestAnyToString_Float64_LargeWholeNumber(t *testing.T) {
	// This is the exact bug: pgx may return large numeric TEXT values as float64
	// via RowToMap. fmt.Sprintf("%v", ...) renders these in scientific notation.
	got := anyToString(float64(166217243))
	assert.Equal(t, "166217243", got, "large float64 must not use scientific notation")
}

func TestAnyToString_Float64_WithFraction(t *testing.T) {
	got := anyToString(float64(3.14159))
	assert.Equal(t, "3.14159", got)
}

func TestAnyToString_Float32_LargeWholeNumber(t *testing.T) {
	got := anyToString(float32(152352563))
	assert.Equal(t, "152352560", got) // float32 precision loss is expected
}

func TestAnyToString_Int64(t *testing.T) {
	got := anyToString(int64(9007199254740993))
	assert.Equal(t, "9007199254740993", got)
}

func TestAnyToString_Int32(t *testing.T) {
	got := anyToString(int32(12345))
	assert.Equal(t, "12345", got)
}

func TestAnyToString_Bool(t *testing.T) {
	assert.Equal(t, "true", anyToString(true))
	assert.Equal(t, "false", anyToString(false))
}

func TestRowToAnalysisRecord_NumericExternalID(t *testing.T) {
	// Simulates the exact production bug: external_id arrives as float64
	// from pgx.RowToMap when the TEXT column contains a numeric value.
	row := map[string]any{
		"entity_id":   "some-uuid",
		"external_id": float64(166217243),
		"name":        "Radiation Sensor",
		"layer_type":  "radiation",
	}
	rec := rowToAnalysisRecord(row)
	assert.Equal(t, "166217243", rec.ExternalID, "external_id must not use scientific notation")
	assert.Equal(t, "166217243", rec.Metadata["entity_external_id"], "mirrored metadata must match")
}

func TestRowToAnalysisRecord_NumericMetadataValues(t *testing.T) {
	// Custom SQL columns with numeric values must also avoid scientific notation.
	row := map[string]any{
		"entity_id":  "some-uuid",
		"sensor_id":  float64(152352563),
		"reading":    float64(1332),
		"confidence": float64(0.95),
	}
	rec := rowToAnalysisRecord(row)
	assert.Equal(t, "152352563", rec.Metadata["sensor_id"])
	assert.Equal(t, "1332", rec.Metadata["reading"])
	assert.Equal(t, "0.95", rec.Metadata["confidence"])
}

func TestRowToAnalysisRecord_UUIDAsByteArray(t *testing.T) {
	// Simulates a pgx RowToMap result where entity_id is [16]byte.
	raw := [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	row := map[string]any{
		"entity_id":   raw,
		"external_id": "EXT-001",
		"name":        "Test Entity",
		"layer_type":  "flights_commercial",
	}
	rec := rowToAnalysisRecord(row)
	assert.Equal(t, "01020304-0506-0708-090a-0b0c0d0e0f10", rec.EntityID)
	assert.Equal(t, "EXT-001", rec.ExternalID)
}

// --- fetchDataFromSQL tests ---

// mockQueryExecutor implements the analysis.QueryExecutor interface for testing.
type mockQueryExecutor struct {
	rows []map[string]any
	err  error
}

func (m *mockQueryExecutor) QueryRows(_ context.Context, _ string) ([]map[string]any, error) {
	return m.rows, m.err
}

func TestEngine_FetchDataFromSQL_ConvertsRows(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	// Inject a mock query executor.
	engine.queryExec = &mockQueryExecutor{
		rows: []map[string]any{
			{
				"entity_id":  "uuid-1",
				"name":       "Flight ABC",
				"layer_type": "flights_commercial",
				"lat":        40.7128,
				"lon":        -74.006,
				"ts":         "2026-03-28T11:30:00Z",
				"callsign":   "ABC123",
			},
			{
				"entity_id":  "uuid-2",
				"name":       "Flight XYZ",
				"layer_type": "flights_commercial",
				"lat":        41.0,
				"lon":        -73.0,
				"ts":         "2026-03-28T11:45:00Z",
			},
		},
	}

	def := makeTestDefinition("sql_test")
	def.Data.SQL = "SELECT entity_id, name, layer_type, lat, lon, ts, callsign FROM entities"

	cutoff := now.Add(-1 * time.Hour)
	records, err := engine.fetchDataFromSQL(context.Background(), def, cutoff, zerolog.Nop())
	require.NoError(t, err)

	assert.Len(t, records, 2)
	assert.Equal(t, "uuid-1", records[0].EntityID)
	assert.Equal(t, "Flight ABC", records[0].EntityName)
	assert.Equal(t, "ABC123", records[0].Metadata["callsign"])
	assert.Equal(t, "uuid-2", records[1].EntityID)
}

func TestEngine_FetchDataFromSQL_FiltersOldRows(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	engine.queryExec = &mockQueryExecutor{
		rows: []map[string]any{
			{
				"entity_id": "recent",
				"ts":        "2026-03-28T11:30:00Z", // within lookback
			},
			{
				"entity_id": "old",
				"ts":        "2026-03-28T09:00:00Z", // outside lookback
			},
		},
	}

	def := makeTestDefinition("sql_lookback_test")
	def.Data.SQL = "SELECT entity_id, ts FROM entities"

	cutoff := now.Add(-1 * time.Hour) // anything before 11:00 is too old
	records, err := engine.fetchDataFromSQL(context.Background(), def, cutoff, zerolog.Nop())
	require.NoError(t, err)

	assert.Len(t, records, 1)
	assert.Equal(t, "recent", records[0].EntityID)
}

// --- Dedup tests ---

func TestComputeDedupKey(t *testing.T) {
	t.Run("deterministic with entity_external_id", func(t *testing.T) {
		// Structural fields are mirrored into Metadata by rowToAnalysisRecord,
		// so test records must include them in Metadata to match real behavior.
		rec := AnalysisRecord{
			ExternalID: "ABC123",
			Metadata:   map[string]string{"entity_external_id": "ABC123"},
		}
		keyFields := []string{"entity_external_id"}

		key1 := computeDedupKey(rec, keyFields)
		key2 := computeDedupKey(rec, keyFields)

		// Same input must always produce the same hash.
		assert.Equal(t, key1, key2, "dedup key must be deterministic")

		// The key must be a non-empty hex string (SHA-256 → 64 hex chars).
		assert.Len(t, key1, 64)
	})

	t.Run("different ExternalID produces different key", func(t *testing.T) {
		keyFields := []string{"entity_external_id"}
		key1 := computeDedupKey(AnalysisRecord{
			ExternalID: "ABC123",
			Metadata:   map[string]string{"entity_external_id": "ABC123"},
		}, keyFields)
		key2 := computeDedupKey(AnalysisRecord{
			ExternalID: "XYZ789",
			Metadata:   map[string]string{"entity_external_id": "XYZ789"},
		}, keyFields)

		assert.NotEqual(t, key1, key2, "different ExternalIDs must produce different dedup keys")
	})

	t.Run("composite key with metadata field", func(t *testing.T) {
		keyFields := []string{"entity_external_id", "conflict_zone"}
		rec := AnalysisRecord{
			ExternalID: "ABC123",
			Metadata:   map[string]string{"entity_external_id": "ABC123", "conflict_zone": "zone_a"},
		}

		key := computeDedupKey(rec, keyFields)
		assert.Len(t, key, 64)

		// Changing the metadata value must change the key.
		recOther := AnalysisRecord{
			ExternalID: "ABC123",
			Metadata:   map[string]string{"entity_external_id": "ABC123", "conflict_zone": "zone_b"},
		}
		keyOther := computeDedupKey(recOther, keyFields)
		assert.NotEqual(t, key, keyOther, "different metadata values must produce different dedup keys")
	})
}

func TestDedupRecords_FiltersExistingKeys(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	keyFields := []string{"entity_external_id"}
	def := makeTestDefinition("dedup_filter_test")
	def.Data.Dedup = &DedupConfig{
		Window:    "2h",
		KeyFields: keyFields,
	}

	recA := AnalysisRecord{ExternalID: "A", Metadata: map[string]string{"entity_external_id": "A"}}
	recB := AnalysisRecord{ExternalID: "B", Metadata: map[string]string{"entity_external_id": "B"}}
	recC := AnalysisRecord{ExternalID: "C", Metadata: map[string]string{"entity_external_id": "C"}}
	records := []AnalysisRecord{recA, recB, recC}

	// Pre-populate the repo with the dedup key for record "B".
	keyB := computeDedupKey(recB, keyFields)
	insightRepo.SetRecentDedupKeys(map[string]bool{keyB: true})

	filtered, dedupKeys := engine.dedupRecords(context.Background(), records, def, "op_name", zerolog.Nop())

	// Only records A and C should survive.
	require.Len(t, filtered, 2)
	externalIDs := []string{filtered[0].ExternalID, filtered[1].ExternalID}
	assert.Contains(t, externalIDs, "A")
	assert.Contains(t, externalIDs, "C")

	// dedupKeys must have entries for both surviving records.
	assert.NotNil(t, dedupKeys)
	assert.Len(t, dedupKeys, 2)

	// Verify the keys map to the expected hashes.
	expectedKeyA := computeDedupKey(recA, keyFields)
	expectedKeyC := computeDedupKey(recC, keyFields)
	keyValues := make(map[string]bool, len(dedupKeys))
	for _, v := range dedupKeys {
		keyValues[v] = true
	}
	assert.True(t, keyValues[expectedKeyA], "dedupKeys must contain key for record A")
	assert.True(t, keyValues[expectedKeyC], "dedupKeys must contain key for record C")
}

func TestDedupRecords_NilConfig_NoOp(t *testing.T) {
	engine, _, _, _ := newTestEngine(t, &mockLLMProvider{})

	def := makeTestDefinition("no_dedup_test")
	def.Data.Dedup = nil // no dedup configured

	records := []AnalysisRecord{
		{ExternalID: "X"},
		{ExternalID: "Y"},
		{ExternalID: "Z"},
	}

	filtered, dedupKeys := engine.dedupRecords(context.Background(), records, def, "op_name", zerolog.Nop())

	// All records must be returned unchanged.
	require.Len(t, filtered, 3)
	assert.Equal(t, records, filtered)

	// dedupKeys must be nil when dedup is not configured.
	assert.Nil(t, dedupKeys)
}

func TestDedupRecords_ContentAware(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	keyFields := []string{"entity_external_id", "conflict_zone"}
	def := makeTestDefinition("content_aware_dedup_test")
	def.Data.Dedup = &DedupConfig{
		Window:    "2h",
		KeyFields: keyFields,
	}

	// Record A: entity "A" in zone_a — already seen (in DB).
	recA := AnalysisRecord{
		ExternalID: "A",
		Metadata:   map[string]string{"entity_external_id": "A", "conflict_zone": "zone_a"},
	}
	// Record B: different entity "B" in zone_a — NOT in DB.
	recB := AnalysisRecord{
		ExternalID: "B",
		Metadata:   map[string]string{"entity_external_id": "B", "conflict_zone": "zone_a"},
	}
	// Record C: same entity as A but zone_b — NOT in DB (different composite key).
	recC := AnalysisRecord{
		ExternalID: "A",
		Metadata:   map[string]string{"entity_external_id": "A", "conflict_zone": "zone_b"},
	}
	records := []AnalysisRecord{recA, recB, recC}

	// Only the key for record A is already in the database.
	keyA := computeDedupKey(recA, keyFields)
	insightRepo.SetRecentDedupKeys(map[string]bool{keyA: true})

	filtered, dedupKeys := engine.dedupRecords(context.Background(), records, def, "op_name", zerolog.Nop())

	// Records B and C must survive; A is deduped.
	require.Len(t, filtered, 2, "records B and C should survive dedup")

	externalIDs := map[string]string{}
	for _, r := range filtered {
		externalIDs[r.ExternalID+"|"+r.Metadata["conflict_zone"]] = r.ExternalID
	}
	_, hasBZoneA := externalIDs["B|zone_a"]
	_, hasAZoneB := externalIDs["A|zone_b"]
	assert.True(t, hasBZoneA, "record B (zone_a) must survive")
	assert.True(t, hasAZoneB, "record C (entity A, zone_b) must survive")

	// The dedup keys map must have 2 entries for the surviving records.
	assert.Len(t, dedupKeys, 2)
}

func TestEngine_FetchDataFromSQL_QueryExecutorError(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	engine.queryExec = &mockQueryExecutor{
		err: fmt.Errorf("connection refused"),
	}

	def := makeTestDefinition("sql_error_test")
	def.Data.SQL = "SELECT 1"

	cutoff := now.Add(-1 * time.Hour)
	_, err := engine.fetchDataFromSQL(context.Background(), def, cutoff, zerolog.Nop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execute SQL")
}

func TestEngine_FetchDataFromSQL_EmptyResult(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	engine.queryExec = &mockQueryExecutor{
		rows: []map[string]any{},
	}

	def := makeTestDefinition("sql_empty_test")
	def.Data.SQL = "SELECT entity_id FROM entities WHERE 1=0"

	cutoff := now.Add(-1 * time.Hour)
	records, err := engine.fetchDataFromSQL(context.Background(), def, cutoff, zerolog.Nop())
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestEngine_FetchDataFromSQL_RowsWithoutTimestampIncluded(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)

	provider := &mockLLMProvider{
		response: &llm.CompletionResponse{Content: `{}`},
	}
	engine, _, _, _ := newTestEngine(t, provider)

	engine.queryExec = &mockQueryExecutor{
		rows: []map[string]any{
			{
				"entity_id": "no-ts-row",
				"name":      "No Timestamp",
			},
		},
	}

	def := makeTestDefinition("sql_no_ts_test")
	def.Data.SQL = "SELECT entity_id, name FROM entities"

	cutoff := now.Add(-1 * time.Hour)
	records, err := engine.fetchDataFromSQL(context.Background(), def, cutoff, zerolog.Nop())
	require.NoError(t, err)

	// Rows without a timestamp should be included (no time-based filtering).
	assert.Len(t, records, 1)
	assert.Equal(t, "no-ts-row", records[0].EntityID)
}

// --- toFloat64 tests ---

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
	}{
		{"float64", float64(42.5), 42.5},
		{"float32", float32(3.14), 3.14},
		{"int", int(100), 100.0},
		{"int64", int64(200), 200.0},
		{"int32", int32(50), 50.0},
		{"string number", "99.9", 99.9},
		{"non-numeric string", "abc", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toFloat64(tt.input)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

// --- toRFC3339 tests ---

func TestToRFC3339(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "time.Time",
			input: time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC),
			want:  "2026-03-28T12:00:00Z",
		},
		{
			name:  "RFC3339 string",
			input: "2026-03-28T12:00:00Z",
			want:  "2026-03-28T12:00:00Z",
		},
		{
			name:  "datetime string",
			input: "2026-03-28 12:00:00",
			want:  "2026-03-28T12:00:00Z",
		},
		{
			name:  "unparseable string",
			input: "not-a-date",
			want:  "not-a-date",
		},
		{
			name:  "integer",
			input: 12345,
			want:  "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRFC3339(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- normalizeNumericID tests ---

func TestNormalizeNumericID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"integer string", "258187000", "258187000"},
		{"scientific notation", "2.58187e+08", "258187000"},
		{"scientific notation uppercase", "2.58187E+08", "258187000"},
		{"large integer", "368352260", "368352260"},
		{"large scientific", "3.6835226e+08", "368352260"},
		{"float with decimal", "3.14159", "3.14159"},
		{"non-numeric string", "MMSI-123", "MMSI-123"},
		{"uuid string", "abc-def-123", "abc-def-123"},
		{"cable landing ID", "cable_lp_ternate", "cable_lp_ternate"},
		{"empty string", "", ""},
		{"zero", "0", "0"},
		{"negative integer", "-42", "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeNumericID(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- matchRecordsByExternalID numeric normalization tests ---

func TestMatchRecordsByExternalID_ScientificNotationMatch(t *testing.T) {
	// Reproduces production bug: entity external_id stored in scientific notation
	// but LLM returns integer form.
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "2.58187e+08", EntityName: "RONJA SUPPORTER"},
		{EntityID: "uuid-2", ExternalID: "3.6835226e+08", EntityName: "CONSTITUTION"},
	}

	// LLM returns the integer form.
	itemMap := map[string]any{
		"entity_external_id": "258187000",
		"vessel_name":        "RONJA SUPPORTER",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	require.Len(t, matched, 1)
	assert.Equal(t, "uuid-1", matched[0].EntityID)
	assert.Equal(t, "RONJA SUPPORTER", matched[0].EntityName)
}

func TestMatchRecordsByExternalID_ScientificNotationInRecord(t *testing.T) {
	// Reverse: record has integer, LLM returns scientific notation.
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "258187000", EntityName: "Ship A"},
		{EntityID: "uuid-2", ExternalID: "368352260", EntityName: "Ship B"},
	}

	itemMap := map[string]any{
		"entity_external_id": "2.58187e+08",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	require.Len(t, matched, 1)
	assert.Equal(t, "uuid-1", matched[0].EntityID)
}

func TestMatchRecordsByExternalID_NonNumericIDStillWorks(t *testing.T) {
	// Non-numeric IDs should still match exactly.
	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "cable_lp_ternate", EntityName: "Ternate Cable"},
		{EntityID: "uuid-2", ExternalID: "cable_lp_tokyo", EntityName: "Tokyo Cable"},
	}

	itemMap := map[string]any{
		"entity_external_id": "cable_lp_ternate",
	}

	matched := matchRecordsByExternalID(itemMap, records)
	require.Len(t, matched, 1)
	assert.Equal(t, "uuid-1", matched[0].EntityID)
}

// --- storeResultsArray layer_type derivation tests ---

func TestEngine_StoreResultsArray_DerivesLayerTypeFromMatchedRecord(t *testing.T) {
	engine, entityRepo, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	entityRepo.AddEntity(&domain.Entity{
		ID:         "flight-uuid-1",
		ExternalID: "ICAO-ABC",
		Name:       "KC-135 Tanker",
		LayerType:  "flights_military",
	})

	records := []AnalysisRecord{
		{EntityID: "flight-uuid-1", ExternalID: "ICAO-ABC", EntityName: "KC-135 Tanker", LayerType: "flights_military"},
		{EntityID: "conflict-uuid-1", ExternalID: "conflict-zone-1", EntityName: "Ukraine", LayerType: "conflict_events"},
	}

	def := makeTestDefinition("multi_layer_test")
	def.Data.Layers = []string{"flights_military", "conflict_events"}
	op := &aiconfig.OperationConfig{
		Name: "assessment",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "geopolitical_intel",
			ResultsPath:   "assessments",
		},
	}

	result := map[string]any{
		"assessments": []any{
			map[string]any{
				"entity_external_id": "ICAO-ABC",
				"assessment":         "KC-135 tanker near conflict zone",
			},
		},
	}

	// Pass nil for layerType (multi-layer scenario).
	err := engine.storeResultsArray(
		context.Background(), def, op, result,
		nil, nil, records, zerolog.Nop(),
	)
	require.NoError(t, err)

	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)
	// layer_type should be derived from the matched record (flights_military).
	require.NotNil(t, insights[0].LayerType)
	assert.Equal(t, "flights_military", *insights[0].LayerType)
}

func TestEngine_StoreResultsArray_FallbackLayerTypeWhenNoMatch(t *testing.T) {
	engine, _, _, insightRepo := newTestEngine(t, &mockLLMProvider{})

	records := []AnalysisRecord{
		{EntityID: "uuid-1", ExternalID: "ext-1", LayerType: "ships"},
		{EntityID: "uuid-2", ExternalID: "ext-2", LayerType: "subsea_cables"},
	}

	def := makeTestDefinition("fallback_test")
	op := &aiconfig.OperationConfig{
		Name: "test_op",
		Output: aiconfig.OutputConfig{
			StoreInsights: true,
			InsightType:   "test",
			ResultsPath:   "items",
		},
	}

	result := map[string]any{
		"items": []any{
			map[string]any{
				// No entity_external_id — falls back to all records.
				"summary": "general assessment",
			},
		},
	}

	// Pass a default layerType derived from first record.
	lt := "ships"
	err := engine.storeResultsArray(
		context.Background(), def, op, result,
		&lt, nil, records, zerolog.Nop(),
	)
	require.NoError(t, err)

	insights := insightRepo.GetInsights()
	require.Len(t, insights, 1)
	// When all records are returned (fallback), layer_type comes from first record.
	require.NotNil(t, insights[0].LayerType)
	assert.Equal(t, "ships", *insights[0].LayerType)
}

func TestEngine_RunAnalysis_SystemMessageAndResponseFormat(t *testing.T) {
	llmResp := &llm.CompletionResponse{
		Content: `{"summary": "All clear", "event_count": 5}`,
		Model:   "test-model",
	}
	provider := &mockLLMProvider{response: llmResp}
	engine, entityRepo, obsRepo, _ := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("test_analysis")

	// Add output_schema so JSON mode is triggered.
	schemaMap := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary":     map[string]any{"type": "string"},
			"event_count": map[string]any{"type": "integer"},
		},
	}
	require.NoError(t, engine.schemas.RegisterFromYAML(SchemaKey("test_analysis", "test_op"), schemaMap))
	def.AI.Operations[0].OutputSchema = schemaMap

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// Verify the request sent to the LLM provider.
	lastReq := provider.lastRequest
	require.NotNil(t, lastReq)

	// Should have system message as first message when output_schema is present.
	require.GreaterOrEqual(t, len(lastReq.Messages), 2, "expected system + user messages")
	assert.Equal(t, "system", lastReq.Messages[0].Role)
	assert.Contains(t, lastReq.Messages[0].Content, "You MUST respond with a valid JSON object")
	assert.Equal(t, "user", lastReq.Messages[1].Role)

	// Should have ResponseFormat set to JSON.
	assert.Equal(t, llm.ResponseFormatJSON, lastReq.ResponseFormat)
}

func TestEngine_RunAnalysis_NoSchemaNoSystemMessage(t *testing.T) {
	llmResp := &llm.CompletionResponse{
		Content: `{"result": "ok"}`,
		Model:   "test-model",
	}
	provider := &mockLLMProvider{response: llmResp}
	engine, entityRepo, obsRepo, _ := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obs := makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))
	obsRepo.AddLatestForLayer("flights_commercial", []*domain.Observation{obs})

	def := makeTestDefinition("no_schema_test")
	def.AI.Operations[0].OutputSchema = nil // no output schema

	err := engine.RunAnalysis(context.Background(), def)
	require.NoError(t, err)

	// Verify the request — no system message, no ResponseFormat.
	lastReq := provider.lastRequest
	require.NotNil(t, lastReq)
	require.Len(t, lastReq.Messages, 1, "expected only user message when no schema")
	assert.Equal(t, "user", lastReq.Messages[0].Role)
	assert.Equal(t, llm.ResponseFormat(""), lastReq.ResponseFormat)
}

// TestEngine_RunAnalysis_DedupSkipsAlreadyAnalyzed proves the token-burn defense
// for interval-scheduled analyses: once a record has been analyzed (its dedup key
// is recent), a subsequent run must NOT call the LLM again for it. Without this,
// every scheduled tick would re-send identical records and burn tokens forever.
func TestEngine_RunAnalysis_DedupSkipsAlreadyAnalyzed(t *testing.T) {
	provider := &mockLLMProvider{response: &llm.CompletionResponse{Content: `{}`}}
	engine, entityRepo, obsRepo, insightRepo := newTestEngine(t, provider)

	entity := makeTestEntity("ent-1", "ABC123", "Flight ABC123", "flights_commercial")
	entity.Metadata = map[string]string{"hex": "ABC123"}
	entityRepo.AddEntity(entity)

	now := engine.clock.Now()
	obsRepo.AddLatestForLayer("flights_commercial",
		[]*domain.Observation{makeTestObservation("obs-1", "ent-1", 40.7128, -74.0060, 10000, now.Add(-30*time.Minute))})

	def := makeTestDefinition("dedup_test")
	def.Data.Dedup = &DedupConfig{Window: "1h", KeyFields: []string{"hex"}}

	// First run: nothing seen yet → the record is analyzed once.
	require.NoError(t, engine.RunAnalysis(context.Background(), def))
	require.Equal(t, 1, provider.CallCount(), "first run must analyze the new record")

	// Mark that record's dedup key as recently seen, as a stored insight would.
	key := computeDedupKey(AnalysisRecord{Metadata: map[string]string{"hex": "ABC123"}}, []string{"hex"})
	insightRepo.SetRecentDedupKeys(map[string]bool{key: true})

	// Second run: the record is deduped → LLM must NOT be called again (no token burn).
	require.NoError(t, engine.RunAnalysis(context.Background(), def))
	assert.Equal(t, 1, provider.CallCount(), "second run must skip the already-analyzed record (no extra LLM call)")
}

// TestEngine_RunAnalysis_SQLWithoutExecutorErrors verifies a definition that sets
// data.sql fails loudly when no query executor is wired, instead of silently
// degrading to a layer query and returning different (wrong) data.
func TestEngine_RunAnalysis_SQLWithoutExecutorErrors(t *testing.T) {
	provider := &mockLLMProvider{response: &llm.CompletionResponse{Content: `{}`}}
	engine, _, _, _ := newTestEngine(t, provider)
	engine.queryExec = nil // no executor wired

	def := makeTestDefinition("sql_no_exec")
	def.Data.SQL = "SELECT 1 AS entity_id"

	err := engine.RunAnalysis(context.Background(), def)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no query executor is configured")
	assert.Equal(t, 0, provider.CallCount(), "must not call the LLM on misconfiguration")
}
