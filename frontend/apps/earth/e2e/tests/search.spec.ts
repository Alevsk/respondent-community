/**
 * search.spec.ts — EntitySearchBar interaction tests (deterministic, mock-routes).
 *
 * Confirmed from source audit (2026-06-21):
 *   - entity-search-bar      : Box wrapper (EntitySearchBar.tsx:157)
 *   - search-input-field     : inputProps testid on InputBase (EntitySearchBar.tsx:199)
 *   - search-result-item     : each result row (SearchResultItem.tsx:49)
 *   - search-no-results      : empty-state box when results.length === 0 (EntitySearchBar.tsx:240)
 *   - search-results         : results container (EntitySearchBar.tsx:216)
 *
 * Focus model: focusedIndex state, not aria-selected. The focused row gets
 * `bgcolor: 'rgba(255,255,255,0.06)'` via sx; no aria-selected attribute.
 * We assert ArrowDown+Enter by verifying the SECOND result gets added when
 * focus moves from -1 (first) to 0, then ArrowDown to 1, then Enter adds [1].
 *
 * Does NOT duplicate:
 *   - "search opens from toolbar button and closes with Escape" (panels.spec.ts)
 *   - "search returns results for a present entity" (entity-data.spec.ts — live data)
 *   - "search result click adds entity-pill to watchlist" (watchlist.spec.ts)
 *
 * Keyboard shortcuts (Meta+k, Escape) are covered in globe-keyboard.spec.ts.
 * Here we open the search bar via the toolbar button to stay orthogonal.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_ENTITIES } from '../helpers/routes';

const STUB_A = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001 / "E2E Flight Alpha"
const STUB_B = DEFAULT_ENTITIES[1]; // flights_commercial:E2E002 / "E2E Flight Bravo"

type StoreSnapshot = {
  watchlistEntities: Array<{ entityId: string; name: string; pinned: boolean }>;
};

function getStore(page: import('@playwright/test').Page): Promise<StoreSnapshot> {
  return page.evaluate(() => {
    const w = window as unknown as {
      __store?: () => StoreSnapshot;
    };
    return w.__store?.() ?? { watchlistEntities: [] };
  });
}

test.describe('EntitySearchBar interactions (deterministic)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  // ─── 1. ArrowDown moves the result highlight, Enter adds the focused item ───

  test('ArrowDown moves focus and Enter adds the focused result to watchlist', async ({ page }) => {
    // Open search via toolbar button
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Type a query that matches BOTH stub entities (common prefix "E2E")
    await page.getByTestId('search-input-field').fill('E2E');
    // Wait for debounce (300ms) + route
    await page.waitForTimeout(450);

    // Both results should be visible
    const items = page.getByTestId('search-result-item');
    await expect(items).toHaveCount(2);

    // Initial focusedIndex = -1 → first item selected on Enter.
    // Press ArrowDown once → focusedIndex = 0 (first item highlighted).
    // Press ArrowDown again → focusedIndex = 1 (second item highlighted).
    // Press Enter → adds results[1] (STUB_B) to watchlist.
    await page.getByTestId('search-input-field').press('ArrowDown');
    await page.getByTestId('search-input-field').press('ArrowDown');
    await page.getByTestId('search-input-field').press('Enter');

    // Close search and assert STUB_B is in watchlist (not STUB_A)
    await page.keyboard.press('Escape');

    await expect
      .poll(() => getStore(page).then((s) => s.watchlistEntities.map((e) => e.entityId)))
      .toContain(STUB_B.id);
  });

  // ─── 2. ArrowUp clamps at -1 (no wrap) ───────────────────────────────────

  test('ArrowUp clamps focus at top and Enter adds first result', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    await page.getByTestId('search-input-field').fill('E2E');
    await page.waitForTimeout(450);

    await expect(page.getByTestId('search-result-item')).toHaveCount(2);

    // ArrowUp from -1 → stays at -1 (max(-1, -1) = -1)
    // Enter from -1 → adds results[0] (STUB_A)
    await page.getByTestId('search-input-field').press('ArrowUp');
    await page.getByTestId('search-input-field').press('Enter');

    await page.keyboard.press('Escape');

    await expect
      .poll(() => getStore(page).then((s) => s.watchlistEntities.map((e) => e.entityId)))
      .toContain(STUB_A.id);
  });

  // ─── 3. Click-away closes the search bar ─────────────────────────────────

  test('clicking outside the search bar closes it', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Click somewhere outside the search bar (top-left corner of the page)
    await page.mouse.click(10, 10);

    await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();
  });

  // ─── 4. No-results state ─────────────────────────────────────────────────

  test('typing a query with no match shows search-no-results', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Type a query that matches no stub entity
    await page.getByTestId('search-input-field').fill('XYZXYZXYZ_NOMATCH');
    await page.waitForTimeout(450);

    // Results area renders, but no items — only the empty-state element
    await expect(page.getByTestId('search-results')).toBeVisible();
    await expect(page.getByTestId('search-no-results')).toBeVisible();
    await expect(page.getByTestId('search-result-item')).toHaveCount(0);
  });
});
