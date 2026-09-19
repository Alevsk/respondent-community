package declarative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// baseTransportYAML returns a minimal valid YAML for a given transport type.
// Callers can append transport-specific config blocks.
func baseTransportYAML(transportType string, schemaVersion int) string {
	return `schema_version: ` + itoa(schemaVersion) + `
name: test_transport
source_type: test_transport
layer_type: test_layer
display_name: "Test Transport"
transport:
  type: ` + transportType + `
  timeout: "10s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
}

func itoa(n int) string {
	if n == 1 {
		return "1"
	}
	return "2"
}

func TestSchemaValidation_AllTransportTypes(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	transportTypes := []string{
		"http_poll", "websocket", "sse", "mqtt", "webhook",
		"grpc_stream", "kafka", "amqp", "tcp_udp", "ftp_sftp", "s3_poll", "nats",
	}

	for _, tt := range transportTypes {
		t.Run(tt, func(t *testing.T) {
			def := SourceDefinition{
				SchemaVersion: 2,
				Name:          "test_source",
				SourceType:    "test_source",
				LayerType:     "test_layer",
				DisplayName:   "Test",
				Transport: TransportSpec{
					Type:    tt,
					Timeout: Duration{Duration: 10e9},
				},
				Parser:      ParserSpec{Format: "json"},
				Entity:      EntityMapping{ExternalID: "record.id", Name: "record.id"},
				Observation: ObsMapping{Latitude: "0.0", Longitude: "0.0", Timestamp: "now()"},
				Recording:   RecordingSpec{Mode: "append"},
				Cache:       CacheSpec{TTL: Duration{Duration: 60e9}},
				Display: DisplaySpec{
					Icon:  IconSpec{Shape: "dot", Scale: 1.0},
					Trail: TrailSpec{Color: "#ffffff"},
					Style: StyleSpec{Color: "#ffffff", PointSize: 6},
				},
			}

			err := v.Struct(def)
			if err != nil {
				t.Errorf("transport type %q rejected by validator: %v", tt, err)
			}
		})
	}
}

func TestSchemaValidation_InvalidTransportType(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	def := SourceDefinition{
		SchemaVersion: 2,
		Name:          "test_source",
		SourceType:    "test_source",
		LayerType:     "test_layer",
		DisplayName:   "Test",
		Transport: TransportSpec{
			Type:    "carrier_pigeon",
			Timeout: Duration{Duration: 10e9},
		},
		Parser:      ParserSpec{Format: "json"},
		Entity:      EntityMapping{ExternalID: "record.id", Name: "record.id"},
		Observation: ObsMapping{Latitude: "0.0", Longitude: "0.0", Timestamp: "now()"},
		Recording:   RecordingSpec{Mode: "append"},
		Cache:       CacheSpec{TTL: Duration{Duration: 60e9}},
		Display: DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
	}

	err = v.Struct(def)
	if err == nil {
		t.Error("expected validation error for invalid transport type, got nil")
	}
}

func TestLoader_StreamingTransportRequiresSchemaV2(t *testing.T) {
	streamingTypes := []string{
		"websocket", "sse", "mqtt", "webhook",
		"grpc_stream", "kafka", "amqp", "tcp_udp", "ftp_sftp", "s3_poll", "nats",
	}

	loader := newTestLoader(t, true)

	for _, tt := range streamingTypes {
		t.Run(tt, func(t *testing.T) {
			yamlContent := baseTransportYAML(tt, 1)

			// Add URL for URL-required types
			switch tt {
			case "websocket":
				yamlContent = strings.Replace(yamlContent, "timeout: \"10s\"",
					"url: \"wss://example.com/ws\"\n  timeout: \"10s\"", 1)
			case "sse":
				yamlContent = strings.Replace(yamlContent, "timeout: \"10s\"",
					"url: \"https://example.com/sse\"\n  timeout: \"10s\"", 1)
			}

			// Add interval for pull-based types
			switch tt {
			case "ftp_sftp", "s3_poll":
				yamlContent = strings.Replace(yamlContent, "timeout: \"10s\"",
					"timeout: \"10s\"\n  interval: \"60s\"", 1)
			}

			// Add required sub-specs
			yamlContent = addSubSpec(yamlContent, tt)

			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, err := loader.LoadFile(path)
			if err == nil {
				t.Fatalf("expected error for %q with schema_version 1, got nil", tt)
			}
			if !strings.Contains(err.Error(), "requires schema_version >= 2") {
				t.Errorf("expected schema_version error, got: %v", err)
			}
		})
	}
}

func TestLoader_MissingSubSpec(t *testing.T) {
	tests := []struct {
		transportType string
		configField   string
	}{
		{"mqtt", "mqtt"},
		{"webhook", "webhook"},
		{"grpc_stream", "grpc_stream"},
		{"kafka", "kafka"},
		{"amqp", "amqp"},
		{"tcp_udp", "tcp_udp"},
		{"ftp_sftp", "ftp_sftp"},
		{"s3_poll", "s3_poll"},
		{"nats", "nats"},
	}

	loader := newTestLoader(t, true)

	for _, tc := range tests {
		t.Run(tc.transportType, func(t *testing.T) {
			yamlContent := baseTransportYAML(tc.transportType, 2)

			// Add interval for pull-based types
			switch tc.transportType {
			case "ftp_sftp", "s3_poll":
				yamlContent = strings.Replace(yamlContent, "timeout: \"10s\"",
					"timeout: \"10s\"\n  interval: \"60s\"", 1)
			}

			// Intentionally do NOT add the sub-spec

			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, err := loader.LoadFile(path)
			if err == nil {
				t.Fatalf("expected error for missing %s sub-spec, got nil", tc.configField)
			}
			if !strings.Contains(err.Error(), tc.configField+" config is required") {
				t.Errorf("expected sub-spec error mentioning %q, got: %v", tc.configField, err)
			}
		})
	}
}

func TestLoader_URLRequiredForHTTPWebSocketSSE(t *testing.T) {
	tests := []struct {
		transportType string
	}{
		{"http_poll"},
		{"websocket"},
		{"sse"},
	}

	loader := newTestLoader(t, true)

	for _, tc := range tests {
		t.Run(tc.transportType, func(t *testing.T) {
			schemaVersion := 1
			if tc.transportType != "http_poll" {
				schemaVersion = 2
			}

			yamlContent := baseTransportYAML(tc.transportType, schemaVersion)
			// No URL is set in the base YAML

			// Add interval for http_poll
			if tc.transportType == "http_poll" {
				yamlContent = strings.Replace(yamlContent, "timeout: \"10s\"",
					"timeout: \"10s\"\n  interval: \"60s\"", 1)
			}

			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, err := loader.LoadFile(path)
			if err == nil {
				t.Fatalf("expected error for missing URL with type %q, got nil", tc.transportType)
			}
			if !strings.Contains(err.Error(), "transport.url is required") {
				t.Errorf("expected URL required error, got: %v", err)
			}
		})
	}
}

func TestLoader_URLNotRequiredForNonHTTPTransports(t *testing.T) {
	// mqtt and nats don't need transport.url
	tests := []struct {
		transportType string
		subSpec       string
	}{
		{"mqtt", `
  mqtt:
    broker: "tcp://broker:1883"
    topics:
      - topic: "test/topic"
`},
		{"nats", `
  nats:
    url: "nats://localhost:4222"
    subject: "test.subject"
`},
	}

	loader := newTestLoader(t, true)

	for _, tc := range tests {
		t.Run(tc.transportType, func(t *testing.T) {
			yamlContent := `schema_version: 2
name: test_no_url
source_type: test_no_url
layer_type: test_layer
display_name: "Test No URL"
transport:
  type: ` + tc.transportType + `
  timeout: "10s"` + tc.subSpec + `parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`

			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			_, err := loader.LoadFile(path)
			if err != nil {
				// The error should NOT be about missing URL
				if strings.Contains(err.Error(), "transport.url is required") {
					t.Errorf("URL should not be required for %q: %v", tc.transportType, err)
				}
				// Other errors (like unsupported transport) are expected since no
				// constructor is registered. We only verify URL is not required.
			}
		})
	}
}

func TestLoader_IntervalRequiredForPullTransports(t *testing.T) {
	loader := newTestLoader(t, true)

	// http_poll requires interval
	yamlContent := `schema_version: 1
name: test_no_interval
source_type: test_no_interval
layer_type: test_layer
display_name: "Test No Interval"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for missing interval with http_poll, got nil")
	}
	if !strings.Contains(err.Error(), "transport.interval is required") {
		t.Errorf("expected interval required error, got: %v", err)
	}
}

