/**
 * Notifications core e2e spec — covers the central notification interactions
 * deterministically with NO live feed.
 *
 * DATA SOURCE (confirmed by source audit, 2026-06-21):
 *   Notifications are AI insights. They reach the store via two paths:
 *     • REST backfill on mount — useAppBootstrap + useNotificationBackfill call
 *       GET /v1/ai/insights?... → { insights:[...], totalCount } → normalizeInsight
 *       → store.mergeNotifications. (deterministic, fires on load)
 *     • WS `ai_insight` frames — useAppBootstrap subscribes and calls addNotification.
 *   The filter toggle only renders once GET /v1/ai/notifications/filters resolves
 *   (useNotificationFilterOptions). Phase 0 stubbed NEITHER endpoint.
 *
 *   This spec uses the REST path: setupNotificationRoutes(page) (helpers/routes.ts)
 *   stubs both endpoints before goto, so two insights backfill into the store and
 *   render as notification-item-{id}. No live feed or WS push is required.
 *
 * OBSERVABLES (confirmed against the components):
 *   - Header text (NotificationPanel.tsx): "NO NOTIFICATIONS" | "{n} UNREAD" | "ALL READ".
 *   - notif-mark-all-read button renders ONLY while unreadCount > 0; after click it
 *     is removed and the header reads "ALL READ".
 *   - notification-bell badge (.MuiBadge-badge) is invisible when unreadCount === 0.
 *   - notification-filter-toggle renders only when filter options load; clicking it
 *     mounts NotificationFilterPanel → notif-filter-attention-{level} chips + MIN ATTENTION.
 *   - notification-item-{id} click toggles expanded (ENTITIES section + notif-entity-chip-{id})
 *     and marks the item read (NotificationItem.handleClick → markRead).
 */

import { test, expect } from '@playwright/test';
import {
  setupMockRoutes,
  setupNotificationRoutes,
  DEFAULT_INSIGHTS,
  DEFAULT_LAYER_ID,
  toApiInsight,
  type StubInsight,
} from '../helpers/routes';

const INSIGHT_A = DEFAULT_INSIGHTS[0]; // insight-e2e-001 / high  / "E2E Flight Anomaly Alpha"
const INSIGHT_B = DEFAULT_INSIGHTS[1]; // insight-e2e-002 / crit. / "E2E Proximity Alert Bravo"

