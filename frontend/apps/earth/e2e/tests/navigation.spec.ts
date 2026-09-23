/**
 * e2e spec — Navigation panel: camera controls.
 *
 * Covers:
 *  - Opening the navigation panel via toolbar-btn-nav
 *  - nav-zoom-in → camera altitude DECREASES (polled via window.__cesiumViewer)
 *  - nav-zoom-out → camera altitude INCREASES (polled via window.__cesiumViewer)
 *  - nav-north-up → camera heading returns to ~0 after being set to a non-zero
 *    value via window.__cesiumViewer (polled with bounded wait for flyTo animation)
 *  - nav-reset-view → camera CONVERGES to DEFAULT_CAMERA (18_000_000 m altitude,
 *    heading 0, lon 0, lat 20) after being perturbed to a very different state;
 *    asserts within ±20% of the default altitude (polled with bounded wait, up to
 *    15 s for the flyTo animation to settle)
 *  - config-panel-minimize → panel collapses to minimized state
 *
 * Camera observability:
 *  - window.__cesiumViewer exposes the Cesium Viewer (set in GlobeScene.tsx).
 *  - viewer.camera.positionCartographic.height gives the geodetic altitude in metres.
 *  - viewer.camera.heading gives the heading in radians (0 = north).
 *  - Zoom (zoomIn/zoomOut) is instantaneous (no animation); altitude is readable
 *    immediately after click, but we still use a short poll (up to 3 s) to absorb
 *    any single-frame delay.
 *  - flyTo (northUp, resetView) animates by default; we poll up to 15 s for the
 *    heading/altitude to settle at the expected value.
 *
 * Conventions: getByTestId only; setupMockRoutes before goto; bottom-toolbar fence;
 * panel scoped close/minimize; stateless / parallel.
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes } from '../helpers/routes';

const BASE_URL = process.env.E2E_BASE_URL ?? 'http://localhost:8090';

// DEFAULT_CAMERA values from frontend/apps/earth/src/features/globe/store.ts.
// handleResetView flies to these exact values via Cartesian3.fromDegrees + CesiumMath.toRadians.
const DEFAULT_ALTITUDE_M = 18_000_000; // 18 Mm
const DEFAULT_LON = 0;
const DEFAULT_LAT = 20;

// Convergence tolerance: ±20% of the default altitude.
const ALT_TOLERANCE_RATIO = 0.2;
const ALT_LOWER = DEFAULT_ALTITUDE_M * (1 - ALT_TOLERANCE_RATIO); // 14_400_000 m
const ALT_UPPER = DEFAULT_ALTITUDE_M * (1 + ALT_TOLERANCE_RATIO); // 21_600_000 m

// ── helpers ───────────────────────────────────────────────────────────────────

type CesiumCamera = {
  positionCartographic: { height: number };
  heading: number;
  flyTo: (opts: unknown) => void;
};

/** Read the current camera altitude (metres) from the Cesium viewer. */
async function getCameraAltitude(page: import('@playwright/test').Page): Promise<number> {
  return page.evaluate(() => {
    const viewer = (window as unknown as Record<string, unknown>).__cesiumViewer as
      { camera: CesiumCamera } | undefined;
    if (!viewer) throw new Error('window.__cesiumViewer not exposed');
    return viewer.camera.positionCartographic.height;
  });
}

/** Read the current camera heading (radians) from the Cesium viewer. */
async function getCameraHeading(page: import('@playwright/test').Page): Promise<number> {
  return page.evaluate(() => {
    const viewer = (window as unknown as Record<string, unknown>).__cesiumViewer as
      { camera: CesiumCamera } | undefined;
    if (!viewer) throw new Error('window.__cesiumViewer not exposed');
    return viewer.camera.heading;
  });
}

/** Read the current camera lon/lat (degrees) from the Cesium viewer. */
async function getCameraLonLat(
  page: import('@playwright/test').Page,
): Promise<{ lon: number; lat: number }> {
  return page.evaluate(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const viewer = (window as any).__cesiumViewer;
    if (!viewer) throw new Error('window.__cesiumViewer not exposed');
    // Cesium exposes positionCartographic in radians; convert to degrees.
    const carto = viewer.camera.positionCartographic;
    return {
      lon: (carto.longitude * 180) / Math.PI,
      lat: (carto.latitude * 180) / Math.PI,
    };
  });
}

/**
 * Poll altitude until the predicate is satisfied (or settled: same value twice
 * in a row) OR timeoutMs elapses. "Settled" detection avoids firing mid-flight.
 * Returns the final altitude.
 */
