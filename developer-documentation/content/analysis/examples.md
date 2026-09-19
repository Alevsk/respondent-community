---
title: "Examples"
description: "Complete analysis definitions — from simple anomaly detection to cross-layer spatial analysis"
weight: 3
---

This page provides two complete analysis definitions that you can copy into your `analysis.d/` directory. The first is a straightforward layer-based analysis; the second demonstrates cross-layer SQL on the read-only SQLite engine using `haversine_km()` proximity joins (there is **no PostGIS**).

{{< callout type="info" title="These examples ship disabled" >}}
Both definitions below ship in the repo under `analysis.d/disabled/` (`ocean_buoy_anomaly.yaml` and `geopolitical_hotspot_index.yaml`) so they don't run out of the box. Files in `disabled/` are not loaded by the engine. To run one, copy it into `analysis.d/` and set `enabled: true`.
{{< /callout >}}

---

## Simple: Ocean Buoy Anomaly Detection

This analysis monitors ocean buoy sensor readings for anomalies that could indicate tsunami precursors, storm surge, or marine heatwaves. It queries a single layer and asks the LLM to flag unusual patterns.

{{< callout type="info" title="When to use layer-based queries" >}}
This analysis uses a simple `layers` array instead of raw SQL. Layer-based queries are the right choice when you analyze data from one layer without needing geographic joins against other layers.
{{< /callout >}}

### Schedule and data

```yaml
schema_version: 1
name: ocean_buoy_anomaly
display_name: "Ocean Buoy Anomaly Detection"
enabled: true

schedule:
  interval: "300s"

data:
  layers: [ocean_buoys]
  lookback: "24h"
  min_records: 5
  max_records: 50
```

{{< callout type="tip" title="min_records prevents empty runs" >}}
Setting `min_records: 5` ensures the analysis only runs when enough data points exist for meaningful anomaly detection. With fewer than 5 buoys, the LLM cannot identify outlier patterns, so the engine skips the tick.
{{< /callout >}}

### AI operation

```yaml
ai:
  enabled: true
  operations:
    - name: buoy_anomaly_scan
      tags: [ocean, buoy, tsunami, anomaly]
      prompt: |
        You are an oceanographic analyst monitoring buoy sensor data for
        anomalies that could indicate natural hazards.

        Analyze {{.RecordCount}} ocean buoy readings from the last {{.Lookback}}:

        {{range .Records}}
        BUOY: {{.EntityName}} ({{.Lat}}, {{.Lon}})
          Station ID: {{index .Metadata "station_id"}}
          Timestamp: {{.Timestamp}}
        {{end}}

        For each buoy, assess whether its position or metadata indicates
        anomalous conditions. Look for:
        - Sudden position changes (possible mooring failure or tsunami drag)
        - Clusters of buoys in unusual proximity (convergence events)
        - Any metadata values outside normal operational ranges

        Classify each finding as: normal, watch, warning, or critical.
        Only include buoys with non-normal classifications in the results.

        Set "attention" DIRECTLY from each finding's classification (do not
        auto-elevate): watch -> low, warning -> medium, critical -> critical.

        Respond with JSON matching this schema:
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [summary, results]
        properties:
          summary:
            type: string
          results:
            type: array
            items:
              type: object
              required: [buoy_name, classification, attention, assessment]
              properties:
                buoy_name: { type: string }
                lat: { type: number }
                lon: { type: number }
                classification:
                  type: string
                  enum: [watch, warning, critical]
                attention:
                  type: string
                  enum: [info, low, medium, high, critical]
                assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 3000
      temperature: 0.1
      output:
        store_insights: true
        min_attention: "low"   # noise gate: drop results below this attention
        insight_type: "ocean_buoy_anomaly"
        results_path: "results"
        websocket_push: true
        retention: "168h"
```

