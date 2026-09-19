# Frontend Mobile Support

This document describes how the Respondent frontend adapts to mobile viewports. The mobile experience is a **progressive enhancement** of the existing React/MUI/Cesium SPA — not a separate app. Desktop behavior is unchanged; mobile-specific layout and interactions activate below the `md` breakpoint (900px).

---

## Architecture

```
┌──────────────────────────────────────────────┐
│                  AppShell                     │
│  ┌────────────────────────────────────────┐  │
│  │         GlobeScene (Cesium)            │  │
│  │  Touch gestures: pinch / pan / rotate  │  │
│  └────────────────────────────────────────┘  │
│                                               │
│  if isMobile:                                 │
│  ┌────────────────────────────────────────┐  │
│  │ MobileHudLayout     (top bar + telem) │  │
│  │ IndicatorHUD        (alert badges)    │  │
│  │ EntitySearchBar     (entity search)   │  │
│  │ WatchlistBar        (watchlist strip) │  │
│  │ MobileDrawerPanels  (layers/settings) │  │
│  │ MobileEntityPanel   (floating detail) │  │
│  │ MobileBottomNav     (tab bar)         │  │
│  └────────────────────────────────────────┘  │
│                                               │
│  if !isMobile:                                │
│  ┌────────────────────────────────────────┐  │
│  │ Desktop HUD + floating ConfigPanels   │  │
│  └────────────────────────────────────────┘  │
│                                               │
│  RecordingMode (both mobile + desktop)        │
└──────────────────────────────────────────────┘
```

The `AppShell` component (`app/AppShell.tsx`) uses the `useResponsive()` hook to branch between mobile and desktop layouts. Both layouts share the same `GlobeScene`, Zustand store, and data layer infrastructure.

---

## Responsive Detection

**File:** `shared/hooks/useResponsive.ts`

```typescript
import { useMediaQuery, useTheme } from '@mui/material'

export function useResponsive() {
  const theme = useTheme()
  const isMobile = useMediaQuery(theme.breakpoints.down('md'))   // <900px
  const isTablet = useMediaQuery(theme.breakpoints.between('md', 'lg'))
  const isDesktop = useMediaQuery(theme.breakpoints.up('lg'))
  return { isMobile, isTablet, isDesktop }
}
```

The breakpoint uses MUI's standard `md` threshold (900px). `useMediaQuery` internally uses `window.matchMedia` and re-renders on viewport changes.

---

## Viewport & Safe Areas

### Viewport meta

`index.html` sets:

```html
<meta name="viewport"
  content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no, viewport-fit=cover" />
```

- `viewport-fit=cover` extends the web content into the notch/Dynamic Island safe area.
- `user-scalable=no` prevents accidental zoom (Cesium handles zoom via pinch gestures).

### CSS safe area variables

`index.css` exposes the four safe area insets as CSS custom properties:

```css
:root {
  --sat: env(safe-area-inset-top, 0px);
  --sar: env(safe-area-inset-right, 0px);
  --sab: env(safe-area-inset-bottom, 0px);
  --sal: env(safe-area-inset-left, 0px);
}
```

Components reference these in `sx` props — e.g., `pt: 'calc(var(--sat) + 8px)'` for top padding that clears the notch, or `pb: 'calc(var(--sab) + 4px)'` for bottom padding that clears the home indicator.

### Dynamic viewport height

`frontend/index.html` includes an inline `<style>#root { height: 100vh }</style>` as a baseline, which is then overridden by `index.css` to use `100dvh` for mobile browsers:

```css
#root {
  height: 100dvh;
}
```

This ensures mobile browsers account for dynamic address bar height (Safari). The inline `100vh` in the HTML file acts as a fallback for environments where CSS custom properties or `dvh` are unsupported.

---

## Mobile HUD Layout

**File:** `shared/hud/MobileHudLayout.tsx`

Replaces the desktop Header, TelemetryStrip, RecBlock, and StatusReadout with a condensed two-bar layout:

```
┌────────────────────────────────┐
│ ● RESPONDENT          ● LIVE  │  ← Top bar (gradient overlay)
│                                │
│          [3D GLOBE]            │
│                                │
│   40.7°N 74.0°W  ALT: 1.2km  │  ← Telemetry strip
│ ┌────┐┌────┐┌────┐┌────┐┌────┐│  ← MobileBottomNav
│ │Src ││Lyr ││Set ││Nav ││Rec ││
│ └────┘└────┘└────┘└────┘└────┘│
└────────────────────────────────┘
```

**Top bar** — absolute-positioned with a downward gradient (`rgba(0,0,0,0.8)` → transparent). Shows the app name and a connection status indicator (LIVE / RANGE / RECONN / OFFLINE) with a pulsing dot.

**Telemetry strip** — positioned above the bottom nav. Shows camera coordinates and altitude in a compact pill.

Both bars use `pointerEvents: 'none'` on their container (with `pointerEvents: 'auto'` on interactive children) so touch events pass through to the globe.

The entire HUD hides when `recordingMode` is active.

---

## Mobile Bottom Navigation

**File:** `shared/hud/MobileBottomNav.tsx`

A fixed bar at the bottom of the viewport with 5 navigation items:

| Item     | Icon           | Action                              |
|----------|----------------|-------------------------------------|
| Search   | `SearchIcon`   | `toggleSearch()`; shows watchlist badge when `watchlistEntities.length > 0` |
| Layers   | `LayersIcon`   | Opens layers drawer                 |
| Settings | `SettingsIcon` | Opens settings drawer               |
| Nav      | `ExploreIcon`  | Opens navigation drawer             |
| Record   | `VideocamIcon` | Toggles recording mode              |

Each item meets the 44×44px minimum touch target (Apple HIG). The active drawer is highlighted with the primary color. Tapping an already-active item closes its drawer.

The nav bar respects `--sab` for home indicator clearance and hides during recording mode.

---

## Mobile Drawer Panels

**File:** `shared/ui/MobileDrawer.tsx`

Wraps MUI's `SwipeableDrawer` (anchor=bottom) to present panels as bottom sheets. Only one drawer is open at a time, controlled by `activeMobileDrawer` in the Zustand store.

```typescript
interface MobileDrawerProps {
  open: boolean
  onClose: () => void
  onOpen: () => void
  title: string
  children: React.ReactNode
  heightPercent?: number  // default: 60
}
```

Key behaviors:
- `disableSwipeToOpen` — prevents accidental open from edge swipe (conflicts with Cesium touch)
- Swipe-down-to-dismiss is enabled
- `keepMounted: true` — preserves panel state when closed
- Max height capped at `85vh` so the globe is always partially visible
- Semi-transparent backdrop (`rgba(0,0,0,0.4)`) keeps context
- Drag handle at the top for visual affordance

Existing panel components (`DataLayersPanel`, `SettingsPanel`, `NavigationPanel`) accept a `mobile` prop that renders their content without the floating `ConfigPanel` wrapper — the drawer provides the container instead.

---

## Mobile Entity Detail Panel

**File:** `features/entity/MobileEntityPanel.tsx`

Entity details on mobile use a **floating panel** instead of a drawer. This is a deliberate design decision — Cesium's `ScreenSpaceEventHandler` runs outside React's render cycle, which created timing issues with imperative drawer opens. The floating panel is purely declarative: it renders whenever `selectedEntities.length > 0` in the store.

### Two-state model

Each entity panel has two states controlled by a boolean `isMaximized`:

| State     | Appearance                                           |
|-----------|------------------------------------------------------|
| Minimized | Compact header bar: entity name + status badge + controls |
| Maximized | Near-full-screen overlay with tabs and scrollable content |

Toggling between states: tap the header bar, or click the maximize/restore icon button.

### Component structure

