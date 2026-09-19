# AI Enrichment Validation Runbook

This runbook is the **live, manual** counterpart to the automated regression
test `TestIntegration_InprocBus_EndToEnd`
(`internal/ai/enrichment/integration_test.go`). The automated test proves the
enrichment pipeline works end-to-end over the real in-process bus with a stub
LLM. This runbook proves the same path works against a **real LLM provider**
(LM Studio / qwen3, or Z.AI), and gives the authoritative SQLite assertions you
run to confirm an enrichment job actually landed in the database.

> **Why this exists.** Round-3 Task-1 fixed a silent-drop bug: the enrichment
> publisher emits `respondent.ai.enrich.<source>` while the worker subscribes the
> wildcard filter `respondent.ai.enrich.>`. A bus that matched subjects exactly
> dropped every job — the feeder logged "published enrichment jobs" while
> `ai_enrichment_log` stayed empty. The negative-control section below reproduces
> exactly that symptom so you can recognize a regression.

---

## 1. Prerequisites

- A built `community` binary (`make build`) or `go run ./cmd/community`.
- `sqlite3` CLI for the assertion queries.
- A running LLM endpoint (LM Studio for the primary path; Z.AI for the variant).
- The SQLite database path from `respondent.yaml` → `database.path`
  (default in this repo: **`./respondent-jc.db`**). Substitute your actual path
  in every `sqlite3` command below.

---

## 2. Primary path — LM Studio (qwen3)

### 2.1 Start LM Studio

1. Launch LM Studio and load the model **`qwen/qwen3.6-35b-a3b`** (any qwen3
   chat model works; this is the configured default).
2. Start the local server (default `http://localhost:1234`).
3. Verify the model is being served:

   ```bash
   curl -s http://localhost:1234/v1/models | jq .
   ```

   You should see the loaded model id in the `data` array. If this returns an
   error or an empty list, the rest of the runbook will fail with LLM transport
   errors, not enrichment-pipeline errors.

### 2.2 Configure `respondent.yaml`

Enable AI and point the LLM section at LM Studio:

```yaml
ai:
  enabled: true                  # was false by default
  analysis_dir: "./analysis.d"
  workers:
    enrichment: 2
    analysis: 1

llm:
  provider: "lmstudio"
  lmstudio:
    base_url: "http://localhost:1234/v1"
    model: "qwen/qwen3.6-35b-a3b"   # optional; LM Studio serves the loaded model when blank
    max_tokens: 4096
```

### 2.3 Enable one analysis definition (fast feedback)

Pick one definition under `analysis.d/` (e.g.
`analysis.d/news_intelligence_digest.yaml`) and, for a quick test:

- set `enabled: true`,
- shorten `schedule.interval` to a small value (e.g. `"30s"`) so you do not wait
  the default 15 minutes,
- ensure at least one operation has `websocket_push: true` so you can also see
  live pushes in the frontend.

> Per-source **enrichment** (entity `ai_metadata` patching) is driven by the
> `sources.d/*.yaml` AI blocks; **analysis** insights are driven by
> `analysis.d/*.yaml`. Both write audit rows to `ai_enrichment_log` and produce
> observable output — enrichment patches `entities.ai_metadata`, analysis writes
> `ai_insights`.

### 2.4 Run and watch

```bash
go run ./cmd/community serve   # or ./community serve
```

Watch the logs for:

- `starting enrichment worker` with `subject=respondent.ai.enrich.>`,
- the feeder publishing jobs as sources ingest,
- `operation completed` lines from the worker.

---

## 3. Authoritative SQLite assertions

Run these against your DB path. **These are the source of truth** — logs can lie,
the database cannot.

