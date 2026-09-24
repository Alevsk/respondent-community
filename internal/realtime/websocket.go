package realtime

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/ai/notify"
	"github.com/Alevsk/respondent/internal/domain"
)

// maxClientMessageSize is the maximum allowed size (in bytes) of a single
// WebSocket message sent by a client.  Control messages from the frontend
// (subscribe, viewport_update, time_range, etc.) are small JSON payloads,
// so 4 KiB is generous.
const maxClientMessageSize = 4096

// maxOnDemandResponseSize caps the body size we will read from an external
// on-demand API to prevent OOM from unexpectedly large responses.
const maxOnDemandResponseSize = 10 << 20 // 10 MiB

// maxSubscriptionsPerClient limits the number of layers a single client can
// subscribe to, preventing resource exhaustion from malicious or buggy clients.
const maxSubscriptionsPerClient = 100

// maxClients limits the total number of concurrent WebSocket connections.
// Each connection spawns 2 goroutines and allocates a 256-element send buffer.
const maxClients = 1000

// LayerRegistry is the read-only view of declarative layer-type metadata that the
// realtime server depends on. Every per-layer behavior the server exhibits
// (spatial viewport filtering, indicator rendering, history windows) is resolved
// through this registry, never from hardcoded layer names — the platform is fully
// declarative. *domain.DynamicSourceRegistry satisfies this interface; the server
// depends on the interface (not the concrete type) per the project's dependency-
// inversion rule, and it is the single source of truth for these lookups.
type LayerRegistry interface {
	LookupLayerType(st domain.SourceType) (domain.LayerType, bool)
	LookupFilteringMode(lt domain.LayerType) (string, bool)
	LookupRenderingMode(lt domain.LayerType) (string, bool)
	LookupIndicatorSpec(lt domain.LayerType) (*domain.IndicatorSpec, bool)
	LookupHistoryConfig(lt domain.LayerType) (*domain.HistoryConfig, bool)
}

// layerTypeForID resolves the layer type for an incoming layer/source identifier.
// Clients address a layer by its layer type, but a source type is also accepted;
// the registry maps a known source type to its layer type, otherwise the id is
// already a layer type. This replaces the former global-registry helper so all
// resolution flows through the injected registry.
func (s *Server) layerTypeForID(id string) domain.LayerType {
	if lt, ok := s.dynReg.LookupLayerType(domain.SourceType(id)); ok {
		return lt
	}
	return domain.LayerType(id)
}

// BackfillConfig holds configuration for Postgres backfill and time range queries.
type BackfillConfig struct {
	Enabled         bool           `json:"enabled"`
	StalenessWindow time.Duration  `json:"staleness_window"` // default: 30m
	Thresholds      map[string]int `json:"thresholds"`       // per-layer minimum entity count
	MaxResults      int            `json:"max_results"`      // default: 500
	QueryTimeout    time.Duration  `json:"query_timeout"`    // default: 3s
	Cooldown        time.Duration  `json:"cooldown"`         // default: 5s per client per layer
}

// TimeRangeConfig holds configuration for time range queries.
type TimeRangeConfig struct {
	Enabled      bool          `json:"enabled"`
	MaxLookback  time.Duration `json:"max_lookback"`   // default: 48h
	MaxRangeSpan time.Duration `json:"max_range_span"` // default: 24h
	QueryTimeout time.Duration `json:"query_timeout"`  // default: 5s
	Cooldown     time.Duration `json:"cooldown"`       // default: 2s
	MaxResults   int           `json:"max_results"`    // default: 500
}

// OnDemandConfig holds configuration for on-demand viewport fetching.
// When the cache is sparse for a viewport, the server fetches live data
// directly from the external API (e.g., adsb.lol) and writes it through
// the standard SetEntity → Pub/Sub pipeline.
type OnDemandConfig struct {
	Enabled       bool                     `json:"enabled"`
	Cooldown      time.Duration            `json:"cooldown"`       // per-client, per-layer (default: 10s)
	QueryTimeout  time.Duration            `json:"query_timeout"`  // HTTP request timeout (default: 5s)
	LayerAPIs     map[string]string        `json:"layer_apis"`     // layer_type -> API URL template
	LayerRadii    map[string]float64       `json:"layer_radii"`    // layer_type -> fetch radius (nm); when set, the viewport is tiled with a hex grid at this radius so the whole view is filled, not just its center
	MinCacheGap   int                      `json:"min_cache_gap"`  // Tier 1 deficit to trigger (default: 10)
	PollIntervals map[string]time.Duration `json:"poll_intervals"` // layer_type -> continuous re-poll interval
	// Transport, when set, is the HTTP RoundTripper for on-demand fetches — e.g. a
	// shared per-host rate limiter so on-demand shares the upstream request budget
	// with the feeder crawl. nil uses the default transport.
	Transport http.RoundTripper `json:"-"`
}

