---
title: "Examples"
description: "Complete, copy-pasteable source definitions for common data formats"
weight: 7
---

This page provides complete, working source definitions that you can copy into your `sources.d/` directory and use immediately. Each example uses a public API that requires no authentication.

---

## GeoJSON API -- Earthquake Data

This source ingests earthquake data from the European-Mediterranean Seismological Centre (EMSC) Seismic Portal. It demonstrates the full lifecycle of a GeoJSON source: HTTP polling, GeoJSON parsing, CEL filtering, entity/observation mapping, and display configuration.

{{< callout type="info" title="Why EMSC?" >}}
The EMSC Seismic Portal provides a free, unauthenticated GeoJSON API that returns earthquake events worldwide. It follows the FDSN (International Federation of Digital Seismograph Networks) standard, making it a reliable reference endpoint.
{{< /callout >}}

```yaml
schema_version: 2
name: emsc_earthquakes
labels:
  category: geological
  priority: low

source_type: emsc_earthquakes
layer_type: earthquakes
display_name: "EMSC Earthquakes"
```

{{< callout type="tip" title="layer_type controls grouping" >}}
Multiple sources can share the same `layer_type`. If you also have a USGS earthquake source, set both to `layer_type: earthquakes` and they will merge onto the same map layer. The `source_type` must be unique per source definition.
{{< /callout >}}

### Transport

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

{{< callout type="info" title="Choosing an interval" >}}
Earthquake APIs update every few minutes. Polling every 120 seconds gives near-real-time data without hammering the endpoint. For slower-updating APIs (news feeds, weather bulletins), 600-1200 seconds is typical.
{{< /callout >}}

### Parser

```yaml
parser:
  format: geojson
  records_path: "features"
  max_records: 10000
```

{{< callout type="info" title="Why geojson format?" >}}
The `geojson` parser automatically handles the GeoJSON FeatureCollection structure. Each feature becomes a record with `record.id`, `record.properties.*`, and `record.geometry.coordinates`. You could use `format: json` with `records_path: "features"` instead, but the `geojson` parser provides better validation and error messages for malformed GeoJSON.
{{< /callout >}}

### Filter

```yaml
filter: >
  has(record.properties.mag) && double(record.properties.mag) >= 1.0
```

{{< callout type="tip" title="Filter early to reduce noise" >}}
This CEL expression discards earthquakes below magnitude 1.0 before they ever reach the database. Without the `has()` guard, records missing the `mag` field would cause runtime errors and be silently skipped.
{{< /callout >}}

### Entity mapping

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
    type: >
      has(record.properties.evtype) ? record.properties.evtype : "earthquake"
    source: >
      "EMSC"
    magtype: >
      has(record.properties.magtype) ? record.properties.magtype : ""
```

{{< callout type="warning" title="All metadata values must be strings" >}}
Use `string()` to convert numeric values like `record.properties.mag`. Returning a raw number causes a runtime error.
{{< /callout >}}

### Observation mapping

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

{{< callout type="info" title="GeoJSON coordinate order and earthquake depth" >}}
GeoJSON uses `[longitude, latitude, altitude]` order -- note the swap at indices 0 and 1. For earthquakes, the third coordinate is depth in kilometers (positive = below surface). Multiplying by `-1000.0` converts to altitude in meters (negative = underground), which the globe renderer uses to display depth correctly.
{{< /callout >}}

### Recording and cache

```yaml
recording:
  mode: upsert

cache:
  ttl: "3600s"
```

{{< callout type="tip" title="Why upsert for earthquakes?" >}}
Earthquake events do not move -- their position is fixed. Use `upsert` so each event keeps only its latest magnitude revision. If you needed historical magnitude updates (rare), switch to `append`.
{{< /callout >}}

### Display

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
    - keys: [type]
      label: "TYPE"
      format:
        type: string
        transform: upper
      priority: 3
```

{{< callout type="info" title="Field renderer keys array" >}}
The `keys: [magnitude, mag]` array matches metadata from this source (`magnitude`) and from other earthquake sources that may use a shorter key (`mag`). This way, multiple sources sharing the `earthquakes` layer all render through the same field renderer.
{{< /callout >}}

### Complete definition

Here is the full source YAML as a single copy-pasteable file. Save it as `sources.d/emsc_earthquakes.yaml`:

```yaml
# sources.d/emsc_earthquakes.yaml
# European-Mediterranean Seismological Centre earthquake data.
# API: FDSN event web service (Seismic Portal) -- no auth required.
schema_version: 2

name: emsc_earthquakes
labels:
  category: geological
  priority: low

source_type: emsc_earthquakes
layer_type: earthquakes
display_name: "EMSC Earthquakes"

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
  has(record.properties.mag) && double(record.properties.mag) >= 1.0

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
    type: >
      has(record.properties.evtype) ? record.properties.evtype : "earthquake"
    source: >
      "EMSC"
    magtype: >
      has(record.properties.magtype) ? record.properties.magtype : ""

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
    - keys: [type]
      label: "TYPE"
      format:
        type: string
        transform: upper
      priority: 3
```

---

## RSS Feed -- News Articles

This source ingests articles from a BBC World News RSS feed. It demonstrates RSS parsing, dedupe recording, and how to add optional AI enrichment for geolocation and classification.

{{< callout type="info" title="RSS sources start without coordinates" >}}
News articles rarely have geographic coordinates in the RSS feed itself. The entity and observation mappings below set latitude and longitude to `0.0` as a placeholder. The optional AI enrichment section at the end shows how to use an LLM to geolocate articles from their content.
{{< /callout >}}

### Transport and parser

```yaml
schema_version: 1
name: bbc_world_news
source_type: bbc_world_news
layer_type: news_articles
display_name: "BBC World News"

transport:
  type: http_poll
  url: "https://feeds.bbci.co.uk/news/world/rss.xml"
  method: GET
  headers:
    Accept: "application/xml"
  timeout: "15s"
  interval: "1200s"

parser:
  format: rss
  max_records: 100
```

{{< callout type="tip" title="Polling interval for RSS" >}}
News feeds typically update every 15-30 minutes. Polling every 1200 seconds (20 minutes) captures new articles without excessive requests. The BBC rate-limits aggressive crawlers, so be conservative.
{{< /callout >}}

### Entity mapping

```yaml
entity:
  external_id: >
    record.link
  name: >
    record.title
  metadata:
    source_url: >
      record.link
    description: >
      has(record.description) ? record.description : ""
    author: >
      has(record.creator) ? record.creator : (has(record.author) ? record.author : "")
    feed_source: >
      "bbc_world"
```

{{< callout type="info" title="Using the article URL as external_id" >}}
The article URL is the most stable unique identifier across RSS polls. Titles can change (editors update headlines), but URLs are permanent. This also doubles as the `content_hash` for deduplication.
{{< /callout >}}

### Observation mapping

```yaml
observation:
  latitude: "0.0"
  longitude: "0.0"
  altitude: "0.0"
  timestamp: >
    has(record.pubDate) ? parse_rfc2822(record.pubDate) : now()
  velocity: {}
  metadata:
    description: >
      has(record.description) ? record.description : ""
  content_hash: >
    record.link
```

{{< callout type="info" title="parse_rfc2822 for RSS dates" >}}
RSS feeds use RFC 2822 date format (`"Mon, 15 Jan 2024 12:30:00 GMT"`). The `parse_rfc2822()` function handles this format. The `has()` guard falls back to `now()` if the date field is missing.
{{< /callout >}}

### Recording, cache, and display

```yaml
recording:
  mode: dedupe

cache:
  ttl: "3600s"

display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 0.8
  style:
    color: "#b388ff"
    point_size: 6
  trail:
    color: "#b388ff"
    width: 1.0
    opacity: 0.6
  field_renderers:
    - keys: [feed_source]
      label: "SOURCE"
      format:
        type: string
        transform: upper
      priority: 0
    - keys: [description]
      label: "DESCRIPTION"
      format:
        type: string
      priority: 1
    - keys: [source_url]
      label: "URL"
      format:
        type: string
      priority: 2
```

{{< callout type="tip" title="Why dedupe for news?" >}}
RSS feeds re-publish the same articles across multiple polls. With `dedupe` mode and `content_hash` set to `record.link`, the feeder only records a new observation when the article URL changes -- meaning a genuinely new article appeared. Without dedupe, you would get duplicate rows for every poll cycle.
{{< /callout >}}

### Complete definition

