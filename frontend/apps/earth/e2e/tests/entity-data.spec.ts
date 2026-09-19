import { test, expect, request } from '@playwright/test';
import { waitForEntityData } from '../helpers/data';

const BASE_URL = process.env.E2E_BASE_URL ?? 'http://localhost:8090';

test.describe('Earth App — live entity data', () => {
  let entityCount = 0;

  test.beforeAll(async () => {
    const api = await request.newContext({ baseURL: BASE_URL });
    // Poll within the 30s beforeAll hook budget so that with no live feed (e.g.
    // `make e2e` runs the server with RESPONDENT_INGEST_ENABLED=false) this returns
    // 0 and the tests skip cleanly, rather than the hook timing out and failing.
    entityCount = await waitForEntityData(api, 15_000);
    await api.dispose();
  });

  test.beforeEach(() => {
    test.skip(
      entityCount === 0,
      'no live entity data (feeder offline or no public sources active) — skipping data-dependent assertions',
    );
  });

  test('entities are present in a layer snapshot', async () => {
    expect(entityCount).toBeGreaterThan(0);
  });

  test('search returns results for a present entity', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
    // Open the search panel via the toolbar button.
    await page.getByTestId('toolbar-btn-search').click();
    await expect(page.getByTestId('entity-search-bar')).toBeVisible();
    // `search-input` is the MUI InputBase wrapper; the actual <input> element
    // has `data-testid="search-input-field"` (set via inputProps). Use that for fill().
    // The search hook fires only for queries >= 2 chars; use a broad 2-char prefix.
    await page.getByTestId('search-input-field').fill('si');
    // search-results appears as soon as localQuery.length > 0; wait for a result item.
    await expect(page.getByTestId('search-results')).toBeVisible();
    await expect(page.locator('[data-testid="search-result-item"]').first()).toBeVisible({
      timeout: 15_000,
    });
  });
});
