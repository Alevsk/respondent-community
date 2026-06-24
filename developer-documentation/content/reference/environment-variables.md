---
title: "Environment Variables"
description: "All RESPONDENT_* environment variables for configuration and credential management"
weight: 5
---

Every Respondent configuration key can be set via environment variable. The naming convention maps config file keys to env vars by uppercasing and replacing dots with underscores, prefixed with `RESPONDENT_`.

```
config.key.path  ->  RESPONDENT_CONFIG_KEY_PATH
```

For example, `server.port` in `respondent.yaml` maps to `RESPONDENT_SERVER_PORT`. Environment variables take precedence over values in the config file.

## Database

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_DATABASE_PATH` | `database.path` | string | `./respondent.db` | Path to the SQLite database file |
| `RESPONDENT_DATABASE_RETENTION_MAX_SIZE` | `database.retention.max_size` | string | `""` | High watermark as a humanized size (e.g. `50GB`); empty disables retention (opt-in) |
| `RESPONDENT_DATABASE_RETENTION_LOW_WATER_RATIO` | `database.retention.low_water_ratio` | float | `0.90` | Prune down to `max_size * this ratio` |
| `RESPONDENT_DATABASE_RETENTION_CHECK_INTERVAL` | `database.retention.check_interval` | duration | `10m` | How often size is measured and pruning runs |
| `RESPONDENT_DATABASE_RETENTION_BATCH_SIZE` | `database.retention.batch_size` | integer | `5000` | Observation rows deleted per transaction |
| `RESPONDENT_DATABASE_RETENTION_MAX_BATCHES_PER_TICK` | `database.retention.max_batches_per_tick` | integer | `200` | Max delete batches per tick |
| `RESPONDENT_DATABASE_RETENTION_INCREMENTAL_VACUUM_PAGES` | `database.retention.incremental_vacuum_pages` | integer | `10000` | Pages reclaimed to the OS per tick |

## Server

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_SERVER_PORT` | `server.port` | integer | `8090` | HTTP/WebSocket listening port |

## Frontend

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_FRONTEND_CESIUM_ION_TOKEN` | `frontend.cesium_ion_token` | string | `""` | Cesium ion client token for 3D globe terrain and imagery; served to the browser via `GET /config.json` |

## Ingest

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_INGEST_ENABLED` | `ingest.enabled` | boolean | `true` | Gates the live feeder; set `false` to serve a fully static instance (no source fetching) |
| `RESPONDENT_INGEST_SOURCES_DIR` | `ingest.sources_dir` | string | _(none)_ | Directory containing source definition YAML files; no code-backed default — empty unless set (the shipped `respondent.yaml` uses `./sources.d`) |
| `RESPONDENT_INGEST_DEV_MODE` | `ingest.dev_mode` | boolean | `false` | Enable dev mode for dry-run testing |

## AI

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_AI_ENABLED` | `ai.enabled` | boolean | `false` | Enable AI enrichment and analysis features |
| `RESPONDENT_AI_MIN_ATTENTION` | `ai.min_attention` | string | `low` | Engine-wide noise floor (`info`/`low`/`medium`/`high`/`critical`); insights below it are not stored or notified unless an analysis sets `output.min_attention` |
| `RESPONDENT_AI_ANALYSIS_DIR` | `ai.analysis_dir` | string | _(none)_ | Directory containing analysis definition YAML files; no code-backed default — empty unless set (the shipped `respondent.yaml` uses `./analysis.d`) |
| `RESPONDENT_AI_WORKERS_ENRICHMENT` | `ai.workers.enrichment` | integer | `2` | Concurrent AI enrichment workers |
| `RESPONDENT_AI_WORKERS_ANALYSIS` | `ai.workers.analysis` | integer | `1` | Concurrent AI analysis workers |

## LLM Providers

{{< field name="RESPONDENT_LLM_PROVIDER" type="string" required="false" >}}
Active LLM provider. Allowed values: `openai`, `anthropic`, `gemini`, `xai`, `zai`, `ollama`, `lmstudio`. Maps to config key `llm.provider`.
{{< /field >}}

### Per-provider variables

Each provider has its own set of configuration variables. Only the active provider's variables need to be set.

**OpenAI**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_OPENAI_API_KEY` | `llm.openai.api_key` | OpenAI API key |
| `RESPONDENT_LLM_OPENAI_MODEL` | `llm.openai.model` | Model name (e.g., `gpt-4o`) |
| `RESPONDENT_LLM_OPENAI_MAX_TOKENS` | `llm.openai.max_tokens` | Max response tokens |
| `RESPONDENT_LLM_OPENAI_BASE_URL` | `llm.openai.base_url` | Custom API base URL |

