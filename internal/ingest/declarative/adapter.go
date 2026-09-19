package declarative

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest"
	"github.com/Alevsk/respondent/internal/ingest/declarative/parsers"
	"github.com/Alevsk/respondent/internal/logging"
)

// Ensure DeclarativeAdapter implements ingest.LayerSource at compile time.
var _ ingest.LayerSource = (*DeclarativeAdapter)(nil)

const (
	// maxEntityIDLen is the maximum length for entity external_id strings.
	maxEntityIDLen = 1024
	// maxEntityNameLen is the maximum length for entity name strings.
	maxEntityNameLen = 512
	// maxMetadataValueLen is the maximum length for metadata value strings.
	maxMetadataValueLen = 4096
	// maxMetadataKeys is the maximum number of metadata keys per entity/observation.
	maxMetadataKeys = 50
	// maxLoggedErrors is the number of per-record CEL errors to log before suppressing.
	maxLoggedErrors = 10
	// defaultMaxResponseBytes is the default response size limit (50MB).
	defaultMaxResponseBytes int64 = 50 * 1024 * 1024
)

// DeclarativeAdapter implements ingest.LayerSource for YAML-defined sources.
// It uses an atomic pointer for hot-reload of CompiledSource without restart.
type DeclarativeAdapter struct {
	compiled  atomic.Pointer[CompiledSource]
	transport Transport
	client    *http.Client
	parser    parsers.RecordParser
	logger    *logging.Logger

	// BaseAdapter fields (mirrors adapters.BaseAdapter pattern)
	name         string
	layerType    string
	mu           sync.RWMutex
	entities     []*domain.Entity
	observations []*domain.Observation

	// Clock for testability (injected from CompiledSource)
	clock Clock

	// Streaming/listening lifecycle guard — prevents the ticker from
	// spawning duplicate reconnect loops on every Start() call.
	bgStarted atomic.Bool

	// Spatial crawl state (Phase 3)
	spatialMu        sync.Mutex
	spatial          *spatialState
	spatialBatchSize int
	ecache           *entityCache
}

// NewDeclarativeAdapter creates a new declarative adapter from a compiled source.
func NewDeclarativeAdapter(cs *CompiledSource, logger *logging.Logger) (*DeclarativeAdapter, error) {
	return NewDeclarativeAdapterWithClient(cs, logger, nil)
}

// NewDeclarativeAdapterWithClient creates a new declarative adapter with an optional custom HTTP client.
// If client is nil, an SSRF-safe client is created from the transport timeout.
func NewDeclarativeAdapterWithClient(cs *CompiledSource, logger *logging.Logger, client *http.Client) (*DeclarativeAdapter, error) {
	def := cs.Definition()

	parser, err := parsers.NewParser(def.Parser.Format)
	if err != nil {
		return nil, fmt.Errorf("create parser for source %q: %w", def.Name, err)
	}

	if client == nil {
		timeout := def.Transport.Timeout.Duration
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		// TODO: Re-enable SSRF-safe client once TLS fingerprint issues are resolved.
		// Some servers (e.g. tle.ivanstanojevic.me) reject Go's TLS client hello when
		// a custom DialContext is used. Using standard http.Client for now.
		// See: NewSSRFSafeClient in safeclient.go
		client = &http.Client{Timeout: timeout}
	}

	adapterLogger := logger.WithSource(def.Name)

	envResolve := cs.EnvResolve()
	if envResolve == nil {
		envResolve = func(string) string { return "" }
	}

	transport, err := NewTransport(def, cs.ResolvedHeaders(), cs.TokenProvider(), envResolve, client, adapterLogger)
	if err != nil {
		return nil, fmt.Errorf("create transport for source %q: %w", def.Name, err)
	}

	adapter := &DeclarativeAdapter{
		transport:    transport,
		client:       client,
		parser:       parser,
		logger:       adapterLogger,
		name:         def.Name,
		layerType:    def.LayerType,
		clock:        cs.SourceClock(),
		entities:     make([]*domain.Entity, 0),
		observations: make([]*domain.Observation, 0),
	}
	adapter.compiled.Store(cs)

	return adapter, nil
}

// Name returns the source name.
func (a *DeclarativeAdapter) Name() string {
	return a.name
}

// LayerType returns the layer type identifier.
func (a *DeclarativeAdapter) LayerType() string {
	return a.layerType
}

// SupportsSourceType returns true if this adapter handles the given source type.
func (a *DeclarativeAdapter) SupportsSourceType(sourceType domain.SourceType) bool {
	cs := a.compiled.Load()
	if cs == nil {
		return false
	}
	return domain.SourceType(cs.Definition().SourceType) == sourceType
}

