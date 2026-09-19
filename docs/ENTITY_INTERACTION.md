# Entity Interaction & Multi-Selection

This document describes how entity selection, multi-selection, camera tracking, and the entity detail panel system work in the Respondent globe application.

---

## Overview

Users interact with entities on the 3D globe through click-based gestures:

- **Single click** selects an entity, opens its detail panel, and plays an audio cue.
- **Ctrl+Click** (Win/Linux) or **Cmd+Click** (Mac) toggles an entity in a multi-selection set (up to a configurable maximum, default 5).
- **Double click** enters Entity View Mode, locking the camera to orbit and follow the entity in real-time.
- **Click on empty space** clears all selections.
- **ESC key** exits Entity View Mode and clears all selections.

All selection state is frontend-only (Zustand store). No backend persistence of selection exists.

---

## Architecture

```
Globe (Cesium Viewer)
  │
  ├─ useEntityInteraction ──► pointerdown listener (modifier keys)
  │                          ScreenSpaceEventHandler (click/dblclick/hover)
  │                            │
  │                            ▼
  │                       Zustand Store (selectedEntities, selectedEntityId, viewMode)
  │                            │
  │                            ├──► AppShell ──► EntityDetailPanel × N (one per selection)
  │                            ├──► SelectionIndicator (bracket billboards + coordinate labels for selections & watchlist)
  │                            └──► useEntityTracking (camera lock on primary entity)
  │
  └─ BillboardLayerRenderer (renders entity points; billboard.id = { entityId, layerId })
```

---

## State Management

**File:** `frontend/apps/web/src/app/store.ts`

The Zustand store holds these selection-related fields:

| Field | Type | Purpose |
|-------|------|---------|
| `selectedEntityId` | `string \| null` | The "primary" entity — last clicked. Used for camera tracking and entity-view mode. |
| `selectedLayerId` | `string \| null` | Layer of the primary entity. |
| `selectedEntities` | `Array<{ entityId, layerId }>` | Ordered list of all selected entities. Each entry opens its own detail panel. |
| `maxSelectedEntities` | `number` | Maximum simultaneous selections (default 5). |
| `viewMode` | `'globe' \| 'entity'` | `globe` is the normal view; `entity` means camera is tracking the primary entity. |

### Actions

| Action | Behavior |
|--------|----------|
| `setSelectedEntity(entityId, layerId)` | **Plain click.** Clears `selectedEntities`, sets a single selection. Also manages `watchlistEntities`. When called with `null`, clears everything. |
| `addSelectedEntity(entityId, layerId)` | **Modifier+click.** Appends to `selectedEntities` if not already present and under `maxSelectedEntities`. Also adds to watchlist. Sets the added entity as primary. No-op if duplicate or at capacity. |
| `removeSelectedEntity(entityId)` | **Modifier+click on already-selected entity.** Removes from `selectedEntities`. Also deletes `entityViewState[entityId]` and `trailPoints[entityId]`. If the removed entity was primary, the last remaining entity becomes the new primary. If no entities remain, primary resets to `null`. |
| `clearSelection()` | Empties `selectedEntities`, resets `selectedEntityId`, `selectedLayerId`, sets `viewMode` back to `'globe'`. Also deletes `entityViewState` and `trailPoints` for all removed entities, and clears unpinned watchlist entities. |
| `setViewMode(mode)` | Switches between globe and entity-tracking mode. |

### Store Tests

**File:** `frontend/apps/web/src/app/store.test.ts`

The test suite covers all multi-selection scenarios: adding entities, duplicate no-ops, max capacity enforcement, removal with primary rotation, clearing, and interaction between `setSelectedEntity` (plain click) and the multi-selection array.

---

## Keyboard Utilities

**File:** `frontend/apps/web/src/shared/input/keyboard.ts`

A pure (non-React) module that provides platform-aware modifier key detection:

- `isMac` — boolean constant, `true` on macOS/iOS (detected via `navigator.platform` or `navigator.userAgentData`).
- `isMultiSelectModifier(event)` — returns `true` if the platform-appropriate multi-select modifier is pressed: **Meta (Cmd)** on Mac, **Ctrl** on Windows/Linux. Accepts any object with `ctrlKey` and `metaKey` booleans, so it works with both DOM events and synthetic test objects.

**Barrel export:** `frontend/apps/web/src/shared/input/index.ts`

**Tests:** `frontend/apps/web/src/shared/input/keyboard.test.ts`

---

## Globe Interaction

**File:** `frontend/apps/web/src/features/globe/useEntityInteraction.ts`

This hook attaches three Cesium `ScreenSpaceEventHandler` actions to the viewer canvas:

### Modifier Key Capture

Cesium's `ScreenSpaceEventHandler` does not expose DOM modifier keys (`ctrlKey`, `metaKey`). To work around this, the hook adds a `pointerdown` event listener directly on `viewer.canvas`. Each pointer-down event is stored in a ref (`lastPointerEventRef`), which is then read inside the Cesium click handler to determine whether a multi-select modifier was active.

### Hover

On `MOUSE_MOVE`, the hook picks the billboard under the cursor. If a billboard is found, the canvas cursor changes to `pointer`; otherwise it resets to `default`.

### Single Click

On `LEFT_CLICK`:

1. Pick the billboard at the click position.
2. If a billboard is found:
   - **With modifier key held** (read from `lastPointerEventRef`): toggle the entity in the multi-selection set via `addSelectedEntity` or `removeSelectedEntity`.
   - **Without modifier key**: replace the entire selection with a single entity via `setSelectedEntity`.
   - Play the audio cue (`playSelectSound`).
3. If no billboard is found (empty space click): call `clearSelection()`, which also exits entity-view mode if active.

### Double Click

On `LEFT_DOUBLE_CLICK`: saves the current camera state to the viewer store, selects the entity, enters entity-view mode (`setViewMode('entity')`), and plays the audio cue. Double-click always operates on the individual entity, regardless of multi-selection state. Other selected entities remain selected.

### Cleanup

On unmount, the hook removes the `pointerdown` listener, destroys the `ScreenSpaceEventHandler`, and resets the cursor.

---

## Selection Indicator (Tactical Brackets)

**File:** `frontend/apps/web/src/features/globe/SelectionIndicator.tsx`

Renders tactical corner brackets around **all selected entities and watchlist entities** on the globe. Watchlist entities (pinned-but-not-selected) are rendered with dimmer brackets (alpha 0.6) compared to selected entities (alpha 0.9).

### Bracket Rendering

A canvas-drawn bracket icon (64×64 pixels, four L-shaped green corners) is created once via `useMemo` and reused for all bracket billboards. The brackets use the application's primary green color (`#00ff9d`) with a subtle glow effect.

```
┌─        ─┐
│  ENTITY  │
└─        ─┘
```

### Billboard Management

A dedicated `BillboardCollection` and `LabelCollection` are added to the Cesium scene primitives (separate from entity billboards). When `selectedEntities` or watchlist entities change:

1. The component uses keyed maps (`bracketMapRef`, `labelMapRef`) for incremental add/remove/update — explicitly avoiding `removeAll()` to prevent flicker.
2. For each entity, the hook reads the entity's position from the `interpolatedPositions` map (written by `BillboardLayerRenderer`), falling back to the store's observation position in `layerEntities`.
3. A bracket billboard is placed at that position with `disableDepthTestDistance: POSITIVE_INFINITY` (always visible regardless of globe occlusion).
4. Each bracket billboard stores the `entityId` in its `id` property for per-entity tracking.
5. A `LabelCollection` renders formatted lat/lon coordinate text below each bracket, providing at-a-glance position readout.

### Real-Time Position Tracking

