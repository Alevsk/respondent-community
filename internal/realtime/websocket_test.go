// Package realtime_test provides unit tests for the WebSocket server.
package realtime_test

import (
	"context"
	"encoding/json"
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
	"github.com/Alevsk/respondent/internal/realtime"
)

func TestNewServer(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())

	assert.NotNil(t, server)
	assert.NotNil(t, server.Clients())
}

func TestServer_ClientConnectDisconnect(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Run(ctx)

	// Create test WebSocket server
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()

		client := realtime.NewClient("test-client-1", conn, make(chan []byte, 256), logger)

		server.Register() <- client

		// Keep connection alive for test
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				server.Unregister() <- client
				return
			}
		}
	}))
	defer wsServer.Close()

	// Connect client
	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()

	// Wait for registration
	time.Sleep(100 * time.Millisecond)

	// Verify client is registered
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Close connection
	_ = conn.Close(websocket.StatusNormalClosure, "")

	// Wait for unregistration
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestServer_BroadcastLayerUpdate(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Run(ctx)

	// Create connected clients
	var clients []*realtime.Client
	var conns []*websocket.Conn
	var mu sync.Mutex

	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		client := realtime.NewClient("test-client-"+time.Now().String(), conn, make(chan []byte, 256), logger)

		mu.Lock()
		clients = append(clients, client)
		conns = append(conns, conn)
		mu.Unlock()

		server.Register() <- client

		// Subscribe to layer
		client.Subscribe("flights_commercial")

		// Read loop
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				server.Unregister() <- client
				return
			}
		}
	}))
	defer wsServer.Close()

	// Connect 2 clients
	wsURL := "ws" + strings.TrimPrefix(wsServer.URL, "http")
	for i := 0; i < 2; i++ {
		conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
		require.NoError(t, err)
		defer func() { _ = conn.CloseNow() }()
	}

	// Wait for registration
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 2
	}, time.Second, 10*time.Millisecond)

	// Broadcast update
	entities := []*domain.Entity{
		{ID: "1", ExternalID: "ext-1", LayerType: "flights_commercial", Name: "Test Flight"},
	}
	observations := []*domain.Observation{
		{ID: "obs-1", EntityID: "1", Timestamp: time.Now()},
	}

	server.BroadcastLayerUpdate("flights_commercial", entities, observations)

	// Verify broadcast is sent (clients should receive message)
	// This tests that no deadlock occurs during broadcast
	time.Sleep(100 * time.Millisecond)
}

func TestServer_BroadcastWithFullChannel(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Run(ctx)

	// Create client with very small buffer
	client := realtime.NewClient("test-client-full", nil, make(chan []byte, 1), logger)
	client.Subscribe("flights_commercial")

	// Register client
	server.Register() <- client

	// Wait for registration
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)

	// Fill the channel
	client.Send <- []byte("fill message")

	// Broadcast update - this should trigger the "channel full" path
	entities := []*domain.Entity{
		{ID: "1", ExternalID: "ext-1", LayerType: "flights_commercial", Name: "Test Flight"},
	}
	observations := []*domain.Observation{}

	// This should NOT deadlock (the bug we're fixing)
	done := make(chan bool, 1)
	go func() {
		server.BroadcastLayerUpdate("flights_commercial", entities, observations)
		done <- true
	}()

	// Verify broadcast completes without deadlock
	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(time.Second):
		t.Fatal("BroadcastLayerUpdate deadlocked")
	}
}

func TestServer_SubscribeUnsubscribe(t *testing.T) {
	logger := zerolog.Nop()

	client := realtime.NewClient("test-client", nil, make(chan []byte, 256), logger)

	// Test subscribe
	client.Subscribe("flights_commercial")
	assert.True(t, client.IsSubscribed("flights_commercial"))
	assert.False(t, client.IsSubscribed("satellites"))

	// Test unsubscribe
	client.Unsubscribe("flights_commercial")
	assert.False(t, client.IsSubscribed("flights_commercial"))

	// Test multiple subscriptions
	client.Subscribe("flights_commercial")
	client.Subscribe("satellites")
	subs := client.GetSubscriptions()
	assert.Len(t, subs, 2)
	assert.Contains(t, subs, "flights_commercial")
	assert.Contains(t, subs, "satellites")
}

