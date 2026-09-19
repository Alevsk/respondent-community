import { defineConfig, devices } from '@playwright/test';

// Community edition: open globe, no auth, single binary on :8090.
// Adapted from the enterprise earth e2e config (pre geo-demolition,
// commit 2d2c6353~1) with all auth/persona/workspace machinery removed.
const BASE_URL = process.env.E2E_BASE_URL ?? 'http://localhost:8090';

export default defineConfig({
  testDir: './tests',
  globalSetup: './global-setup.ts',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  // No retries: the e2e server runs with ingestion disabled
  // (RESPONDENT_INGEST_ENABLED=false), so there is no live-feed WebSocket churn and
  // the deterministic specs are genuinely stable. retries:0 surfaces any real flake
  // immediately instead of masking it.
  retries: 0,
  // Concurrency is a fixed, conservative 2 — NOT a per-core multiplier. Each
  // worker is a full Chromium instance; together with the mobile-chrome project
  // and the single Go server they are CPU- and memory-heavy, so browser e2e does
  // not scale with core count (a "50%-of-cores" setting spawns 8 workers on a
  // 16-core box and oversubscribes badly). The previous hardcoded 4 tipped the
  // heaviest spec (entity-detail tab switching — several Cesium-backed tab swaps)
  // past the 30s timeout on constrained hosts (a small VPS, or a box also running
  // the :8090 server) — a load artifact, not a real flake. 2 is stable across
  // hosts; raise it on a clean, idle, many-core box via E2E_WORKERS.
  workers: process.env.E2E_WORKERS ? Number(process.env.E2E_WORKERS) : 2,
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },
  // The desktop project runs every spec EXCEPT mobile.spec.ts; the mobile
  // project (Pixel 5 emulation) runs ONLY mobile.spec.ts. Scoping via
  // testIgnore/testMatch keeps desktop specs off the phone viewport and the
  // mobile-only surface off the desktop viewport.
  projects: [
    { name: 'chromium', testIgnore: /mobile\.spec\.ts$/, use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile-chrome', testMatch: /mobile\.spec\.ts$/, use: { ...devices['Pixel 5'] } },
  ],
  reporter: [['html', { outputFolder: './playwright-report', open: 'never' }], ['list']],
});
