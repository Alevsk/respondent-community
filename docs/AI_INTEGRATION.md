# AI/LLM Integration Reference

This document describes the AI integration as it exists in production. It covers what is implemented, how it works, and how to use it. For planned but not yet implemented features, see [spec/AI_INTEGRATION.md](spec/AI_INTEGRATION.md).

## 1. Overview

Respondent integrates LLM-powered intelligence across the **sources -> tasks -> targets** pipeline:

- **sources.d/** -- Per-entity AI enrichment at ingestion time (classify, score, extract)
- **analysis.d/** -- Scheduled AI analysis jobs (anomaly detection, correlation, summaries)
- **tasks.d/** -- Multi-step processing pipelines with AI as a first-class step type
- **targets.d/** -- Delivery destinations with optional AI-powered content formatting

All AI features are **opt-in** (`ai.enabled: true` in config), **independently toggleable** per source/analysis/task, and **gracefully degradable** -- the platform runs normally if no LLM provider is configured.

---

## 2. Architecture

```
┌──────────────┐      ┌──────────────┐      ┌──────────────┐
│  sources.d/  │─────>│  tasks.d/    │─────>│  targets.d/  │
│  Ingest +    │      │  Process +   │      │  Deliver to  │
│  enrich      │      │  AI pipeline │      │  Slack, etc. │
│  ai: {...}   │      │  ai: {...}   │      │  ai: {...}   │
└──────┬───────┘      └──────┬───────┘      └──────┬───────┘
       │                     │                     │
       │    ┌────────────────┴────────────────┐    │
       │    │         AI Layer (shared)        │    │
       │    │  LLM Providers (6 adapters)      │    │
       │    │  Schema Registry (JSON Schema)   │    │
       │    │  Prompt Renderer (Go templates)  │    │
       │    │  SQL Validator (pg_query_go)     │    │
       └────┤  WebSocket Notifier              ├────┘
            └─────────────────────────────────┘

            ┌──────────────┐
            │ analysis.d/  │  Scheduled AI analysis (cron/interval)
            │              │  Anomalies, summaries, correlation
            │  ai: {...}   │  Insights stored in ai_insights table
            └──────────────┘
```

### 2.1 Component Map

| Component | Location | Description |
|-----------|----------|-------------|
| LLM Provider Layer | `internal/llm/` | Provider interface, registry, 6 adapters |
| AI Enrichment Worker | `internal/ai/enrichment/` | NATS consumer, LLM calls, entity patching |
| AI Analysis Engine | `internal/ai/analysis/` | YAML loader, scheduler, insight storage |
| AI Task Engine | `internal/ai/tasks/` | Multi-step pipeline executor |
| AI Target Registry | `internal/ai/targets/` | Target definitions, delivery routing |
| Schema Registry | `internal/ai/schema/` | JSON Schema compilation + validation |
| Prompt Renderer | `internal/ai/prompts/` | Go template rendering + prompt hashing |
| SQL Validator | `internal/ai/query/` | PostgreSQL AST validation (pg_query_go) |
| WebSocket Notifier | `internal/ai/notify/` | Insight + enrichment push to clients |
| AI Config | `internal/ai/config/` | Shared config types for ai: sections |
| AI BFF | `internal/bff/ai_service.go` | Business facade (NL search, analysis, insights) |
| AI gRPC Server | `internal/server/ai_grpc_server.go` | gRPC handlers for 5 RPCs |
| AI Proto | `api/proto/ai.proto` | Service definition |
| Database Migrations | `migrations/010-012` | ai_metadata, ai_insights, ai_enrichment_log, ai_query_log |
| Frontend | `frontend/.../AIAnalysisTab.tsx` | Entity AI metadata display |

### 2.2 Data Flow: Enrichment

```
sources.d/*.yaml ──> Feeder persists Entity+Observation to PostgreSQL
                            │
                            │ Publish Job to NATS (respondent.ai.enrich.<source>)
                            │
                            v
                  Enrichment Worker (pool)
                  1. Load source AI config
                  2. Evaluate CEL filter per operation
                  3. Check Valkey dedup cache (entity-level + prompt-hash)
                  4. Render prompt (Go template)
                  5. Call LLM via provider
                  6. Validate response against JSON Schema
                  7. Apply output_mapping to entity/observation ai_metadata
                  8. Publish WebSocket notification
                  9. Write audit log to ai_enrichment_log
```

### 2.3 Data Flow: Analysis

```
analysis.d/*.yaml ──> Analysis Engine loads at startup
                            │
                            │ Scheduler (cron/interval)
                            v
                  Run Analysis
                  1. Fetch data (layer-based query or raw SQL)
                  2. Apply CEL filter
                  3. Dedup records (if dedup configured)
                  4. Render prompt with data + output_schema
                  5. Call LLM
                  6. Validate response
                  7. Inject attention level (base schema)
                  8. Store insights in ai_insights + refs in ai_insight_refs
                  9. Push via WebSocket
                  10. Cleanup expired insights (hourly)
```

---

## 3. LLM Provider Layer

### 3.1 Supported Providers

| Provider | Config Key | Default Model | Notes |
|----------|-----------|---------------|-------|
| OpenAI | `openai` | gpt-4o | API key required |
| Anthropic | `anthropic` | claude-sonnet-4-20250514 | API key required |
| Google Gemini | `gemini` | gemini-2.5-flash | API key required |
| xAI | `xai` | grok-3 | API key required |
| Ollama | `ollama` | llama3 | Local, base_url required |
| LM Studio | `lmstudio` | (auto) | Local, base_url required |

### 3.2 Registry

`internal/llm/provider.go` defines a `Registry` with priority-based provider selection. The registry supports a preferred provider override and fallback. All providers implement the `Provider` interface:

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Name() string
    HealthCheck(ctx context.Context) error
}
```

### 3.3 Retry Logic

`internal/llm/retry.go` provides `RetryableComplete()` with quadratic backoff + jitter. Retries on 429 (rate limit), 5xx (server error), and connection errors.

---

## 4. AI Enrichment Pipeline (sources.d/)

### 4.1 Source YAML: ai: Section

Add an `ai:` block to any source YAML to enable enrichment:

```yaml
ai:
  enabled: true
  operations:
    - name: classification
      tags: [enrichment, classification]
      filter: |
        has(entity.metadata.flight) && !has(entity.metadata.ai_enriched)
      prompt: |
        Analyze this flight: {{.Entity.Metadata.flight}}
        Altitude: {{.Observation.Metadata.alt_baro}}ft
        Speed: {{.Observation.Metadata.gs}}kts
        Heading: {{.Observation.Metadata.track}}deg

        Respond with JSON matching this schema:
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [airline, phase]
        properties:
          airline: { type: string }
          phase: { type: string, enum: [climb, cruise, descent, approach, ground] }
          risk_flags: { type: array, items: { type: string } }
      output_mapping:
        airline: "ai_airline"
        phase: "ai_flight_phase"
        risk_flags: "ai_risk_flags"
      cache_ttl: "24h"
```

### 4.2 Operation Config Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Required. Human-readable identifier |
| `tags` | string[] | Freeform tags for observability |
| `filter` | string | CEL expression to select which entities trigger this operation |
| `prompt` | string | Required. Go template for LLM prompt. Supports `{{.OutputSchema}}` |
| `output_schema` | map | Inline JSON Schema for response validation |
| `output_mapping` | map | Allowlist mapping LLM response fields to `ai_metadata` keys |
| `output_target` | string | `entity` (default) or `observation` -- which `ai_metadata` to patch |
| `cache_ttl` | string | Go duration. Skips enrichment if already enriched within TTL |
| `batch` | object | `{ size: N, timeout: "Xs" }` -- reserved, not yet active |
| `retry` | object | `{ max_attempts: N, backoff: "exponential" }` |
| `max_tokens` | int | Override provider default (1-128000) |
| `temperature` | float | 0.0-2.0; 0 uses provider default |
| `output` | object | Insight storage config (used by analysis.d/, not sources.d/) |

### 4.3 Metadata Separation

The feeder owns `metadata` (rebuilt from CEL each cycle). AI workers own `ai_metadata` (persists across ingestion cycles). These are separate JSONB columns:

```
entities.ai_metadata     -- AI workers write here
entities.metadata         -- Feeder writes here
observations.ai_metadata -- AI workers write here
observations.metadata     -- Feeder writes here
```

### 4.4 Caching (Two Layers)

1. **Entity-level TTL**: `ai:enrich:{entity_uuid}:{operation_name}` in Valkey. If key exists within `cache_ttl`, skip entirely.
2. **Prompt-hash**: `ai:prompt:{sha256_hash}` in Valkey. Different entities with identical prompts return cached response.

### 4.5 Prompt Template Variables

| Variable | Description |
|----------|-------------|
| `{{.Entity.Name}}` | Entity display name |
| `{{.Entity.ExternalID}}` | External identifier (e.g., callsign) |
| `{{.Entity.Metadata.field}}` | Any entity metadata field |
| `{{.Entity.MetadataJSON}}` | Full metadata as JSON string |
| `{{.Observation.Lat}}` | Observation latitude |
| `{{.Observation.Lon}}` | Observation longitude |
| `{{.Observation.Metadata.field}}` | Any observation metadata field |
| `{{.OutputSchema}}` | Compiled JSON Schema string for this operation |
| `{{.LayerType}}` | Entity layer type |
| `{{.HasCoordinates}}` | Whether entity has valid coordinates |
| `{{.MetadataKeys}}` | Comma-separated list of metadata field names |

---

## 5. Declarative Analysis (analysis.d/)

### 5.1 Analysis YAML Schema

```yaml
schema_version: 1
name: my_analysis
display_name: "My Analysis"
enabled: true

schedule:
  interval: "5m"          # Go duration
  # cron: "*/5 * * * *"   # Alternative: cron expression

