package declarative

import (
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestAMQPTransport_Close_DoneAlreadyClosed(t *testing.T) {
	done := make(chan struct{})
	close(done) // Pre-close the done channel.

	transport := &AMQPTransport{
		spec:       &AMQPSpec{URL: "amqp://localhost:5672/"},
		sourceName: "test_amqp_close",
		logger:     logging.NewNopLogger(),
		msgCh:      make(chan []byte, 1),
		done:       done,
	}

	// Close should succeed; the select hits the `<-t.done` case (already closed).
	err := transport.Close()
	if err != nil {
		t.Errorf("Close: %v", err)
	}
}
