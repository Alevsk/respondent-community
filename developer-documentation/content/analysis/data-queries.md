---
title: "Data Queries"
description: "Specify which data feeds into your analysis — layer-based or SQL queries"
weight: 1
---

Every analysis definition has a `data` section that controls which records are fetched before the AI phase runs. You choose between two mutually exclusive query paths: **layer-based** (simple) or **SQL-based** (advanced). The data section also defines scheduling, record limits, and deduplication.

## Schedule

The `schedule` section controls when the analysis runs. You can use a fixed interval or a cron expression.

{{< field name="schedule.interval" type="duration" required="false" >}}
Fixed interval between analysis runs. Accepts Go duration strings: `"60s"`, `"300s"`, `"5m"`, `"1h"`. The engine runs the analysis once per interval.
{{< /field >}}

{{< field name="schedule.cron" type="string" required="false" >}}
Cron expression for schedule-based runs. Uses 6-field format: `"sec min hour dom month dow"`. Example: `"0 */5 * * * *"` runs every 5 minutes.
{{< /field >}}

{{< code-tabs >}}
{{< tab title="Interval" >}}
```yaml
schedule:
  interval: "300s"
```
{{< /tab >}}
{{< tab title="Cron" >}}
```yaml
schedule:
  cron: "0 */5 * * * *"
```
{{< /tab >}}
{{< /code-tabs >}}

{{< callout type="tip" title="Choosing a schedule" >}}
Use `interval` for most analyses -- it is simpler and ensures consistent spacing between runs. Use `cron` when you need alignment to clock times (e.g., "run at the top of every hour" or "run at midnight daily").
{{< /callout >}}

---

## Layer-based queries

Layer-based queries are the simplest data path. You list one or more layer types, and the engine fetches the latest entities from each layer using the internal `GetLatestForLayer()` API.

```yaml
data:
  layers: [earthquakes]
  lookback: "24h"
  min_records: 5
  max_records: 50
  filter: >
    double(entity.metadata.magnitude) >= 4.0
```

{{< field name="data.layers" type="string[]" required="true" >}}
Array of layer types to query. The engine fetches the latest entities for each layer and combines the results. Example: `[earthquakes]`, `[flights_military, conflict_events]`.
{{< /field >}}

{{< field name="data.lookback" type="duration" required="true" >}}
Time window for data freshness. Records with timestamps older than `now() - lookback` are discarded. Example: `"24h"`, `"6h"`, `"168h"`.
{{< /field >}}

{{< field name="data.filter" type="string" required="false" >}}
CEL expression to filter records after fetching. The expression receives `entity` and `observation` as maps; access entity metadata via `entity.metadata.<field>` and observation metadata via `observation.metadata.<field>`. Records where the expression evaluates to `false` are excluded.
{{< /field >}}

{{< field name="data.min_records" type="integer" required="false" >}}
Minimum number of records required to proceed to the AI phase. If fewer records are returned, the analysis tick is skipped and logged as "insufficient records". Set to `1` for analyses that should run even with a single data point.
{{< /field >}}

{{< field name="data.max_records" type="integer" required="false" >}}
Hard cap on records sent to the LLM prompt. Prevents token overflow. Set this to match your prompt's capacity -- 30 is a good default for detailed per-record analysis, 100 for summary-style prompts.
{{< /field >}}

{{< callout type="info" title="When to use layer-based queries" >}}
Layer-based queries are the right choice when you analyze data from one or two layers without needing spatial joins. They are simpler to write and the engine handles all the database logic. Use SQL-based queries only when you need cross-layer geographic correlation.
{{< /callout >}}

---

## SQL-based queries

SQL-based queries give you direct access to the database. The engine runs on read-only **SQLite** (the `modernc.org/sqlite` pure-Go driver) -- there is **no PostGIS**. Spatial proximity is computed with a registered `haversine_km()` SQLite function plus a lat/lon bounding-box prefilter. SQL is the right tool for cross-layer analysis where you correlate entities from different layers based on geographic proximity.

