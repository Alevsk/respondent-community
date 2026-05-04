---
title: "Transports"
description: "Configure how Respondent fetches data — HTTP polling, WebSockets, MQTT, S3, and more"
weight: 1
---

Transports define how the feeder service connects to a data source and retrieves raw data. Every source definition requires exactly one transport. The `type` field selects the transport, and transport-specific fields configure the connection details.

```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data"
  interval: "60s"
```

## HTTP Poll

HTTP polling is the most common transport. Respondent makes periodic HTTP requests and processes the response body through the parser.

### Core fields

{{< field name="type" type="string" required="true" >}}
Transport type. Set to `http_poll` for periodic HTTP requests.
{{< /field >}}

{{< field name="url" type="string" required="true" >}}
The API endpoint URL. Supports environment variable substitution via `${VAR}` syntax -- resolved from `RESPONDENT_VAR` at load time. For spatial sources, use `{lat}` and `{lon}` placeholders.
{{< /field >}}

{{< field name="on_demand_url" type="string" required="false" >}}
Alternative URL used for single-point fetches (schema_version 2 only). Supports `{lat}` and `{lon}` placeholders for user-triggered location queries.
{{< /field >}}

{{< field name="method" type="string" required="false" default="GET" >}}
HTTP method. Allowed values: `GET`, `POST`.
{{< /field >}}

{{< field name="headers" type="map[string]string" required="false" >}}
Static headers sent with every request.
{{< /field >}}

{{< field name="timeout" type="duration" required="false" >}}
HTTP request timeout. Examples: `"10s"`, `"30s"`, `"1m"`.
{{< /field >}}

{{< field name="interval" type="duration" required="false" >}}
Polling interval -- how often to re-fetch data. Examples: `"60s"`, `"5m"`, `"1200s"`.
{{< /field >}}

{{< field name="max_response_bytes" type="integer" required="false" default="10485760" >}}
Maximum response body size in bytes. Range: 1 to 104857600 (100 MB). Default is 10 MB.
{{< /field >}}

### Example

```yaml
transport:
  type: http_poll
  url: "https://www.seismicportal.eu/fdsnws/event/1/query?format=json&limit=100"
  method: GET
  headers:
    Accept: "application/json"
  timeout: "10s"
  interval: "120s"
  max_response_bytes: 52428800
```

### Authentication

Respondent supports four authentication patterns. Secrets are never stored in YAML -- only environment variable names that are resolved at runtime from `RESPONDENT_<env_var>`.

#### Bearer token

```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data"
  auth:
    type: bearer
    env_var: "MY_API_TOKEN"    # Resolved from RESPONDENT_MY_API_TOKEN
```

#### API key in a custom header

```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data"
  auth:
    type: api_key
    header: "X-API-Key"
    env_var: "MY_API_KEY"      # Resolved from RESPONDENT_MY_API_KEY
```

#### Credential embedded in URL

No `auth` block is needed. Use `${VAR}` substitution directly in the URL:

```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data/${MY_KEY}/endpoint"
```

#### OAuth2 token exchange

```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data"
  auth:
    type: oauth2
    oauth2:
      token_url: "https://auth.example.com/oauth/token"
      grant_type: client_credentials   # client_credentials | password
      credentials:
        client_id:
          env_var: "MY_CLIENT_ID"
        client_secret:
          env_var: "MY_CLIENT_SECRET"
      # Optional overrides (defaults shown):
      # token_header: "Authorization"
      # token_prefix: "Bearer "
      # refresh_before_expiry: "300s"
      # response_mapping:
      #   access_token: "access_token"
      #   refresh_token: "refresh_token"
      #   expires_in: "expires_in"
      # extra_params:
      #   audience: "https://api.example.com"
```

For the `password` grant type, add `username` and `password` to `credentials`:

```yaml
credentials:
  client_id:
    env_var: "MY_CLIENT_ID"
  client_secret:
    env_var: "MY_CLIENT_SECRET"
  username: "literal-username"
  password:
    env_var: "MY_PASSWORD"
```

Each credential field accepts either a literal string value or an `env_var` reference.

### Retry configuration

Configure automatic retries for failed HTTP requests.

{{< field name="retry.max_attempts" type="integer" required="false" default="0" >}}
Number of retries after the initial request. Range: 0 to 10. Total attempts = max_attempts + 1.
{{< /field >}}

