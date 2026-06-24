---
title: "Configuration"
description: "Configure the Respondent community edition — database, server, AI, and LLM providers"
weight: 2
---

All Respondent configuration lives in a single file: `respondent.yaml`. This page covers every section.

## Database

{{< field name="database.path" type="string" required="true" default="./respondent.db" >}}
Path to the SQLite database file. Created automatically if it does not exist. Inside Docker, the compose file overrides this to `/data/respondent.db` via the `RESPONDENT_DATABASE_PATH` environment variable so the database is persisted in a Docker volume.
{{< /field >}}

```yaml
database:
  path: "./respondent.db"
```

### Retention (size-cap)

The `observations` table is append-only and grows without bound. The retention routine is **opt-in**: it stays disabled until `database.retention.max_size` is set. When enabled, it measures the on-disk size on an interval and, if the database exceeds `max_size`, prunes the **oldest** observations in batches down to `max_size * low_water_ratio`, then returns the freed space to the OS. Live ingestion is **never throttled** — the cap is chased by pruning.

{{< field name="database.retention.max_size" type="string" required="false" default="" >}}
High watermark as a humanized byte size (e.g. `50GB`, `500MB`). Empty disables retention (the default — retention is opt-in).
{{< /field >}}

{{< field name="database.retention.low_water_ratio" type="float" required="false" default="0.90" >}}
Low watermark as a fraction of `max_size`. Pruning continues until the database is at or below `max_size * low_water_ratio`. The gap between the two is the hysteresis band. Must be between 0 and 1 (exclusive) when retention is enabled.
{{< /field >}}

{{< field name="database.retention.check_interval" type="string" required="false" default="10m" >}}
How often the maintenance routine measures size and runs pruning.
{{< /field >}}

{{< field name="database.retention.batch_size" type="integer" required="false" default="5000" >}}
Number of observation rows deleted per transaction.
{{< /field >}}

{{< field name="database.retention.max_batches_per_tick" type="integer" required="false" default="200" >}}
Maximum delete batches run in a single tick (~1M rows at the default batch size). If still over the low watermark, the next tick continues.
{{< /field >}}

{{< field name="database.retention.incremental_vacuum_pages" type="integer" required="false" default="10000" >}}
Pages reclaimed to the OS per tick via `PRAGMA incremental_vacuum` (~40MB at 4KB pages), bounding the reclaim lock window.
{{< /field >}}

```yaml
database:
  path: "./respondent.db"
  retention:
    max_size: ""                      # e.g. "50GB" / "500MB"; empty disables retention
    low_water_ratio: 0.90
    check_interval: "10m"
    batch_size: 5000
    max_batches_per_tick: 200
    incremental_vacuum_pages: 10000
```

## Server

{{< field name="server.port" type="integer" required="false" default="8090" >}}
The port Respondent listens on for the web UI, REST API, and WebSocket connections. All three are served from the same port.
{{< /field >}}

```yaml
server:
  port: 8090
```

## Ingest

{{< field name="ingest.sources_dir" type="string" required="true" default="./sources.d" >}}
Path to the directory containing source definition YAML files. Respondent loads every `.yaml` file in this directory at startup.
{{< /field >}}

{{< field name="ingest.dev_mode" type="boolean" required="false" default="false" >}}
When enabled, sources with `dry_run: true` will fetch and parse data but skip database persistence. Useful for testing new source definitions without writing to the database.
{{< /field >}}

```yaml
ingest:
  sources_dir: "./sources.d"
  # dev_mode: false
```

## AI

Controls AI-powered enrichment and analysis features. Requires a configured LLM provider.

{{< field name="ai.enabled" type="boolean" required="false" default="false" >}}
Master switch for all AI features. Set to `true` and configure the `llm` section to enable AI enrichment on sources that define `ai.operations`.
{{< /field >}}

{{< field name="ai.analysis_dir" type="string" required="false" default="./analysis.d" >}}
Path to the directory containing analysis definition files.
{{< /field >}}

{{< field name="ai.min_attention" type="string" required="false" default="low" >}}
Engine-wide noise floor: analysis insights below this attention level are neither stored nor notified. One of `info`, `low`, `medium`, `high`, `critical`. `low` drops routine info-level observations platform-wide. A per-analysis `output.min_attention` overrides this value.
{{< /field >}}

{{< field name="ai.workers.enrichment" type="integer" required="false" default="2" >}}
Number of concurrent workers processing AI enrichment operations. Increase if you have many sources with AI operations and sufficient LLM API quota.
{{< /field >}}

{{< field name="ai.workers.analysis" type="integer" required="false" default="1" >}}
Number of concurrent workers processing analysis jobs.
{{< /field >}}

```yaml
ai:
  enabled: true
  analysis_dir: "./analysis.d"
  min_attention: "low"
  workers:
    enrichment: 2
    analysis: 1
```