test.describe('Earth App — notifications (core, deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    // Routes MUST be registered before navigation so the boot fetches hit stubs.
    await setupMockRoutes(page);
    await setupNotificationRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  /** Open the panel and wait for the seeded insights to render. */
  async function openPanelWithInsights(page: import('@playwright/test').Page) {
    await page.getByTestId('notification-bell').click();
    const panel = page.getByTestId('notification-panel');
    await expect(panel).toBeVisible();
    // The two backfilled insights render as addressable items.
    await expect(panel.getByTestId(`notification-item-${INSIGHT_A.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${INSIGHT_B.id}`)).toBeVisible();
    return panel;
  }

  test('bell opens the panel and the stubbed insights render as items', async ({ page }) => {
    // Light re-assert of the bell → panel open (deep-covered in panels.spec),
    // plus proof the data stub deterministically populates the list.
    const panel = await openPanelWithInsights(page);
    // Both seeded titles are visible.
    await expect(panel.getByText('E2E Flight Anomaly Alpha')).toBeVisible();
    await expect(panel.getByText('E2E Proximity Alert Bravo')).toBeVisible();
  });

  test('mark-all-read clears the unread count', async ({ page }) => {
    const panel = await openPanelWithInsights(page);

    // Both insights are unread on load → header shows "{n} UNREAD" and the bell
    // badge is visible with the unread count.
    await expect(panel.getByText('2 UNREAD')).toBeVisible();
    const badge = page.getByTestId('notification-bell').locator('.MuiBadge-badge');
    await expect(badge).toBeVisible();
    await expect(badge).toHaveText('2');

    // Mark all read.
    const markAll = panel.getByTestId('notif-mark-all-read');
    await expect(markAll).toBeVisible();
    await markAll.click();

    // Unread state clears: header flips to ALL READ, the button is gone, the
    // bell badge becomes invisible (unreadCount === 0).
    await expect(panel.getByText('ALL READ')).toBeVisible();
    await expect(panel.getByTestId('notif-mark-all-read')).toHaveCount(0);
    await expect(badge).not.toBeVisible();
  });

  test('filter toggle expands the filter bar', async ({ page }) => {
    const panel = await openPanelWithInsights(page);

    // Filter UI is collapsed initially — the attention chips are not mounted.
    await expect(panel.getByTestId('notif-filter-attention-high')).toHaveCount(0);

    // Toggle the filter bar open.
    await panel.getByTestId('notification-filter-toggle').click();

    // The filter panel mounts: the MIN ATTENTION header and the attention chips
    // (one per stubbed attention level) become visible.
    await expect(panel.getByText('MIN ATTENTION')).toBeVisible();
    await expect(panel.getByTestId('notif-filter-attention-high')).toBeVisible();
    await expect(panel.getByTestId('notif-filter-attention-critical')).toBeVisible();
    await expect(panel.getByTestId('notif-filter-insight-flight_anomaly')).toBeVisible();

    // Collapsing hides them again.
    await panel.getByTestId('notification-filter-toggle').click();
    await expect(panel.getByTestId('notif-filter-attention-high')).toHaveCount(0);
  });

  test('expanding an item marks it read and reveals its expanded content', async ({ page }) => {
    const panel = await openPanelWithInsights(page);

    // Two unread on load.
    await expect(panel.getByText('2 UNREAD')).toBeVisible();

    const itemA = panel.getByTestId(`notification-item-${INSIGHT_A.id}`);
    // Collapsed: the entity chip (only rendered when expanded) is not present.
    const chipId = INSIGHT_A.entities![0].id;
    await expect(panel.getByTestId(`notif-entity-chip-${chipId}`)).toHaveCount(0);

    // Expand item A.
    await itemA.click();

    // Expanded content appears: the ENTITIES section + the entity chip.
    await expect(panel.getByText(/^ENTITIES \(1\)$/)).toBeVisible();
    await expect(panel.getByTestId(`notif-entity-chip-${chipId}`)).toBeVisible();

    // Marking that one item read decrements the unread count (2 → 1).
    await expect(panel.getByText('1 UNREAD')).toBeVisible();
    // The bell badge follows the unread count.
    await expect(page.getByTestId('notification-bell').locator('.MuiBadge-badge')).toHaveText('1');
  });
});

/**
 * Notification filters, items & pagination — deterministic, no live feed.
 *
 * Filtering is SERVER-SIDE: useNotificationBackfill refetches GET /v1/ai/insights
 * with buildFilterParams (min_attention enum when a min-attention filter is active;
 * insight_type when exactly one type is selected; limit/offset for pagination), and
 * clearNotifications runs on every filter change. So these specs register a
 * PARAM-AWARE /v1/ai/insights override (it wins over setupNotificationRoutes' default
 * because it is registered later) that honors those params, making the list visibly
 * respond to filter + load-more interactions.
 */