func TestLoader_TimeoutRequired(t *testing.T) {
	loader := newTestLoader(t, true)

	yamlContent := `schema_version: 2
name: test_no_timeout
source_type: test_no_timeout
layer_type: test_layer
display_name: "Test No Timeout"
transport:
  type: nats
  nats:
    url: "nats://localhost:4222"
    subject: "test.subject"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for missing timeout, got nil")
	}
	if !strings.Contains(err.Error(), "transport.timeout is required") {
		t.Errorf("expected timeout required error, got: %v", err)
	}
}

func TestLoader_WebSocketURLSchemes(t *testing.T) {
	loaderProd := newTestLoader(t, false)

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "wss allowed", url: "wss://example.com/ws", wantErr: false},
		{name: "ws allowed", url: "ws://example.com/ws", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := loaderProd.validateURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for URL %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for URL %q: %v", tc.url, err)
			}
		})
	}
}

func TestSchema_TransportSpecStructs_YAMLParsing(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, ts TransportSpec)
	}{
		{
			name: "websocket spec",
			yaml: `
type: websocket
url: "wss://example.com/ws"
timeout: "10s"
websocket:
  subprotocols: ["graphql-ws"]
  subscribe_messages: ['{"type":"subscribe"}']
  ping_interval: "30s"
  message_format: binary
  compression: true
  origin: "https://example.com"
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.WebSocket == nil {
					t.Fatal("WebSocket spec is nil")
				}
				if len(ts.WebSocket.Subprotocols) != 1 || ts.WebSocket.Subprotocols[0] != "graphql-ws" {
					t.Errorf("Subprotocols = %v", ts.WebSocket.Subprotocols)
				}
				if ts.WebSocket.MessageFormat != "binary" {
					t.Errorf("MessageFormat = %q", ts.WebSocket.MessageFormat)
				}
				if !ts.WebSocket.Compression {
					t.Error("Compression should be true")
				}
			},
		},
		{
			name: "mqtt spec",
			yaml: `
type: mqtt
timeout: "10s"
mqtt:
  broker: "tcp://broker:1883"
  topics:
    - topic: "sensor/+/data"
      qos: 1
    - topic: "alerts/#"
  client_id: "feeder-001"
  qos: 2
  version: "5"
  keep_alive: 60
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.MQTT == nil {
					t.Fatal("MQTT spec is nil")
				}
				if ts.MQTT.Broker != "tcp://broker:1883" {
					t.Errorf("Broker = %q", ts.MQTT.Broker)
				}
				if len(ts.MQTT.Topics) != 2 {
					t.Errorf("Topics len = %d, want 2", len(ts.MQTT.Topics))
				}
				if ts.MQTT.QoS != 2 {
					t.Errorf("QoS = %d, want 2", ts.MQTT.QoS)
				}
			},
		},
		{
			name: "kafka spec",
			yaml: `
type: kafka
timeout: "10s"
kafka:
  brokers: ["kafka1:9092", "kafka2:9092"]
  topic: "events"
  group_id: "feeder-group"
  start_offset: earliest
  sasl:
    mechanism: SCRAM-SHA-256
    username: user
    password: pass
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.Kafka == nil {
					t.Fatal("Kafka spec is nil")
				}
				if len(ts.Kafka.Brokers) != 2 {
					t.Errorf("Brokers len = %d, want 2", len(ts.Kafka.Brokers))
				}
				if ts.Kafka.GroupID != "feeder-group" {
					t.Errorf("GroupID = %q", ts.Kafka.GroupID)
				}
				if ts.Kafka.SASL == nil || ts.Kafka.SASL.Mechanism != "SCRAM-SHA-256" {
					t.Error("SASL not parsed correctly")
				}
			},
		},
		{
			name: "batching spec",
			yaml: `
type: websocket
url: "wss://example.com/ws"
timeout: "10s"
batching:
  mode: window
  window: "5s"
  max_size: 100
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.Batching == nil {
					t.Fatal("Batching spec is nil")
				}
				if ts.Batching.Mode != "window" {
					t.Errorf("Batching.Mode = %q", ts.Batching.Mode)
				}
				if ts.Batching.MaxSize != 100 {
					t.Errorf("Batching.MaxSize = %d", ts.Batching.MaxSize)
				}
			},
		},
		{
			name: "reconnect spec",
			yaml: `
type: websocket
url: "wss://example.com/ws"
timeout: "10s"
reconnect:
  max_attempts: 10
  backoff: exponential
  initial_delay: "1s"
  max_delay: "30s"
  reset_after: "5m"
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.Reconnect == nil {
					t.Fatal("Reconnect spec is nil")
				}
				if ts.Reconnect.MaxAttempts != 10 {
					t.Errorf("MaxAttempts = %d", ts.Reconnect.MaxAttempts)
				}
				if ts.Reconnect.Backoff != "exponential" {
					t.Errorf("Backoff = %q", ts.Reconnect.Backoff)
				}
			},
		},
		{
			name: "nats with jetstream",
			yaml: `
type: nats
timeout: "10s"
nats:
  url: "nats://localhost:4222"
  subject: "events.>"
  queue: "feeder-workers"
  jetstream:
    stream: EVENTS
    consumer: feeder
    deliver_policy: new
    ack_wait: "30s"
    max_deliver: 5
`,
			check: func(t *testing.T, ts TransportSpec) {
				if ts.NATS == nil {
					t.Fatal("NATS spec is nil")
				}
				if ts.NATS.JetStream == nil {
					t.Fatal("JetStream spec is nil")
				}
				if ts.NATS.JetStream.Stream != "EVENTS" {
					t.Errorf("Stream = %q", ts.NATS.JetStream.Stream)
				}
				if ts.NATS.JetStream.MaxDeliver != 5 {
					t.Errorf("MaxDeliver = %d", ts.NATS.JetStream.MaxDeliver)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ts TransportSpec
			if err := yaml.Unmarshal([]byte(tc.yaml), &ts); err != nil {
				t.Fatalf("YAML unmarshal error: %v", err)
			}
			tc.check(t, ts)
		})
	}
}