{{< callout type="info" title="min_attention is the noise gate" >}}
`min_attention: "low"` tells the engine to store and push only results whose `attention` is `low` or higher (the ranking is `info < low < medium < high < critical`). Results below the floor -- and any unclassified result, treated as `info` -- are dropped before storage and before the WebSocket notify. Set the `attention` field from the model's own per-result severity; the gate then suppresses the analysis's own routine/"all clear" conclusions instead of paging the operator with them. When no per-operation floor is set, the engine-wide `ai.min_attention` default applies.
{{< /callout >}}

{{< callout type="info" title="results_path creates per-buoy insights" >}}
Setting `results_path: "results"` tells the engine to iterate the `results` array and store one `ai_insight` per anomalous buoy. If no anomalies are found (empty array), a single summary insight is stored instead. This makes it possible to query "show all buoy warnings" from the API.
{{< /callout >}}

{{< callout type="tip" title="Low temperature for classification" >}}
Temperature `0.1` keeps the LLM deterministic for classification tasks. If you find the assessments too uniform, bump it to `0.2`-`0.3` for slightly more nuanced narrative. Do not go above `0.5` for scoring tasks -- you will get inconsistent classifications across runs.
{{< /callout >}}

### Complete definition

Save this as `analysis.d/ocean_buoy_anomaly.yaml` (the repo ships a placeholder version under `analysis.d/disabled/`; set `enabled: true` to run it):

```yaml
# analysis.d/ocean_buoy_anomaly.yaml
# Detect anomalous ocean buoy readings that could indicate
# tsunami precursors, storm surge, or marine heatwaves.
schema_version: 1

name: ocean_buoy_anomaly
display_name: "Ocean Buoy Anomaly Detection"
enabled: true

schedule:
  interval: "300s"

data:
  layers: [ocean_buoys]
  lookback: "24h"
  min_records: 5
  max_records: 50

ai:
  enabled: true
  operations:
    - name: buoy_anomaly_scan
      tags: [ocean, buoy, tsunami, anomaly]
      prompt: |
        You are an oceanographic analyst monitoring buoy sensor data for
        anomalies that could indicate natural hazards.

        Analyze {{.RecordCount}} ocean buoy readings from the last {{.Lookback}}:

        {{range .Records}}
        BUOY: {{.EntityName}} ({{.Lat}}, {{.Lon}})
          Station ID: {{index .Metadata "station_id"}}
          Timestamp: {{.Timestamp}}
        {{end}}

        For each buoy, assess whether its position or metadata indicates
        anomalous conditions. Look for:
        - Sudden position changes (possible mooring failure or tsunami drag)
        - Clusters of buoys in unusual proximity (convergence events)
        - Any metadata values outside normal operational ranges

        Classify each finding as: normal, watch, warning, or critical.
        Only include buoys with non-normal classifications in the results.

        Set "attention" DIRECTLY from each finding's classification (do not
        auto-elevate): watch -> low, warning -> medium, critical -> critical.

        Respond with JSON matching this schema:
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [summary, results]
        properties:
          summary:
            type: string
          results:
            type: array
            items:
              type: object
              required: [buoy_name, classification, attention, assessment]
              properties:
                buoy_name: { type: string }
                lat: { type: number }
                lon: { type: number }
                classification:
                  type: string
                  enum: [watch, warning, critical]
                attention:
                  type: string
                  enum: [info, low, medium, high, critical]
                assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 3000
      temperature: 0.1
      output:
        store_insights: true
        min_attention: "low"   # noise gate: drop results below this attention
        insight_type: "ocean_buoy_anomaly"
        results_path: "results"
        websocket_push: true
        retention: "168h"
```

---

## Advanced: Geopolitical Hotspot Index

This analysis fuses **four** data layers -- conflict events, military aircraft, disaster alerts, and internet outages -- into a composite threat score per region. It demonstrates SQL-based queries on the SQLite engine using `haversine_km()` proximity joins (no PostGIS), weighted scoring rubrics, and array output. It ships disabled at `analysis.d/disabled/geopolitical_hotspot_index.yaml`.