```yaml
data:
  lookback: "24h"
  min_records: 1
  max_records: 30
  sql: |
    WITH latest_quake AS (
      SELECT o.entity_id, o.lat, o.lon, o.altitude_m, o.ts,
             ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id AND e.layer_type = 'earthquakes'
      WHERE julianday(o.ts) > julianday('now','-24 hours')
    )
    SELECT
      e.id AS entity_id,
      e.name AS name,
      e.external_id AS external_id,
      'earthquakes' AS layer_type,
      lq.lat, lq.lon, lq.altitude_m,
      lq.ts,
      json_extract(e.metadata,'$.magnitude') AS magnitude
    FROM latest_quake lq
    JOIN entities e ON e.id = lq.entity_id
    WHERE lq.rn = 1
    ORDER BY CAST(json_extract(e.metadata,'$.magnitude') AS REAL) DESC
    LIMIT 30
```

{{< field name="data.sql" type="string" required="true" >}}
Raw SQL query executed via the read-only `ReadOnlyQueryExecutor`. The SQL is parsed by `pg_query_go` (the PostgreSQL parser) for validation but **executed on SQLite**, so write PostgreSQL-parseable SQL that uses only SQLite-compatible runtime features. No `INSERT`, `UPDATE`, `DELETE`, `DROP`, or other mutation/DDL statements are allowed -- only a single `SELECT` (a `WITH ... SELECT` CTE counts as one statement).
{{< /field >}}

{{< callout type="warning" title="SQL is read-only" >}}
The query is validated as read-only at **run time** (not at config-load time). Validation rejects mutations, DDL, multiple statements, `SELECT ... INTO`, locking clauses, `COPY`/`EXECUTE`/`CALL`, `SET`, and `EXPLAIN`. The executor also wraps every query with a hard **500-row cap** and a **~30-second timeout** so a runaway spatial join cannot hang an analysis tick.
{{< /callout >}}

{{< callout type="info" title="Parsed as PostgreSQL, executed on SQLite" >}}
The validator parses your SQL with the real PostgreSQL grammar, but the query actually runs on SQLite via `modernc.org/sqlite`. Use SQLite runtime functions: `json_extract()`, `julianday()`, `strftime()`, `group_concat()`, `max()`/`min()`, `CAST(... AS REAL|INTEGER|TEXT)`, and the registered `haversine_km()`. Do **not** use PostgreSQL-only runtime features like `::geography` casts, `ST_DWithin`, `NOW()`, `INTERVAL`, `->>`, `STRING_AGG`, or `GREATEST`/`LEAST` -- they parse but fail (or behave wrongly) at execution.
{{< /callout >}}

### Required output columns

Your SQL query should return these standard columns. The engine maps them by name to the `AnalysisRecord` struct used in prompt templates.

| Column | Type | Maps to |
|--------|------|---------|
| `entity_id` | text | `{{.EntityID}}` (also `{{index .Metadata "entity_id"}}`) |
| `external_id` | text | `{{.ExternalID}}` (also `{{index .Metadata "entity_external_id"}}`) |
| `layer_type` | text | `{{.LayerType}}` |
| `name` | text | `{{.EntityName}}` |
| `lat` | real | `{{.Lat}}` |
| `lon` | real | `{{.Lon}}` |
| `altitude_m` | real | `{{.Altitude}}` |
| `ts` | text (RFC 3339) | `{{.Timestamp}}` |

`entity_id`, `external_id`, and `layer_type` matter for cross-layer analyses: they let the engine link insights back to the correct source entity. Any additional columns beyond the standard set become entries in the `.Metadata` map, accessible in prompts via `{{index .Metadata "column_name"}}`. All metadata values are surfaced as strings.

{{< callout type="tip" title="Cast extra columns to text" >}}
The metadata map stores `map[string]string`. Cast all non-standard columns to text in your SQL with SQLite's `CAST`: `CAST(rs.fatalities AS TEXT) AS fatalities`. This avoids type-coercion surprises in prompt rendering.
{{< /callout >}}

### Latest-observation pattern (ROW_NUMBER window CTE)

The most common SQL pattern is fetching the latest observation for each entity. Entities can have multiple observations (in `append` recording mode), and you typically want only the most recent position. There is no `CROSS JOIN LATERAL` in SQLite -- use a `ROW_NUMBER() OVER (PARTITION BY entity_id ORDER BY ts DESC)` window CTE and join on `rn = 1`.

