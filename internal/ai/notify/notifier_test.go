package notify

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

	"github.com/Alevsk/respondent/internal/domain"
)

// ---------------------------------------------------------------------------
// Mock broadcaster
// ---------------------------------------------------------------------------

// mockBroadcaster captures raw broadcast calls for assertions.
type mockBroadcaster struct {
	mu               sync.Mutex
	messages         [][]byte
	notificationMsgs []struct {
		Meta NotificationMeta
		Data []byte
	}
}

func (m *mockBroadcaster) BroadcastRaw(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.messages = append(m.messages, cp)
}

func (m *mockBroadcaster) BroadcastNotification(meta NotificationMeta, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	m.notificationMsgs = append(m.notificationMsgs, struct {
		Meta NotificationMeta
		Data []byte
	}{Meta: meta, Data: cp})
}

func (m *mockBroadcaster) getMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([][]byte, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func (m *mockBroadcaster) getNotificationData() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]byte, len(m.notificationMsgs))
	for i, nm := range m.notificationMsgs {
		result[i] = nm.Data
	}
	return result
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestWSNotifier_ImplementsNotifier(t *testing.T) {
	var _ Notifier = (*WSNotifier)(nil)
}

// ---------------------------------------------------------------------------
// NotifyInsight tests
// ---------------------------------------------------------------------------

func TestNotifyInsight_FullPayload(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	layerType := "flights_commercial"
	createdAt := time.Date(2026, 3, 20, 14, 32, 0, 0, time.UTC)

	insight := &domain.AIInsight{
		ID:            "ins-001",
		InsightType:   "anomaly",
		SourceName:    "flight_anomaly_detection",
		OperationName: "flight_anomaly_scan",
		LayerType:     &layerType,
		Result:        map[string]any{"anomaly_type": "altitude_drop", "severity": "high"},
		EntityIDs:     []string{"ent-001", "ent-002"},
		Entities: []domain.InsightEntityRef{
			{ID: "ent-001", ExternalID: "ASY485", Name: "C-17 Globemaster", LayerType: "flights_military"},
			{ID: "ent-002", ExternalID: "RCH501", Name: "C-5 Galaxy", LayerType: "flights_military"},
		},
		ObservationIDs: []string{"obs-001"},
		CreatedAt:      createdAt,
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	// Verify top-level message structure.
	var msg wsMessage
	require.NoError(t, json.Unmarshal(msgs[0], &msg))
	assert.Equal(t, WSTypeAIInsight, msg.Type)

	// Re-unmarshal into the concrete payload type.
	var envelope struct {
		Type    string         `json:"type"`
		Payload InsightPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msgs[0], &envelope))
	p := envelope.Payload

	assert.Equal(t, "ins-001", p.ID)
	assert.Equal(t, "anomaly", p.InsightType)
	assert.Equal(t, "flight_anomaly_detection", p.SourceName)
	assert.Equal(t, "flight_anomaly_scan", p.OperationName)
	assert.Equal(t, "flights_commercial", p.LayerType)
	assert.Equal(t, "altitude_drop", p.Result["anomaly_type"])
	assert.Equal(t, "high", p.Result["severity"])
	assert.Equal(t, []string{"ent-001", "ent-002"}, p.EntityIDs)
	assert.Equal(t, []string{"obs-001"}, p.ObservationIDs)
	assert.Equal(t, "2026-03-20T14:32:00Z", p.CreatedAt)

	// Verify entities are included in the payload.
	require.Len(t, p.Entities, 2)
	assert.Equal(t, "ent-001", p.Entities[0].ID)
	assert.Equal(t, "ASY485", p.Entities[0].ExternalID)
	assert.Equal(t, "C-17 Globemaster", p.Entities[0].Name)
	assert.Equal(t, "flights_military", p.Entities[0].LayerType)
	assert.Equal(t, "ent-002", p.Entities[1].ID)
	assert.Equal(t, "RCH501", p.Entities[1].ExternalID)
}

