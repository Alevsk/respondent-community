/**
 * globe-keyboard.spec.ts — Global keyboard shortcut tests (deterministic, mock-routes).
 *
 * Confirmed from source audit (2026-06-21) of useSearchKeyboard.ts:
 *   - Meta+k / Ctrl+k  : toggles search bar open/closed (capture phase, stopPropagation)
 *   - Meta+f / Ctrl+f  : toggles findMode, prevents browser find (capture phase)
 *   - Escape priority  :
 *       1. If searchOpen  → close search (then return; bubble still reaches
 *          useEntityTracking but its handler only fires when viewMode==='entity')
 *       2. If findMode    → toggle findMode off
 *       3. Otherwise      → no-op from this handler
 *
 * Key constraint (from createWatchlistSlice.ts toggleFindMode):
 *   toggleFindMode() is a NO-OP when findMode=false and no pinned entities exist.
 *   Tests that assert findMode toggles ON must first add a pinned entity to the watchlist.
 *
 * Note: `window.__store()` exposes `findMode` but NOT `searchOpen`.
 * We assert search bar visibility via DOM (entity-search-bar testid) for
 * the search-open/close assertions.
 *
 * This file owns the Cmd/Ctrl+K shortcut test; search.spec.ts opens the bar
 * via the toolbar button to stay orthogonal and non-duplicating.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_ENTITIES, DEFAULT_LAYER_ID } from '../helpers/routes';
import { selectEntity, getSelectedEntityId } from '../helpers/selection';

const STUB_A = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001

/**
 * Add a pinned entity to the watchlist via the search UI so that
 * toggleFindMode() is not a no-op. Returns when the entity-pill is visible.
 */
async function addPinnedEntityViaSearch(page: import('@playwright/test').Page): Promise<void> {
  await page.getByTestId('toolbar-btn-search').click();
  await expect(page.getByTestId('entity-search-bar')).toBeVisible();
  await page.getByTestId('search-input-field').fill('E2E Flight Alpha');
  await page.waitForTimeout(400);
  await page.getByTestId('search-result-item').first().click();
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();
  // EntitySearchBar.handleAdd always sets pinned:true
  await expect(page.getByTestId('entity-pill')).toBeVisible();
}

test.describe('Globe keyboard shortcuts (deterministic)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  // ─── 1. Meta+K (and Ctrl+K) opens the search bar ─────────────────────────

  test('Meta+K opens entity-search-bar', async ({ page }) => {
    // Confirm not already open
    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();

    await page.keyboard.press('Meta+k');

    await expect(page.getByTestId('entity-search-bar')).toBeVisible();
  });

  test('Ctrl+K opens entity-search-bar', async ({ page }) => {
    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();

    await page.keyboard.press('Control+k');

    await expect(page.getByTestId('entity-search-bar')).toBeVisible();
  });

  // ─── 2. Meta+K toggles search closed ─────────────────────────────────────

  test('Meta+K toggles search bar open then closed', async ({ page }) => {
    // Open
    await page.keyboard.press('Meta+k');
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Close — second press toggles it shut
    await page.keyboard.press('Meta+k');
    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();
  });

  // ─── 3. Meta+F toggles findMode ──────────────────────────────────────────
  //
  // Prerequisite: toggleFindMode is a no-op when watchlist has no pinned
  // entities (createWatchlistSlice.ts:101-103). We add one first.

  test('Meta+F toggles findMode on when a pinned entity exists', async ({ page }) => {
    // Add pinned entity so toggleFindMode is not a no-op
    await addPinnedEntityViaSearch(page);

    const before = await page.evaluate(() => {
      const w = window as unknown as { __store?: () => { findMode: boolean } };
      return w.__store?.().findMode ?? null;
    });
    expect(before).toBe(false);

    // Press Meta+F — useSearchKeyboard calls preventDefault so browser find
    // should not open; no assertion on browser dialog needed (suppressed).
    await page.keyboard.press('Meta+f');

    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);
  });

  test('Ctrl+F toggles findMode on when a pinned entity exists', async ({ page }) => {
    // Parity with Meta+F: the handler uses (e.metaKey || e.ctrlKey), so Ctrl+F
    // drives the same toggle on non-mac keyboards.
    await addPinnedEntityViaSearch(page);
    await page.keyboard.press('Control+f');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);
  });

  test('Meta+F toggles findMode off on second press', async ({ page }) => {
    // Add pinned entity so toggleFindMode can activate
    await addPinnedEntityViaSearch(page);

    // Toggle on
    await page.keyboard.press('Meta+f');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);

    // Toggle off — no pinned-entity guard when turning off
    await page.keyboard.press('Meta+f');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(false);
  });

  // ─── 4. Escape priority: search first, then findMode ─────────────────────

  test('Escape closes search bar first when both search and findMode are active', async ({
    page,
  }) => {
    // Need pinned entity for findMode to activate
    await addPinnedEntityViaSearch(page);

    // Now activate findMode via keyboard
    await page.keyboard.press('Meta+f');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);

    // Open search (Meta+k)
    await page.keyboard.press('Meta+k');
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Escape — should close search first, leave findMode active
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();

    // findMode must still be true (search-close intercepted this Escape)
    const findModeAfterFirst = await page.evaluate(() => {
      const w = window as unknown as { __store?: () => { findMode: boolean } };
      return w.__store?.().findMode ?? null;
    });
    expect(findModeAfterFirst).toBe(true);

    // Second Escape — search is closed, so this time it exits findMode
    await page.keyboard.press('Escape');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(false);
  });

  test('Escape exits findMode when search is already closed', async ({ page }) => {
    // Need pinned entity for findMode to activate
    await addPinnedEntityViaSearch(page);

    // Activate findMode without opening search
    await page.keyboard.press('Meta+f');
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);

    await page.keyboard.press('Escape');

    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(false);
  });

  // ─── 5. Escape deselects entity when viewMode is not 'entity' ────────────
  //
  // NOTE: selectEntity() calls setSelectedEntity() which does NOT set
  // viewMode → 'entity'. The useEntityTracking Escape handler only fires when
  // viewMode === 'entity', so Escape does NOT deselect via that handler here.
  // We assert selectedEntityId persists after Escape (documents the real
  // behavior; the deselect path requires the Cesium camera-tracking loop
  // to enter entity mode, which is not driven in e2e without a globe click).

  test('Escape with search closed and findMode off does not clear selectedEntityId', async ({
    page,
  }) => {
    await selectEntity(page, STUB_A.id, DEFAULT_LAYER_ID);

    const beforeId = await getSelectedEntityId(page);
    expect(beforeId).toBe(STUB_A.id);

    // Neither search nor findMode active — Escape is a no-op from useSearchKeyboard
    await page.keyboard.press('Escape');

    // selectedEntityId unchanged (viewMode is not 'entity', so clearSelection not triggered)
    const afterId = await getSelectedEntityId(page);
    expect(afterId).toBe(STUB_A.id);
  });
});