func TestServer_ConcurrentBroadcast(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.Run(ctx)

	// Create multiple clients
	for i := 0; i < 10; i++ {
		client := realtime.NewClient("test-client-"+string(rune('A'+i)), nil, make(chan []byte, 256), logger)
		client.Subscribe("flights_commercial")
		server.Register() <- client
	}

	// Wait for registration
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 10
	}, time.Second, 10*time.Millisecond)

	// Concurrent broadcasts
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			entities := []*domain.Entity{
				{ID: string(rune('0' + id)), LayerType: "flights_commercial"},
			}
			server.BroadcastLayerUpdate("flights_commercial", entities, nil)
		}(i)
	}

	// Wait for all broadcasts to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Concurrent broadcasts deadlocked")
	}
}

func TestWSMessage_MarshalUnmarshal(t *testing.T) {
	msg := realtime.WSMessage{
		Type:    realtime.MsgTypeLayerUpdate,
		LayerID: "flights_commercial",
		Data: map[string]any{
			"entities": []*domain.Entity{
				{ID: "1", Name: "Test Flight"},
			},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded realtime.WSMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.LayerID, decoded.LayerID)
}

// ---------------------------------------------------------------------------
// Tests: handleTimeRange — per-layer history limit resolution
// ---------------------------------------------------------------------------

// buildTimeRangeMsg constructs a WSMessage for a time_range request.
// from and to must be RFC3339 strings.
func buildTimeRangeMsg(t *testing.T, layerID, from, to string) realtime.WSMessage {
	t.Helper()
	data := map[string]interface{}{
		"from": from,
		"to":   to,
	}
	return realtime.WSMessage{
		Type:    "time_range",
		LayerID: layerID,
		Data:    data,
	}
}

// newEnabledTimeRangeServer creates a server with time_range enabled and the given
// MaxLookback and MaxRangeSpan. obsRepo is nil so doTimeRangeQuery short-circuits
// without performing any database query — that path is tested separately.
func newEnabledTimeRangeServer(maxLookback, maxSpan time.Duration) *realtime.Server {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  maxLookback,
		MaxRangeSpan: maxSpan,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})
	return server
}

// TestHandleTimeRange_UsesServerDefaults verifies that when no per-layer history
// config is registered in the dynamic registry, the server's default MaxLookback
// and MaxRangeSpan limits are applied to clamp the requested window.
func TestHandleTimeRange_UsesServerDefaults(t *testing.T) {
	const layerID = "usgs_earthquakes" // hardcoded source; no history config registered

	// Server defaults: 6h lookback, 3h max span — deliberately small to make
	// clamping observable.
	const defaultLookback = 6 * time.Hour
	const defaultSpan = 3 * time.Hour

	server := newEnabledTimeRangeServer(defaultLookback, defaultSpan)

	logger := zerolog.Nop()
	client := realtime.NewClient("test-defaults", nil, make(chan []byte, 16), logger)
	client.SetServer(server)

	now := time.Now()
	// Request a window that exceeds both limits: 10h lookback and 8h span.
	requestFrom := now.Add(-10 * time.Hour)
	requestTo := now.Add(-2 * time.Hour) // 8h span

	msg := buildTimeRangeMsg(t,
		layerID,
		requestFrom.UTC().Format(time.RFC3339),
		requestTo.UTC().Format(time.RFC3339),
	)

	client.HandleTimeRange(msg)

	from, _, _ := client.GetTimeRange(layerID)

	// from must not be earlier than now - defaultLookback.
	maxAllowedFrom := now.Add(-defaultLookback)
	// Allow a 2s tolerance for test execution time.
	if from.Before(maxAllowedFrom.Add(-2 * time.Second)) {
		t.Errorf("from (%v) is before the allowed lookback boundary (%v); server defaults not enforced",
			from, maxAllowedFrom)
	}
}

