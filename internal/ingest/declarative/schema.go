// Package declarative provides a YAML-driven, zero-code source onboarding system.
// Source definitions are parsed, validated, and compiled at config load time (control plane),
// then evaluated repeatedly at runtime (data plane).
package declarative

import (
	"fmt"
	"regexp"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
)

// sourceNameRE validates source names: lowercase start, alphanumeric + underscore, max 64 chars.
// Prevents NATS subject injection (wildcards, dots) and SQL weirdness.
var sourceNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// recordsPathRE validates dot-separated path segments with max depth of 8.
// Hyphens are allowed within a segment because real JSON APIs use them as object
// keys (e.g. NGA MSI's "broadcast-warn" envelope); the path is split only on ".".
var recordsPathRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+(\.[a-zA-Z0-9_-]+){0,7}$`)

// Duration is a custom type that unmarshals YAML duration strings ("10s", "5m")
// into time.Duration. gopkg.in/yaml.v3 does not natively support time.Duration.
type Duration struct {
	time.Duration
}

// UnmarshalYAML parses a YAML string value into a Go time.Duration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

// MarshalYAML encodes the Duration back to a YAML string.
func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

// SourceDefinition is the top-level declarative source configuration.
// Flat structure -- no Kubernetes-style apiVersion/kind wrapper.
type SourceDefinition struct {
	SchemaVersion int               `yaml:"schema_version" validate:"required,oneof=1 2"`
	Name          string            `yaml:"name" validate:"required,source_name"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	SourceType    string            `yaml:"source_type" validate:"required,source_name"`
	LayerType     string            `yaml:"layer_type" validate:"required,source_name"`
	DisplayName   string            `yaml:"display_name" validate:"required"`
	// LayerDisplayName optionally overrides the human-readable LAYER label shown in
	// the UI layers panel. When omitted, the layer is labeled by title-casing
	// layer_type (FormatLayerName). DisplayName above names the SOURCE; this names
	// the LAYER. Set it when the title-cased layer_type loses meaning (e.g. a
	// country qualifier: "Traffic Stations (Mexico)").
	LayerDisplayName string `yaml:"layer_display_name,omitempty"`

	// Operational controls — both default to sensible values when omitted.
	// enabled: true (default) — set to false to skip loading this source entirely.
	// dry_run: false (default) — set to true to fetch+parse+log but NOT persist to DB/cache.
	Enabled *bool `yaml:"enabled,omitempty"` // pointer so we can distinguish unset (nil=true) from explicit false
	DryRun  bool  `yaml:"dry_run,omitempty"`

	// v2 additions (all optional for backward compat with schema_version: 1)
	Filtering   string           `yaml:"filtering,omitempty" validate:"omitempty,oneof=viewport all"`
	Backfill    *BackfillSpec    `yaml:"backfill,omitempty"`
	EntityCache *EntityCacheSpec `yaml:"entity_cache,omitempty"`
	GeoCache    *GeoCacheSpec    `yaml:"geo_cache,omitempty"`

	// EntityType discriminates between geographic and non-geographic entities.
	// "geo_entity" (default when omitted) has lat/lon; "global_indicator" has no coordinates.
	EntityType string `yaml:"entity_type,omitempty" validate:"omitempty,oneof=geo_entity global_indicator"`

	LookupTables []LookupTableSpec `yaml:"lookup_tables,omitempty"`

	Transport     TransportSpec     `yaml:"transport" validate:"required"`
	Parser        ParserSpec        `yaml:"parser" validate:"required"`
	Filter        string            `yaml:"filter,omitempty"`
	Entity        EntityMapping     `yaml:"entity" validate:"required"`
	Observation   ObsMapping        `yaml:"observation" validate:"required"`
	Recording     RecordingSpec     `yaml:"recording" validate:"required"`
	Cache         CacheSpec         `yaml:"cache" validate:"required"`
	Display       DisplaySpec       `yaml:"display" validate:"required"`
	MediaActions  []MediaActionSpec `yaml:"media_actions,omitempty"`
	History       *HistorySpec      `yaml:"history,omitempty"`
	Indicator     *IndicatorSpec    `yaml:"indicator,omitempty"`
	FieldMappings []FieldMapping    `yaml:"field_mappings,omitempty"`

	// AI enrichment configuration (optional).
	// When present and enabled, AI operations are validated at load time:
	// CEL filters are compiled, output schemas are registered in the schema registry.
	AI *aiconfig.SourceAIConfig `yaml:"ai,omitempty"`
}

