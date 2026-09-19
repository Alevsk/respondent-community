package declarative

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// TokenProvider abstracts credential resolution for transports.
// Static auth returns a fixed header value; OAuth2 manages token lifecycle.
type TokenProvider interface {
	// GetHeader returns the header name and value to set on each request.
	// For OAuth2, this transparently handles token refresh.
	// Returns ("", "", nil) if no auth is configured.
	GetHeader(ctx context.Context) (headerName, headerValue string, err error)
}

// staticTokenProvider wraps the existing bearer/api_key behavior.
type staticTokenProvider struct {
	headerName  string
	headerValue string
}

func (p *staticTokenProvider) GetHeader(_ context.Context) (string, string, error) {
	return p.headerName, p.headerValue, nil
}

// resolvedOAuth2Credentials holds credential values after env var resolution.
type resolvedOAuth2Credentials struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// oauth2TokenProvider manages the OAuth2 token lifecycle.
type oauth2TokenProvider struct {
	mu          sync.Mutex
	client      *http.Client
	spec        *OAuth2Spec
	credentials resolvedOAuth2Credentials
	sourceName  string
	logger      *logging.Logger

	// Cached state (in-memory only, lost on restart)
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// newOAuth2TokenProvider creates a new OAuth2 token provider.
func newOAuth2TokenProvider(spec *OAuth2Spec, creds resolvedOAuth2Credentials, sourceName string, logger *logging.Logger) *oauth2TokenProvider {
	return &oauth2TokenProvider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		spec:        spec,
		credentials: creds,
		sourceName:  sourceName,
		logger:      logger,
	}
}

// tokenHeader returns the configured header name, defaulting to "Authorization".
func (p *oauth2TokenProvider) tokenHeader() string {
	if p.spec.TokenHeader != "" {
		return p.spec.TokenHeader
	}
	return "Authorization"
}

// tokenPrefix returns the configured token prefix, defaulting to "Bearer ".
func (p *oauth2TokenProvider) tokenPrefix() string {
	if p.spec.TokenPrefix != "" {
		return p.spec.TokenPrefix
	}
	return "Bearer "
}

// refreshBeforeExpiry returns the safety margin before expiry, defaulting to 5 minutes.
func (p *oauth2TokenProvider) refreshBeforeExpiry() time.Duration {
	if p.spec.RefreshBeforeExpiry.Duration > 0 {
		return p.spec.RefreshBeforeExpiry.Duration
	}
	return 5 * time.Minute
}

// GetHeader returns the auth header, transparently handling token acquisition and refresh.
func (p *oauth2TokenProvider) GetHeader(ctx context.Context) (string, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return cached token if still valid.
	if p.accessToken != "" && time.Now().Before(p.expiresAt.Add(-p.refreshBeforeExpiry())) {
		return p.tokenHeader(), p.tokenPrefix() + p.accessToken, nil
	}

	// Try refresh if we have a refresh token.
	if p.refreshToken != "" {
		err := p.doRefresh(ctx)
		if err == nil {
			return p.tokenHeader(), p.tokenPrefix() + p.accessToken, nil
		}
		// Refresh failed — fall through to full re-auth.
		p.logger.Warn("oauth2 refresh failed, re-authenticating",
			logging.String("source_name", p.sourceName),
			logging.Err("error", err),
		)
	}

	// Full authenticate.
	if err := p.doAuthenticate(ctx); err != nil {
		return "", "", fmt.Errorf("oauth2 authenticate: %w", err)
	}

	return p.tokenHeader(), p.tokenPrefix() + p.accessToken, nil
}

// doAuthenticate performs the initial token exchange.
func (p *oauth2TokenProvider) doAuthenticate(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", p.spec.GrantType)

	switch p.spec.GrantType {
	case "password":
		if p.credentials.ClientID != "" {
			form.Set("client_id", p.credentials.ClientID)
		}
		form.Set("username", p.credentials.Username)
		form.Set("password", p.credentials.Password)
	case "client_credentials":
		form.Set("client_id", p.credentials.ClientID)
		form.Set("client_secret", p.credentials.ClientSecret)
	}

	// Add extra params.
	for k, v := range p.spec.ExtraParams {
		form.Set(k, v)
	}

	return p.doTokenRequest(ctx, form, "authenticate")
}

