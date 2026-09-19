#!/usr/bin/env node
/**
 * Report-only FPS / long-task smoke harness for the Respondent globe.
 *
 * Measures rendering frame rate with all available layers toggled on so that
 * local (http://localhost:8090) and prod (https://respondent.alevsk.dev) can
 * be compared as a diagnostic baseline for perf-fix work.
 *
 * Usage:
 *   node tools/perf-smoke.mjs <url> [seconds]
 *
 * Output: structured JSON to stdout
 *   { url, durationSec, layersEnabled, fps: {avg, min, max}, longTasks: {count, totalMs} }
 *
 * NOT a CI gate — report-only. Process exits 0 regardless of fps values.
 */

import { chromium } from 'playwright';

const url = process.argv[2] ?? 'http://localhost:8090';
const durationSec = Number(process.argv[3] ?? 15);

if (!url.startsWith('http')) {
  console.error('[perf-smoke] usage: node tools/perf-smoke.mjs <url> [seconds]');
  process.exit(1);
}

console.error(`[perf-smoke] target=${url} duration=${durationSec}s`);

const browser = await chromium.launch({ headless: true });
try {
const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });

// Surface console errors from the page for diagnostics.
page.on('console', (msg) => {
  if (msg.type() === 'error') {
    console.error(`[page:error] ${msg.text()}`);
  }
});

await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 30_000 });

// Wait for the globe canvas to be visible — mandatory.
console.error('[perf-smoke] waiting for globe canvas...');
try {
  await page.getByTestId('globe-container').locator('canvas').first().waitFor({
    state: 'visible',
    timeout: 30_000,
  });
  console.error('[perf-smoke] globe canvas visible');
} catch (err) {
  console.error(`[perf-smoke] WARNING: globe canvas not found (${err.message}); FPS will still be sampled`);
}

// ---------------------------------------------------------------------------
// BEST-EFFORT: enable all layers.
// Layer toggles are FilterOption boxes with data-testid="filter-option-<layer-name>"
// and data-active attribute. We click any that are not active.
// Wrapped in try/catch so a broken panel never aborts the FPS measurement.
// ---------------------------------------------------------------------------
let layersEnabled = 0;

try {
  // Open the layers panel via the toolbar button.
  const layersBtn = page.getByTestId('toolbar-btn-layers');
  await layersBtn.waitFor({ state: 'visible', timeout: 10_000 });
  await layersBtn.click();
  console.error('[perf-smoke] layers panel button clicked');

  const panel = page.getByTestId('panel-layers');
  await panel.waitFor({ state: 'visible', timeout: 10_000 });
  console.error('[perf-smoke] layers panel open');

  // Layer options are FilterOption boxes: [role="option"] inside the panel.
  // data-active="false" means not yet enabled.
  const layerOptions = panel.locator('[role="option"]');
  const totalOptions = await layerOptions.count();
  console.error(`[perf-smoke] found ${totalOptions} layer options`);

  for (let i = 0; i < totalOptions; i++) {
    const option = layerOptions.nth(i);
    try {
      const isVisible = await option.isVisible({ timeout: 1_000 });
      if (!isVisible) continue;
      const active = await option.getAttribute('data-active');
      if (active !== 'true') {
        await option.click({ timeout: 2_000 });
        layersEnabled++;
      }
    } catch (toggleErr) {
      console.error(`[perf-smoke] could not toggle layer ${i}: ${toggleErr.message}`);
    }
  }

  console.error(`[perf-smoke] enabled ${layersEnabled} layers (of ${totalOptions} found)`);

  // Close the panel so it doesn't occlude the globe.
  await page.keyboard.press('Escape').catch(() => {});
} catch (panelErr) {
  console.error(`[perf-smoke] could not enable layers via panel: ${panelErr.message}`);
  console.error('[perf-smoke] proceeding with default view for FPS measurement');
}

// Let entities stream in before sampling.
console.error('[perf-smoke] waiting 3s for entity stream...');
await page.waitForTimeout(3_000);

// ---------------------------------------------------------------------------
// DETERMINISTIC SCRIPTED CAMERA PAN
//
// Drive viewer.camera through a fixed longitude sweep (0° → 360°) in
// PAN_STEPS equal increments over the ENTIRE sampling window.  The pan runs
// concurrently with FPS/long-task sampling so the measurement captures
// rendering load during camera motion (the worst-case scenario for the globe).
//
// Uses window.__cesiumViewer which GlobeScene.tsx always exposes (prod + dev).
// Cesium's setView() is synchronous — no await needed inside the rAF loop.
// ---------------------------------------------------------------------------
const PAN_STEPS = 36;            // 10° per step → full 360° sweep
const PAN_LAT   = 20;            // fixed latitude (degrees) — nice equator-ish view
const PAN_ALT   = 12_000_000;    // 12 Mm — sees ~1/3 of the globe; enough entities visible