// IsEnabled returns whether this source should be loaded. Defaults to true when omitted.
func (sd *SourceDefinition) IsEnabled() bool {
	return sd.Enabled == nil || *sd.Enabled
}

// TransportSpec defines how data is fetched from the source.
type TransportSpec struct {
	Type             string            `yaml:"type" validate:"required,oneof=http_poll websocket sse mqtt webhook pubsub grpc_stream kafka amqp tcp_udp ftp_sftp s3_poll nats"`
	URL              string            `yaml:"url" validate:"omitempty"`
	OnDemandURL      string            `yaml:"on_demand_url,omitempty" validate:"omitempty,url"`
	Method           string            `yaml:"method" validate:"omitempty,oneof=GET POST"`
	Headers          map[string]string `yaml:"headers,omitempty"`
	Auth             *AuthSpec         `yaml:"auth,omitempty"`
	Timeout          Duration          `yaml:"timeout"`
	Interval         Duration          `yaml:"interval"`
	MaxResponseBytes int64             `yaml:"max_response_bytes" validate:"omitempty,min=1,max=104857600"`
	Retry            *RetrySpec        `yaml:"retry,omitempty"`
	Pagination       *PaginationSpec   `yaml:"pagination,omitempty"`
	Spatial          *SpatialSpec      `yaml:"spatial,omitempty"`

	// Transport-specific configurations
	WebSocket  *WebSocketSpec  `yaml:"websocket,omitempty"`
	SSE        *SSESpec        `yaml:"sse,omitempty"`
	MQTT       *MQTTSpec       `yaml:"mqtt,omitempty"`
	Webhook    *WebhookSpec    `yaml:"webhook,omitempty"`
	GRPCStream *GRPCStreamSpec `yaml:"grpc_stream,omitempty"`
	Kafka      *KafkaSpec      `yaml:"kafka,omitempty"`
	AMQP       *AMQPSpec       `yaml:"amqp,omitempty"`
	TCPUDP     *TCPUDPSpec     `yaml:"tcp_udp,omitempty"`
	FTPSFTP    *FTPSFTPSpec    `yaml:"ftp_sftp,omitempty"`
	S3Poll     *S3PollSpec     `yaml:"s3_poll,omitempty"`
	NATS       *NATSSpec       `yaml:"nats,omitempty"`

	// Streaming behavior
	Batching  *BatchingSpec  `yaml:"batching,omitempty"`
	Reconnect *ReconnectSpec `yaml:"reconnect,omitempty"`

	// RateLimit caps the request rate to this source's host (shared across the
	// feeder crawl and on-demand viewport fills hitting the same host).
	RateLimit *RateLimitSpec `yaml:"rate_limit,omitempty"`
}

// RateLimitSpec declares a per-host request budget so multiple sources (and
// on-demand fills) hitting the same upstream stay under its limit.
type RateLimitSpec struct {
	RequestsPerSecond float64 `yaml:"requests_per_second" validate:"required,gt=0"`
	Burst             int     `yaml:"burst,omitempty" validate:"omitempty,min=1"`
}

// PaginationSpec defines how paginated API responses are fetched.
// Supported types: page_number (page=1,2,3...), offset (offset=0,100,200...), cursor (cursor=abc...).
type PaginationSpec struct {
	Type       string `yaml:"type" validate:"required,oneof=page_number offset cursor"`
	PageParam  string `yaml:"page_param" validate:"required"`
	SizeParam  string `yaml:"size_param,omitempty"`
	Size       int    `yaml:"size" validate:"required,min=1,max=10000"`
	MaxPages   int    `yaml:"max_pages" validate:"required,min=1,max=1000"`
	StopWhen   string `yaml:"stop_when,omitempty"` // CEL expression evaluated per page
	CursorPath string `yaml:"cursor_path,omitempty"`
}

