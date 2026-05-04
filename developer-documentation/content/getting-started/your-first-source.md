---
title: "Your First Source"
description: "Create a data source that ingests earthquake data and displays it on the globe"
weight: 3
---

This guide walks you through creating a data source from scratch. By the end, you will have real-time earthquake data from the European-Mediterranean Seismological Centre (EMSC) displayed on the Respondent globe.

## What you will build

A source definition that:
- Polls a public earthquake API every 2 minutes
- Parses GeoJSON features into entities
- Filters out minor earthquakes (magnitude < 2.0)
- Maps each earthquake to a positioned entity on the globe
- Displays earthquakes as red ripple icons with magnitude, depth, and location details

## Step 1: Create the source file

Create a new file at `sources.d/my_earthquakes.yaml` with the source identity:

```yaml
schema_version: 2

name: my_earthquakes
source_type: my_earthquakes
layer_type: earthquakes
display_name: "My Earthquakes"
```

{{< callout type="info" title="What these fields mean" >}}
- **schema_version** -- always use `2` for new sources. Version 2 enables filtering, caching, and advanced parser options.
- **name** -- unique identifier for this source. Lowercase and underscores only.
- **source_type** -- maps to the internal source registry. By convention, use the same value as `name`.
- **layer_type** -- groups entities on the globe. Multiple sources can feed the same layer (e.g., both USGS and EMSC earthquakes share the `earthquakes` layer).
- **display_name** -- human-readable label shown in the UI layer picker.
{{< /callout >}}

## Step 2: Configure the transport

Add the HTTP polling transport to fetch earthquake data from the EMSC Seismic Portal API:

```yaml
transport:
  type: http_poll
  url: "https://www.seismicportal.eu/fdsnws/event/1/query?format=json&limit=100&orderby=time"
  method: GET
  headers:
    Accept: "application/json"
  timeout: "10s"
  interval: "120s"
  max_response_bytes: 52428800
  retry:
    max_attempts: 3
    backoff: "exponential"
    initial_delay: "1s"
    max_delay: "30s"
```

This tells Respondent to make a GET request to the EMSC API every 120 seconds (2 minutes), with a 10-second timeout and automatic retries on failure.

## Step 3: Configure the parser

The EMSC API returns GeoJSON, which wraps records in a `features` array. Add the parser:

```yaml
parser:
  format: geojson
  records_path: "features"
  max_records: 10000
```

{{< callout type="tip" title="Tip" >}}
For GeoJSON APIs, the `records_path` is almost always `"features"`. Each feature has a `properties` object (metadata), a `geometry` object (coordinates), and an `id` field.
{{< /callout >}}

## Step 4: Add a filter

Filter out small earthquakes to keep the display focused on significant events. Add a CEL (Common Expression Language) filter:

```yaml
filter: >
  has(record.properties.mag) && double(record.properties.mag) >= 2.0
```

{{< cel-example expression="has(record.properties.mag) && double(record.properties.mag) >= 2.0" input="record.properties.mag = 3.5" output="true" >}}
The `has()` function checks that the magnitude field exists before accessing it. `double()` converts the value to a floating-point number for comparison. Records where the magnitude is missing or below 2.0 are discarded.
{{< /cel-example >}}

## Step 5: Map the entity

Define how each earthquake record becomes an entity in Respondent. Entity mappings use CEL expressions that receive each parsed record as the `record` variable:

```yaml
entity:
  external_id: >
    record.id
  name: >
    has(record.properties.flynn_region) ? record.properties.flynn_region : record.id
  metadata:
    magnitude: >
      string(record.properties.mag)
    depth: >
      string(record.properties.depth)
    place: >
      has(record.properties.flynn_region) ? record.properties.flynn_region : "Unknown"
    source: >
      "EMSC"
```

{{< callout type="info" title="Metadata values must be strings" >}}
All `metadata` values must be CEL expressions that return strings. Use `string()` to convert numeric fields. These values appear in the entity detail panel.
{{< /callout >}}

## Step 6: Map the observation

Map each record to a spatial observation -- the position, time, and per-observation metadata:

```yaml
observation:
  latitude: >
    record.geometry.coordinates[1]
  longitude: >
    record.geometry.coordinates[0]
  altitude: >
    record.geometry.coordinates[2] * -1000.0
  timestamp: >
    timestamp(record.properties.time)
  velocity: {}
  metadata:
    magnitude: >
      string(record.properties.mag)
  content_hash: ""
```

{{< cel-example expression="record.geometry.coordinates[1]" input="record.geometry.coordinates = [-122.5, 37.8, 10.0]" output="37.8" >}}
GeoJSON stores coordinates as `[longitude, latitude, depth]`. Index `[0]` is longitude, `[1]` is latitude, and `[2]` is depth in kilometers.
{{< /cel-example >}}

