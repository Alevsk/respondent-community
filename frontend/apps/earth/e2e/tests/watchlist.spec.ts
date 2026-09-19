/**
 * Watchlist e2e spec — covers the six core watchlist interactions deterministically
 * with no live feed (setupMockRoutes stubs all REST endpoints).
 *
 * Confirmed testids and flow from source audit (2026-06-21):
 *   - entity-pill          : Box wrapping each watchlist chip (EntityPill.tsx:52)
 *   - entity-pill-name     : Typography inside the pill (EntityPill.tsx:93)
 *   - entity-pill-pin      : Pin toggle IconButton (EntityPill.tsx:108)
 *   - entity-pill-remove   : Remove IconButton (EntityPill.tsx:123)
 *   - watchlist-clear-all  : Clear all IconButton (WatchlistBar.tsx:158) — calls clearWatchlist()
 *   - find-mode-toggle     : Visibility-cycle IconButton (WatchlistBar.tsx:174)
 *   - search-input-field   : inputProps testid on InputBase (EntitySearchBar.tsx:199)
 *   - search-result-item   : Each result row (SearchResultItem.tsx:49) — click adds to watchlist
 *
 * Flow: EntitySearchBar.handleAdd always sets pinned:true, so newly added entities
 * are pinned. find-mode-toggle requires at least one pinned entity to activate.
 * clearWatchlist (via watchlist-clear-all) clears all entities + resets findMode.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_ENTITIES } from '../helpers/routes';

const STUB_A = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001 / "E2E Flight Alpha"
const STUB_B = DEFAULT_ENTITIES[1]; // flights_commercial:E2E002 / "E2E Flight Bravo"

/**
 * Helper: add an entity to the watchlist via search UI and close the search overlay.
 * Returns when the entity-pill is visible in the watchlist.
 */
async function addToWatchlistViaSearch(
  page: import('@playwright/test').Page,
  query: string,
  entityName: string,
) {
  await page.getByTestId('toolbar-btn-search').click();
  await expect(page.getByTestId('entity-search-bar')).toBeVisible();
  await page.getByTestId('search-input-field').fill(query);
  // Wait for debounce (300ms) + stub response
  await page.waitForTimeout(400);

  // Click the matching result to add to watchlist
  const items = page.getByTestId('search-result-item');
  await expect(items.first()).toBeVisible();
  await items.first().click();

  // Close search with Escape
  await page.keyboard.press('Escape');
  await expect(page.getByTestId('entity-search-bar')).not.toBeVisible();

  // Assert the pill appeared
  await expect(page.getByTestId('entity-pill').filter({ hasText: entityName })).toBeVisible();
}

