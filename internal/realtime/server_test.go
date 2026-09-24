// Package realtime_test contains additional tests to raise coverage to 95%+.
// Focuses on: handleUnsubscribe, handleViewportUpdate, doTimeRangeQuery, doBackfill,
// publishPrioritySignal, cleanupPrioritySignals, publishEntityUpdate,
// persistOnDemandAsync, flushBatchedUpdates, HandleConnection, readPump, writePump,
// BroadcastSnapshot, BroadcastCCTVFrame, BroadcastUpdate, SendSnapshotToClient,
// handleSubscribe edge-cases.
package realtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/parse"
	"github.com/Alevsk/respondent/internal/realtime"
)

// ---------------------------------------------------------------------------
// Mock implementations of domain.ObservationRepository and domain.EntityRepository
// ---------------------------------------------------------------------------

type mockObsRepo struct {
	mu              sync.RWMutex
	snapshots       []*domain.EntitySnapshot
	bboxSnapshots   []*domain.EntitySnapshot // GetLatestByCurrentPositionInBBox: the live viewport
	backfillBBox    []*domain.EntitySnapshot // GetLatestForLayerByBBox: the staleness-window backfill
	latestObs       []*domain.Observation
	err             error
	getBBoxErr      error
	getLiveErr      error
	snapshotAtErr   error
	createBatchErr  error
	upsertErr       error
	getByExternalID map[string]*domain.Entity
}

func newMockObsRepo() *mockObsRepo {
	return &mockObsRepo{
		getByExternalID: make(map[string]*domain.Entity),
	}
}

// SetSnapshots seeds the whole store: both the full-layer page and the bbox
// queries answer from it. Tests that need the viewport to differ from the
// layer use setBBoxSnapshots to override just the spatial answer.
func (m *mockObsRepo) SetSnapshots(snaps []*domain.EntitySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = snaps
	m.bboxSnapshots = snaps
	m.backfillBBox = snaps
}

func (m *mockObsRepo) SetLatestObs(obs []*domain.Observation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latestObs = obs
	// The page query is what snapshots read, so seeding "latest observations"
	// must populate it as well.
	m.snapshots = nil
	for _, o := range obs {
		m.snapshots = append(m.snapshots, &domain.EntitySnapshot{
			Entity:      domain.Entity{ID: o.EntityID, ExternalID: o.EntityID},
			Observation: *o,
		})
	}
}

func (m *mockObsRepo) SetErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *mockObsRepo) Create(ctx context.Context, obs *domain.Observation) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.err
}

func (m *mockObsRepo) CreateBatch(ctx context.Context, obs []*domain.Observation) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.createBatchErr != nil {
		return m.createBatchErr
	}
	return m.err
}

func (m *mockObsRepo) CreateBatchUpsert(ctx context.Context, obs []*domain.Observation) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.upsertErr != nil {
		return m.upsertErr
	}
	return m.err
}

func (m *mockObsRepo) GetByEntityID(ctx context.Context, entityID string, limit int, before time.Time) ([]*domain.Observation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil, m.err
}

func (m *mockObsRepo) GetLatest(ctx context.Context, entityID string) (*domain.Observation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil, m.err
}

func (m *mockObsRepo) CountLatestForLayer(_ context.Context, _ string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return 0, m.err
	}
	return int64(len(m.snapshots)), nil
}

func (m *mockObsRepo) GetLatestForLayerPage(_ context.Context, _ string, limit, offset int) ([]*domain.EntitySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	snaps := m.snapshots
	if offset >= len(snaps) {
		return nil, nil
	}
	snaps = snaps[offset:]
	if limit > 0 && limit < len(snaps) {
		snaps = snaps[:limit]
	}
	return snaps, nil
}

// addSnapshot seeds one entity+observation pair, the shape every page and bbox
// query returns.
func (m *mockObsRepo) addSnapshot(e *domain.Entity, o *domain.Observation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snap := &domain.EntitySnapshot{Entity: *e}
	if o != nil {
		snap.Observation = *o
	}
	m.snapshots = append(m.snapshots, snap)
}

// setBBoxSnapshots seeds the LIVE viewport query
// (GetLatestByCurrentPositionInBBox). The staleness-window backfill query is
// seeded separately by setBackfillBBox — they are different questions, and a
// test that wants the viewport empty while backfill has rows needs both.
func (m *mockObsRepo) setBBoxSnapshots(entities []*domain.Entity, obs []*domain.Observation, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getBBoxErr = err
	m.getLiveErr = err
	m.bboxSnapshots = nil
	for i, e := range entities {
		snap := &domain.EntitySnapshot{Entity: *e}
		if i < len(obs) && obs[i] != nil {
			snap.Observation = *obs[i]
		}
		m.bboxSnapshots = append(m.bboxSnapshots, snap)
	}
}

func (m *mockObsRepo) GetLatestForEntityIDs(ctx context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil, m.err
}

func (m *mockObsRepo) GetLatestContentHashes(ctx context.Context, entityIDs []string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil, m.err
}

func (m *mockObsRepo) GetLayerSnapshotAt(ctx context.Context, layerType string, asOf time.Time, window time.Duration, limit int) ([]*domain.EntitySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.snapshotAtErr != nil {
		return nil, m.snapshotAtErr
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.snapshots, nil
}

func (m *mockObsRepo) GetLatestForLayerByBBox(ctx context.Context, layerType string, south, north, west, east float64, from, to time.Time, limit int) ([]*domain.EntitySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getBBoxErr != nil {
		return nil, m.getBBoxErr
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.backfillBBox, nil
}

// setBackfillBBox seeds the staleness-window backfill query.
func (m *mockObsRepo) setBackfillBBox(snaps []*domain.EntitySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.backfillBBox = snaps
}

func (m *mockObsRepo) GetLatestByCurrentPositionInBBox(ctx context.Context, layerType string, south, north, west, east float64, from, to time.Time, limit int) ([]*domain.EntitySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getLiveErr != nil {
		return nil, m.getLiveErr
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.bboxSnapshots, nil
}

func (m *mockObsRepo) PatchAIMetadata(ctx context.Context, observationID string, metadata map[string]any) error {
	panic("not implemented")
}

// mockEntityRepo implements domain.EntityRepository for tests.
type mockEntityRepo struct {
	mu             sync.RWMutex
	entities       map[string]*domain.Entity
	byExternalID   map[string]*domain.Entity
	createBatchErr error
	getByIDsErr    error
	getByExtIDErr  error
	err            error
	created        []*domain.Entity
}

func newMockEntityRepo() *mockEntityRepo {
	return &mockEntityRepo{
		entities:     make(map[string]*domain.Entity),
		byExternalID: make(map[string]*domain.Entity),
	}
}

func (m *mockEntityRepo) AddEntity(e *domain.Entity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entities[e.ID] = e
	m.byExternalID[e.LayerType+":"+e.ExternalID] = e
}

func (m *mockEntityRepo) Create(ctx context.Context, e *domain.Entity) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.err
}

func (m *mockEntityRepo) CreateBatch(ctx context.Context, entities []*domain.Entity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createBatchErr != nil {
		return m.createBatchErr
	}
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, entities...)
	return nil
}

// createdEntities returns everything CreateBatch was given, so a test can
// assert that a fetch actually reached the durable store.
func (m *mockEntityRepo) createdEntities() []*domain.Entity {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*domain.Entity(nil), m.created...)
}

func (m *mockEntityRepo) GetByID(ctx context.Context, id string) (*domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.err != nil {
		return nil, m.err
	}
	e, ok := m.entities[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (m *mockEntityRepo) GetByIDs(ctx context.Context, ids []string) ([]*domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getByIDsErr != nil {
		return nil, m.getByIDsErr
	}
	if m.err != nil {
		return nil, m.err
	}
	var result []*domain.Entity
	for _, id := range ids {
		if e, ok := m.entities[id]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *mockEntityRepo) GetByExternalID(ctx context.Context, layerType, externalID string) (*domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getByExtIDErr != nil {
		return nil, m.getByExtIDErr
	}
	if m.err != nil {
		return nil, m.err
	}
	key := layerType + ":" + externalID
	e, ok := m.byExternalID[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return e, nil
}

func (m *mockEntityRepo) GetByExternalIDs(ctx context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return nil, m.err
}

func (m *mockEntityRepo) GetDistinctLayerTypes(ctx context.Context) ([]string, error) {
	return nil, m.err
}

func (m *mockEntityRepo) CountByLayerType(ctx context.Context) (map[string]int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	counts := make(map[string]int64)
	for _, e := range m.entities {
		counts[e.LayerType]++
	}
	return counts, nil
}

func (m *mockEntityRepo) Update(ctx context.Context, entity *domain.Entity) error {
	return m.err
}

func (m *mockEntityRepo) Delete(ctx context.Context, id string) error {
	return m.err
}

func (m *mockEntityRepo) SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, m.err
}

func (m *mockEntityRepo) PatchAIMetadata(ctx context.Context, entityID string, metadata map[string]any) error {
	panic("not implemented")
}

func (m *mockEntityRepo) UpdateCoordinates(ctx context.Context, entityID string, lat, lon float64) error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeEntitySnapshot creates a test EntitySnapshot.
func makeEntitySnapshot(layerType, externalID string) *domain.EntitySnapshot {
	return &domain.EntitySnapshot{
		Entity: domain.Entity{
			ID:         layerType + ":" + externalID,
			ExternalID: externalID,
			LayerType:  layerType,
			Name:       "Test " + externalID,
		},
		Observation: domain.Observation{
			ID:        "obs-" + externalID,
			EntityID:  layerType + ":" + externalID,
			Timestamp: time.Now(),
		},
	}
}

// newServerWithRepos creates a server with mock repos pre-wired.
func newServerWithRepos(t *testing.T, obsRepo *mockObsRepo, entityRepo *mockEntityRepo) *realtime.Server {
	t.Helper()
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})
	return server
}

// newClientForServer creates a client wired to a server with a generous send buffer.
func newClientForServer(t *testing.T, server *realtime.Server) (*realtime.Client, chan []byte) {
	t.Helper()
	logger := zerolog.Nop()
	send := make(chan []byte, 512)
	client := realtime.NewClient("test-"+t.Name(), nil, send, logger)
	client.SetServer(server)
	return client, send
}

// ---------------------------------------------------------------------------
// handleUnsubscribe
// ---------------------------------------------------------------------------

func TestHandleUnsubscribe_EmptyLayerID(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.Subscribe("layer1")

	msg := realtime.WSMessage{Type: "unsubscribe", LayerID: ""}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// "layer1" must still be subscribed (empty layer_id was ignored)
	assert.True(t, client.IsSubscribed("layer1"))
}

func TestHandleUnsubscribe_ValidLayerID(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.Subscribe("layer1")
	client.Subscribe("layer2")

	msg := realtime.WSMessage{Type: "unsubscribe", LayerID: "layer1"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	assert.False(t, client.IsSubscribed("layer1"))
	assert.True(t, client.IsSubscribed("layer2"))
}

func TestHandleUnsubscribe_StopsOnDemandTicker(t *testing.T) {
	apiSrv := testAPIServer(t, nil)
	defer apiSrv.Close()

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	_, client := newTickerTestServer(t, apiSrv.URL, 500*time.Millisecond, sc)

	client.Subscribe("layer1")
	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	client.StartOnDemandTicker("layer1", "flights_commercial", bbox)
	require.Equal(t, 1, client.OnDemandTickerCount())

	msg := realtime.WSMessage{Type: "unsubscribe", LayerID: "layer1"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	assert.Equal(t, 0, client.OnDemandTickerCount(), "ticker must be stopped on unsubscribe")
	assert.False(t, client.IsSubscribed("layer1"))
}

// ---------------------------------------------------------------------------
// handleViewportUpdate
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_EmptyLayerID(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	// Must not panic
	client.HandleMessage(data)
}

func TestHandleViewportUpdate_InvalidBBox_TooFarSouth(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	client.SetServer(server)

	tests := []struct {
		name string
		data map[string]interface{}
	}{
		{"south < -90", map[string]interface{}{"west": -74.0, "south": -91.0, "east": -73.0, "north": 41.0}},
		{"north > 90", map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 91.0}},
		{"south > north", map[string]interface{}{"west": -74.0, "south": 42.0, "east": -73.0, "north": 41.0}},
		{"west < -180", map[string]interface{}{"west": -181.0, "south": 40.0, "east": -73.0, "north": 41.0}},
		{"east > 180", map[string]interface{}{"west": -74.0, "south": 40.0, "east": 181.0, "north": 41.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := realtime.WSMessage{
				Type:    "viewport_update",
				LayerID: "layer1",
				Data:    tt.data,
			}
			msgData, _ := json.Marshal(msg)
			client.HandleMessage(msgData) // must not panic
			// Viewport must NOT be set since bbox is invalid
		})
	}
}

func TestHandleViewportUpdate_ValidBBox_NonSpatialLayer(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	client := realtime.NewClient("test", nil, make(chan []byte, 512), logger)
	client.SetServer(server)
	client.Subscribe("layer1")

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "layer1",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	vp := client.GetViewport("layer1")
	require.NotNil(t, vp)
	assert.Equal(t, -74.0, vp.West)
}

func TestHandleViewportUpdate_ValidBBox_SpatialLayer_NoCache(t *testing.T) {
	logger := zerolog.Nop()
	// spatial layer but no cache — should set viewport, then return early
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("test_spatial"))
	client := realtime.NewClient("test", nil, make(chan []byte, 512), logger)
	client.SetServer(server)

	// Register the layer type mapping
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("viewport_test_source")
	lt := domain.LayerType("test_spatial")
	dynReg.Register(st, lt)
	defer dynReg.Unregister(st)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spatial",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// Viewport stored, no panic
	vp := client.GetViewport("test_spatial")
	require.NotNil(t, vp)
}

func TestHandleViewportUpdate_WithTimeRange_TriggersDoTimeRange(t *testing.T) {
	// Client has an active time range, viewport update should call doTimeRangeQuery.
	obsRepo := newMockObsRepo()
	server := newServerWithRepos(t, obsRepo, nil)

	client, _ := newClientForServer(t, server)
	client.Subscribe("layer1")

	// Set a time range (non-frozen, live mode)
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(5 * time.Minute)
	client.SetTimeRange("layer1", from, to, false)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "layer1",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)

	// Must not panic even if obsRepo returns nothing
	client.HandleMessage(data)
}