// Start begins the polling loop. The Manager calls this once in a goroutine.
// For declarative adapters used with the existing IngestionService ticker pattern,
// this performs a single fetch cycle (matching the EarthquakesAdapter pattern).
// For streaming transports, it runs the connect/recv loop with reconnection.
// For listening transports, it starts the inbound listener.
func (a *DeclarativeAdapter) Start(ctx context.Context) error {
	a.logger.Info("starting declarative adapter",
		logging.String("source_name", a.name),
		logging.String("layer_type", a.layerType),
	)

	// Detect transport type and dispatch accordingly.
	// Streaming and listening transports run in background goroutines so Start()
	// returns immediately — the ingestion service ticker then calls Snapshot()
	// periodically to persist accumulated entities to the database.
	//
	// bgStarted guards against the ticker spawning duplicate reconnect loops:
	// the service calls Start() on every tick, but streaming/listening
	// transports must only launch their background goroutine once.
	switch t := a.transport.(type) {
	case StreamTransport:
		if !a.bgStarted.CompareAndSwap(false, true) {
			return nil // already running
		}
		go func() {
			defer a.bgStarted.Store(false) // allow restart on next tick if loop exits
			if err := a.startStreaming(ctx, t); err != nil && ctx.Err() == nil {
				a.logger.Error("streaming loop exited with error",
					logging.String("source_name", a.name),
					logging.Err("error", err),
				)
			}
		}()
		return nil
	case ListenTransport:
		if !a.bgStarted.CompareAndSwap(false, true) {
			return nil // already running
		}
		go func() {
			defer a.bgStarted.Store(false)
			if err := a.startListening(ctx, t); err != nil && ctx.Err() == nil {
				a.logger.Error("listener loop exited with error",
					logging.String("source_name", a.name),
					logging.Err("error", err),
				)
			}
		}()
		return nil
	default:
		return a.fetchAndProcess(ctx)
	}
}

// Stop gracefully stops the adapter.
func (a *DeclarativeAdapter) Stop() error {
	return nil
}

// Snapshot returns the stored entities and observations.
func (a *DeclarativeAdapter) Snapshot(ctx context.Context) ([]*domain.Entity, []*domain.Observation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entities := make([]*domain.Entity, len(a.entities))
	copy(entities, a.entities)

	observations := make([]*domain.Observation, len(a.observations))
	copy(observations, a.observations)

	return entities, observations, nil
}

// Stream is a no-op for polling-based declarative adapters.
func (a *DeclarativeAdapter) Stream(ctx context.Context, out chan<- *domain.EntityUpdate) error {
	return nil
}

// SetEntities stores entities and observations (thread-safe).
func (a *DeclarativeAdapter) SetEntities(entities []*domain.Entity, observations []*domain.Observation) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entities = entities
	a.observations = observations
}

// SwapCompiledSource atomically replaces the compiled source for hot-reload.
// If the underlying transport supports header/token updates, they are refreshed.
func (a *DeclarativeAdapter) SwapCompiledSource(cs *CompiledSource) {
	a.compiled.Store(cs)
	// Update transport headers and token provider if the implementation supports it.
	if ht, ok := a.transport.(*HTTPTransport); ok {
		ht.UpdateHeaders(cs.ResolvedHeaders())
		ht.UpdateTokenProvider(cs.TokenProvider())
	}
}

// fetchAndProcess performs a single fetch-parse-eval cycle.
// If the source has spatial configuration, delegates to fetchSpatial.
// If pagination is configured, delegates to fetchPaginated.
func (a *DeclarativeAdapter) fetchAndProcess(ctx context.Context) error {
	cs := a.compiled.Load()
	if cs == nil {
		return fmt.Errorf("no compiled source loaded for %q", a.name)
	}

	def := cs.Definition()

	// Spatial mode: delegate to spatial crawl loop
	if def.Transport.Spatial != nil {
		return a.fetchSpatial(ctx)
	}

	// If pagination is configured, use the paginated fetch path.
	if def.Transport.Pagination != nil {
		return a.fetchPaginated(ctx, cs)
	}

	fetchStart := time.Now()

	body, statusCode, err := a.fetchWithRetry(ctx, cs, def.Transport.Method, def.Transport.URL)
	fetchDuration := time.Since(fetchStart)

	a.logger.Debug("fetch completed",
		logging.String("source_name", a.name),
		logging.Any("fetch_duration_ms", fetchDuration.Milliseconds()),
		logging.Int("http_status", statusCode),
	)

	if err != nil {
		return fmt.Errorf("fetch failed for source %q: %w", a.name, err)
	}

	records, err := a.parseBody(def, body)
	if err != nil {
		return err
	}

	a.logger.Debug("parsed records",
		logging.String("source_name", a.name),
		logging.Int("records_received", len(records)),
	)

	// Process records through CEL filter + mapping
	entities, observations := a.processRecords(cs, records)

	if len(entities) == 0 && len(records) > 0 {
		a.logger.Warn("empty batch: all records failed or were filtered",
			logging.String("source_name", a.name),
			logging.Int("records_received", len(records)),
		)
	}

	a.SetEntities(entities, observations)

	a.logger.Info("processed declarative source",
		logging.String("source_name", a.name),
		logging.Int("records_received", len(records)),
		logging.Int("entities_produced", len(entities)),
	)

	return nil
}

// fetchWithRetry performs the HTTP fetch with retry logic per the transport.retry spec.
// The fetchURL parameter allows paginated fetches to pass page-specific URLs.
func (a *DeclarativeAdapter) fetchWithRetry(ctx context.Context, cs *CompiledSource, method string, fetchURL string) ([]byte, int, error) {
	return a.transport.Fetch(ctx, method, fetchURL)
}