## LLM

Configure the LLM provider used for AI enrichment. Set `llm.provider` to the provider name, then add the provider-specific block.

{{< field name="llm.provider" type="string" required="true" default="zai" >}}
The LLM provider to use. Must match one of the configured provider blocks below. Valid values: `zai`, `openai`, `anthropic`, `gemini`, `xai`, `ollama`, `lmstudio`. The shipped `respondent.yaml` defaults to `zai` (GLM).
{{< /field >}}

{{< code-tabs >}}
{{< tab title="ZAI (GLM)" >}}
```yaml
llm:
  provider: "zai"
  zai:
    base_url: "https://api.z.ai/api/coding/paas/v4"
    model: "glm-5.2"
    max_tokens: 8192
    timeout: "120s"          # GLM latency headroom on large prompts
    enable_thinking: false   # structured JSON output: skip reasoning (faster, no truncation)
    # api_key: set via RESPONDENT_LLM_ZAI_API_KEY env var
```
{{< /tab >}}
{{< tab title="OpenAI" >}}
```yaml
llm:
  provider: "openai"
  openai:
    model: "gpt-4o"
    max_tokens: 2048
    # api_key: set via RESPONDENT_LLM_OPENAI_API_KEY env var
```
{{< /tab >}}
{{< tab title="Anthropic" >}}
```yaml
llm:
  provider: "anthropic"
  anthropic:
    model: "claude-sonnet-4-20250514"
    max_tokens: 2048
    # api_key: set via RESPONDENT_LLM_ANTHROPIC_API_KEY env var
```
{{< /tab >}}
{{< tab title="xAI" >}}
```yaml
llm:
  provider: "xai"
  xai:
    model: "grok-3"
    max_tokens: 2048
    # api_key: set via RESPONDENT_LLM_XAI_API_KEY env var
```
{{< /tab >}}
{{< tab title="Ollama (local)" >}}
```yaml
llm:
  provider: "ollama"
  ollama:
    model: "llama3"
    max_tokens: 2048
    base_url: "http://host.docker.internal:11434"
```
{{< /tab >}}
{{< tab title="LM Studio (local)" >}}
```yaml
llm:
  provider: "lmstudio"
  lmstudio:
    base_url: "http://localhost:1234/v1"
    # model is optional — LM Studio serves the currently loaded model when blank.
    model: "qwen/qwen3.6-35b-a3b"
    max_tokens: 4096
```
{{< /tab >}}
{{< /code-tabs >}}

`gemini` is also a valid provider; configure a `llm.gemini` block with `api_key`, `model`, and optional `base_url`.

