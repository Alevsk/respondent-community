package declarative

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// audioMediaYAML declares an audio media entry whose playback notification is a
// source-declared GET action built from entity metadata.
const audioMediaYAML = `
  media:
    - id: radio
      kind: audio
      label: Live stream
      url_key: type
      playback_action: report_play
media_actions:
  - name: report_play
    method: GET
    path: '"/json/url/" + metadata["type"]'
`

// rawPathMediaYAML puts the whole path under metadata control so path admission
// can be exercised directly.
const rawPathMediaYAML = `
  media:
    - id: radio
      kind: audio
      label: Live stream
      url_key: type
      playback_action: report_play
media_actions:
  - name: report_play
    method: GET
    path: 'metadata["type"]'
`

// loadLocalMediaTestSource loads a source pointed at an httptest server, which
// serves plain HTTP; the loader only admits that in dev mode.
func loadLocalMediaTestSource(t *testing.T, source string) *CompiledSource {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.yaml")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cs, err := newTestLoader(t, true).LoadFile(path)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	return cs
}

func newTestMediaActionRegistry(t *testing.T, mediaYAML string, client *http.Client) *MediaActionRegistry {
	t.Helper()
	cs, err := loadMediaTestSource(t, validSourceYAML+mediaYAML)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, client, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}
	return reg
}

func TestMediaActionResolvesDeclaredPathFromMetadata(t *testing.T) {
	reg := newTestMediaActionRegistry(t, audioMediaYAML, nil)

	action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": "abc-123"})
	if err != nil {
		t.Fatalf("ResolveMediaAction: %v", err)
	}
	if action.Path != "/json/url/abc-123" {
		t.Errorf("path = %q, want /json/url/abc-123", action.Path)
	}
	if action.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", action.Method)
	}
	if string(action.LayerType) != "test_layer" {
		t.Errorf("layer = %q, want test_layer", action.LayerType)
	}
}

func TestMediaActionRejectsUnknownLayerOrAction(t *testing.T) {
	reg := newTestMediaActionRegistry(t, audioMediaYAML, nil)

	if _, err := reg.ResolveMediaAction("other_layer", "report_play", map[string]string{"type": "x"}); err == nil {
		t.Error("resolved an action for an unregistered layer")
	}
	if _, err := reg.ResolveMediaAction("test_layer", "report_everything", map[string]string{"type": "x"}); err == nil {
		t.Error("resolved an action name the source never declared")
	}
}

// TestMediaActionRejectsUnsafeResolvedPaths covers the case where a hostile or
// broken upstream catalog puts an absolute URL, an authority, or traversal into
// the metadata the declared path expression reads.
func TestMediaActionRejectsUnsafeResolvedPaths(t *testing.T) {
	reg := newTestMediaActionRegistry(t, rawPathMediaYAML, nil)

	rejected := map[string]string{
		"absolute https URL":   "https://evil.example.com/steal",
		"absolute http URL":    "http://evil.example.com/steal",
		"protocol relative":    "//evil.example.com/steal",
		"backslash authority":  "/\\evil.example.com/steal",
		"parent traversal":     "/json/../../etc/passwd",
		"bare traversal":       "../json/url/x",
		"no leading slash":     "json/url/x",
		"empty":                "",
		"scheme relative file": "file:///etc/passwd",
		"fragment":             "/json/url/x#frag",
		"credentials":          "https://user:pw@evil.example.com/",
		// Percent-encoded traversal: a single path segment as far as Go is
		// concerned, so a segment-by-segment comparison never sees it, yet an
		// upstream that decodes before normalizing walks out of /json/url/.
		"encoded traversal":    "/json/url/%2e%2e%2f%2e%2e%2fadmin",
		"mixed case traversal": "/json/%2E%2e/admin",
		"encoded separator":    "/json/url/a%2Fb",
		"encoded backslash":    "/json/url/a%5Cb",
		// The action declares a path, not a query: a metadata value that smuggles
		// one changes the request the source never asked for.
		"query injection": "/json/url/x?admin=1",
		"encoded null":    "/json/url/%00",
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			if action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": value}); err == nil {
				t.Errorf("accepted unsafe path %q as %q", value, action.Path)
			}
		})
	}

	if _, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": "/json/url/ok"}); err != nil {
		t.Errorf("rejected a safe relative path: %v", err)
	}
}

