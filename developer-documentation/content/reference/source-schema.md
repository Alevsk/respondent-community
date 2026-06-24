---
title: "Source Schema Reference"
description: "Complete YAML schema for source definitions -- every field, type, and default"
weight: 2
---

This page documents every field in a source definition YAML file (`sources.d/*.yaml`). Fields are grouped by section. For transport-specific fields, see [Transports]({{< relref "/sources/transports" >}}).

## Top-level

{{< field name="schema_version" type="integer" required="true" >}}
Schema version. Use `2` for new sources. Version 2 adds support for filtering, geo_cache, spatial crawling, entity_cache, on_demand_url, and advanced parser options. Allowed values: `1`, `2`.
{{< /field >}}

{{< field name="name" type="string" required="true" >}}
Unique source identifier. Lowercase letters, digits, and underscores only. Must start with a letter. Max 64 characters. Pattern: `^[a-z][a-z0-9_]{0,63}$`.
{{< /field >}}

{{< field name="source_type" type="string" required="true" >}}
Maps to the domain SourceType for the ingest registry. Convention: same as `name` unless you have a reason to differ. Same naming rules as `name`.
{{< /field >}}

{{< field name="layer_type" type="string" required="true" >}}
Groups entities on the globe. Multiple sources can feed one layer. Same naming rules as `name`.
{{< /field >}}

{{< field name="display_name" type="string" required="true" >}}
Human-readable name for the SOURCE (shown in source listings, logs, and AI prompts). This names the source, not the layer-picker label.
{{< /field >}}

{{< field name="layer_display_name" type="string" required="false" >}}
Overrides the human-readable LAYER label shown in the UI layers panel. When omitted, the layer is labeled by title-casing `layer_type` (`FormatLayerName`, e.g. `traffic_stations` -> `Traffic Stations`). Set it when the title-cased `layer_type` loses meaning -- for example a country qualifier: `"Traffic Stations (Mexico)"`. Distinct from `display_name`, which names the source.
{{< /field >}}

{{< field name="entity_type" type="string" required="false" default="geo_entity" >}}
Discriminates between geographic and non-geographic entities. `geo_entity` has lat/lon coordinates. `global_indicator` has no coordinates and is rendered as a HUD overlay panel. Allowed values: `geo_entity`, `global_indicator`.
{{< /field >}}

{{< field name="enabled" type="boolean" required="false" default="true" >}}
Set to `false` to disable this source without deleting the file. When omitted or `null`, defaults to `true`.
{{< /field >}}

{{< field name="dry_run" type="boolean" required="false" default="false" >}}
When `true`, the source fetches and parses data but does not persist to the database or cache. Useful for testing new sources.
{{< /field >}}

{{< field name="labels" type="map[string]string" required="false" >}}
Optional key-value labels for categorization and filtering. Common keys: `category`, `priority`.
{{< /field >}}

{{< field name="filtering" type="string" required="false" >}}
Controls how the frontend subscribes to this layer (schema_version 2 only). `viewport` sends camera bounds and uses GEOSEARCH. `all` explicitly fetches all entities. When omitted, the frontend subscribes in global mode. Allowed values: `viewport`, `all`.
{{< /field >}}

## Transport

{{< field name="transport.type" type="string" required="true" >}}
Transport type. Allowed values: `http_poll`, `websocket`, `sse`, `mqtt`, `webhook`, `pubsub`, `grpc_stream`, `kafka`, `amqp`, `tcp_udp`, `ftp_sftp`, `s3_poll`, `nats`.
{{< /field >}}

{{< field name="transport.url" type="string" required="false" >}}
The API endpoint URL. Supports `${VAR}` for environment variable substitution (resolved from `RESPONDENT_VAR`). For spatial sources, use `{lat}` and `{lon}` placeholders.
{{< /field >}}

{{< field name="transport.on_demand_url" type="string" required="false" >}}
Alternative URL for single-point fetches (schema_version 2 only). Supports `{lat}` and `{lon}` placeholders. Must be a valid URL.
{{< /field >}}

{{< field name="transport.method" type="string" required="false" default="GET" >}}
HTTP method. Allowed values: `GET`, `POST`.
{{< /field >}}

