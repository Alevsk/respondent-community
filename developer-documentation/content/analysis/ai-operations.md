---
title: "AI Operations"
description: "Configure LLM-powered analysis operations with structured output"
weight: 2
---

The `ai` section defines one or more LLM operations that run on the fetched data. Each operation receives the records from the data phase, renders a prompt template, calls the configured LLM provider, validates the response against a JSON Schema, and stores or pushes the results.

```yaml
ai:
  enabled: true
  operations:
    - name: anomaly_detection
      tags: [seismic, anomaly]
      prompt: |
        Analyze these {{.RecordCount}} records...
      output_schema:
        type: object
        required: [summary]
        properties:
          summary: { type: string }
      max_tokens: 2000
      temperature: 0
      output:
        store_insights: true
        insight_type: "anomaly"
        websocket_push: true
```

---

## Operation structure

Each operation in the `operations` array defines a complete LLM interaction.

{{< field name="name" type="string" required="true" >}}
Unique name for this operation within the analysis definition. Used in logs and metrics. Example: `"hotspot_index_scoring"`, `"anomaly_detection"`.
{{< /field >}}

{{< field name="tags" type="string[]" required="false" >}}
Metadata labels for filtering and searching analysis definitions. Used by the UI and API to categorize insights. Not consumed by the engine. Example: `[osint, geopolitical, cross-layer]`.
{{< /field >}}

{{< field name="max_tokens" type="integer" required="false" >}}
LLM response token budget. Set this high enough to avoid truncation but low enough to control cost. 2000 is usually sufficient for single-record analyses. For multi-record analyses producing per-record assessments, use 4000-5000.
{{< /field >}}

{{< field name="temperature" type="float" required="false" default="0" >}}
LLM creativity dial. Range: 0.0 (deterministic) to 1.0 (creative). Use `0` or `0.1` for scoring and classification tasks where reproducibility matters. Use `0.3`-`0.5` for open-ended narrative generation.
{{< /field >}}

{{< field name="cache_ttl" type="duration" required="false" >}}
Cache LLM responses for this duration to avoid redundant calls. Example: `"1h"`, `"24h"`. If the same input data produces the same prompt, the cached response is returned without an LLM call.
{{< /field >}}

---

## Prompt template syntax

Prompts use Go `text/template` syntax. The engine renders the template with the fetched data before sending it to the LLM.

### Basic template

```yaml
prompt: |
  Analyze {{.RecordCount}} records from the last {{.Lookback}}:

  {{range .Records}}
  - {{.EntityName}} at {{.Lat}}, {{.Lon}} ({{.Timestamp}})
    Magnitude: {{index .Metadata "magnitude"}}
  {{end}}

  Respond with JSON matching this schema:
  {{.OutputSchema}}
```

### Available variables

These are the same template variables described in [Data Queries](../data-queries/):

- `{{.RecordCount}}` -- total records
- `{{.Lookback}}` -- time window as a duration string
- `{{.Records}}` -- iterate with `{{range .Records}}...{{end}}`
- `{{.OutputSchema}}` -- auto-injected JSON Schema string

Inside `{{range .Records}}`:

- `{{.EntityName}}`, `{{.ExternalID}}`, `{{.LayerType}}`
- `{{.Lat}}`, `{{.Lon}}`, `{{.Altitude}}`, `{{.Timestamp}}`
- `{{index .Metadata "key"}}` -- access metadata fields by key

### Metadata access

Use `{{index .Metadata "key"}}` to access metadata fields. The key corresponds to the entity metadata field name (for layer-based queries) or the SQL column name (for SQL-based queries).

```yaml
prompt: |
  Region: {{.EntityName}}
  Fatalities: {{index .Metadata "fatalities"}}
  Military aircraft: {{index .Metadata "military_aircraft"}}
  Disaster count: {{index .Metadata "disaster_count"}}
```

{{< callout type="warning" title="Missing metadata keys" >}}
If a metadata key does not exist on a record, the template will render an empty string. Ensure your SQL uses `COALESCE` to fill in defaults for optional columns, or structure your prompt to handle blank values gracefully.
{{< /callout >}}

### Conditional blocks

Use Go template conditionals for optional sections:

```yaml
prompt: |
  {{if gt .RecordCount 10}}
  Large dataset detected. Focus on the top 5 highest-severity records.
  {{end}}

  {{range .Records}}
  {{.EntityName}}:
    {{if index .Metadata "severity"}}Severity: {{index .Metadata "severity"}}{{end}}
  {{end}}
```

{{< callout type="tip" title="Prompt engineering for consistent output" >}}
Structure your prompts with these patterns for reliable LLM responses:

1. **Role statement** -- Tell the LLM what role it plays ("You are a senior geopolitical risk analyst...")
2. **Data block** -- Present the records in a structured, scannable format
3. **Scoring rubric** -- Define explicit thresholds and calibration anchors so the LLM produces consistent scores across runs
4. **Output instruction** -- End with "Respond with JSON matching this schema: {{.OutputSchema}}"
{{< /callout >}}

