---
title: "Display Configuration"
description: "Control how entities appear on the globe — icons, colors, trails, and data fields"
weight: 6
---

The `display` section defines how entities from a source are rendered on the globe and in the detail panel. This configuration is consumed by the frontend via the GetLayers gRPC API.

```yaml
display:
  icon:
    shape: dot
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#ffffff"
    width: 1.5
    opacity: 0.7
  style:
    color: "#ffffff"
    point_size: 6
  color_by:
    field: status
    values:
      operational: "#00ff9d"
    default_color: "#888888"
  field_renderers: []
```

## Icon

Controls the map icon rendered for each entity on the globe.

{{< field name="icon.shape" type="string" required="true" >}}
Icon shape. See the full list below. Unknown shapes fall back to `dot`.
{{< /field >}}

{{< field name="icon.rotatable" type="boolean" required="false" default="false" >}}
When true, the icon rotates to match the entity's heading. Use for aircraft, ships, and other directional entities that have velocity heading data.
{{< /field >}}

{{< field name="icon.interpolation" type="boolean" required="false" default="false" >}}
When true, the frontend interpolates entity positions between poll updates for smooth movement. Requires velocity data in the observation mapping.
{{< /field >}}

{{< field name="icon.scale" type="float" required="false" default="1.0" >}}
Size multiplier. Range: 0.1 to 5.0. A value of 1.0 is the standard size.
{{< /field >}}

### Available icon shapes

| Category | Shapes |
|----------|--------|
| Aviation | `flight`, `flight-alt-a`, `flight-alt-b`, `flight-alt-c`, `drone`, `helicopter` |
| Space | `diamond`, `satellite`, `iss`, `rocket`, `telescope`, `meteor` |
| Maritime | `ship`, `anchor`, `wave` |
| Military | `missile`, `tank`, `shield`, `crosshair`, `radar`, `explosion`, `bullseye` |
| Weather | `lightning`, `cloud`, `wind`, `tornado`, `snowflake`, `hurricane`, `flood` |
| Hazard | `warning`, `radiation`, `nuclear`, `biohazard`, `skull`, `fire`, `volcano` |
| Seismic | `ripple`, `crack` |
| Radio | `radio` |
| Infrastructure | `tower`, `building`, `bridge`, `factory`, `powerplant`, `crane`, `warehouse` |
| Geometry | `dot`, `star`, `hexagon`, `circle-ring`, `chevron`, `triangle`, `pentagon`, `cross` |
| Other | `marker`, `tree` |

{{< callout type="tip" title="Choosing an icon shape" >}}
Use `dot` as the default for generic data. Pick a domain-specific shape to make layers visually distinct -- `ripple` for earthquakes, `flight` for aircraft, `ship` for vessels, `fire` for hotspots.
{{< /callout >}}

```yaml
# Aircraft with rotation and smooth interpolation
icon:
  shape: flight
  rotatable: true
  interpolation: true
  scale: 1.0
```

```yaml
# Earthquake with seismic ripple icon
icon:
  shape: ripple
  rotatable: false
  interpolation: false
  scale: 1.0
```

---

## Trail

Defines the path trail rendered behind moving entities. Trails show historical positions as a line on the globe.

{{< field name="trail.color" type="string" required="true" >}}
Trail line color in hex format (`#rrggbb`).
{{< /field >}}

{{< field name="trail.width" type="float" required="false" default="1.5" >}}
Trail line width in pixels. Range: 0.5 to 10.0.
{{< /field >}}

{{< field name="trail.opacity" type="float" required="false" default="0.7" >}}
Trail transparency. Range: 0.0 (fully transparent) to 1.0 (fully opaque).
{{< /field >}}

```yaml
trail:
  color: "#00ff9d"
  width: 2.0
  opacity: 0.8
```

{{< callout type="info" title="Trails and recording mode" >}}
Trails are most useful with `recording.mode: append`, where each poll adds a new position to the entity's history. With `upsert` mode, only the latest position exists, so trails will not render meaningful paths.
{{< /callout >}}

---

## Style

Base rendering style for the entity point on the globe.

{{< field name="style.color" type="string" required="true" >}}
Entity color in hex format (`#rrggbb`).
{{< /field >}}

