/**
 * Declarative media e2e spec — deterministic, no third-party media.
 *
 * Every camera frame and audio byte in this spec is served by a Playwright
 * route, so the suite never depends on a public camera or radio station being
 * up. Live playback against a real provider is a separate manual check; CI
 * asserts the lifecycle, the request counts and the controls.
 *
 * What the stubs stand in for:
 *   GET https://camera.e2e.example/frame.jpg   → a 1x1 JPEG, counted per request
 *   GET https://radio.e2e.example/stream.mp3   → a short silent MP3
 *   POST /v1/media/playback                    → { reported: true }, counted
 *
 * Markers asserted here (all rendered by src/features/media/):
 *   media-section, media-snapshot-<id>, media-audio-<id>, media-audio-player
 */

import { test, expect, type Page } from '@playwright/test';
import { setupMockRoutes, type StubEntity, type StubMediaConfig } from '../helpers/routes';
import { selectEntity, clearSelection } from '../helpers/selection';

const LAYER_ID = 'media_e2e';
const CAMERA_URL = 'https://camera.e2e.example/frame.jpg';
const STREAM_URL = 'https://radio.e2e.example/stream.mp3';

const MEDIA: StubMediaConfig[] = [
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
    playbackAction: 'report_play',
    audio: {},
  },
];

const ENTITIES: StubEntity[] = [
  {
    id: `${LAYER_ID}:CAM-1`,
    externalId: 'CAM-1',
    name: 'Harbour East',
    layerType: LAYER_ID,
    metadata: { snapshot_url: CAMERA_URL, stream_url: STREAM_URL, attribution: 'E2E Authority' },
    lat: 40,
    lon: -74,
  },
  {
    id: `${LAYER_ID}:CAM-2`,
    externalId: 'CAM-2',
    name: 'Harbour West',
    layerType: LAYER_ID,
    metadata: { snapshot_url: CAMERA_URL, stream_url: STREAM_URL, attribution: 'E2E Authority' },
    lat: 41,
    lon: -73,
  },
];

// A 1x1 JPEG — smallest thing that decodes to a real image.
const JPEG_1PX = Buffer.from(
  '/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0a' +
    'HBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA/8QAFAABAAAAAAAA' +
    'AAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AKp//2Q==',
  'base64',
);

/** Counters for every outbound media request the page makes. */
interface MediaTraffic {
  frames: () => number;
  playbackReports: () => number;
  streams: () => number;
}

async function stubMedia(page: Page): Promise<MediaTraffic> {
  let frames = 0;
  let playbackReports = 0;
  let streams = 0;

  await page.route('https://camera.e2e.example/**', (route) => {
    frames += 1;
    return route.fulfill({ status: 200, contentType: 'image/jpeg', body: JPEG_1PX });
  });
  await page.route('https://radio.e2e.example/**', (route) => {
    streams += 1;
    // An empty body ends the stream immediately; the spec asserts controls and
    // request accounting, not decoded audio.
    return route.fulfill({ status: 200, contentType: 'audio/mpeg', body: Buffer.alloc(0) });
  });
  await page.route('**/v1/media/playback', (route) => {
    playbackReports += 1;
    return route.fulfill({ status: 200, json: { reported: true } });
  });

  return { frames: () => frames, playbackReports: () => playbackReports, streams: () => streams };
}

/**
 * Turn the media layer on. Layers start inactive in the UI store even when the
 * API advertises them as enabled, and a camera only refreshes for an enabled
 * layer — same rule that stops a disabled layer's media.
 */
async function enableMediaLayer(page: Page): Promise<void> {
  await page.getByTestId('toolbar-btn-layers').click();
  const panel = page.getByTestId('panel-layers');
  await expect(panel).toBeVisible();
  const row = page.getByTestId(`filter-option-${LAYER_ID}`);
  await row.click();
  await expect(row).toHaveAttribute('data-active', 'true');
  await panel.getByTestId('config-panel-close').click();
  await expect(panel).not.toBeVisible();
}