```sql
WITH latest_quake AS (
  SELECT o.entity_id, o.lat, o.lon, o.altitude_m, o.ts,
         ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
  FROM observations o
  JOIN entities e ON e.id = o.entity_id AND e.layer_type = 'earthquakes'
)
SELECT e.name, lq.lat, lq.lon, lq.altitude_m, lq.ts
FROM latest_quake lq
JOIN entities e ON e.id = lq.entity_id
WHERE lq.rn = 1
```

{{< callout type="info" title="Why a window CTE?" >}}
`ROW_NUMBER() OVER (PARTITION BY entity_id ORDER BY ts DESC)` ranks each entity's observations newest-first; filtering `rn = 1` keeps exactly one row per entity (the latest), which is what you want for point-in-time analysis. For large layers, prefer the **observation-first** form (scan `observations`, then `JOIN entities`) and bound it with a recency filter so the window only ranks recent rows. The schema's `observations` table stores plain `lat REAL` / `lon REAL` (no `position` geometry column).
{{< /callout >}}

### haversine_km for spatial proximity

There is no PostGIS. To correlate entities across layers by geographic proximity, use the registered `haversine_km(lat1, lon1, lat2, lon2)` function (great-circle distance in **kilometres**) gated by a radius, **plus a lat/lon bounding-box prefilter** so the expensive geodesic call only runs on plausible candidates.

```sql
-- bounding-box prefilter (cheap), THEN precise haversine gate (expensive)
WHERE b.lat BETWEEN a.lat - (500.0/111.32) AND a.lat + (500.0/111.32)
  AND b.lon BETWEEN a.lon - (500.0 / (111.32 * max(COS(RADIANS(a.lat)), 0.01)))
                AND a.lon + (500.0 / (111.32 * max(COS(RADIANS(a.lat)), 0.01)))
  AND haversine_km(a.lat, a.lon, b.lat, b.lon) <= 500   -- radius in km
```

{{< callout type="warning" title="Bounding box first, then haversine" >}}
`haversine_km` is a deterministic scalar evaluated per row pair; running it across every entity in a second layer is millions of `COS`/`RADIANS` calls. Always pre-filter with the bounding box first. The latitude bound is `radius_km / 111.32` (degrees per km); the longitude bound divides by `max(COS(RADIANS(lat)), 0.01)` to widen the box near the poles. Use SQLite's `max()` for the floor -- **never** `GREATEST`, which does not exist in SQLite. NULL lat/lon are naturally excluded by the `<= radius` predicate.
{{< /callout >}}

A typical cross-layer spatial join uses CTEs (Common Table Expressions) to structure the query. Force the heavy inner hazard/anchor set with `AS MATERIALIZED` so SQLite computes it once instead of re-running it per candidate in the spatial join:

```sql
WITH latest_alert AS (
  -- Latest observation per disaster entity (replaces LATERAL).
  SELECT o.entity_id, o.lat, o.lon,
         ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
  FROM observations o
  JOIN entities e ON e.id = o.entity_id AND e.layer_type = 'disaster_alerts'
),
anchor AS MATERIALIZED (
  -- CTE: Primary layer (geographic anchor points), latest position per entity.
  SELECT e.id, e.name, e.external_id, lq.lat, lq.lon
  FROM (
    SELECT o.entity_id, o.lat, o.lon,
           ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
    FROM observations o
    JOIN entities e ON e.id = o.entity_id AND e.layer_type = 'earthquakes'
    WHERE julianday(o.ts) > julianday('now','-24 hours')
  ) lq
  JOIN entities e ON e.id = lq.entity_id
  WHERE lq.rn = 1
),
nearby_alerts AS (
  -- Secondary layer correlated by proximity (bbox prefilter + haversine gate).
  SELECT
    a.name AS anchor_name,
    COUNT(DISTINCT d.id) AS alert_count
  FROM anchor a
  JOIN entities d ON d.layer_type = 'disaster_alerts'
  JOIN latest_alert o_d ON o_d.entity_id = d.id AND o_d.rn = 1
  WHERE o_d.lat BETWEEN a.lat - (500.0/111.32) AND a.lat + (500.0/111.32)
    AND o_d.lon BETWEEN a.lon - (500.0 / (111.32 * max(COS(RADIANS(a.lat)), 0.01)))
                    AND a.lon + (500.0 / (111.32 * max(COS(RADIANS(a.lat)), 0.01)))
    AND haversine_km(a.lat, a.lon, o_d.lat, o_d.lon) <= 500
  GROUP BY a.name
)
SELECT
  a.id AS entity_id, a.external_id AS external_id, 'earthquakes' AS layer_type,
  a.name, a.lat, a.lon, 0.0 AS altitude_m,
  strftime('%Y-%m-%dT%H:%M:%SZ','now') AS ts,
  CAST(COALESCE(na.alert_count, 0) AS TEXT) AS nearby_alerts
FROM anchor a
LEFT JOIN nearby_alerts na ON na.anchor_name = a.name
```