test.describe('Watchlist interactions (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  // ─── 1. Add entity via search ─────────────────────────────────────────────

  test('search result click adds entity-pill to watchlist', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();

    // Type a prefix that matches STUB_A name
    await page.getByTestId('search-input-field').fill('E2E Flight Alpha');
    await page.waitForTimeout(400);

    const items = page.getByTestId('search-result-item');
    await expect(items.first()).toBeVisible();

    // Click the result — should add it to the watchlist
    await items.first().click();

    // Close search
    await page.keyboard.press('Escape');

    // Assert pill is in the watchlist
    await expect(page.getByTestId('entity-pill')).toBeVisible();
    await expect(page.getByTestId('entity-pill-name').first()).toHaveText(STUB_A.name);
  });

  // ─── 2. Click entity-pill selects the entity ──────────────────────────────

  test('clicking entity-pill sets selectedEntityId in store', async ({ page }) => {
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);

    const pill = page.getByTestId('entity-pill').filter({ hasText: STUB_A.name });
    await pill.click();

    // Assert store selection updated
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { selectedEntityId: string | null } };
          return w.__store?.().selectedEntityId ?? null;
        }),
      )
      .toBe(STUB_A.id);
  });

  // ─── 3. entity-pill-pin toggles pinned state ──────────────────────────────

  test('entity-pill-pin unpin toggles the entity to unpinned', async ({ page }) => {
    // entity is added pinned:true by default (EntitySearchBar.handleAdd)
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);

    // Verify currently pinned
    const isPinnedBefore = await page.evaluate((id: string) => {
      const w = window as unknown as {
        __store?: () => { watchlistEntities: Array<{ entityId: string; pinned: boolean }> };
      };
      return w.__store?.().watchlistEntities.find((e) => e.entityId === id)?.pinned ?? null;
    }, STUB_A.id);
    expect(isPinnedBefore).toBe(true);

    // Click the pin button — should unpin
    await page
      .getByTestId('entity-pill')
      .filter({ hasText: STUB_A.name })
      .getByTestId('entity-pill-pin')
      .click();

    await expect
      .poll(() =>
        page.evaluate((id: string) => {
          const w = window as unknown as {
            __store?: () => { watchlistEntities: Array<{ entityId: string; pinned: boolean }> };
          };
          return w.__store?.().watchlistEntities.find((e) => e.entityId === id)?.pinned ?? null;
        }, STUB_A.id),
      )
      .toBe(false);
  });

  test('entity-pill-pin re-pins an unpinned entity', async ({ page }) => {
    // Add via search (pinned:true by default), then unpin via pin button
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);
    const pinBtn = page
      .getByTestId('entity-pill')
      .filter({ hasText: STUB_A.name })
      .getByTestId('entity-pill-pin');

    // First click: unpin
    await pinBtn.click();
    await expect
      .poll(() =>
        page.evaluate((id: string) => {
          const w = window as unknown as {
            __store?: () => { watchlistEntities: Array<{ entityId: string; pinned: boolean }> };
          };
          return w.__store?.().watchlistEntities.find((e) => e.entityId === id)?.pinned;
        }, STUB_A.id),
      )
      .toBe(false);

    // Second click: re-pin
    await pinBtn.click();
    await expect
      .poll(() =>
        page.evaluate((id: string) => {
          const w = window as unknown as {
            __store?: () => { watchlistEntities: Array<{ entityId: string; pinned: boolean }> };
          };
          return w.__store?.().watchlistEntities.find((e) => e.entityId === id)?.pinned;
        }, STUB_A.id),
      )
      .toBe(true);
  });

  // ─── 4. entity-pill-remove removes the pill ───────────────────────────────

  test('entity-pill-remove removes the pill from watchlist', async ({ page }) => {
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);

    const pill = page.getByTestId('entity-pill').filter({ hasText: STUB_A.name });
    await expect(pill).toBeVisible();

    await pill.getByTestId('entity-pill-remove').click();

    // Pill must disappear from DOM
    await expect(pill).not.toBeVisible();

    // Store must reflect removal
    await expect
      .poll(() =>
        page.evaluate((id: string) => {
          const w = window as unknown as {
            __store?: () => { watchlistEntities: Array<{ entityId: string }> };
          };
          return w.__store?.().watchlistEntities.some((e) => e.entityId === id) ?? false;
        }, STUB_A.id),
      )
      .toBe(false);
  });

  // ─── 5. watchlist-clear-all clears all entities ───────────────────────────

  test('watchlist-clear-all removes all entities from watchlist', async ({ page }) => {
    // Add two entities
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);
    await addToWatchlistViaSearch(page, 'E2E Flight Bravo', STUB_B.name);

    await expect(page.getByTestId('entity-pill')).toHaveCount(2);

    await page.getByTestId('watchlist-clear-all').click();

    // All pills gone
    await expect(page.getByTestId('entity-pill')).toHaveCount(0);

    // Store watchlistEntities empty
    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as {
            __store?: () => { watchlistEntities: unknown[] };
          };
          return w.__store?.().watchlistEntities.length ?? -1;
        }),
      )
      .toBe(0);
  });

  // ─── 6. find-mode-toggle cycles find mode ────────────────────────────────

  test('find-mode-toggle activates findMode (requires pinned entity)', async ({ page }) => {
    // Add an entity — it is pinned:true by default from handleAdd
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);

    // Confirm findMode starts false
    const before = await page.evaluate(() => {
      const w = window as unknown as { __store?: () => { findMode: boolean } };
      return w.__store?.().findMode ?? null;
    });
    expect(before).toBe(false);

    // Click find-mode-toggle — cycle 1: show all → dim non-pinned (findMode becomes true)
    await page.getByTestId('find-mode-toggle').click();

    await expect
      .poll(() =>
        page.evaluate(() => {
          const w = window as unknown as { __store?: () => { findMode: boolean } };
          return w.__store?.().findMode ?? null;
        }),
      )
      .toBe(true);

    // The button aria-label should now reflect the next state in the cycle ("Hide non-pinned entities")
    await expect(page.getByTestId('find-mode-toggle')).toHaveAttribute(
      'aria-label',
      'Hide non-pinned entities',
    );
  });

  test('find-mode-toggle cycles through dim → hide → off states', async ({ page }) => {
    await addToWatchlistViaSearch(page, 'E2E Flight Alpha', STUB_A.name);

    const btn = page.getByTestId('find-mode-toggle');

    // Initial: findMode=false → aria-label "Dim non-pinned entities"
    await expect(btn).toHaveAttribute('aria-label', 'Dim non-pinned entities');

    // Click 1: findMode=true, display=dimmed → aria-label "Hide non-pinned entities"
    await btn.click();
    await expect(btn).toHaveAttribute('aria-label', 'Hide non-pinned entities');

    // Click 2: display=hidden → aria-label "Show all entities"
    await btn.click();
    await expect(btn).toHaveAttribute('aria-label', 'Show all entities');

    // Click 3: findMode=false → aria-label "Dim non-pinned entities"
    await btn.click();
    await expect(btn).toHaveAttribute('aria-label', 'Dim non-pinned entities');

    const findModeOff = await page.evaluate(() => {
      const w = window as unknown as { __store?: () => { findMode: boolean } };
      return w.__store?.().findMode ?? null;
    });
    expect(findModeOff).toBe(false);
  });
});
