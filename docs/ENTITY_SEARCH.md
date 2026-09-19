# Entity Search & Watchlist

**Feature Name:** Entity Search & Watchlist (internally: "Find Mode")
**Status:** Implemented
**Author:** Claude + alevsk
**Date:** 2026-03-13

---

## 1. Problem Statement

Analysts using Respondent need to quickly locate specific entities (flights, satellites, earthquakes, etc.) by identifier or name and persistently track combinations of entities across different layers. Today, entity discovery is limited to visual scanning of the globe and clicking individual billboards. There is no way to:

- Search for a known entity by ID, name, or partial match
- Pin entities for persistent tracking regardless of layer visibility
- Compose a custom watchlist mixing entity types (e.g., a flight + a satellite + a weather alert)
- Focus the view exclusively on tracked entities

---

## 2. User Stories

| # | As a... | I want to... | So that... |
|---|---------|-------------|-----------|
| 1 | Intelligence analyst | Search for an entity by ID or name fragment | I can find a known target without scanning the globe |
| 2 | Analyst | Pin an entity to a persistent watchlist | It stays visible even if its layer is toggled off or deleted |
| 3 | Analyst | See tactical brackets on all pinned entities | I maintain visual awareness of tracked targets |
| 4 | Analyst | Enter Find Mode to isolate only pinned entities | I can focus analysis on a curated set without visual noise |
| 5 | Analyst | Toggle between hidden and dimmed modes in Find Mode | I can choose whether to keep spatial context or have a clean view |
| 6 | Analyst | Have my watchlist persist across sessions | I don't lose my tracking setup on page reload |
| 7 | Analyst | Remove entities from the watchlist | I can clean up targets I no longer need to track |
| 8 | Analyst | Click a pill in the watchlist to fly to that entity | I can quickly navigate between tracked targets |

---

## 3. Feature Overview

### 3.1 Toolbar Entry Point

Replace the non-functional **Map** icon button in `BottomToolbar.tsx` (line 131) and `MobileBottomNav.tsx` with a **Search** icon (`SearchIcon` from MUI). Clicking it toggles the inline search bar visibility.

### 3.2 Watchlist Pill Bar

A ConfigPanel-based floating panel positioned above the bottom toolbar. Displays entity pills in a flex-wrap layout (not horizontal scroll). Supports minimize/expand/close, clear watchlist, and a visibility cycle button.

```
┌──────────────── Watchlist ──── [–][×] ──────────────┐
│ TRACKING  2 pinned / 3 total  [🗑][👁]              │
│ ┌──────────────────────────────────────────────────┐ │
│ │ [📌][✈ UAL1234 ✕] [📌][🛰 ISS ✕] [◆ EQ_4.2 ✕] │ │  ← Flex-wrap pills
│ └──────────────────────────────────────────────────┘ │
│ [🔍][Lyr][Set][Nav][Ann][Sav]                        │  ← Bottom toolbar
└──────────────────────────────────────────────────────┘
```

**Each pill contains:**
- Layer-type icon (canvas-based icon rendering via `layerIcons.tsx`)
- Entity name (truncated with ellipsis at max-width 100px)
- Pin toggle icon (📌) — filled when pinned, outline when unpinned
- Remove button (✕)

**Pill states:**
| State | How it gets there | Behavior |
|-------|-------------------|----------|
| **Selected (unpinned)** | User clicks entity on globe | Pill appears. Disappears when entity is deselected (click empty space). |
| **Pinned** | User clicks pin icon on pill | Pill persists. Tactical brackets persist. Entity stays visible even if layer is toggled off. Survives page reload (localStorage). |

**Pill interactions:**
- **Click pill** → Select entity (show detail panel)
- **Double-click pill** → Fly camera to entity position
- **Click pin** → Toggle pinned state
- **Click ✕** → Remove from watchlist (and unpin if pinned)

**Panel controls:**
- **Minimize/expand** — ConfigPanel minimizable mode; minimized title shows count (e.g., "Watchlist (3)")
- **Close (×)** — Clears unpinned entities and closes panel
- **Clear watchlist (🗑)** — `DeleteSweepIcon`, removes all entities from watchlist
- **Visibility cycle (👁)** — Cycles: show all → dim non-pinned → hide non-pinned → show all

### 3.3 Inline Search Bar

