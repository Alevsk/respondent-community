import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_LAYER_ID } from '../helpers/routes';

test.describe('Earth App — layers', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('layers panel opens from toolbar button', async ({ page }) => {
    await page.getByTestId('toolbar-btn-layers').click();
    await expect(page.getByTestId('panel-layers')).toBeVisible();
  });

  test('layers panel shows content', async ({ page }) => {
    await page.getByTestId('toolbar-btn-layers').click();
    const panel = page.getByTestId('panel-layers');
    await expect(panel).toBeVisible();
    await expect(panel).not.toBeEmpty();
  });

  test('layers panel closes via close button', async ({ page }) => {
    await page.getByTestId('toolbar-btn-layers').click();
    const panel = page.getByTestId('panel-layers');
    await expect(panel).toBeVisible();
    await panel.getByTestId('config-panel-close').click();
    await expect(panel).not.toBeVisible();
  });
});

/**
 * Layer interaction tests — deterministic with setupMockRoutes.
 *
 * The stub advertises DEFAULT_LAYER_ID ('flights_commercial') via GET /v1/layers.
 * The layer row testid is filter-option-{layerId} (FilterOption uses the `id` prop,
 * which DataLayersPanel sets to layer.id). Initial enabledLayers = [] (store starts
 * empty regardless of the API's enabled field), so data-active starts "false".
 *
 * PUT /v1/layers/:id is stubbed to 200 so the mutation doesn't error out.
 *
 * toggle-layers-visibility is disabled when enabledCount === 0 and not hidden.
 * clear-all-layers is disabled when enabledCount === 0.
 */
test.describe('Earth App — layers interactions (deterministic)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);

    // Stub the PUT layer endpoint used by useToggleLayer mutation.
    await page.route(`**/v1/layers/${DEFAULT_LAYER_ID}`, (route) => {
      if (route.request().method() === 'PUT') {
        route.fulfill({ json: { id: DEFAULT_LAYER_ID, enabled: true } });
      } else {
        route.continue();
      }
    });

    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();

    // Open the layers panel.
    await page.getByTestId('toolbar-btn-layers').click();
    await expect(page.getByTestId('panel-layers')).toBeVisible();
  });

  test('layer row starts inactive and toggles to active on click', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    await expect(layerRow).toHaveAttribute('data-active', 'false');

    await layerRow.click();
    await expect(layerRow).toHaveAttribute('data-active', 'true');
  });

  test('layer row toggles back to inactive on second click', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    await layerRow.click();
    await expect(layerRow).toHaveAttribute('data-active', 'true');

    await layerRow.click();
    await expect(layerRow).toHaveAttribute('data-active', 'false');
  });

  test('keyboard Enter on layer row activates it', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    await expect(layerRow).toHaveAttribute('data-active', 'false');

    await layerRow.focus();
    // Use locator.press() so keydown fires on the element directly rather than
    // routing through page-level handling (matches the Space test; page.keyboard
    // was focus-race-prone under parallel workers).
    await layerRow.press('Enter');
    await expect(layerRow).toHaveAttribute('data-active', 'true');
  });

  test('keyboard Space on layer row activates it', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    await expect(layerRow).toHaveAttribute('data-active', 'false');

    await layerRow.focus();
    // Use locator.press() so keydown fires on the element directly rather than
    // routing through page-level scroll handling.
    await layerRow.press(' ');
    await expect(layerRow).toHaveAttribute('data-active', 'true');
  });

  test('toggle-layers-visibility is disabled with no enabled layers', async ({ page }) => {
    // Before enabling any layer, enabledCount=0 and not hidden → button disabled.
    await expect(page.getByTestId('toggle-layers-visibility')).toBeDisabled();
  });

  test('toggle-layers-visibility hides enabled layers and restores them', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    // Enable the layer first.
    await layerRow.click();
    await expect(layerRow).toHaveAttribute('data-active', 'true');

    // Now the visibility button should be enabled.
    const visBtn = page.getByTestId('toggle-layers-visibility');
    await expect(visBtn).toBeEnabled();

    // Hide: clicking toggleLayersVisibility stashes enabledLayers → row becomes inactive.
    await visBtn.click();
    await expect(layerRow).toHaveAttribute('data-active', 'false');

    // Restore: clicking again moves stashedLayers back → row becomes active again.
    await visBtn.click();
    await expect(layerRow).toHaveAttribute('data-active', 'true');
  });

  test('clear-all-layers is disabled with no enabled layers', async ({ page }) => {
    await expect(page.getByTestId('clear-all-layers')).toBeDisabled();
  });

  test('clear-all-layers clears enabled layers and becomes disabled', async ({ page }) => {
    const layerRow = page.getByTestId(`filter-option-${DEFAULT_LAYER_ID}`);
    // Enable the layer.
    await layerRow.click();
    await expect(layerRow).toHaveAttribute('data-active', 'true');

    const clearBtn = page.getByTestId('clear-all-layers');
    await expect(clearBtn).toBeEnabled();

    // Clear: button fires clearAllLayers → enabledLayers=[] → row inactive, button disabled.
    await clearBtn.click();
    await expect(layerRow).toHaveAttribute('data-active', 'false');
    await expect(clearBtn).toBeDisabled();
  });
});