{{< callout type="info" title="Cross-layer signal fusion" >}}
The core idea: anchor on conflict regions with fatalities, then spatially search for corroborating signals from three other layers within a radius -- military aircraft (300km), disaster alerts (500km), and internet outages (500km). A conflict zone with military aircraft overhead, a compounding disaster, and internet blackouts is a far stronger signal than a conflict zone alone. This multi-signal approach produces more accurate threat assessments than single-layer analysis.
{{< /callout >}}

### Schedule and data

```yaml
schema_version: 1
name: geopolitical_hotspot_index
display_name: "Geopolitical Hotspot Index"
enabled: true

schedule:
  interval: "300s"

data:
  lookback: "24h"
  min_records: 1
  max_records: 15
  layers: [conflict_events]
  dedup:
    window: "12h"
    key_fields: ["entity_external_id"]
```

{{< callout type="tip" title="Dedup prevents redundant LLM calls" >}}
The `dedup` block ensures that if the same set of conflict regions is returned within a 12-hour window, the analysis is skipped. This is important for expensive cross-layer analyses -- without dedup, you would call the LLM every 5 minutes with identical data.
{{< /callout >}}

### SQL query

```yaml
  sql: |
    WITH latest_conf AS (
      -- Latest observation position per conflict entity (replaces LATERAL).
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'conflict_events'
    ),
    latest_mil AS (
      SELECT o.entity_id, o.lat, o.lon, o.ts,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'flights_military'
    ),
    latest_dis AS (
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'disaster_alerts'
    ),
    latest_out AS (
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'internet_outages'
    ),
    region_signals AS (
      -- CTE 1: ANCHOR — Conflict regions with confirmed fatalities.
      SELECT
        conf.id, conf.name AS region_name, conf.external_id,
        o_conf.lat, o_conf.lon,
        CAST(COALESCE(json_extract(conf.metadata,'$.fatalities'), '0') AS INTEGER) AS fatalities,
        CAST(COALESCE(json_extract(conf.metadata,'$.events'), '0') AS INTEGER) AS event_count
      FROM entities conf
      JOIN latest_conf o_conf ON o_conf.entity_id = conf.id AND o_conf.rn = 1
      WHERE conf.layer_type = 'conflict_events'
        AND CAST(COALESCE(json_extract(conf.metadata,'$.fatalities'), '0') AS INTEGER) > 0
    ),
    military_presence AS (
      -- CTE 2: SIGNAL — Military aircraft within 300km, observed in last 24h.
      SELECT
        rs.region_name,
        COUNT(DISTINCT mil.id) AS military_aircraft_count,
        group_concat(DISTINCT json_extract(mil.metadata,'$.type')) AS aircraft_types
      FROM region_signals rs
      JOIN entities mil ON mil.layer_type = 'flights_military'
      JOIN latest_mil o_mil ON o_mil.entity_id = mil.id AND o_mil.rn = 1
      WHERE julianday(o_mil.ts) > julianday('now','-24 hours')
        AND o_mil.lat BETWEEN rs.lat-(300/111.32) AND rs.lat+(300/111.32)
        AND o_mil.lon BETWEEN rs.lon-(300/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(300/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_mil.lat, o_mil.lon) <= 300
      GROUP BY rs.region_name
    ),
    nearby_disasters AS (
      -- CTE 3: SIGNAL — Disaster alerts within 500km.
      SELECT
        rs.region_name,
        COUNT(DISTINCT d.id) AS disaster_count,
        group_concat(DISTINCT json_extract(d.metadata,'$.eventtype')) AS disaster_types
      FROM region_signals rs
      JOIN entities d ON d.layer_type = 'disaster_alerts'
      JOIN latest_dis o_d ON o_d.entity_id = d.id AND o_d.rn = 1
      WHERE o_d.lat BETWEEN rs.lat-(500/111.32) AND rs.lat+(500/111.32)
        AND o_d.lon BETWEEN rs.lon-(500/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(500/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_d.lat, o_d.lon) <= 500
      GROUP BY rs.region_name
    ),
    nearby_outages AS (
      -- CTE 4: SIGNAL — Internet outages within 500km.
      SELECT
        rs.region_name,
        COUNT(DISTINCT io.id) AS outage_count,
        group_concat(DISTINCT json_extract(io.metadata,'$.outage_type')) AS outage_types
      FROM region_signals rs
      JOIN entities io ON io.layer_type = 'internet_outages'
      JOIN latest_out o_io ON o_io.entity_id = io.id AND o_io.rn = 1
      WHERE o_io.lat BETWEEN rs.lat-(500/111.32) AND rs.lat+(500/111.32)
        AND o_io.lon BETWEEN rs.lon-(500/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(500/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_io.lat, o_io.lon) <= 500
      GROUP BY rs.region_name
    )
    -- FINAL SELECT: Merge all signals per region via LEFT JOINs.
    SELECT
      rs.id AS entity_id,
      rs.region_name AS name,
      rs.external_id AS external_id,
      'conflict_events' AS layer_type,
      rs.lat, rs.lon, 0.0 AS altitude_m,
      strftime('%Y-%m-%dT%H:%M:%SZ','now') AS ts,
      CAST(rs.fatalities AS TEXT) AS fatalities,
      CAST(rs.event_count AS TEXT) AS event_count,
      CAST(COALESCE(mp.military_aircraft_count, 0) AS TEXT) AS military_aircraft,
      COALESCE(mp.aircraft_types, 'none') AS aircraft_types,
      CAST(COALESCE(nd.disaster_count, 0) AS TEXT) AS disaster_count,
      COALESCE(nd.disaster_types, 'none') AS disaster_types,
      CAST(COALESCE(no2.outage_count, 0) AS TEXT) AS internet_outages,
      COALESCE(no2.outage_types, 'none') AS outage_types
    FROM region_signals rs
    LEFT JOIN military_presence mp ON mp.region_name = rs.region_name
    LEFT JOIN nearby_disasters nd ON nd.region_name = rs.region_name
    LEFT JOIN nearby_outages no2 ON no2.region_name = rs.region_name
    WHERE rs.fatalities > 0
      AND (COALESCE(mp.military_aircraft_count, 0) > 0
           OR COALESCE(nd.disaster_count, 0) > 0
           OR COALESCE(no2.outage_count, 0) > 0
           OR rs.fatalities >= 10)
    ORDER BY rs.fatalities DESC
    LIMIT 15
```