// TestHandleTimeRange_UsesPerLayerOverride verifies that when a HistoryConfig is
// registered in the dynamic registry for a layer, handleTimeRange uses those limits
// instead of the server defaults.
func TestHandleTimeRange_UsesPerLayerOverride(t *testing.T) {
	// Use a unique layer ID to avoid contaminating other tests via the global registry.
	const layerID = "hist_override_layer_ws_test"

	// Server defaults are tight (2h lookback).  Per-layer override is generous (365d).
	const serverLookback = 2 * time.Hour
	const serverSpan = 1 * time.Hour
	const perLayerLookback = 365 * 24 * time.Hour // 1 year
	const perLayerSpan = 7 * 24 * time.Hour       // 1 week

	dynReg := domain.NewDynamicSourceRegistry()
	lt := domain.LayerType(layerID)
	dynReg.SetHistoryConfig(lt, &domain.HistoryConfig{
		MaxLookbackHours:  int32(perLayerLookback.Hours()),
		MaxRangeSpanHours: int32(perLayerSpan.Hours()),
	})

	// Build server with the same registry that holds the per-layer override.
	logger2 := zerolog.Nop()
	server := realtime.NewServer(logger2, nil, nil, nil, dynReg)
	server.SetTimeRangeConfig(realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  serverLookback,
		MaxRangeSpan: serverSpan,
		QueryTimeout: 5 * time.Second,
		Cooldown:     0,
		MaxResults:   500,
	})

	logger := zerolog.Nop()
	client := realtime.NewClient("test-override", nil, make(chan []byte, 16), logger)
	client.SetServer(server)

	now := time.Now()
	// Request a window that exceeds the server defaults but fits within the per-layer limits:
	// request from 30 days ago to 1 day ago (29-day span).
	requestFrom := now.Add(-30 * 24 * time.Hour)
	requestTo := now.Add(-24 * time.Hour)

	msg := buildTimeRangeMsg(t,
		layerID,
		requestFrom.UTC().Format(time.RFC3339),
		requestTo.UTC().Format(time.RFC3339),
	)

	client.HandleTimeRange(msg)

	from, _, _ := client.GetTimeRange(layerID)

	// The from time should not be clamped by the server's 2h default.
	// With a 365-day per-layer lookback the requested from (30d ago) is well within
	// bounds, so it must be stored approximately as-is (within 2s tolerance for
	// test execution time).
	if from.IsZero() {
		t.Fatal("time range not stored on client; HandleTimeRange may have returned early")
	}

	// Ensure the from time is older than the server's default lookback boundary,
	// proving the per-layer override was used (not clamped to 2h ago).
	serverBoundary := now.Add(-serverLookback)
	if !from.Before(serverBoundary) {
		t.Errorf("from (%v) is not before the server default boundary (%v); "+
			"per-layer override was not applied", from, serverBoundary)
	}
}

func TestDefaultBackfillConfig(t *testing.T) {
	cfg := realtime.DefaultBackfillConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30*time.Minute, cfg.StalenessWindow)
	assert.Equal(t, 2000, cfg.MaxResults)
	assert.Equal(t, 3*time.Second, cfg.QueryTimeout)
	assert.Equal(t, 5*time.Second, cfg.Cooldown)
	assert.Nil(t, cfg.Thresholds)
}