func TestHandleViewportUpdate_SpatialLayerWithSpatialCache(t *testing.T) {
	sc := &mockObsRepo{}
	entities := []*domain.Entity{
		{ID: "test_spatial:e1", ExternalID: "e1", LayerType: "test_spatial"},
	}
	obs := []*domain.Observation{
		{ID: "obs1", EntityID: "test_spatial:e1", Timestamp: time.Now()},
	}
	sc.setBBoxSnapshots(entities, obs, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:    false, // disable backfill so we don't need obsRepo
		MaxResults: 500,
		Thresholds: map[string]int{"test_spatial": 1000}, // high threshold to skip sparse path
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spatial",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// Expect at least one snapshot message
	select {
	case msg := <-send:
		assert.NotEmpty(t, msg)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected snapshot message from viewport update")
	}
}

func TestHandleViewportUpdate_ZeroEntities_LogsDebug(t *testing.T) {
	// Spatial layer but cache returns 0 entities and on-demand/backfill disabled
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil) // zero results

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:    false,
		MaxResults: 500,
		Thresholds: map[string]int{"test_spatial": 0}, // 0 threshold means "never trigger sparse"
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spatial",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	// Must not panic or block
	client.HandleMessage(data)
}

// ---------------------------------------------------------------------------
// doTimeRangeQuery
// ---------------------------------------------------------------------------

func TestDoTimeRangeQuery_NoServer(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	// No server set — should return immediately without panic

	now := time.Now()
	msg := buildTimeRangeMsg(t, "layer1",
		now.Add(-1*time.Hour).UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
	)
	client.HandleTimeRange(msg) // no server → early return
}

func TestDoTimeRangeQuery_TimeRangeDisabled(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{Enabled: false})

	send := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	now := time.Now()
	msg := buildTimeRangeMsg(t, "layer1",
		now.Add(-1*time.Hour).UTC().Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
	)
	client.HandleTimeRange(msg)
}

func TestDoTimeRangeQuery_InvalidFrom(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from": "not-a-date",
			"to":   time.Now().UTC().Format(time.RFC3339),
		},
	}
	client.HandleTimeRange(msg) // should log error and return
}

func TestDoTimeRangeQuery_InvalidTo(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"to":   "not-a-date",
		},
	}
	client.HandleTimeRange(msg) // should log error and return
}

func TestDoTimeRangeQuery_FromNotBeforeTo(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	now := time.Now()
	msg := buildTimeRangeMsg(t, "layer1",
		now.UTC().Format(time.RFC3339),
		now.Add(-1*time.Hour).UTC().Format(time.RFC3339), // from == to order flipped
	)
	client.HandleTimeRange(msg) // should warn and return
}

func TestDoTimeRangeQuery_WithObsRepo_Frozen_BBox(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "test-flight")
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	// Frozen time range: to is >10m in the past
	now := time.Now()
	from := now.Add(-3 * time.Hour)
	to := now.Add(-2 * time.Hour) // 2h ago → frozen

	viewport := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	client.SetViewport("layer1", viewport)

	msg := buildTimeRangeMsg(t, "layer1",
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	client.HandleTimeRange(msg)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message")
	}
}

func TestDoTimeRangeQuery_WithObsRepo_Live_BBox(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "live-flight")
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	// Live time range: to is in the future (or within 10m)
	now := time.Now()
	from := now.Add(-30 * time.Minute)
	to := now.Add(5 * time.Minute) // future → live

	viewport := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	client.SetViewport("layer1", viewport)

	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from":     from.UTC().Format(time.RFC3339),
			"to":       to.UTC().Format(time.RFC3339),
			"viewport": map[string]float64{"west": -75, "south": 39, "east": -72, "north": 42},
		},
	}
	client.HandleTimeRange(msg)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message")
	}
}

func TestDoTimeRangeQuery_WithObsRepo_NoViewport(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "no-vp-flight")
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	now := time.Now()
	from := now.Add(-1 * time.Hour)
	to := now.Add(-30 * time.Minute) // frozen

	msg := buildTimeRangeMsg(t, "layer1",
		from.UTC().Format(time.RFC3339),
		to.UTC().Format(time.RFC3339),
	)
	client.HandleTimeRange(msg)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message")
	}
}

func TestDoTimeRangeQuery_ObsRepoError(t *testing.T) {
	obsRepo := newMockObsRepo()
	obsRepo.SetErr(errors.New("db error"))

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	now := time.Now()
	msg := buildTimeRangeMsg(t, "layer1",
		now.Add(-1*time.Hour).UTC().Format(time.RFC3339),
		now.Add(-30*time.Minute).UTC().Format(time.RFC3339),
	)
	client.HandleTimeRange(msg)

	// Should not block or panic; no message sent on error
	select {
	case <-send:
		t.Fatal("unexpected message on error path")
	case <-time.After(200 * time.Millisecond):
		// Expected: error logged, nothing sent
	}
}

func TestDoTimeRangeQuery_Cooldown(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "cooldown-test")
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     10 * time.Second, // long cooldown
		MaxResults:   500,
	})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	now := time.Now()
	msg := buildTimeRangeMsg(t, "layer1",
		now.Add(-1*time.Hour).UTC().Format(time.RFC3339),
		now.Add(-30*time.Minute).UTC().Format(time.RFC3339),
	)

	// First call — consumed cooldown
	client.HandleTimeRange(msg)
	// Drain send channel
	select {
	case <-send:
	case <-time.After(200 * time.Millisecond):
	}

	// Second call within cooldown — must be skipped
	client.HandleTimeRange(msg)

	select {
	case <-send:
		t.Fatal("unexpected message: cooldown should have blocked second query")
	case <-time.After(200 * time.Millisecond):
		// Expected
	}
}

// ---------------------------------------------------------------------------
// doBackfill
// ---------------------------------------------------------------------------

func TestDoBackfill_EmptyResult(t *testing.T) {
	// doBackfill with a repo that returns empty results
	obsRepo := newMockObsRepo()
	obsRepo.SetSnapshots(nil) // returns nil, nil, nil

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoBackfill(context.Background(), "layer1", "flights_commercial", bbox, nil)
	assert.Empty(t, entities)
	assert.Empty(t, obs)
}

func TestDoBackfill_WithSnapshots(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "backfill-entity")
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
		QueryTimeout:    5 * time.Second,
		Cooldown:        5 * time.Second,
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoBackfill(context.Background(), "layer1", "flights_commercial", bbox, nil)

	require.Len(t, entities, 1)
	require.Len(t, obs, 1)
	assert.Equal(t, "stale", entities[0].Source)
	assert.Equal(t, "stale", obs[0].Source)
}

func TestDoBackfill_DeduplicatesExisting(t *testing.T) {
	obsRepo := newMockObsRepo()
	obsRepo.SetSnapshots([]*domain.EntitySnapshot{
		makeEntitySnapshot("flights_commercial", "existing"),
		makeEntitySnapshot("flights_commercial", "new-entity"),
	})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
		QueryTimeout:    5 * time.Second,
	})

	// "existing" is already in cache
	existing := []*domain.Entity{
		{ID: "flights_commercial:existing", ExternalID: "existing", LayerType: "flights_commercial"},
	}

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoBackfill(context.Background(), "layer1", "flights_commercial", bbox, existing)

	// Only "new-entity" should be returned
	require.Len(t, entities, 1)
	assert.Equal(t, "new-entity", entities[0].ExternalID)
}

func TestDoBackfill_RepoError(t *testing.T) {
	obsRepo := newMockObsRepo()
	obsRepo.SetErr(errors.New("db failure"))

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoBackfill(context.Background(), "layer1", "flights_commercial", bbox, nil)
	assert.Empty(t, entities)
	assert.Empty(t, obs)
}

func TestDoBackfill_EmptySnapshots(t *testing.T) {
	obsRepo := newMockObsRepo()
	obsRepo.SetSnapshots(nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoBackfill(context.Background(), "layer1", "flights_commercial", bbox, nil)
	assert.Empty(t, entities)
	assert.Empty(t, obs)
}

// ---------------------------------------------------------------------------
// publishEntityUpdate (via SendSnapshotToClient on indicator layer)
// ---------------------------------------------------------------------------

func TestSendSnapshotToClient_IndicatorLayer(t *testing.T) {
	// Register a test indicator layer
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_snap")
	st := domain.SourceType("test_indicator_snap_source")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Value 1", SourceField: "v1", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, dynReg)
	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)

	entities := []*domain.Entity{
		{ID: "test_indicator_snap:e1", ExternalID: "e1", LayerType: "test_indicator_snap"},
	}
	observations := []*domain.Observation{
		{
			ID:        "obs1",
			EntityID:  "test_indicator_snap:e1",
			Timestamp: time.Now(),
			Metadata:  map[string]string{"v1": "3"},
		},
	}

	server.SendSnapshotToClient(client, "test_indicator_snap", entities, observations)

	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "indicator.update", wsMsg.Type)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected indicator.update message")
	}
}

func TestSendSnapshotToClient_IndicatorLayer_FullBuffer(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_full")
	st := domain.SourceType("test_indicator_full_source")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "v", Label: "V", SourceField: "v", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	send := make(chan []byte, 1) // tiny buffer
	send <- []byte("fill")       // fill it
	client := realtime.NewClient("test", nil, send, logger)

	entities := []*domain.Entity{
		{ID: "test_indicator_full:e1", ExternalID: "e1", LayerType: "test_indicator_full"},
	}
	observations := []*domain.Observation{
		{ID: "obs1", EntityID: "test_indicator_full:e1", Timestamp: time.Now(), Metadata: map[string]string{"v": "2"}},
	}

	done := make(chan struct{})
	go func() {
		server.SendSnapshotToClient(client, "test_indicator_full", entities, observations)
		close(done)
	}()

	select {
	case <-done:
		// Must not block
	case <-time.After(time.Second):
		t.Fatal("SendSnapshotToClient blocked on full buffer for indicator layer")
	}
}