{{< callout type="info" title="SQL structure walkthrough" >}}
The query opens with four `latest_*` window CTEs -- one per layer -- that rank each entity's observations with `ROW_NUMBER() OVER (PARTITION BY entity_id ORDER BY ts DESC)` and are joined on `rn = 1` to get the latest position (the SQLite replacement for `CROSS JOIN LATERAL`). Then:

**`region_signals` (anchor)** -- Conflict regions with confirmed fatalities (`json_extract(metadata,'$.fatalities')` cast to INTEGER, `> 0`).

**`military_presence`, `nearby_disasters`, `nearby_outages` (signals)** -- For each anchor region, count distinct entities from the other layer within radius. Each uses a **lat/lon bounding-box prefilter** (the lon bound divides by `max(COS(RADIANS(rs.lat)), 0.01)`) followed by a precise `haversine_km(...) <= radius` gate: 300km for military, 500km for disasters and outages. Military presence is additionally time-filtered to the last 24h via `julianday(o_mil.ts) > julianday('now','-24 hours')`. Distinct type codes are aggregated with `group_concat(DISTINCT ...)`.

**Final SELECT** -- Merges all signals via `LEFT JOIN` so regions without a given signal still appear, casting every extra column to TEXT and synthesizing `ts` with `strftime('%Y-%m-%dT%H:%M:%SZ','now')`. The `WHERE` clause requires at least one corroborating signal OR high fatalities (>= 10) to reduce noise.
{{< /callout >}}

