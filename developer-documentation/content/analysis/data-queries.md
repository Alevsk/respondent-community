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
    double(metadata.magnitude) >= 4.0
```

{{< field name="data.layers" type="string[]" required="true" >}}
Array of layer types to query. The engine fetches the latest entities for each layer and combines the results. Example: `[earthquakes]`, `[flights_military, conflict_events]`.
{{< /field >}}

{{< field name="data.lookback" type="duration" required="false" >}}
Time window for data freshness. Records with timestamps older than `now() - lookback` are discarded. Example: `"24h"`, `"6h"`, `"168h"`.
{{< /field >}}

{{< field name="data.filter" type="string" required="false" >}}
CEL expression to filter records after fetching. The expression receives `metadata` as a map of entity metadata fields. Records where the expression evaluates to `false` are excluded.
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

SQL-based queries give you direct access to the database with PostGIS spatial functions. This is required for cross-layer analysis where you correlate entities from different layers based on geographic proximity.

```yaml
data:
  lookback: "24h"
  min_records: 1
  max_records: 30
  sql: |
    SELECT
      e.id AS entity_id,
      e.name AS name,
      e.external_id AS external_id,
      e.layer_type AS layer_type,
      o.lat, o.lon, 0.0 AS altitude_m,
      o.ts,
      e.metadata->>'magnitude' AS magnitude
    FROM entities e
    CROSS JOIN LATERAL (
      SELECT lat, lon, position, ts
      FROM observations
      WHERE entity_id = e.id
      ORDER BY ts DESC LIMIT 1
    ) o
    WHERE e.layer_type = 'earthquakes'
      AND o.ts > NOW() - INTERVAL '24 hours'
    ORDER BY (e.metadata->>'magnitude')::float DESC
    LIMIT 30
```

{{< field name="data.sql" type="string" required="true" >}}
Raw SQL query executed via the read-only QueryExecutor. The SQL is validated before execution -- no `INSERT`, `UPDATE`, `DELETE`, or `DROP` statements are allowed.
{{< /field >}}

{{< callout type="warning" title="SQL is read-only" >}}
The engine validates your SQL as read-only before execution. Any mutation statements will be rejected with an error.
{{< /callout >}}

### Required output columns

Your SQL query must return these standard columns. The engine maps them to the `AnalysisRecord` struct used in prompt templates.

| Column | Type | Maps to |
|--------|------|---------|
| `name` | text | `{{.EntityName}}` |
| `lat` | float | `{{.Lat}}` |
| `lon` | float | `{{.Lon}}` |
| `altitude_m` | float | `{{.Altitude}}` |
| `ts` | timestamp | `{{.Timestamp}}` |

Any additional columns beyond these become entries in the `.Metadata` map, accessible in prompts via `{{index .Metadata "column_name"}}`. All metadata values are cast to strings.

{{< callout type="tip" title="Cast extra columns to text" >}}
The metadata map stores `map[string]string`. Cast all non-standard columns to text in your SQL: `rs.fatalities::text AS fatalities`. This avoids type mismatch errors at runtime.
{{< /callout >}}

### CROSS JOIN LATERAL pattern

The most common SQL pattern is using `CROSS JOIN LATERAL` to get the latest observation for each entity. This is the standard approach because entities can have multiple observations (in `append` recording mode), and you typically want only the most recent position.

```sql
SELECT
  e.name, o.lat, o.lon, 0.0 AS altitude_m, o.ts
FROM entities e
CROSS JOIN LATERAL (
  SELECT lat, lon, position, ts
  FROM observations
  WHERE entity_id = e.id
  ORDER BY ts DESC LIMIT 1
) o
WHERE e.layer_type = 'earthquakes'
```

{{< callout type="info" title="Why CROSS JOIN LATERAL?" >}}
`CROSS JOIN LATERAL` allows the subquery to reference `e.id` from the outer query. It returns exactly one row per entity (the latest observation), which is what you want for point-in-time analysis. Without `LATERAL`, you would need a more complex correlated subquery or window function.
{{< /callout >}}

### ST_DWithin for spatial proximity

To correlate entities across layers by geographic proximity, use `ST_DWithin()` with the `geography` type.

```sql
ST_DWithin(
  point_a.position::geography,
  point_b.position::geography,
  500000  -- distance in meters (500km)
)
```

{{< callout type="info" title="Geography vs. geometry" >}}
The `::geography` cast is important. Without it, PostGIS uses planar geometry (degrees), which produces incorrect distances on a spherical earth. With `::geography`, `ST_DWithin` computes great-circle distances in meters using the WGS84 spheroid. The `position` column in the observations table stores a PostGIS `geometry(Point, 4326)` that you cast to `geography` for accurate distance calculations.
{{< /callout >}}

A typical cross-layer spatial join uses CTEs (Common Table Expressions) to structure the query:

```sql
WITH anchor AS (
  -- CTE 1: Primary layer (geographic anchor points)
  SELECT e.id, e.name, o.lat, o.lon, o.position
  FROM entities e
  CROSS JOIN LATERAL (
    SELECT lat, lon, position FROM observations WHERE entity_id = e.id ORDER BY ts DESC LIMIT 1
  ) o
  WHERE e.layer_type = 'earthquakes'
),
nearby_alerts AS (
  -- CTE 2: Secondary layer correlated by proximity
  SELECT
    a.name AS anchor_name,
    COUNT(DISTINCT d.id) AS alert_count
  FROM anchor a
  JOIN entities d ON d.layer_type = 'disaster_alerts'
  CROSS JOIN LATERAL (
    SELECT position FROM observations WHERE entity_id = d.id ORDER BY ts DESC LIMIT 1
  ) o_d
  WHERE ST_DWithin(a.position::geography, o_d.position::geography, 500000)
  GROUP BY a.name
)
SELECT
  a.name, a.lat, a.lon, 0.0 AS altitude_m, NOW() AS ts,
  COALESCE(na.alert_count, 0)::text AS nearby_alerts
FROM anchor a
LEFT JOIN nearby_alerts na ON na.anchor_name = a.name
```

{{< callout type="tip" title="Use LEFT JOINs in the final SELECT" >}}
Use `LEFT JOIN` when merging signal CTEs back to anchor entities. This ensures anchor entities appear even when they have no matches in a secondary layer. Use `COALESCE` to fill in defaults (`0` for counts, `'none'` for strings) for missing signals.
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
| `{{.EntityName}}` | string | Entity name |
| `{{.ExternalID}}` | string | Entity external ID |
| `{{.LayerType}}` | string | Source layer type |
| `{{.Lat}}` | float | Latitude |
| `{{.Lon}}` | float | Longitude |
| `{{.Altitude}}` | float | Altitude in meters |
| `{{.Timestamp}}` | string | RFC 3339 timestamp |
| `{{index .Metadata "key"}}` | string | Any non-standard SQL column or entity metadata field |

{{< callout type="info" title="Metadata comes from extra columns" >}}
For layer-based queries, `.Metadata` contains the entity's metadata fields. For SQL-based queries, any column beyond the standard set (`name`, `lat`, `lon`, `altitude_m`, `ts`) becomes a `.Metadata` entry. The column name becomes the map key.
{{< /callout >}}
