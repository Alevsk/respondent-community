//go:build e2e

package declarative

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// TestACLED_OAuth2_E2E verifies the full OAuth2 token exchange against the real ACLED API.
//
// This is a live-network end-to-end test: it makes real HTTP requests to
// https://acleddata.com, so it is excluded from the default `go test ./...`
// suite and only compiles under the "e2e" build tag. Run it deliberately with:
//
//	go test -tags e2e ./internal/ingest/declarative/ -run TestACLED_OAuth2_E2E
//
// It additionally requires RESPONDENT_ACLED_EMAIL and RESPONDENT_ACLED_PASSWORD
// (loaded from the project .env or the environment); the test skips when those
// are absent, so a bare `-tags e2e` run stays green without credentials.
//
// NOTE: The acled_conflicts.yaml source currently uses the public Explorer endpoint
// (no auth). This test exercises the OAuth2 token provider directly to ensure the
// implementation stays functional for when ACLED API access is granted.
func TestACLED_OAuth2_E2E(t *testing.T) {
	root := projectRoot(t)
	loadDotEnv(t, filepath.Join(root, ".env"))

	email := os.Getenv("RESPONDENT_ACLED_EMAIL")
	password := os.Getenv("RESPONDENT_ACLED_PASSWORD")
	if email == "" || password == "" {
		t.Skip("RESPONDENT_ACLED_EMAIL and RESPONDENT_ACLED_PASSWORD not set; skipping E2E test")
	}

	logger := logging.NewNopLogger()

	spec := &OAuth2Spec{
		TokenURL:  "https://acleddata.com/oauth/token",
		GrantType: "password",
	}
	creds := resolvedOAuth2Credentials{
		ClientID: "acled",
		Username: email,
		Password: password,
	}
	provider := newOAuth2TokenProvider(spec, creds, "acled_e2e_test", logger)

	// Test 1: Acquire token.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hName, hValue, err := provider.GetHeader(ctx)
	if err != nil {
		t.Fatalf("GetHeader (token acquisition) failed: %v", err)
	}
	if hName != "Authorization" {
		t.Errorf("expected header name 'Authorization', got %q", hName)
	}
	if len(hValue) < 10 {
		t.Errorf("token value suspiciously short: %q", hValue)
	}
	t.Logf("Token acquired successfully (header: %s, token length: %d)", hName, len(hValue))

	// Test 2: Refresh token.
	// Force expiry so the next GetHeader triggers a refresh.
	provider.mu.Lock()
	provider.expiresAt = time.Now().Add(-1 * time.Hour)
	provider.mu.Unlock()

	hName2, hValue2, err := provider.GetHeader(ctx)
	if err != nil {
		t.Fatalf("GetHeader (refresh) failed: %v", err)
	}
	if hName2 != "Authorization" {
		t.Errorf("expected header name 'Authorization' after refresh, got %q", hName2)
	}
	if len(hValue2) < 10 {
		t.Errorf("refreshed token suspiciously short: %q", hValue2)
	}
	t.Logf("Token refreshed successfully (token length: %d)", len(hValue2))
}

// loadDotEnv loads a .env file into the process environment for the live E2E test.
// Lines starting with # and empty lines are skipped. Only KEY=VALUE lines are processed.
func loadDotEnv(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Logf(".env not found at %s, skipping env loading", path)
			return
		}
		t.Fatalf("failed to open .env: %v", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		t.Setenv(strings.TrimSpace(k), strings.TrimSpace(v))
	}
}

// projectRoot resolves the project root relative to this test file.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path via runtime.Caller")
	}
	dir := filepath.Dir(filename)
	root := filepath.Join(dir, "..", "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}
	return abs
}
