---
title: "Field Mapping"
description: "Map parsed records to entities and observations using CEL expressions"
weight: 3
---

After parsing, each record is a flat or nested map of fields. Field mappings use [CEL (Common Expression Language)](https://cel.dev/) expressions to transform records into Respondent entities and observations. Every expression receives the current record as the `record` variable.

## Entity mapping

The `entity` section defines how each record becomes a domain entity. Entity mappings produce the identity and metadata for each data point.

```yaml
entity:
  external_id: >
    record.id
  name: >
    record.properties.name
  metadata:
    category: >
      has(record.category) ? record.category : "unknown"
```

### Fields

{{< field name="external_id" type="CEL -> string" required="true" >}}
Unique identifier within this layer. Must be stable across polls -- the same entity must produce the same `external_id` every time it is fetched. This is the key used for upsert and dedupe recording modes.
{{< /field >}}

{{< field name="name" type="CEL -> string" required="true" >}}
Human-readable name displayed in the UI entity detail panel.
{{< /field >}}

{{< field name="metadata" type="map[string]CEL -> string" required="false" >}}
Key-value pairs of additional entity data. All values must be CEL expressions that return strings. These appear in the entity detail panel and can be formatted by `display.field_renderers`.
{{< /field >}}

{{< callout type="warning" title="Metadata values must return strings" >}}
Every metadata value must be a CEL expression that evaluates to a string. Use `string()` to convert numeric fields: `string(record.magnitude)`. Returning a non-string type causes a runtime error.
{{< /callout >}}

### External ID patterns

The `external_id` must be unique and deterministic. Choose a pattern based on your data:

{{< cel-example expression="record.id" input="record.id = \"eq-2024-abc123\"" output="\"eq-2024-abc123\"" >}}
Direct field access -- use when the API provides a unique ID field.
{{< /cel-example >}}

{{< cel-example expression="string(record.eventid)" input="record.eventid = 12345" output="\"12345\"" >}}
Type coercion -- convert numeric IDs to strings with `string()`.
{{< /cel-example >}}

{{< cel-example expression="record.lat + \"_\" + record.lon + \"_\" + record.date" input="record.lat = \"39.7\", record.lon = \"-104.9\", record.date = \"2024-01-15\"" output="\"39.7_-104.9_2024-01-15\"" >}}
Composite key -- combine multiple fields when no single unique ID exists. Ensure the combination is stable across polls.
{{< /cel-example >}}

---

## Observation mapping

The `observation` section maps each record to a spatial observation -- a position-and-time snapshot attached to an entity.

```yaml
observation:
  latitude: >
    double(record.lat)
  longitude: >
    double(record.lon)
  altitude: "0.0"
  timestamp: >
    unix_ms(record.time)
  velocity:
    speed: >
      has(record.speed) ? double(record.speed) : 0.0
    heading: >
      has(record.heading) ? double(record.heading) : 0.0
  metadata:
    status: >
      has(record.status) ? record.status : ""
  content_hash: ""
```

### Position fields

{{< field name="latitude" type="CEL -> double" required="true" >}}
Latitude in decimal degrees. Must evaluate to a float64. Omit when `entity_type` is `global_indicator`.
{{< /field >}}

{{< field name="longitude" type="CEL -> double" required="true" >}}
Longitude in decimal degrees. Must evaluate to a float64. Omit when `entity_type` is `global_indicator`.
{{< /field >}}

{{< field name="altitude" type="CEL -> double" required="false" default="0.0" >}}
Altitude in meters above sea level. Use negative values for underground or underwater entities.
{{< /field >}}

#### Latitude and longitude patterns

{{< cel-example expression="double(record.lat)" input="record.lat = \"39.7392\"" output="39.7392" >}}
String-to-number conversion -- use `double()` when the API returns coordinates as strings.
{{< /cel-example >}}

{{< cel-example expression="record.geometry.coordinates[1]" input="record.geometry.coordinates = [-104.99, 39.74, 1609.0]" output="39.74" >}}
GeoJSON array access -- index `[0]` is longitude, `[1]` is latitude, `[2]` is altitude.
{{< /cel-example >}}

{{< cel-example expression="record.position.latitude" input="record.position = {\"latitude\": 39.74, \"longitude\": -104.99}" output="39.74" >}}
Nested object access -- traverse nested structures with dot notation.
{{< /cel-example >}}

### Timestamp field

{{< field name="timestamp" type="CEL -> timestamp" required="true" >}}
Timestamp of the observation. Must evaluate to a CEL timestamp value.
{{< /field >}}

Available timestamp functions:

| Function | Input | Description |
|----------|-------|-------------|
| `now()` | none | Current time (use as fallback only) |
| `unix_ms(expr)` | integer | Epoch milliseconds |
| `unix_s(expr)` | integer | Epoch seconds |
| `parse_rfc3339(expr)` | string | RFC 3339 format: `"2024-01-15T00:00:00Z"` |
| `parse_iso8601(expr)` | string | ISO 8601 format (appends Z if no timezone) |
| `parse_rfc2822(expr)` | string | RFC 2822 format (RSS feeds) |
| `parse_datetime(expr, layout)` | string, string | Go time layout (e.g., `"2006-01-02 15:04:05"`) |
| `timestamp(expr)` | string | Parse standard timestamp string |

{{< cel-example expression="unix_ms(record.time)" input="record.time = 1705276800000" output="2024-01-15T00:00:00Z" >}}
Epoch milliseconds -- common in APIs that return integer timestamps.
{{< /cel-example >}}

{{< cel-example expression="parse_rfc3339(record.date)" input="record.date = \"2024-01-15T12:30:00Z\"" output="2024-01-15T12:30:00Z" >}}
RFC 3339 -- standard ISO timestamp format with timezone.
{{< /cel-example >}}

{{< cel-example expression="has(record.pubDate) ? parse_rfc2822(record.pubDate) : now()" input="record.pubDate = \"Mon, 15 Jan 2024 12:30:00 GMT\"" output="2024-01-15T12:30:00Z" >}}
Guarded fallback -- parse the date if present, otherwise fall back to current time.
{{< /cel-example >}}

### Event time fields

For time-bounded events (weather alerts, conflict events), you can specify a start and end time:

{{< field name="event_time" type="CEL -> timestamp" required="false" >}}
Event start time. Must evaluate to a CEL timestamp.
{{< /field >}}

{{< field name="event_end" type="CEL -> timestamp" required="false" >}}
Event end time. Must evaluate to a CEL timestamp.
{{< /field >}}

```yaml
observation:
  timestamp: >
    parse_rfc3339(record.updated)
  event_time: >
    parse_rfc3339(record.start_time)
  event_end: >
    parse_rfc3339(record.end_time)
```

### Velocity fields

{{< field name="velocity" type="map[string]CEL -> number" required="false" >}}
Velocity components for moving entities. Keys are arbitrary names (commonly `speed`, `heading`, `climb`). Each value is a CEL expression that must evaluate to a number -- it is stored as `map[string]float64` via numeric evaluation. A component whose expression returns a non-numeric value (or errors) is silently dropped from the velocity map.
{{< /field >}}

```yaml
velocity:
  speed: >
    has(record.speed) ? double(record.speed) : 0.0
  heading: >
    has(record.heading) ? double(record.heading) : 0.0
  climb: >
    has(record.vertical_rate) ? double(record.vertical_rate) : 0.0
```

{{< callout type="info" title="Velocity enables smooth interpolation" >}}
When velocity data is provided along with `display.icon.interpolation: true`, the frontend interpolates entity positions between poll updates for smooth movement on the globe.
{{< /callout >}}

### Observation metadata

{{< field name="observation.metadata" type="map[string]CEL -> string" required="false" >}}
Per-observation metadata. Same rules as entity metadata -- all values must return strings.
{{< /field >}}

### Content hash

{{< field name="content_hash" type="CEL -> string" required="false" >}}
Hash string for deduplication. Required when `recording.mode` is `dedupe`. Should produce a unique string for each distinct observation state. A new observation is only recorded when the content hash changes.
{{< /field >}}

```yaml
# Dedupe by article URL -- only re-record if the URL changes
content_hash: >
  record.link
```

```yaml
# Dedupe by composite state -- re-record if position or brightness changes
content_hash: >
  record.lat + "_" + record.lon + "_" + record.brightness
```

---

## Common CEL patterns

### Optional field guards

Always guard access to fields that may not exist in every record. Without `has()`, accessing a missing field causes a runtime error that silently skips the record.

{{< cel-example expression="has(record.field) ? record.field : \"default\"" input="record = {}" output="\"default\"" >}}
Conditional access with fallback -- returns the field value if present, otherwise a default.
{{< /cel-example >}}

### Type coercion

{{< cel-example expression="string(record.mag)" input="record.mag = 4.2" output="\"4.2\"" >}}
Number to string -- required for all metadata values.
{{< /cel-example >}}

{{< cel-example expression="double(record.latitude)" input="record.latitude = \"39.7392\"" output="39.7392" >}}
String to double -- use for coordinate fields that arrive as strings.
{{< /cel-example >}}

{{< cel-example expression="int(record.count)" input="record.count = \"42\"" output="42" >}}
String to integer -- use for integer fields that arrive as strings.
{{< /cel-example >}}

### Safe numeric conversion

{{< cel-example expression="coerce_double(record.value, 0.0)" input="record.value = \"not_a_number\"" output="0.0" >}}
Safe double conversion -- returns the fallback value if the input cannot be parsed as a double.
{{< /cel-example >}}

### Lookup tables

When you have static reference data (country codes, station metadata), define a lookup table and query it in CEL expressions:

```yaml
lookup_tables:
  - name: country_codes
    key_field: code
    entries:
      - code: US
        name: "United States"
      - code: GB
        name: "United Kingdom"

entity:
  metadata:
    country_name: >
      lookup("country_codes", record.country, "name")
```

{{< cel-example expression="lookup(\"country_codes\", record.country, \"name\")" input="record.country = \"US\"" output="\"United States\"" >}}
Lookup table query -- retrieves a field from a named lookup table by key. Returns `dyn` (the entry value's native type), or a CEL error if the table, key, or field is not found.
{{< /cel-example >}}

The lookup family also includes a presence check and a default-aware variant:

| Function | Returns | Description |
|----------|---------|-------------|
| `lookup(table, key, field)` | dyn | Retrieve a field from a lookup table entry. Errors if the table, key, or field is missing. |
| `has_lookup(table, key)` | bool | Report whether `key` exists in the named lookup table. Returns `false` if the table is unknown. |
| `lookup_or(table, key, field, default)` | dyn | Like `lookup`, but returns `default` instead of erroring when the table, key, or field is missing. |

{{< cel-example expression="has_lookup(\"country_codes\", record.country)" input="record.country = \"US\"" output="true" >}}
Presence check -- safe to call before `lookup` when a key may be absent.
{{< /cel-example >}}

{{< cel-example expression="lookup_or(\"country_codes\", record.country, \"name\", \"unknown\")" input="record.country = \"ZZ\"" output="\"unknown\"" >}}
Default-aware lookup -- returns the supplied default rather than erroring on a missing key.
{{< /cel-example >}}

### TLE orbital functions

For satellite TLE data, use SGP4 propagation functions. Each propagates the TLE to the current time. All four return `0.0` on a malformed or invalid TLE.

| CEL Function | Returns | Description |
|-------------|---------|-------------|
| `sgp4_lat(line1, line2)` | double | Current latitude in degrees |
| `sgp4_lon(line1, line2)` | double | Current longitude in degrees |
| `sgp4_alt_m(line1, line2)` | double | Current altitude in meters |
| `sgp4_vel_mps(line1, line2)` | double | Current velocity in meters per second |

{{< cel-example expression="sgp4_lat(record.line1, record.line2)" input="record.line1 = \"1 25544U ...\"" output="42.35" >}}
Propagate the TLE to the current time and return the satellite's latitude in degrees.
{{< /cel-example >}}