func TestSendSnapshotToClient_ChunkedSnapshot(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	send := make(chan []byte, 1024)
	client := realtime.NewClient("test", nil, send, logger)

	// Create 1100 entities to force multiple snapshot chunks (chunk size = 500)
	const total = 1100
	entities := make([]*domain.Entity, total)
	observations := make([]*domain.Observation, total)
	for i := range total {
		id := domain.EntityID("flights_commercial", string(rune('A'+i%26))+strings.Repeat("x", i%5))
		entities[i] = &domain.Entity{
			ID:        id,
			LayerType: "flights_commercial",
		}
		observations[i] = &domain.Observation{
			ID:       "obs-" + id,
			EntityID: id,
		}
	}

	server.SendSnapshotToClient(client, "flights_commercial", entities, observations)

	// Count messages received
	var msgCount int
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case <-send:
			msgCount++
		case <-deadline:
			goto done
		}
	}
done:
	// 1100 entities / 500 per chunk = 3 chunks
	assert.GreaterOrEqual(t, msgCount, 2, "expected at least 2 snapshot chunks for 1100 entities")
}

func TestSendSnapshotToClient_EmptyEntities(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	send := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, send, logger)

	server.SendSnapshotToClient(client, "layer1", nil, nil)

	select {
	case <-send:
		t.Fatal("should not send message for empty entities")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestSendSnapshotToClient_ObsNilInList(t *testing.T) {
	// Test that nil observations in the list don't cause a panic
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	send := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, send, logger)

	entities := []*domain.Entity{{ID: "flights_commercial:e1", LayerType: "flights_commercial"}}
	observations := []*domain.Observation{nil} // nil obs

	server.SendSnapshotToClient(client, "layer1", entities, observations)

	select {
	case msg := <-send:
		assert.NotEmpty(t, msg)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected snapshot message")
	}
}

// ---------------------------------------------------------------------------
// flushBatchedUpdates
// ---------------------------------------------------------------------------

func TestFlushBatchedUpdates_EmptyPending(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	// Empty pending — should return without panic
	server.FlushBatchedUpdates()
}

func TestFlushBatchedUpdates_WithSubscribedClient(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.Subscribe("layer1")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Directly test FlushBatchedUpdates by calling the exported method.
	// Since we can't inject into pendingUpdates directly, verify flush with no data.
	server.FlushBatchedUpdates()
}

func TestFlushBatchedUpdates_WithFrozenClient(t *testing.T) {
	// Frozen client must NOT receive flushed updates.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.Subscribe("layer1")
	client.SetTimeRange("layer1", time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour), true)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	server.FlushBatchedUpdates()

	select {
	case <-send:
		t.Fatal("frozen client should not receive flushed updates")
	case <-time.After(100 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// HandleConnection (via httptest)
// ---------------------------------------------------------------------------

func TestHandleConnection_Basic(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	_ = conn.Close(websocket.StatusNormalClosure, "")

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestHandleConnection_WithAllowedOrigins(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	server.SetAllowedOrigins([]string{"*"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestHandleConnection_SubscribeViaWebSocket(t *testing.T) {
	// Connect via real WebSocket and send a subscribe message.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	// Wait for client registration
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Send subscribe message
	msg := realtime.WSMessage{Type: "subscribe", LayerID: "flights_commercial"}
	data, _ := json.Marshal(msg)
	err = conn.Write(context.Background(), websocket.MessageText, data)
	require.NoError(t, err)

	// Give the server time to process
	time.Sleep(100 * time.Millisecond)
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestHandleConnection_UnsubscribeViaWebSocket(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Subscribe then unsubscribe
	sub := realtime.WSMessage{Type: "subscribe", LayerID: "layer1"}
	data, _ := json.Marshal(sub)
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, data))

	time.Sleep(50 * time.Millisecond)

	unsub := realtime.WSMessage{Type: "unsubscribe", LayerID: "layer1"}
	data, _ = json.Marshal(unsub)
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, data))

	time.Sleep(50 * time.Millisecond)
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestHandleConnection_ViewportUpdateViaWebSocket(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Send viewport_update
	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "layer1",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, data))

	time.Sleep(50 * time.Millisecond)
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestHandleConnection_InvalidJSONMessage(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Send invalid JSON — server must log and continue, not crash
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, []byte("{invalid")))
	time.Sleep(50 * time.Millisecond)
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// writePump coverage — send on closed channel
// ---------------------------------------------------------------------------

func TestServer_Run_UnregisterClosesChannel(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	server.Unregister() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, time.Second, 10*time.Millisecond)
}

// ---------------------------------------------------------------------------
// BroadcastUpdate — subscribed vs non-subscribed
// ---------------------------------------------------------------------------

func TestBroadcastUpdate_SubscribedClientReceives(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	entity := &domain.Entity{
		ID:        "flights_commercial:ext1",
		LayerType: "flights_commercial",
		Source:    "live",
	}
	obs := &domain.Observation{ID: "obs1", EntityID: entity.ID, Timestamp: time.Now()}

	server.BroadcastUpdate(&domain.EntityUpdate{Entity: entity, Observation: obs})

	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.update", wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}

func TestBroadcastUpdate_SetsSourceWhenEmpty(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Entity with empty Source — should be set to "live"
	entity := &domain.Entity{ID: "layer1:ext1", LayerType: "layer1"}
	obs := &domain.Observation{ID: "obs1", EntityID: entity.ID}
	server.BroadcastUpdate(&domain.EntityUpdate{Entity: entity, Observation: obs})

	select {
	case <-send:
		assert.Equal(t, "live", entity.Source)
		assert.Equal(t, "live", obs.Source)
	case <-time.After(time.Second):
		t.Fatal("expected message")
	}
}

// ---------------------------------------------------------------------------
// BroadcastSnapshot coverage
// ---------------------------------------------------------------------------

func TestBroadcastSnapshot_NilEntities(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Broadcast nil entities — must not panic and must send the message
	server.BroadcastSnapshot("layer1", nil, nil)

	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.snapshot", wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message")
	}
}

// ---------------------------------------------------------------------------
// BroadcastCCTVFrame — no clients registered
// ---------------------------------------------------------------------------