data:
  layers:
    - flights_commercial
  lookback: "1h"
  min_records: 10
  max_records: 500
  filter: |                # Optional CEL expression
    has(entity.metadata.flight)
  sql: |                   # Optional raw SQL (must be SELECT-only)
    SELECT e.id, e.name, e.metadata
    FROM entities e
    JOIN observations o ON o.entity_id = e.id
    WHERE e.layer_type = ANY(ARRAY['flights_commercial'])
      AND o.ts > now() - INTERVAL '1 hour'
    ORDER BY o.ts DESC
    LIMIT 500
  dedup:                   # Optional cross-run dedup
    enabled: true
    key_fields: ["entity_id", "anomaly_type"]
    window: "1h"

ai:
  enabled: true
  operations:
    - name: scan
      tags: [anomaly]
      prompt: |
        Analyze {{.RecordCount}} records from {{.Lookback}}.
        {{range .Records}}
        - {{.EntityName}}: {{.Metadata}}
        {{end}}
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [anomalies]
        properties:
          anomalies:
            type: array
            items:
              type: object
              required: [entity_id, anomaly_type, severity, title, description]
              properties:
                entity_id: { type: string }
                anomaly_type: { type: string }
                severity: { type: integer, minimum: 0, maximum: 4 }
                title: { type: string }
                description: { type: string }
      max_tokens: 2000
      temperature: 0.1
      output:
        store_insights: true
        insight_type: "anomaly"
        results_path: "anomalies"
        websocket_push: true
        retention: "168h"