{{< callout type="tip" title="Adding more signal layers" >}}
Each signal layer is isolated: a `latest_*` window CTE plus an aggregating CTE, joined back with a `LEFT JOIN` in the final SELECT. To add a fifth signal, follow the same pattern. For heavy anchor or hazard sets feeding a spatial join, mark the CTE `AS MATERIALIZED` so SQLite computes it once instead of re-running it per candidate.
{{< /callout >}}

### AI operation

```yaml
ai:
  enabled: true
  operations:
    - name: hotspot_index_scoring
      tags: [osint, geopolitical, composite, cross-layer]
      prompt: |
        You are a senior geopolitical risk analyst producing composite threat
        assessments. Your assessments will be read by decision-makers who need
        accurate, calibrated risk scores.

        GEOPOLITICAL HOTSPOT INDEX: Composite threat scoring for
        {{.RecordCount}} regions using cross-layer signal fusion
        (conflict + military + disasters + comms) over {{.Lookback}}.

        Multi-signal regional data:
        {{range .Records}}
        REGION: {{.EntityName}} ({{.Lat}}, {{.Lon}}, ID: {{.ExternalID}})
          Conflict: {{index .Metadata "fatalities"}} fatalities / {{index .Metadata "event_count"}} events
          Military: {{index .Metadata "military_aircraft"}} aircraft ({{index .Metadata "aircraft_types"}})
          Disasters: {{index .Metadata "disaster_count"}} ({{index .Metadata "disaster_types"}})
          Comms disruption: {{index .Metadata "internet_outages"}} outages ({{index .Metadata "outage_types"}})
        {{end}}

        For each region, compute a composite Hotspot Index (0-100) using
        these weighted signals:
        - Conflict intensity (fatalities + events): 40% weight
        - Military presence (aircraft count + types — ISR/AWACS higher than transport): 25% weight
        - Disaster overlay (compounding humanitarian crisis): 20% weight
        - Communications disruption (internet outages indicate escalation/censorship): 15% weight

        CALIBRATION:
        - critical (81-100): Active large-scale war, 1000+ fatalities, confirmed military ops
        - crisis (61-80): Sustained armed conflict, 100-1000 fatalities, military escalation signals
        - volatile (41-60): Ongoing low-intensity conflict, military posturing
        - elevated (21-40): Chronic instability, intermittent violence
        - stable (0-20): Genuinely low risk, minimal recent violence

        CRITICAL: Do NOT classify a region as "stable" purely because its
        fatality count is low relative to active war zones. Use your world
        knowledge of the region's chronic instability.

        The strategic_assessment MUST be 2-4 sentences explaining what drives
        the score, referencing specific signals from the data.

        Set "attention" DIRECTLY from each region's classification (do not
        auto-elevate): stable -> info, elevated -> low, volatile -> medium,
        crisis -> high, critical -> critical.

        Respond with JSON matching this schema:
        {{.OutputSchema}}
```

{{< callout type="tip" title="Calibration anchors produce consistent scores" >}}
The CALIBRATION section provides concrete examples for each classification bracket. Without these anchors, the LLM tends to cluster scores around 50 or assign "stable" to regions with moderate violence. By giving it explicit thresholds (e.g., "1000+ fatalities = critical"), you get reproducible scoring across runs.
{{< /callout >}}

### Output schema and configuration

```yaml
      output_schema:
        type: object
        required: [hotspots, global_summary]
        properties:
          global_summary:
            type: string
          hotspots:
            type: array
            items:
              type: object
              required: [region, hotspot_index, classification, attention, strategic_assessment]
              properties:
                region: { type: string }
                lat: { type: number }
                lon: { type: number }
                hotspot_index: { type: integer, minimum: 0, maximum: 100 }
                classification:
                  type: string
                  enum: [stable, elevated, volatile, crisis, critical]
                attention:
                  type: string
                  enum: [info, low, medium, high, critical]
                conflict_score: { type: integer, minimum: 0, maximum: 100 }
                military_score: { type: integer, minimum: 0, maximum: 100 }
                disaster_score: { type: integer, minimum: 0, maximum: 100 }
                comms_score: { type: integer, minimum: 0, maximum: 100 }
                strategic_assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 8000
      temperature: 0.2
      output:
        store_insights: true
        min_attention: "medium"   # noise gate: drop results below this attention
        insight_type: "hotspot_index"
        results_path: "hotspots"
        ref_fields:
          - field: "entity_external_id"
            layer_type: "conflict_events"
        websocket_push: true
        retention: "336h"
```

