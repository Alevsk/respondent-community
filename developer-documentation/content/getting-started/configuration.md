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
  workers:
    enrichment: 2
    analysis: 1
```

## LLM

Configure the LLM provider used for AI enrichment. Set `llm.provider` to the provider name, then add the provider-specific block.

{{< field name="llm.provider" type="string" required="true" default="" >}}
The LLM provider to use. Must match one of the configured provider blocks below.
{{< /field >}}

{{< code-tabs >}}
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
{{< /code-tabs >}}

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
| `server.port` | `RESPONDENT_SERVER_PORT` |
| `ai.enabled` | `RESPONDENT_AI_ENABLED` |
| `llm.openai.api_key` | `RESPONDENT_LLM_OPENAI_API_KEY` |
| `llm.anthropic.api_key` | `RESPONDENT_LLM_ANTHROPIC_API_KEY` |
| `logging.level` | `RESPONDENT_LOGGING_LEVEL` |

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