// addSubSpec adds the minimum required sub-spec for a transport type.
func addSubSpec(yamlContent, transportType string) string {
	subSpecs := map[string]string{
		"mqtt": `
  mqtt:
    broker: "tcp://broker:1883"
    topics:
      - topic: "test/topic"
`,
		"webhook": `
  webhook:
    listen_addr: ":8080"
    path: "/hooks/test"
`,
		"grpc_stream": `
  grpc_stream:
    address: "localhost:50051"
    service: "test.Service"
    method: "StreamData"
`,
		"kafka": `
  kafka:
    brokers: ["localhost:9092"]
    topic: "test"
    group_id: "test-group"
`,
		"amqp": `
  amqp:
    url: "amqp://localhost:5672"
    queue: "test"
`,
		"tcp_udp": `
  tcp_udp:
    protocol: tcp
    mode: connect
    address: "localhost:9000"
`,
		"ftp_sftp": `
  ftp_sftp:
    protocol: sftp
    host: "localhost:22"
    path: "/data"
`,
		"s3_poll": `
  s3_poll:
    endpoint: "http://localhost:9000"
    bucket: "test-bucket"
`,
		"nats": `
  nats:
    url: "nats://localhost:4222"
    subject: "test.subject"
`,
	}

	if spec, ok := subSpecs[transportType]; ok {
		// Insert before parser: line
		return strings.Replace(yamlContent, "parser:", spec+"parser:", 1)
	}
	return yamlContent
}