async function pollAltitudeUntilSettled(
  page: import('@playwright/test').Page,
  predicate: (alt: number) => boolean,
  timeoutMs = 15_000,
): Promise<number> {
  const deadline = Date.now() + timeoutMs;
  let prev = await getCameraAltitude(page);
  await page.waitForTimeout(200);
  let curr = await getCameraAltitude(page);

  while (Date.now() < deadline) {
    if (predicate(curr) && Math.abs(curr - prev) < curr * 0.005) {
      // Predicate satisfied and value has stabilised (< 0.5% change per tick).
      break;
    }
    await page.waitForTimeout(300);
    prev = curr;
    curr = await getCameraAltitude(page);
  }
  return curr;
}

/**
 * Poll altitude until the predicate is satisfied or timeoutMs elapses.
 * Returns the final altitude regardless of whether the predicate was satisfied.
 */
async function pollAltitude(
  page: import('@playwright/test').Page,
  predicate: (alt: number) => boolean,
  timeoutMs = 8_000,
): Promise<number> {
  const deadline = Date.now() + timeoutMs;
  let last = await getCameraAltitude(page);
  while (!predicate(last) && Date.now() < deadline) {
    await page.waitForTimeout(200);
    last = await getCameraAltitude(page);
  }
  return last;
}

/**
 * Poll heading until the predicate is satisfied or timeoutMs elapses.
 * Returns the final heading regardless of whether the predicate was satisfied.
 */
async function pollHeading(
  page: import('@playwright/test').Page,
  predicate: (hdg: number) => boolean,
  timeoutMs = 8_000,
): Promise<number> {
  const deadline = Date.now() + timeoutMs;
  let last = await getCameraHeading(page);
  while (!predicate(last) && Date.now() < deadline) {
    await page.waitForTimeout(200);
    last = await getCameraHeading(page);
  }
  return last;
}

/** Set the camera to a known state instantly (no animation) via __cesiumViewer. */
async function setCameraInstant(
  page: import('@playwright/test').Page,
  opts: { lon: number; lat: number; altM: number; headingRad?: number; pitchRad?: number },
): Promise<void> {
  await page.evaluate((o) => {
    const viewer = (window as unknown as Record<string, unknown>).__cesiumViewer as
      { camera: CesiumCamera } | undefined;
    if (!viewer) throw new Error('window.__cesiumViewer not exposed');
    viewer.camera.flyTo({
      destination: {
        // Inline Cartesian3.fromDegrees approximation:
        // Using WGS84 semi-major axis 6378137 m + altitude.
        // Good enough for a test camera placement.
        x:
          (6_378_137 + o.altM) *
          Math.cos((o.lat * Math.PI) / 180) *
          Math.cos((o.lon * Math.PI) / 180),
        y:
          (6_378_137 + o.altM) *
          Math.cos((o.lat * Math.PI) / 180) *
          Math.sin((o.lon * Math.PI) / 180),
        z: (6_378_137 + o.altM) * Math.sin((o.lat * Math.PI) / 180),
      },
      orientation: {
        heading: o.headingRad ?? 0,
        pitch: o.pitchRad ?? -Math.PI / 2,
        roll: 0,
      },
      duration: 0, // instant
    });
  }, opts);
  // Give the synchronous flyTo a single frame to apply.
  await page.waitForTimeout(100);
}

// ── suite ────────────────────────────────────────────────────────────────────

