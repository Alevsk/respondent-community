/**
 * e2e infrastructure smoke spec — proves the deterministic helper stack works
 * with NO live feed:
 *
 *   • setupMockRoutes(page) (BEFORE goto) stubs /v1/layers, the snapshot, and
 *     /v1/entities/search → search returns the stub entity for a typed prefix.
 *   • selectEntity(page, id, layer) drives window.__store_setSelected so the
 *     EntityDetailPanel mounts without a canvas click.
 *   • getSelectedEntityId(page) reads it back via window.__store().
 *
 * If this passes, every Phase-1 spec can rely on the stub feed for determinism.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_LAYER_ID, DEFAULT_ENTITIES } from '../helpers/routes';
import { selectEntity, getSelectedEntityId } from '../helpers/selection';

const STUB = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001 / "E2E Flight Alpha"

test.describe('e2e infrastructure (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    // Routes MUST be registered before navigation so the initial fetches hit stubs.
    await setupMockRoutes(page);
    await page.goto('/');
    // Readiness fence — the shell renders from REST + store, no live feed needed.
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('search returns a stub entity for a matching prefix (deterministic)', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Type a prefix that matches a stub entity's name ("E2E Flight Alpha/Bravo").
    await page.getByTestId('search-input-field').fill('E2E');

    const results = page.getByTestId('search-result-item');
    await expect(results.first()).toBeVisible();
    await expect(page.getByTestId('search-result-name').first()).toHaveText(STUB.name);
  });

  test('selectEntity drives store selection and mounts the entity detail panel', async ({
    page,
  }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);

    await expect.poll(() => getSelectedEntityId(page)).toBe(STUB.id);
    await expect(page.getByTestId('panel-entity-detail')).toBeVisible();
  });
});
