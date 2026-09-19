package inproc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inproc"
)

func TestPublishConsume_Basic(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)
	sub := inproc.NewConsumer(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received [][]byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "test.subject"}, func(msg domain.ConsumedMessage) {
			mu.Lock()
			received = append(received, msg.Data())
			mu.Unlock()
		})
	}()

	// Give the goroutine time to subscribe.
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, pub.Publish(context.Background(), "test.subject", []byte("hello")))
	require.NoError(t, pub.Publish(context.Background(), "test.subject", []byte("world")))

	// Allow messages to propagate.
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, 2)
	assert.Equal(t, []byte("hello"), received[0])
	assert.Equal(t, []byte("world"), received[1])
}

func TestPublishConsume_MultipleSubscribers(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)
	sub1 := inproc.NewConsumer(bus)
	sub2 := inproc.NewConsumer(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var count1, count2 int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	startConsumer := func(sub domain.StreamConsumer, counter *int) {
		go func() {
			defer wg.Done()
			_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "events"}, func(msg domain.ConsumedMessage) {
				mu.Lock()
				*counter++
				mu.Unlock()
			})
		}()
	}

	startConsumer(sub1, &count1)
	startConsumer(sub2, &count2)
	time.Sleep(10 * time.Millisecond)

	require.NoError(t, pub.Publish(context.Background(), "events", []byte("ping")))
	time.Sleep(20 * time.Millisecond)

	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, count1)
	assert.Equal(t, 1, count2)
}

func TestPublishConsume_SubjectIsolation(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)
	sub := inproc.NewConsumer(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received [][]byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "topic.a"}, func(msg domain.ConsumedMessage) {
			mu.Lock()
			received = append(received, msg.Data())
			mu.Unlock()
		})
	}()

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, pub.Publish(context.Background(), "topic.a", []byte("for-a")))
	require.NoError(t, pub.Publish(context.Background(), "topic.b", []byte("for-b")))

	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, received, 1)
	assert.Equal(t, []byte("for-a"), received[0])
}

func TestPublishConsume_NoSubscribers(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)

	// Publishing with no subscribers must not panic or block.
	err := pub.Publish(context.Background(), "orphan.subject", []byte("ignored"))
	assert.NoError(t, err)
}

func TestPublishConsume_ContextCancelStopsConsumer(t *testing.T) {
	bus := inproc.NewBus(16)
	sub := inproc.NewConsumer(bus)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "x"}, func(msg domain.ConsumedMessage) {})
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("consumer did not stop after context cancellation")
	}
}

func TestSubjectMatches(t *testing.T) {
	cases := []struct {
		filter, subject string
		want            bool
	}{
		{"respondent.ai.enrich.>", "respondent.ai.enrich.adsb_military", true},
		{"respondent.ai.enrich.>", "respondent.ai.enrich.a.b", true},
		{"respondent.ai.enrich.>", "respondent.ai.enrich", false}, // ">" needs >=1 token
		{"respondent.ai.enrich.>", "respondent.ai.other.x", false},
		{"a.*.c", "a.b.c", true},
		{"a.*.c", "a.b.d", false},
		{"a.*.c", "a.b.c.d", false},
		{"exact.subject", "exact.subject", true},
		{"exact.subject", "exact.other", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, inproc.SubjectMatches(c.filter, c.subject), "%s ~ %s", c.filter, c.subject)
	}
}

func TestPublishConsume_WildcardDelivery(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)
	sub := inproc.NewConsumer(bus)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received [][]byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "respondent.ai.enrich.>"}, func(m domain.ConsumedMessage) {
			mu.Lock()
			received = append(received, m.Data())
			mu.Unlock()
		})
	}()
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, pub.Publish(context.Background(), "respondent.ai.enrich.adsb_military", []byte("job")))
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	assert.Equal(t, []byte("job"), received[0])
}

func TestInprocMessage_AckNakAreNoops(t *testing.T) {
	bus := inproc.NewBus(16)
	pub := inproc.NewPublisher(bus)
	sub := inproc.NewConsumer(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_ = sub.Consume(ctx, domain.StreamConsumerConfig{FilterSubject: "noop"}, func(msg domain.ConsumedMessage) {
			errCh <- msg.Ack()
			errCh <- msg.Nak()
			errCh <- msg.NakWithDelay(time.Second)
		})
	}()

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, pub.Publish(context.Background(), "noop", []byte("test")))
	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	close(errCh)
	for err := range errCh {
		assert.NoError(t, err)
	}
}