func TestDefaultTimeRangeConfig(t *testing.T) {
	cfg := realtime.DefaultTimeRangeConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 48*time.Hour, cfg.MaxLookback)
	assert.Equal(t, 24*time.Hour, cfg.MaxRangeSpan)
	assert.Equal(t, 5*time.Second, cfg.QueryTimeout)
	assert.Equal(t, 2*time.Second, cfg.Cooldown)
	assert.Equal(t, 2000, cfg.MaxResults)
}
func TestDefaultOnDemandConfig(t *testing.T) {
	cfg := realtime.DefaultOnDemandConfig()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 10*time.Second, cfg.Cooldown)
	assert.Equal(t, 5*time.Second, cfg.QueryTimeout)
	assert.Equal(t, 10, cfg.MinCacheGap)
	assert.Nil(t, cfg.LayerAPIs)
}
func TestServer_SetAllowedOrigins(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	origins := []string{"https://example.com", "https://test.com"}
	server.SetAllowedOrigins(origins)
}
func TestServer_SetBackfillConfig(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	cfg := realtime.BackfillConfig{
		Enabled:         true,
		StalenessWindow: 1 * time.Hour,
		MaxResults:      500,
		QueryTimeout:    2 * time.Second,
		Cooldown:        10 * time.Second,
		Thresholds:      map[string]int{"flights": 50},
	}
	server.SetBackfillConfig(cfg)
}
func TestServer_SetTimeRangeConfig(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	cfg := realtime.TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  72 * time.Hour,
		MaxRangeSpan: 48 * time.Hour,
		QueryTimeout: 10 * time.Second,
		Cooldown:     5 * time.Second,
		MaxResults:   1000,
	}
	server.SetTimeRangeConfig(cfg)
}
func TestServer_SetOnDemandConfig(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	cfg := realtime.OnDemandConfig{
		Enabled:       true,
		Cooldown:      5 * time.Second,
		QueryTimeout:  3 * time.Second,
		MinCacheGap:   20,
		LayerAPIs:     map[string]string{"flights": "http://api.example.com"},
		PollIntervals: map[string]time.Duration{"flights": 10 * time.Second},
	}
	server.SetOnDemandConfig(cfg)
}
func TestClient_SetViewport_GetViewport(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	assert.Nil(t, client.GetViewport("layer1"))
	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	client.SetViewport("layer1", bbox)
	result := client.GetViewport("layer1")
	require.NotNil(t, result)
	assert.Equal(t, -75.0, result.West)
	assert.Equal(t, 39.0, result.South)
	assert.Equal(t, -72.0, result.East)
	assert.Equal(t, 42.0, result.North)
	client.SetViewport("layer2", &domain.BBox{West: -10, South: -5, East: 10, North: 5})
	assert.NotNil(t, client.GetViewport("layer2"))
	assert.NotNil(t, client.GetViewport("layer1"))
}
func TestClient_IsFrozen(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	assert.False(t, client.IsFrozen("layer1"))
	client.SetTimeRange("layer1", time.Now().Add(-1*time.Hour), time.Now().Add(-30*time.Minute), true)
	assert.True(t, client.IsFrozen("layer1"))
	client.SetTimeRange("layer2", time.Now().Add(-1*time.Hour), time.Now(), false)
	assert.False(t, client.IsFrozen("layer2"))
}
func TestClient_EnableSubscription(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetTimeRange("layer1", time.Now().Add(-1*time.Hour), time.Now(), false)
	assert.True(t, client.IsInTimeRange("layer1"))
	client.EnableSubscription("layer1")
	assert.True(t, client.IsSubscribed("layer1"))
	assert.True(t, client.IsInTimeRange("layer1"))
}
func TestClient_SetTimeRange_IsInTimeRange(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	assert.False(t, client.IsInTimeRange("layer1"))
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()
	client.SetTimeRange("layer1", from, to, false)
	assert.True(t, client.IsInTimeRange("layer1"))
	assert.False(t, client.IsFrozen("layer1"))
	from2 := time.Now().Add(-2 * time.Hour)
	to2 := time.Now().Add(-1 * time.Hour)
	client.SetTimeRange("layer2", from2, to2, true)
	assert.True(t, client.IsInTimeRange("layer2"))
	assert.True(t, client.IsFrozen("layer2"))
}
func TestClient_Subscribe_ResetsTimeRange(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.SetTimeRange("layer1", time.Now().Add(-1*time.Hour), time.Now(), true)
	assert.True(t, client.IsInTimeRange("layer1"))
	assert.True(t, client.IsFrozen("layer1"))
	client.Subscribe("layer1")
	assert.True(t, client.IsSubscribed("layer1"))
	assert.False(t, client.IsInTimeRange("layer1"))
	assert.False(t, client.IsFrozen("layer1"))
}
func TestClient_Unsubscribe_ClearsTimeRange(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	client.Subscribe("layer1")
	client.SetTimeRange("layer1", time.Now().Add(-1*time.Hour), time.Now(), true)
	assert.True(t, client.IsSubscribed("layer1"))
	assert.True(t, client.IsInTimeRange("layer1"))
	client.Unsubscribe("layer1")
	assert.False(t, client.IsSubscribed("layer1"))
	assert.False(t, client.IsInTimeRange("layer1"))
	assert.False(t, client.IsFrozen("layer1"))
}
func TestClient_GetSubscriptions(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	subs := client.GetSubscriptions()
	assert.Empty(t, subs)
	client.Subscribe("layer1")
	client.Subscribe("layer2")
	client.Subscribe("layer3")
	subs = client.GetSubscriptions()
	assert.Len(t, subs, 3)
	assert.Contains(t, subs, "layer1")
	assert.Contains(t, subs, "layer2")
	assert.Contains(t, subs, "layer3")
}
func TestServer_IsSpatialLayer(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, viewportRegistry("flights_commercial", "ships"))
	assert.True(t, server.IsSpatialLayer("flights_commercial"))
	assert.True(t, server.IsSpatialLayer("ships"))
	assert.False(t, server.IsSpatialLayer("satellites"))
}

