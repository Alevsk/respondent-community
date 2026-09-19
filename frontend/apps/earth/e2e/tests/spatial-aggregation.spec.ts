/**
 * Spatial aggregation (clustering) e2e — deterministic, no live feed.
 *
 * Per the suite's global constraint, Cesium cluster-marker picks are not
 * testid-addressable, so the cluster→list interaction is driven through the
 * store proxy exposed in main.tsx (window.__store_setActiveCluster), the same
 * action the real pick handler calls. This spec verifies the user-facing,
 * DOM-observable surface: the off-by-default Settings toggle, the cluster member
 * list, row selection opening entity detail, and the loaded-subset affordances.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_LAYER_ID, DEFAULT_ENTITIES } from '../helpers/routes';
import { openCluster, getActiveClusterId, getSelectedEntityId } from '../helpers/selection';

function readStore(page: import('@playwright/test').Page) {
  return page.evaluate(() => {
    const w = window as unknown as { __store?: () => { spatialAggregation: boolean } };
    return w.__store?.();
  });
}

test.describe('Spatial aggregation (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('Spatial Aggregation toggle is off by default and can be enabled', async ({ page }) => {
    await page.getByTestId('toolbar-btn-settings').click();
    await expect(page.getByTestId('panel-settings')).toBeVisible();

    const toggle = page.getByTestId('settings-switch-spatial-aggregation');
    await expect(toggle).toBeVisible();
    expect((await readStore(page))?.spatialAggregation).toBe(false);

    await toggle.click();
    await expect.poll(async () => (await readStore(page))?.spatialAggregation).toBe(true);
  });

  test('clicking a cluster opens the member list; selecting a row opens entity detail', async ({
    page,
  }) => {
    await openCluster(page, {
      clusterId: 'cluster-1:1',
      layerId: DEFAULT_LAYER_ID,
      entityIds: DEFAULT_ENTITIES.map((e) => e.id),
      count: DEFAULT_ENTITIES.length,
      truncated: false,
      screenX: 220,
      screenY: 220,
    });

    await expect(page.getByTestId('cluster-list-panel')).toBeVisible();
    await expect(page.getByTestId('cluster-member-list')).toBeVisible();
    const rows = page.getByTestId('cluster-member-row');
    await expect(rows).toHaveCount(DEFAULT_ENTITIES.length);

    await rows.first().click();

    await expect(page.getByTestId('panel-entity-detail')).toBeVisible();
    await expect.poll(() => getSelectedEntityId(page)).toBe(DEFAULT_ENTITIES[0].id);
    // Selecting a member closes the cluster list.
    await expect.poll(() => getActiveClusterId(page)).toBeNull();
    await expect(page.getByTestId('cluster-list-panel')).not.toBeVisible();
  });

  test('shows loaded-subset and "+N more" affordances when the cluster is capped', async ({
    page,
  }) => {
    await openCluster(page, {
      clusterId: 'cluster-2:2',
      layerId: DEFAULT_LAYER_ID,
      entityIds: DEFAULT_ENTITIES.map((e) => e.id),
      count: 150, // true bin size exceeds the shown ids
      truncated: true,
      screenX: 240,
      screenY: 240,
    });

    await expect(page.getByTestId('cluster-list-panel')).toBeVisible();
    await expect(page.getByTestId('cluster-truncated-indicator')).toBeVisible();
    await expect(page.getByTestId('cluster-more-indicator')).toContainText('more');

    // Escape closes the panel.
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('cluster-list-panel')).not.toBeVisible();
  });
});