func TestNotifyInsight_NilLayerType(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	insight := &domain.AIInsight{
		ID:            "ins-002",
		InsightType:   "summary",
		SourceName:    "cross_layer_summary",
		OperationName: "daily_summary",
		LayerType:     nil, // cross-layer insight
		Result:        map[string]any{"text": "all clear"},
		CreatedAt:     time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	var envelope struct {
		Payload InsightPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msgs[0], &envelope))

	// layer_type should be omitted (empty string -> omitempty).
	assert.Empty(t, envelope.Payload.LayerType)
	// nil slices should be serialized as empty arrays, not null.
	assert.Equal(t, []string{}, envelope.Payload.EntityIDs)
	assert.Equal(t, []string{}, envelope.Payload.ObservationIDs)
}

func TestNotifyInsight_NilInsight(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	err := n.NotifyInsight(context.Background(), nil)
	require.NoError(t, err)

	// No message should be broadcast.
	assert.Empty(t, bc.getNotificationData())
}

func TestNotifyInsight_NilEntityAndObsIDs(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	insight := &domain.AIInsight{
		ID:             "ins-003",
		InsightType:    "pattern",
		SourceName:     "test_source",
		OperationName:  "test_op",
		Result:         map[string]any{"foo": "bar"},
		EntityIDs:      nil,
		ObservationIDs: nil,
		CreatedAt:      time.Now(),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	// Verify that nil slices are serialized as [] not null.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgs[0], &raw))

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["payload"], &payload))

	assert.Equal(t, "[]", string(payload["entity_ids"]))
	assert.Equal(t, "[]", string(payload["observation_ids"]))
}

func TestNotifyInsight_CreatedAtUTC(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	// Use a non-UTC timezone to verify normalization.
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	insight := &domain.AIInsight{
		ID:            "ins-004",
		InsightType:   "anomaly",
		SourceName:    "src",
		OperationName: "op",
		Result:        map[string]any{},
		CreatedAt:     time.Date(2026, 3, 20, 10, 0, 0, 0, loc), // EDT = UTC-4
	}

	require.NoError(t, n.NotifyInsight(context.Background(), insight))

	var envelope struct {
		Payload InsightPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(bc.getNotificationData()[0], &envelope))

	// 10:00 EDT = 14:00 UTC.
	assert.Equal(t, "2026-03-20T14:00:00Z", envelope.Payload.CreatedAt)
}

// ---------------------------------------------------------------------------
// NotifyEnrichment tests
// ---------------------------------------------------------------------------

func TestNotifyEnrichment_FullPayload(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	err := n.NotifyEnrichment(
		context.Background(),
		"ent-001",
		"flight_route_enrichment",
		"adsb_lol_flights",
		[]string{"ai_airline", "ai_route", "ai_flight_phase"},
	)
	require.NoError(t, err)

	msgs := bc.getMessages()
	require.Len(t, msgs, 1)

	var envelope struct {
		Type    string            `json:"type"`
		Payload EnrichmentPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msgs[0], &envelope))

	assert.Equal(t, WSTypeAIEnrichment, envelope.Type)
	assert.Equal(t, "ent-001", envelope.Payload.EntityID)
	assert.Equal(t, "flight_route_enrichment", envelope.Payload.OperationName)
	assert.Equal(t, "adsb_lol_flights", envelope.Payload.SourceName)
	assert.Equal(t, []string{"ai_airline", "ai_route", "ai_flight_phase"}, envelope.Payload.EnrichedFields)
}

func TestNotifyEnrichment_NilFields(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	err := n.NotifyEnrichment(context.Background(), "ent-002", "op", "src", nil)
	require.NoError(t, err)

	msgs := bc.getMessages()
	require.Len(t, msgs, 1)

	// enriched_fields should be [] not null.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgs[0], &raw))

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["payload"], &payload))

	assert.Equal(t, "[]", string(payload["enriched_fields"]))
}