```
MobileEntityPanel (container)
  └─ MobileEntityPanelItem × N (one per selected entity)
        ├─ PanelHeader (React.memo) — name, status, maximize/close
        ├─ TabBar (React.memo) — Overview / AIAnalysis / History / Metadata
       ├─ Tab content (scrollable)
       └─ ExitButton (when tracking an entity)
```

The container is positioned `absolute`, anchored to `bottom: calc(var(--sab) + 72px)` — the base offset is `MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP = 56 + 16 = 72px`. When the `WatchlistBar` is present, an additional `watchlistBarHeight + MOBILE_STACK_GAP` is added to avoid overlap. Maximized panels use `position: fixed` with `zIndex: theme.zIndex.modal`.

### Status badge

- **SELECTED** — entity is selected in globe view mode
- **TRACKING** — entity is the primary selection and `viewMode === 'entity'` (camera follows)

### Shared hook

**File:** `features/entity/useEntityDetailWithFallback.ts`

Both `EntityDetailPanel` (desktop) and `MobileEntityPanel` share this hook. It returns entity detail data from the REST API, falling back to in-memory WebSocket store data when the API hasn't persisted the entity yet (e.g., on-demand fetches pending async Postgres persist).

```typescript
function useEntityDetailWithFallback(entityId: string, layerId: string) {
  // 1. Try REST API (React Query)
  // 2. If no API data, build response from layerEntities store (WebSocket data)
  // 3. Subscribe to layerVersions[layerId] for reactivity
  return { detail, isLoading }
}
```

---

## Recording Mode

**Files:** `features/recording/RecordingMode.tsx`, `AspectRatioOverlay.tsx`, `AspectRatioPicker.tsx`

Recording mode optimizes the viewport for screen recording in social media aspect ratios. It is available on both mobile and desktop.

### Activation

Toggled via `toggleRecordingMode()` in the store. On activation:
1. All HUD elements hide (MobileHudLayout, MobileBottomNav, desktop HUD)
2. Entity selection is cleared
3. Aspect ratio frame overlay appears
4. Aspect ratio picker appears (auto-hides after 3 seconds)
5. Screen orientation lock is attempted (best-effort, not all browsers support it)

### Aspect ratios

Defined in `shared/constants/aspectRatios.ts`:

| Key    | Ratio | Label          | Platforms                       |
|--------|-------|----------------|---------------------------------|
| `9:16` | 9/16  | Stories/Reels  | Instagram, TikTok, YouTube Shorts |
| `4:5`  | 4/5   | Portrait Post  | Instagram, Facebook             |
| `1:1`  | 1/1   | Square         | Instagram, Twitter/X            |
| `16:9` | 16/9  | Landscape      | YouTube, Twitter/X              |
| `free` | —     | Free           | No overlay                      |

### Frame overlay

`AspectRatioOverlay` renders four semi-transparent letterbox/pillarbox bars around the active frame, calculated from the viewport dimensions and selected ratio. The frame area is transparent with a subtle green border. An optional rule-of-thirds grid can be toggled.

All overlay elements use `pointerEvents: 'none'` so globe interaction passes through.

The overlay recalculates on `resize` and `orientationchange` events.

### Picker controls

`AspectRatioPicker` renders a horizontal pill bar at the bottom of the screen with:
- 5 ratio buttons (44×44px touch targets)
- Grid toggle button
- Exit button (red)

The picker auto-hides after 3 seconds of inactivity. Tapping anywhere on the screen reveals it again.

---

## Post-Processing Effects on Mobile

**File:** `shared/hud/PostProcessOverlay.tsx`

The four visual presets (NORMAL, CRT, NVG, FLIR) are CSS-based overlays applied to the globe container. On mobile, the component detects the viewport via `useMediaQuery` and adjusts:

| Effect  | Mobile adjustment                                 |
|---------|---------------------------------------------------|
| CRT     | Wider scanline gap (3px vs 2px) to avoid moiré    |
| NVG     | Reduced noise opacity (0.15 vs 0.25) for video compression |
| NORMAL  | Lighter vignette (0.2 vs 0.3 opacity)             |
| FLIR    | Reduced sensor noise opacity (0.08 vs 0.14) for video compression |