// Regression: a layer's spatial filtering must resolve from the registry, keyed by
// LAYER TYPE, never by source name. The "air_quality" layer is fed by sources named
// "openaq_air_quality"/"purpleair_air_quality"; keying spatial resolution by source
// name made isSpatialLayer("air_quality") false, so viewport_update produced no
// snapshot and billboards froze when panning. Resolving via filtering mode fixes it.
func TestServer_IsSpatialLayer_ResolvesByLayerTypeNotSourceName(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, viewportRegistry("air_quality"))
	assert.True(t, server.IsSpatialLayer("air_quality"), "layer type is viewport-filtered")
	assert.False(t, server.IsSpatialLayer("openaq_air_quality"), "source name is not a layer type")
}
func TestServer_NewClients(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	assert.NotNil(t, server.Clients())
	assert.Empty(t, server.Clients())
}
func TestClient_CooldownMethods(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	cooldown := 100 * time.Millisecond
	assert.True(t, client.CanBackfill("layer1", cooldown))
	assert.False(t, client.CanBackfill("layer1", cooldown))
	time.Sleep(cooldown + 10*time.Millisecond)
	assert.True(t, client.CanBackfill("layer1", cooldown))
	assert.True(t, client.CanTimeRange("layer2", cooldown))
	assert.False(t, client.CanTimeRange("layer2", cooldown))
	assert.True(t, client.CanOnDemand("layer3", cooldown))
	assert.False(t, client.CanOnDemand("layer3", cooldown))
}
func TestServer_Run_ContextCancel(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		server.Run(ctx)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit on context cancel")
	}
}
func TestServer_Run_ClientRegistration(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)
	client := realtime.NewClient("test-client", nil, make(chan []byte, 256), logger)
	server.Register() <- client
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)
}
func TestServer_Run_ClientUnregistration(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test-client", nil, sendCh, logger)
	server.Register() <- client
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)
	server.Unregister() <- client
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 0
	}, time.Second, 10*time.Millisecond)
}
func TestServer_BroadcastUpdate(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test-client", nil, sendCh, logger)
	server.Register() <- client
	assert.Eventually(t, func() bool {
		return len(server.Clients()) == 1
	}, time.Second, 10*time.Millisecond)
	entity := &domain.Entity{
		ID:         "test-1",
		ExternalID: "ext-1",
		LayerType:  "flights_commercial",
		Name:       "Test Flight",
	}
	obs := &domain.Observation{
		ID:        "obs-1",
		EntityID:  "test-1",
		Timestamp: time.Now(),
	}
	server.BroadcastUpdate(&domain.EntityUpdate{
		Entity:      entity,
		Observation: obs,
	})
	select {
	case msg := <-sendCh:
		assert.NotNil(t, msg)
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}
func TestServer_BroadcastUpdate_NilEntity(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	server.BroadcastUpdate(nil)
	server.BroadcastUpdate(&domain.EntityUpdate{Entity: nil})
}
func TestServer_BroadcastSnapshot(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test-client", nil, sendCh, logger)
	server.Register() <- client
	entities := []*domain.Entity{{ID: "1", LayerType: "flights"}}
	observations := []*domain.Observation{{ID: "obs-1", EntityID: "1"}}
	server.BroadcastSnapshot("flights_commercial", entities, observations)
	select {
	case msg := <-sendCh:
		assert.NotNil(t, msg)
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}
func TestServer_BroadcastCCTVFrame(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Run(ctx)
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test-client", nil, sendCh, logger)
	server.Register() <- client
	server.BroadcastCCTVFrame("camera-1", []byte("frame-data"))
	select {
	case msg := <-sendCh:
		assert.NotNil(t, msg)
		var wsMsg realtime.WSMessage
		require.NoError(t, json.Unmarshal(msg, &wsMsg))
		assert.Equal(t, realtime.MsgTypeCCTVFrame, wsMsg.Type)
		assert.Equal(t, "cctv", wsMsg.LayerID)
	case <-time.After(time.Second):
		t.Fatal("expected broadcast message")
	}
}
func TestGenerateClientID(t *testing.T) {
	id1 := realtime.GenerateClientID()
	id2 := realtime.GenerateClientID()
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}
func TestClient_ConcurrentOperations(t *testing.T) {
	logger := zerolog.Nop()
	client := realtime.NewClient("test", nil, make(chan []byte, 256), logger)
	const goroutines = 10
	const iterations = 100
	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				layerID := string(rune('A' + id%5))
				client.Subscribe(layerID)
				client.IsSubscribed(layerID)
				client.SetViewport(layerID, &domain.BBox{West: float64(i), South: 0, East: 1, North: 1})
				client.GetViewport(layerID)
				client.Unsubscribe(layerID)
			}
		}(g)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent operations timed out")
	}
}
func TestServer_ConcurrentConfigUpdates(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	var wg sync.WaitGroup
	const goroutines = 4
	wg.Add(goroutines)
	go func() {
		defer wg.Done()
		for range 10 {
			server.SetBackfillConfig(realtime.BackfillConfig{Enabled: true, MaxResults: 500})
		}
	}()
	go func() {
		defer wg.Done()
		for range 10 {
			server.SetTimeRangeConfig(realtime.TimeRangeConfig{Enabled: true, MaxResults: 1000})
		}
	}()
	go func() {
		defer wg.Done()
		for range 10 {
			server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: true, Cooldown: 5 * time.Second})
		}
	}()
	go func() {
		defer wg.Done()
		for range 10 {
			server.SetAllowedOrigins([]string{"https://example.com"})
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent config updates timed out")
	}
}
func TestWSMessage_AllTypes(t *testing.T) {
	tests := []struct {
		name    string
		msg     realtime.WSMessage
		wantErr bool
	}{
		{
			name: "layer_update",
			msg: realtime.WSMessage{
				Type:    realtime.MsgTypeLayerUpdate,
				LayerID: "flights",
				Data:    map[string]interface{}{"entity": map[string]string{"id": "1"}},
			},
		},
		{
			name: "snapshot",
			msg: realtime.WSMessage{
				Type:    realtime.MsgTypeLayerSnapshot,
				LayerID: "flights",
				Data:    map[string]interface{}{"entities": []interface{}{}},
			},
		},
		{
			name: "indicator_update",
			msg: realtime.WSMessage{
				Type:    realtime.MsgTypeIndicatorUpdate,
				LayerID: "solar",
				Data:    map[string]interface{}{"values": []interface{}{}},
			},
		},
		{
			name: "cctv_frame",
			msg: realtime.WSMessage{
				Type:    realtime.MsgTypeCCTVFrame,
				LayerID: "cctv",
				Data:    map[string]interface{}{"camera_feed_id": "cam1"},
			},
		},
		{
			name: "batch_update",
			msg: realtime.WSMessage{
				Type:    realtime.MsgTypeLayerBatchUpdate,
				LayerID: "flights",
				Data:    map[string]interface{}{"updates": []interface{}{}},
			},
		},
		{
			name: "system_backpressure",
			msg: realtime.WSMessage{
				Type: realtime.MsgTypeSystemBackpressure,
				Data: map[string]interface{}{"dropped": 5, "message": "Some updates were dropped."},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			var decoded realtime.WSMessage
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.msg.Type, decoded.Type)
			assert.Equal(t, tt.msg.LayerID, decoded.LayerID)
		})
	}
}
func TestServer_SendSnapshotToClient(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, sendCh, logger)
	entities := make([]*domain.Entity, 100)
	observations := make([]*domain.Observation, 100)
	for i := range 100 {
		entities[i] = &domain.Entity{
			ID:         string(rune('A' + i%26)),
			ExternalID: string(rune('0' + i%10)),
			LayerType:  "flights_commercial",
		}
		observations[i] = &domain.Observation{
			ID:        string(rune('a' + i%26)),
			EntityID:  entities[i].ID,
			Timestamp: time.Now(),
		}
	}
	server.SendSnapshotToClient(client, "flights_commercial", entities, observations)
	msgCount := 0
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case msg := <-sendCh:
			assert.NotNil(t, msg)
			msgCount++
		case <-timeout:
			goto done
		}
	}