{{< callout type="tip" title="Keep prompts scannable" >}}
Present records in a line-oriented format, not as paragraphs. The LLM processes structured data more reliably:

```
REGION: Ukraine (48.37, 37.62)
  Conflict: 125 fatalities / 902 events
  Military: 14 aircraft (K35R, R135)
```

is better than:

```
Ukraine is located at 48.37, 37.62 and has experienced 125 fatalities across 902 events with 14 military aircraft including K35R and R135.
```
{{< /callout >}}

---

## Output schema

The `output_schema` defines a JSON Schema that the LLM response is validated against. If validation fails, the insight is rejected and the error is logged.

```yaml
output_schema:
  type: object
  required: [severity, summary]
  properties:
    severity:
      type: string
      enum: [low, moderate, high, critical]
    summary:
      type: string
    score:
      type: integer
      minimum: 0
      maximum: 100
    affected_regions:
      type: array
      items:
        type: object
        required: [name, score]
        properties:
          name: { type: string }
          score: { type: integer, minimum: 0, maximum: 100 }
          entity_external_id: { type: string }
```

### Schema design guidelines

- Use `required` to enforce critical fields -- the LLM occasionally omits optional fields.
- Use `enum` for classification fields (`severity`, `category`, `trend`) to constrain the LLM to valid values and prevent hallucinated categories.
- Use `minimum`/`maximum` for bounded numeric scores to keep values in range.
- Include an `entity_external_id` field when you want the engine to link insights back to source entities via the `ai_insight_refs` table.

{{< callout type="tip" title="The OutputSchema variable" >}}
End your prompt with `Respond with JSON matching this schema: {{.OutputSchema}}`. The engine injects the JSON Schema string automatically, so the LLM knows exactly what structure to produce. This works better than describing the format in natural language.
{{< /callout >}}

---

## Output configuration

The `output` section controls how validated LLM responses are stored and delivered.

```yaml
output:
  store_insights: true
  insight_type: "hotspot_index"
  websocket_push: true
  retention: "168h"
  results_path: "results"
```

{{< field name="output.store_insights" type="boolean" required="false" default="false" >}}
When true, the engine persists results to the `ai_insights` table. Set to `false` for dry-run or debug definitions where you want to test prompts without writing to the database.
{{< /field >}}

{{< field name="output.insight_type" type="string" required="false" >}}
Type label stored in `ai_insights.insight_type`. Used by the frontend and API to query and filter insights by category. Keep these stable across deployments -- changing them breaks existing frontend queries. Example: `"hotspot_index"`, `"risk_assessment"`, `"anomaly"`.
{{< /field >}}

{{< field name="output.websocket_push" type="boolean" required="false" default="false" >}}
When true and a notifier is configured, each stored insight is pushed to connected WebSocket clients as an `"ai_insight"` message. This enables real-time dashboard updates without polling.
{{< /field >}}

{{< field name="output.retention" type="duration" required="false" >}}
How long insights are kept before automatic cleanup. The engine runs an hourly cleanup goroutine that deletes expired insights. Example: `"168h"` (7 days), `"336h"` (14 days), `"720h"` (30 days). Use shorter retention for high-frequency analyses to prevent table bloat.
{{< /field >}}

{{< field name="output.results_path" type="string" required="false" >}}
JSON path to an array field in the LLM response. When set, the engine iterates the array and stores **one `ai_insight` per array item**. If the array is empty, a single summary insight is stored instead (the full LLM response). If omitted, the entire LLM response is stored as a single insight.
{{< /field >}}

### results_path behavior

When your LLM returns a response like:

```json
{
  "global_summary": "Three regions show elevated threat levels...",
  "hotspots": [
    { "region": "Ukraine", "score": 92, "classification": "critical" },
    { "region": "Sudan", "score": 78, "classification": "crisis" }
  ]
}
```

Setting `results_path: "hotspots"` tells the engine to extract the `hotspots` array and store each item as a separate `ai_insight` row. This gives you per-region insights that can be queried independently.

{{< callout type="info" title="Single vs. array insights" >}}
Omit `results_path` when your analysis produces a single overall assessment (e.g., "is this entity anomalous?"). Set `results_path` when your analysis produces a list of scored items (e.g., "score each of these 15 regions"). The per-item storage makes it possible to query "show me all regions with score > 80" from the API.
{{< /callout >}}

### Entity linking

When your output schema includes an `entity_external_id` field and you have `data.layers` configured, the engine resolves each external ID to a database entity UUID and creates an `ai_insight_refs` row. This links the insight to the source entity, enabling "show all insights for this entity" queries in the frontend.

{{< callout type="info" title="SQL-path analyses skip entity linking" >}}
For SQL-based analyses without `data.layers`, entity external ID resolution is skipped. This is expected behavior -- the debug log message "no layers configured for external ID resolution" is harmless.
{{< /callout >}}
