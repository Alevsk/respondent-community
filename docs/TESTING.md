# Testing & QA

This document describes the regression net for the community edition. Run the
aggregate targets before every commit.

---

## Quality gates

| Command | What it does |
|---------|--------------|
| `make all-format` | `gofmt` + `goimports` over Go sources (excludes `gen/`, `vendor/`) and Prettier `--write` over `frontend/apps/` + `frontend/packages/`. |
| `make all-lint` | `golangci-lint run ./...` (excludes `gen/`) + ESLint over frontend + `tsc --noEmit` type-check on the earth app. |
| `make all-test` | `go test -race -cover` (excludes `gen/`) + Vitest single-run over all frontend packages. |
| `make ci-fmt` | Check-only Go formatting gate — fails if any Go file is unformatted (no writes). |
| `make ci-format` | Check-only frontend formatting gate — Prettier `--check`, fails if any file is unformatted. |

Individual sub-targets are also available: `make fmt`, `make lint`, `make test`
(Go only) and `make frontend-format`, `make frontend-lint`, `make frontend-typecheck`,
`make frontend-test` (frontend only).

---

## End-to-end tests (Playwright)

Specs live in `frontend/apps/earth/e2e/tests/`:

| Spec | What it covers |
|------|---------------|
| `app-load` | Shell renders, globe canvas is visible, notification bell appears. |
| `panels` | Settings, navigation, notification, and search panels open and close. |
| `layers` | Layers panel opens, shows content, and closes. |
| `entity-data` | Entities are present on the globe and search returns results. **Skips automatically when the feeder has no live data**, so the gate stays green offline. |

### `make e2e` — isolated full run

This is the standard gate. It:

1. Builds the earth frontend with `VITE_API_URL=""` so all API calls use
   relative paths (`e2e-frontend-build` sub-target). This ensures the embedded
   Go server is hit regardless of port.
