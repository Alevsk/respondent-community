/**
 * Mobile surface e2e spec — runs ONLY under the `mobile-chrome` project (Pixel 5
 * emulation; see playwright.config.ts testMatch/testIgnore). The mobile shell
 * (MobileBottomNav, MobileDrawerPanels, MobileEntityPanel) renders only at a
 * mobile viewport (useResponsive → isMobile), so these interactions are not
 * reachable from the desktop project.
 *
 * Deterministic: setupMockRoutes stubs the REST feed before goto; no live data.
 *
 * Observables (confirmed against components):
 *   - mobile-bottom-nav holds five aria-labelled buttons: Search, Layers,
 *     Settings, Nav, Record (MobileBottomNav.tsx). Layers/Settings/Nav open a
 *     MobileDrawer wrapping the same panel-{layers,settings,navigation} testids;
 *     Search toggles entity-search-bar; Record toggles recording mode (which
 *     unmounts the bottom nav and renders aspect-ratio-picker).
 *   - selecting an entity mounts mobile-entity-panel (MobileEntityPanel.tsx).
 */
import { test, expect, type Page } from '@playwright/test';
import { setupMockRoutes, DEFAULT_ENTITIES, DEFAULT_LAYER_ID } from '../helpers/routes';
import { selectEntity, getSelectedEntityId, openCluster } from '../helpers/selection';

const STUB = DEFAULT_ENTITIES[0];

/** A bottom-nav button by its exact accessible (aria-)label. */
function navButton(page: Page, name: string) {
  return page.getByTestId('mobile-bottom-nav').getByRole('button', { name, exact: true });
}

test.describe('Earth App — mobile (Pixel 5)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('mobile-bottom-nav')).toBeVisible();
  });

  test('bottom-nav Layers opens and closes the layers drawer', async ({ page }) => {
    await navButton(page, 'Layers').click();
    await expect(page.getByTestId('mobile-drawer-data-layers')).toBeVisible();
    // The open drawer's backdrop covers the bottom nav; Escape closes the MUI drawer.
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('mobile-drawer-data-layers')).not.toBeVisible();
  });

  test('bottom-nav Settings opens and closes the settings drawer', async ({ page }) => {
    await navButton(page, 'Settings').click();
    await expect(page.getByTestId('mobile-drawer-settings')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('mobile-drawer-settings')).not.toBeVisible();
  });

  test('bottom-nav Nav opens and closes the navigation drawer', async ({ page }) => {
    await navButton(page, 'Nav').click();
    await expect(page.getByTestId('mobile-drawer-navigation')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('mobile-drawer-navigation')).not.toBeVisible();
  });

  test('bottom-nav Search opens the search bar and Escape closes it', async ({ page }) => {
    await navButton(page, 'Search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();
  });

  test('selecting an entity mounts the mobile entity panel', async ({ page }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);
    await expect.poll(() => getSelectedEntityId(page)).toBe(STUB.id);
    await expect(page.getByTestId('mobile-entity-panel').first()).toBeVisible();
  });

  test('opening a cluster shows the cluster drawer; selecting a member selects it', async ({
    page,
  }) => {
    // The cluster list uses the SHARED MobileDrawer, driven directly by
    // activeCluster with disableBackdropClose (a canvas-gesture-opened Modal is
    // otherwise closed by the touch ghost-click on its backdrop).
    await openCluster(page, {
      clusterId: 'cluster-3:3',
      layerId: DEFAULT_LAYER_ID,
      entityIds: DEFAULT_ENTITIES.map((e) => e.id),
      count: DEFAULT_ENTITIES.length,
      truncated: false,
      screenX: 100,
      screenY: 100,
    });

    await expect(page.getByTestId('mobile-drawer-cluster')).toBeVisible();
    const rows = page.getByTestId('cluster-member-row');
    await expect(rows).toHaveCount(DEFAULT_ENTITIES.length);

    await rows.first().click();
    await expect.poll(() => getSelectedEntityId(page)).toBe(DEFAULT_ENTITIES[0].id);
    await expect(page.getByTestId('mobile-drawer-cluster')).not.toBeVisible();
  });

  test('the cluster drawer closes via its close button', async ({ page }) => {
    await openCluster(page, {
      clusterId: 'cluster-4:4',
      layerId: DEFAULT_LAYER_ID,
      entityIds: DEFAULT_ENTITIES.map((e) => e.id),
      count: DEFAULT_ENTITIES.length,
      truncated: false,
      screenX: 100,
      screenY: 100,
    });
    await expect(page.getByTestId('mobile-drawer-cluster')).toBeVisible();
    await page.getByTestId('mobile-drawer-close-cluster').click();
    await expect(page.getByTestId('mobile-drawer-cluster')).not.toBeVisible();
  });

  test('Record enters recording mode and Exit returns to the bottom nav', async ({ page }) => {
    await navButton(page, 'Record').click();
    // Recording mode unmounts the bottom nav and shows the aspect-ratio picker.
    await expect(page.getByTestId('aspect-ratio-picker')).toBeVisible();
    await expect(page.getByTestId('mobile-bottom-nav')).toHaveCount(0);

    await page.getByRole('button', { name: 'Exit recording mode' }).click();
    await expect(page.getByTestId('aspect-ratio-picker')).toHaveCount(0);
    await expect(page.getByTestId('mobile-bottom-nav')).toBeVisible();
  });

  test('recording: selecting an aspect ratio activates it and deactivates the previous', async ({
    page,
  }) => {
    await navButton(page, 'Record').click();
    await expect(page.getByTestId('aspect-ratio-picker')).toBeVisible();

    const r169 = page.getByTestId('ratio-16-9');
    await r169.scrollIntoViewIfNeeded();
    await r169.click();
    await expect(r169).toHaveAttribute('data-active', 'true');

    const r11 = page.getByTestId('ratio-1-1');
    await r11.scrollIntoViewIfNeeded();
    await r11.click();
    await expect(r11).toHaveAttribute('data-active', 'true');
    await expect(r169).toHaveAttribute('data-active', 'false');

    await page.getByRole('button', { name: 'Exit recording mode' }).click();
  });

  test('recording: rule-of-thirds grid toggles the overlay grid on and off', async ({ page }) => {
    await navButton(page, 'Record').click();
    await expect(page.getByTestId('aspect-ratio-picker')).toBeVisible();

    // Grid off by default → no grid overlay element.
    await expect(page.getByTestId('aspect-ratio-grid')).toHaveCount(0);
    const grid = page.getByRole('button', { name: 'Toggle rule of thirds grid' });
    await grid.click();
    await expect(page.getByTestId('aspect-ratio-grid')).toBeVisible();
    await grid.click();
    await expect(page.getByTestId('aspect-ratio-grid')).toHaveCount(0);

    await page.getByRole('button', { name: 'Exit recording mode' }).click();
  });
});