func TestBroadcastCCTVFrame_NoClients(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Drain broadcast channel to avoid blocking
	done := make(chan struct{})
	go func() {
		server.BroadcastCCTVFrame("cam1", []byte("frame"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("BroadcastCCTVFrame blocked with no clients")
	}
}

// ---------------------------------------------------------------------------
// handleSubscribe — non-spatial layer served from the durable store
// ---------------------------------------------------------------------------

func TestHandleSubscribe_FromDurableStore(t *testing.T) {
	sc := &mockObsRepo{}
	entities := []*domain.Entity{
		{ID: "layer1:e1", ExternalID: "e1", LayerType: "layer1"},
	}
	obs := []*domain.Observation{
		{ID: "obs1", EntityID: "layer1:e1", Timestamp: time.Now()},
	}
	// Inject via SetEntity so GetLayerEntities returns it
	sc.addSnapshot(entities[0], obs[0])

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{Type: "subscribe", LayerID: "layer1"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message from cache hit")
	}
}

func TestHandleSubscribe_WithObsRepo(t *testing.T) {
	obsRepo := newMockObsRepo()
	entityRepo := newMockEntityRepo()

	now := time.Now()
	e := &domain.Entity{
		ID:         "obs-repo-entity-id",
		ExternalID: "ext1",
		LayerType:  "obsrepo_layer",
	}
	entityRepo.AddEntity(e)

	o := &domain.Observation{
		ID:        "obs-from-repo",
		EntityID:  "obs-repo-entity-id",
		Timestamp: now,
	}
	obsRepo.SetLatestObs([]*domain.Observation{o})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{Type: "subscribe", LayerID: "obsrepo_layer"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot from obsRepo fallback")
	}
}

func TestHandleSubscribe_ObsRepoError(t *testing.T) {
	obsRepo := newMockObsRepo()
	obsRepo.SetErr(errors.New("db error"))
	entityRepo := newMockEntityRepo()

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{Type: "subscribe", LayerID: "error_layer"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// Error path — no snapshot sent, no panic
	select {
	case <-send:
		t.Fatal("should not send message when obsRepo returns error")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestHandleSubscribe_WithViewport_SpatialCache(t *testing.T) {
	sc := &mockObsRepo{}
	entities := []*domain.Entity{
		{ID: "test_spatial:e1", ExternalID: "e1", LayerType: "test_spatial"},
	}
	obs := []*domain.Observation{
		{ID: "obs1", EntityID: "test_spatial:e1", Timestamp: time.Now()},
	}
	sc.setBBoxSnapshots(entities, obs, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:    false,
		MaxResults: 500,
		Thresholds: map[string]int{"test_spatial": 1000}, // high threshold = never sparse
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	// Subscribe with viewport data
	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "test_spatial",
		Data: map[string]interface{}{
			"viewport": map[string]float64{
				"west": -75, "south": 39, "east": -72, "north": 42,
			},
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(time.Second):
		t.Fatal("expected snapshot from spatial cache")
	}
}

// ---------------------------------------------------------------------------
// persistOnDemandAsync edge cases
// ---------------------------------------------------------------------------

func TestPersistOnDemandAsync_ViaOnDemandFetch(t *testing.T) {
	// Trigger persistOnDemandAsync by running doOnDemandFetchCore via an API call.
	aircraft := []struct {
		Hex    string
		Flight string
		Lat    float64
		Lon    float64
	}{
		{"persist1", "P01", 40.0, -74.0},
	}
	// Build an API server that returns valid ADSB JSON
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: a.Flight, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	obsRepo := newMockObsRepo()
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)

	// Should get entities back
	require.NotEmpty(t, entities, "expected entities from on-demand fetch")
	assert.Equal(t, len(entities), len(obs))

	// Give async persist time to complete
	time.Sleep(200 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// handleSubscribe — with viewport+sparse region triggering
// ---------------------------------------------------------------------------

func TestHandleSubscribe_SparseViewport_TriggersBackfill(t *testing.T) {
	obsRepo := newMockObsRepo()
	snap := makeEntitySnapshot("flights_commercial", "backfill-from-sub")
	// The live viewport query finds nothing, so the region reads as sparse; the
	// staleness-window backfill is what must supply the entity. Seeding both
	// from one field made this pass even with backfill disabled.
	obsRepo.setBBoxSnapshots(nil, nil, nil)
	obsRepo.setBackfillBBox([]*domain.EntitySnapshot{snap})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, obsRepo, viewportRegistry("flights_commercial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		MaxResults:      500,
		QueryTimeout:    5 * time.Second,
		Cooldown:        0, // no cooldown so backfill fires
		Thresholds:      map[string]int{"flights_commercial": 10},
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "flights_commercial",
		Data: map[string]interface{}{
			"viewport": map[string]float64{
				"west": -75, "south": 39, "east": -72, "north": 42,
			},
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("expected snapshot after backfill on sparse viewport subscribe")
	}
}

// ---------------------------------------------------------------------------
// BroadcastLayerUpdate — source field population
// ---------------------------------------------------------------------------

func TestBroadcastLayerUpdate_SetsSource(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	client.Subscribe("layer1")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	entities := []*domain.Entity{{ID: "layer1:e1", LayerType: "layer1"}} // empty source
	obs := []*domain.Observation{{ID: "obs1"}}                           // empty source

	server.BroadcastLayerUpdate("layer1", entities, obs)

	select {
	case <-send:
		assert.Equal(t, "live", entities[0].Source)
		assert.Equal(t, "live", obs[0].Source)
	case <-time.After(time.Second):
		t.Fatal("expected message")
	}
}

// ---------------------------------------------------------------------------
// Concurrent read/write pump coverage via real WebSocket + messages
// ---------------------------------------------------------------------------

func TestHandleConnection_MultipleClients_Broadcast(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")

	const numClients = 3
	conns := make([]*websocket.Conn, numClients)
	for i := range numClients {
		conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
		require.NoError(t, err)
		conns[i] = conn
	}

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == numClients
	}, 2*time.Second, 10*time.Millisecond)

	// Broadcast to all clients
	entity := &domain.Entity{ID: "layer1:e1", LayerType: "layer1", Source: "live"}
	server.BroadcastUpdate(&domain.EntityUpdate{
		Entity:      entity,
		Observation: &domain.Observation{ID: "obs1", EntityID: entity.ID},
	})

	// Close all connections
	for _, conn := range conns {
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
}

// ---------------------------------------------------------------------------
// SetTimeRange with nil maps (coverage for nil-init branch)
// ---------------------------------------------------------------------------

func TestSetTimeRange_NilMapInit(t *testing.T) {
	// NewClient initializes maps, but we test the nil-path in SetTimeRange
	// by re-testing the exported method to ensure coverage on the nil-branch.
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)

	// Should initialize maps internally
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()
	client.SetTimeRange("layer1", from, to, false)
	assert.True(t, client.IsInTimeRange("layer1"))

	from2 := time.Now().Add(-2 * time.Hour)
	to2 := time.Now().Add(-1 * time.Hour)
	client.SetTimeRange("layer2", from2, to2, true)
	assert.True(t, client.IsInTimeRange("layer2"))
	assert.True(t, client.IsFrozen("layer2"))
}

// ---------------------------------------------------------------------------
// handleMessage — all message types
// ---------------------------------------------------------------------------

func TestHandleMessage_AllTypes(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	tests := []struct {
		name string
		msg  realtime.WSMessage
	}{
		{
			name: "subscribe",
			msg:  realtime.WSMessage{Type: "subscribe", LayerID: "layer1"},
		},
		{
			name: "unsubscribe",
			msg:  realtime.WSMessage{Type: "unsubscribe", LayerID: "layer1"},
		},
		{
			name: "viewport_update",
			msg: realtime.WSMessage{
				Type:    "viewport_update",
				LayerID: "layer1",
				Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
			},
		},
		{
			name: "unknown_type",
			msg:  realtime.WSMessage{Type: "other_type", LayerID: "layer1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			send := make(chan []byte, 256)
			client := realtime.NewClient("test", nil, send, logger)
			client.SetServer(server)
			client.Subscribe("layer1") // pre-subscribe for unsubscribe test

			data, err := json.Marshal(tt.msg)
			require.NoError(t, err)
			// Must not panic
			client.HandleMessage(data)
		})
	}
}

// ---------------------------------------------------------------------------
// handleSubscribe — no server reference
// ---------------------------------------------------------------------------

func TestHandleSubscribe_NoServer(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	// No server — subscribe should still mark subscription but return early
	msg := realtime.WSMessage{Type: "subscribe", LayerID: "layer1"}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)
	assert.True(t, client.IsSubscribed("layer1"))
}

// ---------------------------------------------------------------------------
// BroadcastUpdate — nil obs is handled
// ---------------------------------------------------------------------------

func TestBroadcastUpdate_NilObservation(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	entity := &domain.Entity{ID: "layer1:e1", LayerType: "layer1"}
	server.BroadcastUpdate(&domain.EntityUpdate{
		Entity:      entity,
		Observation: nil, // nil observation
	})

	select {
	case msg := <-send:
		assert.NotEmpty(t, msg)
	case <-time.After(time.Second):
		t.Fatal("expected broadcast with nil obs")
	}
}

// ---------------------------------------------------------------------------
// flushBatchedUpdates — with pending data (via AddPendingUpdate export)
// ---------------------------------------------------------------------------

func TestFlushBatchedUpdates_SubscribedClientGetsMessage(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("flush-test", nil, send, logger)
	client.Subscribe("layer1")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Inject a pending update
	updateData := []byte(`{"entity":{"id":"e1"},"observation":{}}`)
	server.AddPendingUpdate("layer1", updateData, 0, 0, false)

	server.FlushBatchedUpdates()

	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.batch_update", wsMsg.Type)
		assert.Equal(t, "layer1", wsMsg.LayerID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected batch_update message after flush")
	}
}

func TestFlushBatchedUpdates_SpatialLayerWithViewport(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("flights_commercial"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("spatial-flush", nil, send, logger)
	client.Subscribe("flights_commercial")
	// Set a viewport that contains the update's coordinates
	client.SetViewport("flights_commercial", &domain.BBox{West: -80, South: 35, East: -70, North: 45})
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Inject a spatial update inside the viewport
	updateData := []byte(`{"entity":{"id":"f1"},"observation":{"position":{"lat":40.0,"lon":-75.0}}}`)
	server.AddPendingUpdate("flights_commercial", updateData, 40.0, -75.0, true)

	server.FlushBatchedUpdates()

	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.batch_update", wsMsg.Type)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected batch_update for in-viewport spatial entity")
	}
}

func TestFlushBatchedUpdates_SpatialEntityOutsideViewport(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("flights_commercial"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("outside-vp", nil, send, logger)
	client.Subscribe("flights_commercial")
	// Viewport is in North America
	client.SetViewport("flights_commercial", &domain.BBox{West: -80, South: 35, East: -70, North: 45})
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Inject entity that is outside the viewport (Europe)
	updateData := []byte(`{"entity":{"id":"eu1"},"observation":{"position":{"lat":48.0,"lon":2.0}}}`)
	server.AddPendingUpdate("flights_commercial", updateData, 48.0, 2.0, true)

	server.FlushBatchedUpdates()

	// Entity is outside viewport → should not deliver
	select {
	case <-send:
		t.Fatal("should not deliver entity outside viewport")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestFlushBatchedUpdates_FullSendBuffer(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 1) // tiny buffer
	send <- []byte("fill")       // fill it
	client := realtime.NewClient("full-buf", nil, send, logger)
	client.Subscribe("layer1")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	updateData := []byte(`{"entity":{"id":"e1"}}`)
	server.AddPendingUpdate("layer1", updateData, 0, 0, false)

	done := make(chan struct{})
	go func() {
		server.FlushBatchedUpdates()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("FlushBatchedUpdates blocked on full send buffer")
	}
}

func TestFlushBatchedUpdates_NotSubscribed(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("not-subbed", nil, send, logger)
	// NOT subscribed to "layer1"
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	updateData := []byte(`{"entity":{"id":"e1"}}`)
	server.AddPendingUpdate("layer1", updateData, 0, 0, false)

	server.FlushBatchedUpdates()

	select {
	case <-send:
		t.Fatal("non-subscribed client should not receive flush update")
	case <-time.After(200 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// Run — context cancel with active clients (covers Conn != nil branch)
// ---------------------------------------------------------------------------

func TestRun_ContextCancel_WithActiveRealClients(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		server.Run(ctx)
		close(done)
	}()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Cancel context — triggers graceful shutdown
	cancel()

	select {
	case <-done:
		// Server exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("Server.Run did not exit after context cancel with active clients")
	}
}

// ---------------------------------------------------------------------------
// Unregister a client that was never in the clients map (covers if _, ok branch)
// ---------------------------------------------------------------------------

func TestRun_UnregisterNonExistentClient(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Client was never registered — unregistering should be a no-op (if _, ok)
	ghost := realtime.NewClient("ghost", nil, make(chan []byte, 1), logger)
	server.Unregister() <- ghost

	// Should not block or panic
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, server.Clients())
}

// ---------------------------------------------------------------------------
// viewportEntityLimit — medium area below 500 limit floor
// ---------------------------------------------------------------------------

func TestViewportEntityLimit_MediumAreaBelowFloor(t *testing.T) {
	// Area that triggers linear scale with result < 500 → should return 500
	// latSpan * lonSpan in (500, 10000] and the computed limit < 500
	// latSpan=50, lonSpan=150 → area=7500
	// frac = 1 - 0.8*(7500-500)/9500 ≈ 1 - 0.589 ≈ 0.411
	// limit = int(50 * 0.411) = 20 < 500 → floor at 500
	bbox := &domain.BBox{West: 0, South: 0, East: 150, North: 50}
	result := realtime.ViewportEntityLimit(bbox, 50) // base=50 → computed < 500 → floor
	assert.Equal(t, 500, result)
}

func TestViewportEntityLimit_LargeAreaBelowFloor(t *testing.T) {
	// area > 10000 and baseLimit/5 < 500 → floor at 500
	bbox := &domain.BBox{West: -180, South: -90, East: 180, North: 90} // area=64800
	result := realtime.ViewportEntityLimit(bbox, 100)                  // 100/5=20 < 500 → 500
	assert.Equal(t, 500, result)
}

// ---------------------------------------------------------------------------
// persistOnDemandAsync — error paths
// ---------------------------------------------------------------------------

func TestPersistOnDemandAsync_EntityCreateBatchError(t *testing.T) {
	// Trigger via doOnDemandFetchDirect with a failing entity repo
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"errtest1", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	entityRepo.createBatchErr = errors.New("entity create batch failed")
	obsRepo := newMockObsRepo()
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.NotEmpty(t, entities) // fetch succeeds, persist errors

	// Wait for async persist to complete
	time.Sleep(200 * time.Millisecond)
}

func TestPersistOnDemandAsync_GetByExternalIDError(t *testing.T) {
	// entityRepo.CreateBatch succeeds but GetByExternalID returns error
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"extiderror", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	entityRepo.getByExtIDErr = errors.New("uuid lookup failed")
	obsRepo := newMockObsRepo()
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.NotEmpty(t, entities)

	time.Sleep(200 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// handleTimeRange — empty layerID path
// ---------------------------------------------------------------------------

func TestHandleTimeRange_EmptyLayerID(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	now := time.Now()
	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "", // empty!
		Data: map[string]interface{}{
			"from": now.Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"to":   now.UTC().Format(time.RFC3339),
		},
	}
	client.HandleTimeRange(msg) // should log warn and return
}

// ---------------------------------------------------------------------------
// handleTimeRange — global mode sets IsGlobalMode and skips viewport
// ---------------------------------------------------------------------------

func TestHandleTimeRange_GlobalMode(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test-global-tr", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	// Pre-set a viewport so we can confirm it is NOT used in global mode
	client.SetViewport("layer1", &domain.BBox{West: -75, South: 39, East: -72, North: 42})

	now := time.Now()
	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from": now.Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"to":   now.UTC().Format(time.RFC3339),
			"mode": "global",
		},
	}
	client.HandleTimeRange(msg)

	// Global mode must be recorded on the client
	assert.True(t, client.IsGlobalMode("layer1"), "expected IsGlobalMode to be true after time_range with mode:global")
}

func TestHandleTimeRange_NonGlobalMode_DoesNotSetGlobalMode(t *testing.T) {
	server := newEnabledTimeRangeServer(48*time.Hour, 24*time.Hour)
	logger := zerolog.Nop()
	client := realtime.NewClient("test-nonglobal-tr", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	now := time.Now()
	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from": now.Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"to":   now.UTC().Format(time.RFC3339),
			// no "mode" field — defaults to viewport-filtered mode
		},
	}
	client.HandleTimeRange(msg)

	assert.False(t, client.IsGlobalMode("layer1"), "expected IsGlobalMode to remain false when mode is not set")
}

// ---------------------------------------------------------------------------
// handleViewportUpdate — nil data and nil server
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_NilServer(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	// No server set

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "layer1",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	// Must not panic when server is nil
	client.HandleViewportUpdate(msg)
}

// ---------------------------------------------------------------------------
// BroadcastLayerUpdate — with non-subscribed clients (no send)
// ---------------------------------------------------------------------------

func TestBroadcastLayerUpdate_NonSubscribedClient(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 256)
	client := realtime.NewClient("c1", nil, send, logger)
	// NOT subscribing to "layer1"
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	entities := []*domain.Entity{{ID: "layer1:e1", LayerType: "layer1"}}
	server.BroadcastLayerUpdate("layer1", entities, nil)

	select {
	case <-send:
		t.Fatal("non-subscribed client should not receive layer update")
	case <-time.After(200 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// handleSubscribe — spatial layer with no observation repository wired
// ---------------------------------------------------------------------------

func TestHandleSubscribe_SpatialLayer_NoObservationRepo(t *testing.T) {
	// The layer is spatial and a viewport is supplied, but nothing can answer
	// the spatial query.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("test_spatial2"))
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "test_spatial2",
		Data: map[string]interface{}{
			"viewport": map[string]float64{
				"west": -75, "south": 39, "east": -72, "north": 42,
			},
		},
	}
	data, _ := json.Marshal(msg)
	// Must not panic — falls through with no entities
	client.HandleMessage(data)
}

// ---------------------------------------------------------------------------
// handleViewportUpdate — non-spatial layer after setting viewport
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_NonSpatialLayer_ExitsAfterViewportSet(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	client := realtime.NewClient("test", nil, make(chan []byte, 512), logger)
	client.SetServer(server)

	// With a valid cache (but layer is not spatial) — should set viewport and return
	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "non_spatial_layer",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	client.HandleViewportUpdate(msg)

	vp := client.GetViewport("non_spatial_layer")
	require.NotNil(t, vp)
}

// ---------------------------------------------------------------------------
// handleViewportUpdate — spatial layer with no observation repository wired
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_SpatialLayer_NoObservationRepo(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("test_spa3"))
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})

	client := realtime.NewClient("test", nil, make(chan []byte, 512), logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spa3",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	client.HandleViewportUpdate(msg)
	// Must not panic — exits early when cache doesn't implement spatialCacheQuerier
}

// ---------------------------------------------------------------------------
// Concurrent FlushBatchedUpdates calls (race coverage)
// ---------------------------------------------------------------------------

func TestFlushBatchedUpdates_Concurrent(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("concurrent-flush", nil, send, logger)
	client.Subscribe("layer1")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	const goroutines = 5
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				server.AddPendingUpdate("layer1", []byte(`{"x":1}`), 0, 0, false)
				server.FlushBatchedUpdates()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent FlushBatchedUpdates timed out")
	}
}

// ---------------------------------------------------------------------------
// buildIndicatorUpdate — missing source field defaults to "0"
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// SetViewport / GetViewport — nil map initialization branch
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// canBackfill / canTimeRange / canOnDemand — nil map init branches
// ---------------------------------------------------------------------------

func TestCanBackfill_NilMapInitBranch(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClientWithNilMaps("nil-cooldown", make(chan []byte, 256), logger)

	// First call initializes nil lastBackfill map
	result := client.CanBackfill("layer1", 5*time.Second)
	assert.True(t, result, "first call with no previous backfill should return true")

	// Second call within cooldown should return false
	result2 := client.CanBackfill("layer1", 5*time.Second)
	assert.False(t, result2, "second call within cooldown should return false")
}

func TestCanTimeRange_NilMapInitBranch(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClientWithNilMaps("nil-cooldown-tr", make(chan []byte, 256), logger)

	result := client.CanTimeRange("layer1", 5*time.Second)
	assert.True(t, result)
	result2 := client.CanTimeRange("layer1", 5*time.Second)
	assert.False(t, result2)
}

func TestCanOnDemand_NilMapInitBranch(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClientWithNilMaps("nil-cooldown-od", make(chan []byte, 256), logger)

	result := client.CanOnDemand("layer1", 5*time.Second)
	assert.True(t, result)
	result2 := client.CanOnDemand("layer1", 5*time.Second)
	assert.False(t, result2)
}

func TestSetViewport_NilMapInitBranch(t *testing.T) {
	// NewClientWithNilMaps creates a client with nil viewports map.
	logger := zerolog.Nop()
	client := realtime.NewClientWithNilMaps("nil-maps", make(chan []byte, 256), logger)

	// GetViewport on nil map should return nil
	assert.Nil(t, client.GetViewport("layer1"))

	// SetViewport should initialize the nil map without panicking
	bbox := &domain.BBox{West: -74, South: 40, East: -73, North: 41}
	client.SetViewport("layer1", bbox)

	result := client.GetViewport("layer1")
	require.NotNil(t, result)
	assert.Equal(t, -74.0, result.West)
}

// ---------------------------------------------------------------------------
// SetTimeRange — nil map initialization branch
// ---------------------------------------------------------------------------

func TestSetTimeRange_NilMapInitBranch(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClientWithNilMaps("nil-time", make(chan []byte, 256), logger)

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()
	// SetTimeRange should initialize nil maps without panicking
	client.SetTimeRange("layer1", from, to, true)

	gotFrom, gotTo, ok := client.GetTimeRange("layer1")
	assert.True(t, ok)
	assert.False(t, gotFrom.IsZero())
	assert.False(t, gotTo.IsZero())
	assert.True(t, client.IsFrozen("layer1"))
}

// ---------------------------------------------------------------------------
// HandleConnection — at capacity rejection
// ---------------------------------------------------------------------------

func TestHandleConnection_AtCapacity(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Fill the server to maxClients (1000) with fake clients.
	// We can't do 1000 real connections in a test, so we use the Register channel.
	// Register exactly 1000 dummy clients.
	const maxClients = 1000
	for i := range maxClients {
		c := realtime.NewClient(strings.Repeat("x", 8)+string(rune('A'+i%26)), nil, make(chan []byte, 1), logger)
		server.Register() <- c
	}
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == maxClients
	}, 5*time.Second, 10*time.Millisecond)

	// Now try to connect — should get 503
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// writePump — write error path (connection closed before write)
// ---------------------------------------------------------------------------

func TestWritePump_WriteError(t *testing.T) {
	// Connect a real WebSocket, then have the server try to write to it after
	// the connection is closed — exercises writePump's error path.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Flood the server's broadcast channel with many messages first, then close
	// so writePump tries to write after the connection is already closed.
	// We close the TCP-level conn to force a write error.
	_ = conn.CloseNow()

	// Give readPump time to detect the close and potentially start unregistering.
	// Then broadcast — the writePump should get a write error.
	for i := range 5 {
		entity := &domain.Entity{
			ID:        strings.Repeat("e", i+1),
			LayerType: "layer1",
			Source:    "live",
		}
		server.BroadcastUpdate(&domain.EntityUpdate{
			Entity:      entity,
			Observation: &domain.Observation{ID: "obs" + string(rune('0'+i)), EntityID: entity.ID},
		})
	}

	// After the write error, the client should be unregistered
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, 3*time.Second, 50*time.Millisecond)
}

// ---------------------------------------------------------------------------
// readPump — read error on normal closure (covers StatusNormalClosure branch)
// ---------------------------------------------------------------------------

func TestReadPump_NormalClosure(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Normal closure — covers StatusNormalClosure branch in readPump
	err = conn.Close(websocket.StatusNormalClosure, "bye")
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, 2*time.Second, 20*time.Millisecond)
}