// DefaultOnDemandConfig returns sensible defaults for on-demand fetching.
func DefaultOnDemandConfig() OnDemandConfig {
	return OnDemandConfig{
		Enabled:      false,
		Cooldown:     10 * time.Second,
		QueryTimeout: 5 * time.Second,
		LayerAPIs:    nil,
		MinCacheGap:  10,
	}
}

// DefaultBackfillConfig returns sensible defaults for backfill.
func DefaultBackfillConfig() BackfillConfig {
	return BackfillConfig{
		Enabled:         true,
		StalenessWindow: 30 * time.Minute,
		Thresholds:      nil,
		MaxResults:      2000,
		QueryTimeout:    3 * time.Second,
		Cooldown:        5 * time.Second,
	}
}

// DefaultTimeRangeConfig returns sensible defaults for time range.
func DefaultTimeRangeConfig() TimeRangeConfig {
	return TimeRangeConfig{
		Enabled:      true,
		MaxLookback:  48 * time.Hour,
		MaxRangeSpan: 24 * time.Hour,
		QueryTimeout: 5 * time.Second,
		Cooldown:     2 * time.Second,
		MaxResults:   2000,
	}
}

// Client represents a WebSocket client connection
type Client struct {
	ID            string
	Conn          *websocket.Conn
	Send          chan []byte
	Logger        zerolog.Logger
	server        *Server
	subscriptions map[string]bool         // layer_id -> enabled
	viewports     map[string]*domain.BBox // layer_id -> viewport bbox
	// Per-client time range state
	timeFrom   map[string]time.Time // layer_id -> range start (zero = live mode)
	timeTo     map[string]time.Time // layer_id -> range end (zero = "now" / sliding)
	timeFrozen map[string]bool      // layer_id -> true if timeTo is in the past (suppress Pub/Sub)
	// Per-client cooldowns
	lastBackfill  map[string]time.Time // layer_id -> last backfill query time
	lastOnDemand  map[string]time.Time // layer_id -> last on-demand fetch time
	lastTimeRange map[string]time.Time // layer_id -> last time_range query time
	// Continuous on-demand polling tickers for sparse viewports
	onDemandTickers map[string]context.CancelFunc // layer_id -> cancel fn
	// globalLayers tracks layers subscribed in "global" mode (dashboard analytics).
	// Global mode skips viewport-based spatial filtering for both snapshots and
	// Pub/Sub updates, returning the full dataset instead.
	globalLayers map[string]bool
	// Per-client notification filter (nil = accept all ai_insight messages).
	notificationFilter *NotificationFilter
	mu                 sync.RWMutex
	// droppedCount tracks messages dropped due to a full send buffer.
	// Incremented atomically at each drop site; drained by writePump to send
	// a single backpressure notification inline (not via the Send channel).
	droppedCount atomic.Int64
}

// OnDemandParserFunc parses raw response bytes from an on-demand external API
// into entities and observations. Injected at construction time to decouple the
// transport layer from the ingest/parse package.
type OnDemandParserFunc func(body []byte, layerType string) ([]*domain.Entity, []*domain.Observation, error)

// AuthenticatorFunc validates a WebSocket connection request and returns an error
// if authentication fails. When nil, authentication is skipped (development mode).
// The function receives the raw *http.Request so it can inspect headers, query
// params, cookies, or any other request metadata.
type AuthenticatorFunc func(r *http.Request) error

// Compile-time interface check.
var _ notify.Broadcaster = (*Server)(nil)

// Server manages WebSocket connections and broadcasts
type Server struct {
	clients               map[*Client]bool
	register              chan *Client
	unregister            chan *Client
	broadcast             chan []byte
	notificationBroadcast chan notificationBroadcastMsg
	logger                zerolog.Logger
	obsRepo               domain.ObservationRepository
	entityRepo            domain.EntityRepository
	httpClient            *http.Client // for on-demand external API fetches
	dynReg                LayerRegistry
	backfillCfg           BackfillConfig
	timeRangeCfg          TimeRangeConfig
	onDemandCfg           OnDemandConfig
	onDemandParser        OnDemandParserFunc // injected parser for on-demand API responses
	// Pub/Sub batching: accumulate updates per layer, flush on timer
	pendingUpdates map[string][]pendingUpdate // layerID -> pending updates
	pendingMu      sync.Mutex
	// Allowed origins for WebSocket connections (passed to websocket.AcceptOptions).
	// Use []string{"*"} to allow all origins (development mode).
	// When empty, only same-origin connections are accepted.
	allowedOrigins []string
	// authenticator is an optional hook called before the WebSocket upgrade.
	// When nil, authentication is skipped (development / backwards-compatible mode).
	authenticator AuthenticatorFunc
	mu            sync.RWMutex
}

