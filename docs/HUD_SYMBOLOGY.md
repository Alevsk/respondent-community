# HUD Symbology — Icon System Reference

This document describes the Canvas2D icon/symbology system used to render entities on the Cesium globe. It covers the design aesthetic, technical architecture, the complete shape catalog, how to run the live preview, and a step-by-step guide for adding new icons.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Running the Icon Preview](#running-the-icon-preview)
4. [Design Guidelines](#design-guidelines)
5. [Shape Catalog](#shape-catalog)
6. [Source Icon Assignments](#source-icon-assignments)
7. [How to Add a New Icon](#how-to-add-a-new-icon)

---

## Overview

Every entity rendered on the globe — flights, earthquakes, satellites, fires, weather events — is represented by a small Canvas2D icon called a **symbol**. Symbols follow a unified **"classified intelligence agency HUD"** aesthetic:

- Bold, filled shapes in a single entity color
- Black (`#000000`) cutout details punched into the filled shape
- No visible strokes (with one documented exception: the `wind` icon)
- High contrast against dark and satellite-imagery backgrounds

The goal is a consistent visual language across all data layers: instantly recognizable at small sizes, readable against the Earth's surface at any zoom level.

---

## Architecture

### Icon files

Each icon is a TypeScript file that exports a single draw function:

```typescript
export function drawXxxIcon(ctx: CanvasRenderingContext2D, color: string): void
```

The function draws directly onto a 32×32 logical canvas. All coordinates are written as if the canvas is 32×32; the registry renders at 2× scale (64×64 physical pixels) so Cesium can display crisp billboards at typical globe zoom levels.

Icon files live at:

```
frontend/apps/earth/src/features/globe/icons/
```

### Canvas rendering

`iconRegistry.ts` manages the full lifecycle:

```
createIconCanvas(drawFn, color)
  → createElement('canvas')  [64×64 physical]
  → ctx.scale(2, 2)
  → drawFn(ctx, color)        [draws at 32×32 logical coords]
```

The resulting canvas is passed to Cesium as a billboard image. Cesium billboards are set to `Color.WHITE` so the colors drawn by each icon function come through unchanged.

### Shape-to-function registry

`SHAPE_TO_DRAW_FN` in `iconRegistry.ts` is the authoritative lookup table that maps every string shape name (as referenced in YAML source definitions) to its draw function. It is populated at compile time from static imports.

```typescript
const SHAPE_TO_DRAW_FN: Record<string, IconDrawFn> = {
  diamond: drawDiamondIcon,
  flight:  drawFlightIcon,
  // ... all shapes
}
```

The string `satellite` is a legacy alias for `diamond` kept for backward compatibility.

### Hardcoded imperative layer map

A smaller `ICON_DRAW_MAP` provides draw functions for the four legacy imperative adapters that predate the declarative YAML system. Dynamic registrations from YAML always take precedence over this map.

```typescript
const ICON_DRAW_MAP: Record<string, IconDrawFn> = {
  flights_commercial: drawFlightIcon,
  flights_military:   drawFlightAltBIcon,
  satellites:         drawDiamondIcon,
  earthquakes:        drawEarthquakeIcon,
}
```

### Dynamic registration

When the frontend loads layer definitions from the backend (`GetLayers` API), each declarative source definition calls `registerDynamicIcon()`:

```typescript
registerDynamicIcon(layerType, {
  shape: 'fire',        // looked up in SHAPE_TO_DRAW_FN
  rotatable: false,
  scale: 1.0,
})
```

Dynamic registrations are idempotent: if the same shape/rotatable/scale combination is registered again (e.g., on a React Query refetch), the function returns early without clearing the canvas cache.

### Canvas caching

Canvases are cached per `layerType:color` string key. The cache is organized by layer type so that re-registration of a single layer type only invalidates canvas entries for that type, not the entire cache.

### Scale and rotation

- `getIconScale(layerType, pointSize?)` — returns the final Cesium billboard scale, combining the `pointSize` ratio from the source YAML with the icon-specific `scale` multiplier and the 2× canvas compensation factor.
- `supportsRotation(layerType)` — returns `true` for layer types that should rotate the billboard to match the entity's heading (e.g., aircraft).

---

## Running the Icon Preview

The preview page renders all 55/56 registered shapes (55 preview entries; 56 registry entries including the `satellite` alias) in a grid with interactive controls.

**Start the dev server from the `frontend/` directory:**

```bash
cd frontend
npm run dev
```

Vite starts on port 3300. Then open:

```
http://localhost:3300/icon-preview.html
```

### Preview controls

| Control | Options | Description |
|---|---|---|
| Color picker | Any hex color | Sets the icon color. Default: `#00ff9d` (primary green) |
| Scale | 1× (32 px), 2× (64 px), 4× (128 px) | Display size. The canvas is always 64×64 physical; CSS scales it. |
| Background | Black, Dark gray, Navy, White, Earth map | Changes the page background behind all icons. |
| Hide "yes" | Checkbox | Hides icons already marked as approved (status: yes), useful for reviewing work-in-progress shapes. |

### Status indicators

Each cell has a colored left border reflecting the icon's approval status:

| Color | Status | Meaning |
|---|---|---|
| Green `#00ff9d` | `yes` | Approved, production-quality |
| Orange `#ff9d00` | `maybe` | Candidate, needs review |
| Red `#ff006e` | `no` | Known problem, needs redesign |
| Yellow `#ffff00` | `needs-improvement` | Works but has a specific flaw |

The **Earth map background** is the most useful option for contrast testing. Many icons that look fine on solid black become illegible against satellite imagery, especially lighter-colored entities at small scale.

---

## Design Guidelines

### Fundamental rule

Draw with `ctx.fill()` only. Never call `ctx.stroke()` on the primary shape elements. Strokes render inconsistently at small sizes, produce anti-aliasing artifacts, and break the visual language of the system.

The `wind` icon is the only approved exception: its curved swoosh lines cannot be represented as filled shapes and intentionally use `ctx.strokeStyle = '#000000'`.

### The three-layer pattern

Most successful icons follow this exact sequence:

1. **Filled outer shape** — draw the icon boundary in `color`
2. **Black cutout** — draw a slightly smaller concentric shape in `#000000` to create an "outlined" effect
3. **Center detail** — draw the symbolic content or a small dot back in `color`

This produces an icon that reads as "color outline, black interior, color symbol" — high contrast against both dark and light backgrounds.

**Reference implementation — `diamond` (`diamondIcon.ts`):**

```
Outer diamond  r=11  → color
Inner diamond  r=6   → #000000
Center dot     r=2.5 → color
```

The diamond is the gold standard. Any new icon should be evaluated against it.

### Circle-ring container pattern

Icons whose symbolic silhouette is not immediately recognizable at 32×32 should be enclosed in a **circle-ring container**: a filled circle, a black cutout circle, and the symbol drawn back in color inside the cutout.

```
Filled circle  r=14  → color       (outer ring)
Black circle   r=11  → #000000     (creates the ring)
Symbol inside          → color
```

Existing icons using this pattern: `radiation`, `lightning`, `biohazard`, `circle-ring`, `crosshair`.

### Strong-silhouette icons

Icons with strong, immediately recognizable silhouettes can stand alone without a circle-ring container. The silhouette itself provides enough contrast.

Approved standalone silhouettes: `nuclear` (mushroom cloud), `tornado` (funnel), `skull`, `ship`, `flight` (aircraft plan view).

### Sizing conventions

All coordinates are in 32×32 logical units. Canvas center is `CX = 16, CY = 16`. Standard radius for a full-bleed circle-ring outer circle is 13–14. Leave 2–3 pixels of padding at each edge to prevent clipping when Cesium scales billboards.

### Heading rotation

Icons intended for entities that travel in a direction (aircraft, missiles) should point "up" (north, toward `y = 0`) in their resting orientation. The registry rotates the Cesium billboard based on heading at render time. Setting `rotatable: true` in the source YAML is required.

---

## Shape Catalog

The table below lists every shape name, its source file, a brief description, its approval status, and which `sources.d` files currently reference it.

Shape names are the exact strings used in the `display.icon.shape` field of a YAML source definition.

| Shape name | Source file | Description | Status | Used by |
|---|---|---|---|---|
| `flight` | `flightIcon.ts` | Top-down commercial aircraft silhouette, swept wings, points north | yes | `adsb_lol_flights`, `open_sky_flights` |
| `flight-alt-a` | `flightAltAIcon.ts` | Alternative aircraft silhouette variant A | maybe | — |
| `flight-alt-b` | `flightAltBIcon.ts` | Alternative aircraft silhouette variant B | maybe | `adsb_military` |
| `flight-alt-c` | `flightAltCIcon.ts` | Alternative aircraft silhouette variant C | maybe | — |
| `diamond` | `diamondIcon.ts` | Four-point rhombus with inner cutout and center dot; gold standard pattern | yes | `celestrak_satellites`, `tle_api_satellites` |
| `satellite` | `diamondIcon.ts` | Alias for `diamond`; retained for backward compatibility | yes | — |
| `ripple` | `earthquakeIcon.ts` | Concentric ring seismic wave pattern | yes | `usgs_earthquakes`, `emsc_earthquakes` |
| `radio` | `radioIcon.ts` | Radio/signal tower symbol | needs-improvement | `sondehub_radiosondes`, `meshtastic_nodes`, `aprs_fi_stations` |
| `warning` | `warningIcon.ts` | Filled triangle with inner cutout and exclamation mark | yes | `noaa_weather_alerts`, `gdacs_disasters`, `cloudflare_radar_outages`, `copernicus_ems`, `aviationweather_sigmets`, `reliefweb_disasters`, `ukraine_air_raids`, `who_disease_outbreaks` |
| `radiation` | `radiationIcon.ts` | Circle-ring container with three-blade trefoil radiation symbol | yes | `safecast_radiation`, `epa_radnet` |
| `fire` | `fireIcon.ts` | Flame silhouette | yes | `nasa_firms_fires`, `nifc_wildfires` |
| `missile` | `missileIcon.ts` | Missile silhouette, points north | no | `spacedevs_launches` |
| `drone` | `droneIcon.ts` | Top-down quadcopter frame | yes | — |
| `helicopter` | `helicopterIcon.ts` | Top-down helicopter silhouette | yes | — |
| `shield` | `shieldIcon.ts` | Heraldic shield shape | yes | `cbp_border_wait`, `peeringdb_facilities` |
| `crosshair` | `crosshairIcon.ts` | Circle-ring container with crosshair lines | yes | — |
| `radar` | `radarIcon.ts` | Radar dish / sweep symbol | no | — |
| `explosion` | `explosionIcon.ts` | Starburst explosion shape | yes | `bellingcat_ukraine`, `acled_conflicts` |
| `tank` | `tankIcon.ts` | Top-down tank silhouette | no | — |
| `ship` | `shipIcon.ts` | Top-down naval vessel with pointed bow, inset hull cutout | yes | `aisstream_ships` |
| `anchor` | `anchorIcon.ts` | Nautical anchor | no | `noaa_buoys`, `submarine_cable_landings` |
| `wave` | `waveIcon.ts` | Ocean wave shape | no | — |
| `lightning` | `lightningIcon.ts` | Circle-ring container with lightning bolt | no | `blitzortung_lightning` |
| `cloud` | `cloudIcon.ts` | Cloud silhouette | no | `openaq_air_quality`, `purpleair_air_quality` |
| `wind` | `windIcon.ts` | Filled square with black wind swoosh strokes (exception to the fill-only rule) | no | — |
| `tornado` | `tornadoIcon.ts` | Funnel silhouette with rotation band strokes | yes | — |
| `snowflake` | `snowflakeIcon.ts` | Six-pointed snowflake | yes | — |
| `tower` | `towerIcon.ts` | Transmission or cell tower | maybe | — |
| `marker` | `markerIcon.ts` | Teardrop map pin | yes | — |
| `building` | `buildingIcon.ts` | Front elevation of a building | no | — |
| `bridge` | `bridgeIcon.ts` | Bridge span silhouette | no | — |
| `factory` | `factoryIcon.ts` | Factory with chimney stacks | maybe | — |
| `powerplant` | `powerplantIcon.ts` | Power plant with cooling towers | maybe | — |
| `crane` | `craneIcon.ts` | Construction crane | no | — |
| `warehouse` | `warehouseIcon.ts` | Warehouse / storage building | no | — |
| `biohazard` | `biohazardIcon.ts` | Circle-ring container with three-lobe biohazard symbol | yes | — |
| `skull` | `skullIcon.ts` | Skull with dome cranium, eye socket cutouts, and tooth detail | no | — |
| `flood` | `floodIcon.ts` | Flood / rising water symbol | no | — |
| `hurricane` | `hurricaneIcon.ts` | Hurricane spiral | no | — |
| `meteor` | `meteorIcon.ts` | Falling meteor with trail | no | — |
| `tree` | `treeIcon.ts` | Conifer tree silhouette | maybe | — |
| `crack` | `earthquakeCrackIcon.ts` | Ground crack / fault line | maybe | — |
| `star` | `starIcon.ts` | Five-pointed star | yes | — |
| `hexagon` | `hexagonIcon.ts` | Regular hexagon | yes | — |
| `circle-ring` | `circleRingIcon.ts` | Plain circle-ring (outer ring, black cutout, center dot); bare container pattern | yes | — |
| `chevron` | `chevronIcon.ts` | Upward-pointing chevron | no | — |
| `triangle` | `triangleIcon.ts` | Solid upward triangle | yes | — |
| `pentagon` | `pentagonIcon.ts` | Regular pentagon | yes | — |
| `cross` | `crossIcon.ts` | Plus-sign cross | yes | — |
| `rocket` | `rocketIcon.ts` | Rocket silhouette, points north | yes | — |
| `telescope` | `telescopeIcon.ts` | Telescope / observatory silhouette | no | — |
| `bullseye` | `bullseyeIcon.ts` | Concentric rings target / bullseye | yes | — |
| `nuclear` | `nuclearIcon.ts` | Mushroom cloud silhouette (standalone, no ring container) | yes | — |
| `iss` | `issIcon.ts` | ISS top-down: truss + four solar panel pairs with cross-grid cutouts | maybe | `iss_position` |
| `volcano` | `volcanoIcon.ts` | Volcano silhouette; fully registered in `iconRegistry.ts` and `_preview.ts` (status: `no`) | no | `smithsonian_volcanoes` |
| `dot` | `defaultIcon.ts` | Small filled circle; default fallback for unregistered shapes | yes | (fallback), `noaa_space_weather` |

**Note on `satelliteIcon.ts` (orphaned):** A `satelliteIcon.ts` file exists in the icons directory but is **orphaned** — the registry maps the `satellite` shape key to `drawDiamondIcon` (via the `diamond` alias) rather than importing `drawSatelliteIcon`. This file is dead code and can be removed or repurposed if a distinct satellite icon is ever needed.

---

## Source Icon Assignments

| Source file | Display name | Shape | Color (from YAML) |
|---|---|---|---|
| `acled_conflicts.yaml` | ACLED Armed Conflict Events | `explosion` | `#ff4444` |
| `adsb_lol_flights.yaml` | ADSB.lol Flights | `flight` | per source |
| `adsb_military.yaml` | Military Flights | `flight-alt-b` | per source |
| `aisstream_ships.yaml` | AIS Ship Tracking | `ship` | `#4fc3f7` |
| `aprs_fi_stations.yaml` | APRS Radio Stations | `radio` | `#00ff88` |
| `aviationweather_sigmets.yaml` | Aviation SIGMETs | `warning` | `#ff9800` |
| `bellingcat_ukraine.yaml` | Bellingcat Ukraine Civilian Harm | `explosion` | `#ff0033` |
| `blitzortung_lightning.yaml` | Lightning Strikes | `lightning` | `#ffeb3b` |
| `cbp_border_wait.yaml` | CBP Border Wait Times | `shield` | per source |
| `celestrak_satellites.yaml` | CelesTrak Satellites | `diamond` | per source |
| `cloudflare_radar_outages.yaml` | Cloudflare Radar Internet Outages | `warning` | `#ff6600` |
| `copernicus_ems.yaml` | Copernicus EMS Activations | `warning` | `#1976d2` |
| `emsc_earthquakes.yaml` | EMSC Earthquakes | `ripple` | per source |
| `epa_radnet.yaml` | EPA RadNet | `radiation` | `#ffcc00` |
| `gdacs_disasters.yaml` | GDACS Disaster Alerts | `warning` | per source |
| `gdelt_events.yaml` | GDELT Global Events | *(disabled — API returns 404)* | — |
| `iss_position.yaml` | ISS Position | `iss` | per source |
| `meshtastic_nodes.yaml` | Meshtastic LoRa Mesh Nodes | `radio` | `#9b59b6` |
| `nasa_firms_fires.yaml` | NASA FIRMS Active Fires | `fire` | per source |
| `nifc_wildfires.yaml` | US Wildfires | `fire` | `#ff6d00` |
| `noaa_buoys.yaml` | NOAA Ocean Buoys | `anchor` | per source |
| `noaa_space_weather.yaml` | Space Weather | `dot` | `#ffcc00` |
| `noaa_weather_alerts.yaml` | NWS Weather Alerts | `warning` | per source |
| `open_sky_flights.yaml` | OpenSky Flights | `flight` | per source |
| `openaq_air_quality.yaml` | Air Quality (PM2.5) | `cloud` | per source |
| `peeringdb_facilities.yaml` | PeeringDB Facilities | `shield` | `#00ccff` |
| `purpleair_air_quality.yaml` | Air Quality — PurpleAir (PM2.5) | `cloud` | `#88cc00` |
| `reliefweb_disasters.yaml` | ReliefWeb Disasters (UN OCHA) | `warning` *(disabled — requires approved appname)* | `#ff8c00` |
| `safecast_radiation.yaml` | Safecast Radiation | `radiation` | per source |
| `smithsonian_volcanoes.yaml` | Smithsonian Volcanoes | `volcano` | per source |
| `sondehub_radiosondes.yaml` | SondeHub Radiosondes | `radio` | per source |
| `spacedevs_launches.yaml` | Upcoming Launches | `missile` | `#00bcd4` |
| `submarine_cable_landings.yaml` | Submarine Cable Landing Points | `anchor` | `#00bfff` |
| `tle_api_satellites.yaml` | TLE API Satellites | `diamond` | per source |
| `ukraine_air_raids.yaml` | Ukraine Air Raid Alerts | `warning` *(disabled — requires API key)* | `#ff0000` |
| `usgs_earthquakes.yaml` | USGS Earthquakes | `ripple` | per source |
| `who_disease_outbreaks.yaml` | WHO Disease Outbreak News | `warning` *(disabled — no coordinates in API)* | `#ff0044` |

---

## How to Add a New Icon

Follow these four steps in order. All paths are relative to `frontend/apps/earth/src/features/globe/icons/`.

### Step 1 — Create the icon file

Create `xxxIcon.ts`. Export a single draw function named `drawXxxIcon`.

```typescript
// xxxIcon.ts
const SIZE = 32
const CX = SIZE / 2
const CY = SIZE / 2

export function drawXxxIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Step 1: filled outer shape
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.arc(CX, CY, 14, 0, Math.PI * 2)
  ctx.fill()

  // Step 2: black cutout
  ctx.fillStyle = '#000000'
  ctx.beginPath()
  ctx.arc(CX, CY, 11, 0, Math.PI * 2)
  ctx.fill()

  // Step 3: symbol or center dot
  ctx.fillStyle = color
  ctx.beginPath()
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2)
  ctx.fill()
}
```

Rules to follow:
- Use `ctx.fill()` only on primary shapes. Avoid `ctx.stroke()`.
- Coordinates are in 32×32 logical units. Leave 2–3 px margin at all edges.
- Icons for directional entities (aircraft, rockets) must point north (up, toward `y = 0`).
- Use `#000000` for all cutout fills. Do not use semi-transparent values.

### Step 2 — Register in `iconRegistry.ts`

Add the import and a `SHAPE_TO_DRAW_FN` entry in `frontend/apps/earth/src/features/globe/icons/iconRegistry.ts`.

```typescript
// At the top of the file with other imports:
import { drawXxxIcon } from './xxxIcon'

// Inside SHAPE_TO_DRAW_FN:
const SHAPE_TO_DRAW_FN: Record<string, IconDrawFn> = {
  // ... existing entries ...
  xxx: drawXxxIcon,
}
```

### Step 3 — Register in `_preview.ts`

Add the import and an `ICONS` array entry in `frontend/apps/earth/src/features/globe/icons/_preview.ts`.

```typescript
// At the top of the file with other imports:
import { drawXxxIcon } from './xxxIcon'

// Inside the ICONS array:
const ICONS: IconEntry[] = [
  // ... existing entries ...
  { name: 'xxx', drawFn: drawXxxIcon, status: 'maybe' },
]
```

Set `status` to `'maybe'` initially. Promote to `'yes'` once the icon has been tested against multiple backgrounds and approved.

### Step 4 — Reference in a YAML source

In any `sources.d/*.yaml` file, set:

```yaml
display:
  icon:
    shape: xxx
    rotatable: false   # set true if the entity has a heading
    scale: 1.0
```

The shape name must exactly match the key added to `SHAPE_TO_DRAW_FN` in step 2.

### Verification checklist

Before promoting an icon's status to `yes`:

- [ ] Renders clearly at 1× (32 px) on the Earth map background
- [ ] Readable at 2× (64 px) on black background
- [ ] No visible clipping at canvas edges (check with 4× scale)
- [ ] Color and black cutout visible against white background (inverted contrast test)
- [ ] If rotatable: north-facing orientation confirmed by checking heading = 0 at runtime
- [ ] `_preview.ts` entry added and visible in the preview grid