// AuthSpec defines inline credential configuration for a source.
// Secrets are never stored in YAML — only env var names that are resolved at runtime.
type AuthSpec struct {
	Type   string `yaml:"type" validate:"required,oneof=bearer api_key oauth2"`
	Header string `yaml:"header,omitempty"`
	EnvVar string `yaml:"env_var,omitempty"` // required for bearer/api_key, not for oauth2
	// OAuth2 fields (only when type=oauth2)
	OAuth2 *OAuth2Spec `yaml:"oauth2,omitempty"`
}

// OAuth2Spec defines OAuth2 token exchange configuration.
type OAuth2Spec struct {
	TokenURL    string            `yaml:"token_url" validate:"required,url"`
	GrantType   string            `yaml:"grant_type" validate:"required,oneof=password client_credentials"`
	Credentials OAuth2Credentials `yaml:"credentials" validate:"required"`
	// Response field mapping (defaults provided)
	ResponseMapping *OAuth2ResponseMapping `yaml:"response_mapping,omitempty"`
	// How to send the token on data requests
	TokenHeader string `yaml:"token_header,omitempty"` // default: "Authorization"
	TokenPrefix string `yaml:"token_prefix,omitempty"` // default: "Bearer "
	// Safety margin before expiry
	RefreshBeforeExpiry Duration `yaml:"refresh_before_expiry,omitempty"` // default: 300s
	// Extra static form fields to include in token requests
	ExtraParams map[string]string `yaml:"extra_params,omitempty"`
}

// OAuth2Credentials defines the credential fields for token exchange.
// Each field is either a literal string or an env var reference.
type OAuth2Credentials struct {
	ClientID     OAuth2Value `yaml:"client_id,omitempty"`
	ClientSecret OAuth2Value `yaml:"client_secret,omitempty"`
	Username     OAuth2Value `yaml:"username,omitempty"`
	Password     OAuth2Value `yaml:"password,omitempty"`
}

// OAuth2Value is a string that can be either a literal or an env var reference.
// If EnvVar is set, the value is resolved from RESPONDENT_<env_var> at runtime.
// If EnvVar is empty, the literal Value is used directly.
type OAuth2Value struct {
	Value  string `yaml:"value,omitempty"`
	EnvVar string `yaml:"env_var,omitempty"`
}

// UnmarshalYAML allows OAuth2Value to be specified as either a plain string
// (treated as a literal value) or as a mapping with value/env_var fields.
func (v *OAuth2Value) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try as a plain string first (literal value).
	var s string
	if err := unmarshal(&s); err == nil {
		v.Value = s
		return nil
	}
	// Otherwise, unmarshal as a struct.
	type raw OAuth2Value
	var r raw
	if err := unmarshal(&r); err != nil {
		return err
	}
	*v = OAuth2Value(r)
	return nil
}

// OAuth2ResponseMapping defines where to find tokens in the JSON response.
type OAuth2ResponseMapping struct {
	AccessToken  string `yaml:"access_token,omitempty"`  // default: "access_token"
	RefreshToken string `yaml:"refresh_token,omitempty"` // default: "refresh_token"
	ExpiresIn    string `yaml:"expires_in,omitempty"`    // default: "expires_in"
}

// RetrySpec defines retry behavior for HTTP fetches.
type RetrySpec struct {
	MaxAttempts  int      `yaml:"max_attempts" validate:"min=0,max=10"`
	Backoff      string   `yaml:"backoff" validate:"oneof=exponential fixed"`
	InitialDelay Duration `yaml:"initial_delay"`
	MaxDelay     Duration `yaml:"max_delay"`
}