{{< field name="retry.backoff" type="string" required="false" >}}
Backoff strategy between retries. Allowed values: `exponential`, `fixed`.
{{< /field >}}

{{< field name="retry.initial_delay" type="duration" required="false" >}}
Delay before the first retry. Example: `"2s"`.
{{< /field >}}

{{< field name="retry.max_delay" type="duration" required="false" >}}
Maximum delay between retries (caps exponential growth). Example: `"30s"`.
{{< /field >}}

```yaml
retry:
  max_attempts: 3
  backoff: "exponential"
  initial_delay: "2s"
  max_delay: "30s"
```

### Pagination

For APIs that return paginated results, configure automatic page traversal.

{{< code-tabs >}}
{{< tab title="Page Number" >}}
```yaml
pagination:
  type: page_number
  page_param: "page"
  size_param: "per_page"
  size: 100
  max_pages: 10
  stop_when: "size(records) < 100"
```
{{< /tab >}}
{{< tab title="Offset" >}}
```yaml
pagination:
  type: offset
  page_param: "offset"
  size_param: "limit"
  size: 100
  max_pages: 10
```
{{< /tab >}}
{{< tab title="Cursor" >}}
```yaml
pagination:
  type: cursor
  page_param: "cursor"
  size_param: "limit"
  size: 100
  max_pages: 10
  cursor_path: "meta.next_cursor"
```
{{< /tab >}}
{{< /code-tabs >}}

{{< field name="pagination.type" type="string" required="true" >}}
Pagination strategy. Allowed values: `page_number`, `offset`, `cursor`.
{{< /field >}}

{{< field name="pagination.page_param" type="string" required="true" >}}
Query parameter name for the page indicator (page number, offset value, or cursor token).
{{< /field >}}

{{< field name="pagination.size_param" type="string" required="false" >}}
Query parameter name for the page size or limit.
{{< /field >}}

{{< field name="pagination.size" type="integer" required="true" >}}
Number of items per page. Range: 1 to 10000.
{{< /field >}}

{{< field name="pagination.max_pages" type="integer" required="true" >}}
Maximum number of pages to fetch per poll. Range: 1 to 1000.
{{< /field >}}

{{< field name="pagination.stop_when" type="string" required="false" >}}
CEL expression evaluated per page. When it returns true, pagination stops early. The variable `records` is bound to the current page's record array.
{{< /field >}}

{{< field name="pagination.cursor_path" type="string" required="false" >}}
Dot-path to extract the next cursor value from the JSON response. Only used with `type: cursor`.
{{< /field >}}

### Spatial crawling

For APIs that require geographic coordinates (schema_version 2 only), configure spatial crawling to cover the globe systematically. The URL must contain `{lat}` and `{lon}` placeholders.

{{< code-tabs >}}
{{< tab title="Hex Grid" >}}
```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data/lat/{lat}/lon/{lon}/dist/250"
  spatial:
    type: hex_grid
    radius_nm: 250
    target_refresh: "30s"
    batch_size: 10
    viewport_priority: true
    priority_cap_pct: 30
    lat_min: -60
    lat_max: 75
```
{{< /tab >}}
{{< tab title="Static Regions" >}}
```yaml
transport:
  type: http_poll
  url: "https://api.example.com/data/lat/{lat}/lon/{lon}/dist/250"
  spatial:
    type: static_regions
    regions:
      - lat: 40.7128
        lon: -74.0060
        label: "New York"
      - lat: 51.5074
        lon: -0.1278
        label: "London"
```
{{< /tab >}}
{{< /code-tabs >}}

{{< callout type="tip" title="Latitude band filtering" >}}
Set `lat_min` and `lat_max` to skip regions without useful data. For flights, `lat_min: -60` and `lat_max: 75` prunes Antarctica and the Arctic, reducing initial scan time from ~10 minutes to under 1 minute by eliminating ~40% of grid regions.
{{< /callout >}}

---

## WebSocket

Persistent WebSocket connection for real-time streaming data.