test.describe('Earth App — notification filters / items / pagination', () => {
  const A_HIGH: StubInsight = {
    id: 'nf-high-001',
    insightType: 'flight_anomaly',
    attention: 'high',
    layerType: DEFAULT_LAYER_ID,
    result: { title: 'NF High Anomaly', description: 'High-attention anomaly.' },
    entities: [
      {
        id: 'flights_commercial:NF1',
        externalId: 'NF1',
        name: 'NF One',
        layerType: DEFAULT_LAYER_ID,
      },
    ],
  };
  const B_CRIT: StubInsight = {
    id: 'nf-crit-002',
    insightType: 'proximity_alert',
    attention: 'critical',
    layerType: DEFAULT_LAYER_ID,
    result: { title: 'NF Critical Proximity', description: 'Critical proximity alert.' },
    entities: [
      {
        id: 'flights_commercial:NF2',
        externalId: 'NF2',
        name: 'NF Two',
        layerType: DEFAULT_LAYER_ID,
      },
    ],
  };

  /**
   * Register a param-aware /v1/ai/insights override + the filter-options route,
   * then open the panel. The override:
   *   - min_attention (enum 1=info..5=critical; default filter sends 3=medium) →
   *     return insights whose attention rank is >= the requested minimum.
   *   - insight_type present  → return only insights of that type.
   *   - limit/offset          → paginate (so load-more is reachable).
   */
  async function openWithFilterableInsights(
    page: import('@playwright/test').Page,
    insights: StubInsight[],
  ) {
    await setupMockRoutes(page);
    await page.route('**/v1/ai/insights*', (route) => {
      const url = new URL(route.request().url());
      const minAttn = url.searchParams.get('min_attention');
      const insightType = url.searchParams.get('insight_type');
      const limit = Number(url.searchParams.get('limit') ?? 20);
      const offset = Number(url.searchParams.get('offset') ?? 0);
      // ATTENTION_ENUM (core constants): info=1 low=2 medium=3 high=4 critical=5.
      const rank: Record<string, number> = { info: 1, low: 2, medium: 3, high: 4, critical: 5 };
      const minNum = minAttn ? Number(minAttn) : 0;
      let set = insights.filter((i) => (rank[i.attention ?? 'medium'] ?? 0) >= minNum);
      if (insightType) set = set.filter((i) => (i.insightType ?? 'analysis') === insightType);
      const total = set.length;
      const pageItems = set.slice(offset, offset + limit);
      route.fulfill({ json: { insights: pageItems.map(toApiInsight), totalCount: total } });
    });
    await page.route('**/v1/ai/notifications/filters*', (route) =>
      route.fulfill({
        json: {
          insightTypes: Array.from(new Set(insights.map((i) => i.insightType ?? 'analysis'))).map(
            (v) => ({ value: v, displayName: v.replace(/_/g, ' '), sourceName: 'e2e-stub' }),
          ),
          attentionLevels: ['info', 'low', 'medium', 'high', 'critical'],
          layerTypes: [DEFAULT_LAYER_ID],
        },
      }),
    );
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
    await page.getByTestId('notification-bell').click();
    const panel = page.getByTestId('notification-panel');
    await expect(panel).toBeVisible();
    return panel;
  }

  test('attention filter (critical) narrows the list and reset restores it', async ({ page }) => {
    const panel = await openWithFilterableInsights(page, [A_HIGH, B_CRIT]);
    // Both seeded insights show initially (default min-attention shows >= medium).
    await expect(panel.getByTestId(`notification-item-${A_HIGH.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${B_CRIT.id}`)).toBeVisible();

    await panel.getByTestId('notification-filter-toggle').click();
    const critChip = panel.getByTestId('notif-filter-attention-critical');
    await expect(critChip).toBeVisible();
    await critChip.click();

    // Refetch with min_attention=critical → only the critical insight remains.
    await expect(critChip).toHaveAttribute('data-active', 'true');
    await expect(panel.getByTestId(`notification-item-${B_CRIT.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${A_HIGH.id}`)).toHaveCount(0);

    // Reset clears the filter → both insights return; reset disappears.
    const reset = panel.getByTestId('notif-filter-reset');
    await expect(reset).toBeVisible();
    await reset.click();
    await expect(panel.getByTestId(`notification-item-${A_HIGH.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${B_CRIT.id}`)).toBeVisible();
    await expect(panel.getByTestId('notif-filter-reset')).toHaveCount(0);
  });

  test('insight-type filter narrows the list to the selected type', async ({ page }) => {
    const panel = await openWithFilterableInsights(page, [A_HIGH, B_CRIT]);
    await expect(panel.getByTestId(`notification-item-${A_HIGH.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${B_CRIT.id}`)).toBeVisible();

    await panel.getByTestId('notification-filter-toggle').click();
    const typeChip = panel.getByTestId('notif-filter-insight-flight_anomaly');
    await expect(typeChip).toBeVisible();
    await typeChip.click();

    // insight_type=flight_anomaly → only A_HIGH remains.
    await expect(typeChip).toHaveAttribute('data-active', 'true');
    await expect(panel.getByTestId(`notification-item-${A_HIGH.id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${B_CRIT.id}`)).toHaveCount(0);
  });

  test('clicking an entity chip selects the entity', async ({ page }) => {
    const panel = await openWithFilterableInsights(page, [A_HIGH, B_CRIT]);
    const item = panel.getByTestId(`notification-item-${A_HIGH.id}`);
    await item.click(); // expand to reveal entity chips

    const chipId = A_HIGH.entities![0].id;
    const chip = panel.getByTestId(`notif-entity-chip-${chipId}`);
    await expect(chip).toBeVisible();
    await chip.click();

    // handleEntityClick → selectMultipleEntities: the entity is now selected in the store.
    await expect
      .poll(() =>
        page.evaluate((id: string) => {
          const w = window as unknown as {
            __store?: () => { selectedEntities: Array<{ entityId: string }> };
          };
          return (w.__store?.().selectedEntities ?? []).some((e) => e.entityId === id);
        }, chipId),
      )
      .toBe(true);
  });

  test('load-more pages in additional notifications', async ({ page }) => {
    // 25 insights with a page size of 20 → the first page leaves more to load.
    const many: StubInsight[] = Array.from({ length: 25 }, (_, i) => ({
      id: `nf-many-${String(i).padStart(2, '0')}`,
      insightType: 'analysis',
      attention: 'high',
      layerType: DEFAULT_LAYER_ID,
      result: { title: `NF Many ${i}` },
      entities: [],
    }));
    const panel = await openWithFilterableInsights(page, many);

    // First page rendered 20 items; the 25th is not yet present.
    await expect(panel.getByTestId(`notification-item-${many[0].id}`)).toBeVisible();
    await expect(panel.getByTestId(`notification-item-${many[24].id}`)).toHaveCount(0);

    const loadMore = panel.getByTestId('notif-load-more');
    await expect(loadMore).toBeVisible();
    await loadMore.click();

    // The remaining 5 page in; the last item is now present and load-more is gone.
    await expect(panel.getByTestId(`notification-item-${many[24].id}`)).toBeVisible();
    await expect(panel.getByTestId('notif-load-more')).toHaveCount(0);
  });

  test('show-more-entities reveals the capped entity chips', async ({ page }) => {
    // ENTITY_DISPLAY_LIMIT is 5; an insight with 6 entities shows a "+1 more" toggle.
    const sixEntities: StubInsight = {
      id: 'nf-six-entities',
      insightType: 'analysis',
      attention: 'high',
      layerType: DEFAULT_LAYER_ID,
      result: { title: 'NF Six Entities' },
      entities: Array.from({ length: 6 }, (_, i) => ({
        id: `flights_commercial:S${i}`,
        externalId: `S${i}`,
        name: `S Entity ${i}`,
        layerType: DEFAULT_LAYER_ID,
      })),
    };
    const panel = await openWithFilterableInsights(page, [sixEntities]);
    const item = panel.getByTestId(`notification-item-${sixEntities.id}`);
    await item.click(); // expand

    // Only the first 5 chips show; the 6th is hidden behind the toggle.
    await expect(panel.getByTestId('notif-entity-chip-flights_commercial:S0')).toBeVisible();
    await expect(panel.getByTestId('notif-entity-chip-flights_commercial:S5')).toHaveCount(0);

    const showMore = panel.getByTestId('notif-show-more-entities');
    await expect(showMore).toBeVisible();
    await showMore.click();

    // All six chips are now visible.
    await expect(panel.getByTestId('notif-entity-chip-flights_commercial:S5')).toBeVisible();
  });
});
