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
import {
  setupMockRoutes,
  DEFAULT_ENTITIES,
  DEFAULT_LAYER_ID,
  type StubEntity,
  type StubMediaConfig,
} from '../helpers/routes';
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

/**
 * Declarative media on the phone surface. Desktop coverage lives in
 * media.spec.ts; this project is the only one that renders MobileEntityPanel,
 * so the mobile controls are asserted here.
 */
const MEDIA_LAYER_ID = 'media_e2e';
const MEDIA_CAMERA_URL = 'https://camera.e2e.example/frame.jpg';
const MEDIA_STREAM_URL = 'https://radio.e2e.example/stream.mp3';

const MEDIA_CONFIG: StubMediaConfig[] = [
  {
    id: 'camera',
    kind: 'snapshot',
    label: 'Harbour camera',
    urlKey: 'snapshot_url',
    attributionKey: 'attribution',
    allowedOrigins: ['https://camera.e2e.example'],
    snapshot: { refreshIntervalSeconds: 5, cacheBustParam: '_frame' },
  },
  {
    id: 'radio',
    kind: 'audio',
    label: 'Harbour radio',
    urlKey: 'stream_url',
    attributionKey: 'attribution',
    audio: {},
  },
];

const MEDIA_ENTITY: StubEntity = {
  id: `${MEDIA_LAYER_ID}:CAM-1`,
  externalId: 'CAM-1',
  name: 'Harbour East',
  layerType: MEDIA_LAYER_ID,
  metadata: {
    snapshot_url: MEDIA_CAMERA_URL,
    stream_url: MEDIA_STREAM_URL,
    attribution: 'E2E Authority',
  },
  lat: 40,
  lon: -74,
};

// A 1x1 JPEG — the smallest thing that decodes to a real image.
const JPEG_1PX = Buffer.from(
  '/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0a' +
    'HBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAA' +
    'AAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AKp//2Q==',
  'base64',
);

test.describe('Earth App — mobile declarative media (Pixel 5)', () => {
  let frames: number;

  test.beforeEach(async ({ page }) => {
    frames = 0;
    await page.route('https://camera.e2e.example/**', (route) => {
      frames += 1;
      return route.fulfill({ status: 200, contentType: 'image/jpeg', body: JPEG_1PX });
    });
    await page.route('https://radio.e2e.example/**', (route) =>
      route.fulfill({ status: 200, contentType: 'audio/mpeg', body: Buffer.alloc(0) }),
    );
    await setupMockRoutes(page, {
      layerId: MEDIA_LAYER_ID,
      entities: [MEDIA_ENTITY],
      media: MEDIA_CONFIG,
    });
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();

    // Layers start inactive in the store; a camera only refreshes for an
    // enabled layer.
    await navButton(page, 'Layers').click();
    const drawer = page.getByTestId('mobile-drawer-data-layers');
    await expect(drawer).toBeVisible();
    const row = drawer.getByTestId(`filter-option-${MEDIA_LAYER_ID}`);
    await row.click();
    await expect(row).toHaveAttribute('data-active', 'true');
    await page.keyboard.press('Escape');
    await expect(drawer).not.toBeVisible();
  });

  test('the mobile entity panel renders the media section and refreshes one camera', async ({
    page,
  }) => {
    await selectEntity(page, MEDIA_ENTITY.id, MEDIA_LAYER_ID);
    const panel = page.getByTestId('mobile-entity-panel');
    await expect(panel).toBeVisible();
    // The phone panel opens collapsed to a header; the tab body — and with it
    // the camera — only exists once it is expanded.
    await panel.getByRole('button', { name: 'Maximize panel' }).click();

    const section = panel.getByTestId('media-section');
    await expect(section).toBeVisible();
    await expect(section.getByText('Harbour camera')).toBeVisible();
    await expect.poll(() => frames, { timeout: 10_000 }).toBeGreaterThan(0);

    // The card stays inside the phone viewport.
    const box = await section.boundingBox();
    const width = page.viewportSize()!.width;
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);
  });

  test('the persistent audio player is reachable and dismissible on a phone', async ({ page }) => {
    await selectEntity(page, MEDIA_ENTITY.id, MEDIA_LAYER_ID);
    const panel = page.getByTestId('mobile-entity-panel');
    await expect(panel).toBeVisible();
    await panel.getByRole('button', { name: 'Maximize panel' }).click();

    await page.getByRole('button', { name: 'Play Harbour radio' }).first().click();
    const player = page.getByTestId('media-audio-player');
    await expect(player).toBeVisible();
    await expect(player).toContainText('Harbour East');

    const box = await player.boundingBox();
    const width = page.viewportSize()!.width;
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width + 1);

    await player.getByRole('button', { name: 'Stop audio' }).click();
    await expect(page.getByTestId('media-audio-player')).toHaveCount(0);
  });
});
