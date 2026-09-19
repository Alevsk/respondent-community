/**
 * Globe interaction e2e spec — deterministic, no live feed, NO canvas pixel clicks.
 *
 * Cesium picking (viewer.scene.pick on a billboard) is not testid-addressable,
 * so the global e2e constraint drives selection through the store hooks exposed
 * in main.tsx, which are the same store actions the canvas handlers call:
 *   - single-click select  → window.__store_setSelected (setSelectedEntity)
 *   - empty-click clear     → window.__store_setSelected(null, null)
 *   - double-click view     → window.__store_enterEntityView (setSelectedEntity +
 *                             setViewMode('entity')), mirroring the LEFT_DOUBLE_CLICK
 *                             handler (useEntityInteraction.ts:116-129).
 *
 * These exercise the DOM-observable consequences (panel mount/unmount, the
 * exit-view affordance, viewMode in the store snapshot) without faking React
 * internals. The exit-view flow here resolves the T7 deferral in
 * entity-panel.spec.ts (its placeholder test.fixme was removed).
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_LAYER_ID, DEFAULT_ENTITIES } from '../helpers/routes';
import {
  selectEntity,
  clearSelection,
  enterEntityView,
  getSelectedEntityId,
  getViewMode,
} from '../helpers/selection';

const STUB = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001 / "E2E Flight Alpha"

test.describe('Globe interaction (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    // Routes MUST be registered before navigation so the boot fetches hit stubs.
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('single-click selects an entity and opens the detail panel', async ({ page }) => {
    // DOM-observable proxy for a billboard single-click.
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);

    await expect.poll(() => getSelectedEntityId(page)).toBe(STUB.id);
    await expect(page.getByTestId('panel-entity-detail')).toBeVisible();
    // A single click stays in globe view (no entity-tracking transition).
    await expect.poll(() => getViewMode(page)).toBe('globe');
  });

  test('empty-click clears the selection and closes the panel', async ({ page }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);
    await expect(page.getByTestId('panel-entity-detail')).toBeVisible();

    // Empty-click proxy: clear selection (no entity, no layer).
    await clearSelection(page);

    await expect.poll(() => getSelectedEntityId(page)).toBeNull();
    await expect(page.getByTestId('panel-entity-detail')).not.toBeVisible();
  });

  test('double-click enters entity-view and shows the exit affordance', async ({ page }) => {
    // DOM-observable proxy for a billboard double-click (frame + track).
    await enterEntityView(page, STUB.id, DEFAULT_LAYER_ID);

    await expect.poll(() => getSelectedEntityId(page)).toBe(STUB.id);
    await expect.poll(() => getViewMode(page)).toBe('entity');

    // The "Exit Entity View" button only mounts when viewMode === 'entity'.
    const panel = page.getByTestId('panel-entity-detail');
    await expect(panel).toBeVisible();
    await expect(panel.getByTestId('entity-exit-view')).toBeVisible();
  });

  test('exit-view returns viewMode to globe and removes the button (resolves T7 deferral)', async ({
    page,
  }) => {
    await enterEntityView(page, STUB.id, DEFAULT_LAYER_ID);
    await expect.poll(() => getViewMode(page)).toBe('entity');

    const panel = page.getByTestId('panel-entity-detail');
    const exitButton = panel.getByTestId('entity-exit-view');
    await expect(exitButton).toBeVisible();

    // Clicking exit clears the selection, which returns viewMode to 'globe'
    // (clearSelection resets viewMode) and unmounts the button.
    await exitButton.click();

    await expect.poll(() => getViewMode(page)).toBe('globe');
    await expect(page.getByTestId('entity-exit-view')).toHaveCount(0);
  });
});