{{< callout type="info" title="results_path + entity_external_id" >}}
`results_path: "hotspots"` extracts each item from the `hotspots` array and stores it as a separate `ai_insight` row. If the LLM includes `entity_external_id` in each item, the engine matches it against the input records and links the insight to the source entity -- enabling "show all insights for Ukraine" queries.
{{< /callout >}}

{{< callout type="info" title="ref_fields links extra entities" >}}
`ref_fields` resolves an additional cross-layer entity reference per result. Each entry names a result field and the `layer_type` to look it up in; the engine resolves `(layer_type, field-value)` to an entity and adds an `ai_insight_refs` row. Here `entity_external_id` is resolved against `conflict_events`. Use this when a single insight should link to entities the matcher would not otherwise associate (e.g., the threatened cable or weather alert in the infrastructure and aviation analyses).
{{< /callout >}}

{{< callout type="info" title="min_attention is the noise gate" >}}
`min_attention: "medium"` stores and pushes only hotspots whose `attention` is `medium` or higher (ranking: `info < low < medium < high < critical`). Lower or unclassified results are dropped before storage and WebSocket notify -- so `stable`/`elevated` regions mapped to `info`/`low` are suppressed, and only `volatile`/`crisis`/`critical` regions page through. Set the `attention` field from the model's own classification; when no per-operation floor is set, the engine-wide `ai.min_attention` default applies.
{{< /callout >}}

{{< callout type="info" title="Retention controls table size" >}}
`retention: "336h"` (14 days) means insights older than two weeks are automatically deleted by the hourly cleanup goroutine. Increase retention for audit trails; decrease it for high-frequency analyses.
{{< /callout >}}

### Complete definition

Save this as `analysis.d/geopolitical_hotspot_index.yaml` (it ships under `analysis.d/disabled/`; copy it up and set `enabled: true` to run it):