```

### 5.2 Attention Base Schema

Every analysis operation automatically gets an `attention` field injected into its `output_schema`. The engine appends a prompt suffix instructing the LLM to include it. Values:

| Level | Rank | Meaning |
|-------|------|---------|
| `info` | 0 | Routine background, no action needed |
| `low` | 1 | Minor observation, review when convenient |
| `medium` | 2 | Notable, review within hours |
| `high` | 3 | Significant, requires prompt attention |
| `critical` | 4 | Urgent, requires immediate action |

YAML authors do not need to define this field -- it is injected automatically by `MergeBaseSchema()` at load time. The `attention` and `attention_rank` columns on `ai_insights` enable efficient filtering (e.g., "show me all high and critical insights").

### 5.3 Insight Storage and Associations

When `output.store_insights: true`, the engine stores results in `ai_insights`:

- If `results_path` is set (e.g., `"anomalies"`), each array item becomes one `ai_insights` row.
- If `results_path` is omitted, the entire response becomes one row.

Associations in `ai_insight_refs` are created by convention fields in the LLM response:

| Field | Action |
|-------|--------|
| `entity_external_id` | Resolve via `entities.external_id` + `layer_type`, create ref |
| `entity_external_ids` | Resolve each, create one ref per entity |
| `entity_id` | Direct ref by database UUID |
| `entity_ids` | Direct ref per UUID |
| `observation_id` | Create ref to observation |
| `observation_ids` | Create one ref per observation |

### 5.4 Cross-Run Dedup

The `dedup` section prevents duplicate insights across analysis runs:

```yaml
data:
  dedup:
    enabled: true
    key_fields: ["entity_id", "anomaly_type"]
    window: "1h"
