/**
 * Indicators — intentionally deferred from the deterministic e2e tier.
 *
 * WHY: IndicatorHUD renders a panel for a layer ONLY when BOTH hold:
 *   1. the layer is an indicator layer (layer.renderingMode === 'indicator',
 *      → createLayerSlice.indicatorLayerIds) and is enabled, AND
 *   2. an IndicatorSnapshot exists in the store for that layer.
 * Snapshots arrive EXCLUSIVELY via live WebSocket `indicator.update` frames
 * (useLayerStream.handleIndicatorMessage → setIndicator). There is no REST
 * endpoint to stub — so without a snapshot, IndicatorHUD returns null and there
 * is nothing DOM-addressable to assert. A deterministic e2e would require either
 * proven mock-WS frame injection (helpers/mock-ws has pushSnapshot but no
 * indicator push, and the WS-push path is unexercised) or a new production store
 * seam (window.__store_setIndicator, mirroring __store_setSelected).
 *
 * COVERAGE TODAY (unit, not e2e): the indicator UI and store logic are covered by
 *   - frontend/apps/earth/src/features/indicators/IndicatorHUD.test.tsx
 *     (panel/gauge rendering, content, colors)
 *   - frontend/packages/core/src/stores/createIndicatorSlice.test.ts
 *     (setIndicator snapshot handling)
 *
 * This matches the round's definition of done: WS-driven content that cannot be
 * cleanly and deterministically driven in e2e is covered by unit tests and documented
 * here rather than shipped as a flaky WS-injection spec. If deterministic e2e
 * indicator coverage is wanted later, add a window.__store_setIndicator seam (like
 * the entity-view seam) + an indicator-renderingMode enabled-layer stub and assert
 * indicator-panel-{layerId} open/close/minimize.
 */
import { test } from '@playwright/test';

test.fixme('indicator panel open/close/minimize/disable — WS-driven (covered by unit tests; see file header)', () => {
  // Deferred: requires WS frame injection or a window.__store_setIndicator seam.
  // Indicator UI + slice logic are unit-tested (IndicatorHUD.test.tsx,
  // createIndicatorSlice.test.ts).
});