test.describe('Navigation panel — camera controls', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto(BASE_URL + '/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();

    // Open navigation panel.
    await page.getByTestId('toolbar-btn-nav').click();
    await expect(page.getByTestId('panel-navigation')).toBeVisible();
  });

  // ── zoom-in: altitude DECREASES ───────────────────────────────────────────
  test('nav-zoom-in decreases camera altitude', async ({ page }) => {
    // Place the camera at a known mid-level altitude so zoom-in has room.
    await setCameraInstant(page, { lon: 0, lat: 20, altM: 5_000_000 });

    const altBefore = await getCameraAltitude(page);

    await page.getByTestId('nav-zoom-in').click();

    // zoomIn is instantaneous; poll briefly to absorb any single-frame delay.
    const altAfter = await pollAltitude(page, (a) => a < altBefore, 3_000);

    expect(altAfter).toBeLessThan(altBefore);
  });

  // ── zoom-out: altitude INCREASES ──────────────────────────────────────────
  test('nav-zoom-out increases camera altitude', async ({ page }) => {
    // Place the camera at a mid-level altitude so zoom-out has room upward.
    await setCameraInstant(page, { lon: 0, lat: 20, altM: 1_000_000 });

    const altBefore = await getCameraAltitude(page);

    await page.getByTestId('nav-zoom-out').click();

    // zoomOut is instantaneous; poll briefly to absorb any single-frame delay.
    const altAfter = await pollAltitude(page, (a) => a > altBefore, 3_000);

    expect(altAfter).toBeGreaterThan(altBefore);
  });

  // ── north-up: heading → ~0 (north) ────────────────────────────────────────
  test('nav-north-up resets heading to ~0 (north)', async ({ page }) => {
    // Set a non-zero heading (~45°) so north-up has a measurable effect.
    const fortyFiveRad = Math.PI / 4; // 45°
    await setCameraInstant(page, { lon: 0, lat: 20, altM: 5_000_000, headingRad: fortyFiveRad });

    // Verify the non-zero heading was applied before clicking.
    const headingBefore = await getCameraHeading(page);
    // Allow some tolerance since flyTo duration:0 may still slightly adjust.
    // Accept anything >= 0.1 rad (~5.7°) as "non-north".
    expect(headingBefore).toBeGreaterThanOrEqual(0.1);

    await page.getByTestId('nav-north-up').click();

    // northUp calls flyTo which animates; poll until heading converges to ~0.
    // Math.PI * 2 = 360° = same as 0 — normalise to [0, 2π) and check < 0.1 rad.
    const headingAfter = await pollHeading(
      page,
      (h) => {
        const normalised = ((h % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
        return normalised < 0.1 || normalised > Math.PI * 2 - 0.1;
      },
      8_000,
    );

    const normalised = ((headingAfter % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
    // Within 0.1 rad (~5.7°) of north (0 or 2π).
    const isNearNorth = normalised < 0.1 || normalised > Math.PI * 2 - 0.1;
    expect(isNearNorth).toBe(true);
  });

  // ── reset-view: camera CONVERGES to DEFAULT_CAMERA (18 Mm, heading 0, lon 0, lat 20) ──
  test('nav-reset-view flies camera toward default altitude (~18 Mm)', async ({ page }) => {
    // Perturb to a clearly-different state: low altitude, off-center lon/lat, non-zero heading.
    // handleResetView targets: lon=0, lat=20, alt=18_000_000, heading=0 (store DEFAULT_CAMERA).
    await setCameraInstant(page, {
      lon: 45,
      lat: -30,
      altM: 100_000,
      headingRad: Math.PI / 3, // 60° — clearly non-north
    });

    const altBefore = await getCameraAltitude(page);
    // Verify camera is far from the default altitude before clicking.
    expect(altBefore).toBeLessThan(DEFAULT_ALTITUDE_M * 0.1); // < 1.8 Mm

    await page.getByTestId('nav-reset-view').click();

    // Poll until altitude converges into the ±20% band around DEFAULT_ALTITUDE_M
    // (14.4 Mm – 21.6 Mm), with settlement detection to avoid asserting mid-flight.
    // Bound: 15 s to let the flyTo animation finish.
    const altAfter = await pollAltitudeUntilSettled(
      page,
      (a) => a >= ALT_LOWER && a <= ALT_UPPER,
      15_000,
    );

    // Must have moved upward from the start.
    expect(altAfter).toBeGreaterThan(altBefore);

    // Must have converged to the DEFAULT_ALTITUDE_M ±20% band.
    // This assertion FAILS if the camera merely moved up (e.g. to 2 Mm) without
    // reaching the default — distinguishing a working reset from a broken one.
    expect(altAfter).toBeGreaterThanOrEqual(ALT_LOWER); // >= 14.4 Mm
    expect(altAfter).toBeLessThanOrEqual(ALT_UPPER); // <= 21.6 Mm

    // Also assert heading has converged to ~0 (north), matching DEFAULT_CAMERA.heading=0.
    const headingAfter = await pollHeading(
      page,
      (h) => {
        const norm = ((h % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
        return norm < 0.15 || norm > Math.PI * 2 - 0.15;
      },
      3_000, // heading settles quickly once altitude is done
    );
    const normHeading = ((headingAfter % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);
    // Within 0.15 rad (~8.6°) of north — DEFAULT_CAMERA.heading = 0°.
    expect(normHeading < 0.15 || normHeading > Math.PI * 2 - 0.15).toBe(true);

    // Assert lon/lat converged toward the default (lon=0, lat=20) within ±5°.
    const { lon: lonAfter, lat: latAfter } = await getCameraLonLat(page);
    expect(Math.abs(lonAfter - DEFAULT_LON)).toBeLessThanOrEqual(5);
    expect(Math.abs(latAfter - DEFAULT_LAT)).toBeLessThanOrEqual(5);
  });

  // ── minimize ──────────────────────────────────────────────────────────────
  test('config-panel-minimize collapses the navigation panel', async ({ page }) => {
    const panel = page.getByTestId('panel-navigation');
    await panel.getByTestId('config-panel-minimize').click();

    // After minimize the nav buttons should no longer be visible.
    // The panel container may still exist (minimized chip) but content is hidden.
    await expect(page.getByTestId('nav-zoom-in')).not.toBeVisible();
  });
});