```yaml
transport:
  type: websocket
  url: "wss://stream.example.com/data"
  headers:
    Origin: "https://example.com"
  websocket:
    subprotocols: []
    subscribe_messages:
      - '{"type":"subscribe","channel":"data"}'
    ping_interval: "30s"
    message_format: text        # text | binary
    compression: false
    origin: "https://example.com"
    decode: lzw                 # Application-level decoding (e.g., Blitzortung LZW)
  reconnect:
    max_attempts: 0             # 0 = infinite
    backoff: exponential
    initial_delay: "2s"
    max_delay: "60s"
  batching:
    mode: window
    window: "3s"
    max_size: 1000
```

{{< field name="websocket.subscribe_messages" type="string[]" required="false" >}}
Messages sent immediately after the connection opens. Use for channel subscriptions.
{{< /field >}}

{{< field name="websocket.ping_interval" type="duration" required="false" >}}
Heartbeat ping interval to keep the connection alive.
{{< /field >}}

{{< field name="websocket.message_format" type="string" required="false" >}}
Message encoding. Allowed values: `text`, `binary`.
{{< /field >}}

{{< field name="websocket.decode" type="string" required="false" >}}
Application-level decoding applied to each message before parsing. Allowed values: `lzw`.
{{< /field >}}

---

## SSE (Server-Sent Events)

Server-Sent Events stream for unidirectional real-time data.

```yaml
transport:
  type: sse
  url: "https://stream.example.com/events"
  headers:
    Accept: "text/event-stream"
  sse:
    last_event_id: false
    event_filter:
      - "message"
      - "update"
```

{{< field name="sse.last_event_id" type="boolean" required="false" default="false" >}}
Resume from the last event ID on reconnect.
{{< /field >}}

{{< field name="sse.event_filter" type="string[]" required="false" >}}
Only process these SSE event types. An empty list processes all events.
{{< /field >}}

---

## MQTT

Subscribe to topics on an MQTT broker.

```yaml
transport:
  type: mqtt
  mqtt:
    broker: "tcp://broker.example.com:1883"
    topics:
      - topic: "sensors/+/data"
        qos: 1
    client_id: "respondent-source"
    qos: 1
    clean_session: true
    version: "3.1.1"            # 3.1.1 | 5
    keep_alive: 60              # seconds (10-3600)
    username: ""
    password: ""
    tls:
      ca_cert: "/path/to/ca.pem"
      insecure_skip_verify: false
```

{{< field name="mqtt.broker" type="string" required="true" >}}
MQTT broker address. Example: `"tcp://broker.example.com:1883"`.
{{< /field >}}

{{< field name="mqtt.topics" type="array" required="true" >}}
Topic subscriptions. Each entry has a `topic` (supports MQTT wildcards `+` and `#`) and an optional per-topic `qos` override.
{{< /field >}}

{{< field name="mqtt.qos" type="integer" required="false" default="0" >}}
Default Quality of Service level. Range: 0 to 2.
{{< /field >}}

{{< field name="mqtt.client_id" type="string" required="false" >}}
MQTT client identifier.
{{< /field >}}

---

## S3 Poll

Poll an S3-compatible bucket for new objects.

```yaml
transport:
  type: s3_poll
  interval: "300s"
  s3_poll:
    endpoint: "https://s3.amazonaws.com"
    bucket: "my-data-bucket"
    prefix: "exports/"
    region: "us-east-1"
    access_key: ""              # Or use IAM role
    secret_key: ""
    file_pattern: "*.json"
    since_last_modified: "24h"
    delete_after_fetch: false
    path_style: false           # true for MinIO and other S3-compatible stores
```

{{< field name="s3_poll.endpoint" type="string" required="true" >}}
S3 endpoint URL.
{{< /field >}}

{{< field name="s3_poll.bucket" type="string" required="true" >}}
Bucket name.
{{< /field >}}

{{< field name="s3_poll.prefix" type="string" required="false" >}}
Object key prefix to filter results.
{{< /field >}}

{{< field name="s3_poll.file_pattern" type="string" required="false" >}}
Glob pattern for object keys. Example: `"*.json"`.
{{< /field >}}

{{< field name="s3_poll.since_last_modified" type="duration" required="false" >}}
Only fetch objects modified within this time window.
{{< /field >}}

{{< field name="s3_poll.path_style" type="boolean" required="false" default="false" >}}
Use path-style URLs instead of virtual-hosted-style. Required for MinIO and some S3-compatible stores.
{{< /field >}}

---

## FTP / SFTP

Periodic file retrieval from FTP or SFTP servers.

