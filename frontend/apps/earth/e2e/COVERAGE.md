# Earth App — UI Interaction Coverage Map

Round 4 (e2e hardening) raised Playwright coverage from a ~15% smoke layer to
pretty much every **mounted, non-stub** UI interaction. Each interaction below
maps to an e2e spec, a unit test, or a documented exclusion (with reason).

**Harness:** isolated Go binary on `:8091` + throwaway `.e2e/` SQLite DB
(`make e2e`). Deterministic tier uses `helpers/routes.ts` (`setupMockRoutes`,
`setupNotificationRoutes`) so specs pass with NO live feed; the data-gated tier
(`waitForEntityData` + `test.skip`) still runs against a live feed where present.
Globe selection is driven via `helpers/selection.ts` + the `window.__store*`
automation hooks (no canvas pixel clicks).

## E2E specs (deterministic unless noted)

| Spec | Interactions covered |
|------|----------------------|
| `app-load.spec.ts` | shell + canvas + bell load |
| `ws-reconnect.spec.ts` | WS LIVE + auto-reconnect |
| `progressive-loading.spec.ts` | layer enable → billboards → pan persistence (**data-gated**) |
| `entity-data.spec.ts` | search returns results (**data-gated**) |
| `panels.spec.ts` | settings/nav panel open+close, bell, search open/Escape |
| `_infra.spec.ts` | stub-feed + selection-helper smoke |
| `layers.spec.ts` | layer toggle `data-active`, toggle-all visibility, clear-all + disabled, Enter/Space |
| `settings.spec.ts` | display presets (NORMAL/CRT/NVG/FLIR) w/ old-preset deactivation + keyboard; switches (occluded/geo-labels/smooth-motion/cinematic-drift) two-direction; 3D-buildings disabled branch; render-limit slider via keyboard |
| `watchlist.spec.ts` | add via search, pill select, pin/unpin, remove, clear-all, find-mode cycle |
| `search.spec.ts` | ArrowDown/Up focus, Enter-adds-to-watchlist, click-away close, no-results |
| `globe-keyboard.spec.ts` | Cmd/Ctrl+K search, Cmd/Ctrl+F findMode, Escape priority (search→findMode) |
| `entity-panel.spec.ts` | selection mounts panel + content, tab switch (overview/timeline/metadata), maximize/restore/close |
| `globe-interaction.spec.ts` | single-click select, empty-click clear, double-click→entity-view, exit-view |
| `entity-history.spec.ts` | view-mode switch (Trajectory/Table/Chart), trails/isolate, load-more, copy-menu opens |
| `time-range.spec.ts` | trigger, presets (LIVE/1h/8h/24h), custom-range (tabs/month/day/time/apply) w/ value-tied assertion |
| `navigation.spec.ts` | reset-view (converges to default camera), zoom in/out, north-up, minimize |
| `notifications.spec.ts` | bell, mark-all-read, filter-toggle, item expand+read; attention/insight filter narrowing (server-side refetch), reset, entity-chip selection, load-more, show-more-entities |
| `mobile.spec.ts` (project `mobile-chrome`, Pixel 5) | bottom-nav drawers (Layers/Settings/Nav), search, mobile entity panel, recording (enter/exit, aspect-ratio, rule-of-thirds grid) |

## Covered by unit tests (hard-to-e2e → unit, per DoD)

| Interaction | Unit test |
|-------------|-----------|
| History copy CSV/JSON/Markdown (clipboard) | `tabs/ObservationDataView.test.tsx` (`buildCsvString`/`buildJsonString`/`buildMarkdownString`) |
| ObservationChart field-pill toggle + min-one-active | `tabs/ObservationChart.test.tsx` |
| Indicator gauge/panel rendering + slice | `features/indicators/IndicatorHUD.test.tsx`, `packages/core/.../createIndicatorSlice.test.ts` |
| Notification filter bar / item / panel logic | `shared/notifications/*.test.tsx` |

## Documented exclusions

| Interaction | Why excluded | Mitigation |
|-------------|--------------|------------|
| **Indicators** (panel open/close/disable in e2e) | Render only from live WS `indicator.update` frames (no REST path); deterministic e2e needs WS-frame injection or a `window.__store_setIndicator` seam | `indicators.spec.ts` documents it; UI+slice unit-tested. Follow-up: add the seam for e2e if wanted |
| Drag-to-reposition panels | Synthetic drag + transform timing is fragile | excluded (low-value) |
| Camera tilt/pitch, scroll-zoom, hover-cursor | WebGL-canvas, no DOM observable / Playwright viewport-agnostic visibility | excluded; camera move covered via `__cesiumViewer` reads in navigation/progressive-loading |
| Recharts tooltip hover | recharts internals have no stable testid | excluded; chart logic unit-tested |
| Observation-table alt+click / context-menu copy | clipboard + brittle menu | excluded; formatters unit-tested |

## Dead / unmounted UI (NOT covered — Round 5 deletion candidates)

CCTVPanel, EffectsPanel, StylePresetBar/PresetBar, SavedLocationsModal, and the
inert Annotate/Save toolbar buttons are not mounted in `EarthShell` and have no
route — verified during the audit. They are excluded from e2e and flagged for
deletion in the publish-hygiene round.