func TestNotifyEnrichment_EmptyFields(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	err := n.NotifyEnrichment(context.Background(), "ent-003", "op", "src", []string{})
	require.NoError(t, err)

	var envelope struct {
		Payload EnrichmentPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(bc.getMessages()[0], &envelope))

	assert.Equal(t, []string{}, envelope.Payload.EnrichedFields)
}

// ---------------------------------------------------------------------------
// JSON serialization conformance
// ---------------------------------------------------------------------------

func TestInsightMessage_MatchesSpecFormat(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	layerType := "flights_commercial"
	insight := &domain.AIInsight{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		InsightType:    "anomaly",
		SourceName:     "flight_anomaly_detection",
		OperationName:  "flight_anomaly_scan",
		LayerType:      &layerType,
		Result:         map[string]any{"anomaly_type": "altitude_drop"},
		EntityIDs:      []string{"660e8400-e29b-41d4-a716-446655440000"},
		ObservationIDs: []string{},
		CreatedAt:      time.Date(2026, 3, 20, 14, 32, 0, 0, time.UTC),
	}

	require.NoError(t, n.NotifyInsight(context.Background(), insight))

	// Parse as generic map to verify field names match spec exactly.
	var raw map[string]any
	require.NoError(t, json.Unmarshal(bc.getNotificationData()[0], &raw))

	assert.Equal(t, "ai_insight", raw["type"])

	payload, ok := raw["payload"].(map[string]any)
	require.True(t, ok)

	// All required fields present per spec Section 10.2.
	expectedFields := []string{"id", "insight_type", "source_name", "operation_name", "layer_type", "result", "entity_ids", "entities", "observation_ids", "created_at"}
	for _, field := range expectedFields {
		_, exists := payload[field]
		assert.True(t, exists, "missing field: %s", field)
	}
}

func TestEnrichmentMessage_MatchesSpecFormat(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	require.NoError(t, n.NotifyEnrichment(
		context.Background(),
		"660e8400-e29b-41d4-a716-446655440000",
		"flight_route_enrichment",
		"adsb_lol_flights",
		[]string{"ai_airline", "ai_route", "ai_flight_phase"},
	))

	var raw map[string]any
	require.NoError(t, json.Unmarshal(bc.getMessages()[0], &raw))

	assert.Equal(t, "ai_enrichment", raw["type"])

	payload, ok := raw["payload"].(map[string]any)
	require.True(t, ok)

	expectedFields := []string{"entity_id", "operation_name", "enriched_fields", "source_name"}
	for _, field := range expectedFields {
		_, exists := payload[field]
		assert.True(t, exists, "missing field: %s", field)
	}
}

// ---------------------------------------------------------------------------
// Concurrency safety
// ---------------------------------------------------------------------------

func TestWSNotifier_ConcurrentCalls(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			insight := &domain.AIInsight{
				ID:            fmt.Sprintf("ins-%d", idx),
				InsightType:   "anomaly",
				SourceName:    "src",
				OperationName: "op",
				Result:        map[string]any{},
				CreatedAt:     time.Now(),
			}
			_ = n.NotifyInsight(context.Background(), insight)
		}(i)

		go func(idx int) {
			defer wg.Done()
			_ = n.NotifyEnrichment(context.Background(), fmt.Sprintf("ent-%d", idx), "op", "src", []string{"f1"})
		}(i)
	}

	wg.Wait()

	// Enrichment calls go through BroadcastRaw; insight calls through BroadcastNotification.
	assert.Len(t, bc.getMessages(), goroutines)
	assert.Len(t, bc.getNotificationData(), goroutines)
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestWSTypeConstants(t *testing.T) {
	assert.Equal(t, "ai_insight", WSTypeAIInsight)
	assert.Equal(t, "ai_enrichment", WSTypeAIEnrichment)
}

// ---------------------------------------------------------------------------
// Attention field handling
// ---------------------------------------------------------------------------

// strPtr returns a pointer to the given string value.
func strPtr(s string) *string {
	return &s
}