test.describe('Declarative media (deterministic fixtures)', () => {
  let traffic: MediaTraffic;

  test.beforeEach(async ({ page }) => {
    traffic = await stubMedia(page);
    await setupMockRoutes(page, { layerId: LAYER_ID, entities: ENTITIES, media: MEDIA });
    await page.goto('/');
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    await enableMediaLayer(page);
  });

  test('loads no media until an entity with declared media is opened', async ({ page }) => {
    // The globe is up and the layer is enabled, but nothing is selected.
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
    expect(traffic.frames()).toBe(0);
    expect(traffic.streams()).toBe(0);

    await selectEntity(page, ENTITIES[0].id, LAYER_ID);
    const section = page.getByTestId('media-section').first();
    await expect(section).toBeVisible();
    await expect(section.getByText('Harbour camera')).toBeVisible();
    await expect(section.getByText('E2E Authority').first()).toBeVisible();

    await expect.poll(() => traffic.frames(), { timeout: 10_000 }).toBeGreaterThan(0);
    // Opening a camera must not start audio.
    expect(traffic.streams()).toBe(0);
    expect(traffic.playbackReports()).toBe(0);
  });

  test('cache-busts each refresh and never overlaps requests', async ({ page }) => {
    const seen: string[] = [];
    page.on('request', (req) => {
      if (req.url().startsWith('https://camera.e2e.example/')) seen.push(req.url());
    });

    await selectEntity(page, ENTITIES[0].id, LAYER_ID);
    await expect.poll(() => seen.length, { timeout: 10_000 }).toBeGreaterThan(0);
    // The declared cache-bust parameter is applied, and the declared path is kept.
    expect(seen[0]).toContain('/frame.jpg');
    expect(seen[0]).toContain('_frame=');

    // Two refresh intervals later there are more frames, each with a distinct
    // cache-bust value — and never two in flight at once.
    await expect.poll(() => seen.length, { timeout: 20_000 }).toBeGreaterThan(1);
    expect(new Set(seen).size).toBe(seen.length);
  });

  test('hands the single camera session over when the selection changes', async ({ page }) => {
    // Two panels at once need addSelectedEntity, which the e2e store hook does
    // not expose; the two-panel "View camera" hand-off is covered by
    // MediaSection.test.tsx. Here the desktop flow — switching selection — must
    // still leave exactly one live camera.
    await selectEntity(page, ENTITIES[0].id, LAYER_ID);
    await expect(page.getByTestId('media-snapshot-camera')).toHaveCount(1);
    await expect.poll(() => traffic.frames(), { timeout: 10_000 }).toBeGreaterThan(0);

    await selectEntity(page, ENTITIES[1].id, LAYER_ID);
    await expect(page.getByTestId('panel-entity-detail').first()).toContainText('Harbour West');
    await expect(page.getByTestId('media-snapshot-camera')).toHaveCount(1);
    // The new panel owns the slot outright — no hand-off prompt is left behind.
    await expect(page.getByRole('button', { name: 'View camera' })).toHaveCount(0);

    const before = traffic.frames();
    await expect.poll(() => traffic.frames(), { timeout: 15_000 }).toBeGreaterThan(before);
  });

  test('pauses and resumes the visible camera on demand', async ({ page }) => {
    await selectEntity(page, ENTITIES[0].id, LAYER_ID);
    const pause = page.getByRole('button', { name: 'Pause Harbour camera' });
    await expect(pause).toBeVisible();
    await expect.poll(() => traffic.frames(), { timeout: 10_000 }).toBeGreaterThan(0);

    await pause.click();
    await expect(page.getByRole('button', { name: 'Resume Harbour camera' })).toBeVisible();
    const paused = traffic.frames();
    // Well past one refresh interval: a paused camera fetches nothing.
    await page.waitForTimeout(7_000);
    expect(traffic.frames()).toBe(paused);

    await page.getByRole('button', { name: 'Resume Harbour camera' }).click();
    await expect.poll(() => traffic.frames(), { timeout: 10_000 }).toBeGreaterThan(paused);
  });

  test('audio needs a gesture, survives closing the panel, and stops on demand', async ({
    page,
  }) => {
    await selectEntity(page, ENTITIES[0].id, LAYER_ID);
    await expect(page.getByTestId('media-audio-radio').first()).toBeVisible();
    // No player and no stream traffic before the user asks for it.
    await expect(page.getByTestId('media-audio-player')).toHaveCount(0);
    expect(traffic.streams()).toBe(0);

    await page.getByRole('button', { name: 'Play Harbour radio' }).first().click();
    const player = page.getByTestId('media-audio-player');
    await expect(player).toBeVisible();
    await expect(player).toContainText('Harbour East');
    await expect(player.getByText('LIVE')).toBeVisible();

    // Closing the detail panel leaves the player in place.
    await clearSelection(page);
    await expect(page.getByTestId('media-section')).toHaveCount(0);
    await expect(player).toBeVisible();

    await player.getByRole('button', { name: 'Stop audio' }).click();
    await expect(page.getByTestId('media-audio-player')).toHaveCount(0);
  });
});
