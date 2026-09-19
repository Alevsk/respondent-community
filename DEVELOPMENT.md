# Contributing to Respondent

Thanks for your interest in contributing. This guide covers local development setup, authoring new YAML definitions, making code changes, testing, and submitting pull requests.

## Overview

This repository contains the full Respondent Community Edition:

- **Go backend** — `cmd/`, `internal/` (hexagonal architecture, SQLite, gRPC-gateway)
- **React + CesiumJS frontend** — `frontend/` (Vite, TanStack Query, Zustand, MUI)
- **Protobuf API definitions** — `api/proto/`
- **50+ data source definitions** — `sources.d/*.yaml`
- **AI analysis definitions** — `analysis.d/*.yaml`
- **Runtime config** — `respondent.yaml`, `.env.example`, `compose.yaml`
- **Hugo developer documentation site** — `developer-documentation/`

**Contributions welcome:**

- New YAML data sources in `sources.d/`
- New YAML AI analyses in `analysis.d/`
- Go backend improvements (features, bug fixes, tests)
- Frontend improvements (components, hooks, styling)
- Documentation improvements in `developer-documentation/content/`
- Bug reports and reproduction recipes

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.25+ | Required for backend |
| Node.js | 20+ | Required for frontend (npm included) |
| C compiler | gcc or clang | CGO is required for SQLite (`mattn/go-sqlite3`) |
| Docker + Compose | v20.10+ / v2.0+ | For container-based workflows |
| Buf CLI | Latest | Only if modifying `.proto` files (`make proto`) |

---

## Local Development Setup

### Clone and configure

```bash
git clone https://github.com/alevsk/respondent-community.git
cd respondent-community

# Optional: copy the env template and fill in API keys you have
cp .env.example .env
```

### Building from source

```bash
# Build the frontend and the Go binary
make run-frontend-build
make build

# Run the server
./bin/community serve --config respondent.yaml

# Open the globe
open http://localhost:8090
```

### Development with hot reload

```bash
# Terminal 1: Go backend (builds frontend + binary, runs the server)
make run

# Terminal 2: Vite dev server (proxies /v1 and /ws to localhost:8090)
make run-frontend
```

### Container-based workflow

If you only need to test YAML changes (sources, analyses, config), you can use the pre-built container:

```bash
docker compose up -d
docker compose logs -f
```

The container bind-mounts `sources.d/` and `analysis.d/` read-only. Edit YAML files and restart to reload:

```bash
docker compose restart
```

To reset the database (destructive):

```bash
docker compose down -v
docker compose up -d
```

---

## Adding a Data Source

A "source" is a YAML file that describes a complete ingestion pipeline: where to fetch data, how to parse it, how to map fields onto entities and observations, and how to display the result on the globe.

### Step-by-step

1. **Copy the template.**

   ```bash
   cp sources.d/TEMPLATE.yaml sources.d/my_new_source.yaml
   ```

   Or copy an existing source whose shape is similar to what you need (e.g. `usgs_earthquakes.yaml` for a GeoJSON poll, `aisstream_ships.yaml` for a WebSocket stream, `noaa_buoys.yaml` for a delimited text format).

2. **Set the identity fields.** `name`, `source_type`, and `layer_type` must each be unique within the repo (lowercase, underscores only). `display_name` is the human-readable label shown in the layer picker.

3. **Configure the transport.** Pick `http_poll`, `websocket`, or `mqtt` and fill in the URL, headers, interval, and retry policy. See [`developer-documentation/content/sources/transports.md`](./developer-documentation/content/sources/transports.md) for every field.

4. **Configure the parser.** Choose the format — `json`, `geojson`, `csv`, `array_columns`, `object_to_records`, or `text` — and set `records_path` to the array key in the response. See [`developer-documentation/content/sources/parsers.md`](./developer-documentation/content/sources/parsers.md).

5. **Add a CEL filter** (optional but recommended) to drop noise records before they hit the database. CEL receives each parsed record as the `record` variable. Use `has(record.field)` before accessing nested keys to avoid crashes on missing data.

6. **Map the entity and observation.** The `entity` block defines persistent identity (`external_id`, `name`, `metadata`); the `observation` block defines time-stamped, located samples (`latitude`, `longitude`, `timestamp`, optional `velocity`, `altitude`). All `metadata` values must be CEL expressions returning strings — wrap numbers with `string()`. See [`developer-documentation/content/sources/field-mapping.md`](./developer-documentation/content/sources/field-mapping.md).

7. **Configure the display.** Choose an `icon.shape`, color, point size, and `field_renderers` for the entity detail panel. See [`developer-documentation/content/sources/display-config.md`](./developer-documentation/content/sources/display-config.md).

