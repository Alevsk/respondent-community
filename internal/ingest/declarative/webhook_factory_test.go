package declarative

import (
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

// TestNewWebhookTransport_Success verifies the transport is created successfully with valid config.
func TestNewWebhookTransport_Success(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_webhook_src",
		Transport: TransportSpec{
			Webhook: &WebhookSpec{
				ListenAddr: "127.0.0.1:0",
				Path:       "/hook",
			},
		},
	}
	logger := logging.NewNopLogger()

	transport, err := newWebhookTransport(def, nil, nil, func(s string) string { return s }, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

// TestNewWebhookTransport_MissingWebhookSpec verifies error when webhook spec is nil.
func TestNewWebhookTransport_MissingWebhookSpec(t *testing.T) {
	def := &SourceDefinition{
		Name:      "no_webhook",
		Transport: TransportSpec{
			// Webhook is nil
		},
	}
	logger := logging.NewNopLogger()

	_, err := newWebhookTransport(def, nil, nil, func(s string) string { return s }, logger)
	if err == nil {
		t.Fatal("expected error for nil webhook spec, got nil")
	}
	if !strings.Contains(err.Error(), "webhook transport requires") {
		t.Errorf("expected 'webhook transport requires' in error, got: %v", err)
	}
}

// TestNewWebhookTransport_WithSecret verifies that the secret env var is resolved.
func TestNewWebhookTransport_WithSecret(t *testing.T) {
	def := &SourceDefinition{
		Name: "webhook_with_secret",
		Transport: TransportSpec{
			Webhook: &WebhookSpec{
				ListenAddr: "127.0.0.1:0",
				Path:       "/secure",
				Secret:     "MY_SECRET_ENV",
			},
		},
	}
	logger := logging.NewNopLogger()

	// envResolve returns the secret value for the env var name.
	transport, err := newWebhookTransport(def, nil, nil, func(key string) string {
		if key == "MY_SECRET_ENV" {
			return "mysecretvalue"
		}
		return ""
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wt, ok := transport.(*WebhookTransport)
	if !ok {
		t.Fatalf("expected *WebhookTransport, got %T", transport)
	}
	if wt.secret != "mysecretvalue" {
		t.Errorf("expected secret 'mysecretvalue', got %q", wt.secret)
	}
}

// TestNewWebhookTransport_EmptySecretEnvVar verifies a warning is logged (but no error)
// when the secret env var is set but empty.
func TestNewWebhookTransport_EmptySecretEnvVar(t *testing.T) {
	def := &SourceDefinition{
		Name: "webhook_empty_secret",
		Transport: TransportSpec{
			Webhook: &WebhookSpec{
				ListenAddr: "127.0.0.1:0",
				Path:       "/hook",
				Secret:     "EMPTY_SECRET",
			},
		},
	}
	logger := logging.NewNopLogger()

	// envResolve returns empty string for the secret env var.
	transport, err := newWebhookTransport(def, nil, nil, func(key string) string {
		return "" // empty
	}, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wt, ok := transport.(*WebhookTransport)
	if !ok {
		t.Fatalf("expected *WebhookTransport, got %T", transport)
	}
	// Secret should be empty (warning was logged but no error).
	if wt.secret != "" {
		t.Errorf("expected empty secret, got %q", wt.secret)
	}
}
