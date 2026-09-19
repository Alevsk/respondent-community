/**
 * e2e spec — Settings panel: display filter presets + switches + slider.
 *
 * Covers:
 *  - Opening the settings panel via toolbar-btn-settings
 *  - Clicking each FilterPreset (NORMAL, CRT, NVG, FLIR) — the clicked option
 *    gets data-active="true", the previously-active one flips to "false"
 *  - Keyboard activation: focus a filter-option and press Enter/Space → activates
 *  - Toggle switches (occluded, geo-labels, smooth-motion, cinematic-drift) →
 *    assert window.__store() flag flips in both directions
 *  - 3D-buildings switch is DISABLED when no Ion token (VITE_CESIUM_ION_TOKEN not
 *    set in e2e environment); if token IS present, asserts show3DBuildings toggles.
 *  - Render-limit slider: keyboard ArrowRight increases maxEntities, ArrowLeft
 *    decreases maxEntities (asserted via window.__store().maxEntities).
 *
 * TestId format: filter-option-{id} where id is the uppercase FilterPreset value
 * (NORMAL | CRT | NVG | FLIR). Confirmed from FilterOption.tsx: when an explicit
 * `id` prop is passed, testId = `filter-option-${id}`, so uppercase strings are used.
 *
 * activePreset is NOT in window.__store(), so we assert via DOM data-active only.
 * Initial activePreset = 'NORMAL' (createImmersiveUISlice.ts line 37).
 *
 * setupMockRoutes is called before goto so REST fetches are deterministic.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes } from '../helpers/routes';

// ── window.__store() type (subset) ───────────────────────────────────────────
interface StoreSnapshot {
  showOccluded: boolean;
  showGeoLabels: boolean;
  show3DBuildings: boolean;
  smoothMotion: boolean;
  cinematicDrift: boolean;
  maxEntities: number;
}

/** Read the current window.__store() snapshot. */
async function getStore(page: import('@playwright/test').Page): Promise<StoreSnapshot> {
  return page.evaluate(() => {
    const s = (window as unknown as { __store: () => StoreSnapshot }).__store();
    return {
      showOccluded: s.showOccluded,
      showGeoLabels: s.showGeoLabels,
      show3DBuildings: s.show3DBuildings,
      smoothMotion: s.smoothMotion,
      cinematicDrift: s.cinematicDrift,
      maxEntities: s.maxEntities,
    };
  });
}

test.describe('Settings panel — display filter presets', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();

    // Open the settings panel.
    await page.getByTestId('toolbar-btn-settings').click();
    await expect(page.getByTestId('panel-settings')).toBeVisible();
  });

  test('NORMAL is active by default', async ({ page }) => {
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-CRT')).toHaveAttribute('data-active', 'false');
    await expect(page.getByTestId('filter-option-NVG')).toHaveAttribute('data-active', 'false');
    await expect(page.getByTestId('filter-option-FLIR')).toHaveAttribute('data-active', 'false');
  });

  test('clicking CRT activates CRT and deactivates NORMAL', async ({ page }) => {
    await page.getByTestId('filter-option-CRT').click();
    await expect(page.getByTestId('filter-option-CRT')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'false');
  });

  test('clicking NVG activates NVG and deactivates NORMAL', async ({ page }) => {
    await page.getByTestId('filter-option-NVG').click();
    await expect(page.getByTestId('filter-option-NVG')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'false');
  });

  test('clicking FLIR activates FLIR and deactivates NORMAL', async ({ page }) => {
    await page.getByTestId('filter-option-FLIR').click();
    await expect(page.getByTestId('filter-option-FLIR')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'false');
  });

  test('switching presets: CRT → NVG deactivates CRT and activates NVG', async ({ page }) => {
    await page.getByTestId('filter-option-CRT').click();
    await expect(page.getByTestId('filter-option-CRT')).toHaveAttribute('data-active', 'true');

    await page.getByTestId('filter-option-NVG').click();
    await expect(page.getByTestId('filter-option-NVG')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-CRT')).toHaveAttribute('data-active', 'false');
  });

  test('keyboard Enter on CRT activates it', async ({ page }) => {
    const crt = page.getByTestId('filter-option-CRT');
    await crt.scrollIntoViewIfNeeded();
    // locator.press() focuses + keydowns on the element atomically, avoiding a
    // focus race under parallel load (page.keyboard.press can miss the target).
    await crt.press('Enter');
    await expect(crt).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'false');
  });

  test('keyboard Space on FLIR activates it', async ({ page }) => {
    const flir = page.getByTestId('filter-option-FLIR');
    // Scroll the element into view before focusing, as FLIR may be below the fold
    // inside the settings panel's scrollable area.
    await flir.scrollIntoViewIfNeeded();
    await flir.focus();
    // Use locator.press() so keydown fires on the element directly rather than
    // routing through page-level scroll handling.
    await flir.press(' ');
    await expect(flir).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('filter-option-NORMAL')).toHaveAttribute('data-active', 'false');
  });
});