```

The engine hashes the specified fields from each result, checks recent `ai_insights.dedup_key` entries within the window, and skips duplicates. Requires migration `012_ai_insights_dedup_key`.

### 5.5 Active Analysis Definitions

16 analysis definitions are shipped in `analysis.d/`:

| Definition | Layers | Interval |
|-----------|--------|----------|
| `military_conflict_proximity` | flights_military | 5m |
| `space_launch_awareness` | space_objects | 10m |
| `volcanic_aviation_hazard` | volcanic_activity | 5m |
| `source_quality_audit` | all | 30m |
| `severe_weather_aviation` | weather, flights_commercial | 5m |
| `satellite_conjunction` | space_objects | 15m |
| `maritime_cable_threat` | maritime | 10m |
| `ocean_buoy_anomaly` | ocean | 5m |
| `lightning_fire_prediction` | lightning, weather | 10m |
| `radiation_anomaly_correlation` | radiation, space_weather | 15m |
| `infrastructure_threat_assessment` | infrastructure | 10m |
| `environmental_cascade` | multiple | 15m |
| `geopolitical_hotspot_index` | geopolitical | 30m |
| `border_tension_index` | border | 15m |
| `ham_radio_emergency` | ham_radio | 5m |
| `flight_anomaly_detection` | flights_commercial | disabled |

---

## 6. Task Pipelines (tasks.d/)

### 6.1 Task YAML Schema

```yaml
schema_version: 1
name: my_task
display_name: "My Task Pipeline"
enabled: true

trigger:
  type: event              # "event" or "schedule"
  source: usgs_earthquakes  # For event: match enrichment source
  filter: |                 # Optional CEL
    double(entity.metadata.magnitude) >= 5.0

  # For schedule triggers:
  # type: schedule
  # interval: "10m"
  # cron: "0 * * * *"

steps:
  - name: fetch_data
    type: http_request
    url: "https://api.example.com/data?lat={{.Entity.Lat}}"
    method: GET

  - name: analyze
    type: ai
    prompt: |
      Analyze this event: {{.Entity.Name}}
      External data: {{.Steps.fetch_data.Result}}
      {{.OutputSchema}}
    output_schema:
      type: object
      required: [impact_level]
      properties:
        impact_level: { type: string }

  - name: deliver
    type: target
    targets:
      - slack_alerts
```

### 6.2 Step Types

| Type | Description |
|------|-------------|
| `ai` | LLM call with prompt rendering, schema validation |
| `http_request` | External HTTP request with template rendering |
| `target` | Route output to named delivery targets |

### 6.3 Trigger Types

- **event**: Fires when enrichment completes on a matching source. The `EventHandler` subscribes to enrichment completion notifications.
- **schedule**: Runs on cron/interval via `robfig/cron` scheduler.

---

## 7. Targets (targets.d/)

### 7.1 Target YAML Schema

```yaml
schema_version: 1
name: slack_alerts
type: slack
enabled: true

connection:
  webhook_url: "${SLACK_WEBHOOK_URL}"
  channel: "#alerts"

ai:
  enabled: true
  operations:
    - name: format_for_slack
      tags: [formatting]
      prompt: |
        Format this for Slack: {{.Insight}}
      output_schema:
        type: object
        required: [text]
        properties:
          text: { type: string, maxLength: 500 }
```

### 7.2 Delivery Status

Target definitions load and validate correctly. The `Registry.Deliver()` method currently logs delivery but does not send to actual external services (Slack, Discord, email). Actual delivery adapters are a future enhancement.

---

## 8. Structured Output Validation

### 8.1 Validation Pipeline

```
LLM Response (raw string)
  │
  ├── extractJSON() -- strip markdown fences
  │
  ├── json.Unmarshal() into map[string]any
  │
  ├── jsonschema.Validate() -- validate against YAML-defined JSON Schema
  │
  └── Validated map[string]any -- applied via output_mapping