func TestNotifyInsight_WithAttention(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	insight := &domain.AIInsight{
		ID:            "ins-attn-001",
		InsightType:   "anomaly",
		SourceName:    "threat_detector",
		OperationName: "threat_scan",
		Attention:     strPtr("high"),
		Result:        map[string]any{"threat": "incoming"},
		CreatedAt:     time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	var envelope struct {
		Payload InsightPayload `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(msgs[0], &envelope))

	assert.Equal(t, "high", envelope.Payload.Attention)
}

func TestNotifyInsight_WithNilAttention(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	insight := &domain.AIInsight{
		ID:            "ins-attn-002",
		InsightType:   "summary",
		SourceName:    "summarizer",
		OperationName: "daily_summary",
		Attention:     nil,
		Result:        map[string]any{"text": "all clear"},
		CreatedAt:     time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	// Parse the raw payload map to verify the "attention" key is absent (omitempty).
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgs[0], &raw))

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["payload"], &payload))

	_, exists := payload["attention"]
	assert.False(t, exists, "attention key must be omitted from JSON when nil")
}

func TestNotifyInsight_AttentionAllLevels(t *testing.T) {
	levels := []string{"info", "low", "medium", "high", "critical"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			bc := &mockBroadcaster{}
			n := NewWSNotifier(bc, zerolog.Nop())

			insight := &domain.AIInsight{
				ID:            fmt.Sprintf("ins-attn-%s", level),
				InsightType:   "anomaly",
				SourceName:    "threat_detector",
				OperationName: "threat_scan",
				Attention:     strPtr(level),
				Result:        map[string]any{},
				CreatedAt:     time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			}

			err := n.NotifyInsight(context.Background(), insight)
			require.NoError(t, err)

			msgs := bc.getNotificationData()
			require.Len(t, msgs, 1)

			var envelope struct {
				Payload InsightPayload `json:"payload"`
			}
			require.NoError(t, json.Unmarshal(msgs[0], &envelope))

			assert.Equal(t, level, envelope.Payload.Attention)
		})
	}
}

func TestNotifyInsight_AttentionFieldSpecConformance(t *testing.T) {
	bc := &mockBroadcaster{}
	n := NewWSNotifier(bc, zerolog.Nop())

	insight := &domain.AIInsight{
		ID:            "ins-attn-spec",
		InsightType:   "anomaly",
		SourceName:    "threat_detector",
		OperationName: "threat_scan",
		Attention:     strPtr("critical"),
		Result:        map[string]any{"severity": "max"},
		CreatedAt:     time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := bc.getNotificationData()
	require.Len(t, msgs, 1)

	// Unmarshal into a raw map to confirm the spec-mandated JSON key name "attention".
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msgs[0], &raw))

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["payload"], &payload))

	attnRaw, exists := payload["attention"]
	require.True(t, exists, "attention key must be present in JSON payload")

	var attnValue string
	require.NoError(t, json.Unmarshal(attnRaw, &attnValue))
	assert.Equal(t, "critical", attnValue)
}

func TestNotifyInsight_CallsBroadcastNotification(t *testing.T) {
	mb := &mockBroadcaster{}
	n := NewWSNotifier(mb, zerolog.Nop())

	attention := "high"
	layerType := "flights_military"
	insight := &domain.AIInsight{
		ID:            "test-1",
		InsightType:   "geopolitical_intel",
		SourceName:    "test",
		OperationName: "test_op",
		LayerType:     &layerType,
		Attention:     &attention,
		Result:        map[string]any{"title": "Test"},
		CreatedAt:     time.Now(),
	}

	err := n.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	mb.mu.Lock()
	defer mb.mu.Unlock()

	require.Len(t, mb.notificationMsgs, 1, "expected 1 notification broadcast")

	meta := mb.notificationMsgs[0].Meta
	assert.Equal(t, "geopolitical_intel", meta.InsightType)
	assert.Equal(t, "high", meta.Attention)
	assert.Equal(t, "flights_military", meta.LayerType)

	// Should NOT have called BroadcastRaw (old path)
	assert.Empty(t, mb.messages, "BroadcastRaw should not be called for insights")
}
