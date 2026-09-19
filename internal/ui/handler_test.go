package ui_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/Alevsk/respondent/internal/ui"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":       {Data: []byte("<html>Earth</html>")},
		"assets/main.js":   {Data: []byte("console.log('earth')")},
		"assets/style.css": {Data: []byte("body{}")},
	}
}

func TestNewHandler_ServesIndexAtRoot(t *testing.T) {
	h := ui.NewHandler(testFS())
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Earth")
}

func TestNewHandler_ServesStaticAssets(t *testing.T) {
	h := ui.NewHandler(testFS())
	req := httptest.NewRequest("GET", "/assets/main.js", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "console.log")
}

func TestNewHandler_SPAFallbackToIndex(t *testing.T) {
	h := ui.NewHandler(testFS())
	// Request a path that doesn't exist in the FS — should serve index.html.
	req := httptest.NewRequest("GET", "/some/deep/route", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Earth")
}

func TestNewHandler_NilFS_Returns503(t *testing.T) {
	h := ui.NewHandler(nil)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	resp := rec.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