```

### 8.2 Design Principle

No hardcoded Go structs for entity types. All output schemas are defined inline in YAML (`output_schema`). The Go runtime is a generic execution engine. Adding AI enrichment for a new source requires only editing YAML -- zero Go changes.

### 8.3 Schema Registry

`internal/ai/schema/registry.go` compiles JSON Schema definitions from YAML at source load time. Schemas are keyed as `{source_name}:{operation_name}`. The compiled JSON Schema string is available for prompt injection via `SchemaJSON(key)`.

---

## 9. SQL Validation (Natural Language Search)

### 9.1 Multi-Layer Defense

| Layer | Mechanism |
|-------|-----------|
| 1 | `pg_query_go` AST parser rejects non-SELECT at parse time |
| 2 | Read-only DB connection (`SET default_transaction_read_only = ON`) |
| 3 | Statement timeout (`SET LOCAL statement_timeout`) per query |
| 4 | Row limit (LIMIT 1000 max if not present) |
| 5 | All generated SQL logged in `ai_query_log` for audit |

The validator (`internal/ai/query/validator.go`) rejects INSERT, UPDATE, DELETE, DDL, DCL, COPY, EXPLAIN, transaction control, SET, locking clauses, and CTEs with mutations. 84 tests, 91.7% coverage.

---

## 10. API

### 10.1 gRPC / REST Endpoints

| RPC | HTTP | Description |
|-----|------|-------------|
| `NaturalLanguageSearch` | POST `/v1/ai/search` | Text-to-SQL entity search |
| `AnalyzeEntity` | POST `/v1/ai/analyze` | On-demand entity deep-dive |
| `GetInsights` | GET `/v1/ai/insights` | List stored AI insights |
| `ExplainQuery` | POST `/v1/ai/explain` | SQL explanation in plain English |
| `ListAnalysisDefinitions` | GET `/v1/ai/analysis` | List loaded analysis definitions |

### 10.2 WebSocket Messages

**Insight notification:**
```json
{
  "type": "ai_insight",
  "payload": {
    "id": "uuid",
    "insight_type": "anomaly",
    "source_name": "flight_anomaly_detection",
    "operation_name": "flight_anomaly_scan",
    "result": { ... },
    "entity_ids": ["uuid"],
    "created_at": "2026-03-20T14:32:00Z"
  }
}
```

**Enrichment notification:**
```json
{
  "type": "ai_enrichment",
  "payload": {
    "entity_id": "uuid",
    "operation_name": "classification",
    "enriched_fields": ["ai_airline", "ai_flight_phase"],
    "source_name": "adsb_military"
  }
}
```

---

## 11. Database Schema

### 11.1 Tables (migration 010)

- `entities.ai_metadata` / `observations.ai_metadata` -- JSONB, GIN indexed. AI enrichment data separate from ingestion metadata.
- `ai_insights` -- Stored AI analysis results with `insight_type`, `source_name`, `operation_name`, `layer_type`, `result` (JSONB), `expires_at`
- `ai_insight_refs` -- Many-to-many between insights and entities/observations. Partial unique indexes prevent duplicate associations.
- `ai_enrichment_log` -- Audit trail for enrichment jobs (status tracking, token usage, latency, error logging)
- `ai_query_log` -- Audit trail for NL search queries (generated SQL, explanation, token usage)

### 11.2 Additional Columns (migrations 011-012)

- `ai_insights.attention` -- TEXT, one of info/low/medium/high/critical
- `ai_insights.attention_rank` -- SMALLINT generated column (0-4) for >= filtering
- `ai_insights.dedup_key` -- TEXT, composite hash for cross-run deduplication

---

## 12. Configuration

### 12.1 respondent.yaml: LLM Section

```yaml
llm:
  provider: "openai"       # Active provider
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4o"
    max_tokens: 1024
  anthropic:
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-sonnet-4-20250514"
  gemini:
    api_key: "${GEMINI_API_KEY}"
    model: "gemini-2.5-flash"
  xai:
    api_key: "${XAI_API_KEY}"
    model: "grok-3"
  ollama:
    base_url: "http://localhost:11434"
    model: "llama3"
  lmstudio:
    base_url: "http://localhost:1234/v1"