```yaml
transport:
  type: ftp_sftp
  interval: "600s"
  ftp_sftp:
    protocol: sftp              # ftp | sftp
    host: "ftp.example.com:22"
    path: "/data/exports/"
    file_pattern: "*.json"
    username: "user"
    password: ""
    private_key: "/path/to/id_rsa"   # SFTP only
    delete_after_fetch: false
    track_seen: true
```

{{< field name="ftp_sftp.protocol" type="string" required="true" >}}
Transfer protocol. Allowed values: `ftp`, `sftp`.
{{< /field >}}

{{< field name="ftp_sftp.host" type="string" required="true" >}}
Server hostname and port.
{{< /field >}}

{{< field name="ftp_sftp.path" type="string" required="true" >}}
Remote directory path to scan for files.
{{< /field >}}

{{< field name="ftp_sftp.file_pattern" type="string" required="false" >}}
Glob pattern to filter files. Example: `"*.json"`.
{{< /field >}}

{{< field name="ftp_sftp.track_seen" type="boolean" required="false" >}}
Skip files that have already been fetched in previous polls.
{{< /field >}}

---

## Webhook

Inbound HTTP listener for push-style data sources.

```yaml
transport:
  type: webhook
  webhook:
    listen_addr: ":9090"
    path: "/webhooks/my-source"
    secret: ""
    signature_header: "X-Hub-Signature-256"
    signature_algorithm: sha256   # sha256 | sha1
    max_body_bytes: 10485760
    allowed_ips:
      - "192.168.1.0/24"
    tls:
      cert_file: "/path/to/cert.pem"
      key_file: "/path/to/key.pem"
```

{{< field name="webhook.listen_addr" type="string" required="true" >}}
Address to bind the HTTP listener. Example: `":9090"`.
{{< /field >}}

{{< field name="webhook.path" type="string" required="true" >}}
HTTP path for incoming webhook requests.
{{< /field >}}

{{< field name="webhook.secret" type="string" required="false" >}}
HMAC secret for request signature verification.
{{< /field >}}

{{< field name="webhook.max_body_bytes" type="integer" required="false" >}}
Maximum request body size in bytes. Range: 1024 to 104857600.
{{< /field >}}

---

## Kafka

Consume messages from a Kafka topic.

```yaml
transport:
  type: kafka
  kafka:
    brokers:
      - "kafka1.example.com:9092"
      - "kafka2.example.com:9092"
    topic: "events"
    group_id: "respondent-consumer"
    start_offset: earliest        # earliest | latest
    max_bytes: 10485760
    commit_interval: "5s"
    sasl:
      mechanism: SCRAM-SHA-256    # PLAIN | SCRAM-SHA-256 | SCRAM-SHA-512
      username: "user"
      password: "pass"
    tls:
      insecure_skip_verify: false
```

{{< field name="kafka.brokers" type="string[]" required="true" >}}
Kafka broker addresses. At least one required.
{{< /field >}}

{{< field name="kafka.topic" type="string" required="true" >}}
Kafka topic to consume.
{{< /field >}}

{{< field name="kafka.group_id" type="string" required="true" >}}
Consumer group ID for offset tracking.
{{< /field >}}

{{< field name="kafka.start_offset" type="string" required="false" >}}
Initial offset for new consumer groups. Allowed values: `earliest`, `latest`.
{{< /field >}}

---

## AMQP

Consume messages from an AMQP (RabbitMQ) exchange or queue.

```yaml
transport:
  type: amqp
  amqp:
    url: "amqp://user:pass@rabbitmq.example.com:5672/"
    queue: "events"
    exchange: "events-exchange"
    routing_key: "events.#"
    exchange_type: topic          # direct | fanout | topic | headers
    prefetch_count: 100
    auto_ack: false
```

{{< field name="amqp.url" type="string" required="true" >}}
AMQP connection URL.
{{< /field >}}

{{< field name="amqp.exchange_type" type="string" required="false" >}}
Exchange type. Allowed values: `direct`, `fanout`, `topic`, `headers`.
{{< /field >}}

{{< field name="amqp.prefetch_count" type="integer" required="false" >}}
QoS prefetch count. Range: 1 to 10000.
{{< /field >}}

---

## NATS

Subscribe to a NATS subject with optional JetStream durable consumer support.

