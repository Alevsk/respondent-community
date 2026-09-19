package declarative

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Alevsk/respondent/internal/logging"
)

func init() {
	RegisterTransport("sse", newSSETransport)
}

// SSETransport implements StreamTransport for Server-Sent Events streams.
// SSE spec: https://html.spec.whatwg.org/multipage/server-sent-events.html
type SSETransport struct {
	url           string
	headers       map[string]string
	tokenProvider TokenProvider
	spec          *SSESpec
	sourceName    string
	logger        *logging.Logger

	mu          sync.Mutex
	resp        *http.Response
	scanner     *bufio.Scanner
	lastEventID string
	client      *http.Client
}

// newSSETransport constructs an SSETransport from a source definition.
func newSSETransport(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, _ EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.URL == "" {
		return nil, fmt.Errorf("sse transport requires a URL")
	}

	sseSpec := def.Transport.SSE
	if sseSpec == nil {
		sseSpec = &SSESpec{}
	}

	headers := make(map[string]string, len(resolvedHeaders))
	for k, v := range resolvedHeaders {
		headers[k] = v
	}

	return &SSETransport{
		url:           def.Transport.URL,
		headers:       headers,
		tokenProvider: tokenProvider,
		spec:          sseSpec,
		sourceName:    def.Name,
		logger:        logger,
		client: &http.Client{
			// No timeout on the client — SSE streams are long-lived.
			// The connect timeout is handled via context on the request.
			Timeout: 0,
		},
	}, nil
}

// Fetch returns ErrNotPullBased because SSE is a push-based transport.
func (t *SSETransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect establishes the SSE connection to the configured URL.
// If called after a previous connection, the old response body is closed first.
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close any previous connection.
	if t.resp != nil {
		_ = t.resp.Body.Close()
		t.resp = nil
		t.scanner = nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Set Last-Event-ID header if enabled and we have a stored ID.
	if t.spec.LastEventID && t.lastEventID != "" {
		req.Header.Set("Last-Event-ID", t.lastEventID)
	}

	// Apply custom headers.
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	// Dynamic auth header (may trigger OAuth2 token refresh on reconnect).
	if t.tokenProvider != nil {
		hName, hValue, err := t.tokenProvider.GetHeader(ctx)
		if err != nil {
			return fmt.Errorf("resolve auth token for SSE: %w", err)
		}
		if hName != "" {
			req.Header.Set(hName, hValue)
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("SSE connect: %w", err)
	}

	// Verify Content-Type.
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		_ = resp.Body.Close()
		return fmt.Errorf("unexpected Content-Type %q, expected text/event-stream", ct)
	}

	t.resp = resp
	t.scanner = bufio.NewScanner(resp.Body)

	t.logger.Info("SSE connected",
		logging.String("source_name", t.sourceName),
		logging.String("url", t.url),
	)

	return nil
}

// Recv blocks until the next complete SSE event is available and returns its data payload.
// It parses SSE events according to the spec, handling multi-line data, event types,
// IDs, comments, and retry hints. Returns an error on stream end or read failure.
func (t *SSETransport) Recv(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	scanner := t.scanner
	t.mu.Unlock()

	if scanner == nil {
		return nil, fmt.Errorf("SSE transport not connected")
	}

	// Build the event filter set for fast lookup.
	filterSet := make(map[string]struct{}, len(t.spec.EventFilter))
	for _, ef := range t.spec.EventFilter {
		filterSet[ef] = struct{}{}
	}

	for {
		// Check context before blocking on scan.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		data, eventType, eventID, ok := t.readEvent(scanner)
		if !ok {
			// Scanner finished — stream closed or error.
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("SSE read error: %w", err)
			}
			return nil, fmt.Errorf("SSE stream closed")
		}

		// Store event ID if last-event-id tracking is enabled.
		if t.spec.LastEventID && eventID != "" {
			t.mu.Lock()
			t.lastEventID = eventID
			t.mu.Unlock()
		}

		// Skip events with no data.
		if data == "" {
			continue
		}

		// Apply event type filter if configured.
		if len(filterSet) > 0 {
			if _, match := filterSet[eventType]; !match {
				continue
			}
		}

		return []byte(data), nil
	}
}

// readEvent reads lines from the scanner until a complete SSE event is assembled
// (terminated by an empty line). Returns the concatenated data, event type, event ID,
// and whether a valid event was read (false on EOF/error).
func (t *SSETransport) readEvent(scanner *bufio.Scanner) (data, eventType, eventID string, ok bool) {
	var dataLines []string
	hasFields := false

	for scanner.Scan() {
		line := scanner.Text()

		// Remove \r if present (handles \r\n line endings since Scanner splits on \n).
		line = strings.TrimRight(line, "\r")

		// Empty line signals end of event.
		if line == "" {
			if hasFields {
				data = strings.Join(dataLines, "\n")
				ok = true
				return
			}
			// Empty line with no fields — skip.
			continue
		}

		// Comment line — skip.
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field:value.
		field, value := parseSSELine(line)

		switch field {
		case "data":
			dataLines = append(dataLines, value)
			hasFields = true
		case "event":
			eventType = value
			hasFields = true
		case "id":
			// Per spec, ignore id fields containing null.
			if !strings.Contains(value, "\x00") {
				eventID = value
				hasFields = true
			}
		case "retry":
			// Store but don't act on retry hints.
			hasFields = true
		default:
			// Unknown fields are ignored per spec.
		}
	}

	// Scanner stopped (EOF or error) — if we have accumulated fields, emit the event.
	if hasFields {
		data = strings.Join(dataLines, "\n")
		ok = true
	}
	return
}

// parseSSELine splits an SSE line into field name and value.
// Per the spec: if the line contains ":", the field is the part before the first ":"
// and the value is the part after (with a leading space stripped if present).
// If there is no ":", the entire line is the field name with an empty value.
func parseSSELine(line string) (field, value string) {
	var ok bool
	field, value, ok = strings.Cut(line, ":")
	if !ok {
		return line, ""
	}
	// Strip single leading space per SSE spec.
	value = strings.TrimPrefix(value, " ")
	return
}

// Close tears down the SSE connection by closing the response body.
// Safe to call multiple times.
func (t *SSETransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.resp != nil {
		err := t.resp.Body.Close()
		t.resp = nil
		t.scanner = nil
		return err
	}
	return nil
}