Toggled by the Search toolbar button. Opens as a full-screen fixed overlay (zIndex: 1200) with `rgba(5, 5, 5, 0.95)` background and `backdrop-filter: blur(16px)`. Centers a search panel on screen.

```
┌──────────────── Full-screen overlay (rgba(5,5,5,0.95)) ──────────────────┐
│                                                                           │
│              ┌──────────────────────────────────────┐                     │
│              │ 🔍 Search by ID, name, or callsign.. │                     │
│              │──────────────────────────────────────│                     │
│              │ ✈  UAL1234       FLIGHTS_COMMERCIAL  │                     │
│              │ ✈  UAL1235       FLIGHTS_COMMERCIAL  │                     │
│              │ 🛰  ISS (ZARYA)  SATELLITES           │                     │
│              │ ◆  EQ_4.2 Tonga  EARTHQUAKES          │                     │
│              └──────────────────────────────────────┘                     │
│                                                                           │
└───────────────────────────────────────────────────────────────────────────┘
```

**Search behavior:**
- Debounced input (300ms) triggers API query
- Searches across **all entities in PostgreSQL** (not just live cache)
- Matches against: `external_id` and `name` columns only
- Supports partial/prefix matching (e.g., "UAL" matches "UAL1234")
- Results show: layer icon, entity name (bold), layer type label (dimmed, uppercase)
- Clicking a result row adds the entity to the watchlist (pinned by default); already-added items show a check icon and persistent highlight
- Max 20 results displayed; shows "N more — refine your search" footer if truncated
- Empty state: "No entities found" message
- Search input auto-focuses when opened; ESC or click-away closes the search bar

### 3.4 Find Mode

A focused analysis mode where only watchlist entities are visible on the globe.

**Activation:** Single visibility cycle button in the WatchlistBar toolbar (VisibilityIcon/BlurOnIcon/VisibilityOffIcon). The button cycles through three states:

1. **Show all** (VisibilityIcon) — Normal rendering, all entities visible
2. **Dim non-pinned** (BlurOnIcon, primary color) — Non-pinned entities render at reduced opacity, no interaction
3. **Hide non-pinned** (VisibilityOffIcon, primary color) — Non-pinned entities are completely removed from rendering

**Behavior:**
- Only **pinned** entities render on the globe with full brightness and tactical brackets
- Non-pinned layer entities are either **hidden** or **dimmed** depending on the current cycle state
- Layer panel shows a visual indicator that Find Mode is active
- Exiting Find Mode (cycling back to "show all") restores normal layer rendering
- Also accessible via keyboard shortcut (`Ctrl+F` / `Cmd+F`)

### 3.5 Entity Persistence (Pinned Entities)

Pinned entities have special rendering behavior:

1. **Tactical brackets persist** — brackets remain visible regardless of selection state
2. **Layer independence** — pinned entities render even if their layer is toggled off, deleted, or not subscribed
3. **Last-observation fetch** — for entities not in the live WebSocket stream, the frontend fetches their last known observation from the REST API (`GET /v1/entities/{id}`) and renders a billboard at that position
4. **Stale indicator** — pinned entities whose last observation is older than the layer's cache TTL display a dimmed or outlined icon to indicate staleness
5. **Auto-refresh** — if a pinned entity's layer is active and streaming, the entity's position updates in real-time as normal

### 3.6 Selection ↔ Watchlist Integration

The watchlist integrates with the existing entity selection system:

| Action | Watchlist effect |
|--------|-----------------|
| Click entity on globe | Entity added to pill bar as **unpinned** |
| Click empty space (deselect) | **Unpinned** entities removed from pill bar. **Pinned** entities stay. |
| Ctrl+Click multi-select | Each entity added to pill bar as **unpinned** |
| Search → Add | Entity added as **pinned** |
| Pin toggle | Toggles between pinned/unpinned |
| Remove (✕) | Removed from pill bar entirely |

The existing `selectedEntities` array continues to drive detail panel rendering. The watchlist is a **superset** — it contains all selected entities plus any pinned entities that aren't currently selected.

---

## 4. Data Model

### 4.1 Frontend State (Zustand Store)

New state fields in `app/store.ts`:

```typescript
// --- Constants ---
const MAX_PINNED_ENTITIES = 20;

// --- Watchlist State ---
interface WatchlistEntity {
  entityId: string        // composite: "layerType:externalId"
  layerId: string         // layer type
  name: string            // display name
  pinned: boolean         // persists through deselection and page reload
  addedAt: number         // timestamp for ordering
}

interface WatchlistState {
  // Watchlist
  watchlistEntities: WatchlistEntity[]
  findMode: boolean
  findModeDisplay: 'hidden' | 'dimmed'
  searchOpen: boolean
  searchQuery: string
  watchlistPanelOpen: boolean
  watchlistBarHeight: number

  // Actions
  addToWatchlist: (entity: WatchlistEntity) => void
  removeFromWatchlist: (entityId: string) => void
  togglePin: (entityId: string) => void
  pinEntity: (entityId: string) => void
  unpinEntity: (entityId: string) => void
  clearUnpinned: () => void
  clearWatchlist: () => void
  toggleFindMode: () => void
  setFindModeDisplay: (mode: 'hidden' | 'dimmed') => void
  toggleSearch: () => void
  setSearchQuery: (query: string) => void
  setWatchlistPanelOpen: (open: boolean) => void
  setWatchlistBarHeight: (height: number) => void
}
```

### 4.2 LocalStorage Persistence

Pinned entities are persisted to `localStorage` under key `respondent:watchlist`:

```json
{
  "version": 1,
  "pinnedEntities": [
    {
      "entityId": "flights_commercial:UAL1234",
      "layerId": "flights_commercial",
      "name": "UAL1234",
      "addedAt": 1710288000000
    }
  ]
}
```

On app startup:
1. Load pinned entities from localStorage
2. For each pinned entity, fetch latest observation via `GET /v1/entities/{entityId}`
3. Render billboards at last-known positions
4. If entity's layer is active and streaming, real-time updates take over

### 4.3 Backend: Search API

New gRPC RPC and REST endpoint:

**Proto definition** (`api/proto/entities.proto`):

```protobuf
// New RPC
rpc SearchEntities(SearchEntitiesRequest) returns (SearchEntitiesResponse) {
  option (google.api.http) = {
    get: "/v1/entities/search"
  };
}

message SearchEntitiesRequest {
  string query = 1;           // Search term (partial match on id, name, metadata)
  string layer_type = 2;      // Optional: filter by layer type
  int32 limit = 3;            // Max results (default 20, max 100)
}

message SearchEntitiesResponse {
  repeated EntitySearchResult results = 1;
  int32 total_count = 2;      // Total matches (for "N more results" UI)
}

message EntitySearchResult {
  string entity_id = 1;       // composite: "layerType:externalId"
  string external_id = 2;
  string layer_type = 3;
  string name = 4;
  Observation latest_observation = 5;  // Last known position
  map<string, string> metadata = 6;  // optional
}
```

**REST endpoint:** `GET /v1/entities/search?query=UAL&layer_type=flights_commercial&limit=20`

### 4.4 Backend: PostgreSQL Query

New repository method in `EntityRepository`:

```go
// SearchEntities performs a full-text + prefix search across entities.
SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*EntitySearchResult, int, error)
```

**SQL implementation:**

Uses a CTE with UNION of two separate queries — one matching on `name`, one on `external_id`. Each branch independently sorts and caps results before merging, allowing PostgreSQL to use per-column GIN trigram indexes independently (significantly faster than a single OR across columns).

```sql
WITH candidates AS (
  (SELECT id, name, CASE WHEN name ILIKE $3 THEN 0 ELSE 1 END AS rank
   FROM entities
   WHERE name ILIKE $1 AND ($2 = '' OR layer_type = $2)
   ORDER BY CASE WHEN name ILIKE $3 THEN 0 ELSE 1 END, name
   LIMIT $4)
  UNION
  (SELECT id, name, 1 AS rank
   FROM entities
   WHERE external_id ILIKE $1 AND ($2 = '' OR layer_type = $2)
   ORDER BY name
   LIMIT $4)
)
SELECT
  e.id,
  e.external_id,
  e.layer_type,
  e.name,
  e.metadata,
  e.created_at,
  o.ts AS obs_ts,
  ST_Y(o.position::geometry) AS lat,
  ST_X(o.position::geometry) AS lon,
  o.altitude_m,
  o.velocity
FROM candidates c
JOIN entities e ON e.id = c.id
LEFT JOIN LATERAL (
  SELECT ts, position, altitude_m, velocity
  FROM observations
  WHERE entity_id = e.id
  ORDER BY ts DESC
  LIMIT 1
) o ON true
ORDER BY
  c.rank, e.name ASC
LIMIT $4
```

