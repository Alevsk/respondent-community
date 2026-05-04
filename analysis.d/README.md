# Declarative Analysis Definitions

YAML analysis definitions for the Respondent analyzer. Each file defines a complete AI analysis pipeline — schedule, data query (layer-based or SQLite SQL), LLM prompt, output schema, and storage configuration — without any Go code.

The analyzer loads all enabled definitions from this directory on startup, schedules them independently, and stores structured insights in the `ai_insights` table.

---

## Analysis Catalog

### Active Analyses

| File | Display Name | Layers Consumed | Schedule | Status |
|---|---|---|---|---|
| `environmental_cascade.yaml` | Environmental Cascade Detection | `fires_active`, `air_quality` | 180s | active |
| `geopolitical_hotspot_index.yaml` | Geopolitical Hotspot Index | `conflict_events`, `flights_military`, `disaster_alerts`, `internet_outages` (SQL) | 300s | active |
| `infrastructure_threat_assessment.yaml` | Critical Infrastructure Threat Assessment | `earthquakes`, `disaster_alerts`, `volcanoes`, `weather_alerts`, `subsea_cables`, `internet_infrastructure` | 180s | active |
| `lightning_fire_prediction.yaml` | Lightning-Fire Prediction | `fires_active`, `lightning` | 180s | active |
| `maritime_cable_threat.yaml` | Maritime Cable Threat Detection | `ships`, `subsea_cables` (SQL) | 120s | active |
| `military_conflict_proximity.yaml` | Military Activity Near Conflict Zones | `flights_military`, `conflict_events` | 120s | active |
| `news_intelligence_digest.yaml` | News Intelligence Digest | `news_articles` | 900s | active |
| `radiation_anomaly_correlation.yaml` | Radiation Anomaly Correlation | `radiation`, `radiation_us`, `earthquakes`, `conflict_events`, `volcanoes` (SQL) | 300s | active |
| `severe_weather_aviation.yaml` | Severe Weather Aviation Impact | `flights_commercial`, `weather_alerts`, `aviation_weather` (SQL) | 120s | active |
| `source_quality_audit.yaml` | Source Data Quality Audit | all layers | 3600s | active |
| `volcanic_aviation_hazard.yaml` | Volcanic Aviation Hazard | `flights_commercial`, `volcanoes` (SQL) | 180s | active |

### Placeholder Analyses

Definitions exist but contain no implemented AI operations. They document the intended design and list the data layers and SQL approach for the eventual implementation.

| File | Display Name | Intended Layers | Status |
|---|---|---|---|
| `border_tension_index.yaml` | Border Tension Index | `border_crossings`, `conflict_events`, `flights_military`, `internet_outages` | placeholder |
| `flight_anomaly_detection.yaml` | Flight Anomaly Detection | `flights_commercial`, `flights_military` | placeholder — needs time-series SQL redesign |
| `ham_radio_emergency.yaml` | Ham Radio Emergency Detection | `radio_aprs`, `disaster_alerts`, `internet_outages` | placeholder — insufficient APRS data |
| `ocean_buoy_anomaly.yaml` | Ocean Buoy Anomaly Detection | `ocean_buoys`, `earthquakes`, `disaster_alerts` | placeholder — metadata coverage incomplete |
| `satellite_conjunction.yaml` | Satellite Conjunction / Space Domain Awareness | `satellites`, `conflict_events` | placeholder — requires orbital computation |
| `space_launch_awareness.yaml` | Space Launch Awareness | `rocket_launches`, `flights_commercial`, `flights_military`, `satellites` | placeholder |

---

## Adding a New Analysis

1. Copy `TEMPLATE.yaml` and rename it to match your analysis name (lowercase, underscores only):

   ```
   cp analysis.d/TEMPLATE.yaml analysis.d/my_new_analysis.yaml
   ```