Save this as `sources.d/bbc_world_news.yaml`:

```yaml
# sources.d/bbc_world_news.yaml
# BBC World News RSS feed -- no auth required.
schema_version: 1

name: bbc_world_news
source_type: bbc_world_news
layer_type: news_articles
display_name: "BBC World News"

transport:
  type: http_poll
  url: "https://feeds.bbci.co.uk/news/world/rss.xml"
  method: GET
  headers:
    Accept: "application/xml"
  timeout: "15s"
  interval: "1200s"

parser:
  format: rss
  max_records: 100

entity:
  external_id: >
    record.link
  name: >
    record.title
  metadata:
    source_url: >
      record.link
    description: >
      has(record.description) ? record.description : ""
    author: >
      has(record.creator) ? record.creator : (has(record.author) ? record.author : "")
    feed_source: >
      "bbc_world"

observation:
  latitude: "0.0"
  longitude: "0.0"
  altitude: "0.0"
  timestamp: >
    has(record.pubDate) ? parse_rfc2822(record.pubDate) : now()
  velocity: {}
  metadata:
    description: >
      has(record.description) ? record.description : ""
  content_hash: >
    record.link

recording:
  mode: dedupe

cache:
  ttl: "3600s"

display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 0.8
  style:
    color: "#b388ff"
    point_size: 6
  trail:
    color: "#b388ff"
    width: 1.0
    opacity: 0.6
  field_renderers:
    - keys: [feed_source]
      label: "SOURCE"
      format:
        type: string
        transform: upper
      priority: 0
    - keys: [description]
      label: "DESCRIPTION"
      format:
        type: string
      priority: 1
    - keys: [source_url]
      label: "URL"
      format:
        type: string
      priority: 2
```

### Optional: AI enrichment for geolocation

If you have an LLM provider configured in your Respondent deployment, you can add an `ai` block to automatically geolocate articles and classify them by category. This is optional -- the source works without it, but articles will remain at the `0.0, 0.0` placeholder coordinates.

{{< callout type="warning" title="Requires LLM configuration" >}}
AI enrichment requires an LLM provider (OpenAI, Anthropic, or compatible) configured in your deployment. See the AI configuration docs for setup. Without it, the `ai` block is silently ignored.
{{< /callout >}}

Add this block to the source definition:

```yaml
ai:
  enabled: true
  operations:
    - name: news_geo_enrichment
      cache_ttl: "24h"
      prompt: |
        You are an OSINT analyst. Given a news article, identify the primary
        geographic location it covers and classify it.

        <article>
          <title>{{.Entity.Name}}</title>
          <description>{{index .Entity.Metadata "description"}}</description>
          <source>{{index .Entity.Metadata "source_url"}}</source>
        </article>

        Tasks:
        1. GEOLOCATE: Identify the most specific location. If you can
           confidently provide coordinates, include lat/lon. Otherwise
           leave them null and provide city/region/country as text.
        2. CLASSIFY: Assign a primary category from the allowed enum.
        3. SUMMARIZE: Write a 1-2 sentence summary in English.

        Respond with JSON matching this schema:
        {{.OutputSchema}}
      output_schema:
        type: object
        required: [country, category, summary]
        properties:
          lat: { type: [number, "null"], minimum: -90, maximum: 90 }
          lon: { type: [number, "null"], minimum: -180, maximum: 180 }
          city: { type: [string, "null"] }
          region: { type: [string, "null"] }
          country: { type: string }
          category:
            type: string
            enum:
              - armed_conflict
              - natural_disaster
              - political_instability
              - economic
              - humanitarian_crisis
              - public_safety
              - environmental
              - other
          summary: { type: string }
      max_tokens: 2048
      temperature: 0
      output:
        target: entity
        patch_coordinates:
          enabled: true
          lat_field: "lat"
          lon_field: "lon"
          min_confidence: 0.7
```

{{< callout type="tip" title="How coordinate patching works" >}}
When `patch_coordinates.enabled` is true, the LLM-extracted `lat` and `lon` values replace the placeholder `0.0, 0.0` coordinates on the entity. This moves the article marker from the Gulf of Guinea (where 0,0 is) to its actual geographic location. The `min_confidence` threshold prevents low-confidence guesses from overriding the position.
{{< /callout >}}
