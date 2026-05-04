---
title: "Parsers"
description: "Parse JSON, GeoJSON, CSV, RSS, XML, and TLE data formats"
weight: 2
---

Parsers transform raw transport responses into arrays of records. Each record becomes a map that CEL expressions in `entity`, `observation`, and `filter` can access via the `record` variable.

```yaml
parser:
  format: json
  records_path: "data.items"
  max_records: 10000
```

## Common fields

These fields apply to all parser formats.

{{< field name="format" type="string" required="true" >}}
Parser format. Allowed values: `json`, `geojson`, `csv`, `rss`, `xml`, `tle`, `json_table`.
{{< /field >}}

{{< field name="records_path" type="string" required="false" >}}
Dot-separated path to the array of records in the response. Omit when the response root is already an array. Path segments must match `^[a-zA-Z0-9_]+$` with a maximum depth of 8.
{{< /field >}}

{{< field name="max_records" type="integer" required="false" default="10000" >}}
Safety cap on the number of records to process per poll. Range: 1 to 100000.
{{< /field >}}

---

## JSON

Standard JSON parsing. Expects the response to be a JSON array of objects, or a JSON object with a nested array at `records_path`.

```yaml
parser:
  format: json
  records_path: "data.items"
  max_records: 5000
```

When the response root is already an array, omit `records_path`:

```yaml
parser:
  format: json
  max_records: 5000
```

### Array columns (schema_version 2)

Some APIs return arrays of arrays instead of arrays of objects (e.g., OpenSky returns `[icao24, callsign, origin_country, ...]`). Use `array_columns` to map array indices to named fields:

{{< field name="array_columns" type="string[]" required="false" >}}
Map array indices to named fields. The Nth entry becomes the field name for index N.
{{< /field >}}

```yaml
parser:
  format: json
  records_path: "states"
  array_columns:
    - icao24
    - callsign
    - origin_country
    - time_position
    - longitude
    - latitude
    - baro_altitude
    - on_ground
    - velocity
    - true_track
```

After parsing, you access fields by name: `record.icao24`, `record.callsign`, etc.

### Object-to-records (schema_version 2)

Some APIs return a dictionary keyed by ID instead of an array. Use `object_to_records` to convert it:

{{< field name="object_to_records" type="boolean" required="false" default="false" >}}
Convert a top-level JSON object `{"key": {...}, ...}` into an array of its values `[{...}, ...]`.
{{< /field >}}

{{< field name="object_key_field" type="string" required="false" >}}
When `object_to_records` is true, inject the object key into each record under this field name.
{{< /field >}}

{{< code-tabs >}}
{{< tab title="API Response" >}}
```json
{
  "S1234567": {"type": "RS41", "lat": 48.2, "lon": 11.8},
  "S7654321": {"type": "RS92", "lat": 52.5, "lon": 13.4}
}
```
{{< /tab >}}
{{< tab title="Parser Config" >}}
```yaml
parser:
  format: json
  object_to_records: true
  object_key_field: "serial"
```
{{< /tab >}}
{{< tab title="Parsed Records" >}}
```json
[
  {"serial": "S1234567", "type": "RS41", "lat": 48.2, "lon": 11.8},
  {"serial": "S7654321", "type": "RS92", "lat": 52.5, "lon": 13.4}
]
```
{{< /tab >}}
{{< /code-tabs >}}

### Array of arrays (schema_version 2)

Some APIs (e.g., NOAA SWPC) return data as a table where the first row is column headers and subsequent rows are data values.

{{< field name="array_of_arrays" type="boolean" required="false" default="false" >}}
Treat a JSON array-of-arrays as a table. The first row becomes column headers, subsequent rows become data records.
{{< /field >}}

{{< code-tabs >}}
{{< tab title="API Response" >}}
```json
[
  ["time_tag", "Kp", "a_running"],
  ["2024-01-15 00:00:00", "2.33", "7"],
  ["2024-01-15 03:00:00", "3.67", "22"]
]
```
{{< /tab >}}
{{< tab title="Parser Config" >}}
```yaml
parser:
  format: json
  array_of_arrays: true
```
{{< /tab >}}
{{< tab title="Parsed Records" >}}
```json
[
  {"time_tag": "2024-01-15 00:00:00", "Kp": "2.33", "a_running": "7"},
  {"time_tag": "2024-01-15 03:00:00", "Kp": "3.67", "a_running": "22"}
]
```
{{< /tab >}}
{{< /code-tabs >}}

---

## GeoJSON

Parses GeoJSON FeatureCollection responses. The `records_path` defaults to `"features"` -- you only need to set it if the FeatureCollection is nested inside a wrapper object.

```yaml
parser:
  format: geojson
  records_path: "features"
  max_records: 10000
```

Each parsed record has the standard GeoJSON structure:

- `record.id` -- the feature ID
- `record.properties.*` -- feature properties
- `record.geometry.coordinates` -- coordinate array in `[longitude, latitude, altitude?]` order

{{< callout type="warning" title="GeoJSON coordinate order" >}}
GeoJSON uses `[longitude, latitude]` order, not `[latitude, longitude]`. When mapping to observations, use `record.geometry.coordinates[0]` for longitude and `record.geometry.coordinates[1]` for latitude.
{{< /callout >}}

```yaml
# Mapping GeoJSON coordinates to observations
observation:
  latitude: >
    record.geometry.coordinates[1]
  longitude: >
    record.geometry.coordinates[0]
  altitude: >
    record.geometry.coordinates[2] * -1000.0
```