done:
	assert.Greater(t, msgCount, 0, "expected at least one snapshot message")
}
func TestServer_SendSnapshotToClient_FullBuffer(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	sendCh := make(chan []byte, 1)
	client := realtime.NewClient("test-client-full", nil, sendCh, logger)
	sendCh <- []byte("fill")
	entities := []*domain.Entity{{ID: "1", LayerType: "flights"}}
	observations := []*domain.Observation{{ID: "obs-1", EntityID: "1"}}
	done := make(chan struct{})
	go func() {
		server.SendSnapshotToClient(client, "flights_commercial", entities, observations)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SendSnapshotToClient should not block on full buffer")
	}
}
func TestClient_HandleMessage_UnknownType(t *testing.T) {
	logger := zerolog.Nop()
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, sendCh, logger)
	msg := realtime.WSMessage{
		Type:    "unknown_type",
		LayerID: "test_layer",
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	client.HandleMessage(data)
}
func TestClient_HandleMessage_InvalidJSON(t *testing.T) {
	logger := zerolog.Nop()
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test", nil, sendCh, logger)
	client.HandleMessage([]byte("invalid json"))
}

func TestHandleMessage_Ping(t *testing.T) {
	logger := zerolog.Nop()
	sendCh := make(chan []byte, 256)
	client := realtime.NewClient("test-ping", nil, sendCh, logger)

	data, err := json.Marshal(realtime.WSMessage{Type: "ping"})
	require.NoError(t, err)
	client.HandleMessage(data)

	select {
	case raw := <-sendCh:
		var reply realtime.WSMessage
		require.NoError(t, json.Unmarshal(raw, &reply))
		assert.Equal(t, "pong", reply.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("no pong received on Send channel")
	}
}

func TestServer_GetBackfillThreshold(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	threshold := server.GetBackfillThreshold("unknown_layer")
	assert.Equal(t, 10, threshold)
	server.SetBackfillConfig(realtime.BackfillConfig{
		Thresholds: map[string]int{"flights": 50, "ships": 100},
	})
	assert.Equal(t, 50, server.GetBackfillThreshold("flights"))
	assert.Equal(t, 100, server.GetBackfillThreshold("ships"))
	assert.Equal(t, 10, server.GetBackfillThreshold("unknown"))
}
func TestClient_SubscriptionLimit(t *testing.T) {
	logger := zerolog.Nop()
	sendCh := make(chan []byte, 256)
	server := realtime.NewServer(logger, nil, nil, nil, domain.NewDynamicSourceRegistry())
	client := realtime.NewClient("test", nil, sendCh, logger)
	client.SetServer(server)
	for i := range 101 {
		msg := realtime.WSMessage{
			Type:    "subscribe",
			LayerID: string(rune('A'+i%26)) + string(rune('0'+i/26)),
		}
		data, _ := json.Marshal(msg)
		client.HandleMessage(data)
	}
}
func TestViewportEntityLimit(t *testing.T) {
	tests := []struct {
		name      string
		bbox      *domain.BBox
		baseLimit int
		wantMin   int
		wantMax   int
	}{
		{
			name:      "nil bbox returns base limit",
			bbox:      nil,
			baseLimit: 1000,
			wantMin:   1000,
			wantMax:   1000,
		},
		{
			name:      "small area returns full limit",
			bbox:      &domain.BBox{West: 0, South: 0, East: 10, North: 10},
			baseLimit: 1000,
			wantMin:   1000,
			wantMax:   1000,
		},
		{
			name:      "medium area scales linearly",
			bbox:      &domain.BBox{West: 0, South: 0, East: 50, North: 50},
			baseLimit: 2000,
			wantMin:   500,
			wantMax:   2000,
		},
		{
			name:      "large area returns 20 percent",
			bbox:      &domain.BBox{West: -180, South: -90, East: 180, North: 90},
			baseLimit: 2500,
			wantMin:   500,
			wantMax:   600,
		},
		{
			name:      "antimeridian wrapping counts as large area",
			bbox:      &domain.BBox{West: 170, South: -45, East: -170, North: 45},
			baseLimit: 2000,
			wantMin:   500,
			wantMax:   2000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := realtime.ViewportEntityLimit(tt.bbox, tt.baseLimit)
			assert.GreaterOrEqual(t, got, tt.wantMin)
			assert.LessOrEqual(t, got, tt.wantMax)
		})
	}
}
func TestBBoxCenter_Antimeridian(t *testing.T) {
	tests := []struct {
		name    string
		bbox    domain.BBox
		wantLat float64
		wantLon float64
	}{
		{
			name:    "normal bbox",
			bbox:    domain.BBox{West: -74.0, South: 40.0, East: -73.0, North: 41.0},
			wantLat: 40.5,
			wantLon: -73.5,
		},
		{
			name:    "antimeridian crossing west to east",
			bbox:    domain.BBox{West: 170.0, South: -10.0, East: -170.0, North: 10.0},
			wantLat: 0.0,
			wantLon: 0.0,
		},
		{
			name:    "antimeridian crossing east to west near dateline",
			bbox:    domain.BBox{West: 175.0, South: -5.0, East: -175.0, North: 5.0},
			wantLat: 0.0,
			wantLon: 0.0,
		},
		{
			name:    "global bbox",
			bbox:    domain.BBox{West: -180.0, South: -90.0, East: 180.0, North: 90.0},
			wantLat: 0.0,
			wantLon: 0.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon := realtime.BBoxCenter(&tt.bbox)
			assert.InDelta(t, tt.wantLat, lat, 0.01)
			assert.InDelta(t, tt.wantLon, lon, 0.01)
		})
	}
}