{{< callout type="tip" title="Use LEFT JOINs in the final SELECT" >}}
Use `LEFT JOIN` when merging signal CTEs back to anchor entities. This ensures anchor entities appear even when they have no matches in a secondary layer. Use `COALESCE` to fill in defaults (`0` for counts, `'none'` for strings) for missing signals. When a row has no real observation timestamp to report (e.g. an aggregated anchor), synthesize one with `strftime('%Y-%m-%dT%H:%M:%SZ','now') AS ts`.
{{< /callout >}}

---

## Deduplication

The dedup settings prevent the same analysis from being re-run on identical data, reducing unnecessary LLM calls.

```yaml
data:
  dedup:
    window: "12h"
    key_fields: ["entity_external_id"]
```

{{< field name="data.dedup.window" type="duration" required="false" >}}
Time window for deduplication. If the same key was analyzed within this window, the analysis tick is skipped. Example: `"12h"`, `"1h"`.
{{< /field >}}

{{< field name="data.dedup.key_fields" type="string[]" required="false" >}}
Record fields used to compute the dedup key. The engine hashes these fields for each record set. If the hash matches a recent run within the `window`, the analysis is skipped.
{{< /field >}}

---

## Template variables

After the data phase completes, the fetched records are passed to the AI prompt as template variables. These are available inside the Go `text/template` prompt:

### Top-level variables

| Variable | Type | Description |
|----------|------|-------------|
| `{{.RecordCount}}` | int | Number of records after min/max filtering |
| `{{.Lookback}}` | string | Human-readable duration (e.g., `"24h0m0s"`) |
| `{{.Records}}` | array | Slice of AnalysisRecord structs |
| `{{.LayerCount}}` | int | Number of distinct layer types in the records |
| `{{.OutputSchema}}` | string | JSON string of the output schema (auto-injected) |

### Per-record variables

Inside `{{range .Records}}...{{end}}`, each record exposes:

| Variable | Type | Description |
|----------|------|-------------|
| `{{.EntityID}}` | string | Entity ID (internal UUID) |
| `{{.EntityName}}` | string | Entity name |
| `{{.ExternalID}}` | string | Entity external ID |
| `{{.LayerType}}` | string | Source layer type |
| `{{.Lat}}` | float | Latitude |
| `{{.Lon}}` | float | Longitude |
| `{{.Altitude}}` | float | Altitude in meters |
| `{{.Timestamp}}` | string | RFC 3339 timestamp |
| `{{index .Metadata "key"}}` | string | Any non-standard SQL column or entity metadata field |

{{< callout type="info" title="Metadata comes from extra columns" >}}
For layer-based queries, `.Metadata` contains the entity's metadata fields. For SQL-based queries, any column beyond the standard set (`entity_id`, `external_id`, `name`, `layer_type`, `lat`, `lon`, `altitude_m`, `ts`) becomes a `.Metadata` entry; the column name becomes the map key. The engine also mirrors the structural fields into `.Metadata` as `entity_id`, `entity_external_id`, `entity_name`, and `layer_type`, so `dedup.key_fields` and CEL filters can reference them uniformly.
{{< /callout >}}