---

## Zustand Store Extensions

The following state fields and actions in `app/store.ts` support the mobile system:

### State

```typescript
// Mobile drawer
activeMobileDrawer: 'layers' | 'settings' | 'nav' | 'effects' | null

// Mobile HUD
mobileHudExpanded: boolean

// Recording mode
recordingMode: boolean
recordingAspectRatio: '9:16' | '4:5' | '1:1' | '16:9' | 'free'
recordingShowGrid: boolean
```

### Actions

| Action                    | Behavior                                          |
|---------------------------|---------------------------------------------------|
| `openMobileDrawer(type)`  | Sets `activeMobileDrawer` to the given type       |
| `closeMobileDrawer()`     | Sets `activeMobileDrawer` to `null`               |
| `toggleMobileHud()`       | Toggles `mobileHudExpanded`                       |
| `toggleRecordingMode()`   | Toggles `recordingMode`; closes drawer and resets grid on exit |
| `setRecordingAspectRatio(key)` | Sets the active aspect ratio                |
| `setRecordingShowGrid(show)`   | Toggles rule-of-thirds grid                 |

---

## File Map

### Mobile-specific files

| File | Purpose |
|------|---------|
| `shared/hooks/useResponsive.ts` | Responsive breakpoint hook |
| `shared/hud/MobileHudLayout.tsx` | Condensed mobile HUD (top bar + telemetry) |
| `shared/hud/MobileBottomNav.tsx` | Touch-friendly bottom tab navigation |
| `shared/ui/MobileDrawer.tsx` | Bottom sheet drawer wrapper |
| `features/entity/MobileEntityPanel.tsx` | Floating entity detail panel |
| `features/entity/useEntityDetailWithFallback.ts` | Shared entity detail hook (API + WS fallback) |
| `features/recording/RecordingMode.tsx` | Recording mode orchestrator |
| `features/recording/AspectRatioOverlay.tsx` | Aspect ratio frame overlay |
| `features/recording/AspectRatioPicker.tsx` | Aspect ratio selector bar |
| `shared/constants/aspectRatios.ts` | Aspect ratio definitions |
| `shared/constants/layout.ts` | Mobile layout constants (`MOBILE_NAV_HEIGHT`, `MOBILE_NAV_GAP`, `MOBILE_STACK_GAP`, `MOBILE_PANEL_MARGIN`) |

### Modified files

| File | Mobile-related changes |
|------|------------------------|
| `frontend/index.html` | `viewport-fit=cover`, `user-scalable=no` |
| `index.css` | Safe area CSS variables, `100dvh` |
| `app/store.ts` | Mobile drawer, recording mode, HUD state |
| `app/AppShell.tsx` | Conditional mobile/desktop layout branching |
| `shared/hud/PostProcessOverlay.tsx` | Mobile-optimized effect parameters |

---

## Testing

Unit tests (Vitest + jsdom) cover the mobile components:

| Test file | What it covers |
|-----------|----------------|
| `MobileEntityPanel.test.tsx` | Visibility, entity name, status badge, minimize/maximize, tabs, close, multi-select |
| `MobileBottomNav.test.tsx` | Nav item rendering, drawer toggling, recording mode hiding |
| `MobileHudLayout.test.tsx` | Top bar, telemetry strip, recording mode hiding |
| `MobileDrawer.test.tsx` | Open/close, title, drag handle, content rendering |
| `AspectRatioOverlay.test.tsx` | Frame calculation, letterbox/pillarbox rendering, grid toggle |
| `AspectRatioPicker.test.tsx` | Ratio selection, auto-hide, grid toggle, exit |
| `useResponsive.test.ts` | Breakpoint detection via mocked `matchMedia` |
| `store.mobile.test.ts` | Mobile store actions and state transitions |

Manual testing targets iPhone 16 Pro Max (Safari + Chrome) at 440×956 viewport.
