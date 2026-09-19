package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrontendConfigPayload_OmitsEmpty(t *testing.T) {
	assert.Empty(t, frontendConfigPayload(FrontendConfig{}),
		"empty config must yield no keys so the frontend falls back to build-time defaults")

	p := frontendConfigPayload(FrontendConfig{CesiumIonToken: "tok"})
	assert.Equal(t, "tok", p["cesiumIonToken"])
}

func TestFrontendConfigHandler_ServesToken(t *testing.T) {
	rec := httptest.NewRecorder()
	frontendConfigHandler(FrontendConfig{CesiumIonToken: "tok-xyz"})(
		rec, httptest.NewRequest("GET", "/config.json", nil))

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "tok-xyz", got["cesiumIonToken"])
}

func TestFrontendConfigHandler_EmptyIsEmptyObject(t *testing.T) {
	rec := httptest.NewRecorder()
	frontendConfigHandler(FrontendConfig{})(
		rec, httptest.NewRequest("GET", "/config.json", nil))

	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "{}", strings.TrimSpace(string(body)),
		"no token → empty JSON object (key omitted)")
}