{{< field name="transport.headers" type="map[string]string" required="false" >}}
Static headers sent with every request.
{{< /field >}}

{{< field name="transport.timeout" type="duration" required="false" >}}
HTTP request timeout (e.g., `"15s"`).
{{< /field >}}

{{< field name="transport.interval" type="duration" required="false" >}}
Polling interval -- how often to re-fetch data (e.g., `"60s"`).
{{< /field >}}

{{< field name="transport.max_response_bytes" type="integer" required="false" >}}
Maximum response body size in bytes. Min: `1`, max: `104857600` (100 MB).
{{< /field >}}

### Auth

{{< field name="transport.auth.type" type="string" required="true" >}}
Authentication type. Allowed values: `bearer`, `api_key`, `oauth2`.
{{< /field >}}

{{< field name="transport.auth.header" type="string" required="false" >}}
Custom header name for `api_key` auth type.
{{< /field >}}

{{< field name="transport.auth.env_var" type="string" required="false" >}}
Environment variable name (without the `RESPONDENT_` prefix). Resolved from `RESPONDENT_<env_var>` at runtime. Required for `bearer` and `api_key` types.
{{< /field >}}

{{< field name="transport.auth.oauth2" type="object" required="false" >}}
OAuth2 token exchange configuration. Required when `type` is `oauth2`. See the [Transports]({{< relref "/sources/transports" >}}) page for full OAuth2 field reference.
{{< /field >}}

### Retry

{{< field name="transport.retry.max_attempts" type="integer" required="false" >}}
Number of retry attempts. Range: `0`-`10`. Total attempts = `max_attempts + 1`.
{{< /field >}}

{{< field name="transport.retry.backoff" type="string" required="false" >}}
Backoff strategy. Allowed values: `exponential`, `fixed`.
{{< /field >}}

{{< field name="transport.retry.initial_delay" type="duration" required="false" >}}
Starting delay between retries (e.g., `"2s"`).
{{< /field >}}

{{< field name="transport.retry.max_delay" type="duration" required="false" >}}
Maximum delay between retries (e.g., `"30s"`).
{{< /field >}}

### Pagination

{{< field name="transport.pagination.type" type="string" required="true" >}}
Pagination strategy. Allowed values: `page_number`, `offset`, `cursor`.
{{< /field >}}

{{< field name="transport.pagination.page_param" type="string" required="true" >}}
Query parameter name for the page number, offset, or cursor.
{{< /field >}}

{{< field name="transport.pagination.size_param" type="string" required="false" >}}
Query parameter name for page size or limit.
{{< /field >}}

{{< field name="transport.pagination.size" type="integer" required="true" >}}
Items per page. Range: `1`-`10000`.
{{< /field >}}

{{< field name="transport.pagination.max_pages" type="integer" required="true" >}}
Stop after this many pages. Range: `1`-`1000`.
{{< /field >}}

{{< field name="transport.pagination.stop_when" type="string" required="false" >}}
CEL expression evaluated per page. The `records` variable is bound to the current page array. When `true`, pagination stops early.
{{< /field >}}

{{< field name="transport.pagination.cursor_path" type="string" required="false" >}}
Dot-path to extract the next cursor value from the JSON response (for `cursor` type).
{{< /field >}}

## Parser

{{< field name="parser.format" type="string" required="true" >}}
Response format. Allowed values: `json`, `json_table`, `geojson`, `csv`, `tle`, `xml`, `rss`.
{{< /field >}}

{{< field name="parser.records_path" type="string" required="false" >}}
Dot-separated path to the array of records in the response. Max depth: 8 segments. Pattern: `^[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+){0,7}$`. Omit when the response is already the array.
{{< /field >}}

{{< field name="parser.max_records" type="integer" required="false" >}}
Safety cap on the number of records to process. Range: `1`-`100000`.
{{< /field >}}

### CSV Options

{{< field name="parser.csv_options.delimiter" type="string" required="false" default="," >}}
Field delimiter character.
{{< /field >}}

{{< field name="parser.csv_options.has_header" type="boolean" required="false" default="false" >}}
Whether the first row contains column headers.
{{< /field >}}