8. **Set recording mode and cache TTL.** `upsert` keeps one row per entity (good for slowly-changing data); `append` writes a new row every poll (good for time-series). `cache.ttl` controls how long entities stay visible after the last successful poll — set this higher than your `transport.interval` to survive missed polls.

9. **If the source needs an API key**, add a placeholder to `.env.example` and reference it in your YAML using `${VAR}` substitution (resolved as `RESPONDENT_<VAR>` from the environment) or the structured `transport.auth.env_var` form.

10. **Update the catalog.** Add a row to the appropriate table in [`sources.d/README.md`](./sources.d/README.md) so the source is discoverable.

For a fully worked example, see the [Your First Source](./developer-documentation/content/getting-started/your-first-source.md) tutorial — it walks through building an EMSC earthquake source from a blank file.

### Useful references while authoring

- [`sources.d/TEMPLATE.yaml`](./sources.d/TEMPLATE.yaml) — annotated template covering every supported field.
- [`developer-documentation/content/sources/`](./developer-documentation/content/sources/) — full schema docs:
  - `transports.md` — `http_poll`, `websocket`, `mqtt`, retry, auth
  - `parsers.md` — every parser format
  - `field-mapping.md` — CEL expressions for entity / observation mapping
  - `display-config.md` — icons, colors, field renderers
  - `filtering.md` — CEL filter expressions
  - `recording-modes.md` — `upsert` vs `append`
  - `examples.md` — annotated real-world sources
- CEL language spec: <https://github.com/google/cel-spec>

---

## Adding an Analysis

An "analysis" is a scheduled AI pipeline: it queries one or more layers, formats the results into a prompt, calls an LLM with a constrained JSON Schema, and stores the structured output as `ai_insights`.

### Step-by-step

1. **Copy the template.**

   ```bash
   cp analysis.d/TEMPLATE.yaml analysis.d/my_new_analysis.yaml
   ```

   Set `name` to match the filename (without `.yaml`), give it a `display_name`, and leave `enabled: false` while developing.

2. **Configure the schedule.** Use `interval` (e.g. `120s`, `5m`) for fixed cadence or `cron` for time-of-day control. Match the interval to the underlying data — there's no point analyzing every 30s if the source updates every 15 minutes.

3. **Choose your data path.** Community Edition supports layer-based queries (`data.layers: [...]`) with optional CEL filters. SQL-based analyses that use PostGIS are Platform-only and will not run on the Community Edition's SQLite backend. See [`analysis.d/README.md`](./analysis.d/README.md) for the difference and [`developer-documentation/content/analysis/data-queries.md`](./developer-documentation/content/analysis/data-queries.md) for the schema.

4. **Author the prompt.** Use Go `text/template` syntax. Iterate over `{{range .Records}}` to inject the data, define each severity tier in concrete terms, and end with the schema reference:

   ```text
   Respond with JSON matching this schema:
   {{.OutputSchema}}
   ```

   The engine injects the JSON Schema string at render time. See the prompt design guide in [`analysis.d/README.md`](./analysis.d/README.md) and the worked examples in [`developer-documentation/content/analysis/examples.md`](./developer-documentation/content/analysis/examples.md).

5. **Define the output schema** with JSON Schema — `required`, `enum`, `minimum`/`maximum` constraints help the LLM stay on the rails. Set `output.results_path` to the array field that holds per-entity items so each one is stored as a separate insight.

6. **Configure dedup** to suppress re-analysis of unchanged data. Pick `key_fields` that uniquely identify the same event (e.g. `external_id`) and set `window` to at least twice your `schedule.interval`.

7. **Make sure AI is on.** In `respondent.yaml`:

   ```yaml
   ai:
     enabled: true

   llm:
     provider: "openai"   # or anthropic, xai, gemini, zai, ollama, lmstudio
     openai:
       model: "gpt-4o"
       max_tokens: 2048
   ```

   Set the matching API key in `.env` (e.g. `RESPONDENT_LLM_OPENAI_API_KEY`).

8. **Update the catalog** in [`analysis.d/README.md`](./analysis.d/README.md).

### Useful references

- [`analysis.d/TEMPLATE.yaml`](./analysis.d/TEMPLATE.yaml)
- [`analysis.d/README.md`](./analysis.d/README.md) — full author guide, prompt design tips, dedup configuration
- [`developer-documentation/content/analysis/`](./developer-documentation/content/analysis/):
  - `data-queries.md` — layer queries and SQL paths
  - `ai-operations.md` — prompts, schemas, output config
  - `examples.md` — annotated real analyses