Where `$1` = `'%' + query + '%'` (wildcard), `$3` = `query + '%'` (prefix for ranking), `$2` = `layerType` filter, `$4` = `limit + 1` (extra row signals "more results exist").

**Index requirements:**

```sql
-- Trigram index for fast ILIKE on name
CREATE INDEX idx_entities_name_trgm ON entities USING gin (name gin_trgm_ops);

-- Trigram index for fast ILIKE on external_id
CREATE INDEX idx_entities_external_id_trgm ON entities USING gin (external_id gin_trgm_ops);

-- B-tree index for layer_type filtering (likely already exists)
CREATE INDEX idx_entities_layer_type ON entities (layer_type);
```

Requires the `pg_trgm` extension: `CREATE EXTENSION IF NOT EXISTS pg_trgm;`

---

## 5. Architecture

### 5.1 Component Hierarchy

```
AppShell
├── calls useSearchEntities() → passes results as props to EntitySearchBar
├── GlobeScene
│   ├── BillboardLayerRenderer (existing)
│   ├── PinnedEntityRenderer (NEW) ← renders pinned entities not in active layers
│   ├── SelectionIndicator (existing, enhanced) ← also renders brackets for pinned entities
│   └── FindModeFilter (NEW) ← controls opacity/visibility of non-pinned entities
├── WatchlistBar (NEW, ConfigPanel-based)
│   ├── EntityPill × N
│   │   ├── LayerIcon (canvas-based via layerIcons.tsx)
│   │   ├── EntityName
│   │   ├── PinButton
│   │   └── RemoveButton
│   ├── ClearWatchlistButton
│   └── VisibilityCycleButton (show all → dim → hide)
├── EntitySearchBar (NEW, full-screen overlay)
│   ├── SearchInput
│   └── SearchResultsList
│       └── SearchResultItem × N
├── BottomToolbar (modified: Map → Search icon)
└── EntityDetailPanel × N (existing, unchanged)
```

### 5.2 Data Flow

```
                    ┌─────────────────────────┐
                    │      User Actions        │
                    └─────────┬───────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
   Click entity          Type search          Toggle pin
   on globe              query                on pill
          │                   │                   │
          ▼                   ▼                   ▼
   setSelectedEntity    AppShell calls       togglePin()
   + addToWatchlist     useSearchEntities    in store
   (unpinned)           (React Query)               │
          │                   │                   ▼
          │                   ▼              localStorage
          │            GET /v1/entities/     sync
          │            search?query=...
          │                   │
          │                   ▼
          │            SearchEntitiesResponse
          │                   │
          │                   ▼
          │            Passed as props to
          │            EntitySearchBar
          │                   │
          │                   ▼
          │            User clicks result row
          │                   │
          │                   ▼
          └──────────► addToWatchlist(pinned)
                              │
                              ▼
                    ┌─────────────────────────┐
                    │   Zustand Store          │
                    │   watchlistEntities[]    │
                    └─────────┬───────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       WatchlistBar    PinnedEntity    SelectionIndicator
       (pill UI)       Renderer        (brackets for all
                       (globe)         watchlist entities)
```

### 5.3 Frontend File Map

