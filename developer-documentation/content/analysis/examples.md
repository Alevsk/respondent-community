---
title: "Examples"
description: "Complete analysis definitions — from simple anomaly detection to cross-layer spatial analysis"
weight: 3
---

This page provides two complete analysis definitions that you can copy into your `analysis.d/` directory. The first is a straightforward layer-based analysis; the second demonstrates cross-layer SQL with PostGIS spatial joins.

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
              required: [buoy_name, classification, assessment]
              properties:
                buoy_name: { type: string }
                lat: { type: number }
                lon: { type: number }
                classification:
                  type: string
                  enum: [watch, warning, critical]
                assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 3000
      temperature: 0.1
      output:
        store_insights: true
        insight_type: "ocean_buoy_anomaly"
        results_path: "results"
        websocket_push: true
        retention: "168h"
```

{{< callout type="info" title="results_path creates per-buoy insights" >}}
Setting `results_path: "results"` tells the engine to iterate the `results` array and store one `ai_insight` per anomalous buoy. If no anomalies are found (empty array), a single summary insight is stored instead. This makes it possible to query "show all buoy warnings" from the API.
{{< /callout >}}

{{< callout type="tip" title="Low temperature for classification" >}}
Temperature `0.1` keeps the LLM deterministic for classification tasks. If you find the assessments too uniform, bump it to `0.2`-`0.3` for slightly more nuanced narrative. Do not go above `0.5` for scoring tasks -- you will get inconsistent classifications across runs.
{{< /callout >}}

### Complete definition

Save this as `analysis.d/ocean_buoy_anomaly.yaml`:

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
              required: [buoy_name, classification, assessment]
              properties:
                buoy_name: { type: string }
                lat: { type: number }
                lon: { type: number }
                classification:
                  type: string
                  enum: [watch, warning, critical]
                assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 3000
      temperature: 0.1
      output:
        store_insights: true
        insight_type: "ocean_buoy_anomaly"
        results_path: "results"
        websocket_push: true
        retention: "168h"
```

---

## Advanced: Geopolitical Hotspot Index

This analysis fuses two data layers -- conflict events and military aircraft -- into a composite threat score per region. It demonstrates SQL-based queries with PostGIS spatial joins, weighted scoring rubrics, and array output.

{{< callout type="info" title="Cross-layer signal fusion" >}}
The core idea: anchor on conflict regions, then spatially search for corroborating signals from other layers within a radius. Military aircraft near a conflict zone is a stronger signal than a conflict zone alone. This multi-signal approach produces more accurate threat assessments than single-layer analysis.
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
    WITH region_signals AS (
      SELECT
        conf.id, conf.name AS region_name, conf.external_id,
        o_conf.lat, o_conf.lon, o_conf.position,
        CAST(COALESCE(conf.metadata->>'fatalities', '0') AS int) AS fatalities,
        CAST(COALESCE(conf.metadata->>'events', '0') AS int) AS event_count
      FROM entities conf
      CROSS JOIN LATERAL (
        SELECT lat, lon, position
        FROM observations
        WHERE entity_id = conf.id
        ORDER BY ts DESC LIMIT 1
      ) o_conf
      WHERE conf.layer_type = 'conflict_events'
        AND CAST(COALESCE(conf.metadata->>'fatalities', '0') AS int) > 0
    ),
    military_presence AS (
      SELECT
        rs.region_name,
        COUNT(DISTINCT mil.id) AS military_aircraft_count,
        STRING_AGG(DISTINCT mil.metadata->>'type', ', ') AS aircraft_types
      FROM region_signals rs
      JOIN entities mil ON mil.layer_type = 'flights_military'
      CROSS JOIN LATERAL (
        SELECT position, ts
        FROM observations
        WHERE entity_id = mil.id
        ORDER BY ts DESC LIMIT 1
      ) o_mil
      WHERE o_mil.ts > NOW() - INTERVAL '24 hours'
        AND ST_DWithin(
            rs.position::geography,
            o_mil.position::geography,
            300000
        )
      GROUP BY rs.region_name
    )
    SELECT
      rs.id AS entity_id,
      rs.region_name AS name,
      rs.external_id AS external_id,
      'conflict_events' AS layer_type,
      rs.lat, rs.lon, 0.0 AS altitude_m,
      NOW() AS ts,
      rs.fatalities::text AS fatalities,
      rs.event_count::text AS event_count,
      COALESCE(mp.military_aircraft_count, 0)::text AS military_aircraft,
      COALESCE(mp.aircraft_types, 'none') AS aircraft_types
    FROM region_signals rs
    LEFT JOIN military_presence mp ON mp.region_name = rs.region_name
    WHERE rs.fatalities > 0
      AND (COALESCE(mp.military_aircraft_count, 0) > 0
           OR rs.fatalities >= 10)
    ORDER BY rs.fatalities DESC
    LIMIT 15