{{< field name="parser.csv_options.skip_lines" type="integer" required="false" default="0" >}}
Number of lines to skip before parsing.
{{< /field >}}

{{< field name="parser.csv_options.collapse_whitespace" type="boolean" required="false" default="false" >}}
Collapse consecutive whitespace in field values.
{{< /field >}}

{{< field name="parser.csv_options.comment_prefix" type="string" required="false" >}}
Lines starting with this string are ignored.
{{< /field >}}

### JSON Reshaping (schema_version 2)

{{< field name="parser.array_columns" type="string[]" required="false" >}}
Map array indices to named fields. Use when the API returns arrays of arrays instead of objects (e.g., OpenSky returns `[icao24, callsign, ...]`). The Nth entry becomes the field name for index N.
{{< /field >}}

{{< field name="parser.object_to_records" type="boolean" required="false" default="false" >}}
Convert a top-level JSON object `{"key": {...}, ...}` into an array of its values `[{...}, ...]`. Useful for dictionary-keyed APIs (e.g., SondeHub).
{{< /field >}}

{{< field name="parser.object_key_field" type="string" required="false" >}}
When `object_to_records` is `true`, inject the object key into each record under this field name.
{{< /field >}}

{{< field name="parser.array_of_arrays" type="boolean" required="false" default="false" >}}
Treat a JSON array-of-arrays as a table where the first row is column headers and subsequent rows are data. Converts to an array of objects. Useful for APIs like NOAA SWPC.
{{< /field >}}

## Filter

{{< field name="filter" type="string" required="false" >}}
CEL expression that must return `true` for a record to be processed. The `record` variable is bound to each parsed record. Use `has()` to guard optional fields. See [CEL Functions]({{< relref "/reference/cel-functions" >}}) for available functions.
{{< /field >}}

## Entity

{{< field name="entity.external_id" type="string (CEL)" required="true" >}}
CEL expression returning a unique identifier for this entity within the layer. Must be stable across polls. Examples: `record.id`, `string(record.eventid)`, `record.lat + "_" + record.lon`.
{{< /field >}}

{{< field name="entity.name" type="string (CEL)" required="true" >}}
CEL expression returning a human-readable name for display in the UI.
{{< /field >}}

{{< field name="entity.metadata" type="map[string]string (CEL)" required="false" >}}
Map of metadata key to CEL expression. Each expression must return a string. These appear in the entity detail panel.
{{< /field >}}

## Observation

{{< field name="observation.latitude" type="string (CEL)" required="true" >}}
CEL expression returning latitude as a `double`. Required for `geo_entity`, omit for `global_indicator`.
{{< /field >}}

{{< field name="observation.longitude" type="string (CEL)" required="true" >}}
CEL expression returning longitude as a `double`. Required for `geo_entity`, omit for `global_indicator`.
{{< /field >}}

{{< field name="observation.altitude" type="string (CEL)" required="false" default="0.0" >}}
CEL expression returning altitude in meters as a `double`.
{{< /field >}}

{{< field name="observation.timestamp" type="string (CEL)" required="true" >}}
CEL expression returning a CEL `timestamp`. See time functions in [CEL Functions]({{< relref "/reference/cel-functions" >}}).
{{< /field >}}

{{< field name="observation.event_time" type="string (CEL)" required="false" >}}
CEL expression returning a timestamp for the start of a time-bounded event (e.g., weather alerts, conflict events).
{{< /field >}}

{{< field name="observation.event_end" type="string (CEL)" required="false" >}}
CEL expression returning a timestamp for the end of a time-bounded event.
{{< /field >}}

{{< field name="observation.velocity" type="map[string]string (CEL)" required="false" >}}
Map of velocity component to CEL expression. Each expression must return a string. Common keys: `speed`, `heading`, `climb`.
{{< /field >}}

{{< field name="observation.metadata" type="map[string]string (CEL)" required="false" >}}
Map of metadata key to CEL expression. Each expression must return a string.
{{< /field >}}

{{< field name="observation.content_hash" type="string (CEL)" required="false" >}}
CEL expression returning a unique string for deduplication. Required when `recording.mode` is `dedupe`.
{{< /field >}}

## Recording