// ---------------------------------------------------------------------------
// handleMessage — time_range message via real connection
// ---------------------------------------------------------------------------

func TestHandleConnection_TimeRangeMessageViaWebSocket(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	now := time.Now()
	msg := realtime.WSMessage{
		Type:    "time_range",
		LayerID: "layer1",
		Data: map[string]interface{}{
			"from": now.Add(-1 * time.Hour).UTC().Format(time.RFC3339),
			"to":   now.UTC().Format(time.RFC3339),
		},
	}
	data, _ := json.Marshal(msg)
	require.NoError(t, conn.Write(context.Background(), websocket.MessageText, data))

	time.Sleep(50 * time.Millisecond)
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// ---------------------------------------------------------------------------
// doOnDemandFetchCore — various error paths
// ---------------------------------------------------------------------------

func TestDoOnDemandFetchCore_BadURL(t *testing.T) {
	// API URL template with invalid URL
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": "http://[invalid-url]/{lat}/{lon}"},
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.Empty(t, entities)
	assert.Empty(t, obs)
}

func TestDoOnDemandFetchCore_InvalidJSONResponse(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not valid json at all"))
	}))
	defer apiSrv.Close()

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/{lat}/{lon}"},
	})

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, obs := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.Empty(t, entities)
	assert.Empty(t, obs)
}

// ---------------------------------------------------------------------------
// readPump — connection closed abruptly (non-normal status) triggers error log
// ---------------------------------------------------------------------------

func TestReadPump_AbruptClose(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Close abruptly — this causes a non-normal-closure error on readPump side
	_ = conn.CloseNow()

	// readPump should detect the error and unregister the client
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, 3*time.Second, 50*time.Millisecond)
}

// ---------------------------------------------------------------------------
// BroadcastLayerUpdate — second call within unregister channel full path
// ---------------------------------------------------------------------------