A `scene.preUpdate` listener runs every frame. It iterates through all bracket billboards, reads each one's entity ID, looks up the position from `interpolatedPositions` (with fallback to store observation position), and updates the billboard and label positions. This keeps brackets and labels glued to moving entities.

---

## Entity View Mode (Camera Tracking)

**File:** `frontend/apps/web/src/features/globe/useEntityTracking.ts`

When `viewMode === 'entity'`, this hook locks the camera to orbit around the **primary** entity (`selectedEntityId`). Multi-selection does not affect which entity is tracked — only the primary (last-clicked) entity drives the camera.

### Entry

On entering entity-view mode (triggered by double-click):

1. The hook computes a viewing range based on the entity's altitude (minimum 100 km).
2. `camera.lookAt` frames the entity immediately with a -45° pitch and the computed range.
3. The tracking loop activates.

### Tracking Loop

A `scene.preUpdate` listener runs every frame while in entity-view mode:

1. Reads the entity's current position from `layerEntities` in the store. If `trackingOverridePosition` is set (e.g., by the history timeline to temporarily track a historical position), that override position is used instead.
2. Clones the camera's current position offset (which encodes the user's orbit adjustments — heading, pitch, distance).
3. Re-applies `camera.lookAt` with the new entity position and the preserved offset.

This design allows the user to freely orbit (rotate, zoom) around the entity while it moves. Only the lookAt target moves; the user's viewing angle is never overwritten.

### Exit

Entity-view mode can be exited via:

- **ESC key** — global `keydown` listener calls `clearSelection()`.
- **"Exit Entity View" button** — visible in the primary entity's detail panel.
- **Click on empty space** — `clearSelection()` via the interaction hook.

On exit, the hook detects the `entity → globe` viewMode transition, unlocks the camera from `lookAt` via `lookAtTransform(Matrix4.IDENTITY)`, and restores the previously saved camera position with a 1-second `flyTo` animation.

---

## Entity Detail Panel

**File:** `frontend/apps/web/src/features/entity/EntityDetailPanel.tsx`

Each selected entity gets its own `EntityDetailPanel` instance. The component accepts `entityId` and `layerId` as props (rather than reading from the store), enabling multiple instances to coexist. Entity data is fetched via `useEntityDetailWithFallback`, which wraps `useEntityDetail` with a fallback to WebSocket store data for real-time updates.

### Props vs Store

The panel reads the following from props:
- `entityId` — which entity to display
- `layerId` — which layer the entity belongs to

The panel reads the following from the store:
- `selectedEntityId` — to determine if this panel's entity is the "primary" (tracked) entity
- `viewMode` — to show/hide the "Exit Entity View" button
- `removeSelectedEntity` — for the close button
- `clearSelection` — for the "Exit Entity View" button

### Panel Layout System

Each panel registers with the layout system using a **unique** panel ID: `entity-detail-${entityId}`. This causes the auto-layout system (`usePanelPosition`) to treat each as a separate panel and stack them horizontally in the `top-right` zone.

Example with 3 entities selected:

```
top-right zone:
  EffectsPanel      (order 0, w=280):  right: 16
  EntityDetail-A    (order 1, w=320):  right: 312
  EntityDetail-B    (order 2, w=320):  right: 648
  EntityDetail-C    (order 3, w=320):  right: 984
```

Closing one panel (via `removeSelectedEntity`) causes the layout system to automatically shift the remaining panels.

### Tab System

The panel uses a plugin-based tab registry defined in `frontend/apps/web/src/features/entity/tabs/tabRegistry.ts`. Tabs self-register on import and are filtered by entity layer type. Built-in tabs:

| Tab | Description |
|-----|-------------|
| **Overview** | Name, type badge, position, altitude, speed, heading |
| **History** | Timeline event list with trail visualization (see [Entity History Trail](#entity-history-trail) below) |
| **Metadata** | Key-value table of all metadata fields |
| **AI Analysis** | AI-generated analysis and insights. Registered conditionally — only appears when AI metadata exists for the entity. |

### Primary Entity Indicator

The panel shows contextual status labels:
- **TRACKING** — displayed when this entity is both the primary entity and in entity-view mode.
- **SELECTED** — displayed in all other cases.

The "Exit Entity View" button is **only visible on the primary entity's panel** (the one being camera-tracked), not on other multi-selected entities' panels.

---

## Multi-Selection Rendering in AppShell

**File:** `frontend/apps/web/src/app/AppShell.tsx`

The `AppShell` renders entity detail panels through a helper component that maps over `selectedEntities` from the store. Each entry in the array produces one `EntityDetailPanel` with the corresponding `entityId` and `layerId` props. Panels are only rendered when the UI is not in clean mode and not in recording mode (`!cleanUI && !recordingMode`).

---

## Audio Cue

**File:** `frontend/apps/web/src/shared/audio/selectSound.ts`

A short metallic "lock-on" click sound plays on every entity selection (both single and multi-select clicks). The sound is synthesized at playback time using a Web Audio API oscillator with a gain envelope — no pre-decoded audio buffer is used. The `AudioContext` is initialized lazily on first user interaction to comply with browser autoplay policies.

---

## Data Flow Summary

1. **User clicks entity on globe** → `useEntityInteraction` picks billboard, reads modifier keys from captured `pointerdown` event.
2. **Store updates** → `setSelectedEntity` (plain click) or `addSelectedEntity`/`removeSelectedEntity` (modifier click).
3. **Panels react** → `AppShell` re-renders the `selectedEntities` map, mounting/unmounting `EntityDetailPanel` instances.
4. **Brackets react** → `SelectionIndicator` rebuilds bracket billboards for all entries in `selectedEntities`.
5. **Camera reacts** (entity-view only) → `useEntityTracking` follows the primary entity.
6. **Panel close** → `removeSelectedEntity` removes from array, layout system shifts remaining panels.
7. **Clear all** → `clearSelection` empties everything, exits entity-view mode, restores saved camera.

---

## Backend Integration

Entity detail data is fetched via REST (not WebSocket) because entity metadata is indexed in PostgreSQL and does not change in real-time. Position updates continue flowing through the existing WebSocket layer stream.

| Endpoint | Purpose |
|----------|---------|
| `GET /v1/entities/{id}` | Fetch entity metadata and latest observation |
| `GET /v1/entities/{id}/observations` | Fetch paginated observation history |

These are consumed by React Query hooks (`useEntityDetail`, `useEntityObservations`) defined in `frontend/apps/web/src/shared/api/queries.ts`. The backend service implementation lives in `internal/bff/entity_service.go`.

---

## Entity History Trail

When an entity is selected, its observation history is progressively loaded and rendered as a trail on the globe. The History tab displays these observations as an interactive timeline.

### Trail Data

**File:** `frontend/apps/web/src/features/entity/hooks/useEntityTrail.ts`

The `useEntityTrail(entityId)` hook progressively fetches observation history via the existing `GET /v1/entities/{id}/observations` endpoint. It accumulates `TrailPoint[]` (defined in `features/entity/types.ts`) sorted oldest-first, with a configurable `maxPoints` limit (default 500). Points are synced to the Zustand store via `setTrailPoints()` for cross-component access.

### Trail Rendering

**File:** `frontend/apps/web/src/features/globe/EntityTrailRenderer.tsx`

Renders polylines on the globe for all selected entities using Cesium's `PolylineCollection` for batched GPU rendering. Trails use Catmull-Rom spline interpolation (8 samples per segment) for smooth curves.

**Trail styling** is sourced from the layer's `displayConfig.trail` in the store (populated from YAML configuration), falling back to hardcoded `TRAIL_COLORS` when no display config is set. Trail width reads from `displayConfig.trail.width` with a `DEFAULT_TRAIL_WIDTH` of 1.5px.

**Default trail colors by layer type:**

| Layer Type | Color | Default Width |
|-----------|-------|---------------|
| `satellites` | Green (`#00ff9d`) | 1.5px |
| `flights_commercial` | Cyan (`#00d4ff`) | 1.5px |
| `flights_military` | Orange (`#ff9d00`) | 1.5px |
| Default | Green (`#00ff9d`) | 1.5px |

**Segment splitting:** Trails are split into segments at time gaps > 1 hour. Historical segments (older than the most recent segment) are rendered at reduced opacity, making the active trail leg visually distinct.

**Leader line:** A leader line is rendered from the last trail point to the entity's current position, connecting the historical trail to the live entity marker.

When a timeline event is highlighted, a white point with outline is rendered at the corresponding trail position.

### Timeline Event List

**File:** `frontend/apps/web/src/features/entity/tabs/TimelineEventList.tsx`

A vertical scrollable timeline with a spine line and event cards. Each card shows timestamp, coordinates, altitude, and speed. Interactions:

- **Click** an event → highlights the corresponding point on the trail.
- **Double-click** an event → flies the camera to that historical position.
- **Load More** at bottom → fetches the next page of observations.

In entity-view (tracking) mode, single click both highlights and flies to the point.

### Store State

Trail state is stored per-entity in the Zustand store:

| Field | Type | Purpose |
|-------|------|---------|
| `trailPoints` | `Record<string, TrailPoint[]>` | Trail point data per entity (written by hook, read by renderer) |
| `entityViewState[id].trailHighlight` | `number \| null` | Highlighted point timestamp for the timeline ↔ trail interaction |
| `entityViewState[id].showTrails` | `boolean` | Per-entity trail visibility toggle |
| `entityViewState[id].isolateEntity` | `boolean \| undefined` | When true, hides all other entities on the globe, showing only this entity and its trail |

Trail state is cleaned up automatically when an entity is deselected via `removeSelectedEntity` or `clearSelection`.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Frontend-only selection state** | No multi-user selection requirement. Adding backend sync later via WebSocket is straightforward. |
| **Primary entity concept** | Multi-selection needs a "focus" entity for camera tracking. The last-clicked entity serves this role naturally. |
| **Max selection limit** | Prevents UI clutter and performance degradation from too many open panels and bracket billboards. Configurable via `maxSelectedEntities`. |
| **Pointer event capture for modifiers** | Cesium's event handler API does not expose DOM modifier keys. Capturing `pointerdown` on the canvas is the cleanest workaround without patching Cesium. |
| **Platform-aware modifier key** | Mac users expect Cmd for multi-select; Windows/Linux users expect Ctrl. The `isMultiSelectModifier` utility centralizes this logic. |
| **Props-driven detail panel** | Accepting `entityId`/`layerId` as props (rather than reading from store) enables multiple panel instances and makes the component testable in isolation. |
| **Unique panel IDs per entity** | The layout system stacks panels by ID. Using `entity-detail-${entityId}` ensures each selected entity gets its own layout slot. |
| **Tab registry pattern** | New entity types (orbital, seismic, CCTV) will need custom tabs. The registry allows adding tabs without modifying the panel component. |
| **Free orbit camera tracking** | More flexible than a chase-cam for inspection. The user can zoom and rotate while `lookAt` keeps the entity centered. |
| **REST for entity detail** | Entity metadata doesn't change in real-time. REST with React Query caching is simpler and more appropriate than streaming. |

---

## Related Documentation

- **[Entity Search & Watchlist](ENTITY_SEARCH.md)** — Search, persistent watchlists, Find Mode, and pinned entity rendering. Extends the selection system described here with pinned entities that persist across sessions and a batch API for fetching observations outside the viewport.
- **[Entity Visualization](ENTITY_VISUALIZATION.md)** — Billboard rendering pipeline, occlusion effects, and data provenance. The watchlist system injects stale observations into the same `layerEntities` store consumed by the rendering pipeline.