{{< field name="recording.mode" type="string" required="true" >}}
How observations are persisted. Allowed values:

- `append` -- always insert a new row (time-series style)
- `upsert` -- insert or update based on entity_id (latest-only)
- `dedupe` -- insert only if `content_hash` differs from the latest observation (requires `observation.content_hash`)
{{< /field >}}

## Cache

{{< field name="cache.ttl" type="duration" required="true" >}}
TTL for the hot cache (Valkey in production, MemCache in community edition). Controls how long entities remain visible on the globe after the last fetch. Set higher than `transport.interval` to survive missed polls.
{{< /field >}}

## Entity Cache (schema_version 2)

Accumulates entities across spatial batches. Different from `cache.ttl` -- this controls in-memory accumulation for cross-batch deduplication.

{{< field name="entity_cache.enabled" type="boolean" required="false" default="false" >}}
Enable entity-level caching for spatial sources.
{{< /field >}}

{{< field name="entity_cache.key" type="string (CEL)" required="true" >}}
CEL expression for the cache key.
{{< /field >}}

{{< field name="entity_cache.ttl" type="duration" required="true" >}}
How long to keep an entity in the accumulation cache.
{{< /field >}}

{{< field name="entity_cache.accumulate" type="boolean" required="false" default="false" >}}
Merge entities across spatial batches.
{{< /field >}}

## Geo Cache

Controls geo sorted set maintenance for viewport-filtered layers. Required alongside `filtering: viewport`.

{{< field name="geo_cache.overfetch_ratio" type="float" required="false" >}}
Fetch this multiple of members to find enough alive ones. Range: `1.0`-`10.0`.
{{< /field >}}

{{< field name="geo_cache.alive_ratio_threshold" type="float" required="false" >}}
Trigger GC when fewer than this ratio of results are alive. Range: `0.1`-`1.0`.
{{< /field >}}

{{< field name="geo_cache.gc_batch_size" type="integer" required="false" >}}
Max dead members to prune per query. Range: `10`-`10000`.
{{< /field >}}

## Backfill (schema_version 2)

{{< field name="backfill.threshold" type="integer" required="false" >}}
Trigger backfill when a spatial region has fewer entities than this threshold. Range: `1`-`10000`.
{{< /field >}}

## Display

### Icon

{{< field name="display.icon.shape" type="string" required="true" >}}
Icon shape for entity rendering. See [Icon Shapes]({{< relref "/reference/icon-shapes" >}}) for the full list of 57 available shapes.
{{< /field >}}

{{< field name="display.icon.rotatable" type="boolean" required="false" default="false" >}}
Rotate the icon based on entity heading. Enable for directional entities like aircraft and ships.
{{< /field >}}

{{< field name="display.icon.interpolation" type="boolean" required="false" default="false" >}}
Interpolate position between updates for smooth movement.
{{< /field >}}

{{< field name="display.icon.scale" type="float" required="false" default="1.0" >}}
Size multiplier. Range: `0.1`-`5.0`. `1.0` is the standard size.
{{< /field >}}

### Trail

{{< field name="display.trail.color" type="string" required="true" >}}
Hex color for the entity trail (e.g., `"#ffffff"`). Must be a valid hex color.
{{< /field >}}

{{< field name="display.trail.width" type="float" required="false" >}}
Trail line width. Range: `0.5`-`10.0`.
{{< /field >}}

{{< field name="display.trail.opacity" type="float" required="false" >}}
Trail transparency. Range: `0.0`-`1.0`.
{{< /field >}}

### Style

{{< field name="display.style.color" type="string" required="true" >}}
Entity color on the globe (e.g., `"#ffffff"`). Must be a valid hex color.
{{< /field >}}

{{< field name="display.style.point_size" type="integer" required="true" >}}
Base point size. Range: `1`-`32`. `8` is the standard size.
{{< /field >}}

### Field Renderers

An array defining how metadata fields are displayed in the entity detail panel. Renderers are sorted by priority (lower = shown first).

{{< field name="display.field_renderers[].keys" type="string[]" required="true" >}}
Metadata key(s) to match. Min length: 1. Matches keys from `entity.metadata` or `observation.metadata`.
{{< /field >}}

