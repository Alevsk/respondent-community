package declarative

import (
	"context"
	"fmt"

	"github.com/Alevsk/respondent/internal/logging"
)

// startStreaming runs the streaming loop for StreamTransport implementations.
func (a *DeclarativeAdapter) startStreaming(ctx context.Context, st StreamTransport) error {
	cs := a.compiled.Load()
	if cs == nil {
		return fmt.Errorf("no compiled source loaded for %q", a.name)
	}
	def := cs.Definition()

	batcher := NewBatcher(def.Transport.Batching)

	// Start batcher in background
	go batcher.Run(ctx)

	// Start batch processing in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case batch, ok := <-batcher.Batches():
				if !ok {
					return
				}
				if err := a.processBatch(batch); err != nil {
					a.logger.Warn("batch processing error",
						logging.String("source_name", a.name),
						logging.Err("error", err),
					)
				}
			}
		}
	}()

	// Run connect -> recv loop with reconnection
	err := reconnectLoop(ctx, def.Transport.Reconnect, a.name, a.logger,
		func(ctx context.Context) error {
			return st.Connect(ctx)
		},
		func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				msg, err := st.Recv(ctx)
				if err != nil {
					return err
				}

				batcher.Add(msg)
			}
		},
	)

	batcher.Stop()
	_ = st.Close()

	return err
}

// startListening runs the listener loop for ListenTransport implementations.
func (a *DeclarativeAdapter) startListening(ctx context.Context, lt ListenTransport) error {
	cs := a.compiled.Load()
	if cs == nil {
		return fmt.Errorf("no compiled source loaded for %q", a.name)
	}
	def := cs.Definition()

	batcher := NewBatcher(def.Transport.Batching)

	// Start batcher in background
	go batcher.Run(ctx)

	// Start batch processing in background
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case batch, ok := <-batcher.Batches():
				if !ok {
					return
				}
				if err := a.processBatch(batch); err != nil {
					a.logger.Warn("batch processing error",
						logging.String("source_name", a.name),
						logging.Err("error", err),
					)
				}
			}
		}
	}()

	// Create payload channel and start listener
	payloads := make(chan []byte, 256)

	// Forward payloads to batcher
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-payloads:
				if !ok {
					return
				}
				batcher.Add(msg)
			}
		}
	}()

	err := lt.Listen(ctx, payloads)
	batcher.Stop()
	_ = lt.Close()
	return err
}

// processBatch processes a batch of raw messages through the parser -> CEL pipeline.
func (a *DeclarativeAdapter) processBatch(messages [][]byte) error {
	cs := a.compiled.Load()
	if cs == nil {
		return fmt.Errorf("no compiled source loaded for %q", a.name)
	}
	def := cs.Definition()

	var allRecords []map[string]interface{}

	for _, msg := range messages {
		records, err := a.parseBody(def, msg)
		if err != nil {
			a.logger.Warn("message parse error in batch, skipping",
				logging.String("source_name", a.name),
				logging.Err("error", err),
			)
			continue
		}
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		return nil
	}

	entities, observations := a.processRecords(cs, allRecords)

	if len(entities) > 0 {
		a.SetEntities(entities, observations)

		a.logger.Debug("processed streaming batch",
			logging.String("source_name", a.name),
			logging.Int("messages", len(messages)),
			logging.Int("records", len(allRecords)),
			logging.Int("entities", len(entities)),
		)
	}

	return nil
}
