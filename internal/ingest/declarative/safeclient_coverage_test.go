package declarative

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSSRFSafeClient_BlocksLoopback(t *testing.T) {
	client := NewSSRFSafeClient(5 * time.Second)

	// Start a real listener on localhost to get a valid port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// The SSRF safe client should block 127.x.x.x.
	_, err = client.Get("http://" + addr + "/")
	if err == nil {
		t.Error("expected SSRF block for loopback address, got nil error")
	}
}

func TestNewSSRFSafeClient_BlockedIP(t *testing.T) {
	client := NewSSRFSafeClient(5 * time.Second)

	// httptest.NewServer listens on 127.0.0.1 which is in the blocked CIDR.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := client.Get(srv.URL)
	if err == nil {
		t.Fatal("expected SSRF block error, got nil")
	}
	if !strings.Contains(err.Error(), "SSRF blocked") {
		t.Errorf("expected 'SSRF blocked' in error, got: %v", err)
	}
}

func TestNewSSRFSafeClient_InvalidAddress(t *testing.T) {
	client := NewSSRFSafeClient(1 * time.Second)
	transport := client.Transport.(*http.Transport)

	// Call DialContext directly with an address missing a port (SplitHostPort fails).
	_, err := transport.DialContext(context.Background(), "tcp", "invalid_no_port")
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
	if !strings.Contains(err.Error(), "invalid address") && !strings.Contains(err.Error(), "address") {
		t.Errorf("expected address error, got: %v", err)
	}
}

func TestNewSSRFSafeClient_DNSLookupFailure(t *testing.T) {
	client := NewSSRFSafeClient(2 * time.Second)

	_, err := client.Get("http://thisdomaindoesnotexist.invalid/path")
	if err == nil {
		t.Fatal("expected DNS lookup failure, got nil")
	}
	if strings.Contains(err.Error(), "SSRF blocked") {
		t.Errorf("expected DNS error, not SSRF block: %v", err)
	}
}

func TestNewSSRFSafeClient_ReturnsClient(t *testing.T) {
	timeout := 15 * time.Second
	client := NewSSRFSafeClient(timeout)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, client.Timeout)
	}
}

func TestSSRFSafeClient_NilIPFromResolver(t *testing.T) {
	// The nil-IP continue path fires when net.ParseIP returns nil for a resolved address.
	// This is essentially dead code since DNS always returns valid IPs.
	// We test it via a direct call to the custom DialContext with a known-blocked
	// loopback address to verify SSRF detection fires.
	client := NewSSRFSafeClient(2 * time.Second)
	transport := client.Transport.(*http.Transport)

	// Using 127.0.0.1:80 — should be blocked SSRF.
	_, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected SSRF block for 127.0.0.1, got nil")
	}
	if !strings.Contains(err.Error(), "SSRF blocked") {
		t.Errorf("expected SSRF block error, got: %v", err)
	}
}

func TestSSRFSafeClient_ContextCancelledDuringLookup(t *testing.T) {
	client := NewSSRFSafeClient(5 * time.Second)
	transport := client.Transport.(*http.Transport)

	// Cancel the context before the dial attempt.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	_, err := transport.DialContext(ctx, "tcp", "example.com:80")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// The error should be context-related or DNS lookup failure.
	t.Logf("Got expected error: %v", err)
}