**Anthropic**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_ANTHROPIC_API_KEY` | `llm.anthropic.api_key` | Anthropic API key |
| `RESPONDENT_LLM_ANTHROPIC_MODEL` | `llm.anthropic.model` | Model name (e.g., `claude-sonnet-4-20250514`) |
| `RESPONDENT_LLM_ANTHROPIC_MAX_TOKENS` | `llm.anthropic.max_tokens` | Max response tokens |
| `RESPONDENT_LLM_ANTHROPIC_BASE_URL` | `llm.anthropic.base_url` | Custom API base URL |

**Gemini**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_GEMINI_API_KEY` | `llm.gemini.api_key` | Google Gemini API key |
| `RESPONDENT_LLM_GEMINI_MODEL` | `llm.gemini.model` | Model name |
| `RESPONDENT_LLM_GEMINI_BASE_URL` | `llm.gemini.base_url` | Custom API base URL |

**xAI**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_XAI_API_KEY` | `llm.xai.api_key` | xAI API key |
| `RESPONDENT_LLM_XAI_MODEL` | `llm.xai.model` | Model name |
| `RESPONDENT_LLM_XAI_MAX_TOKENS` | `llm.xai.max_tokens` | Max response tokens |
| `RESPONDENT_LLM_XAI_BASE_URL` | `llm.xai.base_url` | Custom API base URL |

**ZAI**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_ZAI_API_KEY` | `llm.zai.api_key` | ZAI (GLM) API key |
| `RESPONDENT_LLM_ZAI_MODEL` | `llm.zai.model` | Model name (e.g. `glm-5.2`) |
| `RESPONDENT_LLM_ZAI_MAX_TOKENS` | `llm.zai.max_tokens` | Max response tokens |
| `RESPONDENT_LLM_ZAI_BASE_URL` | `llm.zai.base_url` | Custom API base URL |
| `RESPONDENT_LLM_ZAI_TIMEOUT` | `llm.zai.timeout` | Per-request HTTP timeout (default `120s`) |
| `RESPONDENT_LLM_ZAI_ENABLE_THINKING` | `llm.zai.enable_thinking` | Enable the GLM reasoning phase (default `false`) |

**Ollama**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_OLLAMA_MODEL` | `llm.ollama.model` | Model name |
| `RESPONDENT_LLM_OLLAMA_BASE_URL` | `llm.ollama.base_url` | Ollama server URL (required) |

**LM Studio**

| Variable | Config Key | Description |
|----------|-----------|-------------|
| `RESPONDENT_LLM_LMSTUDIO_MODEL` | `llm.lmstudio.model` | Model name |
| `RESPONDENT_LLM_LMSTUDIO_MAX_TOKENS` | `llm.lmstudio.max_tokens` | Max response tokens |
| `RESPONDENT_LLM_LMSTUDIO_BASE_URL` | `llm.lmstudio.base_url` | LM Studio server URL (required) |

## Logging

| Variable | Config Key | Type | Default | Description |
|----------|-----------|------|---------|-------------|
| `RESPONDENT_LOGGING_LEVEL` | `logging.level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |

{{< callout type="info" title="Log format is always console" >}}
The community binary always emits human-readable console output; there is no log-format setting. `RESPONDENT_LOGGING_FORMAT` is not honored.
{{< /callout >}}

## Source Credentials

Source definitions use the `${VAR}` pattern for credential injection. When the YAML loader encounters `${MY_API_KEY}` in a source file, it resolves it from the environment variable `RESPONDENT_MY_API_KEY`.

This applies to:
- **URL paths**: `url: "https://api.example.com/data/${MY_KEY}/endpoint"`
- **Auth env_var fields**: `env_var: "MY_API_TOKEN"` resolves from `RESPONDENT_MY_API_TOKEN`
- **OAuth2 credentials**: `env_var: "MY_CLIENT_SECRET"` resolves from `RESPONDENT_MY_CLIENT_SECRET`

```yaml
# Source YAML
transport:
  url: "https://api.example.com/data"
  auth:
    type: bearer
    env_var: "NASA_FIRMS_MAP_KEY"
```

```bash
# Environment
export RESPONDENT_NASA_FIRMS_MAP_KEY="your-api-key-here"
```

{{< callout type="warning" title="Never hardcode secrets" >}}
Secrets are never stored in YAML source files. The `env_var` field stores only the variable name (without the `RESPONDENT_` prefix). The actual value is resolved at runtime from the environment.
{{< /callout >}}

{{< callout type="info" title="Docker Compose" >}}
In the Docker Compose deployment, set source credentials in a `.env` file or directly in the `environment` section of the feeder service. All variables must use the `RESPONDENT_` prefix.
{{< /callout >}}
