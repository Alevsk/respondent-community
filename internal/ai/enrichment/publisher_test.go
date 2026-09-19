package enrichment

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSubjectPrefix = "respondent.ai.enrich"

// ---------------------------------------------------------------------------
// capturePublisher — minimal domain.MessagePublisher that records every call
// ---------------------------------------------------------------------------

// capturedMsg holds one published envelope.
type capturedMsg struct {
	subject string
	data    []byte
}

// capturePublisher implements domain.MessagePublisher and records all Publish
// calls. This removes the need for a real NATS/inproc bus when the tests only
// care about what the enrichment.Publisher sends, not about routing or delivery.
type capturePublisher struct {
	mu   sync.Mutex
	msgs []capturedMsg
}

func (c *capturePublisher) Publish(_ context.Context, subject string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	dst := make([]byte, len(data))
	copy(dst, data)
	c.msgs = append(c.msgs, capturedMsg{subject: subject, data: dst})
	return nil
}

func (c *capturePublisher) get() []capturedMsg {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]capturedMsg, len(c.msgs))
	copy(cp, c.msgs)
	return cp
}

// newTestPublisher returns an enrichment.Publisher wired to a capturePublisher.
func newTestPublisher() (*Publisher, *capturePublisher) {
	cap := &capturePublisher{}
	pub := NewPublisher(cap, testSubjectPrefix)
	return pub, cap
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPublisher_Publish(t *testing.T) {
	pub, cap := newTestPublisher()

	job := Job{
		EntityID:    "ent-001",
		ExternalID:  "ext-001",
		SourceName:  "adsb",
		LayerType:   "aircraft",
		PublishedAt: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
	}

	err := pub.Publish(context.Background(), job)
	require.NoError(t, err)

	msgs := cap.get()
	require.Len(t, msgs, 1)

	var received Job
	err = json.Unmarshal(msgs[0].data, &received)
	require.NoError(t, err)

	assert.Equal(t, job.EntityID, received.EntityID)
	assert.Equal(t, job.ExternalID, received.ExternalID)
	assert.Equal(t, job.SourceName, received.SourceName)
	assert.Equal(t, job.LayerType, received.LayerType)
	assert.Equal(t, job.PublishedAt.UTC(), received.PublishedAt.UTC())
}

func TestPublisher_PublishBatch(t *testing.T) {
	pub, cap := newTestPublisher()

	jobs := []Job{
		{
			EntityID:    "ent-010",
			ExternalID:  "ext-010",
			SourceName:  "adsb",
			LayerType:   "aircraft",
			PublishedAt: time.Now(),
		},
		{
			EntityID:    "ent-011",
			ExternalID:  "ext-011",
			SourceName:  "ais",
			LayerType:   "vessel",
			PublishedAt: time.Now(),
		},
		{
			EntityID:    "ent-012",
			ExternalID:  "ext-012",
			SourceName:  "acled",
			LayerType:   "event",
			PublishedAt: time.Now(),
		},
	}

	err := pub.PublishBatch(context.Background(), jobs)
	require.NoError(t, err)

	msgs := cap.get()
	require.Len(t, msgs, 3)

	receivedIDs := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		var j Job
		err := json.Unmarshal(msg.data, &j)
		require.NoError(t, err)
		receivedIDs = append(receivedIDs, j.EntityID)
	}
	assert.ElementsMatch(t, []string{"ent-010", "ent-011", "ent-012"}, receivedIDs)
}

func TestPublisher_SubjectFormat(t *testing.T) {
	pub, cap := newTestPublisher()

	tests := []struct {
		name        string
		sourceName  string
		wantSubject string
	}{
		{
			name:        "adsb source",
			sourceName:  "adsb",
			wantSubject: testSubjectPrefix + ".adsb",
		},
		{
			name:        "ais source",
			sourceName:  "ais",
			wantSubject: testSubjectPrefix + ".ais",
		},
		{
			name:        "acled source",
			sourceName:  "acled",
			wantSubject: testSubjectPrefix + ".acled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{
				EntityID:    "ent-sub-001",
				SourceName:  tt.sourceName,
				LayerType:   "test",
				PublishedAt: time.Now(),
			}
			err := pub.Publish(context.Background(), job)
			require.NoError(t, err)
		})
	}

	msgs := cap.get()
	require.Len(t, msgs, 3)

	subjects := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		subjects = append(subjects, msg.subject)
	}

	for _, tt := range tests {
		assert.Contains(t, subjects, tt.wantSubject, "expected subject %s in received subjects", tt.wantSubject)
	}
}