{{< field name="display.field_renderers[].label" type="string" required="true" >}}
Display label shown in the entity detail panel.
{{< /field >}}

{{< field name="display.field_renderers[].format.type" type="string" required="true" >}}
Format type. Allowed values: `string`, `float`, `integer`, `raw`.
{{< /field >}}

{{< field name="display.field_renderers[].format.precision" type="integer" required="false" >}}
Decimal places for `float` format type.
{{< /field >}}

{{< field name="display.field_renderers[].format.prefix" type="string" required="false" >}}
String prepended to the formatted value.
{{< /field >}}

{{< field name="display.field_renderers[].format.suffix" type="string" required="false" >}}
String appended to the formatted value.
{{< /field >}}

{{< field name="display.field_renderers[].format.transform" type="string" required="false" >}}
Text transform for `string` format type. Allowed values: `upper`, `lower`.
{{< /field >}}

{{< field name="display.field_renderers[].priority" type="integer" required="false" default="0" >}}
Sort order. Lower values are shown first.
{{< /field >}}

### Color By

Colors entities by the value of a metadata field. Overrides `display.style.color` per entity based on the matched value. When present, the client reads this map instead of applying a single base color to every entity.

{{< field name="display.color_by.field" type="string" required="true" >}}
Metadata field name whose value selects the color.
{{< /field >}}

{{< field name="display.color_by.values" type="map[string]string" required="true" >}}
Map of field value to hex color (e.g., `operational: "#00ff9d"`). Min 1 entry. Each value must be a valid hex color.
{{< /field >}}

{{< field name="display.color_by.default_color" type="string" required="false" >}}
Hex color used when the field value is not present in `values`. Must be a valid hex color.
{{< /field >}}

## History

Per-layer time range limits for historical exploration. When present, overrides the server defaults (48h lookback, 24h span).

{{< field name="history.max_lookback" type="duration" required="false" >}}
Maximum historical lookback (e.g., `"8760h"` for 1 year).
{{< /field >}}

{{< field name="history.max_range_span" type="duration" required="false" >}}
Maximum time range span per query (e.g., `"168h"` for 1 week).
{{< /field >}}

## Indicator

Maps a global indicator entity's metadata fields to structured indicator values for HUD rendering. Only meaningful when `entity_type` is `global_indicator`.

{{< field name="indicator.summary_expr" type="string (CEL)" required="false" >}}
CEL expression that computes a human-readable summary. Variables: `level` (int), `metadata` (map[string]string). When omitted, a default severity scale (Quiet -> Extreme) is used.
{{< /field >}}

### Values (array)

{{< field name="indicator.values[].key" type="string" required="false" >}}
Identifier for this indicator reading.
{{< /field >}}

{{< field name="indicator.values[].label" type="string" required="false" >}}
Human-readable label for the reading.
{{< /field >}}

{{< field name="indicator.values[].source_field" type="string" required="false" >}}
Metadata key holding the raw value.
{{< /field >}}

{{< field name="indicator.values[].unit" type="string" required="false" >}}
Unit of measurement displayed with the value.
{{< /field >}}

{{< field name="indicator.values[].max_level" type="integer" required="false" >}}
Maximum severity level on the scale.
{{< /field >}}

{{< field name="indicator.values[].level_thresholds" type="float[]" required="false" >}}
Ascending thresholds used to compute the severity level from a raw value (when `level_expr` is omitted).
{{< /field >}}

{{< field name="indicator.values[].level_expr" type="string (CEL)" required="false" >}}
CEL expression that computes the severity level from a raw value. Variables: `value` (string), `change_pct` (string). Must return an int. When omitted, the level is computed from `level_thresholds`/`max_level`.
{{< /field >}}

{{< field name="indicator.values[].change_source_field" type="string" required="false" >}}
Metadata key holding the percentage change.
{{< /field >}}

{{< field name="indicator.values[].format" type="string" required="false" >}}
Value display format. Allowed values: `scale`, `number`, `currency`, `percent`.
{{< /field >}}

{{< field name="indicator.values[].precision" type="integer" required="false" >}}
Decimal places for the displayed value.
{{< /field >}}

{{< field name="indicator.values[].prefix" type="string" required="false" >}}
String prepended to the value (e.g., `"$"`).
{{< /field >}}

