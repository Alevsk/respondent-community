/**
 * Entity Detail Panel e2e spec — deterministic, no live feed.
 *
 * setupMockRoutes (BEFORE goto) stubs /v1/layers, the snapshot, search, and
 * /v1/entities/detail so the selected entity renders real content (name +
 * Location/metadata), not a permanent loading state. selectEntity drives
 * window.__store_setSelected → mounts the desktop EntityDetailPanel (one per
 * selected entity) without a canvas click.
 *
 * Source audit (2026-06-21) — confirmed against the real components:
 *   panel-entity-detail            : ConfigPanel root (EntityDetailPanel.tsx:162)
 *   config-panel-status-value      : entity name in the status bar (ConfigPanel.tsx:458)
 *   entity-tab-overview|timeline|metadata
 *                                  : tab boxes with data-active (EntityDetailPanel.tsx:87-88).
 *                                    There is NO `ai` tab — only these three register
 *                                    (OverviewTab/HistoryTab(id 'timeline')/MetadataTab).
 *   config-panel-maximize|minimize|close
 *                                  : scoped header IconButtons (ConfigPanel.tsx:398/379/413).
 *                                    maximize aria-label toggles Maximize panel ↔ Restore panel
 *                                    and the panel root grows to 90vw when maximized.
 *   entity-exit-view               : only rendered when viewMode === 'entity'
 *                                    (EntityDetailPanel.tsx:123). See the deferred test below.
 *
 * Per-tab body markers (unique to each tab, asserted inside config-panel-content):
 *   Overview : layer-type badge text + "Location" section header + "Latitude" field.
 *   Timeline : the "Table view" toggle button (aria-label) — only TimelineTab renders it.
 *   Metadata : the "Entity" section header + raw "callsign" metadata key — only MetadataTab
 *              shows raw metadata keys (Overview routes them through the field-renderer).
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes, DEFAULT_LAYER_ID, DEFAULT_ENTITIES } from '../helpers/routes';
import { selectEntity, getSelectedEntityId } from '../helpers/selection';

const STUB = DEFAULT_ENTITIES[0]; // flights_commercial:E2E001 / "E2E Flight Alpha"

test.describe('Entity Detail Panel (deterministic, no live feed)', () => {
  test.beforeEach(async ({ page }) => {
    // Routes MUST be registered before navigation so the boot fetches hit stubs.
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('selecting an entity mounts the panel and shows entity content', async ({ page }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);

    // Store reflects the selection and the panel mounts.
    await expect.poll(() => getSelectedEntityId(page)).toBe(STUB.id);
    const panel = page.getByTestId('panel-entity-detail');
    await expect(panel).toBeVisible();

    // Real content: the entity name appears in the status bar (driven by the
    // stubbed /v1/entities/detail response), proving it is not stuck loading.
    await expect(panel.getByTestId('config-panel-status-value')).toHaveText(STUB.name);

    // Overview (default tab) shows the Location section from latestObservation.
    const content = panel.getByTestId('config-panel-content');
    await expect(content.getByText('Location', { exact: true })).toBeVisible();
    await expect(content.getByText('Latitude', { exact: true })).toBeVisible();
  });

  test('tab switching flips data-active and changes the tab body', async ({ page }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);
    const panel = page.getByTestId('panel-entity-detail');
    await expect(panel).toBeVisible();

    const overviewTab = panel.getByTestId('entity-tab-overview');
    const timelineTab = panel.getByTestId('entity-tab-timeline');
    const metadataTab = panel.getByTestId('entity-tab-metadata');
    const content = panel.getByTestId('config-panel-content');

    // Default: Overview active, others inactive; Overview body visible.
    await expect(overviewTab).toHaveAttribute('data-active', 'true');
    await expect(timelineTab).toHaveAttribute('data-active', 'false');
    await expect(metadataTab).toHaveAttribute('data-active', 'false');
    await expect(content.getByText('Location', { exact: true })).toBeVisible();

    // Switch to Timeline: data-active flips, Timeline-only control appears,
    // Overview's Location header is gone.
    await timelineTab.click();
    await expect(timelineTab).toHaveAttribute('data-active', 'true');
    await expect(overviewTab).toHaveAttribute('data-active', 'false');
    await expect(metadataTab).toHaveAttribute('data-active', 'false');
    await expect(content.getByLabel('Table view')).toBeVisible();
    await expect(content.getByText('Location', { exact: true })).toHaveCount(0);

    // Switch to Metadata: data-active flips, the MetadataTab "Entity" section
    // header and the raw "callsign" metadata key appear (Overview routes that key
    // through the field renderer, so the literal key is unique to this tab); the
    // Timeline control is gone. The key shows in both the Entity and Observation
    // sections, so assert the first occurrence.
    await metadataTab.click();
    await expect(metadataTab).toHaveAttribute('data-active', 'true');
    await expect(overviewTab).toHaveAttribute('data-active', 'false');
    await expect(timelineTab).toHaveAttribute('data-active', 'false');
    await expect(content.getByText('Entity', { exact: true })).toBeVisible();
    await expect(content.getByText('callsign', { exact: true }).first()).toBeVisible();
    await expect(content.getByLabel('Table view')).toHaveCount(0);

    // Switch back to Overview: body returns.
    await overviewTab.click();
    await expect(overviewTab).toHaveAttribute('data-active', 'true');
    await expect(metadataTab).toHaveAttribute('data-active', 'false');
    await expect(content.getByText('Location', { exact: true })).toBeVisible();
  });

  test('maximize toggles the panel size, restore reverts, close dismisses it', async ({ page }) => {
    await selectEntity(page, STUB.id, DEFAULT_LAYER_ID);
    const panel = page.getByTestId('panel-entity-detail');
    await expect(panel).toBeVisible();

    const maximize = panel.getByTestId('config-panel-maximize');
    // Normal mode: button invites maximizing.
    await expect(maximize).toHaveAttribute('aria-label', 'Maximize panel');
    const normalBox = await panel.boundingBox();
    expect(normalBox).not.toBeNull();

    // Maximize: button toggles to a restore affordance and the panel grows.
    await maximize.click();
    await expect(maximize).toHaveAttribute('aria-label', 'Restore panel');
    await expect
      .poll(async () => (await panel.boundingBox())?.width ?? 0)
      .toBeGreaterThan((normalBox!.width ?? 0) + 100);

    // Restore: button reverts and the panel shrinks back near its original width.
    await maximize.click();
    await expect(maximize).toHaveAttribute('aria-label', 'Maximize panel');
    await expect
      .poll(async () => (await panel.boundingBox())?.width ?? 0)
      .toBeLessThan((normalBox!.width ?? 0) + 50);

    // Close (scoped to this panel) dismisses it.
    await panel.getByTestId('config-panel-close').click();
    await expect(panel).not.toBeVisible();
  });

  // exit-view (entity-exit-view) — NOW COVERED in globe-interaction.spec.ts (Task 11).
  //
  // The button only renders when viewMode === 'entity' (EntityDetailPanel.tsx:123),
  // which the Cesium canvas LEFT_DOUBLE_CLICK handler reaches via setViewMode('entity')
  // (useEntityInteraction.ts:116-129). Task 11 added a deterministic store seam
  // (window.__store_enterEntityView, mirroring that handler) plus viewMode in the
  // window.__store() snapshot, so the enter → assert → exit → assert flow lives in
  // globe-interaction.spec.ts ("exit-view returns viewMode to globe..."). The prior
  // test.fixme placeholder has been removed now that the coverage exists.
});