2. Builds `bin/community` with the embedded frontend (`make build`).
3. Wipes and recreates `.e2e/`, then starts `bin/community serve` on **port 8091**
   with a throwaway SQLite database at `.e2e/respondent-e2e.db`. The isolation
   is achieved via env overrides (`RESPONDENT_DATABASE_PATH` and
   `RESPONDENT_SERVER_PORT`; see [Config env overrides](#config-env-overrides)
   below).
4. Waits for `/readyz` to return 200.
5. Runs the Playwright specs (`make e2e-earth-test` with `E2E_BASE_URL` set to
   `http://localhost:8091`).
6. Traps `EXIT`/`INT`/`TERM` to kill the server and delete `.e2e/`.

Because it runs on port 8091 against its own throwaway DB, `make e2e` is safe
to run while a dev server is already up on port 8090 via `make run`.

### `make e2e-earth-test` — specs against a running server

Runs the Playwright specs against an already-running server. Set
`E2E_BASE_URL` to target a different host/port (default: `http://localhost:8090`).
Useful for running specs against a dev server started with `make run`.

### Viewing the HTML report

```bash
cd frontend/apps/earth && npm run e2e:report
```

---

## Config env overrides

All `respondent.yaml` keys can be overridden by environment variables using
the pattern `RESPONDENT_<KEY>` where dots in the YAML key are replaced with
underscores. Examples:

| YAML key | Env override |
|----------|-------------|
| `database.path` | `RESPONDENT_DATABASE_PATH` |
| `server.port` | `RESPONDENT_SERVER_PORT` |

This is handled by `internal/config.InitViper` via Viper's `SetEnvPrefix` +
`AutomaticEnv`. The e2e isolation in `make e2e` relies on exactly this
mechanism to spin up a fresh server on a different port with a throwaway DB
without touching `respondent.yaml`.

---

## Perf smoke (report-only, not a gate)

`make perf-smoke` runs `tools/perf-smoke.mjs` in headless Chromium against
two targets sequentially:

- `http://localhost:8090` — local dev server
- `https://respondent.alevsk.dev` — production

The script opens each URL, waits for the globe canvas, attempts to enable all
available layers via the layers panel, waits 3 seconds for the entity stream,
then samples FPS via `requestAnimationFrame` for 15 seconds and collects
`longtask` PerformanceObserver entries.

Output is structured JSON to stdout; progress logs go to stderr:

```json
{
  "url": "http://localhost:8090",
  "durationSec": 15,
  "layersEnabled": 12,
  "fps": { "avg": 58.3, "min": 31.2, "max": 60.1, "samples": 871 },
  "longTasks": { "count": 4, "totalMs": 312 }
}
```

The process exits 0 regardless of fps values. This is intentionally not a CI
gate — use it as a diagnostic baseline input for Round 2 performance work.

### Known caveats

**Prod: layer-enabling is best-effort / may report `layersEnabled: 0`.**
The production build strips `data-testid` attributes, so the toolbar button
(`toolbar-btn-layers`) and panel (`panel-layers`) lookups fail silently. The
script catches the error and continues; the FPS measurement still runs, but
against the default globe view with no layers toggled.

**Prod: FPS reflects an idle globe.**
Headless Chromium rejects the production WSS certificate, so the WebSocket
connection to the live feeder does not establish. Prod numbers represent a
static globe with no streaming entities.

**Local: requires a running server.**
`make perf-smoke` does not start a server. Start one first with `make run`
(or `make build && ./bin/community serve`), then run `make perf-smoke`.

The local-vs-prod gap exposed by this tool is the primary input for the Round 2
performance harvest.

---

### Phase 1 baseline & after-trace

This section records the deterministic before/after perf trace for Round 2 Phase 1 globe
performance work (branch `round2-phase1-globe-perf`).

#### How to reproduce

```bash
# 1. Build
make e2e-frontend-build build

# 2. Start isolated server on port 8090
mkdir -p .e2e
RESPONDENT_DATABASE_PATH=.e2e/perf.db RESPONDENT_SERVER_PORT=8090 \
  ./bin/community serve --config respondent.yaml &

# 3. Wait for readyz
for i in $(seq 1 30); do curl -sf http://localhost:8090/readyz && break; sleep 1; done

# 4. Run harness (repeat 3× for median)
node tools/perf-smoke.mjs http://localhost:8090 15

# 5. Cleanup
kill %1; rm -f .e2e/perf.db .e2e/perf.db-shm .e2e/perf.db-wal
```

The pre-Phase-1 baseline is stored in `tools/perf-baseline.json`.

#### Phase 1 results

All three after-runs, measured on `round2-phase1-globe-perf` (darwin arm64, CGO_ENABLED=1,
e2e-frontend-build, scripted 360° pan, 15 s duration):

| Run | layersEnabled | fps.avg | fps.min | fps.max | longTasks.count | longTasks.totalMs |
|-----|--------------|---------|---------|---------|-----------------|-------------------|
| Run 1 | 17 | 110.6 | 3.9 | 285.7 | 98 | 5920 |
| Run 2 | 20 | 110.6 | 4.0 | 357.1 | 104 | 6340 |
| Run 3 | 17 | 110.4 | 3.8 | 188.7 | 115 | 6789 |
| **Baseline** | **16** | **115.3** | **4** | **416.7** | **69** | **4043** |

#### Gate verdict: FAIL (DONE_WITH_CONCERNS)

Gate conditions (vs baseline — fps.min=4, longTasks.totalMs=4043):
- `fps.min > 4`: **NOT MET** — after-run fps.min values are 3.9 / 4.0 / 3.8 (effectively unchanged)
- `longTasks.totalMs < 4043`: **NOT MET** — all three runs show higher long-task totals (5920 / 6340 / 6789)

**Analysis:** The after-runs had `layersEnabled` of 17–20 versus 16 in the baseline. The test
environment streams live data, and more layers were enabled (and entities streaming in) by the time
each after-run was measured. This inflates the long-task budget because more WebSocket traffic and
more store updates occur during the sampling window. The Phase 1 changes (rAF coalescing for bulk
snapshots) are directionally correct but the gate numbers are not conclusive due to this environment
variance. A controlled re-run with a fixed, seeded dataset is recommended to get a clean before/after
signal.
