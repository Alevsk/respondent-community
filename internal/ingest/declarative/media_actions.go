package declarative

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/cel-go/cel"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

const (
	// mediaActionMaxConcurrency caps simultaneous playback notifications across
	// every source. Notifications are courtesy calls to third-party catalogs;
	// a burst of listeners must not turn into a burst of outbound requests.
	mediaActionMaxConcurrency = 4
	// mediaActionMaxResponseBytes bounds the response read. Nothing from the
	// response reaches the caller, so only enough is read to drain the body.
	mediaActionMaxResponseBytes int64 = 16 * 1024
	// mediaActionMaxPathLen bounds a resolved action path.
	mediaActionMaxPathLen = 512
	// mediaActionTimeout bounds one notification when the injected client has
	// no timeout of its own.
	mediaActionTimeout = 5 * time.Second
)

// MediaActionRegistry executes the playback notifications a source declares in
// its `media_actions` block.
//
// It is deliberately the only path from an API request to an outbound media
// call: a caller names an entity and a media slot, and everything else — the
// origin, the method, the path expression — comes from the trusted source
// definition compiled at load time.
type MediaActionRegistry struct {
	byLayer map[domain.LayerType]*layerMediaActions
	sem     chan struct{}
	logger  *logging.Logger
}

// layerMediaActions holds one source's compiled actions and the transport that
// reaches its origin.
type layerMediaActions struct {
	sourceName string
	origin     string
	actions    map[string]MediaActionSpec
	programs   map[string]cel.Program
	transport  Transport
}

// NewMediaActionRegistry builds the registry from compiled sources. Sources
// without media_actions are skipped, so the registry is empty unless some YAML
// asked for it.
//
// Each source gets its own transport rather than borrowing the feeder's: a
// playback notification must never repin the mirror a catalog refresh is
// paginating through. The injected client still carries the shared per-host
// rate budget, so both paths spend from one bucket.
func NewMediaActionRegistry(sources []*CompiledSource, client *http.Client, logger *logging.Logger) (*MediaActionRegistry, error) {
	if logger == nil {
		logger = logging.NewNopLogger()
	}
	if client == nil {
		client = &http.Client{Timeout: mediaActionTimeout}
	} else if client.Timeout <= 0 {
		bounded := *client
		bounded.Timeout = mediaActionTimeout
		client = &bounded
	}

	reg := &MediaActionRegistry{
		byLayer: make(map[domain.LayerType]*layerMediaActions),
		sem:     make(chan struct{}, mediaActionMaxConcurrency),
		logger:  logger,
	}

	for _, cs := range sources {
		if cs == nil {
			continue
		}
		def := cs.Definition()
		if len(def.MediaActions) == 0 {
			continue
		}

		origin, err := originOf(def.Transport.URL)
		if err != nil {
			return nil, fmt.Errorf("source %q: media_actions need a source URL with an origin: %w", def.Name, err)
		}

		envResolve := cs.EnvResolve()
		if envResolve == nil {
			envResolve = func(string) string { return "" }
		}
		transport, err := NewTransport(def, cs.ResolvedHeaders(), cs.TokenProvider(), envResolve, client, logger.WithSource(def.Name))
		if err != nil {
			return nil, fmt.Errorf("source %q: build media action transport: %w", def.Name, err)
		}
		if ht, ok := transport.(*HTTPTransport); ok {
			ht.maxResponseBytes = mediaActionMaxResponseBytes
		}

		actions := make(map[string]MediaActionSpec, len(def.MediaActions))
		for _, a := range def.MediaActions {
			actions[a.Name] = a
		}

		// Last definition wins per layer, matching DynamicSourceRegistry.
		reg.byLayer[domain.LayerType(def.LayerType)] = &layerMediaActions{
			sourceName: def.Name,
			origin:     origin,
			actions:    actions,
			programs:   cs.MediaActions(),
			transport:  transport,
		}
	}

	return reg, nil
}

