package declarative

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// ErrNotPullBased is returned by StreamTransport.Fetch() since streaming transports don't support pull-based fetching.
var ErrNotPullBased = errors.New("transport is not pull-based")

// Transport abstracts data fetching from a source.
// Implementations handle protocol-specific concerns (HTTP polling, WebSocket, etc.)
type Transport interface {
	// Fetch retrieves raw data from the given URL using the specified method.
	// Returns the response body, HTTP status code (or 0 for non-HTTP transports), and any error.
	Fetch(ctx context.Context, method, url string) ([]byte, int, error)
}

// StreamTransport extends Transport for push-based, long-lived connections.
// Implementations handle protocols like WebSocket, SSE, MQTT, gRPC streaming, etc.
type StreamTransport interface {
	Transport
	Connect(ctx context.Context) error
	Recv(ctx context.Context) ([]byte, error)
	Close() error
}

// ListenTransport extends Transport for inbound/server-side transports (webhooks).
// The transport starts an HTTP server or similar listener and pushes payloads to the channel.
type ListenTransport interface {
	Transport
	Listen(ctx context.Context, payloads chan<- []byte) error
	Close() error
}

// HTTPTransport implements Transport for HTTP polling sources.
// It handles retries with backoff, response size limiting, and request headers.
type HTTPTransport struct {
	mu               sync.RWMutex // guards headers and tokenProvider for hot-reload
	client           *http.Client
	headers          map[string]string
	tokenProvider    TokenProvider
	retry            *RetrySpec
	maxResponseBytes int64
	sourceName       string
	logger           *logging.Logger

	// Optional DNS SRV origin discovery. When set, Fetch rewrites the origin
	// of every request URL to a discovered mirror, keeping the declared path
	// and query. mirrorIdx/mirrorPinned are guarded by mu.
	discovery    *endpointResolver
	mirrorIdx    int
	mirrorPinned bool
}

// HTTPTransportConfig holds configuration for creating an HTTPTransport.
type HTTPTransportConfig struct {
	Client           *http.Client
	Headers          map[string]string
	TokenProvider    TokenProvider
	Retry            *RetrySpec
	MaxResponseBytes int64
	SourceName       string
	Logger           *logging.Logger
	Discovery        *endpointResolver
}

// NewHTTPTransport creates a new HTTP transport from the given configuration.
func NewHTTPTransport(cfg HTTPTransportConfig) *HTTPTransport {
	return &HTTPTransport{
		client:           cfg.Client,
		headers:          cfg.Headers,
		tokenProvider:    cfg.TokenProvider,
		retry:            cfg.Retry,
		maxResponseBytes: cfg.MaxResponseBytes,
		sourceName:       cfg.SourceName,
		logger:           cfg.Logger,
		discovery:        cfg.Discovery,
	}
}

// Fetch performs an HTTP request with retry logic to the given URL.
// When origin discovery is configured, the request is routed to a discovered
// mirror; see fetchDiscovered for the mirror selection rules.
func (t *HTTPTransport) Fetch(ctx context.Context, method, url string) ([]byte, int, error) {
	// Expand {date:...} rolling-window macros once per fetch (before retries) so a
	// static source URL can target a relative date range (e.g. GFW events).
	url = expandDateMacros(url, time.Now())

	if t.discovery != nil {
		return t.fetchDiscovered(ctx, method, url)
	}
	return t.fetchURL(ctx, method, url)
}

// fetchDiscovered routes one request through the discovered mirror list.
//
// Mirror selection is deliberately sticky: once a mirror answers, every later
// request in the same refresh goes to that same mirror, so a paginated catalog
// is never stitched together from two directory snapshots. When the pinned
// mirror fails the error is returned to the caller rather than silently
// retried elsewhere — the caller decides whether to restart the refresh from
// page zero on the next mirror.
func (t *HTTPTransport) fetchDiscovered(ctx context.Context, method, rawURL string) ([]byte, int, error) {
	origins, err := t.discovery.Origins(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("discover origins for source %q: %w", t.sourceName, err)
	}

	t.mu.RLock()
	idx, pinned := t.mirrorIdx, t.mirrorPinned
	t.mu.RUnlock()

	if idx >= len(origins) {
		return nil, 0, fmt.Errorf("source %q: all %d discovered mirrors exhausted", t.sourceName, len(origins))
	}

	last := len(origins) - 1
	if pinned {
		last = idx
	}

	var lastErr error
	var lastStatus int
	for i := idx; i <= last; i++ {
		pageURL, err := rewriteOrigin(rawURL, origins[i])
		if err != nil {
			return nil, 0, err
		}

		body, status, err := t.fetchURL(ctx, method, pageURL)
		if err == nil {
			t.setMirror(i, true)
			return body, status, nil
		}
		if ctx.Err() != nil {
			return nil, status, err
		}

		lastErr, lastStatus = err, status
		// Advance past the failed mirror so the next attempt starts on the
		// following one instead of re-trying a mirror already known to be down.
		t.setMirror(i+1, false)
		t.logger.Warn("discovered mirror failed",
			logging.String("source_name", t.sourceName),
			logging.String("origin", origins[i]),
			logging.Err("error", err),
		)
	}

	return nil, lastStatus, fmt.Errorf("source %q: discovered mirror request failed: %w", t.sourceName, lastErr)
}