func TestPublisher_SetsPublishedAt(t *testing.T) {
	pub, cap := newTestPublisher()

	before := time.Now()

	job := Job{
		EntityID:   "ent-time-001",
		SourceName: "adsb",
		LayerType:  "aircraft",
		// PublishedAt intentionally left as zero value.
	}

	err := pub.Publish(context.Background(), job)
	require.NoError(t, err)

	after := time.Now()

	msgs := cap.get()
	require.Len(t, msgs, 1)

	var received Job
	err = json.Unmarshal(msgs[0].data, &received)
	require.NoError(t, err)

	assert.False(t, received.PublishedAt.IsZero(), "PublishedAt must be set when originally zero")
	assert.True(t, received.PublishedAt.After(before) || received.PublishedAt.Equal(before),
		"PublishedAt (%v) must be >= before (%v)", received.PublishedAt, before)
	assert.True(t, received.PublishedAt.Before(after) || received.PublishedAt.Equal(after),
		"PublishedAt (%v) must be <= after (%v)", received.PublishedAt, after)
}

func TestPublisher_PreservesExplicitPublishedAt(t *testing.T) {
	pub, cap := newTestPublisher()

	explicit := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	job := Job{
		EntityID:    "ent-time-002",
		SourceName:  "adsb",
		LayerType:   "aircraft",
		PublishedAt: explicit,
	}

	err := pub.Publish(context.Background(), job)
	require.NoError(t, err)

	msgs := cap.get()
	require.Len(t, msgs, 1)

	var received Job
	err = json.Unmarshal(msgs[0].data, &received)
	require.NoError(t, err)

	assert.Equal(t, explicit.UTC(), received.PublishedAt.UTC(),
		"explicit PublishedAt must not be overwritten")
}

func TestPublisher_MarshalJSON(t *testing.T) {
	pub, cap := newTestPublisher()

	job := Job{
		EntityID:      "ent-json-001",
		ObservationID: "obs-json-001",
		ExternalID:    "ext-json-001",
		SourceName:    "adsb",
		LayerType:     "aircraft",
		PublishedAt:   time.Date(2025, 3, 20, 8, 0, 0, 0, time.UTC),
	}

	err := pub.Publish(context.Background(), job)
	require.NoError(t, err)

	msgs := cap.get()
	require.Len(t, msgs, 1)

	data := msgs[0].data

	// Verify valid JSON.
	assert.True(t, json.Valid(data), "message body must be valid JSON")

	// Verify all expected fields are present.
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	expectedFields := []string{
		"entity_id",
		"observation_id",
		"external_id",
		"source_name",
		"layer_type",
		"published_at",
	}
	for _, field := range expectedFields {
		assert.Contains(t, raw, field, "JSON must contain field %q", field)
	}

	assert.Equal(t, "ent-json-001", raw["entity_id"])
	assert.Equal(t, "obs-json-001", raw["observation_id"])
	assert.Equal(t, "ext-json-001", raw["external_id"])
	assert.Equal(t, "adsb", raw["source_name"])
	assert.Equal(t, "aircraft", raw["layer_type"])
}

func TestPublisher_PublishBatch_Empty(t *testing.T) {
	pub, cap := newTestPublisher()

	err := pub.PublishBatch(context.Background(), nil)
	require.NoError(t, err, "PublishBatch with nil slice must not error")

	err = pub.PublishBatch(context.Background(), []Job{})
	require.NoError(t, err, "PublishBatch with empty slice must not error")

	msgs := cap.get()
	assert.Empty(t, msgs, "no messages should be published for empty/nil input")
}
