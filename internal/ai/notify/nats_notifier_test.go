package notify

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
)

// mockPublisher captures published messages.
type mockPublisher struct {
	mu       sync.Mutex
	messages []publishedMsg
}

type publishedMsg struct {
	Subject string
	Data    []byte
}

func (m *mockPublisher) Publish(_ context.Context, subject string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, publishedMsg{Subject: subject, Data: data})
	return nil
}

func (m *mockPublisher) getMessages() []publishedMsg {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]publishedMsg, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func TestNATSNotifier_NotifyInsight(t *testing.T) {
	pub := &mockPublisher{}
	logger := zerolog.Nop()
	notifier := NewNATSNotifier(pub, "respondent.notifications", logger)

	now := time.Now().UTC()
	insight := &domain.AIInsight{
		ID:            "insight-1",
		InsightType:   "anomaly",
		SourceName:    "opensky",
		OperationName: "detect_anomalies",
		Result:        map[string]any{"score": 0.95},
		EntityIDs:     []string{"e-1"},
		Entities: []domain.InsightEntityRef{
			{ID: "e-1", ExternalID: "ext-1", Name: "Entity 1", LayerType: "flights"},
		},
		CreatedAt: now,
	}

	err := notifier.NotifyInsight(context.Background(), insight)
	require.NoError(t, err)

	msgs := pub.getMessages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "respondent.notifications.ai_insight", msgs[0].Subject)

	var msg wsMessage
	require.NoError(t, json.Unmarshal(msgs[0].Data, &msg))
	assert.Equal(t, WSTypeAIInsight, msg.Type)
}

func TestNATSNotifier_NotifyEnrichment(t *testing.T) {
	pub := &mockPublisher{}
	logger := zerolog.Nop()
	notifier := NewNATSNotifier(pub, "respondent.notifications", logger)

	err := notifier.NotifyEnrichment(context.Background(), "entity-1", "classify", "opensky", []string{"category", "description"})
	require.NoError(t, err)

	msgs := pub.getMessages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "respondent.notifications.ai_enrichment", msgs[0].Subject)

	var msg wsMessage
	require.NoError(t, json.Unmarshal(msgs[0].Data, &msg))
	assert.Equal(t, WSTypeAIEnrichment, msg.Type)
}

func TestNATSNotifier_NilInsightNoOp(t *testing.T) {
	pub := &mockPublisher{}
	logger := zerolog.Nop()
	notifier := NewNATSNotifier(pub, "respondent.notifications", logger)

	err := notifier.NotifyInsight(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, pub.getMessages())
}