// ParserSpec defines how the HTTP response is parsed into records.
type ParserSpec struct {
	Format       string   `yaml:"format" validate:"required,oneof=json json_table geojson csv tle xml rss"`
	RecordsPath  string   `yaml:"records_path,omitempty" validate:"omitempty,records_path"`
	MaxRecords   int      `yaml:"max_records" validate:"omitempty,min=1,max=100000"`
	CSVOptions   *CSVOpts `yaml:"csv_options,omitempty"`
	ArrayColumns []string `yaml:"array_columns,omitempty"` // v2: map array indices to named fields

	// v2: JSON reshaping transforms.
	// ObjectToRecords converts a top-level JSON object {"k": {...}, ...} into an
	// array of its values [{...}, ...]. Useful for APIs that return dictionaries
	// keyed by ID (e.g., SondeHub /sondes returns {"serial": {...}, ...}).
	ObjectToRecords bool `yaml:"object_to_records,omitempty"`
	// ObjectKeyField injects the object key into each record under this field name.
	// Only meaningful when ObjectToRecords is true.
	ObjectKeyField string `yaml:"object_key_field,omitempty"`

	// ArrayOfArrays treats a JSON array-of-arrays as a table where the first row
	// is column headers and subsequent rows are data. Converts to array-of-objects.
	// Useful for APIs like NOAA SWPC that return [["col1","col2"],[val1,val2],...].
	ArrayOfArrays bool `yaml:"array_of_arrays,omitempty"`
}

// CSVOpts holds CSV-specific parsing options.
type CSVOpts struct {
	Delimiter          string `yaml:"delimiter"`
	HasHeader          bool   `yaml:"has_header"`
	SkipLines          int    `yaml:"skip_lines"`
	CollapseWhitespace bool   `yaml:"collapse_whitespace"`
	CommentPrefix      string `yaml:"comment_prefix"`
}

// EntityMapping defines CEL expressions for mapping records to domain entities.
type EntityMapping struct {
	ExternalID string            `yaml:"external_id" validate:"required"`
	Name       string            `yaml:"name" validate:"required"`
	Metadata   map[string]string `yaml:"metadata,omitempty"`
}