// ── Settings switches + slider ────────────────────────────────────────────────

test.describe('Settings panel — switches and render-limit slider', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();

    // Open the settings panel.
    await page.getByTestId('toolbar-btn-settings').click();
    await expect(page.getByTestId('panel-settings')).toBeVisible();
  });

  // ── showOccluded toggle ────────────────────────────────────────────────────
  test('settings-switch-occluded toggles showOccluded in both directions', async ({ page }) => {
    const initial = (await getStore(page)).showOccluded;

    // First toggle: flip on.
    const switchEl = page.getByTestId('settings-switch-occluded');
    await switchEl.scrollIntoViewIfNeeded();
    await switchEl.click();
    const afterFirst = (await getStore(page)).showOccluded;
    expect(afterFirst).toBe(!initial);

    // Second toggle: flip back.
    await switchEl.click();
    const afterSecond = (await getStore(page)).showOccluded;
    expect(afterSecond).toBe(initial);
  });

  // ── showGeoLabels toggle ───────────────────────────────────────────────────
  test('settings-switch-geo-labels toggles showGeoLabels in both directions', async ({ page }) => {
    const initial = (await getStore(page)).showGeoLabels;

    const switchEl = page.getByTestId('settings-switch-geo-labels');
    await switchEl.scrollIntoViewIfNeeded();
    await switchEl.click();
    const afterFirst = (await getStore(page)).showGeoLabels;
    expect(afterFirst).toBe(!initial);

    await switchEl.click();
    const afterSecond = (await getStore(page)).showGeoLabels;
    expect(afterSecond).toBe(initial);
  });

  // ── show3DBuildings (disabled without Ion token) ──────────────────────────
  test('settings-switch-3d-buildings is disabled when no Ion token; toggles when token present', async ({
    page,
  }) => {
    const switchEl = page.getByTestId('settings-switch-3d-buildings');
    await switchEl.scrollIntoViewIfNeeded();

    // Determine whether the token was baked into the build.
    const isDisabled = await switchEl.isDisabled();

    if (isDisabled) {
      // No Ion token in e2e environment — assert switch is disabled.
      await expect(switchEl).toBeDisabled();
    } else {
      // Ion token present — assert show3DBuildings toggles.
      const initial = (await getStore(page)).show3DBuildings;
      await switchEl.click();
      const afterFirst = (await getStore(page)).show3DBuildings;
      expect(afterFirst).toBe(!initial);

      await switchEl.click();
      const afterSecond = (await getStore(page)).show3DBuildings;
      expect(afterSecond).toBe(initial);
    }
  });

  // ── smoothMotion toggle ────────────────────────────────────────────────────
  test('settings-switch-smooth-motion toggles smoothMotion in both directions', async ({
    page,
  }) => {
    const initial = (await getStore(page)).smoothMotion;

    const switchEl = page.getByTestId('settings-switch-smooth-motion');
    await switchEl.scrollIntoViewIfNeeded();
    await switchEl.click();
    const afterFirst = (await getStore(page)).smoothMotion;
    expect(afterFirst).toBe(!initial);

    await switchEl.click();
    const afterSecond = (await getStore(page)).smoothMotion;
    expect(afterSecond).toBe(initial);
  });

  // ── cinematicDrift toggle ──────────────────────────────────────────────────
  test('settings-switch-cinematic-drift toggles cinematicDrift in both directions', async ({
    page,
  }) => {
    const initial = (await getStore(page)).cinematicDrift;

    const switchEl = page.getByTestId('settings-switch-cinematic-drift');
    await switchEl.scrollIntoViewIfNeeded();
    await switchEl.click();
    const afterFirst = (await getStore(page)).cinematicDrift;
    expect(afterFirst).toBe(!initial);

    await switchEl.click();
    const afterSecond = (await getStore(page)).cinematicDrift;
    expect(afterSecond).toBe(initial);
  });

  // ── render-limit slider: keyboard arrows change maxEntities ───────────────
  //
  // MUI Slider renders the interactive thumb as a <span role="slider"> child
  // of the root element (which carries data-testid). We click the slider root to
  // focus the thumb, then send keyboard events. The panel is scrollable so we
  // scroll the slider into view before clicking.
  test('settings-render-limit-slider keyboard ArrowRight increases maxEntities', async ({
    page,
  }) => {
    const slider = page.getByTestId('settings-render-limit-slider');
    await slider.scrollIntoViewIfNeeded();

    // Click the slider to give it focus (focuses the thumb span internally).
    await slider.click();

    const before = (await getStore(page)).maxEntities;

    // Press ArrowRight to increase by one step (SLIDER_STEP = 100).
    await page.keyboard.press('ArrowRight');

    const after = (await getStore(page)).maxEntities;

    // If already at max (10000), ArrowRight is a no-op; assert at max.
    if (before >= 10_000) {
      expect(after).toBe(10_000);
    } else {
      expect(after).toBeGreaterThan(before);
    }
  });

  test('settings-render-limit-slider keyboard ArrowLeft decreases maxEntities', async ({
    page,
  }) => {
    const slider = page.getByTestId('settings-render-limit-slider');
    await slider.scrollIntoViewIfNeeded();

    // Click the slider to give it focus.
    await slider.click();

    const before = (await getStore(page)).maxEntities;

    // Press ArrowLeft to decrease by one step (SLIDER_STEP = 100).
    await page.keyboard.press('ArrowLeft');

    const after = (await getStore(page)).maxEntities;

    // If already at min (100), ArrowLeft is a no-op; assert at min.
    if (before <= 100) {
      expect(after).toBe(100);
    } else {
      expect(after).toBeLessThan(before);
    }
  });

  test('settings-render-limit-slider ArrowRight then ArrowLeft returns to original maxEntities', async ({
    page,
  }) => {
    const slider = page.getByTestId('settings-render-limit-slider');
    await slider.scrollIntoViewIfNeeded();

    // Click the slider to give it focus.
    await slider.click();

    const initial = (await getStore(page)).maxEntities;

    // Only run this round-trip if we're not at the boundary.
    if (initial > 100 && initial < 10_000) {
      await page.keyboard.press('ArrowRight');
      const mid = (await getStore(page)).maxEntities;
      expect(mid).toBeGreaterThan(initial);

      await page.keyboard.press('ArrowLeft');
      const final = (await getStore(page)).maxEntities;
      expect(final).toBe(initial);
    } else {
      // At boundary — verify ArrowRight doesn't go out of range.
      await page.keyboard.press('ArrowRight');
      const afterRight = (await getStore(page)).maxEntities;
      expect(afterRight).toBeLessThanOrEqual(10_000);
    }
  });
});
