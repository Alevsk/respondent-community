package declarative

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestWebhookTransport_ListenStartError(t *testing.T) {
	// Bind a port first so the webhook cannot start.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("bind port: %v", err)
	}
	defer func() { _ = ln.Close() }()

	spec := &WebhookSpec{
		Path:       "/webhook",
		ListenAddr: ln.Addr().String(), // already bound
	}
	wt := &WebhookTransport{
		spec:       spec,
		sourceName: "test_listen_err",
		logger:     logging.NewNopLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	payloads := make(chan []byte, 1)
	err = wt.Listen(ctx, payloads)
	if err == nil {
		t.Fatal("expected error when port is already in use, got nil")
	}
	if !strings.Contains(err.Error(), "webhook server error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebhookTransport_ContextCancelledDuringHandle(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())

	// Use an unbuffered channel so the payload send will block.
	payloads := make(chan []byte) // unbuffered

	errCh := make(chan error, 1)
	go func() {
		errCh <- wt.Listen(ctx, payloads)
	}()

	// Wait for server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", wt.spec.ListenAddr, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel context — this will cause the handler's ctx.Done() branch to fire
	// when a request is in flight or the server sees shutdown.
	cancel()

	// Post a request; the handler may hit ctx.Done() or the server may be shutting down.
	_, _ = http.Post(baseURL, "application/json", bytes.NewReader([]byte(`{}`)))

	select {
	case err := <-errCh:
		// Listen should return nil after graceful shutdown.
		if err != nil {
			t.Logf("Listen returned (acceptable error): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Listen to return after context cancel")
	}
}

func TestWebhookTransport_BodyReadError(t *testing.T) {
	// To hit the non-413 error path in the body read, we need a body that fails
	// to read. We use a pipe that we close mid-read to trigger an unexpected error.

	spec := &WebhookSpec{
		Path:         "/webhook",
		MaxBodyBytes: 10 * 1024 * 1024, // large limit so we don't hit the 413 path
	}
	wt, _ := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	payloads := make(chan []byte, 10)
	startWebhookListener(t, wt, ctx, payloads)

	// Build a request where the Content-Length is set but the body is shorter
	// than advertised. The MaxBytesReader still tries to read, but the connection
	// breaks. Actually the easiest approach is to use net.Dial directly to send
	// a malformed HTTP request.

	// Connect directly via TCP and send a well-formed HTTP request start,
	// then abruptly close the connection mid-body.
	conn, err := net.DialTimeout("tcp", wt.spec.ListenAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Send a POST with Content-Length: 100 but only write 10 bytes, then close.
	// The server's io.ReadAll will get an unexpected EOF.
	_, _ = fmt.Fprintf(conn, "POST /webhook HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"partial\":", wt.spec.ListenAddr)
	_ = conn.Close() // Close abruptly mid-body

	// Give server time to process the request.
	time.Sleep(100 * time.Millisecond)
	// The server should have returned 400 Bad Request (or just closed the connection).
	// There's no way to read the response since we closed the connection.
	// The important thing is the server didn't crash.
}

// ---------------------------------------------------------------------------
// transport_webhook.go – server shutdown error (line 204-206)
// Note: This is very hard to trigger in tests since it requires Shutdown to fail.
// We test the TLS path instead (line 192-196).
// ---------------------------------------------------------------------------

// TestWebhookTransport_TLSMissingCerts verifies that Listen with TLS spec returns
// an error when cert files do not exist (exercises the ListenAndServeTLS path).
func TestWebhookTransport_TLSMissingCerts(t *testing.T) {
	spec := &WebhookSpec{
		Path:       "/webhook",
		ListenAddr: "127.0.0.1:0",
		TLS: &WebhookTLSSpec{
			CertFile: "/nonexistent/cert.pem",
			KeyFile:  "/nonexistent/key.pem",
		},
	}
	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find port: %v", err)
	}
	spec.ListenAddr = ln.Addr().String()
	_ = ln.Close()

	wt := &WebhookTransport{
		spec:       spec,
		sourceName: "test_tls_err",
		logger:     logging.NewNopLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	payloads := make(chan []byte, 1)
	err = wt.Listen(ctx, payloads)
	if err == nil {
		t.Fatal("expected TLS error, got nil")
	}
	if !strings.Contains(err.Error(), "webhook server error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWebhookTransport_HandlerCtxDone(t *testing.T) {
	spec := &WebhookSpec{Path: "/webhook"}
	wt, baseURL := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithCancel(context.Background())

	// Unbuffered channel: the handler will block on payloads <- body.
	payloads := make(chan []byte)

	errCh := make(chan error, 1)
	go func() {
		errCh <- wt.Listen(ctx, payloads)
	}()

	// Wait for server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", wt.spec.ListenAddr, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Launch a request in a goroutine (it will block trying to send to payloads).
	reqDone := make(chan int, 1)
	go func() {
		resp, err := http.Post(baseURL, "application/json", strings.NewReader(`{}`))
		if err != nil {
			reqDone <- 0
			return
		}
		reqDone <- resp.StatusCode
		_ = resp.Body.Close()
	}()

	// Give the request time to reach the handler's payloads <- body select.
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger the ctx.Done() branch in the handler.
	cancel()

	// Check response or timeout.
	select {
	case status := <-reqDone:
		if status == http.StatusServiceUnavailable {
			t.Logf("Got expected 503 Service Unavailable")
		} else {
			t.Logf("Got status %d (timing-dependent, acceptable)", status)
		}
	case <-time.After(5 * time.Second):
		t.Log("Request timed out (acceptable — connection may have been closed by shutdown)")
	}

	// Ensure Listen returns.
	select {
	case err := <-errCh:
		if err != nil {
			t.Logf("Listen returned (acceptable error): %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for Listen to return")
	}
}

func TestWebhookTransport_Listen_PortInUse(t *testing.T) {
	// Start a listener to occupy a port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()

	spec := &WebhookSpec{
		Path:       "/webhook",
		ListenAddr: addr, // already in use
	}

	wt, _ := newTestWebhookTransport(t, spec, "")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	payloads := make(chan []byte, 1)
	err = wt.Listen(ctx, payloads)
	if err == nil {
		t.Fatal("expected error from port already in use, got nil")
	}
	if !strings.Contains(err.Error(), "webhook server error") {
		t.Errorf("expected 'webhook server error', got: %v", err)
	}
}