// NewServer creates a new WebSocket server. dynReg supplies all per-layer-type
// metadata (filtering mode, rendering mode, history, indicators); whether a layer
// uses viewport-based spatial filtering is resolved from it, not from a separate
// precomputed set, so there is a single source of truth and nothing to drift.
func NewServer(logger zerolog.Logger, entityRepo domain.EntityRepository, obsRepo domain.ObservationRepository, dynReg LayerRegistry) *Server {
	return &Server{
		clients:               make(map[*Client]bool),
		register:              make(chan *Client),
		unregister:            make(chan *Client),
		broadcast:             make(chan []byte, 256),
		notificationBroadcast: make(chan notificationBroadcastMsg, 256),
		logger:                logger,
		obsRepo:               obsRepo,
		entityRepo:            entityRepo,
		httpClient:            &http.Client{Timeout: 5 * time.Second},
		dynReg:                dynReg,
		backfillCfg:           DefaultBackfillConfig(),
		timeRangeCfg:          DefaultTimeRangeConfig(),
		onDemandCfg:           DefaultOnDemandConfig(),
		pendingUpdates:        make(map[string][]pendingUpdate),
	}
}

// SetAllowedOrigins configures the set of origin patterns permitted to open
// WebSocket connections.  Patterns are matched via Go's path.Match against the
// origin host.  Pass []string{"*"} to allow all origins (development mode).
// An empty slice uses the default same-origin policy (request host only).
func (s *Server) SetAllowedOrigins(origins []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedOrigins = origins
}

// SetAuthenticator registers an authentication hook that is called for every
// incoming WebSocket connection request before the upgrade is performed.
// Returning a non-nil error from fn causes the connection to be rejected with
// HTTP 401 Unauthorized. Passing nil disables authentication (default).
func (s *Server) SetAuthenticator(fn AuthenticatorFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticator = fn
}

// SetBackfillConfig sets the backfill configuration.
func (s *Server) SetBackfillConfig(cfg BackfillConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backfillCfg = cfg
}

// SetTimeRangeConfig sets the time range configuration.
func (s *Server) SetTimeRangeConfig(cfg TimeRangeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timeRangeCfg = cfg
}

// SetOnDemandConfig sets the on-demand fetch configuration.
// A dedicated HTTP client is created with the configured QueryTimeout to avoid
// mutating the shared httpClient.Timeout field from a concurrent goroutine.
func (s *Server) SetOnDemandConfig(cfg OnDemandConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDemandCfg = cfg
	if cfg.QueryTimeout > 0 {
		// Transport may be a shared rate limiter so on-demand fetches share the
		// upstream's request budget with the feeder crawl (nil = default transport).
		s.httpClient = &http.Client{Timeout: cfg.QueryTimeout, Transport: cfg.Transport}
	}
}

// SetOnDemandParser injects the parsing function used to decode raw on-demand
// API responses into domain entities and observations. Must be called before
// the server starts handling on-demand requests.
func (s *Server) SetOnDemandParser(fn OnDemandParserFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDemandParser = fn
}

// Config getters — thread-safe read access to fields that may be updated
// at runtime by the corresponding Set*Config methods. Structs containing
// maps are deep-copied so the caller never shares backing memory with a
// concurrent Set*Config writer (which would cause a map read/write panic).

func (s *Server) getBackfillConfig() BackfillConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.backfillCfg
	if s.backfillCfg.Thresholds != nil {
		cfg.Thresholds = make(map[string]int, len(s.backfillCfg.Thresholds))
		for k, v := range s.backfillCfg.Thresholds {
			cfg.Thresholds[k] = v
		}
	}
	return cfg
}

func (s *Server) getTimeRangeConfig() TimeRangeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.timeRangeCfg
}

func (s *Server) getOnDemandConfig() OnDemandConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.onDemandCfg
	if s.onDemandCfg.LayerAPIs != nil {
		cfg.LayerAPIs = make(map[string]string, len(s.onDemandCfg.LayerAPIs))
		for k, v := range s.onDemandCfg.LayerAPIs {
			cfg.LayerAPIs[k] = v
		}
	}
	if s.onDemandCfg.PollIntervals != nil {
		cfg.PollIntervals = make(map[string]time.Duration, len(s.onDemandCfg.PollIntervals))
		for k, v := range s.onDemandCfg.PollIntervals {
			cfg.PollIntervals[k] = v
		}
	}
	return cfg
}

func (s *Server) getHTTPClient() *http.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.httpClient
}