| File | Purpose | Status |
|------|---------|--------|
| `app/store.ts` | Add WatchlistState slice + `MAX_PINNED_ENTITIES` | Modify |
| `app/store.watchlist.test.ts` | Watchlist store tests | New |
| `shared/hud/BottomToolbar.tsx` | Replace Map icon with Search icon | Modify |
| `shared/hud/MobileBottomNav.tsx` | Replace Map icon with Search icon | Modify |
| `features/search/WatchlistBar.tsx` | ConfigPanel-based pill bar component | New |
| `features/search/WatchlistBar.test.tsx` | Pill bar tests | New |
| `features/search/EntityPill.tsx` | Individual pill component | New |
| `features/search/EntityPill.test.tsx` | Pill tests | New |
| `features/search/EntitySearchBar.tsx` | Full-screen overlay search input + results | New |
| `features/search/EntitySearchBar.test.tsx` | Search bar tests | New |
| `features/search/SearchResultItem.tsx` | Search result row | New |
| `features/search/layerIcons.tsx` | Canvas-based layer icon rendering | New |
| `features/search/useSearchEntities.ts` | React Query hook for search API | New |
| `features/search/useWatchlistPersistence.ts` | localStorage sync hook | New |
| `features/search/useWatchlistPersistence.test.ts` | Persistence tests | New |
| `features/search/useWatchlistBarHeight.ts` | ResizeObserver hook for bar height | New |
| `features/search/useWatchlistSync.ts` | Batch-fetches observation data for watchlist | New |
| `features/globe/PinnedEntityRenderer.tsx` | Renders billboards for pinned entities not in active layers | New |
| `features/globe/FindModeFilter.tsx` | Controls entity visibility in Find Mode | New |
| `features/globe/SelectionIndicator.tsx` | Extend to render brackets for pinned entities | Modify |
| `features/globe/useSearchKeyboard.ts` | Keyboard shortcut implementation (`Ctrl+K`, etc.) | New |

### 5.4 Backend File Map

| File | Purpose | Status |
|------|---------|--------|
| `api/proto/entities.proto` | Add SearchEntities RPC + messages | Modify |
| `internal/repo/interfaces.go` | Add SearchEntities to EntityRepository | Modify |
| `internal/repo/postgres/entity.go` | Implement SearchEntities SQL query | Modify |
| `internal/bff/entity_service.go` | Add SearchEntities service method | Modify |
| `internal/server/entity_grpc_server.go` | Add SearchEntities gRPC handler | Modify |
| `migrations/NNN_entity_search_indexes.up.sql` | Add pg_trgm extension + trigram indexes | New |
| `migrations/NNN_entity_search_indexes.down.sql` | Drop indexes | New |

---

## 6. UI Specifications

### 6.1 Watchlist Pill Bar

- **Component:** ConfigPanel-based (minimizable, with close button)
- **Position:** Fixed, centered horizontally, above `BottomToolbar` (desktop: bottom 100px) or `MobileBottomNav` (mobile)
- **Width:** 480px (desktop) / full-width minus margins (mobile); max-width 560px
- **Max height:** 300px (desktop) / 40dvh (mobile)
- **Title:** "Watchlist" with BookmarksIcon
- **Status label:** "TRACKING" — shows "{N} pinned / {M} total" or "{N} tracked"
- **Background:** Inherited from ConfigPanel (glassmorphism)
- **Layout:** Flex-wrap (not horizontal scroll)
- **Gap between pills:** 6px (0.75 × 8)
- **Toolbar actions:** Clear watchlist (DeleteSweepIcon) + Visibility cycle button
- **Visibility:** Only visible when `watchlistEntities.length > 0`
- **Clean UI mode:** Hidden when `cleanUI === true`
- **Recording mode:** Hidden when `recordingMode === true`
- **Auto-open:** Panel opens and de-minimizes when entities are added

### 6.2 Entity Pill

- **Size:** Auto-width, 32px height
- **Background:** `rgba(255, 255, 255, 0.06)`
- **Border:** 1px `rgba(255, 255, 255, 0.12)`, rounded 16px (full pill)
- **Border (pinned):** 1px `rgba(0, 255, 157, 0.4)` (primary color)
- **Icon:** 14px layer icon (canvas-based via `layerIcons.tsx`), left side
- **Name:** Monospace, 0.6875rem, `#e0e0e0`, max-width 100px with ellipsis
- **Pin icon:** 14px, right side. Filled (`PushPinIcon`) `primary.main` when pinned, outlined (`PushPinOutlinedIcon`) `#666` when unpinned
- **Remove (✕):** 12px (`CloseIcon`), right side, `#666`, hover `secondary.main`
- **Hover:** Background lightens to `rgba(255, 255, 255, 0.1)`
- **Active:** Background flashes `rgba(0, 255, 157, 0.15)`
- **Gap between pills:** 6px
- **Double-click:** Fly camera to entity position

### 6.3 Search Bar

