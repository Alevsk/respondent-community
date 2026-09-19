package declarative

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

const (
	// defaultMaxBodyBytes is the default maximum request body size (10MB).
	defaultMaxBodyBytes int64 = 10 * 1024 * 1024

	// defaultSignatureHeader is the default HTTP header for HMAC signatures.
	defaultSignatureHeader = "X-Hub-Signature-256"

	// defaultSignatureAlgorithm is the default HMAC algorithm.
	defaultSignatureAlgorithm = "sha256"

	// webhookShutdownTimeout is the graceful shutdown timeout for the HTTP server.
	webhookShutdownTimeout = 5 * time.Second
)

// Compile-time check that WebhookTransport implements ListenTransport.
var _ ListenTransport = (*WebhookTransport)(nil)

func init() {
	RegisterTransport("webhook", newWebhookTransport)
}

// WebhookTransport implements ListenTransport by running an HTTP server
// that receives incoming webhook POST requests and pushes payloads to a channel.
type WebhookTransport struct {
	spec       *WebhookSpec
	sourceName string
	logger     *logging.Logger

	// Resolved secret for HMAC verification.
	secret string

	mu     sync.Mutex
	server *http.Server
}

// newWebhookTransport creates a new WebhookTransport from a source definition.
func newWebhookTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.Webhook == nil {
		return nil, fmt.Errorf("webhook transport requires transport.webhook configuration")
	}

	spec := def.Transport.Webhook

	var secret string
	if spec.Secret != "" {
		secret = envResolve(spec.Secret)
		if secret == "" {
			logger.Warn("webhook HMAC secret env var is empty or not set",
				logging.String("source_name", def.Name),
				logging.String("env_var", spec.Secret),
			)
		}
	}

	return &WebhookTransport{
		spec:       spec,
		sourceName: def.Name,
		logger:     logger,
		secret:     secret,
	}, nil
}

// Fetch returns ErrNotPullBased because webhooks are push-based.
func (w *WebhookTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Listen starts an HTTP server and pushes incoming webhook payloads to the channel.
// Blocks until ctx is cancelled or a fatal server error occurs.
func (w *WebhookTransport) Listen(ctx context.Context, payloads chan<- []byte) error {
	maxBody := w.spec.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}

	sigHeader := w.spec.SignatureHeader
	if sigHeader == "" {
		sigHeader = defaultSignatureHeader
	}

	sigAlgorithm := w.spec.SignatureAlgorithm
	if sigAlgorithm == "" {
		sigAlgorithm = defaultSignatureAlgorithm
	}

	mux := http.NewServeMux()
	mux.HandleFunc(w.spec.Path, func(rw http.ResponseWriter, r *http.Request) {
		// Only accept POST.
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// IP allowlist check.
		if !checkIPAllowed(r.RemoteAddr, w.spec.AllowedIPs) {
			w.logger.Warn("webhook request from disallowed IP",
				logging.String("source_name", w.sourceName),
				logging.String("remote_addr", r.RemoteAddr),
			)
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}

		// Read body with size limit.
		r.Body = http.MaxBytesReader(rw, r.Body, maxBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// MaxBytesReader returns a specific error when the limit is exceeded.
			if err.Error() == "http: request body too large" {
				http.Error(rw, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(rw, "failed to read body", http.StatusBadRequest)
			return
		}

		// HMAC signature validation.
		if w.secret != "" {
			sig := r.Header.Get(sigHeader)
			if sig == "" {
				w.logger.Warn("webhook request missing signature header",
					logging.String("source_name", w.sourceName),
					logging.String("header", sigHeader),
				)
				http.Error(rw, "forbidden", http.StatusForbidden)
				return
			}
			if !validateHMAC(body, sig, sigAlgorithm, w.secret) {
				w.logger.Warn("webhook HMAC validation failed",
					logging.String("source_name", w.sourceName),
				)
				http.Error(rw, "forbidden", http.StatusForbidden)
				return
			}
		}

		// Send payload to channel. Use a select to avoid blocking if the
		// consumer is slow or context is cancelled.
		select {
		case payloads <- body:
			rw.WriteHeader(http.StatusOK)
		case <-ctx.Done():
			http.Error(rw, "server shutting down", http.StatusServiceUnavailable)
		}
	})

	w.mu.Lock()
	w.server = &http.Server{
		Addr:    w.spec.ListenAddr,
		Handler: mux,
	}
	srv := w.server
	w.mu.Unlock()

	w.logger.Info("starting webhook server",
		logging.String("source_name", w.sourceName),
		logging.String("listen_addr", w.spec.ListenAddr),
		logging.String("path", w.spec.Path),
	)

	// Goroutine to shut down the server when context is cancelled.
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), webhookShutdownTimeout)
		defer cancel()
		errCh <- srv.Shutdown(shutdownCtx)
	}()

	// Start serving (blocks until server is shut down).
	var listenErr error
	if w.spec.TLS != nil {
		listenErr = srv.ListenAndServeTLS(w.spec.TLS.CertFile, w.spec.TLS.KeyFile)
	} else {
		listenErr = srv.ListenAndServe()
	}

	// ErrServerClosed is expected after graceful shutdown.
	if listenErr != nil && listenErr != http.ErrServerClosed {
		return fmt.Errorf("webhook server error: %w", listenErr)
	}

	// Wait for the shutdown goroutine to complete.
	if err := <-errCh; err != nil {
		return fmt.Errorf("webhook server shutdown error: %w", err)
	}

	w.logger.Info("webhook server stopped",
		logging.String("source_name", w.sourceName),
	)

	return nil
}

// Close gracefully shuts down the HTTP server.
// Safe to call multiple times or before Listen() is called.
func (w *WebhookTransport) Close() error {
	w.mu.Lock()
	srv := w.server
	w.mu.Unlock()

	if srv == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), webhookShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// validateHMAC verifies an HMAC signature in "algorithm=hex" format.
// Uses constant-time comparison to prevent timing attacks.
func validateHMAC(body []byte, signatureHeader, algorithm, secret string) bool {
	parts := strings.SplitN(signatureHeader, "=", 2)
	if len(parts) != 2 {
		return false
	}

	sigBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	var mac hash.Hash
	switch algorithm {
	case "sha1":
		mac = hmac.New(sha1.New, []byte(secret))
	default: // sha256
		mac = hmac.New(sha256.New, []byte(secret))
	}

	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(sigBytes, expected)
}

// checkIPAllowed verifies the remote address is within one of the allowed CIDR ranges.
// Returns true if allowedCIDRs is empty (no restriction).
func checkIPAllowed(remoteAddr string, allowedCIDRs []string) bool {
	if len(allowedCIDRs) == 0 {
		return true
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, cidr := range allowedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