---

## CSV

Parses comma-separated (or custom-delimited) text data.

```yaml
parser:
  format: csv
  max_records: 50000
  csv_options:
    delimiter: ","
    has_header: true
    skip_lines: 0
    collapse_whitespace: false
    comment_prefix: "#"
```

{{< field name="csv_options.delimiter" type="string" required="false" default="," >}}
Field delimiter character. Use `"\t"` for tab-separated values.
{{< /field >}}

{{< field name="csv_options.has_header" type="boolean" required="false" default="false" >}}
When true, the first row (after `skip_lines`) is treated as column headers. Fields are accessed by header name (e.g., `record.latitude`). When false, fields are accessed by index.
{{< /field >}}

{{< field name="csv_options.skip_lines" type="integer" required="false" default="0" >}}
Number of lines to skip before parsing begins. Use to skip comment blocks or preamble text.
{{< /field >}}

{{< field name="csv_options.collapse_whitespace" type="boolean" required="false" default="false" >}}
Collapse consecutive whitespace characters within fields.
{{< /field >}}

{{< field name="csv_options.comment_prefix" type="string" required="false" >}}
Lines starting with this string are treated as comments and ignored.
{{< /field >}}

{{< code-tabs >}}
{{< tab title="CSV Response" >}}
```text
# Station data export
ID,Name,LAT,LON,TEMP
WX001,Denver,39.7392,-104.9903,22.5
WX002,Miami,25.7617,-80.1918,31.2
```
{{< /tab >}}
{{< tab title="Parser Config" >}}
```yaml
parser:
  format: csv
  csv_options:
    delimiter: ","
    has_header: true
    comment_prefix: "#"
```
{{< /tab >}}
{{< /code-tabs >}}

After parsing, you access fields by header name: `record.ID`, `record.Name`, `record.LAT`, etc.

---

## RSS

Parses RSS and Atom feeds. No `records_path` is needed -- the parser automatically extracts items from the feed.

```yaml
parser:
  format: rss
  max_records: 100
```

Each parsed record automatically contains these fields (when present in the feed):

| Field | Description |
|-------|-------------|
| `record.title` | Article title |
| `record.link` | Article URL |
| `record.description` | Article summary or content |
| `record.pubDate` | Publication date string |
| `record.creator` | Dublin Core creator |
| `record.author` | Feed author |

{{< callout type="info" title="RSS timestamp parsing" >}}
RSS feeds use RFC 2822 date format. Use `parse_rfc2822(record.pubDate)` in your observation timestamp mapping.
{{< /callout >}}

```yaml
# Complete RSS source example
parser:
  format: rss
  max_records: 100

entity:
  external_id: >
    record.link
  name: >
    record.title

observation:
  latitude: "0.0"
  longitude: "0.0"
  timestamp: >
    has(record.pubDate) ? parse_rfc2822(record.pubDate) : now()
  content_hash: >
    record.link
```

---

## XML

Parses XML documents. Use `records_path` to select the repeating element that represents individual records.

```yaml
parser:
  format: xml
  records_path: "response.items.item"
  max_records: 5000
```

{{< field name="records_path" type="string" required="true" >}}
Dot-separated path to the repeating XML element. Each matched element becomes a record.
{{< /field >}}

{{< code-tabs >}}
{{< tab title="XML Response" >}}
```xml
<response>
  <items>
    <item>
      <id>1</id>
      <name>Station Alpha</name>
      <lat>40.7128</lat>
      <lon>-74.0060</lon>
    </item>
    <item>
      <id>2</id>
      <name>Station Beta</name>
      <lat>34.0522</lat>
      <lon>-118.2437</lon>
    </item>
  </items>
</response>
```
{{< /tab >}}
{{< tab title="Parser Config" >}}
```yaml
parser:
  format: xml
  records_path: "response.items.item"
```
{{< /tab >}}
{{< /code-tabs >}}

After parsing, fields are accessed by element name: `record.id`, `record.name`, `record.lat`, etc.

---

## TLE

Parses Two-Line Element set data for satellite orbital tracking. Each TLE set (name line + line 1 + line 2) becomes a single record.

```yaml
parser:
  format: tle
  max_records: 50000
```

TLE records expose orbital data through special CEL functions that propagate the orbit to the current time using SGP4:

| CEL Function | Description |
|-------------|-------------|
| `sgp4_lat(record.line1, record.line2)` | Current latitude in degrees |
| `sgp4_lon(record.line1, record.line2)` | Current longitude in degrees |
| `sgp4_alt_m(record.line1, record.line2)` | Current altitude in meters |
| `sgp4_vel_mps(record.line1, record.line2)` | Current velocity in meters per second |

```yaml
# TLE satellite source example
filter: >
  has(record.line1) && has(record.line2) && sgp4_lat(record.line1, record.line2) != 0.0

observation:
  latitude: >
    sgp4_lat(record.line1, record.line2)
  longitude: >
    sgp4_lon(record.line1, record.line2)
  altitude: >
    sgp4_alt_m(record.line1, record.line2)
  timestamp: >
    now()
```

---

## JSON Table

Parses JSON objects where keys are entity identifiers and values are data objects. This is a shorthand for `json` with `object_to_records: true`.

```yaml
parser:
  format: json_table
  max_records: 5000
```

{{< callout type="tip" title="json_table vs json + object_to_records" >}}
`json_table` is syntactic sugar. If you need to control the key field name, use `format: json` with `object_to_records: true` and `object_key_field` instead.
{{< /callout >}}