## Field Mappings

A top-level array of generalized CEL mappings from source data to domain structures.

{{< field name="field_mappings[].source" type="string (CEL)" required="true" >}}
CEL expression producing the value from the source record.
{{< /field >}}

{{< field name="field_mappings[].target" type="string" required="true" >}}
Destination field for the mapped value.
{{< /field >}}

{{< field name="field_mappings[].type" type="string" required="true" >}}
Value type. Allowed values: `timestamp`, `string`, `integer`, `float`, `boolean`.
{{< /field >}}

## Lookup Tables

An array of static lookup tables for CEL enrichment. Loaded and indexed at config time.

{{< field name="lookup_tables[].name" type="string" required="true" >}}
Unique table identifier. Same naming rules as source `name`.
{{< /field >}}

{{< field name="lookup_tables[].key_field" type="string" required="true" >}}
Primary key field name in each entry.
{{< /field >}}

{{< field name="lookup_tables[].entries" type="array" required="false" >}}
Inline entries. Each entry is a map with arbitrary fields. Use either `entries` or `file`, not both.
{{< /field >}}

{{< field name="lookup_tables[].file" type="string" required="false" >}}
Path to an external data file (relative to working directory). Use either `entries` or `file`, not both.
{{< /field >}}

{{< field name="lookup_tables[].format" type="string" required="false" >}}
File format when using `file`. Allowed values: `json`, `csv`.
{{< /field >}}

## AI

AI enrichment configuration. Optional. Requires `ai.enabled: true` in the server config and a configured LLM provider.

{{< field name="ai.enabled" type="boolean" required="false" default="false" >}}
Enable AI operations for this source.
{{< /field >}}

### Operations (array)

{{< field name="ai.operations[].name" type="string" required="true" >}}
Unique operation name.
{{< /field >}}

{{< field name="ai.operations[].tags" type="string[]" required="false" >}}
Optional tags for filtering and grouping.
{{< /field >}}

{{< field name="ai.operations[].filter" type="string" required="false" >}}
CEL expression to select which entities are processed.
{{< /field >}}

{{< field name="ai.operations[].prompt" type="string" required="true" >}}
LLM prompt template. Supports Go template syntax with `.Entity` and `.Observation` variables.
{{< /field >}}

{{< field name="ai.operations[].output_schema" type="object" required="false" >}}
JSON Schema for structured LLM output.
{{< /field >}}

{{< field name="ai.operations[].output_mapping" type="map[string]string" required="false" >}}
Map LLM result fields to entity or observation metadata keys.
{{< /field >}}

{{< field name="ai.operations[].output_target" type="string" required="false" default="entity" >}}
Where results are stored. Allowed values: `entity`, `observation`.
{{< /field >}}

{{< field name="ai.operations[].max_tokens" type="integer" required="false" >}}
Maximum tokens for the LLM response. Range: `1`-`128000`.
{{< /field >}}

{{< field name="ai.operations[].temperature" type="float" required="false" >}}
LLM temperature. Range: `0.0`-`2.0`.
{{< /field >}}

{{< field name="ai.operations[].cache_ttl" type="string" required="false" >}}
Cache LLM results for this duration to avoid re-processing (e.g., `"24h"`).
{{< /field >}}

{{< field name="ai.operations[].priority" type="integer" required="false" default="0" >}}
Operation priority. Lower values are processed first.
{{< /field >}}

### Operation Output

{{< field name="ai.operations[].output.store_insights" type="boolean" required="false" default="false" >}}
Store results as AI insights.
{{< /field >}}

{{< field name="ai.operations[].output.insight_type" type="string" required="false" >}}
Insight type identifier for querying.
{{< /field >}}

{{< field name="ai.operations[].output.websocket_push" type="boolean" required="false" default="false" >}}
Push results to the frontend via WebSocket.
{{< /field >}}

{{< field name="ai.operations[].output.retention" type="string" required="false" >}}
Insight retention period (e.g., `"720h"`).
{{< /field >}}

{{< field name="ai.operations[].output.results_path" type="string" required="false" >}}
Dot-path to the results in the LLM response.
{{< /field >}}