{{< callout type="warning" title="Never put API keys in YAML" >}}
Store API keys in environment variables or a `.env` file. See [Environment variable overrides](#environment-variable-overrides) below.
{{< /callout >}}

## Geocoder

Configures the geocoding service used by AI enrichment to resolve location names to coordinates.

{{< field name="geocoder.provider" type="string" required="false" default="nominatim" >}}
Geocoding provider. Currently supports `nominatim` (OpenStreetMap's free geocoding service).
{{< /field >}}

{{< field name="geocoder.nominatim.base_url" type="string" required="false" default="https://nominatim.openstreetmap.org" >}}
Nominatim API base URL. Use the public instance or host your own.
{{< /field >}}

{{< field name="geocoder.nominatim.user_agent" type="string" required="false" default="respondent/1.0 (geospatial-intelligence)" >}}
User-Agent header sent with geocoding requests. The public Nominatim instance requires a descriptive user agent.
{{< /field >}}

{{< field name="geocoder.rate_limit.requests_per_second" type="float" required="false" default="1.0" >}}
Maximum geocoding requests per second. The public Nominatim instance enforces a limit of 1 request per second.
{{< /field >}}

{{< field name="geocoder.rate_limit.burst" type="integer" required="false" default="5" >}}
Maximum burst of requests allowed before rate limiting kicks in.
{{< /field >}}

{{< field name="geocoder.cache.enabled" type="boolean" required="false" default="true" >}}
Cache geocoding results to reduce API calls and improve latency.
{{< /field >}}

{{< field name="geocoder.cache.ttl" type="string" required="false" default="168h" >}}
How long cached geocoding results are kept. Default is 7 days.
{{< /field >}}

```yaml
geocoder:
  provider: "nominatim"
  nominatim:
    base_url: "https://nominatim.openstreetmap.org"
    user_agent: "respondent/1.0 (geospatial-intelligence)"
  rate_limit:
    requests_per_second: 1.0
    burst: 5
  cache:
    enabled: true
    ttl: "168h"
```

## Frontend

Client-safe values the server hands to the embedded SPA at runtime via `GET /config.json`, so per-deployment frontend config reaches the already-compiled bundle without a rebuild. Empty values are omitted from the payload so the frontend's fallback chain (runtime config → build-time env → default) stays intact.

{{< field name="frontend.cesium_ion_token" type="string" required="false" default="" >}}
A Cesium ion **client** token used by the browser for 3D globe terrain and imagery. When empty, the frontend falls back to its build-time token / Stadia-OSM imagery.
{{< /field >}}

```yaml
frontend:
  cesium_ion_token: ""   # set via RESPONDENT_FRONTEND_CESIUM_ION_TOKEN
```

{{< callout type="warning" title="Browser-exposed by design" >}}
Everything in the `frontend` block is served to the browser via `GET /config.json`. Never put server secrets here — only client-safe values like the Cesium ion client token.
{{< /callout >}}

## Logging

{{< field name="logging.level" type="string" required="false" default="info" >}}
Log verbosity level. One of: `debug`, `info`, `warn`, `error`.
{{< /field >}}

```yaml
logging:
  level: "info"
```

## Environment variable overrides

Any `respondent.yaml` field can be overridden with an environment variable using the `RESPONDENT_` prefix. Convert the YAML path to uppercase, replacing dots and nested keys with underscores.

| YAML path | Environment variable |
|---|---|
| `database.path` | `RESPONDENT_DATABASE_PATH` |
| `database.retention.max_size` | `RESPONDENT_DATABASE_RETENTION_MAX_SIZE` |
| `server.port` | `RESPONDENT_SERVER_PORT` |
| `ingest.enabled` | `RESPONDENT_INGEST_ENABLED` |
| `ai.enabled` | `RESPONDENT_AI_ENABLED` |
| `ai.min_attention` | `RESPONDENT_AI_MIN_ATTENTION` |
| `frontend.cesium_ion_token` | `RESPONDENT_FRONTEND_CESIUM_ION_TOKEN` |
| `llm.openai.api_key` | `RESPONDENT_LLM_OPENAI_API_KEY` |
| `llm.anthropic.api_key` | `RESPONDENT_LLM_ANTHROPIC_API_KEY` |
| `llm.zai.api_key` | `RESPONDENT_LLM_ZAI_API_KEY` |
| `logging.level` | `RESPONDENT_LOGGING_LEVEL` |

The other `database.retention.*` sub-keys follow the same pattern (e.g. `RESPONDENT_DATABASE_RETENTION_LOW_WATER_RATIO`, `RESPONDENT_DATABASE_RETENTION_CHECK_INTERVAL`). Setting `ingest.enabled` to `false` (e.g. `RESPONDENT_INGEST_ENABLED=false`) serves a fully static instance with no source fetching.

Set these in a `.env` file in the same directory as `compose.yaml`:

```bash
RESPONDENT_LLM_OPENAI_API_KEY=sk-your-key-here
RESPONDENT_AI_ENABLED=true
```

{{< callout type="info" title="Note" >}}
The `compose.yaml` from the installation guide already includes `env_file` with `required: false`, so the `.env` file is loaded automatically when it exists.
{{< /callout >}}

## Source-level credential expansion

Inside source YAML files (in `sources.d/`), you can reference environment variables with the `${VAR}` syntax. At load time, `${VAR}` is expanded by looking up `RESPONDENT_VAR` in the environment.

For example, if a source needs an API key:

```yaml
# sources.d/my_source.yaml
transport:
  type: http_poll
  url: "https://api.example.com/data?key=${MY_API_KEY}"
```

Set the corresponding environment variable:

```bash
# .env
RESPONDENT_MY_API_KEY=your-api-key-here
```

You can also use the `auth` block for structured credential injection:

```yaml
transport:
  auth:
    type: api_key
    header: "X-API-Key"
    env_var: "MY_API_KEY"   # Resolved from RESPONDENT_MY_API_KEY
```

## Complete example

A full `respondent.yaml` with AI enabled using OpenAI:

```yaml
database:
  path: "./respondent.db"

server:
  port: 8090

ingest:
  sources_dir: "./sources.d"

ai:
  enabled: true
  analysis_dir: "./analysis.d"
  workers:
    enrichment: 2
    analysis: 1

llm:
  provider: "openai"
  openai:
    model: "gpt-4o"
    max_tokens: 2048
    # api_key: set via RESPONDENT_LLM_OPENAI_API_KEY env var

geocoder:
  provider: "nominatim"
  nominatim:
    base_url: "https://nominatim.openstreetmap.org"
    user_agent: "respondent/1.0 (geospatial-intelligence)"
  rate_limit:
    requests_per_second: 1.0
    burst: 5
  cache:
    enabled: true
    ttl: "168h"

logging:
  level: "info"
```

## Next steps

- [Create your first data source]({{< relref "your-first-source" >}})
