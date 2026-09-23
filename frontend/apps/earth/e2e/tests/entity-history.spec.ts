/**
 * Entity Timeline/History tab e2e spec — deterministic, no live feed.
 *
 * setupMockRoutes (BEFORE goto) stubs the boot endpoints AND the observation
 * history (GET /v1/entities/observations) so the Timeline tab renders real
 * trajectory/table/chart content instead of an empty state. The advertised
 * layer is marked `movingEntities` (displayConfig.icon.interpolation = true) so
 * HistoryTab takes the moving-entity branch — it renders the Trajectory view and
 * the Trails/Isolate switches (stationary entities omit both).
 *
 * Source audit (2026-06-21) — confirmed against the real components:
 *   entity-tab-timeline        : the Timeline tab box (EntityDetailPanel.tsx:87).
 *   Trajectory/Table/Chart     : aria-labelled IconButtons in TimelineTab
 *                                ("Trajectory view"/"Table view"/"Chart view",
 *                                HistoryTab.tsx:240-271). Trajectory only renders
 *                                for moving entities.
 *   timeline-event-list        : Trajectory body (TimelineEventList.tsx:126).
 *   observation-table          : Table body (ObservationTable.tsx:211).
 *   "Altitude (m)"             : Chart field pill — unique to the Chart body
 *                                (ObservationChart.tsx:48), present because the
 *                                stub observations carry non-zero altitude.
 *   history-switch-trails      : Trails Switch (HistoryTab.tsx:201, default on).
 *   history-switch-isolate     : Isolate Switch (HistoryTab.tsx:215, default on).
 *   load-more-observations     : pagination button, rendered when trail.hasMore
 *                                (TimelineEventList.tsx:288 / ObservationDataView.tsx:151).
 *   history-copy-csv|json|md   : copy menu items (HistoryTab.tsx:312-326). The
 *                                copy trigger ("Copy data") only renders in
 *                                table/chart view with >0 points. Clipboard
 *                                content is unit-tested (Task 15) — here we only
 *                                assert the menu OPENS.
 */

import { test, expect } from '@playwright/test';
import {
  setupMockRoutes,
  DEFAULT_LAYER_ID,
  type StubEntity,
  type StubObservation,
} from '../helpers/routes';
import { selectEntity } from '../helpers/selection';

// A moving entity with a small, paginated observation history. Observations are
// newest-first (the wire order the real server uses). Three rows with a page
// size of two make `load-more-observations` reachable: page 1 returns 2 rows +
// hasMore, page 2 returns the final row.
const HIST_ENTITY: StubEntity = {
  id: 'flights_commercial:E2EHIST',
  externalId: 'E2EHIST',
  name: 'E2E Flight History',
  layerType: DEFAULT_LAYER_ID,
  metadata: { callsign: 'E2EHST', origin: 'TEST' },
  lat: 40.5,
  lon: -74.5,
};

const BASE_TS = 1_700_000_000_000;
const HIST_OBS: StubObservation[] = [
  { ts: BASE_TS + 2000, lat: 40.7, lon: -74.7, altitudeM: 10200, metadata: { heading: '270' } },
  { ts: BASE_TS + 1000, lat: 40.6, lon: -74.6, altitudeM: 10100, metadata: { heading: '268' } },
  { ts: BASE_TS, lat: 40.5, lon: -74.5, altitudeM: 10000, metadata: { heading: '265' } },
];

async function openTimeline(page: import('@playwright/test').Page) {
  await selectEntity(page, HIST_ENTITY.id, DEFAULT_LAYER_ID);
  const panel = page.getByTestId('panel-entity-detail');
  await expect(panel).toBeVisible();
  await panel.getByTestId('entity-tab-timeline').click();
  await expect(panel.getByTestId('entity-tab-timeline')).toHaveAttribute('data-active', 'true');
  return panel;
}

test.describe('Entity Timeline tab (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    // Routes MUST be registered before navigation so boot fetches hit stubs.
    await setupMockRoutes(page, {
      movingEntities: true,
      entities: [HIST_ENTITY],
      observations: { [HIST_ENTITY.id]: HIST_OBS },
      observationsPageSize: 2,
    });
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('switching Trajectory / Table / Chart changes the view body', async ({ page }) => {
    const panel = await openTimeline(page);
    const content = panel.getByTestId('config-panel-content');

    // Moving entity → Trajectory is the default view; the event list renders.
    await expect(content.getByTestId('timeline-event-list')).toBeVisible();
    await expect(content.getByTestId('observation-table')).toHaveCount(0);

    // Table view: the table renders, the trajectory list is gone.
    await content.getByLabel('Table view').click();
    await expect(content.getByTestId('observation-table')).toBeVisible();
    await expect(content.getByTestId('timeline-event-list')).toHaveCount(0);

    // Chart view: the "Altitude (m)" field pill (chart-only) appears, table gone.
    await content.getByLabel('Chart view').click();
    await expect(content.getByText('Altitude (m)', { exact: true })).toBeVisible();
    await expect(content.getByTestId('observation-table')).toHaveCount(0);

    // Back to Trajectory: the event list returns.
    await content.getByLabel('Trajectory view').click();
    await expect(content.getByTestId('timeline-event-list')).toBeVisible();
  });

  test('toggling Trails and Isolate flips the switch state', async ({ page }) => {
    const panel = await openTimeline(page);

    // Both switches default to checked (HistoryTab seeds showTrails/isolate true).
    const trails = panel.getByTestId('history-switch-trails');
    const isolate = panel.getByTestId('history-switch-isolate');
    await expect(trails).toBeChecked();
    await expect(isolate).toBeChecked();

    // Toggling each flips its observable checked state.
    await trails.click();
    await expect(trails).not.toBeChecked();

    await isolate.click();
    await expect(isolate).not.toBeChecked();

    // Toggling back restores the on state.
    await trails.click();
    await expect(trails).toBeChecked();
  });

  test('Load More fetches the next page of observations', async ({ page }) => {
    const panel = await openTimeline(page);
    const content = panel.getByTestId('config-panel-content');

    // Table view exposes counted rows. Page 1 returns 2 of 3 observations.
    await content.getByLabel('Table view').click();
    const rows = content.getByTestId('observation-table').locator('tbody tr');
    await expect(rows).toHaveCount(2);

    // The pagination button is present because a third observation remains.
    const loadMore = content.getByTestId('load-more-observations');
    await expect(loadMore).toBeVisible();

    // Loading the next page appends the final row and exhausts the history.
    await loadMore.click();
    await expect(rows).toHaveCount(3);
    await expect(content.getByTestId('load-more-observations')).toHaveCount(0);
  });

  test('the Copy menu opens from Table view (content asserted in unit tests)', async ({ page }) => {
    const panel = await openTimeline(page);
    const content = panel.getByTestId('config-panel-content');

    // The Copy trigger only renders in table/chart view with observations.
    await content.getByLabel('Table view').click();
    await content.getByLabel('Copy data').click();

    // The menu opens — assert the three format items are present. (Clipboard
    // payload is covered by unit tests in Task 15, not here.)
    await expect(page.getByTestId('history-copy-csv')).toBeVisible();
    await expect(page.getByTestId('history-copy-json')).toBeVisible();
    await expect(page.getByTestId('history-copy-md')).toBeVisible();
  });
});