---

## Code Contributions

### Backend (Go)

The backend follows hexagonal architecture. See `CLAUDE.md` for the full layer breakdown.

**Adding a new feature:**
1. Define types and repository interface in `internal/domain/`.
2. Implement the SQLite repository in `internal/infra/sqlite/` (hand-written SQL).
3. Create an app service in `internal/app/<service>/` with constructor injection.
4. Add a proto RPC in `api/proto/` → `make proto` → add a gRPC-gateway handler in `internal/transport/grpc/`.

### Frontend (React)

The frontend lives in `frontend/apps/earth/` with shared code in `frontend/packages/core/`. Use TanStack Query for server state and Zustand for UI state. See `CLAUDE.md` for conventions.

### Proto changes

When modifying `.proto` files, always regenerate:

```bash
make proto && go build ./...
```

Never hand-edit files under `gen/` or `frontend/packages/core/src/api/generated/`.

---

## Testing

```bash
# Run all Go tests + frontend Vitest
make all-test

# Run Playwright E2E suite (boots isolated server on :8091)
make e2e
```

For YAML-only changes, verify by restarting the container and checking logs:

```bash
docker compose restart
docker compose logs -f
```

Things to check before opening a PR:

- Sources load cleanly — no `failed to compile CEL expression` or schema validation errors at startup.
- After the first poll, ingest activity appears in logs (record counts, no parser errors).
- Entities render on the globe with the expected icon, color, and label.
- For analyses: the LLM is invoked, the response validates against your `output_schema`, and insights are stored.

---

## Linting and Formatting

```bash
# Format Go + frontend code
make all-format

# Lint Go + lint & typecheck frontend
make all-lint
```

Run both before submitting a PR. The full validation pipeline:

```bash
make all-format && make all-lint && make all-test && make e2e
```

---

## Documentation Changes

The developer documentation in `developer-documentation/` is a Hugo site published at <https://respondent-docs.alevsk.dev>.

```bash
# Local dev server with live reload
make docs-dev
# Then open http://localhost:1313
```

To add a page, create a Markdown file under `developer-documentation/content/<section>/` with front matter copied from a neighbouring file. The site uses custom shortcodes (`{{< callout >}}`, `{{< field >}}`, `{{< code-tabs >}}`, `{{< cel-example >}}`) defined in `developer-documentation/layouts/`.

---

## Submitting a Pull Request

### Branch and commit conventions

- Branch from `main`. Suggested naming: `source/<source-name>`, `analysis/<analysis-name>`, `feat/<description>`, `fix/<description>`, `docs/<topic>`.
- Keep commits focused. One source or one analysis per PR is ideal; bundle pure refactors separately.
- Write descriptive commit messages — first line under 72 chars, body explaining the why if it's not obvious from the diff.

### PR description checklist

Please confirm in your PR description:

- [ ] I ran `make all-format && make all-lint && make all-test && make e2e` and all passed.
- [ ] My YAML loads without schema or CEL errors (if applicable).
- [ ] For sources: entities render on the globe and the detail panel displays the expected fields.
- [ ] For analyses: the analysis runs on its schedule and produces insights matching the declared `output_schema`.
- [ ] I updated the catalog table in [`sources.d/README.md`](./sources.d/README.md) or [`analysis.d/README.md`](./analysis.d/README.md) (if applicable).
- [ ] If the source/analysis needs a new env var, I added a commented placeholder to [`.env.example`](./.env.example).
- [ ] I did not commit secrets or my real `.env` file.

### Things that will get a PR sent back

- A new source whose data overlaps an existing one without a clear rationale (we prefer one canonical source per type of data).
- Sources or analyses with hardcoded credentials or API keys.
- Analyses that depend on PostGIS or PostgreSQL-only SQL — Community Edition uses SQLite.
- Code changes that break the hexagonal layer dependency rule.
- Unformatted or unlinted code.

---

## Getting Help

- **Bug reports and feature requests**: <https://github.com/alevsk/respondent-community/issues>
- **Documentation**: <https://respondent-docs.alevsk.dev> or the local [`developer-documentation/`](./developer-documentation/)
- **Catalog questions**: read [`sources.d/README.md`](./sources.d/README.md) and [`analysis.d/README.md`](./analysis.d/README.md) — they describe every active definition and the conventions around them.

When filing an issue, please include:

- The Respondent version (`./bin/community --version` or `docker compose images`)
- A minimal reproduction (the diff, the log lines you see)
- Whether AI is enabled and which LLM provider you're using (if relevant)