```

### 12.2 respondent.yaml: AI Section

```yaml
ai:
  enabled: true

  workers:
    enrichment_concurrency: 4    # Parallel enrichment workers
    analysis_concurrency: 2      # Parallel analysis workers
    rate_limit: 10               # Max LLM calls/sec (global token-bucket, 0 to disable)
    max_queue_size: 10000
    dlq_max_retries: 3

  search:
    enabled: true
    max_results: 1000
    query_timeout: "5s"
    cache_ttl: "10m"

  nats:
    url: "nats://localhost:4222"
    stream_name: "RESPONDENT_AI"
    enrichment_subject: "respondent.ai.enrich"
    analysis_subject: "respondent.ai.analysis"
    consumer_group: "ai-workers"
    max_deliver: 3
    ack_wait: "60s"
    stream_max_msgs: 10000
    stream_max_bytes: 52428800
    stream_max_age: "24h"
    circuit_breaker:
      failure_threshold: 5
      initial_cooldown: "5m"
      max_cooldown: "1h"

  insights:
    retention: "168h"           # 7 days default
    max_per_entity: 100
    websocket_push: true

  budget:
    daily_token_limit: 1000000  # Config only, not enforced
    alert_threshold: 0.8
    enrichment_max_tokens: 500
    search_max_tokens: 1000
    analysis_max_tokens: 2000

  analysis_dir: "analysis.d/"
  tasks_dir: "tasks.d/"
  targets_dir: "targets.d/"
```

### Stream Backpressure

The `RESPONDENT_AI` JetStream stream enforces bounded limits to prevent unbounded
message accumulation when the enrichment worker is unavailable:

| Setting | Config Key | Default | Description |
|---------|------------|---------|-------------|
| Max Messages | `ai.nats.stream_max_msgs` | 10,000 | Maximum messages in stream |
| Max Bytes | `ai.nats.stream_max_bytes` | 52,428,800 (50 MB) | Maximum stream storage |
| Max Age | `ai.nats.stream_max_age` | 24h | Message TTL |

The stream uses `DiscardNew` policy: when limits are reached, new publishes are
rejected rather than silently dropping old unprocessed messages.

### Enrichment Publisher Circuit Breaker

The feeder's enrichment publisher includes a circuit breaker that detects
consecutive publish failures and backs off with exponential cooldown:

| Setting | Config Key | Default | Description |
|---------|------------|---------|-------------|
| Failure Threshold | `ai.nats.circuit_breaker.failure_threshold` | 5 | Consecutive failures to open circuit |
| Initial Cooldown | `ai.nats.circuit_breaker.initial_cooldown` | 5m | First cooldown duration |
| Max Cooldown | `ai.nats.circuit_breaker.max_cooldown` | 1h | Cooldown cap |

**States:** Closed (normal) → Open (dropping) → Half-Open (probing) → Closed.
Each failed probe doubles the cooldown, capped at `max_cooldown`. A successful
publish resets the cooldown to the initial value.

Dropped enrichment jobs are acceptable: entities are already persisted, and the
next ingest cycle produces new jobs. The enrichment worker deduplicates via
entity-level and prompt-hash caches.

---

## 13. Security

### SQL Injection Prevention (NL Search)

6-layer defense: AST parsing, read-only connection, statement timeout, row limit, restricted DB role, audit log.

### Prompt Injection Mitigation

- Entity metadata sanitized before prompt inclusion
- LLM output validated against JSON Schema -- free text never executed
- Output mapping is allowlist-based (only mapped fields stored)

### API Key Security

- Keys in environment variables only, never in YAML or logs
- Keys excluded from health check responses and error messages

---

## 14. Tech Stack

| Concern | Technology | Location |
|---------|-----------|----------|
| LLM Interface | `internal/llm.Provider` | 6 provider adapters |
| Job Queue | NATS JetStream | `RESPONDENT_AI` stream |
| Cache | Valkey/Redis | Entity-level TTL + prompt-hash dedup |
| Database | PostgreSQL | 4 AI tables + ai_metadata columns |
| SQL AST Parser | `pganalyze/pg_query_go` v6 | Formal SQL validation |
| JSON Schema | `santhosh-tekuri/jsonschema` v6 | LLM response validation |
| CEL Expressions | `google/cel-go` v0.24 | Filter expressions |
| Cron Scheduling | `robfig/cron` v3 | Analysis/task scheduling |
| Struct Validation | `go-playground/validator` v10 | Config validation |
| API | gRPC + grpc-gateway | 5 RPCs with REST endpoints |