{{< field name="style.point_size" type="integer" required="true" >}}
Base point size in pixels. Range: 1 to 32. A value of 6-8 is standard.
{{< /field >}}

```yaml
style:
  color: "#ff006e"
  point_size: 6
```

---

## Color by

Colors each entity by the value of a metadata field instead of applying a single base color. When present, `color_by` overrides `style.color` per entity: the client looks up the entity's `field` value in `values` and uses the matched hex color, falling back to `default_color`.

{{< field name="color_by.field" type="string" required="true" >}}
Metadata field name whose value selects the color.
{{< /field >}}

{{< field name="color_by.values" type="map[string]string" required="true" >}}
Map of field value to hex color (`#rrggbb`). At least one entry is required, and each value must be a valid hex color.
{{< /field >}}

{{< field name="color_by.default_color" type="string" required="false" >}}
Hex color used when the entity's field value is not found in `values`.
{{< /field >}}

This example, from `sources.d/wikidata_nuclear_facilities.yaml`, colors each facility by its operational status:

```yaml
color_by:
  field: status
  values:
    operational: "#00ff9d"
    under_construction: "#ffcc00"
    shutdown: "#ff9900"
    decommissioned: "#ff4444"
  default_color: "#888888"
```

---

## Field renderers

Field renderers control how metadata fields appear in the entity detail panel. Each renderer maps one or more metadata keys to a formatted display.

```yaml
field_renderers:
  - keys: [magnitude, mag]
    label: "MAGNITUDE"
    format:
      type: float
      precision: 1
      prefix: "M"
    priority: 0
```

{{< field name="field_renderers[].keys" type="string[]" required="true" >}}
Array of metadata field names to match. The renderer activates when any key matches a field in the entity or observation metadata. Use multiple keys to handle different naming conventions across sources sharing a layer.
{{< /field >}}

{{< field name="field_renderers[].label" type="string" required="true" >}}
Display label shown above the formatted value in the detail panel.
{{< /field >}}

{{< field name="field_renderers[].format" type="object" required="true" >}}
Formatting specification for the value. See format options below.
{{< /field >}}

{{< field name="field_renderers[].priority" type="integer" required="false" default="0" >}}
Sort order in the detail panel. Lower values are shown first.
{{< /field >}}

### Format options

{{< field name="format.type" type="string" required="true" >}}
Value type for formatting. Allowed values: `string`, `float`, `integer`, `raw`.
{{< /field >}}

{{< field name="format.precision" type="integer" required="false" >}}
Decimal places for `float` type. Example: precision 1 formats `4.237` as `4.2`.
{{< /field >}}

{{< field name="format.prefix" type="string" required="false" >}}
String prepended to the formatted value. Example: `"M"` produces `M4.2`.
{{< /field >}}

{{< field name="format.suffix" type="string" required="false" >}}
String appended to the formatted value. Example: `" km"` produces `10.5 km`.
{{< /field >}}

{{< field name="format.transform" type="string" required="false" >}}
String transformation. Allowed values: `upper`, `lower`. Only applies to `string` type.
{{< /field >}}

### Complete example

This example defines renderers for an earthquake source with magnitude, depth, location, and type fields:

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
    - keys: [source]
      label: "SOURCE"
      format:
        type: string
        transform: upper
      priority: 4
```

This renders the detail panel as:

```
MAGNITUDE  M4.2
DEPTH      10.5 km
LOCATION   Central California
TYPE       EARTHQUAKE
SOURCE     EMSC
```

### Matching across sources

When multiple sources feed the same `layer_type`, their metadata field names may differ. Use the `keys` array to match all variants:

```yaml
# Matches "magnitude" from source A and "mag" from source B
- keys: [magnitude, mag]
  label: "MAGNITUDE"
  format:
    type: float
    precision: 1
    prefix: "M"
  priority: 0
```

{{< callout type="tip" title="Priority ordering" >}}
Arrange field renderers by importance. Put the most critical fields (magnitude, status, name) at low priority numbers so they appear first in the detail panel. Less important fields (source, URL) should have higher priority numbers.
{{< /callout >}}