```

{{< callout type="info" title="SQL structure walkthrough" >}}
This query has two CTEs:

**CTE 1 (`region_signals`)** -- Anchors on conflict regions with confirmed fatalities. Uses `CROSS JOIN LATERAL` to get each region's latest geographic position.

**CTE 2 (`military_presence`)** -- For each anchor region, counts distinct military aircraft within 300km using `ST_DWithin`. The 300km radius represents the operational range where aircraft could be supporting or monitoring the conflict zone. Only aircraft observed in the last 24 hours are included.

**Final SELECT** -- Merges signals via `LEFT JOIN` so regions without military presence still appear. The `WHERE` clause requires either a corroborating military signal OR high fatalities (>= 10) to reduce noise.
{{< /callout >}}

{{< callout type="tip" title="Adding more signal layers" >}}
To add a third signal (e.g., disaster alerts), add another CTE following the same pattern as `military_presence`, then add a `LEFT JOIN` in the final SELECT. The CTE structure scales cleanly -- each signal layer is isolated in its own CTE.
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
        (conflict + military) over {{.Lookback}}.

        Multi-signal regional data:
        {{range .Records}}
        REGION: {{.EntityName}} ({{.Lat}}, {{.Lon}}, ID: {{.ExternalID}})
          Conflict: {{index .Metadata "fatalities"}} fatalities / {{index .Metadata "event_count"}} events
          Military: {{index .Metadata "military_aircraft"}} aircraft ({{index .Metadata "aircraft_types"}})
        {{end}}

        For each region, compute a composite Hotspot Index (0-100) using
        these weighted signals:
        - Conflict intensity (fatalities + events): 60% weight
        - Military presence (aircraft count + type significance): 40% weight

        CALIBRATION:
        - critical (81-100): Active large-scale war, 1000+ fatalities, confirmed military ops
        - crisis (61-80): Sustained armed conflict, 100-1000 fatalities, military escalation signals
        - volatile (41-60): Ongoing low-intensity conflict, military posturing
        - elevated (21-40): Chronic instability, intermittent violence
        - stable (0-20): Genuinely low risk, minimal recent violence

        CRITICAL: Do NOT classify a region as "stable" purely because its
        fatality count is low relative to active war zones. Use your world
        knowledge of the region's chronic instability.

        The strategic_assessment MUST be 2-3 sentences explaining what drives
        the score, referencing specific signals from the data.

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
              required: [region, hotspot_index, classification, strategic_assessment]
              properties:
                region: { type: string }
                lat: { type: number }
                lon: { type: number }
                hotspot_index: { type: integer, minimum: 0, maximum: 100 }
                classification:
                  type: string
                  enum: [stable, elevated, volatile, crisis, critical]
                conflict_score: { type: integer, minimum: 0, maximum: 100 }
                military_score: { type: integer, minimum: 0, maximum: 100 }
                strategic_assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 5000
      temperature: 0.2
      output:
        store_insights: true
        insight_type: "hotspot_index"
        results_path: "hotspots"
        websocket_push: true
        retention: "336h"
```

{{< callout type="info" title="results_path + entity_external_id" >}}
`results_path: "hotspots"` extracts each item from the `hotspots` array and stores it as a separate `ai_insight` row. If the LLM includes `entity_external_id` in each item, the engine links the insight to the source entity -- enabling "show all insights for Ukraine" queries.
{{< /callout >}}

{{< callout type="info" title="Retention controls table size" >}}
`retention: "336h"` (14 days) means insights older than two weeks are automatically deleted by the hourly cleanup goroutine. For a 5-minute analysis, this means ~4,000 insight rows per region before cleanup. Increase retention for audit trails; decrease it for high-frequency analyses.
{{< /callout >}}

### Complete definition

Save this as `analysis.d/geopolitical_hotspot_index.yaml`:

```yaml
# analysis.d/geopolitical_hotspot_index.yaml
# Cross-layer composite threat scoring: conflict + military signals.
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
    WITH region_signals AS (
      SELECT
        conf.id, conf.name AS region_name, conf.external_id,
        o_conf.lat, o_conf.lon, o_conf.position,
        CAST(COALESCE(conf.metadata->>'fatalities', '0') AS int) AS fatalities,
        CAST(COALESCE(conf.metadata->>'events', '0') AS int) AS event_count
      FROM entities conf
      CROSS JOIN LATERAL (
        SELECT lat, lon, position
        FROM observations
        WHERE entity_id = conf.id
        ORDER BY ts DESC LIMIT 1
      ) o_conf
      WHERE conf.layer_type = 'conflict_events'
        AND CAST(COALESCE(conf.metadata->>'fatalities', '0') AS int) > 0
    ),
    military_presence AS (
      SELECT
        rs.region_name,
        COUNT(DISTINCT mil.id) AS military_aircraft_count,
        STRING_AGG(DISTINCT mil.metadata->>'type', ', ') AS aircraft_types
      FROM region_signals rs
      JOIN entities mil ON mil.layer_type = 'flights_military'
      CROSS JOIN LATERAL (
        SELECT position, ts
        FROM observations
        WHERE entity_id = mil.id
        ORDER BY ts DESC LIMIT 1
      ) o_mil
      WHERE o_mil.ts > NOW() - INTERVAL '24 hours'
        AND ST_DWithin(
            rs.position::geography,
            o_mil.position::geography,
            300000
        )
      GROUP BY rs.region_name
    )
    SELECT
      rs.id AS entity_id,
      rs.region_name AS name,
      rs.external_id AS external_id,
      'conflict_events' AS layer_type,
      rs.lat, rs.lon, 0.0 AS altitude_m,
      NOW() AS ts,
      rs.fatalities::text AS fatalities,
      rs.event_count::text AS event_count,
      COALESCE(mp.military_aircraft_count, 0)::text AS military_aircraft,
      COALESCE(mp.aircraft_types, 'none') AS aircraft_types
    FROM region_signals rs
    LEFT JOIN military_presence mp ON mp.region_name = rs.region_name
    WHERE rs.fatalities > 0
      AND (COALESCE(mp.military_aircraft_count, 0) > 0
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
        (conflict + military) over {{.Lookback}}.

        Multi-signal regional data:
        {{range .Records}}
        REGION: {{.EntityName}} ({{.Lat}}, {{.Lon}}, ID: {{.ExternalID}})
          Conflict: {{index .Metadata "fatalities"}} fatalities / {{index .Metadata "event_count"}} events
          Military: {{index .Metadata "military_aircraft"}} aircraft ({{index .Metadata "aircraft_types"}})
        {{end}}

        For each region, compute a composite Hotspot Index (0-100) using
        these weighted signals:
        - Conflict intensity (fatalities + events): 60% weight
        - Military presence (aircraft count + type significance): 40% weight

        CALIBRATION:
        - critical (81-100): Active large-scale war, 1000+ fatalities, confirmed military ops
        - crisis (61-80): Sustained armed conflict, 100-1000 fatalities, military escalation signals
        - volatile (41-60): Ongoing low-intensity conflict, military posturing
        - elevated (21-40): Chronic instability, intermittent violence
        - stable (0-20): Genuinely low risk, minimal recent violence

        CRITICAL: Do NOT classify a region as "stable" purely because its
        fatality count is low relative to active war zones. Use your world
        knowledge of the region's chronic instability.

        The strategic_assessment MUST be 2-3 sentences explaining what drives
        the score, referencing specific signals from the data.

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
              required: [region, hotspot_index, classification, strategic_assessment]
              properties:
                region: { type: string }
                lat: { type: number }
                lon: { type: number }
                hotspot_index: { type: integer, minimum: 0, maximum: 100 }
                classification:
                  type: string
                  enum: [stable, elevated, volatile, crisis, critical]
                conflict_score: { type: integer, minimum: 0, maximum: 100 }
                military_score: { type: integer, minimum: 0, maximum: 100 }
                strategic_assessment: { type: string }
                entity_external_id: { type: string }
      max_tokens: 5000
      temperature: 0.2
      output:
        store_insights: true
        insight_type: "hotspot_index"
        results_path: "hotspots"
        websocket_push: true
        retention: "336h"
```