func TestBroadcastLayerUpdate_UnregisterChannelFull(t *testing.T) {
	// This tests the BroadcastLayerUpdate default path when the unregister channel
	// is also full (client send buffer full + unregister channel full).
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Fill the send buffer
	send := make(chan []byte, 1)
	client := realtime.NewClient("full-both", nil, send, logger)
	client.Subscribe("layer1")
	send <- []byte("fill")
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Broadcast rapidly so the unregister channel fills too
	entities := []*domain.Entity{{ID: "layer1:e1", LayerType: "layer1", Source: "live"}}
	done := make(chan struct{})
	go func() {
		server.BroadcastLayerUpdate("layer1", entities, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("BroadcastLayerUpdate blocked")
	}
}

func TestBuildIndicatorUpdate_MissingSourceField(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_missing_field")
	st := domain.SourceType("test_missing_field_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "present", Label: "Present", SourceField: "present_field", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
			{Key: "missing", Label: "Missing", SourceField: "missing_field", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	obs := &domain.Observation{
		Timestamp: time.Now(),
		Metadata:  map[string]string{"present_field": "3"}, // "missing_field" is absent
	}

	result := realtime.BuildIndicatorUpdateExport(dynReg, "test_missing_field", obs)
	require.NotNil(t, result)
	assert.Len(t, result.Values, 2)

	// Find the missing field value — should default to "0" with level 0
	var missingVal *realtime.IndicatorValueWS
	for i := range result.Values {
		if result.Values[i].Key == "missing" {
			missingVal = &result.Values[i]
			break
		}
	}
	require.NotNil(t, missingVal)
	assert.Equal(t, "0", missingVal.Value)
	assert.Equal(t, int32(0), missingVal.Level)
}

// ---------------------------------------------------------------------------
// persistOnDemandAsync — no entity UUID found (validObs is empty)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// persistOnDemandAsync — entity UUID found, obs upsert succeeds/fails
// ---------------------------------------------------------------------------

func TestPersistOnDemandAsync_EntityUUIDFound_UpsertSucceeds(t *testing.T) {
	// Pre-register the entity in entityRepo so GetByExternalID finds it.
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"founduuid1", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	// Pre-add entity so GetByExternalID returns it — this means validObs gets populated
	preEntity := &domain.Entity{
		ID:         "db-uuid-for-founduuid1",
		ExternalID: "founduuid1",
		LayerType:  "flights_commercial",
	}
	entityRepo.AddEntity(preEntity)

	obsRepo := newMockObsRepo()
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.NotEmpty(t, entities)

	// Give async persist time to complete
	time.Sleep(200 * time.Millisecond)
}

func TestPersistOnDemandAsync_EntityUUIDFound_UpsertFails(t *testing.T) {
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"upsertfail1", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	preEntity := &domain.Entity{
		ID:         "db-uuid-upsertfail1",
		ExternalID: "upsertfail1",
		LayerType:  "flights_commercial",
	}
	entityRepo.AddEntity(preEntity)

	obsRepo := newMockObsRepo()
	obsRepo.upsertErr = errors.New("upsert failed") // CreateBatchUpsert returns error

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.NotEmpty(t, entities)

	time.Sleep(200 * time.Millisecond)
}

func TestPersistOnDemandAsync_NoEntityUUIDFound(t *testing.T) {
	// entityRepo.CreateBatch succeeds, but GetByExternalID returns not found
	// so validObs is empty → CreateBatchUpsert is NOT called.
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"nofound1", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	// GetByExternalID returns "not found" for all entities → empty uuidMap
	// CreateBatch succeeds (no error set)

	obsRepo := newMockObsRepo()
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	assert.NotEmpty(t, entities)

	// Let async persist complete — even though UUID not found, no panic
	time.Sleep(200 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// handleSubscribe — empty layerID branch (line 927-930)
// ---------------------------------------------------------------------------

func TestHandleSubscribe_EmptyLayerID(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test-empty-sub", nil, make(chan []byte, 256), logger)

	msg := realtime.WSMessage{Type: "subscribe", LayerID: ""}
	data, _ := json.Marshal(msg)
	// Must not panic; subscribe is silently ignored.
	client.HandleMessage(data)

	assert.Equal(t, 0, len(client.GetSubscriptions()), "no subscription should be added for empty layerID")
}

// ---------------------------------------------------------------------------
// handleViewportUpdate — json.Unmarshal bbox error (lines 1089-1092)
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_BBoxUnmarshalError(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())
	client := realtime.NewClient("test-vp-err", nil, make(chan []byte, 256), logger)
	client.SetServer(server)

	// msg.Data is a string — not an object — so json.Unmarshal into BBox will fail.
	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "layer1",
		Data:    "this-is-not-a-bbox-object",
	}
	data, _ := json.Marshal(msg)
	// Must not panic; viewport is silently dropped.
	client.HandleMessage(data)

	vp := client.GetViewport("layer1")
	assert.Nil(t, vp, "viewport must not be set when bbox data cannot be parsed")
}

// ---------------------------------------------------------------------------
// handleViewportUpdate — GetLayerEntitiesByBBox error (lines 1148-1151)
// ---------------------------------------------------------------------------

func TestHandleViewportUpdate_BBoxQueryError(t *testing.T) {
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, errors.New("geo index error"))

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 256)
	client := realtime.NewClient("test-geo-err", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spatial",
		Data:    map[string]interface{}{"west": -74.0, "south": 40.0, "east": -73.0, "north": 41.0},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// No snapshot should be delivered after a geo-index error.
	select {
	case <-send:
		t.Fatal("no snapshot expected after geo-index query error")
	case <-time.After(200 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// flushBatchedUpdates — frozen subscribed client (lines 738-739)
// ---------------------------------------------------------------------------

func TestFlushBatchedUpdates_FrozenSubscribedClient(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("frozen-sub", nil, send, logger)
	client.Subscribe("layer1")
	// Freeze this client's layer1 time range (historical, not live).
	client.SetTimeRange("layer1", time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour), true)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Now inject a pending update so flushBatchedUpdates has work to do.
	server.AddPendingUpdate("layer1", []byte(`{"entity":{"id":"e1"}}`), 0, 0, false)

	server.FlushBatchedUpdates()

	// Frozen client must NOT receive the update.
	select {
	case <-send:
		t.Fatal("frozen client must not receive flushed updates")
	case <-time.After(200 * time.Millisecond):
	}
}

// ---------------------------------------------------------------------------
// HandleConnection — websocket.Accept error (lines 813-816)
// ---------------------------------------------------------------------------

func TestHandleConnection_AcceptError(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Serve the handler via httptest but send a plain HTTP GET (no WS upgrade).
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleConnection(w, r)
	}))
	defer httpSrv.Close()

	// A plain HTTP GET will cause websocket.Accept to reject the upgrade and
	// return an error — covering the "failed to accept" branch.
	resp, err := http.Get(httpSrv.URL) //nolint:noctx
	if err == nil {
		_ = resp.Body.Close()
	}
	// No clients should be registered after a failed upgrade.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, len(server.Clients()))
}

// ---------------------------------------------------------------------------
// Run — broadcast channel full default (line 621-623)
// ---------------------------------------------------------------------------

func TestRun_BroadcastChannelFull(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Create a client with a full send buffer so the server hits the default branch.
	send := make(chan []byte, 1)
	send <- []byte("fill") // pre-fill so subsequent sends hit default
	client := realtime.NewClient("full-broadcast", nil, send, logger)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// BroadcastUpdate goes through the broadcast channel → Run loop → client.Send default.
	entity := &domain.Entity{ID: "layer1:e1", LayerType: "layer1", Source: "live"}
	obs := &domain.Observation{EntityID: "layer1:e1", Timestamp: time.Now()}
	server.BroadcastUpdate(&domain.EntityUpdate{Entity: entity, Observation: obs})

	// Must not block; test passes if we reach here without deadlock.
	time.Sleep(100 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// doOnDemandFetchCore — cache SetEntity error (lines 1758-1760)
// ---------------------------------------------------------------------------

func TestDoOnDemandFetchCore_PersistError(t *testing.T) {
	aircraft := []struct {
		Hex string
		Lat float64
		Lon float64
	}{{"cacheErr1", 40.0, -74.0}}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		acs := make([]ac, len(aircraft))
		for i, a := range aircraft {
			acs[i] = ac{Hex: a.Hex, Flight: "T" + a.Hex, Lat: a.Lat, Lon: a.Lon, AltBaro: 35000}
		}
		_ = json.NewEncoder(w).Encode(resp{AC: acs, Total: len(acs)})
	}))
	defer apiSrv.Close()

	// Persisting the fetched entities fails; the entities must still reach the
	// client rather than the whole on-demand fetch being lost.
	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	sc.createBatchErr = errors.New("persist failure")

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	// Should still return entities even if cache write fails.
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	// The parse succeeds, cache write is attempted (and logs a warning), entities are returned.
	assert.NotEmpty(t, entities, "entities must be returned even when cache write fails")
}

// ---------------------------------------------------------------------------
// doOnDemandFetchCore — persist semaphore full (lines 1775-1776)
// ---------------------------------------------------------------------------

func TestDoOnDemandFetchCore_PersistsBeforeReturning(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type ac struct {
			Hex     string  `json:"hex"`
			Flight  string  `json:"flight"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
			AltBaro float64 `json:"alt_baro"`
		}
		type resp struct {
			AC    []ac `json:"ac"`
			Total int  `json:"total"`
		}
		_ = json.NewEncoder(w).Encode(resp{
			AC:    []ac{{Hex: "abc123", Flight: "TEST1", Lat: 40.5, Lon: -73.5, AltBaro: 35000}},
			Total: 1,
		})
	}))
	defer apiSrv.Close()

	entityRepo := newMockEntityRepo()
	obsRepo := &mockObsRepo{}
	obsRepo.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     0,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiSrv.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75, South: 39, East: -72, North: 42}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "layer1", "flights_commercial", bbox)
	require.NotEmpty(t, entities)

	// The persist used to run in a goroutine behind a semaphore that DROPPED
	// the write when full, so a fetched entity could reach the client and never
	// be stored. It is synchronous now: by the time the fetch returns, the
	// durable store has it.
	assert.NotEmpty(t, entityRepo.createdEntities(),
		"on-demand entities must be persisted before the fetch returns")
}

// ---------------------------------------------------------------------------
// BroadcastLayerUpdate — unregister channel full default (lines 1990-1992)
// ---------------------------------------------------------------------------

func TestBroadcastLayerUpdate_UnregisterChannelFullNoRun(t *testing.T) {
	// Without a running Run() goroutine, the unregister channel is unbuffered
	// and has no receiver. The select default branch is taken immediately.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	// Add client directly to the server's clients map (bypasses register channel).
	send := make(chan []byte, 1)
	send <- []byte("fill") // pre-fill so client.Send hits default in BroadcastLayerUpdate
	client := realtime.NewClient("no-run-client", nil, send, logger)
	client.Subscribe("layer1")
	server.AddClientDirect(client)

	entities := []*domain.Entity{{ID: "layer1:e1", LayerType: "layer1", Source: "live"}}

	done := make(chan struct{})
	go func() {
		server.BroadcastLayerUpdate("layer1", entities, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("BroadcastLayerUpdate blocked when unregister channel has no receiver")
	}
}

// ---------------------------------------------------------------------------
// SendSnapshotToClient — nil obs in indicator layer (lines 2064-2065)
// ---------------------------------------------------------------------------

func TestSendSnapshotToClient_IndicatorLayer_NilObsInList(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_nilobs")
	st := domain.SourceType("test_indicator_nilobs_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Val1", SourceField: "field1", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	send := make(chan []byte, 64)
	client := realtime.NewClient("ind-nilobs", nil, send, logger)

	entity := &domain.Entity{ID: "test_indicator_nilobs:e1", LayerType: "test_indicator_nilobs", ExternalID: "e1"}
	// Pass one nil observation and one real observation — the nil should be skipped (line 2064-2065).
	obs := &domain.Observation{
		EntityID:  "test_indicator_nilobs:e1",
		Timestamp: time.Now(),
		Source:    "live",
		Metadata:  map[string]string{"field1": "3"},
	}

	server.SendSnapshotToClient(client, "test_indicator_nilobs", []*domain.Entity{entity}, []*domain.Observation{nil, obs})

	select {
	case msg := <-send:
		assert.NotEmpty(t, msg)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected indicator update message")
	}
}

// ---------------------------------------------------------------------------
// ---------------------------------------------------------------------------
// Global mode subscribe + pagination tests
// ---------------------------------------------------------------------------

func TestHandleSubscribe_GlobalMode_WithLimitAndCursor(t *testing.T) {
	// 50 entities in the store, subscribe with mode:"global" limit:10.
	// Expect: 10 entities, total=50, cursor present. The total comes from the
	// entity repository, not from the length of the page.
	sc := &mockObsRepo{}
	entityRepo := newMockEntityRepo()
	now := time.Now()
	for i := 0; i < 50; i++ {
		eid := fmt.Sprintf("e%d", i)
		e := &domain.Entity{ID: "test_layer:" + eid, ExternalID: eid, LayerType: "test_layer"}
		o := &domain.Observation{ID: "obs-" + eid, EntityID: "test_layer:" + eid, Timestamp: now}
		sc.addSnapshot(e, o)
		entityRepo.AddEntity(e)
	}

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, entityRepo, sc, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("global-test", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "test_layer",
		Data: map[string]interface{}{
			"mode":  "global",
			"limit": 10,
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	// Should receive a snapshot with pagination metadata
	select {
	case m := <-send:
		var wsMsg struct {
			Type    string `json:"type"`
			LayerID string `json:"layer_id"`
			Data    struct {
				Entities     []json.RawMessage `json:"entities"`
				Observations []json.RawMessage `json:"observations"`
				Total        int64             `json:"total"`
				Cursor       string            `json:"cursor"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
		assert.Equal(t, "test_layer", wsMsg.LayerID)
		assert.Equal(t, 10, len(wsMsg.Data.Entities), "should have 10 entities (limit)")
		assert.Equal(t, int64(50), wsMsg.Data.Total, "total should be 50")
		assert.NotEmpty(t, wsMsg.Data.Cursor, "cursor should be present when more data available")

		// Verify cursor decodes to offset=10
		offset, err := realtime.DecodeCursor(wsMsg.Data.Cursor)
		require.NoError(t, err)
		assert.Equal(t, 10, offset, "cursor offset should equal limit")
	case <-time.After(time.Second):
		t.Fatal("expected paginated snapshot message")
	}

	// Verify global mode is set on the client
	assert.True(t, client.IsGlobalMode("test_layer"))
}