```yaml
transport:
  type: nats
  nats:
    url: "nats://nats.example.com:4222"
    subject: "events.>"
    queue: "respondent-workers"
    creds_file: "/path/to/nats.creds"
    jetstream:
      stream: "EVENTS"
      consumer: "respondent"
      deliver_policy: new         # all | last | new | by_start_time
      ack_wait: "30s"
      max_deliver: 5
```

{{< field name="nats.url" type="string" required="true" >}}
NATS server URL.
{{< /field >}}

{{< field name="nats.subject" type="string" required="true" >}}
NATS subject to subscribe to. Supports wildcards (`*`, `>`).
{{< /field >}}

{{< field name="nats.queue" type="string" required="false" >}}
Queue group name for load-balanced consumption.
{{< /field >}}

---

## Pub/Sub

Subscribe to cloud Pub/Sub (GCP) or Valkey/Redis Pub/Sub channels.

```yaml
transport:
  type: pubsub
  pubsub:
    provider: gcp                 # gcp | valkey | redis
    # GCP:
    project_id: "my-gcp-project"
    subscription_id: "my-subscription"
    max_outstanding_messages: 1000
    # Valkey/Redis:
    # addr: "localhost:6379"
    # channels: ["events:*"]
    # patterns: ["events:*"]
    ack_mode: auto                # auto | manual
```

{{< field name="pubsub.provider" type="string" required="true" >}}
Pub/Sub provider. Allowed values: `gcp`, `valkey`, `redis`.
{{< /field >}}

---

## gRPC Stream

Connect to a gRPC server-streaming endpoint.

```yaml
transport:
  type: grpc_stream
  grpc_stream:
    address: "grpc.example.com:443"
    service: "my.package.MyService"
    method: "StreamData"
    request_json: '{"filter":"active"}'
    use_tls: true
    metadata:
      authorization: "Bearer ${MY_TOKEN}"
```

{{< field name="grpc_stream.address" type="string" required="true" >}}
gRPC server address.
{{< /field >}}

{{< field name="grpc_stream.service" type="string" required="true" >}}
Fully-qualified gRPC service name.
{{< /field >}}

{{< field name="grpc_stream.method" type="string" required="true" >}}
RPC method name.
{{< /field >}}

---

## TCP / UDP

Raw TCP or UDP socket connection, either outbound (connect) or inbound (listen).

```yaml
transport:
  type: tcp_udp
  tcp_udp:
    protocol: tcp                 # tcp | udp
    mode: connect                 # connect (outbound) | listen (inbound)
    address: "data.example.com:5000"
    delimiter: "\n"
    max_message_bytes: 65536
    buffer_size: 65536
```

{{< field name="tcp_udp.protocol" type="string" required="true" >}}
Socket protocol. Allowed values: `tcp`, `udp`.
{{< /field >}}

{{< field name="tcp_udp.mode" type="string" required="true" >}}
Connection mode. `connect` opens an outbound connection; `listen` binds a local listener.
{{< /field >}}

{{< field name="tcp_udp.address" type="string" required="true" >}}
Socket address (host:port).
{{< /field >}}

---

## Streaming behavior

Long-lived transports (WebSocket, SSE, MQTT, Kafka, AMQP, NATS, Pub/Sub, gRPC, TCP/UDP) support batching and reconnection configuration at the transport level.

### Batching

Controls how streaming messages are accumulated before being sent to the parser.

```yaml
batching:
  mode: window              # per_message | window
  window: "3s"              # Accumulation window (window mode only)
  max_size: 1000            # Max messages per batch (1-100000)
```

{{< field name="batching.mode" type="string" required="true" >}}
Batching strategy. `per_message` processes each message immediately. `window` accumulates messages for a duration before processing.
{{< /field >}}

### Reconnection

Defines automatic reconnection behavior when a long-lived connection drops.

```yaml
reconnect:
  max_attempts: 0           # 0 = infinite retries
  backoff: exponential      # exponential | fixed
  initial_delay: "2s"
  max_delay: "60s"
  reset_after: "10m"        # Reset backoff after stable connection
```

{{< field name="reconnect.max_attempts" type="integer" required="false" default="0" >}}
Maximum reconnection attempts. Set to 0 for infinite retries.
{{< /field >}}

{{< field name="reconnect.reset_after" type="duration" required="false" >}}
Reset the backoff counter after the connection has been stable for this duration.
{{< /field >}}