- **Layout:** Full-screen fixed overlay (zIndex: 1200), centered on screen
- **Overlay background:** `rgba(5, 5, 5, 0.95)` with `backdrop-filter: blur(16px)`
- **Panel:** Max-width 560px, centered, with border-radius and border
- **Panel background:** `rgba(5, 5, 5, 0.95)` with `backdrop-filter: blur(16px)`
- **Border:** 1px `rgba(0, 255, 157, 0.2)` with subtle green box-shadow
- **Input:** Full-width, monospace, 0.8125rem, transparent background, `#e0e0e0` text
- **Placeholder:** "Search by ID, name, or callsign..." in `#555`
- **Search icon:** 16px `#555` prefix icon
- **Results list:** Max-height 60vh (desktop) / 50dvh (mobile), scrollable
- **Result item:** 36px height (desktop) / 44px (mobile), hover highlight `rgba(255, 255, 255, 0.06)`
- **Result item content:** Layer icon (18px) or CheckCircleIcon if already in watchlist, entity name (bold, 0.8125rem), layer type (0.625rem, uppercase, dimmed)
- **Already-added items:** Persistent green highlight `rgba(0, 255, 157, 0.08)`, check icon replaces layer icon, click is no-op
- **Loading state:** Skeleton shimmer on 3 placeholder rows
- **No results:** Centered "No entities found" in `#555`, 0.75rem monospace
- **Animation:** Fade in (200ms)
- **Close:** Click-away or ESC

### 6.4 Find Mode Controls

- **Visibility cycle button:** Single IconButton in WatchlistBar toolbar
  - **Show all** (VisibilityIcon, `text.secondary`) → no filtering
  - **Dim non-pinned** (BlurOnIcon, `primary.main`) → Find Mode active, `findModeDisplay: 'dimmed'`
  - **Hide non-pinned** (VisibilityOffIcon, `primary.main`) → Find Mode active, `findModeDisplay: 'hidden'`
- **Clear watchlist button:** DeleteSweepIcon, `text.secondary`, hover `error.main`
- **No separate glow, label, or pulse indicators** — Find Mode state is communicated via the visibility cycle button icon and color

### 6.5 Mobile Adaptations

- WatchlistBar renders above `MobileBottomNav` with same safe area padding (`--sab`)
- Pills use flex-wrap layout with touch-friendly targets
- Search bar overlay aligns to top of screen on mobile with `max-height: calc(50dvh - var(--sat))`
- Result items are 44px height for touch targets
- Visibility cycle button has 40×40px minimum touch target
- Long-press on pill shows a context menu: Pin / Unpin / Remove / Fly To

---

## 7. Implementation Phases

### Phase 1: Backend Search API
1. Add `pg_trgm` extension migration
2. Add trigram indexes on `entities.name` and `entities.external_id`
3. Define `SearchEntities` RPC in `entities.proto`
4. Generate gRPC code
5. Implement `SearchEntities` in entity repository (PostgreSQL)
6. Implement `SearchEntities` in BFF entity service
7. Wire gRPC handler and HTTP gateway route
8. Write backend tests

### Phase 2: Watchlist Store & Persistence
1. Add `WatchlistState` to Zustand store
2. Implement localStorage persistence hook
3. Implement `clearUnpinned()` integration with `clearSelection()`
4. Write store unit tests

### Phase 3: Watchlist Pill Bar UI
1. Create `EntityPill` component
2. Create `WatchlistBar` component (ConfigPanel-based, flex-wrap layout)
3. Replace Map icon with Search icon in `BottomToolbar` and `MobileBottomNav`
4. Integrate with selection system (auto-add on select, auto-remove unpinned on deselect)
5. Implement fly-to-entity on pill click
6. Write component tests

### Phase 4: Inline Search Bar
1. Create `useSearchEntities` React Query hook
2. Create `SearchResultItem` component
3. Create `EntitySearchBar` component
4. Wire search open/close to toolbar button
5. Implement add-to-watchlist from search results
6. Write component tests

### Phase 5: Pinned Entity Rendering
1. Create `PinnedEntityRenderer` — fetches and renders billboards for pinned entities not in active layers
2. Extend `SelectionIndicator` to render brackets for all pinned entities (not just selected)
3. Implement stale entity visual indicator
4. Write rendering tests

