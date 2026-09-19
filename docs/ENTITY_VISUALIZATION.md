# Entity Visualization System

**Audience**: Engineers maintaining or extending the Respondent platform's real-time entity rendering pipeline.

**Scope**: Full data flow from feeder ingestion through Valkey caching, WebSocket streaming, Zustand store, and CesiumJS billboard rendering on the 3D globe. Covers tiered retrieval (including on-demand fetching and continuous polling), predictive interpolation, time range control, spatial indexing, data provenance, and known performance characteristics.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Data Pipeline](#2-data-pipeline)
3. [Backend Components](#3-backend-components)
4. [Frontend Rendering Pipeline](#4-frontend-rendering-pipeline)
5. [Spatial Indexing and Viewport Management](#5-spatial-indexing-and-viewport-management)
6. [Data Provenance and Visual Encoding](#6-data-provenance-and-visual-encoding)
7. [Performance Profile](#7-performance-profile)
8. [Configuration Reference](#8-configuration-reference)
9. [Known Issues and Recent Fixes](#9-known-issues-and-recent-fixes)
10. [Next Steps](#10-next-steps)

---

## 1. Overview

The entity visualization system renders geospatial entities (flights, satellites, earthquakes) as billboards on a CesiumJS 3D globe. The system is built around four tiers of data retrieval (with Tier 1.5 on-demand fetch and continuous polling for sparse regions), client-side predictive interpolation for smooth motion between server updates, a real-time WebSocket channel, and a render-on-demand CesiumJS configuration.

### High-Level Architecture

```
 ┌──────────────────────────────────────────────────────────────────────┐
 │  FEEDER PROCESS                                                      │
 │                                                                      │
 │  ADSBLolAdapter  ──hex grid──► round-robin crawl ──► SetEntities()  │
 │  SatelliteAdapter ─────────────────────────────────► SetEntities()  │
 │  EarthquakeAdapter ────────────────────────────────► SetEntities()  │
 │         │                                                            │
 │         ▼                                                            │
 │  ValkeyClient.SetEntity()                                            │
 │   ├─ HSET  entity:{layerType}:{externalID}   (TTL 60s / 300s)       │
  │   ├─ SADD  layer:{layerType}:index           (TTL entity+30s)      │
 │   └─ GEOADD layer:{layerType}:geo            (no TTL — global idx)  │
 │         │                                                            │
 │         ▼                                                            │
 │  redis.Publish("layer:{layerType}:updates", payload)                 │
 └──────────────────────────────────────────────────────────────────────┘
          │  Pub/Sub
          ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  RESPONDENT API PROCESS                                              │
 │                                                                      │
 │  WebSocket Server                                                    │
  │   ├─ listenForUpdates()  PSubscribe("layer:*:updates")              │
  │   │    └─► flushBatchedUpdates()  → layer.batch_update per client   │
 │   │                                                                  │
 │   ├─ handleSubscribe()                                               │
 │   │    ├─ Tier 1: ValkeyAdapter.GetLayerEntitiesByBBox()            │
 │   │    │    └─ (internally: GC-aware overfetch + lazy prune when    │
 │   │    │        geo_cache config exists for the layer)               │
 │   │    └─ [if sparse] fetchSparseRegion()                            │
 │   │         ├─ Tier 1.5: doOnDemandFetch() → HTTP API (concurrent)  │
 │   │         ├─ Tier 2: doBackfill() (concurrent)                    │
 │   │         └─ startOnDemandTicker() (continuous polling)            │
 │   │                                                                  │
 │   ├─ handleViewportUpdate()                                          │
 │   │    ├─ Tier 1: ValkeyAdapter.GetLayerEntitiesByBBox()            │
 │   │    │    └─ (same GC-aware path as above)                        │
 │   │    ├─ [if sparse] fetchSparseRegion() (same as above)           │
 │   │    └─ [if not sparse] stopOnDemandTicker()                      │
 │   │                                                                  │
 │   └─ handleTimeRange()                                               │
 │        └─ obsRepo.GetLatestForLayerByBBox() or GetLayerSnapshotAt() │
 └──────────────────────────────────────────────────────────────────────┘
          │  WebSocket (ws://)
          ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  FRONTEND                                                            │
 │                                                                      │
 │  wsClient (singleton)                                                │
 │   └─ message routing → useLayerStream() handler                     │
 │        ├─ snapshot / layer_update / layer.snapshot → immediate store │
 │        └─ layer.update → RAF batch → store upsert                   │
 │                                                                      │
 │  useUIStore (Zustand)                                                │
 │   └─ layerEntities: Map<layerId, {entityMap, obsMap}>               │
 │        └─ layerVersions: Record<layerId, number>                     │
 │                                                                      │
 │  BillboardLayerRenderer (per enabled layer)                          │
 │   ├─ Effect 1: BillboardCollection lifecycle                         │
 │   ├─ Effect 2: billboard upsert / rebuild on version change         │
 │   ├─ Effect 2b: preUpdate interpolation + dead-reckoning (flights)  │
 │   ├─ Effect 2c: occlusion dirty flag on showOccluded change         │
 │   ├─ Effect 3: postRender occlusion fading (EllipsoidalOccluder)    │
 │   └─ Effect 4: entity isolation (show/hide by entityId)             │
 └──────────────────────────────────────────────────────────────────────┘
          │  Priority feedback loop
          ▼
 ┌──────────────────────────────────────────────────────────────────────┐
 │  PRIORITY CRAWLING                                                   │
 │                                                                      │
 │  WebSocket Server publishPrioritySignal()                            │
 │   └─ redis.Publish("viewport:priority:requests", bbox)              │
 │        └─► PrioritySubscriber (feeder process)                      │
 │               └─► ADSBLolAdapter.selectBatch() Phase 1              │
 └──────────────────────────────────────────────────────────────────────┘
```

### Key Design Decisions

- **Render-on-demand**: `requestRenderMode: true` means Cesium only renders when explicitly told to, or on camera movement. This keeps CPU/GPU idle when nothing is changing.
- **Viewport-progressive loading**: Spatial layers send only the entities within the current camera view, not the global dataset.
- **Tiered retrieval**: In **live mode**, the frontend sends `subscribe` messages (not `time_range`), and the backend serves data from the Valkey cache — no Postgres query, no time window filtering. Spatial layers include a viewport so the backend uses `GEOSEARCH`; non-spatial layers omit the viewport and receive the full cache index. On-demand fetch (Tier 1.5) fills gaps for spatial layers by directly calling external APIs; Postgres backfill provides recent historical data; continuous polling sustains data flow in sparse regions; priority crawling closes the loop by reordering the feeder. In **range mode**, the frontend sends `time_range` messages which query Postgres with explicit time bounds.
- **In-place store mutation with version counters**: `layerEntities` Map is mutated in-place for O(1) upserts. Selectors use `layerVersions` as the change signal so only affected layers trigger re-renders.

---

## 2. Data Pipeline

### Stage 1: Feeder Ingestion

Adapters (e.g., `ADSBLolAdapter`) poll external APIs on a configurable interval. For each aircraft/satellite/event they produce a `domain.Entity` and a `domain.Observation`. These are passed to `SetEntities()` on the base `PollingAdapter`, which calls `ValkeyClient.SetEntity()` for each pair.

`SetEntity()` executes a Redis pipeline with three operations:

1. `HSET entity:{layerType}:{externalID}` — stores entity fields and observation data as a flat hash. TTL is 60s for flights, 300s for satellites.
2. `SADD layer:{layerType}:index` — adds the external ID to the layer membership set, TTL set to entity TTL + 30s buffer so the index outlives individual entity hashes.
3. `GEOADD layer:{layerType}:geo` — adds the entity to the geo sorted set using its lat/lon. **This key has no TTL** — it is a persistent global index. Expired entity hashes are filtered at query time when `HGetAll` returns empty.

After writing to Valkey, the adapter publishes to `layer:{layerType}:updates` via Redis Pub/Sub. The payload is the `WSMessage` JSON that the WebSocket server will forward to subscribed clients.

Relevant files:
- `internal/cache/valkey.go` — `SetEntity()`, `GetLayerEntitiesByBBox()`
- `sources.d/adsb_lol_flights.yaml` — declarative source definition; `internal/ingest/declarative/spatial.go` — spatial crawling logic (`fetchAndProcess()`, `selectBatch()`)

### Stage 2: WebSocket Server (Tiered Retrieval)

The WebSocket server (`internal/realtime/websocket.go`) maintains one goroutine per client (read pump + write pump). The frontend sends two types of subscription messages:

- **`subscribe`** (live mode) — the server reads from the Valkey cache. Spatial layers (those with `filtering_mode: viewport`) include a viewport for `GEOSEARCH`; non-spatial layers omit the viewport and receive the full cache index via `SMEMBERS`. Pub/Sub real-time updates are enabled for all subscribed layers.
- **`time_range`** (range mode) — the server queries Postgres with explicit `[from, to]` time bounds.

When a client subscribes (live mode) or updates its viewport, the server executes a multi-tier retrieval:

**Tier 1 — Valkey geo query (GC-aware)**

`ValkeyAdapter.GetLayerEntitiesByBBox()` issues `GEOSEARCH BYBOX` centered on the viewport. It computes the box center and dimensions in km using the haversine formula. Results are pipeline-fetched via `HGetAll`. Antimeridian wrapping is handled by shifting the center longitude when `west > east`.

For layers with a `geo_cache` configuration (see §8.9), the adapter transparently applies **over-fetch + lazy GC**:

1. Over-fetches by `overfetch_ratio` (e.g., 3×) to compensate for stale geo index members
2. Separates results into alive (hash exists) and dead (hash expired) sets
3. If `alive_ratio < alive_ratio_threshold`, prunes dead members via `ZREM` (capped at `gc_batch_size`)
4. If alive count is still below the requested limit, performs a second `GEOSEARCH` to backfill the deficit (dead members are now removed)

This is encapsulated entirely within the adapter — the WebSocket handler calls a single `GetLayerEntitiesByBBox()` method and never needs to know whether GC occurs (DIP: adapter encapsulates optimizations).

**Sparse Region Handling — `fetchSparseRegion()`**

If Tier 1 returns fewer than `threshold` entities (default 10, configurable per layer), the server calls `fetchSparseRegion()` which launches Tier 1.5 and Tier 2 concurrently as goroutines, merges results with deduplication, and optionally starts continuous polling.

Relevant file: `internal/realtime/websocket.go` — `fetchSparseRegion()`

**Tier 1.5 — On-demand fetch**

`doOnDemandFetch()` makes a single HTTP call to the external API (e.g., adsb.lol) for the viewport center. The response is parsed via the shared `parse.ParseADSBLolResponse()` function (`internal/ingest/parse/adsblol.go`), written through the standard `SetEntity()` → Valkey + Pub/Sub pipeline, and delivered to the client as `source: "live"` entities with full opacity.

This is a parallel, additive path — the feeder's global crawl continues unmodified. On-demand data enters the cache and Postgres identically to feeder-produced data, so subsequent viewport updates find it via Tier 1 without re-fetching.

Rate limiting: per-client, per-layer cooldown tracked via `lastOnDemand` on the `Client` struct. The `canOnDemand()` method enforces the cooldown (default 10s, configurable).

Relevant file: `internal/realtime/websocket.go` — `doOnDemandFetch()`, `doOnDemandFetchCore()`

**Continuous On-Demand Polling**

When Tier 1.5 fires, `startOnDemandTicker()` launches a per-client, per-layer polling goroutine that re-fetches from the external API at a configured interval (e.g., 30s for flights). This turns the one-shot Tier 1.5 fetch into a sustained data stream. The ticker uses `doOnDemandFetchDirect()` which bypasses the cooldown check since the ticker interval itself is the rate limiter.

The ticker stops when:
- The client disconnects (`stopAllOnDemandTickers()` called during unregister)
- The client moves to a non-sparse region (Tier 1 returns sufficient entities → `stopOnDemandTicker()`)
- The client unsubscribes from the layer

Relevant file: `internal/realtime/websocket.go` — `startOnDemandTicker()`, `runOnDemandPoll()`, `stopOnDemandTicker()`

**Tier 2 — Postgres backfill**

`doBackfill()` calls `obsRepo.GetLatestForLayerByBBox()`. This query uses `CROSS JOIN LATERAL` to return the single latest observation per entity within the spatial and temporal bounds. Results are tagged `source: "stale"`.

Backfill also triggers `publishPrioritySignal()`, which publishes the viewport bbox to `viewport:priority:requests`. The feeder's `PrioritySubscriber` receives this and instructs the next `selectBatch()` to prioritize overlapping hex grid cells.

**Merge Strategy**: On-demand results are merged first (they carry `source: "live"`, full opacity). Backfill results are filtered to exclude any ExternalIDs already present from cache or on-demand. Live entities win over stale. On-demand's `SetEntity()` call also overwrites stale Valkey cache entries, so subsequent Tier 1 queries return the fresh data.

**Tier 3 — Time range query**

When a client sends a `time_range` message, `doTimeRangeQuery()` calls either:
- `GetLatestForLayerByBBox()` when a viewport is available
- `GetLayerSnapshotAt()` for a global layer-wide snapshot at a point in time

Results are tagged `source: "historical"`. Live Pub/Sub updates are suppressed for frozen ranges (`timeTo` more than 10 minutes in the past).

### Stage 3: WebSocket Transport

The server sends three types of bulk messages toward the client:

| Message Type | When Sent | Handler |
|---|---|---|
| `snapshot` | On subscribe, viewport update, or time range query — targeted to one client | `SendSnapshotToClient()` |
| `layer.snapshot` | Broadcast to all subscribed clients | `BroadcastSnapshot()` |
| `layer_update` | Broadcast to subscribed clients | `BroadcastLayerUpdate()` |
| `layer.update` | Per-entity result from on-demand fetch or backfill | `doOnDemandFetchCore()`, `doBackfill()` |

For spatial layers, `flushBatchedUpdates()` extracts each update's lat/lon from the Pub/Sub payload and skips clients whose stored viewport does not contain that position.

### Stage 4: Frontend WebSocket Client

`wsClient` (`frontend/apps/web/src/shared/api/websocket.ts`) is a singleton `WebSocketClient` instance. It maintains a handler registry keyed by message type and tracks subscribed layers for reconnection replay.

Reconnection uses exponential backoff with jitter: `delay = min(baseDelay * 2^attempt + jitter, 30s)`, where `jitter` is a random value up to 50% of the base delay. Capped at 5 attempts.

### Stage 5: Normalization and Store Writes

`useLayerStream()` (`frontend/apps/web/src/shared/api/useLayerStream.ts`) registers handlers for all four incoming message types:

- **`snapshot`, `layer_update`, `layer.snapshot`**: Applied immediately via `setLayerEntities()`. The raw Go snake_case JSON is normalized to camelCase TypeScript at this boundary via `normalizeEntity()` and `normalizeObservation()`.
- **`layer.update`**: Per-entity Pub/Sub messages are buffered in `batchRef` and flushed as a group via `requestAnimationFrame`. This coalesces potentially hundreds of per-second entity updates into one store write per frame.

`setLayerEntities()` in the Zustand store mutates the `entityMap` and `obsMap` in-place using `Map.set()`. The outer `layerEntities` Map is also mutated in-place. Only `layerVersions[layerId]` is incremented, producing a new object reference that triggers Zustand's selector comparison.

### Stage 6: CesiumJS Rendering

`useLayerEntities()` subscribes to `layerVersions[layerId]` as its change signal. On version change, `useMemo` materializes `Array.from(entityMap.values())` and `Array.from(obsMap.values())`, applying the `maxEntities` cap. The resulting arrays are passed as props to `BillboardLayerRenderer`.

`BillboardLayerRenderer` drives five `useEffect` hooks that manage the CesiumJS `BillboardCollection` lifetime and content. See [Section 4](#4-frontend-rendering-pipeline) for the full effect breakdown.

---

## 3. Backend Components

### 3.1 File Responsibilities

| File | Primary Responsibility |
|---|---|
| `internal/realtime/websocket.go` | WebSocket server: client registry, read/write pumps, subscription and viewport handlers, tiered retrieval orchestration, time range handling, priority signal publishing, per-client state |
| `internal/storage/cache/valkey_adapter.go` | Cache storage adapter (implements `repo.SpatialCacheStorage`): `GetLayerEntitiesByBBox()` with transparent GC-aware overfetch for spatial layers, `SetEntity()`, `GetLayerCount()`, `ClearLayer()` |
| `internal/cache/valkey.go` | Lower-level Valkey client: `SetEntity()`, `GetLayerEntitiesByBBoxWithGC()`, `GetLayerEntitiesPaginated()` (SSCAN), key schema, TTL management |
| `internal/repo/postgres/observation.go` | `GetLayerSnapshotAt()` (temporal point-in-time), `GetLatestForLayerByBBox()` (spatial+temporal), `GetLatestByCurrentPositionInBBox()` (position-filtered latest), `CreateBatch()` / `CreateBatchUpsert()` |
| `internal/ingest/grid/grid.go` | `GenerateGlobalGrid()` — hex-packed point coverage, `ComputeBatchSize()` |
| `internal/ingest/priority/subscriber.go` | `PrioritySubscriber` — Pub/Sub listener, TTL-based region queue, deduplication by 2° grid |
| `internal/ingest/declarative/spatial.go` | Spatial crawling — `fetchAndProcess()`, `selectBatch()` with Phase 1 priority / Phase 2 round-robin |
| `internal/ingest/parse/adsblol.go` | `ParseADSBLolResponse()` — shared parser for adsb.lol API responses, used by both the feeder adapter and on-demand fetch |

### 3.2 Valkey Key Schema

| Key Pattern | Type | TTL | Purpose |
|---|---|---|---|
| `entity:{layerType}:{externalID}` | Hash | 60s (flights) / 300s (satellites) | Entity fields + latest observation |
| `layer:{layerType}:index` | Set | Entity TTL + 30s buffer | Membership index for non-spatial scans |
| `layer:{layerType}:geo` | Sorted Set (geo) | None | GEOSEARCH spatial index — persistent global index |
| `layer:{layerType}:count` | String | — | Entity count cache |

The geo index (`layer:{layerType}:geo`) intentionally has no expiry. Members accumulate as entities are written. When an entity hash expires, the geo index member remains as a stale entry. Callers that pipeline `HGetAll` after `GEOSEARCH` skip members that return empty hashes (`len(data) == 0`).

**Lazy GC**: For layers with a `geo_cache` configuration, the `ValkeyAdapter` performs lazy garbage collection during spatial queries. When the ratio of alive-to-total members drops below `alive_ratio_threshold`, dead members are pruned from the geo sorted set via `ZREM` (capped at `gc_batch_size` per query). This keeps the geo index from growing unboundedly while avoiding the need for a separate background cleanup process. See §8.9 for configuration.

### 3.3 WebSocket Protocol

All messages are JSON with this envelope:

```json
{
  "type": "...",
  "layer_id": "...",
  "data": { ... }
}
```

**Client to Server**

| Message Type | `data` Shape | Behavior |
|---|---|---|
| `subscribe` | `{ viewport?: BBox }` | Enable layer. Resets time range state to live mode. Triggers Tier 1 + optional Tier 2 snapshot. |
| `unsubscribe` | `{}` | Disable layer. Clears subscription, viewport, and time range state. |
| `viewport_update` | `BBox` (`west`, `south`, `east`, `north` as floats) | Update stored viewport. Triggers geo query + optional backfill. If time range is active, re-runs time range query with new viewport. |
| `time_range` | `{ from: RFC3339, to: RFC3339, viewport?: BBox }` | Execute bounded query. Sets frozen mode if `to` is more than 10 minutes in the past. |

**Server to Client**

| Message Type | `data` Shape | When Sent |
|---|---|---|
| `snapshot` | `{ entities: [...], observations: [...] }` | Targeted: on subscribe, viewport update, time range query |
| `layer_update` | `{ entities: [...], observations: [...] }` | Broadcast to subscribed clients |
| `layer.snapshot` | `{ entities: [...], observations: [...] }` | Broadcast to all clients |
| `layer.update` | `{ entity: {...}, observation: {...} }` | Per-entity Pub/Sub fanout (real-time position updates) |
| `layer.batch_update` | `{ updates: [{ entity: {...}, observation: {...} }, ...] }` | Batched Pub/Sub updates — server accumulates per-entity updates and sends them as one message |
| `indicator.update` | `{ values: [...] }` | Structured indicator values for indicator layers (merged observations) |
| `cctv.frame` | `{ camera_feed_id, frame }` | Base64-encoded CCTV camera frame |

**BBox fields**: `west`, `south`, `east`, `north` as float64 degrees. Antimeridian case: `west > east` (e.g., 170 to -170).

### 3.4 Per-Client State

Each `Client` struct in `websocket.go` tracks:

```go
subscriptions   map[string]bool               // layer_id -> enabled
 viewports       map[string]*domain.BBox        // layer_id -> current viewport
timeFrom        map[string]time.Time          // layer_id -> range start (zero = live)
timeTo          map[string]time.Time          // layer_id -> range end (zero = "now")
timeFrozen      map[string]bool               // layer_id -> suppress live Pub/Sub
lastBackfill    map[string]time.Time          // layer_id -> cooldown tracking
lastOnDemand    map[string]time.Time          // layer_id -> on-demand cooldown tracking
onDemandTickers map[string]context.CancelFunc // layer_id -> active polling ticker cancel fn
```

All fields use `sync.RWMutex` protection via `c.mu`.

`Subscribe()` resets all time range state for the layer, returning it to live mode. This is the correct behavior when a user disables and re-enables a layer.

### 3.5 Postgres Queries

**`GetLatestForLayerByBBox()`** — used for both backfill and time range queries with viewport:

```sql
SELECT
    e.id, e.external_id, e.layer_type, e.name, e.metadata,
    latest.id, latest.ts, latest.event_time, latest.event_end,
    latest.lat, latest.lon, latest.altitude_m, latest.velocity,
    latest.metadata, latest.source_type
FROM entities e
CROSS JOIN LATERAL (
    SELECT o.id, o.ts, o.event_time, o.event_end, o.lat, o.lon,
           o.altitude_m, o.velocity, o.metadata, o.source_type
    FROM observations o
    WHERE o.entity_id = e.id
      AND o.lat BETWEEN $2 AND $3
      AND o.lon BETWEEN $4 AND $5   -- or >= $4 OR <= $5 for antimeridian
      AND o.event_time >= $6 AND o.event_time <= $7
    ORDER BY o.event_time DESC
    LIMIT 1
) latest
WHERE e.layer_type = $1
LIMIT $8
```

`CROSS JOIN LATERAL` allows Postgres to use the `(entity_id, ts DESC)` index for each entity individually — ~500× faster than `DISTINCT ON` for large datasets. The lon predicate is dynamically constructed based on whether `west > east`.

**`GetLayerSnapshotAt()`** — used for time range queries without viewport:

Queries for the latest observation per entity at a specific `asOf` timestamp, within a `window` lookback. Also uses `CROSS JOIN LATERAL`. This supports historical replay: "show me the world as it was at time T."

**`GetLatestByCurrentPositionInBBox()`** — finds each entity's truly-latest observation within the time window, then filters by whether that latest position falls inside the bbox. This avoids the "jumping" problem where different viewport positions match different historical observations for the same entity. Uses `CROSS JOIN LATERAL` with the spatial predicate applied to the outer `WHERE` (on the `latest` alias) rather than inside the lateral subquery.

### 3.6 Time Range Handling

The server enforces bounds before executing any time range query. Limits are resolved **per-layer**: if a source declares a `history` block in its YAML definition, those limits override the server defaults for that layer.

**Default limits** (applied when no per-layer config exists):

- **Max lookback**: 48h from now. `from` values older than this are clamped.
- **Max span**: 24h. If `to - from > 24h`, `from` is advanced to `to - 24h`.

**Per-layer override** (from `sources.d/*.yaml`):

Sources with long-lived data (e.g., radiation, earthquakes, fires) declare extended limits:

```yaml
history:
  max_lookback: "8760h"   # 1 year
  max_range_span: "168h"  # 7 days
```

These flow through the dynamic registry (`SetHistoryConfig()` / `LookupHistoryConfig()`) and are resolved in `handleTimeRange()` before clamping. The per-layer `HistoryConfig` is also served to the frontend via the `GetLayers` API response so the custom range picker can validate client-side.

**Frozen detection**: `now - to > 10 minutes` → `frozen = true`, live Pub/Sub suppressed for this client+layer pair.

**Frontend time range picker**: The `CustomRangePicker` derives the most permissive lookback and span limits from all loaded layers (not just enabled ones), since the time range is global and the backend handles per-layer clamping independently. When browsing beyond the default 48h window the HUD shows a "HISTORICAL" badge.

The frontend sends `timeTo: null` for sliding window presets (1h/8h/24h). The server receives an explicit `to` because the frontend resolves `null` to `new Date().toISOString()` before sending.

### 3.7 Spatial Layer Registration

The server constructor accepts a `spatialLayers map[string]bool`. Layers in this map use viewport-based progressive loading; others receive unfiltered global snapshots. The frontend derives spatial layer status dynamically from the `filteringMode` field in the layer API response (`GET /v1/layers`). Any declarative source with `filtering: viewport` in its YAML is automatically treated as spatial — no frontend code changes required.

Current spatial layers (determined by `filtering: viewport` in `sources.d/*.yaml`):
- `flights_commercial` (adsb_lol_flights, open_sky_flights)
- `flights_military` (adsb_military)
- `satellites` (celes_trak_satellites, tle_api_satellites)
- `earthquakes` (usgs_earthquakes, emsc_earthquakes)
- `wildfires` (nifc_wildfires)
- `ems_activations` (copernicus_ems)
- Any future source with `filtering: viewport`

---

## 4. Frontend Rendering Pipeline

### 4.1 Component Hierarchy

```
GlobeScene
├── useLayerStream()          — WebSocket subscriptions, message handling, RAF batching
├── useViewportSync()         — debounced viewport_update to server (300ms)
├── useEntityInteraction()    — click/hover/double-click on billboards
├── useEntityTracking()       — camera tracking for entity view mode
└── BillboardLayerRenderer    — one instance per enabled layer (keyed by layerId)
    ├── Effect 1: BillboardCollection lifecycle
    ├── Effect 2: billboard data sync
    ├── Effect 2b: animation interpolation (flights only)
    ├── Effect 2c: occlusion dirty on showOccluded change
    ├── Effect 3: occlusion fading
    └── Effect 4: entity isolation
```

`GlobeScene.tsx` initializes the Cesium `Viewer` once via `useEffect`. It calls `viewer.camera.changed.addEventListener(updateCameraState)` to push camera state to `useViewerStore`. The Viewer is stored in a ref (`viewerRef`) and passed to all child renderers.

### 4.2 Zustand Store Design

Two Zustand stores:

**`useUIStore`** (`frontend/apps/web/src/app/store.ts`) — application state:
- `layerEntities: Map<string, LayerData>` — entity and observation data, keyed by layer ID
- `layerVersions: Record<string, number>` — per-layer version counter for selector invalidation
- `timeMode`, `timeFrom`, `timeTo`, `timePreset` — time range control state
- `enabledLayers: string[]` — ordered list of enabled layer IDs
- `smoothMotion: boolean` — predictive interpolation toggle (default `true`), controlled via Settings > Visibility

`LayerData` is defined as:
```typescript
interface LayerData {
  entityMap: Map<string, Entity>
  obsMap: Map<string, Observation>
}
```

`setLayerEntities()` mutates `entityMap` and `obsMap` in-place via `Map.set()`. The outer `layerEntities` Map is also mutated in-place (no new Map allocation). Only `layerVersions` produces a new object, which is the Zustand change signal.

**`useViewerStore`** (`frontend/apps/web/src/features/globe/store.ts`) — Cesium-specific state:
- `viewer: Viewer | null`
- `viewport: ViewportBBox | null` — current camera bounding box, updated on every `camera.changed` event
- `camera: CameraState | null` — lat/lon/altitude/heading/pitch/roll, persisted to `localStorage` under key `respondent:camera` with 1s debounce

### 4.3 Version-Based Selectors

`useLayerEntities(layerId)` in `useLayerStream.ts` is the selector hook used by `BillboardLayerRenderer`:

```typescript
const version = useUIStore((s) => s.layerVersions[layerId] ?? 0)

return useMemo(() => {
  const data = useUIStore.getState().layerEntities.get(layerId)
  // ... materialize arrays from Maps ...
}, [version, layerId, maxEntities, config])
```

Subscribing to `layerVersions[layerId]` rather than `layerEntities` means only the specific layer's consumers re-run when that layer's data changes. Other layers' renderers are unaffected.

The `useMemo` returns stable array references that only change when the version changes. This prevents unnecessary Cesium billboard updates.

### 4.4 BillboardLayerRenderer Effects

**Effect 1 — BillboardCollection Lifecycle**

Deps: `[viewerRef]` — runs once on mount.

Creates a `BillboardCollection` and adds it to `viewer.scene.primitives`. On cleanup, removes it from the primitive list and calls `collection.destroy()`. Also clears `billboardMapRef`, `animStateRef`, and `entitySourceRef`.

This effect is intentionally isolated from data so collection creation is stable and does not race with data updates.

**Effect 2 — Billboard Data Sync**

Deps: `[version, hasData, layerId, layerType, color, pointSize, isFlightLayer, viewerRef, maxEntities]`

This is the core rendering update. It runs on every version bump. Two code paths:

**Flight layers** (`isFlightLayer = true`): Animated diff. Iterates the incoming entity array and for each entity:
- If a billboard already exists in `billboardMapRef`, records a new `AnimationState` in `animStateRef` describing the interpolation from current position to new position. Also detects source changes for alpha animation.
- If no billboard exists, creates one and adds a trivial (from=to) `AnimationState`.
- After processing all current entities, removes billboards for departed entity IDs.

**Non-flight layers**: Full rebuild. Calls `collection.removeAll()`, then adds a new billboard for each entity. Simpler but O(N) on every update.

After both paths, sets `occlusionDirtyRef.current = true` and calls `scene.requestRender()` twice: once immediately, once on the next `requestAnimationFrame`. The second call is a safety net for React StrictMode — see [Section 9](#9-known-issues-and-recent-fixes).

**Effect 2b — Animation Interpolation + Dead-Reckoning Loop**

Deps: `[viewerRef, isFlightLayer]` — runs once per flight layer.

Registers a `scene.preUpdate` listener that handles two phases of motion: server-to-server lerp and predictive extrapolation.

**Phase 1 — Lerp (0–2s after server update):**
1. Iterates `animStateRef`. For active entries: computes `t = smoothstep((now - startTime) / duration)`, lerps position via `Cartesian3.lerp()`, lerps rotation via `CesiumMath.lerp()`, and lerps alpha if `fromAlpha !== toAlpha`.
2. The smoothstep function `t * t * (3 - 2 * t)` provides ease-in/ease-out over the 2-second interpolation window (`INTERP_DURATION_MS = 2000`).

**Phase 2 — Dead-Reckoning Extrapolation (after lerp completes):**

When the `smoothMotion` store flag is enabled (default: `true`), entities with known heading and ground speed continue moving along their heading at their last known speed after the lerp finishes. This creates the illusion of continuous motion between server updates (which may arrive every 30s).

The `AnimationState` interface carries dead-reckoning fields:
- `groundSpeedMps` — meters/second, converted from knots (`KNOTS_TO_MPS = 0.514444`)
- `headingRad` — radians for geodesic projection
- `pauseUntil` — timestamp to freeze extrapolation after backward motion detection

Position projection uses `projectPosition()` (`frontend/apps/web/src/features/globe/entityUtils.ts`) which applies spherical approximation — accurate within meters for the <60s extrapolation cap.

**Backward Motion Detection:**

When a new server update arrives, the entity's current extrapolated position becomes the `fromPos` for the new lerp. If `isBackwardMotion()` (`entityUtils.ts`) detects that the new server position is behind the current extrapolated position (>100m backward along heading, within 30° heading delta), the system snaps to the server position and pauses extrapolation for `CATCHUP_PAUSE_MS` (5000ms). This prevents visual artifacts from over-extrapolation (e.g., when aircraft decelerate or enter holding patterns).

**`interpolatedPositions` Map:**

A shared mutable `Map<string, Cartesian3>` (`frontend/apps/web/src/features/globe/interpolatedPositions.ts`) is updated every frame by Effect 2b with each billboard's current interpolated position. This provides O(1) position lookups for other components (`SelectionIndicator`, `EntityTrailRenderer`) without requiring store subscriptions. It is frame-synchronous data, not reactive state.

3. If any billboard was mutated, calls `scene.requestRender()`.

**Effect 2c — Occlusion Dirty Flag**

Deps: `[showOccluded]`

Sets `occlusionDirtyRef.current = true` when the user toggles the "show occluded entities" control. Effect 3's stable `postRender` listener picks this up on the next frame without needing to re-subscribe.

**Effect 3 — Occlusion Fading**

Deps: `[viewerRef]` — runs once per renderer instance.

Registers two stable Cesium listeners:
- `camera.changed` → calls `updateOcclusion()` immediately
- `scene.postRender` → calls `updateOcclusion()` only if `occlusionDirtyRef.current` is set (dirty flag from data rebuilds or toggle)

`updateOcclusion()` creates an `EllipsoidalOccluder` seeded with `viewer.camera.positionWC`. For each billboard, it calls `occluder.isPointVisible(billboard.position)`. Visible billboards retain their source-based alpha; occluded billboards are set to `occludedAlpha` (0.12 when "show occluded" is on, 0.0 when off). Alpha updates are skipped if the change is less than 0.01, reducing GPU uploads.

`EllipsoidalOccluder` is not in Cesium's bundled TypeScript declarations; it is imported via `(Cesium as any).EllipsoidalOccluder`.

**Effect 4 — Entity Isolation**

Deps: `[isolatedEntityId, version, viewerRef]`

When `isolatedEntityId` is non-null (user is in entity view mode for a specific entity), iterates every billboard and sets `billboard.show` based on whether `billboard.id.entityId === isolatedEntityId`. When isolation is cleared, shows all billboards. Always calls `scene.requestRender()`.

### 4.5 Icon and Color System

`getIconCanvas(layerType)` (`frontend/apps/web/src/features/globe/icons/iconRegistry.ts`) returns a prerendered HTML `Canvas` element for the given layer type. Each icon module (e.g., `flightIcon.ts`, `satelliteIcon.ts`) draws to a canvas using the 2D API.

`supportsRotation(layerType)` returns `true` for flight layers. When true, Effect 2 sets `billboard.rotation` from `obs.heading` (converted to radians via `extractHeadingRadians()`) and `billboard.alignedAxis = Cartesian3.UNIT_Z` so the icon rotates around the screen-space vertical axis.

Color is parsed once per Effect 2 invocation from the layer's CSS color string. The `desaturateColor()` function uses luminosity weighting (`0.299R + 0.587G + 0.114B`) to reduce saturation for historical entities.

Source alpha values are defined as constants:

```typescript
const SOURCE_ALPHA: Record<DataSource, number> = {
  live: 1.0,
  stale: 1.0,
  historical: 1.0,
}
```

All sources render at full opacity — stale/historical provenance is indicated in the Entity Detail panel rather than billboard dimming, which users perceived as a selection-related bug.

---

## 5. Spatial Indexing and Viewport Management

### 5.1 Hex Grid Coverage

`GenerateGlobalGrid(radiusNM float64)` (`internal/ingest/grid/grid.go`) generates a set of hex-packed center points that together cover the entire Earth surface with circles of `radiusNM` nautical mile radius.

Hex packing geometry:
- Vertical row spacing: `1.5 * radiusKm`
- Horizontal point spacing within a row: `sqrt(3) * radiusKm`, adjusted per-latitude by `cos(lat)` to account for meridian convergence
- Odd-numbered rows are offset by half the horizontal spacing

At `radiusNM = 250` this produces approximately 800 points. The equilateral triangle formed by three adjacent hex centers has a circumradius equal to exactly one radius, guaranteeing that no location on Earth is farther than `radiusNM` from the nearest center point.

`ComputeBatchSize(regionCount, interval, targetRefresh)` calculates how many regions to fetch per polling cycle to achieve full global coverage within `targetRefresh`:

```
batch_size = ceil(regionCount / (targetRefresh / interval))
```

### 5.2 Priority Crawling

When backfill occurs (Tier 2 fires), the WebSocket server publishes the viewport bbox to `viewport:priority:requests`:

```json
{
  "layer_type": "adsb_lol_flights",
  "bbox": { "west": -122.5, "south": 37.2, "east": -121.8, "north": 37.9 },
  "client_count": 3,
  "requested_at": "2024-01-15T10:30:00Z"
}
```

Deduplication on the server side: the signal is suppressed if the same quantized region (rounded to ~2° grid) was published within the last 10 seconds.

The feeder's `PrioritySubscriber` (`internal/ingest/priority/subscriber.go`) maintains a queue of `PriorityRegion` entries with TTL-based expiry (default 60s, max queue 100). Deduplication: if an existing entry's center is within 2° of the new signal's center, the existing entry's TTL is refreshed instead of adding a duplicate.

`ADSBLolAdapter.selectBatch()` has two phases:

**Phase 1 — Priority regions**: Queries `prioritySub.GetPriorityRegions()`. For each priority region, finds the closest overlapping hex grid cell (overlap tolerance: 5°). Adds up to `batchPer / 2` priority cells to the batch. Uses a `seen` set to prevent duplicates.

**Phase 2 — Round-robin**: Fills remaining slots from the flat grid cursor, skipping indices already in `seen`. Advances the cursor for the next cycle.

### 5.3 Frontend Viewport Sync

`useViewportSync()` (`useLayerStream.ts:L306`) watches `useViewerStore.viewport`. On each viewport change:
1. Deduplicates against the last sent viewport via JSON key comparison.
2. Applies **center-distance dedup**: if the viewport center moved less than 2° in both lat and lon from the last sent viewport, the update is suppressed. This prevents flooding the server during orbital fly-overs where the synthetic viewport drifts slightly on each camera frame.
3. Debounces for 300ms using `setTimeout`.
4. Sends `viewport_update` for each spatial layer in live mode, or `time_range` with the new viewport in range mode.

#### Synthetic Viewport at Orbital Altitude

`updateCameraState()` in `GlobeScene.tsx` is called on every `camera.changed` event. It computes the viewport bounding box from `camera.computeViewRectangle()` with a special case for **full-globe view** (orbital altitude):

When `(east - west) > 350° AND (north - south) > 170°`, the camera is at high altitude and the raw viewport is effectively `[-180, -90, 180, 90]`. Sending this as a `viewport_update` defeats spatial dedup — every layer receives the same global bbox regardless of where the user is looking.

The fix computes a **synthetic viewport** centered on the camera's look-at point:

1. `camera.pickEllipsoid()` casts a ray from the screen center to the ellipsoid surface, returning the geographic point the user is looking at.
2. A ±30° bounding box (60° × 60°, ~6,600 km per side) is constructed around that point, clamped to `[-180, -90, 180, 90]`.
3. This synthetic viewport is stored in the `useViewerStore` and sent via `viewport_update`.

If `pickEllipsoid` fails (camera pointing at deep space), the raw full-globe viewport is used as a fallback.

**Why ±30°?** At 8,000+ km altitude, the visible globe surface covers roughly ±40° from the sub-camera point, but the central 60° is where the user perceives spatial detail. This gives `GEOSEARCH BYBOX` a region large enough to cover a continent but small enough to avoid returning the entire planet's geo index.

### 5.4 Antimeridian Handling

All three spatial query paths handle the antimeridian case (`west > east`):

- **Valkey**: Centers the GEOSEARCH box by computing `centerLon = west + (east + 360 - west) / 2 % 360`.
- **Postgres backfill**: Dynamically switches the lon predicate from `BETWEEN` to `>= west OR <= east`.
- **Frontend `BBox.Contains()`**: Checks `lon >= west || lon <= east` when `west > east`.

---

## 6. Data Provenance and Visual Encoding

### 6.1 Source Types

Every entity and observation carries a `source` field assigned at the retrieval boundary:

| Source | Assigned In | Meaning |
|---|---|---|
| `live` | `handleSubscribe()`, `handleViewportUpdate()` — cache results; `doOnDemandFetchCore()` — on-demand API fetch | From Valkey real-time cache or fresh external API data |
| `stale` | `doBackfill()` | From Postgres backfill (cache miss, entity seen within 30min) |
| `historical` | `doTimeRangeQuery()` | From explicit bounded time range query |

The frontend normalizes `source` from the wire JSON at `useLayerStream.ts:L36` and stores it on the `Entity` and `Observation` interfaces.

### 6.2 Visual Encoding in BillboardLayerRenderer

Source type drives three visual properties applied in Effect 2:

| Source | Alpha | Saturation | Status Indicator |
|---|---|---|---|
| `live` | 1.0 | Full color | Green pulsing dot |
| `stale` | 1.0 | Full color | Green pulsing dot |
| `historical` | 1.0 | Desaturated (50% toward gray) | Amber static dot |

All sources render at full opacity. Stale/historical provenance is shown in the Entity Detail panel ("Last seen: Xm ago") rather than through billboard dimming. Historical entities are still desaturated (50% toward gray) to distinguish them visually.

Desaturation uses luminosity weighting: `gray = 0.299R + 0.587G + 0.114B`. The resulting color is `original + (gray - original) * 0.5`.

### 6.3 Status Indicator (Time Mode)

The HUD status indicator reflects the current time mode:

| Mode | Indicator |
|---|---|
| Live (default) | Green pulsing dot (CSS animation) |
| Sliding window (1h/8h/24h preset, `timeTo: null`) | Green pulsing dot — live Pub/Sub still flows |
| Frozen range (custom with past `timeTo`) | Amber static dot — live Pub/Sub suppressed |

The frozen determination is made both server-side (`now - to > 10 minutes` in `handleTimeRange()`) and client-side (`timeMode === 'range' && timeTo !== null` in `useLayerStream.ts:L128`).

### 6.4 Entity ID Format

Entities are identified by a composite ID: `{layerType}:{externalID}` (e.g., `adsb_lol_flights:a3f2b1`). This ID is assigned at the transport boundary in `doBackfill()` and `doTimeRangeQuery()`:

```go
compositeID := domain.EntityID(domain.LayerType(e.LayerType), e.ExternalID)
e.ID = compositeID
o.EntityID = compositeID
```

This ensures the frontend's entity detail lookup (`ParseEntityID`) works correctly, since the database stores raw UUIDs while the frontend expects the composite format.

---

## 7. Performance Profile

### 7.1 Measured Frame Times (Chrome DevTools)

#### Before O(delta) Fix (O(N) per version bump)

| Entity Count | Avg Frame | p50 | p95 | FPS |
|---|---|---|---|---|
| 29 (satellites) | 15.8ms | ~16ms | ~17ms | 63.4 |
| 361 (mixed) | 33.0ms | ~33ms | ~34ms | 30.3 |
| 2,050 (full load) | 22.4ms | 16.8ms | 18ms | 44.6 |

#### After O(delta) Fix (incremental dirty-set processing)

| Entity Count | Avg Frame | p50 | p95 | p99 | FPS |
|---|---|---|---|---|---|
| 3,491 entities / 2,000 billboards | 16.76ms | 16.8ms | 17.6ms | 17.8ms | 59.6 |

The fix maintains stable ~60fps with 3,491 total entities and 2,000 rendered billboards. No frames exceeded 33ms (no drops below 30fps). The slight above-16.67ms average is due to Cesium's render loop overhead, not entity update cost.

### 7.2 Incremental Update Cost (Effect 2)

Measured via `performance.now()` instrumentation over 494 incremental updates:

| Dirty Count Range | Samples | Avg Duration | Max Duration |
|---|---|---|---|
| 1–5 entities | 26 | 0.023ms | 0.1ms |
| 6–20 entities | 462 | 0.016ms | 0.1ms |
| 21–50 entities | 5 | 0.040ms | 0.1ms |
| 101–200 entities | 1 | 0.000ms | 0.0ms |

Full rebuild (initial load): 50.7ms for 3,491 entities → 2,000 billboards.

**Key finding**: Incremental updates are effectively **O(delta)** — processing 1–200 dirty entities costs ≤0.1ms regardless of total entity count (3,491). The cost scales with the dirty set size, not the total dataset size. This is orders of magnitude better than the pre-fix O(N) behavior where every version bump re-scanned all entities.

### 7.3 Algorithmic Complexity

| Operation | Complexity | Status | Notes |
|---|---|---|---|
| Store entity upsert (`setLayerEntities`) | O(1) per entity | On target | `Map.set()` + `Set.add()` for dirty tracking |
| Billboard data update (Effect 2) — incremental | O(delta) per version bump | **Fixed** | Only iterates `dirtyEntityIds` set, not full entity map |
| Billboard data update (Effect 2) — full rebuild | O(N) per layer | Expected | First load or after `clearLayerEntities`; runs once per subscription |
| Animation tick (Effect 2b, `preUpdate`) | O(A) where A = active animations | On target | Only iterates animation map; POS_EPSILON skips no-op animations |
| Occlusion check (Effect 3, `postRender`) | O(N) every 3rd frame | **Throttled** | Reduced from every-frame to every 3rd `postRender` frame |
| Entity isolation (Effect 4) | O(N) — all billboards | Acceptable | Only runs on isolation toggle, not on every update |

### 7.4 Remaining Bottlenecks

**Effect 3 — O(N) occlusion (throttled)**: Still iterates all billboards but now at 1/3 frequency. Acceptable at 2,000 billboards. At 10k+ entities, consider viewport-aware culling (Next Steps item 3).

**Effect 4 — O(N) isolation**: Toggling entity isolation iterates all billboards. Acceptable at current entity counts; problematic at 10k+ entities.

**RAF batch normalization on main thread**: `flushBatch()` in `useLayerStream.ts` normalizes entities (snake_case → camelCase) and writes to the store synchronously on the main thread. At high Pub/Sub rates this can block the frame.

---

## 8. Configuration Reference

### 8.1 OnDemandConfig (`internal/realtime/websocket.go`)

Configured via `Server.SetOnDemandConfig()`. Controls on-demand fetching (Tier 1.5) and continuous polling for sparse viewports.

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Feature flag for on-demand fetching |
| `Cooldown` | `10s` | Per-client, per-layer cooldown between on-demand fetches |
| `QueryTimeout` | `5s` | HTTP request timeout for external API calls |
| `LayerAPIs` | `nil` | Map of `layer_type → API URL template`. Only layers with configured URLs trigger on-demand. URL templates use `{lat}` and `{lon}` placeholders (e.g., `https://api.adsb.lol/api/v2/point/{lat}/{lon}`) |
| `MinCacheGap` | `10` | Minimum Tier 1 result deficit to trigger on-demand fetch |
| `PollIntervals` | `nil` | Map of `layer_type → re-poll interval`. Controls continuous polling interval (e.g., `30s` for flights). Layers without an entry do not start tickers |

### 8.2 BackfillConfig (`internal/realtime/websocket.go`)

Configured via `Server.SetBackfillConfig()`.

| Field | Default | Description |
|---|---|---|
| `Enabled` | `true` | Whether to perform Postgres backfill at all |
| `StalenessWindow` | `30m` | How far back to look in Postgres for backfill observations |
| `Thresholds` | `nil` (defaults to 10 per layer) | Per-layer minimum entity count below which backfill triggers |
| `MaxResults` | `2000` | Maximum rows returned per backfill query |
| `QueryTimeout` | `3s` | Postgres query timeout for backfill |
| `Cooldown` | `5s` | Per-client, per-layer minimum interval between backfill queries; reduced to 1s when cache returns 0 entities |

### 8.3 TimeRangeConfig (`internal/realtime/websocket.go`)

Configured via `Server.SetTimeRangeConfig()`. These are the **server-wide defaults**; individual layers can override `MaxLookback` and `MaxRangeSpan` via their source YAML `history:` block (see §3.6).

| Field | Default | Description |
|---|---|---|
| `Enabled` | `true` | Whether time range queries are accepted |
| `MaxLookback` | `48h` | Maximum age of `from` timestamp; older values are clamped. Per-layer override: `history.max_lookback` in source YAML |
| `MaxRangeSpan` | `24h` | Maximum `to - from` duration; spans wider than this advance `from`. Per-layer override: `history.max_range_span` in source YAML |
| `QueryTimeout` | `5s` | Postgres query timeout for time range queries |
| `Cooldown` | `2s` | Minimum interval between time range queries per client per layer |
| `MaxResults` | `2000` | Maximum rows returned per time range query |

### 8.4 PriorityCrawlingConfig (`internal/realtime/websocket.go`)

Configured via `Server.SetPriorityCrawlingConfig()`.

| Field | Default | Description |
|---|---|---|
| `Enabled` | `false` | Whether backfill triggers a priority signal to the feeder |

### 8.5 Priority Signal Deduplication

Deduplication is applied at two points:

- **Server side** (`publishPrioritySignal()`): quantized to ~2° grid, 10s cooldown per quantized key.
- **Feeder side** (`PrioritySubscriber.handleMessage()`): 2° center-point comparison, TTL refresh on match.

Priority queue limits: max 100 entries, 60s TTL, oldest entry dropped when full.

### 8.6 ADSBLolAdapter Config (feeder YAML)

| Key | Default | Description |
|---|---|---|
| `interval_seconds` | `30` | Polling interval |
| `max_age_seconds` | `300` | Max age for in-memory flight cache entries |
| `use_global_grid` | `true` | Use hex grid; `false` falls back to legacy regions |
| `coverage_radius_nm` | `250` | Hex grid circle radius in nautical miles |
| `target_refresh` | `"30m"` | Target full-globe sweep duration |
| `batch_size` | `0` | Regions per cycle; 0 = auto-computed from `target_refresh` |

### 8.7 Cesium Viewer Options (`GlobeScene.tsx`)

Configured as `CESIUM_VIEWER_OPTIONS` constant.

| Option | Value | Rationale |
|---|---|---|
| `requestRenderMode` | `true` | Only render on explicit `requestRender()` calls or camera movement. Keeps CPU/GPU idle when no data changes. |
| `maximumRenderTimeChange` | `0.5` | Safety-net auto-render every 500ms simulation time. Prevents blank globe on StrictMode timing gaps. Was previously `Infinity`. |
| `targetFrameRate` | `60` | Cap render rate at 60fps |
| `scene3DOnly` | `true` | Disables 2D/Columbus view switching. Reduces bundle size and Cesium overhead. |
| `shouldAnimate` | `true` | Advances simulation clock. Required for `preUpdate` / `postRender` events to fire. |
| `shadows` | `false` | Disabled for performance. |

### 8.8 Frontend Constants

| Constant | Location | Value | Description |
|---|---|---|---|
| `INTERP_DURATION_MS` | `BillboardLayerRenderer.tsx` | `2000` | Lerp duration for server-to-server position interpolation |
| `CATCHUP_PAUSE_MS` | `BillboardLayerRenderer.tsx` | `5000` | Pause duration after backward motion detection |
| `BACKWARD_THRESHOLD_M` | `entityUtils.ts` | `100` | Distance threshold (meters) for backward motion detection |
| `MAX_HEADING_DELTA_RAD` | `entityUtils.ts` | `π/6` (30°) | Heading delta threshold for backward motion detection |
| `KNOTS_TO_MPS` | `entityUtils.ts` | `0.514444` | Knots to meters/second conversion factor |
| `smoothMotion` (default) | `app/store.ts` | `true` | Default state for predictive interpolation toggle |
| `isSpatialLayer()` | `useLayerStream.ts` | Dynamic | Checks `filteringMode === 'viewport'` from store — no hardcoded list |
| `maxEntities` | `app/store.ts` initial state | `2000` | Render limit applied when materializing entity arrays |
| Viewport debounce | `useLayerStream.ts:L332` | `300ms` | `setTimeout` delay for viewport_update messages |
| Viewport center dedup | `useLayerStream.ts:L321-327` | `2°` | Skip viewport_update if center moved <2° in both lat and lon |
| Full-globe threshold | `GlobeScene.tsx:L105` | `350° × 170°` | Width/height thresholds to detect orbital altitude (full-globe view) |
| Synthetic viewport half-span | `GlobeScene.tsx:L122` | `30°` | ±30° box centered on camera look-at point at orbital altitude |
| Camera persist debounce | `globe/store.ts` | `1000ms` | `localStorage` write debounce for camera state |
| WS reconnect attempts | `websocket.ts:L77` | `5` | Max reconnection attempts before giving up |
| WS reconnect base delay | `websocket.ts:L78` | `1000ms` | Base delay; multiplied by `2^attempt` |
| WS reconnect max delay | `websocket.ts:L79` | `30000ms` | Hard cap on reconnect delay |
| WS reconnect jitter | `websocket.ts:L278` | `0–50%` | Random jitter added to base delay to prevent thundering herd |

### 8.9 Geo Cache Config (`sources.d/*.yaml`)

Per-layer configuration for the Valkey geo sorted set maintenance. Declared as a top-level `geo_cache` key in source YAML files (peer of `filtering`, `backfill`, `cache`, `transport`). Only relevant for sources with `filtering: viewport`.

```yaml
geo_cache:
  overfetch_ratio: 3.0
  alive_ratio_threshold: 0.5
  gc_batch_size: 500
```

| Field | Default | Description |
|---|---|---|
| `overfetch_ratio` | `3.0` | Ratio of extra members to fetch beyond the requested limit. 3.0 means: if limit=2000, fetch COUNT=6000 from GEOSEARCH. Higher values improve alive-entity yield at the cost of more Redis I/O. |
| `alive_ratio_threshold` | `0.5` | Minimum fraction of alive entities before triggering lazy GC + second-pass fetch. 0.5 means: if fewer than 50% of returned members resolve to live hashes, prune dead members and re-query. |
| `gc_batch_size` | `500` | Maximum number of dead geo members to ZREM per query cycle. Caps write load on Valkey; dead members beyond this cap are cleaned up on subsequent queries. |

Registered at startup via the declarative source loader → `DynamicSourceRegistry.SetGeoCacheConfig()`. The `ValkeyAdapter` receives the registry via constructor injection and looks up the config by layer type on each `GetLayerEntitiesByBBox()` call. Sources without `geo_cache` skip the GC path entirely (standard GEOSEARCH).

Domain struct: `domain.GeoCacheConfig` (`internal/domain/source.go`).

---

## 9. Known Issues and Recent Fixes

### 9.1 requestRenderMode Blank Globe (Fixed)

**Problem**: `requestRenderMode: true` combined with `maximumRenderTimeChange: Infinity` meant Cesium performed zero auto-renders. The globe would remain blank until the user moved the camera. This was compounded by React StrictMode's double-effect-invocation behavior in development: effects are mounted, cleaned up, and remounted. The first `requestRender()` call in Effect 2 could be consumed during the cleanup-remount cycle, before billboard GPU uploads were completed, causing entities to not appear.

**Fix applied** (commits around `399b04a` and `541d894`):

1. `maximumRenderTimeChange` changed from `Infinity` to `0.5` (`GlobeScene.tsx:L49`). This triggers an auto-render at most every 500ms of simulation time, serving as a fallback when explicit `requestRender()` calls are missed.

2. Effect 2 now calls `requestRender()` twice (`BillboardLayerRenderer.tsx:L341-348`):
   ```typescript
   viewerRef.current?.scene.requestRender()
   const rafId = requestAnimationFrame(() => {
     viewerRef.current?.scene.requestRender()
   })
   return () => cancelAnimationFrame(rafId)
   ```
   The second call is scheduled on the next animation frame via `requestAnimationFrame`, ensuring a render occurs after the StrictMode remount completes. The `cancelAnimationFrame` cleanup prevents the deferred call from firing after unmount.

**React StrictMode behavior**: In development, React mounts each component twice (mount → unmount → mount) to detect side effects. This means Effect 1 (collection creation) and Effect 2 (billboard population) each run twice in sequence. The safety-net render ensures the second mount's billboard uploads result in a visible render.

This behavior does not occur in production builds, but `maximumRenderTimeChange: 0.5` is retained as a general safety net that adds at most ~2 additional renders per second.

### 9.2 BroadcastLayerUpdate Race Condition (Fixed)

**Problem**: The original `BroadcastLayerUpdate()` implementation spawned a goroutine while holding `sync.RWMutex` read lock. If the goroutine needed to send to the `unregister` channel, it would deadlock against the `Run()` loop which requires a write lock before processing unregistration.

**Fix**: `BroadcastLayerUpdate()` was rewritten to collect clients-to-unregister in a local slice while holding the read lock, release the lock, then send unregistration requests outside the critical section. The code comment at `websocket.go:L1110-1113` documents this.

### 9.3 Stale Geo Index Causing Entity Disappearance (Fixed)

**Problem**: The `layer:{layerType}:geo` sorted set has no TTL — members accumulate permanently while entity hashes expire at 60s/300s. Over time, the geo index fills with stale members. `GEOSEARCH COUNT N ASC` returns the N *nearest* members, which are disproportionately stale. The backend would see "28 alive entities" against a threshold of 10, log `cache sufficient: skipping backfill`, and never trigger Tier 1.5/Tier 2 recovery. Users saw entities disappear when panning to regions where the geo index had accumulated many stale entries.

**Fix applied**: Over-fetch + lazy GC, encapsulated in the `ValkeyAdapter`:

1. `ValkeyAdapter.GetLayerEntitiesByBBox()` checks the `DynamicSourceRegistry` (injected via constructor) for a `GeoCacheConfig` for the layer type.
2. When config exists, the adapter over-fetches by `overfetch_ratio` (e.g., 3×), separates alive/dead members via pipeline `HGetAll`, and prunes dead members via `ZREM` when the alive ratio drops below `alive_ratio_threshold` (capped at `gc_batch_size`).
3. If alive count is still below limit after pruning, a second `GEOSEARCH` backfills the deficit.
4. Sources without `geo_cache` YAML use the standard (non-GC) query path — no behavior change.

**Architecture**: The GC logic is entirely within the adapter (DIP — adapter encapsulates optimizations). The WebSocket handler calls a single `GetLayerEntitiesByBBox()` method and never needs to know about GC. The `DynamicSourceRegistry` is injected into the adapter at construction time, not accessed via a global singleton.

Configuration: `geo_cache` section in `sources.d/*.yaml` (see §8.9). Currently enabled for all 11 viewport-filtered sources.

Relevant files:
- `internal/storage/cache/valkey_adapter.go` — `GetLayerEntitiesByBBox()`, `geoSearchWithGC()`, `geoSearchStandard()`
- `internal/domain/source.go` — `GeoCacheConfig` struct
- `internal/domain/dynamic_registry.go` — `SetGeoCacheConfig()`, `LookupGeoCacheConfig()`
- `sources.d/adsb_lol_flights.yaml` (and other viewport sources) — `geo_cache:` configuration

### 9.4 Global-Zoom Viewport Dedup Suppressing Updates (Fixed)

**Problem**: At orbital altitude (≥8,000 km), Cesium's `computeViewRectangle()` returns `[-180, -90, 180, 90]` — the full globe. Panning at this altitude does not change the viewport JSON, so `useViewportSync`'s dedup (`lastSentRef`) suppressed every `viewport_update` after the first. The server never received updated viewports and never issued new `GEOSEARCH` queries for the region the user was actually looking at.

**Fix applied** (two parts):

1. **Synthetic viewport** (`GlobeScene.tsx`): When `(east - west) > 350° AND (north - south) > 170°`, `camera.pickEllipsoid()` computes the geographic point the user is looking at. A ±30° bounding box (~6,600 km per side) is constructed around that point and stored as the viewport. This replaces the useless `[-180, -90, 180, 90]` with a meaningful spatial region.

2. **Center-distance dedup** (`useLayerStream.ts`): Even with synthetic viewports, rapid orbital panning produces many small viewport changes. A 2° center-distance threshold suppresses updates where the viewport center moved less than 2° in both lat and lon from the last sent viewport. The existing 300ms debounce still applies on top.

Relevant files:
- `frontend/apps/web/src/features/globe/GlobeScene.tsx` — `updateCameraState()`, lines 96-135
- `frontend/apps/web/src/shared/api/useLayerStream.ts` — `useViewportSync()`, lines 306-361

Test coverage:
- `GlobeScene.test.tsx` — 22 tests covering normal viewport passthrough, full-globe synthetic viewport, pickEllipsoid fallback, boundary clamping, and threshold conditions
- `useLayerStream.viewportSync.test.ts` — 20 tests covering center-distance dedup, debounce behavior, and spatial/non-spatial layer routing

---

## 10. Next Steps

The following optimizations are listed in priority order based on expected impact.

### ~~1. O(delta) Billboard Updates~~ — COMPLETED

Implemented via `dirtyEntityIds: Set<string>` in the Zustand store's `LayerData` interface. `setLayerEntities()` adds modified entity IDs to the dirty set. Effect 2 in `BillboardLayerRenderer.tsx` processes only the dirty set on incremental updates, clearing it after processing. Full rebuilds are triggered only on first load or after `clearLayerEntities()`.

**Measured result**: Incremental updates of 1–200 dirty entities cost ≤0.1ms with 3,491 total entities. See [Performance Profile](#7-performance-profile) for full metrics.

### ~~2. Throttled Occlusion Checks~~ — COMPLETED

Effect 3 now runs occlusion checks every 3rd `postRender` frame using a frame counter, instead of on every `camera.changed` event. An `occlusionDirtyRef` flag forces immediate recalculation after data changes.

### 3. Viewport-Aware Billboard Culling

**Current**: All entities for a layer are added to the `BillboardCollection` regardless of viewport.

**Target**: Only add billboards for entities within the current viewport plus a configurable margin. This reduces both the occlusion iteration set (Effect 3) and GPU memory usage. Requires a frontend spatial index (e.g., a grid or quadtree over entity lat/lon positions) to efficiently query which entities are in-viewport. The `useViewerStore.viewport` bbox is available for this check.

### 4. Web Worker for Data Normalization

**Current**: The RAF batch flush in `useLayerStream.ts` performs snake_case-to-camelCase normalization and `Map.set()` upserts synchronously on the main thread. At high Pub/Sub rates this competes with rendering.

**Target**: Move `normalizeEntity()`, `normalizeObservation()`, and batch assembly off the main thread using a Web Worker. The worker receives raw JSON messages and posts normalized batches back to the main thread, which then writes to the store. This requires making entity/observation objects transferable or structured-cloneable.

### 5. BillboardCollection Instancing

**Current**: One `BillboardCollection` per layer. All entities in a layer use the same icon canvas but Cesium's internal batching behavior for billboard collections is not fully characterized.

**Target**: Investigate whether instanced rendering applies to `BillboardCollection` for identical images. For layers with many entities sharing one icon (e.g., all satellites using the same SVG), there may be a draw-call reduction available. Profile with Cesium's `requestRenderMode` and CPU/GPU frame analysis tools.

### 6. Remove React StrictMode in Production Builds

**Current**: `maximumRenderTimeChange: 0.5` adds up to 2 auto-renders per second in production as a safety net against the StrictMode double-effect issue.

**Target**: The double-effect behavior only occurs in development (React StrictMode). Conditionally apply the safety-net `maximumRenderTimeChange` only in development builds, and restore `maximumRenderTimeChange: Infinity` in production. This eliminates the periodic idle renders entirely in production. The `import.meta.env.DEV` flag is already used elsewhere in `GlobeScene.tsx` (e.g., `window.__cesiumViewer = viewer`).

### 7. Observation Deduplication in Store

**Current**: When the same entity receives multiple Pub/Sub updates within one RAF batch interval, all updates are pushed to the batch array and processed sequentially. The last write wins via `Map.set()`, but all prior writes also iterate through normalization.

**Target**: Deduplicate within `scheduleBatch()` by entity ID before the RAF fires. If an entry for a given entity ID already exists in the batch buffer, replace it rather than appending. This reduces normalization cost at high update rates.

### ~~8. Per-Entity Incremental Animations~~ — COMPLETED

Effect 2 now checks position delta against `POS_EPSILON` (1e-6) before creating `AnimationState` entries. Only entities whose position actually changed (beyond floating-point noise) enter the animation map. Combined with O(delta) dirty tracking, this means only truly-updated entities incur animation overhead.

---

## Related Documentation

- **[Entity Search & Watchlist](ENTITY_SEARCH.md)** — Search, persistent watchlists, Find Mode, and pinned entity rendering. The `FindModeFilter` and `PinnedEntityRenderer` components coordinate with the billboard rendering pipeline described here, sharing the `preUpdate`/`postRender` frame lifecycle and `layerEntities` store.
- **[Entity Interaction & Multi-Selection](ENTITY_INTERACTION.md)** — Click-based selection, multi-selection, camera tracking, and the entity detail panel system. The watchlist extends this selection model with pinned entities and fly-to navigation.