```bash
# Recent analysis insights (analysis.d output). Empty = analysis not producing.
sqlite3 ./respondent-jc.db \
  'SELECT source_name, operation_name, attention, created_at
     FROM ai_insights ORDER BY created_at DESC LIMIT 5;'

# Enrichment audit trail. The KEY assertion: rows must reach a terminal status
# (completed / failed / skipped), not sit at "pending". A "completed" row with an
# empty error_message is a healthy enrichment.
sqlite3 ./respondent-jc.db \
  "SELECT operation_name, status, error_message
     FROM ai_enrichment_log ORDER BY rowid DESC LIMIT 5;"

# Entities whose ai_metadata was actually patched by an enrichment operation.
# Empty (only '{}') after jobs were published = the silent-drop symptom.
sqlite3 ./respondent-jc.db \
  "SELECT id, ai_metadata
     FROM entities WHERE ai_metadata IS NOT NULL AND ai_metadata != '{}' LIMIT 5;"
```

### What "green" looks like

- `ai_enrichment_log` has rows with `status = 'completed'` and empty
  `error_message`.
- `entities.ai_metadata` shows JSON beyond `{}` for enriched entities (the
  operation result nested under the operation name, e.g.
  `{"threat_assessment": {...}}`).
- `ai_insights` accrues rows for each enabled `analysis.d` definition.

### Useful follow-ups

```bash
# Status histogram — fastest way to spot stuck "pending" rows.
sqlite3 ./respondent-jc.db \
  "SELECT status, COUNT(*) FROM ai_enrichment_log GROUP BY status;"

# Inspect one completed row's provider/model/tokens.
sqlite3 ./respondent-jc.db \
  "SELECT operation_name, provider, model, prompt_tokens, completion_tokens, latency_ms
     FROM ai_enrichment_log WHERE status='completed' ORDER BY rowid DESC LIMIT 3;"
```

---

## 4. Variant — Z.AI (GLM) provider

To validate the second supported provider, switch the LLM section to `zai` and
supply the API key via environment variable (config keys map to `RESPONDENT_`-
prefixed env vars: `llm.zai.api_key` → `RESPONDENT_LLM_ZAI_API_KEY`).

```yaml
llm:
  provider: "zai"
  zai:
    base_url: "https://api.z.ai/api/paas/v4"   # default
    model: "glm-4.5-flash"                       # default
    max_tokens: 1024
```

```bash
export RESPONDENT_LLM_ZAI_API_KEY="YOUR_ZAI_API_KEY"
go run ./cmd/community serve
```

Then re-run the **same** SQLite assertions from Section 3. The expected outcome
is identical: `completed` rows in `ai_enrichment_log`, populated
`entities.ai_metadata`, fresh `ai_insights`. Only the `provider` column in
`ai_enrichment_log` changes (`zai` instead of `lmstudio`).

---

## 5. Negative control (recognizing the silent-drop regression)

This is the failure mode Round-3 Task-1 fixed. Use it to confirm what a broken
bus looks like, and to sanity-check that a "no enrichment" report is genuinely a
delivery problem and not a misconfiguration.

**Pre-fix symptom (do NOT ship this state):**

- Feeder logs show enrichment jobs being **published** (e.g. a
  "published enrichment jobs" line, with a non-zero count).
- The worker logs `starting enrichment worker` but **never** logs
  `operation completed`.
- `ai_enrichment_log` stays **empty** (or unchanged):

  ```bash
  sqlite3 ./respondent-jc.db \
    "SELECT COUNT(*) FROM ai_enrichment_log;"   # returns 0 despite published jobs
  ```

- `entities.ai_metadata` stays `{}` for all entities.

The root cause was subject-matching: the worker subscribes
`respondent.ai.enrich.>` (a wildcard filter) but the publisher emits
`respondent.ai.enrich.<source>`. With Task-1's wildcard-aware
`inproc.SubjectMatches`, the wildcard subscription matches the concrete subject
and jobs are delivered. The automated guard for this is
`TestIntegration_InprocBus_EndToEnd`, which fails ("worker handler never ran")
against an exact-match bus and passes against the fixed one.

If you see published-but-empty-`ai_enrichment_log` again, first confirm the bus
subject match (`internal/infra/inproc/bus.go`) and the worker's filter
(`Subject + ".>"` in `internal/ai/enrichment/worker.go`) before suspecting the
LLM provider.