2. Set the required top-level fields: `name` (must match the filename without `.yaml`), `display_name`, and `schema_version: 1`. Leave `enabled: false` while developing.

3. Configure `schedule`. Choose `interval` for fixed cadence or `cron` for time-of-day control. See the schedule guidelines in `TEMPLATE.yaml`.

4. Choose your data path (see [Two Data Paths](#two-data-paths) below).

5. Write the `ai.operations` block:
   - Author a `prompt` using Go `text/template` syntax. See [Prompt Design Guide](#prompt-design-guide).
   - Define `output_schema` with JSON Schema. Use `required`, `enum`, and `minimum`/`maximum` to constrain the LLM output.
   - Configure `output.insight_type` with a stable, lowercase-underscore label.
   - Set `output.results_path` to the array field in your schema that holds per-entity items.

6. Optionally configure `data.dedup` to suppress re-analysis when data has not meaningfully changed. See [Dedup Configuration](#dedup-configuration).

7. Validate by setting `enabled: true`, restarting the container (`docker compose restart`), and watching the logs for the analyzer scheduler to pick up the definition. Schema, CEL, and SQL errors are surfaced at startup; runtime errors appear on the first scheduled tick.

   ```bash
   docker compose logs -f respondent
   ```

8. Once it runs cleanly and produces the expected insights, leave `enabled: true` and commit.

---

## Two Data Paths

### Path A — Layer-based

Set `data.layers` to a list of `layer_type` values. The engine calls `GetLatestForLayer()` for each layer and merges the results. An optional CEL `data.filter` expression can narrow the record set before the AI phase.

```yaml
data:
  layers: [fires_active, air_quality]
  lookback: "6h"
  min_records: 1
  max_records: 30
  filter: 'double(observation.metadata["frp"]) > 10.0'
```

Use Path A when:
- The analysis operates on one or two layers with manageable data volume.
- Proximity or join logic can live in the AI prompt rather than SQL.
- You need `data.layers: []` to query all layers at once (e.g., audit analyses).

### Path B — SQL-based

Set `data.sql` to a raw SQLite query. The engine executes it directly against the database. The query is validated as read-only at load time — only `SELECT` statements are accepted.

```yaml
data:
  lookback: "6h"
  min_records: 1
  max_records: 30
  sql: |
    SELECT e.name, 'my_layer' AS layer_type,
           o.lat, o.lon, 0.0 AS altitude_m, o.ts
    FROM entities e
    JOIN (
      SELECT entity_id, lat, lon, ts
      FROM observations
      WHERE (entity_id, ts) IN (
        SELECT entity_id, MAX(ts) FROM observations GROUP BY entity_id
      )
    ) o ON o.entity_id = e.id
    WHERE e.layer_type = 'my_layer'
    LIMIT 30
```

Use Path B when:
- You need to aggregate, compute approximate distances, or filter by proximity in the database rather than in the LLM prompt.
- The analysis joins across multiple layers and the join logic is more naturally expressed in SQL than in CEL.
- Data volume or complexity would make layer-based fetching impractical.

SQLite does not ship with PostGIS-style geography functions. For proximity work, use the bounding-box + haversine pattern shown in [SQL Reference](#sql-reference) below.

---

## SQL Reference

### Database Schema

```sql
entities(
  id TEXT PK, external_id TEXT, layer_type TEXT, name TEXT,
  metadata TEXT,        -- JSON, accessed via json_extract()
  ai_metadata TEXT,     -- JSON
  source TEXT, created_at TEXT  -- ISO-8601 timestamps stored as TEXT
)
observations(
  id TEXT PK, entity_id TEXT FK, ts TEXT,
  event_time TEXT, event_end TEXT,
  lat REAL, lon REAL,
  altitude_m REAL, velocity TEXT,   -- JSON
  metadata TEXT, ai_metadata TEXT,  -- JSON
  source_type TEXT, content_hash TEXT, source TEXT, created_at TEXT
)
ai_insights(
  id TEXT PK, insight_type TEXT, source_name TEXT, operation_name TEXT,
  layer_type TEXT, attention TEXT, attention_rank INTEGER, dedup_key TEXT,
  result TEXT,           -- JSON
  expires_at TEXT, created_at TEXT
)
```

SQLite stores timestamps as ISO-8601 text. Use `datetime('now', '-6 hours')` for relative time filters and compare with `o.ts > datetime('now', '-6 hours')`.

### Required SELECT Columns

The engine maps these fixed column names to `AnalysisRecord` fields. All other columns become entries in the `.Metadata` map (accessed in prompts with `{{index .Metadata "column_name"}}`).

| Column | Type | Maps to |
|---|---|---|
| `name` | TEXT | `.EntityName` |
| `layer_type` | TEXT | `.LayerType` |
| `lat` | REAL | `.Lat` |
| `lon` | REAL | `.Lon` |
| `altitude_m` | REAL | `.Altitude` |
| `ts` | TEXT (ISO-8601) | `.Timestamp` |
| `external_id` | TEXT (optional) | `.ExternalID` |
| `entity_id` | TEXT (optional) | used for insight ref resolution |

Cast non-text metadata columns to text in the `SELECT` so they render cleanly in prompts: `CAST(my_count AS TEXT) AS my_count`.

### Pattern 1 — Latest Observation per Entity

Standard pattern for most analyses. Gets the most recent observation for each entity using a correlated subquery.

```sql
SELECT e.id AS entity_id, e.name, e.external_id,
       'my_layer' AS layer_type,
       o.lat, o.lon, 0.0 AS altitude_m, o.ts
FROM entities e
JOIN observations o ON o.id = (
  SELECT id FROM observations
  WHERE entity_id = e.id
  ORDER BY ts DESC LIMIT 1
)
WHERE e.layer_type = 'my_layer'
  AND o.ts > datetime('now', '-6 hours')
```

### Pattern 2 — Observation-first with GROUP BY

More efficient when the layer has many entities but only a subset have recent observations. Avoids a full `entities` table scan.

```sql
SELECT
  o.entity_id, e.name, e.external_id,
  'my_layer' AS layer_type,
  o.lat, o.lon, o.altitude_m, o.ts
FROM observations o
JOIN entities e ON e.id = o.entity_id AND e.layer_type = 'my_layer'
WHERE o.ts > datetime('now', '-6 hours')
  AND (o.entity_id, o.ts) IN (
    SELECT entity_id, MAX(ts) FROM observations
    WHERE ts > datetime('now', '-6 hours')
    GROUP BY entity_id
  )
```

### Pattern 3 — Proximity Check (Bounding Box + Haversine)

SQLite has no built-in geography type. Filter with a cheap lat/lon bounding box first, then refine with a haversine calculation.

```sql
-- Bounding-box pre-filter using ~111km per degree of latitude.
-- A 100km radius corresponds to about 0.9 degrees of latitude.
WHERE b.lat BETWEEN a.lat - 0.9 AND a.lat + 0.9
  AND b.lon BETWEEN a.lon - (0.9 / MAX(COS(RADIANS(a.lat)), 0.01))
                 AND a.lon + (0.9 / MAX(COS(RADIANS(a.lat)), 0.01))
```

For an exact within-distance check, add a haversine expression — see Pattern 4.

### Pattern 4 — Haversine Distance in Kilometers

Great-circle distance between two points in kilometers. Use this both as a filter (`< 100`) and as a returned column.

```sql
6371.0 * 2.0 * ASIN(SQRT(
    POW(SIN(RADIANS(b.lat - a.lat) / 2.0), 2)
  + COS(RADIANS(a.lat)) * COS(RADIANS(b.lat))
  * POW(SIN(RADIANS(b.lon - a.lon) / 2.0), 2)
)) AS distance_km
```

`6371.0` is the mean Earth radius in km; replace with `6371000.0` for meters.

### Pattern 5 — Multi-CTE Cross-Layer Signal Fusion

The standard architecture for composite analyses. Each CTE isolates one signal layer; the final `SELECT` merges them via `LEFT JOIN`. See `geopolitical_hotspot_index.yaml` for a full four-layer example.

```sql
WITH anchor_entities AS (
  -- CTE 1: anchor layer with latest observation per entity
  SELECT e.id, e.name, e.external_id, o.lat, o.lon, o.ts
  FROM entities e
  JOIN observations o ON o.id = (
    SELECT id FROM observations
    WHERE entity_id = e.id ORDER BY ts DESC LIMIT 1
  )
  WHERE e.layer_type = 'anchor_layer'
    AND o.ts > datetime('now', '-6 hours')
),
signal_layer AS (
  -- CTE 2: count nearby signal entities per anchor (200km radius)
  SELECT
    a.name AS anchor_name,
    COUNT(DISTINCT s.id) AS signal_count,
    GROUP_CONCAT(DISTINCT json_extract(s.metadata, '$.type')) AS signal_types
  FROM anchor_entities a
  JOIN entities s ON s.layer_type = 'signal_layer'
  JOIN observations o_s ON o_s.id = (
    SELECT id FROM observations
    WHERE entity_id = s.id ORDER BY ts DESC LIMIT 1
  )
  WHERE o_s.ts > datetime('now', '-6 hours')
    AND o_s.lat BETWEEN a.lat - 1.8 AND a.lat + 1.8
    AND o_s.lon BETWEEN a.lon - (1.8 / MAX(COS(RADIANS(a.lat)), 0.01))
                     AND a.lon + (1.8 / MAX(COS(RADIANS(a.lat)), 0.01))
    AND 6371.0 * 2.0 * ASIN(SQRT(
          POW(SIN(RADIANS(o_s.lat - a.lat) / 2.0), 2)
        + COS(RADIANS(a.lat)) * COS(RADIANS(o_s.lat))
        * POW(SIN(RADIANS(o_s.lon - a.lon) / 2.0), 2)
        )) < 200.0
  GROUP BY a.name
)
-- FINAL SELECT: LEFT JOIN preserves anchor rows with no signal
SELECT
  a.id AS entity_id, a.name, a.external_id,
  'anchor_layer' AS layer_type,
  a.lat, a.lon, 0.0 AS altitude_m, a.ts,
  CAST(COALESCE(sl.signal_count, 0) AS TEXT) AS signal_count,
  COALESCE(sl.signal_types, 'none')          AS signal_types
FROM anchor_entities a
LEFT JOIN signal_layer sl ON sl.anchor_name = a.name
WHERE COALESCE(sl.signal_count, 0) > 0
ORDER BY sl.signal_count DESC
LIMIT 30
```

### Useful SQL Functions

| Function | Description |
|---|---|
| `datetime('now', '-6 hours')` | Current UTC timestamp shifted by a relative offset. |
| `julianday(a) - julianday(b)` | Difference between two timestamps in days (multiply for hours/minutes). |
| `RADIANS(deg)`, `SIN`, `COS`, `ASIN`, `SQRT`, `POW` | Math primitives for the haversine pattern. |
| `GROUP_CONCAT(DISTINCT expr, ', ')` | Comma-separated aggregate of distinct values. |
| `COALESCE(value, default)` | Substitutes `default` when `value` is NULL. |
| `json_extract(metadata, '$.key')` | Extract a JSON field from the `metadata` text column. |
| `CAST(expr AS INTEGER)` / `CAST(expr AS TEXT)` | Type conversion. |

---

## Prompt Design Guide

Well-constructed prompts consistently produce higher-quality, more reproducible insights. Follow these principles when authoring a new prompt.

**State the analyst role explicitly.** Open with a one-sentence role statement that describes the domain and decision context. "You are a senior maritime security analyst..." outperforms generic openers.

**Present data in a structured block.** Use consistent indentation inside `{{range .Records}}` so each record reads like a labeled table. The LLM parses structured input more reliably than narrative prose.

**Provide calibration anchors.** Define what each severity tier or classification means in quantitative terms. Include real-world reference points when possible ("100+ fatalities = crisis, cf. Sudan").

**Use CRITICAL callouts for hard constraints.** When a classification must never be applied without a specific signal, state this explicitly. Unconstrained classifications drift toward the LLM's priors rather than your data.

**Ask for explanations, not just scores.** A `reasoning` or `assessment` field forces the LLM to ground its output in the presented data. This also makes insights useful to human readers and easier to audit for correctness.

**Close with the schema reference.** Always end the prompt body with:

```
Respond with JSON matching this schema:
{{.OutputSchema}}
```

The engine injects the JSON Schema string at render time. This line is required for structured output validation.

**Budget tokens carefully.** Static prompt text (role + instructions + calibration) should stay under ~1500 tokens. Budget the rest for `{{range .Records}}` expansion and the LLM response. Use `max_tokens` to set a hard ceiling on the response.

---

## Output Configuration

### Insight Storage

Each analysis operation stores results to the `ai_insights` table when `output.store_insights: true`. Set `output.results_path` to the name of the array field in the LLM response — the engine stores one `ai_insight` row per array item, enabling per-entity querying.

If `results_path` resolves to an empty array, the engine falls back to storing the entire LLM response as a single insight.

### insight_type

The `output.insight_type` label is stored in `ai_insights.insight_type` and is used by the frontend and API to query and filter insights. Treat it as a stable identifier — changing it after deployment breaks existing consumers. Use descriptive, lowercase-underscore values: `hotspot_index`, `cable_threat`, `weather_impact`.

### WebSocket Push

When `output.websocket_push: true`, each stored insight is immediately pushed to connected WebSocket clients as an `ai_insight` message. This enables real-time dashboard updates without client-side polling.

### Retention

The `output.retention` field controls how long insights are kept before the hourly cleanup goroutine deletes them. Match retention to the pace of the underlying data:

| Analysis frequency | Typical retention |
|---|---|
| Every 60-120s | `1h` to `6h` |
| Every 300s (5m) | `24h` to `168h` |
| Hourly | `168h` (7 days) |
| Daily digest | `336h` (14 days) |

### Entity References

Include `entity_external_id` (or a field listed in `output.ref_fields`) in the LLM output schema to create links between insights and source entities in `ai_insight_refs`. This allows the frontend to navigate from an insight back to the entity that produced it.

---

## Dedup Configuration

The `data.dedup` block prevents the same insight from being emitted on every tick when the underlying data has not meaningfully changed.

```yaml
data:
  dedup:
    window: "6h"           # Suppress re-analysis for this duration
    key_fields: ["external_id"]   # Fields that together identify the same event
```

The engine computes a hash of the `key_fields` values for each record. If a matching insight (same `insight_type` + hash) was stored within the `window`, the AI phase is skipped for that record.

Choose `key_fields` that are stable and specific:
- `["external_id"]` — for named regions or entities with stable external IDs
- `["entity_id", "alert_type"]` — for typed events where the same entity can have different alert types
- `["region", "classification"]` — for composite scores where the tier change is the meaningful event

The `window` must be longer than `schedule.interval` to suppress any duplicates. A window equal to twice the interval is the minimum effective configuration.

---

## Reference

- `TEMPLATE.yaml` — annotated starter template covering every supported field
- CEL language: <https://github.com/google/cel-spec>
- JSON Schema: <https://json-schema.org/>
- SQLite SQL syntax: <https://www.sqlite.org/lang.html>
- SQLite JSON functions: <https://www.sqlite.org/json1.html>