{{< callout type="info" title="Altitude and depth" >}}
The altitude field converts depth (km below surface) to altitude (meters). Multiplying by `-1000.0` converts positive depth-in-km to negative altitude-in-meters, placing the entity below the surface on the globe.
{{< /callout >}}

## Step 7: Set the recording mode

Configure how observations are persisted:

```yaml
recording:
  mode: upsert

cache:
  ttl: "3600s"
```

- **upsert** -- each poll updates the existing observation for an entity rather than appending a new row. This keeps one record per earthquake with the latest data.
- **cache.ttl** -- entities remain visible on the globe for 3600 seconds (1 hour) after the last successful fetch. Set this higher than the polling interval to survive missed polls.

## Step 8: Configure the display

Define how earthquakes appear on the globe and in the entity detail panel:

```yaml
display:
  icon:
    shape: ripple
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#ff006e"
    width: 1.5
    opacity: 0.7
  style:
    color: "#ff006e"
    point_size: 6
  field_renderers:
    - keys: [magnitude, mag]
      label: "MAGNITUDE"
      format:
        type: float
        precision: 1
        prefix: "M"
      priority: 0
    - keys: [depth, depth_km]
      label: "DEPTH"
      format:
        type: float
        precision: 1
        suffix: " km"
      priority: 1
    - keys: [place]
      label: "LOCATION"
      format:
        type: string
      priority: 2
```

- **shape: ripple** -- an expanding ring icon, fitting for seismic events.
- **field_renderers** -- define how metadata fields are formatted in the entity detail panel. The `keys` array matches against both entity and observation metadata. A magnitude of `4.2` renders as `M4.2`. Priority controls display order (lower = first).

## Step 9: Restart Respondent

Apply the new source by restarting the container:

```bash
docker compose restart
```

Check the logs to confirm the source loaded successfully:

```bash
docker compose logs -f
```

Look for log lines mentioning `my_earthquakes` -- you should see the source being loaded and the first poll being scheduled.

## Step 10: See earthquakes on the globe

Open `http://localhost:8090` in your browser. After the first polling interval (up to 2 minutes), red ripple icons will appear on the globe at earthquake locations. Click any earthquake to see the detail panel with magnitude, depth, and location fields.

## Complete source file

Here is the full `sources.d/my_earthquakes.yaml` for reference:

```yaml
schema_version: 2

name: my_earthquakes
source_type: my_earthquakes
layer_type: earthquakes
display_name: "My Earthquakes"

transport:
  type: http_poll
  url: "https://www.seismicportal.eu/fdsnws/event/1/query?format=json&limit=100&orderby=time"
  method: GET
  headers:
    Accept: "application/json"
  timeout: "10s"
  interval: "120s"
  max_response_bytes: 52428800
  retry:
    max_attempts: 3
    backoff: "exponential"
    initial_delay: "1s"
    max_delay: "30s"

parser:
  format: geojson
  records_path: "features"
  max_records: 10000

filter: >
  has(record.properties.mag) && double(record.properties.mag) >= 2.0

entity:
  external_id: >
    record.id
  name: >
    has(record.properties.flynn_region) ? record.properties.flynn_region : record.id
  metadata:
    magnitude: >
      string(record.properties.mag)
    depth: >
      string(record.properties.depth)
    place: >
      has(record.properties.flynn_region) ? record.properties.flynn_region : "Unknown"
    source: >
      "EMSC"

observation:
  latitude: >
    record.geometry.coordinates[1]
  longitude: >
    record.geometry.coordinates[0]
  altitude: >
    record.geometry.coordinates[2] * -1000.0
  timestamp: >
    timestamp(record.properties.time)
  velocity: {}
  metadata:
    magnitude: >
      string(record.properties.mag)
  content_hash: ""

recording:
  mode: upsert

cache:
  ttl: "3600s"

display:
  icon:
    shape: ripple
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#ff006e"
    width: 1.5
    opacity: 0.7
  style:
    color: "#ff006e"
    point_size: 6
  field_renderers:
    - keys: [magnitude, mag]
      label: "MAGNITUDE"
      format:
        type: float
        precision: 1
        prefix: "M"
      priority: 0
    - keys: [depth, depth_km]
      label: "DEPTH"
      format:
        type: float
        precision: 1
        suffix: " km"
      priority: 1
    - keys: [place]
      label: "LOCATION"
      format:
        type: string
      priority: 2
```

## Next steps

Now that you have a working source, explore further:

- Add **labels** for categorization: `labels: { category: geological, priority: medium }`
- Enable **AI enrichment** to generate risk assessments for each earthquake (see the [Configuration]({{< relref "configuration" >}}) guide to set up an LLM provider first)
- Create additional sources for other data -- flights, ships, weather, satellites -- using the same pattern