### Phase 6: Find Mode
1. Add visibility cycle button to WatchlistBar
2. Create `FindModeFilter` — controls billboard opacity/visibility
3. Implement show all → dim → hide cycle logic
4. Add keyboard shortcut (`Ctrl+F` / `Cmd+F`) via `useSearchKeyboard`
5. Write tests

### Phase 7: Mobile Adaptations
1. Mobile pill bar layout with safe areas
2. Touch-optimized search bar
3. Long-press context menu on pills
4. Mobile Find Mode UI
5. Manual testing on iOS Safari

---

## 8. Performance Considerations

| Concern | Mitigation |
|---------|-----------|
| Search query load on PostgreSQL | GIN trigram indexes for fast ILIKE. CTE with UNION lets PostgreSQL use per-column indexes independently. Debounce input (300ms). Limit results to 20. |
| Too many pinned entities | Hard cap at `MAX_PINNED_ENTITIES` (= 20). Store silently rejects pins beyond the limit. |
| Billboard rendering for pinned entities not in active layers | Fetch last observation once, cache in store. Only re-fetch on explicit refresh or when entity's layer becomes active. |
| Find Mode opacity changes | Apply opacity via `BillboardCollection` alpha, not per-billboard. Single GPU state change. |
| localStorage bloat | Only store entity identifiers + names (not observations). ~100 bytes per entity. Cap at 50 entries. |
| Search API abuse | Rate limit search endpoint (10 req/s per client). Minimum query length of 2 characters. |

---

## 9. Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+F` / `Cmd+F` | Toggle Find Mode |
| `Ctrl+K` / `Cmd+K` | Open/focus search bar |
| `Escape` | Close search bar (if open), exit Find Mode (if active) |
| `Enter` (in search) | Add first result to watchlist |
| `Arrow Up/Down` (in search) | Navigate search results |

---

## 10. Edge Cases

| Scenario | Behavior |
|----------|----------|
| Pinned entity's layer deleted from backend | Entity remains in watchlist. Billboard renders from last cached observation. Stale indicator shown. |
| Search returns entity already in watchlist | [+ Add] button shows checkmark. Clicking it is a no-op. |
| Entity selected on globe + already pinned | Pill remains. No duplicate. Selection state updated. |
| Page reload with pinned entities | Fetch last observations for all pinned entities on startup (parallel requests). Show loading skeleton pills while fetching. |
| Find Mode with zero pinned entities | Warn user: "Pin entities to use Find Mode". Don't activate. |
| Pinned entity has no observations in DB | Show pill with "?" icon. No billboard rendered. Tooltip: "No position data available." |
| Search query matches thousands of entities | Return first 20 ordered by relevance (exact prefix first). Show "N more results — refine your search" footer. |
| Multiple sessions/tabs | localStorage changes are not synced across tabs in real-time (acceptable limitation). |

---

## 11. Testing Strategy

### Unit Tests
- Watchlist store: add, remove, pin, unpin, clearUnpinned, Find Mode toggle, display mode
- localStorage persistence: save, load, migration, corruption recovery
- EntityPill: render states (pinned/unpinned), click handlers, truncation
- WatchlistBar: flex-wrap layout, empty state, max entities warning, visibility cycle
- EntitySearchBar: debounce, results rendering, add-to-watchlist, keyboard navigation
- FindModeFilter: hidden vs dimmed rendering

### Integration Tests
- Selection → watchlist flow: click entity → pill appears → deselect → unpinned removed, pinned stays
- Search → add → pin flow: search → add result → verify billboard + bracket rendered
- Find Mode: activate → verify only pinned entities visible → deactivate → verify all entities restored
- Persistence: pin entities → reload page → verify pills restored → verify billboards rendered

### Backend Tests
- SearchEntities: exact match, prefix match, partial match, layer filter, empty results, SQL injection safety
- Performance: search query execution time < 100ms for 1M entity table

---

## 12. Future Enhancements (Out of Scope)

- **Shared watchlists** — multiple analysts sharing tracked entities via WebSocket
- **Watchlist naming** — save/load named watchlists (e.g., "Pacific Fleet Tracking")
- **Alert rules** — notify when a pinned entity enters/exits a geofence
- **Entity grouping** — group pills by layer type with collapsible sections
- **Search filters UI** — layer type dropdown, time range filter, spatial bounding box filter in the search bar
- **Fuzzy matching** — Levenshtein distance for typo tolerance in search
