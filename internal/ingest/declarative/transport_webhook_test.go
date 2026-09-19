package declarative

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// newTestWebhookTransport creates a WebhookTransport for testing.
// It picks a random available port and returns the transport plus the base URL.
func newTestWebhookTransport(t *testing.T, spec *WebhookSpec, secret string) (*WebhookTransport, string) {
	t.Helper()

	if spec.ListenAddr == "" {
		// Find a free port.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("find free port: %v", err)
		}
		spec.ListenAddr = ln.Addr().String()
		_ = ln.Close()
	}

	logger := logging.NewNopLogger()

	wt := &WebhookTransport{
		spec:       spec,
		sourceName: "test_webhook",
		logger:     logger,
		secret:     secret,
	}

	baseURL := fmt.Sprintf("http://%s%s", spec.ListenAddr, spec.Path)
	return wt, baseURL
}

// startWebhookListener starts the Listen goroutine and waits for the server to be ready.
func startWebhookListener(t *testing.T, wt *WebhookTransport, ctx context.Context, payloads chan []byte) {
	t.Helper()

	errCh := make(chan error, 1)
	go func() {
		errCh <- wt.Listen(ctx, payloads)
	}()

	// Wait for the server to be ready by attempting connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", wt.spec.ListenAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("webhook server did not start within 2s")
}

func TestWebhookTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	wt := &WebhookTransport{
		spec:       &WebhookSpec{ListenAddr: ":0", Path: "/hook"},
		sourceName: "test",
		logger:     logging.NewNopLogger(),
	}

	_, _, err := wt.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Fatalf("expected ErrNotPullBased, got %v", err)
	}
}