// doRefresh performs a token refresh using the stored refresh token.
func (p *oauth2TokenProvider) doRefresh(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", p.refreshToken)
	if p.credentials.ClientID != "" {
		form.Set("client_id", p.credentials.ClientID)
	}

	// Add extra params.
	for k, v := range p.spec.ExtraParams {
		form.Set(k, v)
	}

	return p.doTokenRequest(ctx, form, "refresh")
}

// doTokenRequest performs the actual HTTP POST to the token endpoint.
func (p *oauth2TokenProvider) doTokenRequest(ctx context.Context, form url.Values, operation string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.spec.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create %s request: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("token %s request failed: %w", operation, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return fmt.Errorf("read token %s response: %w", operation, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Include a truncated body excerpt for diagnostics.
		excerpt := string(body)
		if len(excerpt) > 256 {
			excerpt = excerpt[:256] + "..."
		}
		return fmt.Errorf("token %s returned HTTP %d: %s", operation, resp.StatusCode, excerpt)
	}

	// Parse the JSON response with UseNumber() so numeric fields decode as json.Number
	// instead of float64, preserving integer precision for expires_in values.
	var raw map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("parse token %s response JSON: %w", operation, err)
	}

	// Extract fields using response mapping (with defaults).
	accessTokenKey := "access_token"
	refreshTokenKey := "refresh_token"
	expiresInKey := "expires_in"
	if p.spec.ResponseMapping != nil {
		if p.spec.ResponseMapping.AccessToken != "" {
			accessTokenKey = p.spec.ResponseMapping.AccessToken
		}
		if p.spec.ResponseMapping.RefreshToken != "" {
			refreshTokenKey = p.spec.ResponseMapping.RefreshToken
		}
		if p.spec.ResponseMapping.ExpiresIn != "" {
			expiresInKey = p.spec.ResponseMapping.ExpiresIn
		}
	}

	accessToken, err := extractJSONString(raw, accessTokenKey)
	if err != nil {
		return fmt.Errorf("token %s response missing %q: %w", operation, accessTokenKey, err)
	}
	p.accessToken = accessToken

	// Refresh token is optional.
	if rt, err := extractJSONString(raw, refreshTokenKey); err == nil {
		p.refreshToken = rt
	}

	// Parse expires_in (seconds).
	if expiresIn, ok := extractJSONNumber(raw, expiresInKey); ok && expiresIn > 0 {
		p.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	} else {
		// Default to 1 hour if not provided.
		p.expiresAt = time.Now().Add(1 * time.Hour)
	}

	p.logger.Info("oauth2 token acquired",
		logging.String("source_name", p.sourceName),
		logging.String("grant_type", form.Get("grant_type")),
		logging.Any("expires_in_seconds", time.Until(p.expiresAt).Seconds()),
	)

	return nil
}

// extractJSONString extracts a string value from a JSON map using a dot-separated path.
func extractJSONString(data map[string]interface{}, path string) (string, error) {
	val := extractJSONPath(data, path)
	if val == nil {
		return "", fmt.Errorf("field %q not found", path)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("field %q is %T, expected string", path, val)
	}
	return s, nil
}

// extractJSONNumber extracts a numeric value from a JSON map using a dot-separated path.
// Expects json.Number values (produced by json.Decoder with UseNumber()).
func extractJSONNumber(data map[string]interface{}, path string) (float64, bool) {
	val := extractJSONPath(data, path)
	if val == nil {
		return 0, false
	}
	n, ok := val.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil
}

// extractJSONPath traverses a nested map using a dot-separated key path.
func extractJSONPath(data map[string]interface{}, path string) interface{} {
	segments := strings.Split(path, ".")
	var current interface{} = data
	for _, seg := range segments {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		val, exists := m[seg]
		if !exists {
			return nil
		}
		current = val
	}
	return current
}
