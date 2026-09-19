---
title: "Analysis Schema Reference"
description: "Complete YAML schema for analysis definitions"
weight: 3
---

Analysis definitions live in `analysis.d/*.yaml` and define scheduled AI analysis jobs that run against ingested data. They share the same AI operation format as source definitions but add scheduling and data query configuration.

```yaml
schema_version: 1
name: anomaly_detection
display_name: "Anomaly Detection"
enabled: true

schedule:
  interval: "5m"

data:
  lookback: "1h"
  min_records: 10
  max_records: 500
  layers:
    - flights_commercial
    - ships

ai:
  enabled: true
  operations:
    - name: detect_anomalies
      prompt: >
        Analyze the following {{ .RecordCount }} records from the last {{ .Lookback }}...
      output_schema:
        type: object
        properties:
          anomalies:
            type: array
      max_tokens: 2048
      temperature: 0.0
```

## Top-level

{{< field name="schema_version" type="integer" required="false" >}}
Schema version number.
{{< /field >}}

{{< field name="name" type="string" required="true" >}}
Unique analysis definition name. Used as the key in the definition registry and in schema registry keys.
{{< /field >}}

{{< field name="display_name" type="string" required="false" >}}
Human-readable name for display in the UI.
{{< /field >}}

{{< field name="enabled" type="boolean" required="false" default="false" >}}
Whether this analysis job is active. When `false`, the definition is loaded but the scheduler does not run it.
{{< /field >}}

## Schedule

Defines when the analysis job runs. Exactly one of `interval` or `cron` must be set.

{{< field name="schedule.interval" type="string" required="false" >}}
Go duration string defining the run frequency (e.g., `"5m"`, `"1h"`, `"30s"`). Mutually exclusive with `cron`.
{{< /field >}}

{{< field name="schedule.cron" type="string" required="false" >}}
Cron expression defining the run schedule (e.g., `"*/5 * * * *"` for every 5 minutes). Mutually exclusive with `interval`.
{{< /field >}}

{{< callout type="warning" title="Mutually exclusive" >}}
You must set exactly one of `interval` or `cron`. Setting both or neither causes a validation error.
{{< /callout >}}

## Data

Defines what data to fetch for analysis. The engine queries the database for entity+observation records matching these criteria and passes them to the AI prompt template.

{{< field name="data.lookback" type="string" required="true" >}}
Time window for data queries as a Go duration string (e.g., `"1h"`, `"24h"`). Only records with timestamps within this window from the current time are included.
{{< /field >}}

{{< field name="data.min_records" type="integer" required="false" default="0" >}}
Minimum number of records required before the analysis runs. If fewer records are available, the job is skipped for that cycle.
{{< /field >}}

{{< field name="data.max_records" type="integer" required="false" default="500" >}}
Cap on the number of records sent to the LLM prompt. When `0`, the default of 500 is used.
{{< /field >}}

{{< field name="data.layers" type="string[]" required="false" >}}
Layer types to query. When empty, records from all layers are included.
{{< /field >}}

{{< field name="data.filter" type="string" required="false" >}}
CEL expression for pre-filtering records before they are sent to the LLM. The expression has access to `entity` and `observation` map variables.
{{< /field >}}

{{< field name="data.sql" type="string" required="false" >}}
Raw SQL query for advanced data retrieval. Validated read-only at **run time** -- on each execution the engine calls `aiquery.ValidateReadOnlySQL` (in `fetchDataFromSQL`) and aborts the run if the query is not read-only. It is not checked at load time, and a definition that sets `data.sql` fails its run if no query executor is configured.
{{< /field >}}

### Dedup

Optional deduplication configuration. When set, records that already have recent insights (within the window) are filtered out before calling the LLM, preventing duplicate analysis.

{{< field name="data.dedup.window" type="string" required="true" >}}
Go duration string defining the dedup window (e.g., `"2h"`). Records with existing insights newer than this are excluded.
{{< /field >}}

{{< field name="data.dedup.key_fields" type="string[]" required="true" >}}
Fields used as the composite dedup key. Min length: 1.
{{< /field >}}

## AI Operations

The `ai` section uses the same `SourceAIConfig` structure as source definitions. See the [Source Schema Reference]({{< relref "/reference/source-schema#ai" >}}) for the full field reference on `ai.enabled` and `ai.operations[]`.

### Output Noise Control and Cross-Layer Refs

Two `operations[].output` fields are specific to analysis insight storage and are not covered by the source-schema delegation above.

{{< field name="operations[].output.min_attention" type="string" required="false" >}}
Minimum attention level a result must carry to be stored and pushed. One of `info`, `low`, `medium`, `high`, `critical` (validator `oneof`). Results below this floor -- including absent or unclassified attention, treated as `info` -- are dropped. When `results_path` resolves to an empty array, the all-clear summary insight is suppressed if a floor is configured. When unset, the engine-wide `ai.min_attention` default applies (community default: `low`).
{{< /field >}}

{{< field name="operations[].output.ref_fields" type="object[]" required="false" >}}
Cross-layer entity references. Each entry maps a field in the LLM result item (whose value is an `external_id`) to an entity in another layer, creating an extra `ai_insight_refs` row. Both keys are required per entry:

- `field` (string, required) -- result-item property holding the external ID
- `layer_type` (string, required) -- layer to scope the `external_id` lookup

```yaml
output:
  results_path: "assessments"
  ref_fields:
    - field: "conflict_external_id"
      layer_type: "conflict_events"
```
{{< /field >}}

### Prompt Template Variables

Analysis operation prompts receive an `AnalysisPromptData` context with these variables:

| Variable | Type | Description |
|----------|------|-------------|
| `.RecordCount` | `int` | Total number of records in the query result |
| `.Lookback` | `string` | The lookback duration string |
| `.Records` | `[]AnalysisRecord` | Array of entity+observation records |
| `.LayerCount` | `int` | Number of distinct layer types in the results |
| `.LayerStats` | `[]LayerStat` | Per-layer statistics |
| `.OutputSchema` | `string` | The JSON schema string for structured output |

Each `AnalysisRecord` contains:

| Field | Type | Description |
|-------|------|-------------|
| `EntityID` | `string` | Internal entity ID |
| `EntityName` | `string` | Human-readable entity name |
| `ExternalID` | `string` | External ID from the source |
| `LayerType` | `string` | Layer type this record belongs to |
| `Lat` | `float64` | Latitude |
| `Lon` | `float64` | Longitude |
| `Altitude` | `float64` | Altitude in meters |
| `Metadata` | `map[string]string` | Merged entity + observation metadata |
| `EntityMetadata` | `map[string]string` | Entity-only metadata |
| `ObsMetadata` | `map[string]string` | Observation-only metadata |
| `Timestamp` | `string` | Observation timestamp |

### CEL Filter Environment

Analysis CEL filters use a different environment from source filters. The available variables are:

| Variable | Type | Description |
|----------|------|-------------|
| `entity` | `map[string]dyn` | Entity fields including metadata |
| `observation` | `map[string]dyn` | Observation fields including metadata |

The string extension library is available. Math and time custom functions from source CEL are not available in analysis filters.