console.error(`[perf-smoke] sampling FPS for ${durationSec}s with scripted camera pan (${PAN_STEPS} steps, 360° sweep)...`);

// Kick off the deterministic pan in the page in parallel with FPS sampling.
// We launch it as a floating Promise inside page.evaluate so it runs alongside
// the rAF loop rather than blocking it.  Step interval = samplingMs / PAN_STEPS.
const report = await page.evaluate(async ({ samplingMs, panSteps, panLat, panAlt }) => {
  // ── Deterministic camera pan (runs concurrently with FPS loop) ──────────
  // Computes WGS84 Cartesian3 without the Cesium global (bundled ES modules
  // don't expose window.Cesium in production builds).
  function lonLatAltToCartesian3(lonDeg, latDeg, altM) {
    const a = 6378137.0;                // WGS84 semi-major axis
    const e2 = 0.00669437999014;        // WGS84 first eccentricity squared
    const lon = lonDeg * Math.PI / 180;
    const lat = latDeg * Math.PI / 180;
    const N = a / Math.sqrt(1 - e2 * Math.sin(lat) ** 2);
    return {
      x: (N + altM) * Math.cos(lat) * Math.cos(lon),
      y: (N + altM) * Math.cos(lat) * Math.sin(lon),
      z: (N * (1 - e2) + altM) * Math.sin(lat),
    };
  }

  const viewer = window.__cesiumViewer;
  const panApplied = !!(viewer && viewer.camera && viewer.scene);
  if (panApplied) {
    const stepMs = samplingMs / panSteps;
    (async () => {
      for (let i = 0; i <= panSteps; i++) {
        const lon = (i / panSteps) * 360 - 180;          // -180 → +180
        try {
          viewer.camera.setView({
            destination: lonLatAltToCartesian3(lon, panLat, panAlt),
            orientation: {
              heading: 0,                 // north-up
              pitch:   -Math.PI / 4,     // -45° tilt
              roll:    0,
            },
          });
          viewer.scene.requestRender();
        } catch {
          // non-fatal if viewer is destroyed mid-run
        }
        await new Promise((r) => setTimeout(r, stepMs));
      }
    })();
  }

  // ── Longtask observer ───────────────────────────────────────────────────
  const longTaskDurations = [];
  try {
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        longTaskDurations.push(Math.round(entry.duration));
      }
    });
    observer.observe({ entryTypes: ['longtask'] });
  } catch {
    // longtask may be unsupported in some Chromium builds — non-fatal.
  }

  // ── rAF-based FPS sampling ──────────────────────────────────────────────
  const frameDeltasMs = [];
  let lastTs = performance.now();

  await new Promise((resolve) => {
    const deadline = performance.now() + samplingMs;

    function tick(now) {
      const delta = now - lastTs;
      if (delta > 0) frameDeltasMs.push(delta);
      lastTs = now;
      if (now < deadline) {
        requestAnimationFrame(tick);
      } else {
        resolve();
      }
    }

    requestAnimationFrame(tick);
  });

  if (frameDeltasMs.length === 0) {
    return {
      fps: { avg: 0, min: 0, max: 0, samples: 0 },
      longTasks: { count: 0, totalMs: 0 },
      panApplied,
    };
  }

  const fpsSamples = frameDeltasMs.map((d) => 1000 / d);
  const avg = fpsSamples.reduce((a, b) => a + b, 0) / fpsSamples.length;
  const min = Math.min(...fpsSamples);
  const max = Math.max(...fpsSamples);
  const longTaskTotal = longTaskDurations.reduce((a, b) => a + b, 0);

  return {
    fps: {
      avg: Math.round(avg * 10) / 10,
      min: Math.round(min * 10) / 10,
      max: Math.round(max * 10) / 10,
      samples: fpsSamples.length,
    },
    longTasks: {
      count: longTaskDurations.length,
      totalMs: longTaskTotal,
    },
    panApplied,
  };
}, { samplingMs: durationSec * 1000, panSteps: PAN_STEPS, panLat: PAN_LAT, panAlt: PAN_ALT });

const result = {
  url,
  durationSec,
  layersEnabled,
  panApplied: report.panApplied ?? false,
  fps: report.fps,
  longTasks: report.longTasks,
};

// Structured JSON to stdout — stderr carries progress logs.
process.stdout.write(JSON.stringify(result, null, 2) + '\n');
} finally {
  await browser.close();
}