// ResolveMediaAction evaluates the declared path expression against the
// entity's merged metadata and admits the result as a relative path.
func (r *MediaActionRegistry) ResolveMediaAction(layerType domain.LayerType, name string, metadata map[string]string) (*domain.MediaPlaybackAction, error) {
	entry, ok := r.byLayer[layerType]
	if !ok {
		return nil, domain.NewNotFoundError(fmt.Sprintf("layer %q declares no media actions", layerType), nil)
	}
	spec, ok := entry.actions[name]
	if !ok {
		return nil, domain.NewNotFoundError(fmt.Sprintf("layer %q declares no media action %q", layerType, name), nil)
	}
	prg, ok := entry.programs[name]
	if !ok {
		return nil, fmt.Errorf("media action %q has no compiled path expression", name)
	}

	if metadata == nil {
		metadata = map[string]string{}
	}
	out, _, err := prg.Eval(map[string]interface{}{"metadata": metadata})
	if err != nil {
		return nil, fmt.Errorf("evaluate media action %q path: %w", name, err)
	}
	path, ok := out.Value().(string)
	if !ok {
		return nil, fmt.Errorf("media action %q path did not evaluate to a string", name)
	}

	if err := validateActionPath(path); err != nil {
		return nil, fmt.Errorf("media action %q: %w", name, err)
	}

	return &domain.MediaPlaybackAction{
		LayerType: layerType,
		Name:      name,
		Method:    spec.Method,
		Path:      path,
	}, nil
}

// ExecuteMediaAction performs the notification and discards the response. The
// caller learns only whether it succeeded.
func (r *MediaActionRegistry) ExecuteMediaAction(ctx context.Context, action *domain.MediaPlaybackAction) error {
	if action == nil {
		return fmt.Errorf("nil media action")
	}
	entry, ok := r.byLayer[action.LayerType]
	if !ok {
		return domain.NewNotFoundError(fmt.Sprintf("layer %q declares no media actions", action.LayerType), nil)
	}
	// Re-admit the path: the action crossed a package boundary since it was
	// resolved, and this is the last point before an outbound request.
	if err := validateActionPath(action.Path); err != nil {
		return err
	}
	if action.Method != http.MethodGet {
		return fmt.Errorf("media action %q: only GET is supported", action.Name)
	}

	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}

	target := entry.origin + action.Path
	if _, _, err := entry.transport.Fetch(ctx, http.MethodGet, target); err != nil {
		return fmt.Errorf("notify playback for source %q: %w", entry.sourceName, err)
	}
	return nil
}

// validateActionPath admits only an origin-relative path. Anything that could
// move the request to another origin, walk out of the declared path space, or
// smuggle a scheme is rejected.
func validateActionPath(path string) error {
	if path == "" {
		return fmt.Errorf("resolved path is empty")
	}
	if len(path) > mediaActionMaxPathLen {
		return fmt.Errorf("resolved path exceeds %d bytes", mediaActionMaxPathLen)
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("resolved path %q must start with /", path)
	}
	// "//host" is a protocol-relative URL; "/\host" is the backslash variant
	// browsers and some parsers also treat as an authority.
	if strings.HasPrefix(path, "//") || strings.HasPrefix(path, `/\`) {
		return fmt.Errorf("resolved path %q looks like an authority", path)
	}
	if strings.ContainsAny(path, "\\ \t\r\n") {
		return fmt.Errorf("resolved path %q contains an illegal character", path)
	}

	u, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("resolved path %q is not a valid URL path: %w", path, err)
	}
	if u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" {
		return fmt.Errorf("resolved path %q must be origin-relative", path)
	}
	if u.Fragment != "" || strings.Contains(path, "#") {
		return fmt.Errorf("resolved path %q must not carry a fragment", path)
	}
	// An action declares a path. A query would be something the source never
	// asked for, smuggled in through a metadata value.
	if u.RawQuery != "" || u.ForceQuery || strings.Contains(path, "?") {
		return fmt.Errorf("resolved path %q must not carry a query", path)
	}

	// Segment checks run on BOTH the escaped and the decoded forms. Go treats
	// "%2e%2e%2f%2e%2e" as one segment, so splitting the escaped path alone
	// never sees the traversal an upstream sees after decoding.
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
		return fmt.Errorf("resolved path %q must not encode a path separator", path)
	}
	decoded, err := url.PathUnescape(path)
	if err != nil {
		return fmt.Errorf("resolved path %q has invalid percent-encoding: %w", path, err)
	}
	if strings.ContainsAny(decoded, "\\ \t\r\n\x00") {
		return fmt.Errorf("resolved path %q contains an illegal character once decoded", path)
	}
	for _, form := range []string{escaped, strings.ToLower(decoded)} {
		for _, seg := range strings.Split(form, "/") {
			if seg == ".." || seg == "%2e%2e" {
				return fmt.Errorf("resolved path %q must not traverse upward", path)
			}
		}
	}
	return nil
}

// originOf extracts scheme://host from a source URL.
func originOf(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("URL %q has no origin", rawURL)
	}
	return u.Scheme + "://" + u.Host, nil
}