func TestWebhookTransport_ReceivePOST(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	// POST a payload.
	payload := []byte(`{"event":"test","data":123}`)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case received := <-payloads:
		if !bytes.Equal(received, payload) {
			t.Fatalf("payload mismatch: got %q, want %q", received, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestWebhookTransport_RejectNonPOST(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	resp, err := http.Get(baseURL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_HMACValidation_Valid(t *testing.T) {
	secret := "test-secret-key"
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	payload := []byte(`{"event":"signed"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	select {
	case received := <-payloads:
		if !bytes.Equal(received, payload) {
			t.Fatalf("payload mismatch: got %q, want %q", received, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestWebhookTransport_HMACValidation_Invalid(t *testing.T) {
	secret := "test-secret-key"
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	payload := []byte(`{"event":"bad-sig"}`)
	req, _ := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(payload))
	req.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_HMACValidation_Missing(t *testing.T) {
	secret := "test-secret-key"
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	// POST without the signature header.
	payload := []byte(`{"event":"no-sig"}`)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_HMACSha1(t *testing.T) {
	secret := "sha1-secret"
	spec := &WebhookSpec{
		Path:               "/webhook",
		SignatureHeader:    "X-Signature",
		SignatureAlgorithm: "sha1",
	}
	wt, baseURL := newTestWebhookTransport(t, spec, secret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	payload := []byte(`{"event":"sha1-test"}`)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	sig := "sha1=" + hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(payload))
	req.Header.Set("X-Signature", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	select {
	case received := <-payloads:
		if !bytes.Equal(received, payload) {
			t.Fatalf("payload mismatch: got %q, want %q", received, payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestWebhookTransport_AllowedIPs_Allowed(t *testing.T) {
	spec := &WebhookSpec{
		Path:       "/webhook",
		AllowedIPs: []string{"127.0.0.0/8"},
	}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	payload := []byte(`{"event":"allowed-ip"}`)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_AllowedIPs_Denied(t *testing.T) {
	spec := &WebhookSpec{
		Path:       "/webhook",
		AllowedIPs: []string{"10.0.0.0/8"},
	}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	// Request comes from 127.0.0.1 which is not in 10.0.0.0/8.
	payload := []byte(`{"event":"denied-ip"}`)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_MaxBodyBytes(t *testing.T) {
	spec := &WebhookSpec{
		Path:         "/webhook",
		MaxBodyBytes: 16, // very small limit
	}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	// Send a body larger than 16 bytes.
	payload := []byte(`{"event":"this payload is way too large for the limit"}`)
	resp, err := http.Post(baseURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestWebhookTransport_GracefulShutdown(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, _ := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())

	payloads := make(chan []byte, 10)
	errCh := make(chan error, 1)
	go func() {
		errCh <- wt.Listen(ctx, payloads)
	}()

	// Wait for server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", wt.spec.ListenAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel context to trigger shutdown.
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Listen returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for graceful shutdown")
	}
}

func TestWebhookTransport_CloseIdempotent(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, _ := newTestWebhookTransport(t, spec, "")

	// Close before Listen -- should be a no-op.
	if err := wt.Close(); err != nil {
		t.Fatalf("first Close() returned error: %v", err)
	}
	if err := wt.Close(); err != nil {
		t.Fatalf("second Close() returned error: %v", err)
	}

	// Start and then close multiple times.
	ctx, cancel := context.WithCancel(context.Background())
	payloads := make(chan []byte, 10)
	go func() {
		_ = wt.Listen(ctx, payloads)
	}()

	// Wait for server.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", wt.spec.ListenAddr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	// Close after shutdown -- should be safe.
	if err := wt.Close(); err != nil {
		t.Fatalf("Close() after shutdown returned error: %v", err)
	}
	if err := wt.Close(); err != nil {
		t.Fatalf("second Close() after shutdown returned error: %v", err)
	}
}

func TestCheckIPAllowed(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		allowedCIDRs []string
		want         bool
	}{
		{
			name:         "empty allowlist allows all",
			remoteAddr:   "192.168.1.1:12345",
			allowedCIDRs: nil,
			want:         true,
		},
		{
			name:         "localhost in 127.0.0.0/8",
			remoteAddr:   "127.0.0.1:54321",
			allowedCIDRs: []string{"127.0.0.0/8"},
			want:         true,
		},
		{
			name:         "private IP in 10.0.0.0/8",
			remoteAddr:   "10.0.1.5:8080",
			allowedCIDRs: []string{"10.0.0.0/8"},
			want:         true,
		},
		{
			name:         "IP not in allowlist",
			remoteAddr:   "192.168.1.1:9090",
			allowedCIDRs: []string{"10.0.0.0/8"},
			want:         false,
		},
		{
			name:         "multiple CIDRs second matches",
			remoteAddr:   "172.16.0.5:443",
			allowedCIDRs: []string{"10.0.0.0/8", "172.16.0.0/12"},
			want:         true,
		},
		{
			name:         "addr without port",
			remoteAddr:   "10.0.0.1",
			allowedCIDRs: []string{"10.0.0.0/8"},
			want:         true,
		},
		{
			name:         "invalid remote addr",
			remoteAddr:   "not-an-ip",
			allowedCIDRs: []string{"10.0.0.0/8"},
			want:         false,
		},
		{
			name:         "invalid CIDR is skipped",
			remoteAddr:   "10.0.0.1:1234",
			allowedCIDRs: []string{"bad-cidr", "10.0.0.0/8"},
			want:         true,
		},
		{
			name:         "IPv6 loopback",
			remoteAddr:   "[::1]:8080",
			allowedCIDRs: []string{"::1/128"},
			want:         true,
		},
		{
			name:         "exact /32 match",
			remoteAddr:   "203.0.113.42:5000",
			allowedCIDRs: []string{"203.0.113.42/32"},
			want:         true,
		},
		{
			name:         "exact /32 no match",
			remoteAddr:   "203.0.113.43:5000",
			allowedCIDRs: []string{"203.0.113.42/32"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkIPAllowed(tt.remoteAddr, tt.allowedCIDRs)
			if got != tt.want {
				t.Errorf("checkIPAllowed(%q, %v) = %v, want %v", tt.remoteAddr, tt.allowedCIDRs, got, tt.want)
			}
		})
	}
}

func TestValidateHMAC(t *testing.T) {
	secret := "my-secret"

	computeSig := func(algo string, body []byte) string {
		var h hash.Hash
		switch algo {
		case "sha1":
			h = hmac.New(sha1.New, []byte(secret))
		default:
			h = hmac.New(sha256.New, []byte(secret))
		}
		h.Write(body)
		return algo + "=" + hex.EncodeToString(h.Sum(nil))
	}

	tests := []struct {
		name      string
		body      []byte
		signature string
		algorithm string
		secret    string
		want      bool
	}{
		{
			name:      "valid sha256",
			body:      []byte("hello"),
			signature: computeSig("sha256", []byte("hello")),
			algorithm: "sha256",
			secret:    secret,
			want:      true,
		},
		{
			name:      "valid sha1",
			body:      []byte("hello"),
			signature: computeSig("sha1", []byte("hello")),
			algorithm: "sha1",
			secret:    secret,
			want:      true,
		},
		{
			name:      "wrong signature",
			body:      []byte("hello"),
			signature: "sha256=0000000000000000000000000000000000000000000000000000000000000000",
			algorithm: "sha256",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			body:      []byte("hello"),
			signature: computeSig("sha256", []byte("hello")),
			algorithm: "sha256",
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "missing equals separator",
			body:      []byte("hello"),
			signature: "noequalssign",
			algorithm: "sha256",
			secret:    secret,
			want:      false,
		},
		{
			name:      "invalid hex",
			body:      []byte("hello"),
			signature: "sha256=ZZZZ",
			algorithm: "sha256",
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty signature",
			body:      []byte("hello"),
			signature: "",
			algorithm: "sha256",
			secret:    secret,
			want:      false,
		},
		{
			name:      "sha256 body mismatch",
			body:      []byte("different body"),
			signature: computeSig("sha256", []byte("hello")),
			algorithm: "sha256",
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateHMAC(tt.body, tt.signature, tt.algorithm, tt.secret)
			if got != tt.want {
				t.Errorf("validateHMAC() = %v, want %v", got, tt.want)
			}
		})
	}
}