// setMirror records which discovered mirror the next request should use.
func (t *HTTPTransport) setMirror(idx int, pinned bool) {
	t.mu.Lock()
	t.mirrorIdx, t.mirrorPinned = idx, pinned
	t.mu.Unlock()
}

// resetMirrors returns mirror selection to the highest-priority origin. The
// adapter calls it once per refresh cycle, never between pagination attempts.
func (t *HTTPTransport) resetMirrors() {
	t.setMirror(0, false)
}

// hasDiscovery reports whether this transport selects origins dynamically.
func (t *HTTPTransport) hasDiscovery() bool { return t != nil && t.discovery != nil }

// fetchURL performs an HTTP request with retry logic to one fully-resolved URL.
func (t *HTTPTransport) fetchURL(ctx context.Context, method, url string) ([]byte, int, error) {
	maxAttempts := 1
	if t.retry != nil && t.retry.MaxAttempts > 0 {
		maxAttempts = t.retry.MaxAttempts + 1 // MaxAttempts is retries, not total attempts
	}

	// Snapshot mutable fields under read lock so hot-reload doesn't race with inflight requests.
	t.mu.RLock()
	headers := t.headers
	tp := t.tokenProvider
	t.mu.RUnlock()

	var lastErr error
	var lastStatus int

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := calculateBackoff(t.retry, attempt)
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, 0, fmt.Errorf("create request: %w", err)
		}

		// Static headers (Accept, User-Agent, etc.)
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// Dynamic auth header (may trigger OAuth2 token refresh)
		if tp != nil {
			hName, hValue, err := tp.GetHeader(ctx)
			if err != nil {
				return nil, 0, fmt.Errorf("resolve auth token: %w", err)
			}
			if hName != "" {
				req.Header.Set(hName, hValue)
			}
		}

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = err
			t.logger.Warn("HTTP request failed",
				logging.String("source_name", t.sourceName),
				logging.Int("attempt", attempt+1),
				logging.Err("error", err),
			)
			continue
		}

		lastStatus = resp.StatusCode

		body, readErr := func() ([]byte, error) {
			defer func() { _ = resp.Body.Close() }()
			return t.readResponseBody(resp, t.maxResponseBytes)
		}()

		if readErr != nil {
			lastErr = fmt.Errorf("read response body: %w", readErr)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, resp.StatusCode, nil
		}

		// Retryable status codes
		if resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			t.logger.Warn("retryable HTTP error",
				logging.String("source_name", t.sourceName),
				logging.Int("status", resp.StatusCode),
				logging.Int("attempt", attempt+1),
			)
			continue
		}

		// Non-retryable error
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return nil, lastStatus, fmt.Errorf("all %d attempts failed for source %q: %w", maxAttempts, t.sourceName, lastErr)
}

// readResponseBody reads a response body with size limit.
func (t *HTTPTransport) readResponseBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxResponseBytes
	}
	limitedReader := io.LimitReader(resp.Body, maxBytes)
	return io.ReadAll(limitedReader)
}

// UpdateHeaders replaces the transport's static request headers.
// This supports hot-reload of compiled sources where resolved headers may change.
func (t *HTTPTransport) UpdateHeaders(headers map[string]string) {
	t.mu.Lock()
	t.headers = headers
	t.mu.Unlock()
}

// UpdateTokenProvider replaces the transport's token provider.
// This supports hot-reload of compiled sources where the auth config may change.
func (t *HTTPTransport) UpdateTokenProvider(tp TokenProvider) {
	t.mu.Lock()
	t.tokenProvider = tp
	t.mu.Unlock()
}