// getOnDemandPollInterval returns the continuous polling interval for a layer type.
// Returns 0 if no interval is configured (disables continuous polling for that type).
func (s *Server) getOnDemandPollInterval(layerType string) time.Duration {
	cfg := s.getOnDemandConfig()
	if cfg.PollIntervals == nil {
		return 0
	}
	return cfg.PollIntervals[layerType]
}

// isSpatialLayer reports whether the layer type uses viewport-based spatial
// filtering, as declared in its source definition. The filtering mode is resolved
// from the registry — the single source of truth — so this never hardcodes layer
// names and never drifts from the declared configuration.
func (s *Server) isSpatialLayer(layerType string) bool {
	mode, ok := s.dynReg.LookupFilteringMode(domain.LayerType(layerType))
	return ok && mode == string(domain.FilteringViewport)
}

// Clients returns a copy of the current set of connected clients (for testing)
func (s *Server) Clients() map[*Client]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[*Client]bool, len(s.clients))
	for k, v := range s.clients {
		cp[k] = v
	}
	return cp
}

// Register returns the register channel (for testing)
func (s *Server) Register() chan *Client {
	return s.register
}

// Unregister returns the unregister channel (for testing)
func (s *Server) Unregister() chan *Client {
	return s.unregister
}

// Run starts the WebSocket server
func (s *Server) Run(ctx context.Context) {
	// In-process update batching: updates queued via QueueUpdate (the feeder's
	// direct publisher bridge) are batched and delivered every 150ms instead of
	// flooding clients. The community edition is a single binary with no external
	// pub/sub broker, so this is the only delivery path.
	go s.runFlushTicker(ctx)

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: close all client connections and Send channels.
			// This unblocks readPump (conn.Read returns error) and writePump
			// (range over Send exits), preventing goroutine leaks.
			s.mu.Lock()
			for client := range s.clients {
				client.stopAllOnDemandTickers()
				close(client.Send)
				if client.Conn != nil {
					_ = client.Conn.CloseNow()
				}
				delete(s.clients, client)
			}
			s.mu.Unlock()
			s.logger.Info().Msg("websocket server shut down, all clients disconnected")
			return

		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			s.mu.Unlock()
			s.logger.Info().Str("id", client.ID).Msg("client registered")

		case client := <-s.unregister:
			client.stopAllOnDemandTickers()
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.Send)
				s.logger.Info().Str("id", client.ID).Msg("client unregistered")
			}
			s.mu.Unlock()

		case message := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.Send <- message:
				default:
					// Channel full — increment the counter so writePump can
					// notify the client about the drop via a backpressure frame.
					client.droppedCount.Add(1)
					s.logger.Warn().Str("id", client.ID).Msg("client send buffer full, dropping")
				}
			}
			s.mu.RUnlock()

		case nb := <-s.notificationBroadcast:
			s.mu.RLock()
			for client := range s.clients {
				client.mu.RLock()
				passes := matchesNotificationFilter(client.notificationFilter, nb.Meta)
				client.mu.RUnlock()
				if !passes {
					continue
				}
				select {
				case client.Send <- nb.Data:
				default:
					client.droppedCount.Add(1)
					s.logger.Warn().Str("id", client.ID).Msg("client send buffer full, dropping notification")
				}
			}
			s.mu.RUnlock()
		}
	}
}

// HandleConnection handles a new WebSocket connection.
// Capacity and origin checks happen here — before the WebSocket upgrade and
// goroutine launch — so a rejected connection never allocates a Send channel
// or spawns readPump/writePump goroutines.
func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock to avoid races with SetAllowedOrigins,
	// SetAuthenticator, and to pre-check capacity before the expensive upgrade.
	s.mu.RLock()
	atCapacity := len(s.clients) >= maxClients
	origins := s.allowedOrigins
	authenticator := s.authenticator
	s.mu.RUnlock()

	if atCapacity {
		s.logger.Warn().Int("max", maxClients).Msg("max client limit reached, rejecting connection")
		http.Error(w, "server at capacity", http.StatusServiceUnavailable)
		return
	}

	if authenticator != nil {
		if err := authenticator(r); err != nil {
			s.logger.Warn().Err(err).
				Str("remote_addr", r.RemoteAddr).
				Msg("websocket authentication failed")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	opts := &websocket.AcceptOptions{}
	if len(origins) > 0 {
		opts.OriginPatterns = origins
	}

	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to accept websocket connection")
		return
	}
	conn.SetReadLimit(maxClientMessageSize)

	client := NewClient(generateClientID(), conn, make(chan []byte, 256), s.logger)
	client.server = s

	s.register <- client

	// Start read and write pumps
	go client.readPump(s.unregister)
	go client.writePump()
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type    string `json:"type"`
	LayerID string `json:"layer_id,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func generateClientID() string {
	return uuid.New().String()
}