func TestMediaActionExecutesGETAgainstDeclaredOriginOnly(t *testing.T) {
	var gotMethod, gotPath, gotHost string
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		gotMethod, gotPath, gotHost = r.Method, r.URL.RequestURI(), r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"stream":"https://upstream.example.com/secret.mp3"}`))
	}))
	defer srv.Close()

	yaml := strings.Replace(validSourceYAML, "https://example.com/api/data", srv.URL+"/api/data", 1)
	cs := loadLocalMediaTestSource(t, yaml+audioMediaYAML)
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, srv.Client(), logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}

	action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": "abc-123"})
	if err != nil {
		t.Fatalf("ResolveMediaAction: %v", err)
	}
	if err := reg.ExecuteMediaAction(context.Background(), action); err != nil {
		t.Fatalf("ExecuteMediaAction: %v", err)
	}

	if calls.Load() != 1 {
		t.Errorf("calls = %d, want exactly 1", calls.Load())
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	// The declared origin is kept and the declared action path replaces the
	// catalog path — the action never inherits /api/data.
	if gotPath != "/json/url/abc-123" {
		t.Errorf("request URI = %q, want /json/url/abc-123", gotPath)
	}
	if want := strings.TrimPrefix(srv.URL, "http://"); gotHost != want {
		t.Errorf("host = %q, want %q", gotHost, want)
	}
}

func TestMediaActionBoundsConcurrency(t *testing.T) {
	var inFlight, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inFlight.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	yaml := strings.Replace(validSourceYAML, "https://example.com/api/data", srv.URL+"/api/data", 1)
	cs := loadLocalMediaTestSource(t, yaml+audioMediaYAML)
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, srv.Client(), logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < mediaActionMaxConcurrency*3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": fmt.Sprintf("s-%d", i)})
			if err != nil {
				t.Errorf("ResolveMediaAction: %v", err)
				return
			}
			if err := reg.ExecuteMediaAction(context.Background(), action); err != nil {
				t.Errorf("ExecuteMediaAction: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if peak.Load() > int32(mediaActionMaxConcurrency) {
		t.Errorf("peak in-flight notifications = %d, want <= %d", peak.Load(), mediaActionMaxConcurrency)
	}
}

func TestMediaActionBoundsTimeAndResponseSize(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	yaml := strings.Replace(validSourceYAML, "https://example.com/api/data", slow.URL+"/api/data", 1)
	cs := loadLocalMediaTestSource(t, yaml+audioMediaYAML)
	client := slow.Client()
	client.Timeout = 50 * time.Millisecond
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, client, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}

	action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": "abc"})
	if err != nil {
		t.Fatalf("ResolveMediaAction: %v", err)
	}
	start := time.Now()
	if err := reg.ExecuteMediaAction(context.Background(), action); err == nil {
		t.Error("a notification that outlived the client timeout reported success")
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Errorf("notification took %s, want it bounded well under the handler's 300ms", elapsed)
	}

}

// countingBody records how much of a response the notification path actually
// pulls, so the response ceiling is proven to be APPLIED rather than merely
// declared as a constant.
type countingBody struct {
	io.Reader
	read *int64
}

func (c countingBody) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	*c.read += int64(n)
	return n, err
}

func (countingBody) Close() error { return nil }

func TestMediaActionStopsReadingAtTheResponseCeiling(t *testing.T) {
	var read int64
	// A notification response is never relayed, so a chatty (or hostile)
	// upstream must not be read into memory in full.
	huge := 4 * 1024 * 1024
	client := &http.Client{Transport: mediaRoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       countingBody{Reader: io.LimitReader(neverEnding{}, int64(huge)), read: &read},
			Request:    r,
		}, nil
	})}

	cs := loadLocalMediaTestSource(t, strings.Replace(validSourceYAML,
		"https://example.com/api/data", "http://media-action.test/api/data", 1)+audioMediaYAML)
	reg, err := NewMediaActionRegistry([]*CompiledSource{cs}, client, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewMediaActionRegistry: %v", err)
	}
	action, err := reg.ResolveMediaAction("test_layer", "report_play", map[string]string{"type": "abc"})
	if err != nil {
		t.Fatalf("ResolveMediaAction: %v", err)
	}
	if err := reg.ExecuteMediaAction(context.Background(), action); err != nil {
		t.Fatalf("ExecuteMediaAction: %v", err)
	}

	if read == 0 {
		t.Fatal("nothing was read; the test is not exercising the response path")
	}
	// io.ReadAll over a LimitReader may overshoot by one chunk, never by more.
	if read > mediaActionMaxResponseBytes+64*1024 {
		t.Errorf("read %d bytes of a %d byte response; the %d byte ceiling is not applied",
			read, huge, mediaActionMaxResponseBytes)
	}
}

// neverEnding yields zero bytes forever, so only the ceiling stops the read.
type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) { return len(p), nil }

// Compile-time proof that the registry satisfies the domain ports the app
// service depends on, so the app layer never imports this package.
var (
	_ domain.MediaActionResolver = (*MediaActionRegistry)(nil)
	_ domain.MediaActionExecutor = (*MediaActionRegistry)(nil)
)