func TestHandlePage_FetchesNextPage(t *testing.T) {
	// Set up cache with 30 entities, subscribe globally, then request page 2.
	sc := &mockObsRepo{}
	entityRepo := newMockEntityRepo()
	now := time.Now()
	for i := 0; i < 30; i++ {
		eid := fmt.Sprintf("e%d", i)
		e := &domain.Entity{ID: "page_layer:" + eid, ExternalID: eid, LayerType: "page_layer"}
		o := &domain.Observation{ID: "obs-" + eid, EntityID: "page_layer:" + eid, Timestamp: now}
		sc.addSnapshot(e, o)
		entityRepo.AddEntity(e)
	}

	logger := zerolog.Nop()
	// The layer total comes from the entity repository, not the page length.
	server := realtime.NewServer(logger, entityRepo, sc, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("page-test", nil, send, logger)
	client.SetServer(server)

	// Subscribe in global mode first
	client.Subscribe("page_layer")
	client.SetGlobalMode("page_layer", true)

	// Drain any subscribe snapshot
	for {
		select {
		case <-send:
			continue
		case <-time.After(100 * time.Millisecond):
		}
		break
	}

	// Send a page request for offset=10, limit=10
	cursor := realtime.EncodeCursor(10)
	pageMsg := realtime.WSMessage{
		Type:    "page",
		LayerID: "page_layer",
		Data: map[string]interface{}{
			"cursor": cursor,
			"limit":  10,
		},
	}
	pageData, _ := json.Marshal(pageMsg)
	client.HandleMessage(pageData)

	select {
	case m := <-send:
		var wsMsg struct {
			Type    string `json:"type"`
			LayerID string `json:"layer_id"`
			Data    struct {
				Entities     []json.RawMessage `json:"entities"`
				Observations []json.RawMessage `json:"observations"`
				Total        int64             `json:"total"`
				Cursor       string            `json:"cursor"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
		assert.Equal(t, 10, len(wsMsg.Data.Entities), "page should have 10 entities")
		assert.Equal(t, int64(30), wsMsg.Data.Total)
		// There are 30 total, offset=10 + limit=10 = 20, so there are still 10 more
		assert.NotEmpty(t, wsMsg.Data.Cursor, "should have next cursor")

		nextOffset, err := realtime.DecodeCursor(wsMsg.Data.Cursor)
		require.NoError(t, err)
		assert.Equal(t, 20, nextOffset, "next cursor offset should be 20")
	case <-time.After(time.Second):
		t.Fatal("expected page snapshot message")
	}
}

func TestHandleSubscribe_GlobalMode_SmallLayer_NoCursor(t *testing.T) {
	// Layer with 3 entities and global mode — no cursor should be sent.
	sc := &mockObsRepo{}
	now := time.Now()
	for i := 0; i < 3; i++ {
		eid := fmt.Sprintf("e%d", i)
		e := &domain.Entity{ID: "small_layer:" + eid, ExternalID: eid, LayerType: "small_layer"}
		o := &domain.Observation{ID: "obs-" + eid, EntityID: "small_layer:" + eid, Timestamp: now}
		sc.addSnapshot(e, o)
	}

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, domain.NewDynamicSourceRegistry())
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("small-global", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "small_layer",
		Data: map[string]interface{}{
			"mode": "global",
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg struct {
			Type    string `json:"type"`
			LayerID string `json:"layer_id"`
			Data    struct {
				Entities     []json.RawMessage `json:"entities"`
				Observations []json.RawMessage `json:"observations"`
				Total        int64             `json:"total"`
				Cursor       string            `json:"cursor"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
		assert.Equal(t, 3, len(wsMsg.Data.Entities), "should have all 3 entities")
		assert.Equal(t, int64(3), wsMsg.Data.Total)
		assert.Empty(t, wsMsg.Data.Cursor, "no cursor for small layer that fits in one page")
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message")
	}
}

func TestFlushBatchedUpdates_GlobalMode_SkipsViewportFilter(t *testing.T) {
	// A global-mode client should receive spatial updates that are outside its viewport.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("flights_commercial"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	send := make(chan []byte, 512)
	client := realtime.NewClient("global-flush", nil, send, logger)
	client.Subscribe("flights_commercial")
	// Set a small viewport in North America
	client.SetViewport("flights_commercial", &domain.BBox{West: -80, South: 35, East: -70, North: 45})
	// Enable global mode — should bypass viewport filter
	client.SetGlobalMode("flights_commercial", true)
	server.Register() <- client

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Inject a spatial update OUTSIDE the viewport (in Europe)
	updateData := []byte(`{"entity":{"id":"eu1"},"observation":{"position":{"lat":48.0,"lon":2.0}}}`)
	server.AddPendingUpdate("flights_commercial", updateData, 48.0, 2.0, true)

	server.FlushBatchedUpdates()

	// Global mode client should still receive this update
	select {
	case msg := <-send:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.batch_update", wsMsg.Type)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("global mode client should receive spatial updates outside viewport")
	}
}

func TestHandlePage_NonGlobalSubscription_Rejected(t *testing.T) {
	// Page request on a non-global subscription should be rejected (no crash).
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	send := make(chan []byte, 512)
	client := realtime.NewClient("non-global-page", nil, send, logger)
	client.SetServer(server)
	client.Subscribe("some_layer")

	cursor := realtime.EncodeCursor(0)
	pageMsg := realtime.WSMessage{
		Type:    "page",
		LayerID: "some_layer",
		Data: map[string]interface{}{
			"cursor": cursor,
			"limit":  10,
		},
	}
	data, _ := json.Marshal(pageMsg)
	client.HandleMessage(data)

	// Should not receive any message — the page request is rejected
	select {
	case <-send:
		t.Fatal("non-global subscription should not receive page response")
	case <-time.After(200 * time.Millisecond):
		// Expected: no message
	}
}

func TestHandlePage_EmptyLayerID(t *testing.T) {
	logger := zerolog.Nop()
	send := make(chan []byte, 512)
	client := realtime.NewClient("page-empty", nil, send, logger)

	pageMsg := realtime.WSMessage{
		Type:    "page",
		LayerID: "",
		Data: map[string]interface{}{
			"cursor": realtime.EncodeCursor(0),
		},
	}
	data, _ := json.Marshal(pageMsg)
	client.HandleMessage(data) // Should not panic

	select {
	case <-send:
		t.Fatal("should not receive message for empty layer_id page request")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCursorEncodeDecode_Roundtrip(t *testing.T) {
	for _, offset := range []int{0, 10, 500, 9999} {
		cursor := realtime.EncodeCursor(offset)
		decoded, err := realtime.DecodeCursor(cursor)
		require.NoError(t, err)
		assert.Equal(t, offset, decoded, "roundtrip failed for offset %d", offset)
	}
}

func TestDecodeCursor_InvalidBase64(t *testing.T) {
	_, err := realtime.DecodeCursor("not-valid-base64!!!")
	assert.Error(t, err)
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but not valid JSON
	cursor := "dGhpcyBpcyBub3QganNvbg==" // "this is not json"
	_, err := realtime.DecodeCursor(cursor)
	assert.Error(t, err)
}

func TestHandleSubscribe_GlobalMode_SpatialLayerBypassesGeoQuery(t *testing.T) {
	// A spatial layer subscribed in global mode should NOT use the geo bbox query,
	// and should instead use GetLayerEntities (the non-spatial path).
	sc := &mockObsRepo{}
	now := time.Now()
	for i := 0; i < 5; i++ {
		eid := fmt.Sprintf("f%d", i)
		e := &domain.Entity{ID: "flights_commercial:" + eid, ExternalID: eid, LayerType: "flights_commercial"}
		o := &domain.Observation{ID: "obs-" + eid, EntityID: "flights_commercial:" + eid, Timestamp: now}
		sc.addSnapshot(e, o)
	}
	// BBox results return nothing — if global mode properly bypasses geo query,
	// we should still get the entities from GetLayerEntities.
	sc.setBBoxSnapshots(nil, nil, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("global-spatial", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "flights_commercial",
		Data: map[string]interface{}{
			"mode": "global",
			"viewport": map[string]interface{}{
				"west": -80.0, "south": 35.0, "east": -70.0, "north": 45.0,
			},
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg struct {
			Type string `json:"type"`
			Data struct {
				Entities []json.RawMessage `json:"entities"`
				Total    int64             `json:"total"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
		assert.Equal(t, 5, len(wsMsg.Data.Entities), "global mode should bypass geo query and return all entities")
	case <-time.After(time.Second):
		t.Fatal("expected snapshot from global mode spatial subscribe")
	}
}

func TestUnsubscribe_ClearsGlobalMode(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("unsub-global", nil, make(chan []byte, 256), logger)
	client.Subscribe("layer1")
	client.SetGlobalMode("layer1", true)
	assert.True(t, client.IsGlobalMode("layer1"))

	client.Unsubscribe("layer1")
	assert.False(t, client.IsGlobalMode("layer1"))
}

// ---------------------------------------------------------------------------
// Viewport bbox filtering — comprehensive lock tests (Task 4, Round 2 Phase 3)
// ---------------------------------------------------------------------------

// TestHandleSubscribe_ViewportBBoxFiltering verifies that subscribing with a
// viewport causes exactly the entities returned by GetLayerEntitiesByBBox to be
// delivered in the snapshot — no more, no less — confirming the geo-query path.
func TestHandleSubscribe_ViewportBBoxFiltering_SnapshotContainsOnlyBBoxEntities(t *testing.T) {
	sc := &mockObsRepo{}

	// Two entities inside the viewport (New York area)
	insideEntities := []*domain.Entity{
		{ID: "test_spatial:e1", ExternalID: "e1", LayerType: "test_spatial"},
		{ID: "test_spatial:e2", ExternalID: "e2", LayerType: "test_spatial"},
	}
	insideObs := []*domain.Observation{
		{ID: "obs1", EntityID: "test_spatial:e1", Timestamp: time.Now()},
		{ID: "obs2", EntityID: "test_spatial:e2", Timestamp: time.Now()},
	}
	// Mock returns only bbox-filtered entities — simulates real geo-index behavior.
	sc.setBBoxSnapshots(insideEntities, insideObs, nil)

	// Also seed entities in the "non-spatial" flat store so we can verify the
	// subscribe path does NOT fall through to GetLayerEntities when bbox results exist.
	outsideEntity := &domain.Entity{ID: "test_spatial:e99", ExternalID: "e99", LayerType: "test_spatial"}
	outsideObs := &domain.Observation{ID: "obs99", EntityID: "test_spatial:e99"}
	sc.addSnapshot(outsideEntity, outsideObs)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:    false,
		MaxResults: 500,
		Thresholds: map[string]int{"test_spatial": 1000}, // high threshold prevents sparse path
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("vp-bbox-sub", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "subscribe",
		LayerID: "test_spatial",
		Data: map[string]interface{}{
			"viewport": map[string]float64{
				"west": -75.0, "south": 39.0, "east": -72.0, "north": 42.0,
			},
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg struct {
			Type    string `json:"type"`
			LayerID string `json:"layer_id"`
			Data    struct {
				Entities     []json.RawMessage `json:"entities"`
				Observations []json.RawMessage `json:"observations"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type, "message type must be layer.snapshot")
		assert.Equal(t, "test_spatial", wsMsg.LayerID)
		// Must contain exactly the 2 entities returned by GetLayerEntitiesByBBox,
		// NOT the additional entity seeded into the flat store.
		assert.Equal(t, 2, len(wsMsg.Data.Entities),
			"snapshot must contain exactly the entities returned by GetLayerEntitiesByBBox")
	case <-time.After(time.Second):
		t.Fatal("expected snapshot message from viewport subscribe")
	}
}

// TestHandleViewportUpdate_BBoxFiltering_SuccessPath verifies the positive
// viewport_update flow: GetLayerEntitiesByBBox is called and its results are
// delivered as a layer.snapshot. This complements the existing error-path test.
func TestHandleViewportUpdate_BBoxFiltering_SuccessPath(t *testing.T) {
	sc := &mockObsRepo{}
	entities := []*domain.Entity{
		{ID: "test_spatial:a1", ExternalID: "a1", LayerType: "test_spatial"},
		{ID: "test_spatial:a2", ExternalID: "a2", LayerType: "test_spatial"},
	}
	obs := []*domain.Observation{
		{ID: "obs-a1", EntityID: "test_spatial:a1", Timestamp: time.Now()},
		{ID: "obs-a2", EntityID: "test_spatial:a2", Timestamp: time.Now()},
	}
	sc.setBBoxSnapshots(entities, obs, nil)

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))
	server.SetBackfillConfig(realtime.BackfillConfig{
		Enabled:    false,
		MaxResults: 500,
		Thresholds: map[string]int{"test_spatial": 1000},
	})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	send := make(chan []byte, 512)
	client := realtime.NewClient("vp-update-success", nil, send, logger)
	client.SetServer(server)
	client.Subscribe("test_spatial")

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "test_spatial",
		Data: map[string]interface{}{
			"west": -75.0, "south": 39.0, "east": -72.0, "north": 42.0,
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case m := <-send:
		var wsMsg struct {
			Type    string `json:"type"`
			LayerID string `json:"layer_id"`
			Data    struct {
				Entities     []json.RawMessage `json:"entities"`
				Observations []json.RawMessage `json:"observations"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(m, &wsMsg))
		assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type, "viewport_update must deliver a layer.snapshot")
		assert.Equal(t, "test_spatial", wsMsg.LayerID)
		assert.Equal(t, 2, len(wsMsg.Data.Entities),
			"snapshot must contain exactly the 2 entities returned by GetLayerEntitiesByBBox")
	case <-time.After(time.Second):
		t.Fatal("expected snapshot from viewport_update")
	}
}

// TestHandleViewportUpdate_NonSpatialLayer_NoSnapshot verifies that
// viewport_update on a non-spatial layer is silently ignored (no snapshot sent).
func TestHandleViewportUpdate_NonSpatialLayer_NoSnapshot(t *testing.T) {
	sc := &mockObsRepo{}
	// Only "test_spatial" is registered as viewport-filtered; "non_spatial" is not.
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("test_spatial"))

	send := make(chan []byte, 256)
	client := realtime.NewClient("vp-non-spatial", nil, send, logger)
	client.SetServer(server)

	msg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: "non_spatial",
		Data: map[string]interface{}{
			"west": -75.0, "south": 39.0, "east": -72.0, "north": 42.0,
		},
	}
	data, _ := json.Marshal(msg)
	client.HandleMessage(data)

	select {
	case <-send:
		t.Fatal("non-spatial layer viewport_update must not send any snapshot")
	case <-time.After(200 * time.Millisecond):
		// Expected: no message
	}
}

// TestFlushBatchedUpdates_ViewportFilterAndGlobalBypass is a combined scenario
// that runs two clients simultaneously against the same spatial layer:
//   - Client A: viewport-subscribed (North America) — must NOT receive a European update
//   - Client B: global mode — MUST receive the same European update
//
// This locks both the spatial filter AND the global-mode bypass in a single test.
func TestFlushBatchedUpdates_ViewportFilterAndGlobalBypass_Combined(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, viewportRegistry("flights_commercial"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)

	// Client A — viewport-subscribed in North America
	sendA := make(chan []byte, 512)
	clientA := realtime.NewClient("viewport-client", nil, sendA, logger)
	clientA.Subscribe("flights_commercial")
	clientA.SetViewport("flights_commercial", &domain.BBox{West: -80, South: 35, East: -70, North: 45})
	// clientA is NOT in global mode
	server.Register() <- clientA

	// Client B — global mode (no viewport restriction)
	sendB := make(chan []byte, 512)
	clientB := realtime.NewClient("global-client", nil, sendB, logger)
	clientB.Subscribe("flights_commercial")
	clientB.SetViewport("flights_commercial", &domain.BBox{West: -80, South: 35, East: -70, North: 45})
	clientB.SetGlobalMode("flights_commercial", true)
	server.Register() <- clientB

	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 2
	}, time.Second, 10*time.Millisecond)

	// Inject a spatial update OUTSIDE Client A's viewport (Paris, Europe)
	updateData := []byte(`{"entity":{"id":"eu-flight"},"observation":{"position":{"lat":48.9,"lon":2.3}}}`)
	server.AddPendingUpdate("flights_commercial", updateData, 48.9, 2.3, true)
	server.FlushBatchedUpdates()

	// Client A (viewport): outside update must NOT arrive
	select {
	case <-sendA:
		t.Fatal("viewport client must not receive update outside its bbox")
	case <-time.After(200 * time.Millisecond):
	}

	// Client B (global): outside update MUST arrive
	select {
	case msg := <-sendB:
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, "layer.batch_update", wsMsg.Type,
			"global client must receive batch_update for any spatial position")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("global-mode client must receive update regardless of position")
	}
}

// ---------------------------------------------------------------------------
// Snapshot completeness: the durable store is the authoritative snapshot source,
// not the partial hot cache. Regression tests for the WS render-completeness fix.
// ---------------------------------------------------------------------------

// drainSnapshotEntities drains the send channel and returns the raw entity JSON
// across all layer.snapshot messages (snapshots may be chunked).
func drainSnapshotEntities(t *testing.T, send chan []byte) []json.RawMessage {
	t.Helper()
	var ents []json.RawMessage
	for {
		select {
		case raw := <-send:
			var m struct {
				Type string `json:"type"`
				Data struct {
					Entities []json.RawMessage `json:"entities"`
				} `json:"data"`
			}
			if json.Unmarshal(raw, &m) == nil && m.Type == "layer.snapshot" {
				ents = append(ents, m.Data.Entities...)
			}
		case <-time.After(500 * time.Millisecond):
			return ents
		}
	}
}

// assertAllLive checks every snapshot entity is tagged with source "live".
func assertAllLive(t *testing.T, ents []json.RawMessage) {
	t.Helper()
	for i, raw := range ents {
		var e struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("entity %d: bad json: %v", i, err)
		}
		if e.Source != "live" {
			t.Errorf("entity %d: source = %q, want \"live\"", i, e.Source)
			return
		}
	}
}

func sendSubscribeMsg(t *testing.T, client *realtime.Client, layerID string, vp *domain.BBox) {
	t.Helper()
	data := map[string]any{}
	if vp != nil {
		data["viewport"] = vp
	}
	msg := realtime.WSMessage{Type: "subscribe", LayerID: layerID, Data: data}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal subscribe: %v", err)
	}
	client.HandleMessage(b)
}

func TestHandleSubscribe_NonViewport_DurableStoreOverPartialCache(t *testing.T) {
	logger := zerolog.Nop()
	const lt = "border_crossings"

	// Durable store holds the full latest-per-entity set (3 entities).
	obsRepo := newMockObsRepo()
	entityRepo := newMockEntityRepo()
	var latest []*domain.Observation
	for _, id := range []string{"c1", "c2", "c3"} {
		entityRepo.AddEntity(&domain.Entity{ID: id, ExternalID: id, LayerType: lt})
		latest = append(latest, &domain.Observation{ID: "o-" + id, EntityID: id, Timestamp: time.Now()})
	}
	obsRepo.SetLatestObs(latest)

	// Empty registry => non-spatial layer => no viewport scoping.
	server := realtime.NewServer(logger, entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	client, send := newClientForServer(t, server)
	sendSubscribeMsg(t, client, lt, nil)

	ents := drainSnapshotEntities(t, send)
	if len(ents) != 3 {
		t.Errorf("snapshot entities = %d, want 3 (the full durable set)", len(ents))
	}
	assertAllLive(t, ents)
}

func TestHandleSubscribe_Viewport_DurableBBoxSupplementsSparseCache(t *testing.T) {
	logger := zerolog.Nop()
	const lt = "flights_commercial"

	// Durable store (current-position bbox) holds the full fresh set (5 flights).
	obsRepo := newMockObsRepo()
	entityRepo := newMockEntityRepo()
	var snaps []*domain.EntitySnapshot
	for _, id := range []string{"f1", "f2", "f3", "f4", "f5"} {
		snaps = append(snaps, &domain.EntitySnapshot{
			Entity:      domain.Entity{ID: id, ExternalID: id, LayerType: lt},
			Observation: domain.Observation{ID: "o-" + id, EntityID: id, Timestamp: time.Now(), Position: &domain.GeoPoint{Lat: 19, Lon: -99}},
		})
	}
	obsRepo.SetSnapshots(snaps)

	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry(lt))
	// threshold=3: cache(1) < 3 triggers the durable bbox query; result(5) >= 3 so
	// the rate-limited on-demand path stays off.
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500, Thresholds: map[string]int{lt: 3}})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	client, send := newClientForServer(t, server)
	vp := &domain.BBox{West: -120, South: 0, East: -80, North: 40}
	sendSubscribeMsg(t, client, lt, vp)

	ents := drainSnapshotEntities(t, send)
	if len(ents) != 5 {
		t.Errorf("snapshot entities = %d, want 5 (durable bbox supplements the 1-entity sparse cache)", len(ents))
	}
	assertAllLive(t, ents)
}

// TestViewportUpdate_PanToUncachedRegion_LoadsFromDurableStore reproduces the
// reported bug: subscribe with viewport A (cached), then pan to viewport B whose
// entities are NOT in the hot cache. Before the fix, handleViewportUpdate consulted
// only the cache (then on-demand/backfill), so panning to an uncached region
// rendered nothing until the layer was toggled off and on. With on-demand AND
// backfill disabled, the durable-store fallback in buildSpatialViewportSnapshot is
// the ONLY path that can serve region B — so this test fails against the old code.
func TestViewportUpdate_PanToUncachedRegion_LoadsFromDurableStore(t *testing.T) {
	logger := zerolog.Nop()
	const lt = "air_quality"
	europe := &domain.BBox{West: -10, South: 35, East: 40, North: 70}
	us := &domain.BBox{West: -125, South: 25, East: -66, North: 49}

	obsRepo := newMockObsRepo()
	entityRepo := newMockEntityRepo()

	server := realtime.NewServer(logger, entityRepo, obsRepo, viewportRegistry(lt))
	// Disable on-demand AND backfill so the durable-store fallback is the only path
	// that can serve a panned-to region — isolating the regression.
	server.SetBackfillConfig(realtime.BackfillConfig{Enabled: false, MaxResults: 500, Thresholds: map[string]int{lt: 10}})
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	client, send := newClientForServer(t, server)

	// 1) Subscribe with the Europe viewport.
	obsRepo.setBBoxSnapshots(
		[]*domain.Entity{
			{ID: lt + ":eu1", ExternalID: "eu1", LayerType: lt},
			{ID: lt + ":eu2", ExternalID: "eu2", LayerType: lt},
		},
		[]*domain.Observation{
			{ID: "o-eu1", EntityID: lt + ":eu1", Timestamp: time.Now()},
			{ID: "o-eu2", EntityID: lt + ":eu2", Timestamp: time.Now()},
		},
		nil)
	sendSubscribeMsg(t, client, lt, europe)
	if got := len(drainSnapshotEntities(t, send)); got != 2 {
		t.Fatalf("europe subscribe: got %d entities, want 2", got)
	}

	// 2) Pan to the US — a different region, so a different answer.
	var usSnaps []*domain.EntitySnapshot
	for _, id := range []string{"us1", "us2", "us3"} {
		usSnaps = append(usSnaps, &domain.EntitySnapshot{
			Entity:      domain.Entity{ID: id, ExternalID: id, LayerType: lt},
			Observation: domain.Observation{ID: "o-" + id, EntityID: id, Timestamp: time.Now(), Position: &domain.GeoPoint{Lat: 40, Lon: -100}},
		})
	}
	obsRepo.SetSnapshots(usSnaps)

	vpMsg := realtime.WSMessage{
		Type:    "viewport_update",
		LayerID: lt,
		Data:    map[string]any{"west": us.West, "south": us.South, "east": us.East, "north": us.North},
	}
	b, err := json.Marshal(vpMsg)
	if err != nil {
		t.Fatalf("marshal viewport_update: %v", err)
	}
	client.HandleMessage(b)

	ents := drainSnapshotEntities(t, send)
	if len(ents) != 3 {
		t.Errorf("pan to US: got %d entities, want 3 — viewport_update must load a panned-to region from the durable store, not render nothing", len(ents))
	}
	assertAllLive(t, ents)
}