// ObsMapping defines CEL expressions for mapping records to observations.
// Latitude and Longitude are required for geo_entity sources but optional for global_indicator.
type ObsMapping struct {
	Latitude    string            `yaml:"latitude"`
	Longitude   string            `yaml:"longitude"`
	Altitude    string            `yaml:"altitude"`
	Timestamp   string            `yaml:"timestamp" validate:"required"`
	EventTime   string            `yaml:"event_time,omitempty"`
	EventEnd    string            `yaml:"event_end,omitempty"`
	Velocity    map[string]string `yaml:"velocity,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	ContentHash string            `yaml:"content_hash,omitempty"`
}

// FieldMapping defines a generalized CEL mapping from source data to domain structures.
type FieldMapping struct {
	Source string `yaml:"source" validate:"required"`
	Target string `yaml:"target" validate:"required"`
	Type   string `yaml:"type" validate:"required,oneof=timestamp string integer float boolean"`
}

// RecordingSpec defines how observations are recorded.
type RecordingSpec struct {
	Mode string `yaml:"mode" validate:"required,oneof=append upsert dedupe"`
}

// CacheSpec defines caching behavior.
type CacheSpec struct {
	TTL Duration `yaml:"ttl" validate:"required"`
}

// DisplaySpec defines frontend rendering configuration.
type DisplaySpec struct {
	Media          []MediaSpec         `yaml:"media,omitempty"`
	Icon           IconSpec            `yaml:"icon"`
	Trail          TrailSpec           `yaml:"trail"`
	Style          StyleSpec           `yaml:"style"`
	FieldRenderers []FieldRendererSpec `yaml:"field_renderers,omitempty"`
	ColorBy        *ColorBySpec        `yaml:"color_by,omitempty"`
}

// ColorBySpec colors entities by the value of a metadata field. Keeps per-value
// color rules in the source YAML rather than hardcoded in the client.
type ColorBySpec struct {
	Field        string            `yaml:"field" validate:"required"`
	Values       map[string]string `yaml:"values" validate:"required,min=1,dive,hexcolor"`
	DefaultColor string            `yaml:"default_color" validate:"omitempty,hexcolor"`
}

// IconSpec defines map icon rendering.
type IconSpec struct {
	Shape         string  `yaml:"shape" validate:"required"`
	Rotatable     bool    `yaml:"rotatable"`
	Interpolation bool    `yaml:"interpolation"`
	Scale         float64 `yaml:"scale" validate:"min=0.1,max=5.0"`
}

// TrailSpec defines entity trail rendering.
type TrailSpec struct {
	Color   string  `yaml:"color" validate:"required,hexcolor"`
	Width   float64 `yaml:"width" validate:"omitempty,min=0.5,max=10.0"`
	Opacity float64 `yaml:"opacity" validate:"omitempty,min=0.0,max=1.0"`
}

// StyleSpec defines base point styling.
type StyleSpec struct {
	Color     string `yaml:"color" validate:"required,hexcolor"`
	PointSize int    `yaml:"point_size" validate:"required,min=1,max=32"`
}

// FieldRendererSpec defines how a metadata field is displayed in the frontend.
type FieldRendererSpec struct {
	Keys     []string   `yaml:"keys" validate:"required,min=1"`
	Label    string     `yaml:"label" validate:"required"`
	Format   FormatSpec `yaml:"format" validate:"required"`
	Priority int        `yaml:"priority"`
}

// FormatSpec is a discriminated union for field formatting.
type FormatSpec struct {
	Type      string `yaml:"type" validate:"required,oneof=string float integer raw"`
	Precision *int   `yaml:"precision,omitempty"`
	Prefix    string `yaml:"prefix,omitempty"`
	Suffix    string `yaml:"suffix,omitempty"`
	Transform string `yaml:"transform,omitempty" validate:"omitempty,oneof=upper lower"`
}

// SpatialSpec defines spatial coverage (URL templating + grid) for v2 sources.
type SpatialSpec struct {
	Type          string         `yaml:"type" validate:"required,oneof=hex_grid static_regions"`
	RadiusNM      float64        `yaml:"radius_nm,omitempty" validate:"omitempty,min=10,max=5000"`
	TargetRefresh Duration       `yaml:"target_refresh,omitempty"`
	BatchSize     int            `yaml:"batch_size,omitempty" validate:"omitempty,min=0,max=500"`
	Regions       []StaticRegion `yaml:"regions,omitempty"`

	// LatMin/LatMax filter out grid regions outside the useful latitude band.
	// For flights, set lat_min: -60 to skip Antarctica and lat_max: 75 to skip the Arctic.
	// Reduces initial scan time from ~10min to <1min by pruning ~40% of grid regions.
	LatMin *float64 `yaml:"lat_min,omitempty" validate:"omitempty,min=-90,max=90"`
	LatMax *float64 `yaml:"lat_max,omitempty" validate:"omitempty,min=-90,max=90"`
}

// StaticRegion defines a fixed geographic region for spatial crawling.
type StaticRegion struct {
	Lat   float64 `yaml:"lat" validate:"required,latitude"`
	Lon   float64 `yaml:"lon" validate:"required,longitude"`
	Label string  `yaml:"label,omitempty"`
}

// EntityCacheSpec defines entity-level caching for accumulation across spatial batches.
type EntityCacheSpec struct {
	Enabled    bool     `yaml:"enabled"`
	Key        string   `yaml:"key" validate:"required"` // CEL expression for cache key
	TTL        Duration `yaml:"ttl" validate:"required"`
	Accumulate bool     `yaml:"accumulate"`
}

// BackfillSpec defines sparse-region backfill thresholds.
type BackfillSpec struct {
	Threshold int `yaml:"threshold" validate:"min=1,max=10000"`
}

// GeoCacheSpec defines geo sorted set maintenance configuration for spatial layers.
// Controls over-fetching and lazy GC of stale geo index members.
type GeoCacheSpec struct {
	OverfetchRatio      float64 `yaml:"overfetch_ratio" validate:"omitempty,min=1.0,max=10.0"`
	AliveRatioThreshold float64 `yaml:"alive_ratio_threshold" validate:"omitempty,min=0.1,max=1.0"`
	GCBatchSize         int     `yaml:"gc_batch_size" validate:"omitempty,min=10,max=10000"`
}

// HistorySpec defines per-layer time range limits for historical exploration.
// When present, the frontend custom range picker and backend WebSocket handler
// use these limits instead of the server defaults (48h lookback, 24h span).
type HistorySpec struct {
	MaxLookback  Duration `yaml:"max_lookback"`   // e.g., "8760h" for 1 year
	MaxRangeSpan Duration `yaml:"max_range_span"` // e.g., "168h" for 1 week
}

// IndicatorSpec defines how a global indicator entity's metadata fields
// are mapped to structured indicator values for HUD rendering.
type IndicatorSpec struct {
	Values []IndicatorValueSpec `yaml:"values"`
	// SummaryExpr is a CEL expression that computes a human-readable summary.
	// Variables: level (int), metadata (map[string]string).
	// When omitted, a default severity scale (Quiet → Extreme) is used.
	SummaryExpr string `yaml:"summary_expr,omitempty"`
}

// IndicatorValueSpec defines a single indicator reading within an IndicatorSpec.
type IndicatorValueSpec struct {
	Key             string    `yaml:"key"`
	Label           string    `yaml:"label"`
	SourceField     string    `yaml:"source_field"`
	Unit            string    `yaml:"unit"`
	MaxLevel        int       `yaml:"max_level"`
	LevelThresholds []float64 `yaml:"level_thresholds"`
	// LevelExpr is a CEL expression that computes the severity level from a raw value.
	// Variables: value (string), change_pct (string). Must return an int.
	// When omitted, the level is computed from LevelThresholds/MaxLevel.
	LevelExpr string `yaml:"level_expr,omitempty"`

	// Extended display fields
	ChangeSourceField string `yaml:"change_source_field,omitempty"` // metadata key for % change
	Format            string `yaml:"format,omitempty"`              // "scale", "number", "currency", "percent"
	Precision         int    `yaml:"precision,omitempty"`           // decimal places
	Prefix            string `yaml:"prefix,omitempty"`              // "$", etc.
}

// WebSocketSpec configures WebSocket transport connections.
type WebSocketSpec struct {
	Subprotocols      []string `yaml:"subprotocols,omitempty"`
	SubscribeMessages []string `yaml:"subscribe_messages,omitempty"`
	PingInterval      Duration `yaml:"ping_interval,omitempty"`
	MessageFormat     string   `yaml:"message_format,omitempty" validate:"omitempty,oneof=text binary"`
	Compression       bool     `yaml:"compression,omitempty"`
	Origin            string   `yaml:"origin,omitempty"`
	// Decode specifies application-level decoding applied to each message before parsing.
	// Supported values: "lzw" (LZW decompression, as used by Blitzortung).
	Decode string `yaml:"decode,omitempty" validate:"omitempty,oneof=lzw"`
}

// SSESpec configures Server-Sent Events transport connections.
type SSESpec struct {
	LastEventID bool     `yaml:"last_event_id,omitempty"`
	EventFilter []string `yaml:"event_filter,omitempty"`
}

// MQTTSpec configures MQTT transport connections.
type MQTTSpec struct {
	Broker       string         `yaml:"broker" validate:"required"`
	Topics       []MQTTTopicSub `yaml:"topics" validate:"required,min=1"`
	ClientID     string         `yaml:"client_id,omitempty"`
	QoS          int            `yaml:"qos,omitempty" validate:"omitempty,min=0,max=2"`
	CleanSession *bool          `yaml:"clean_session,omitempty"`
	Version      string         `yaml:"version,omitempty" validate:"omitempty,oneof=3.1.1 5"`
	KeepAlive    int            `yaml:"keep_alive,omitempty" validate:"omitempty,min=10,max=3600"`
	Username     string         `yaml:"username,omitempty"`
	Password     string         `yaml:"password,omitempty"`
	TLS          *TLSSpec       `yaml:"tls,omitempty"`
}

// MQTTTopicSub defines a single MQTT topic subscription.
type MQTTTopicSub struct {
	Topic string `yaml:"topic" validate:"required"`
	QoS   *int   `yaml:"qos,omitempty" validate:"omitempty,min=0,max=2"`
}

// WebhookSpec configures an inbound HTTP listener for webhook-style sources.
type WebhookSpec struct {
	ListenAddr         string          `yaml:"listen_addr" validate:"required"`
	Path               string          `yaml:"path" validate:"required"`
	Secret             string          `yaml:"secret,omitempty"`
	SignatureHeader    string          `yaml:"signature_header,omitempty"`
	SignatureAlgorithm string          `yaml:"signature_algorithm,omitempty" validate:"omitempty,oneof=sha256 sha1"`
	MaxBodyBytes       int64           `yaml:"max_body_bytes,omitempty" validate:"omitempty,min=1024,max=104857600"`
	AllowedIPs         []string        `yaml:"allowed_ips,omitempty"`
	TLS                *WebhookTLSSpec `yaml:"tls,omitempty"`
}

// WebhookTLSSpec configures TLS for the webhook listener.
type WebhookTLSSpec struct {
	CertFile string `yaml:"cert_file" validate:"required"`
	KeyFile  string `yaml:"key_file" validate:"required"`
}

// GRPCStreamSpec configures a gRPC server-streaming client transport.
type GRPCStreamSpec struct {
	Address     string            `yaml:"address" validate:"required"`
	Service     string            `yaml:"service" validate:"required"`
	Method      string            `yaml:"method" validate:"required"`
	RequestJSON string            `yaml:"request_json,omitempty"`
	UseTLS      bool              `yaml:"use_tls,omitempty"`
	TLS         *TLSSpec          `yaml:"tls,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// KafkaSpec configures a Kafka consumer transport.
type KafkaSpec struct {
	Brokers        []string       `yaml:"brokers" validate:"required,min=1"`
	Topic          string         `yaml:"topic" validate:"required"`
	GroupID        string         `yaml:"group_id" validate:"required"`
	StartOffset    string         `yaml:"start_offset,omitempty" validate:"omitempty,oneof=earliest latest"`
	SASL           *KafkaSASLSpec `yaml:"sasl,omitempty"`
	TLS            *TLSSpec       `yaml:"tls,omitempty"`
	MaxBytes       int            `yaml:"max_bytes,omitempty"`
	CommitInterval Duration       `yaml:"commit_interval,omitempty"`
}

// KafkaSASLSpec configures SASL authentication for Kafka.
type KafkaSASLSpec struct {
	Mechanism string `yaml:"mechanism" validate:"required,oneof=PLAIN SCRAM-SHA-256 SCRAM-SHA-512"`
	Username  string `yaml:"username" validate:"required"`
	Password  string `yaml:"password" validate:"required"`
}

// AMQPSpec configures an AMQP (RabbitMQ) consumer transport.
type AMQPSpec struct {
	URL           string   `yaml:"url" validate:"required"`
	Queue         string   `yaml:"queue,omitempty"`
	Exchange      string   `yaml:"exchange,omitempty"`
	RoutingKey    string   `yaml:"routing_key,omitempty"`
	ExchangeType  string   `yaml:"exchange_type,omitempty" validate:"omitempty,oneof=direct fanout topic headers"`
	PrefetchCount int      `yaml:"prefetch_count,omitempty" validate:"omitempty,min=1,max=10000"`
	AutoAck       bool     `yaml:"auto_ack,omitempty"`
	TLS           *TLSSpec `yaml:"tls,omitempty"`
}

// TCPUDPSpec configures a raw TCP or UDP socket listener/connector.
type TCPUDPSpec struct {
	Protocol        string `yaml:"protocol" validate:"required,oneof=tcp udp"`
	Mode            string `yaml:"mode" validate:"required,oneof=connect listen"`
	Address         string `yaml:"address" validate:"required"`
	Delimiter       string `yaml:"delimiter,omitempty"`
	MaxMessageBytes int    `yaml:"max_message_bytes,omitempty" validate:"omitempty,min=64,max=10485760"`
	BufferSize      int    `yaml:"buffer_size,omitempty"`
}

// FTPSFTPSpec configures periodic file retrieval from FTP or SFTP servers.
type FTPSFTPSpec struct {
	Protocol         string `yaml:"protocol" validate:"required,oneof=ftp sftp"`
	Host             string `yaml:"host" validate:"required"`
	Path             string `yaml:"path" validate:"required"`
	FilePattern      string `yaml:"file_pattern,omitempty"`
	Username         string `yaml:"username,omitempty"`
	Password         string `yaml:"password,omitempty"`
	PrivateKey       string `yaml:"private_key,omitempty"`
	DeleteAfterFetch bool   `yaml:"delete_after_fetch,omitempty"`
	TrackSeen        *bool  `yaml:"track_seen,omitempty"`
}

// S3PollSpec configures periodic polling of an S3-compatible bucket for new objects.
type S3PollSpec struct {
	Endpoint          string   `yaml:"endpoint" validate:"required"`
	Bucket            string   `yaml:"bucket" validate:"required"`
	Prefix            string   `yaml:"prefix,omitempty"`
	Region            string   `yaml:"region,omitempty"`
	AccessKey         string   `yaml:"access_key,omitempty"`
	SecretKey         string   `yaml:"secret_key,omitempty"`
	DeleteAfterFetch  bool     `yaml:"delete_after_fetch,omitempty"`
	SinceLastModified Duration `yaml:"since_last_modified,omitempty"`
	FilePattern       string   `yaml:"file_pattern,omitempty"`
	PathStyle         bool     `yaml:"path_style,omitempty"`
}

// NATSSpec configures a NATS/JetStream consumer transport.
type NATSSpec struct {
	URL       string         `yaml:"url" validate:"required"`
	Subject   string         `yaml:"subject" validate:"required"`
	Queue     string         `yaml:"queue,omitempty"`
	JetStream *JetStreamSpec `yaml:"jetstream,omitempty"`
	CredsFile string         `yaml:"creds_file,omitempty"`
	Token     string         `yaml:"token,omitempty"`
	TLS       *TLSSpec       `yaml:"tls,omitempty"`
}

// JetStreamSpec configures JetStream consumer parameters.
type JetStreamSpec struct {
	Stream        string   `yaml:"stream" validate:"required"`
	Consumer      string   `yaml:"consumer" validate:"required"`
	DeliverPolicy string   `yaml:"deliver_policy,omitempty" validate:"omitempty,oneof=all last new by_start_time"`
	AckWait       Duration `yaml:"ack_wait,omitempty"`
	MaxDeliver    int      `yaml:"max_deliver,omitempty" validate:"omitempty,min=1,max=100"`
}

// TLSSpec configures TLS for transports that support it.
type TLSSpec struct {
	CACert             string `yaml:"ca_cert,omitempty"`
	ClientCert         string `yaml:"client_cert,omitempty"`
	ClientKey          string `yaml:"client_key,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"`
}

// BatchingSpec controls how streaming transports accumulate messages before processing.
type BatchingSpec struct {
	Mode    string   `yaml:"mode" validate:"required,oneof=per_message window"`
	Window  Duration `yaml:"window,omitempty"`
	MaxSize int      `yaml:"max_size,omitempty" validate:"omitempty,min=1,max=100000"`
}

// ReconnectSpec defines reconnection behavior for long-lived transports.
type ReconnectSpec struct {
	MaxAttempts  int      `yaml:"max_attempts" validate:"min=0"`
	Backoff      string   `yaml:"backoff" validate:"oneof=exponential fixed"`
	InitialDelay Duration `yaml:"initial_delay"`
	MaxDelay     Duration `yaml:"max_delay"`
	ResetAfter   Duration `yaml:"reset_after,omitempty"`
}

// LookupTableSpec defines a static lookup table for CEL enrichment.
// Tables are loaded and indexed at config time; CEL expressions reference them
// via lookup("table_name", key, "field").
type LookupTableSpec struct {
	Name     string                   `yaml:"name" validate:"required,source_name"`
	KeyField string                   `yaml:"key_field" validate:"required"`
	Entries  []map[string]interface{} `yaml:"entries,omitempty"`
	File     string                   `yaml:"file,omitempty"`
	Format   string                   `yaml:"format,omitempty" validate:"omitempty,oneof=json csv"`
}

// NewValidator creates a validator instance with custom validation rules
// for source_name and records_path.
func NewValidator() (*validator.Validate, error) {
	v := validator.New()

	err := v.RegisterValidation("source_name", func(fl validator.FieldLevel) bool {
		return sourceNameRE.MatchString(fl.Field().String())
	})
	if err != nil {
		return nil, fmt.Errorf("register source_name validator: %w", err)
	}

	err = v.RegisterValidation("records_path", func(fl validator.FieldLevel) bool {
		return recordsPathRE.MatchString(fl.Field().String())
	})
	if err != nil {
		return nil, fmt.Errorf("register records_path validator: %w", err)
	}

	return v, nil
}