```yaml
# analysis.d/geopolitical_hotspot_index.yaml
# Cross-layer composite threat scoring:
# conflict + military + disaster + comms signals.
schema_version: 1

name: geopolitical_hotspot_index
display_name: "Geopolitical Hotspot Index"
enabled: true

schedule:
  interval: "300s"

data:
  lookback: "24h"
  min_records: 1
  max_records: 15
  layers: [conflict_events]
  dedup:
    window: "12h"
    key_fields: ["entity_external_id"]
  sql: |
    WITH latest_conf AS (
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'conflict_events'
    ),
    latest_mil AS (
      SELECT o.entity_id, o.lat, o.lon, o.ts,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'flights_military'
    ),
    latest_dis AS (
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'disaster_alerts'
    ),
    latest_out AS (
      SELECT o.entity_id, o.lat, o.lon,
        ROW_NUMBER() OVER (PARTITION BY o.entity_id ORDER BY o.ts DESC) AS rn
      FROM observations o
      JOIN entities e ON e.id = o.entity_id
      WHERE e.layer_type = 'internet_outages'
    ),
    region_signals AS (
      -- CTE 1: ANCHOR — Conflict regions with confirmed fatalities.
      SELECT
        conf.id, conf.name AS region_name, conf.external_id,
        o_conf.lat, o_conf.lon,
        CAST(COALESCE(json_extract(conf.metadata,'$.fatalities'), '0') AS INTEGER) AS fatalities,
        CAST(COALESCE(json_extract(conf.metadata,'$.events'), '0') AS INTEGER) AS event_count
      FROM entities conf
      JOIN latest_conf o_conf ON o_conf.entity_id = conf.id AND o_conf.rn = 1
      WHERE conf.layer_type = 'conflict_events'
        AND CAST(COALESCE(json_extract(conf.metadata,'$.fatalities'), '0') AS INTEGER) > 0
    ),
    military_presence AS (
      -- CTE 2: SIGNAL — Military aircraft within 300km, observed in last 24h.
      SELECT
        rs.region_name,
        COUNT(DISTINCT mil.id) AS military_aircraft_count,
        group_concat(DISTINCT json_extract(mil.metadata,'$.type')) AS aircraft_types
      FROM region_signals rs
      JOIN entities mil ON mil.layer_type = 'flights_military'
      JOIN latest_mil o_mil ON o_mil.entity_id = mil.id AND o_mil.rn = 1
      WHERE julianday(o_mil.ts) > julianday('now','-24 hours')
        AND o_mil.lat BETWEEN rs.lat-(300/111.32) AND rs.lat+(300/111.32)
        AND o_mil.lon BETWEEN rs.lon-(300/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(300/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_mil.lat, o_mil.lon) <= 300
      GROUP BY rs.region_name
    ),
    nearby_disasters AS (
      -- CTE 3: SIGNAL — Disaster alerts within 500km.
      SELECT
        rs.region_name,
        COUNT(DISTINCT d.id) AS disaster_count,
        group_concat(DISTINCT json_extract(d.metadata,'$.eventtype')) AS disaster_types
      FROM region_signals rs
      JOIN entities d ON d.layer_type = 'disaster_alerts'
      JOIN latest_dis o_d ON o_d.entity_id = d.id AND o_d.rn = 1
      WHERE o_d.lat BETWEEN rs.lat-(500/111.32) AND rs.lat+(500/111.32)
        AND o_d.lon BETWEEN rs.lon-(500/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(500/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_d.lat, o_d.lon) <= 500
      GROUP BY rs.region_name
    ),
    nearby_outages AS (
      -- CTE 4: SIGNAL — Internet outages within 500km.
      SELECT
        rs.region_name,
        COUNT(DISTINCT io.id) AS outage_count,
        group_concat(DISTINCT json_extract(io.metadata,'$.outage_type')) AS outage_types
      FROM region_signals rs
      JOIN entities io ON io.layer_type = 'internet_outages'
      JOIN latest_out o_io ON o_io.entity_id = io.id AND o_io.rn = 1
      WHERE o_io.lat BETWEEN rs.lat-(500/111.32) AND rs.lat+(500/111.32)
        AND o_io.lon BETWEEN rs.lon-(500/(111.32*max(COS(RADIANS(rs.lat)),0.01))) AND rs.lon+(500/(111.32*max(COS(RADIANS(rs.lat)),0.01)))
        AND haversine_km(rs.lat, rs.lon, o_io.lat, o_io.lon) <= 500
      GROUP BY rs.region_name
    )
    -- FINAL SELECT: Merge all signals per region via LEFT JOINs.
    SELECT
      rs.id AS entity_id,
      rs.region_name AS name,
      rs.external_id AS external_id,
      'conflict_events' AS layer_type,
      rs.lat, rs.lon, 0.0 AS altitude_m,
      strftime('%Y-%m-%dT%H:%M:%SZ','now') AS ts,
      CAST(rs.fatalities AS TEXT) AS fatalities,
      CAST(rs.event_count AS TEXT) AS event_count,
      CAST(COALESCE(mp.military_aircraft_count, 0) AS TEXT) AS military_aircraft,
      COALESCE(mp.aircraft_types, 'none') AS aircraft_types,
      CAST(COALESCE(nd.disaster_count, 0) AS TEXT) AS disaster_count,
      COALESCE(nd.disaster_types, 'none') AS disaster_types,
      CAST(COALESCE(no2.outage_count, 0) AS TEXT) AS internet_outages,
      COALESCE(no2.outage_types, 'none') AS outage_types
    FROM region_signals rs
    LEFT JOIN military_presence mp ON mp.region_name = rs.region_name
    LEFT JOIN nearby_disasters nd ON nd.region_name = rs.region_name
    LEFT JOIN nearby_outages no2 ON no2.region_name = rs.region_name
    WHERE rs.fatalities > 0
      AND (COALESCE(mp.military_aircraft_count, 0) > 0
           OR COALESCE(nd.disaster_count, 0) > 0
           OR COALESCE(no2.outage_count, 0) > 0
           OR rs.fatalities >= 10)
    ORDER BY rs.fatalities DESC
    LIMIT 15

ai:
  enabled: true
  operations:
    - name: hotspot_index_scoring
      tags: [osint, geopolitical, composite, cross-layer]
      prompt: |
        You are a senior geopolitical risk analyst producing composite threat
        assessments. Your assessments will be read by decision-makers who need
        accurate, calibrated risk scores.

        GEOPOLITICAL HOTSPOT INDEX: Composite threat scoring for
        {{.RecordCount}} regions using cross-layer signal fusion
        (conflict + military + disasters + comms) over {{.Lookback}}.

        Multi-signal regional data:
        {{range .Records}}
        REGION: {{.EntityName}} ({{.Lat}}, {{.Lon}}, ID: {{.ExternalID}})
          Conflict: {{index .Metadata "fatalities"}} fatalities / {{index .Metadata "event_count"}} events
          Military: {{index .Metadata "military_aircraft"}} aircraft ({{index .Metadata "aircraft_types"}})
          Disasters: {{index .Metadata "disaster_count"}} ({{index .Metadata "disaster_types"}})
          Comms disruption: {{index .Metadata "internet_outages"}} outages ({{index .Metadata "outage_types"}})
        {{end}}

        For each region, compute a composite Hotspot Index (0-100) using
        these weighted signals:
        - Conflict intensity (fatalities + events): 40% weight
        - Military presence (aircraft count + types — ISR/AWACS higher than transport): 25% weight
        - Disaster overlay (compounding humanitarian crisis): 20% weight
        - Communications disruption (internet outages indicate escalation/censorship): 15% weight

        CALIBRATION:
        - critical (81-100): Active large-scale war, 1000+ fatalities, confirmed military ops
        - crisis (61-80): Sustained armed conflict, 100-1000 fatalities, military escalation signals
        - volatile (41-60): Ongoing low-intensity conflict, military posturing
        - elevated (21-40): Chronic instability, intermittent violence
        - stable (0-20): Genuinely low risk, minimal recent violence

        CRITICAL: Do NOT classify a region as "stable" purely because its
        fatality count is low relative to active war zones. Use your world
        knowledge of the region's chronic instability.

        The strategic_assessment MUST be 2-4 sentences explaining what drives
        the score, referencing specific signals from the data.

        Set "attention" DIRECTLY from each region's classification (do not
        auto-elevate): stable -> info, elevated -> low, volatile -> medium,
        crisis -> high, critical -> critical.

        Respond with JSON matching this schema:
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [hotspots, global_summary]
        properties:
          global_summary:
            type: string
          hotspots:
            type: array
            items:
              type: object
              required: [region, hotspot_index, classification, attention, strategic_assessment]
              properties:
                region: { type: string }
                lat: { type: number }
                lon: { type: number }
                hotspot_index: { type: integer, minimum: 0, maximum: 100 }
                classification:
                  type: string
                  enum: [stable, elevated, volatile, crisis, critical]
                attention:
                  type: string
                  enum: [info, low, medium, high, critical]
                conflict_score: { type: integer, minimum: 0, maximum: 100 }
                military_score: { type: integer, minimum: 0, maximum: 100 }
                disaster_score: { type: integer, minimum: 0, maximum: 100 }
                comms_score: { type: integer, minimum: 0, maximum: 100 }
                strategic_assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 8000
      temperature: 0.2
      output:
        store_insights: true
        min_attention: "medium"   # noise gate: drop results below this attention
        insight_type: "hotspot_index"
        results_path: "hotspots"
        ref_fields:
          - field: "entity_external_id"
            layer_type: "conflict_events"
        websocket_push: true
        retention: "336h"
```
