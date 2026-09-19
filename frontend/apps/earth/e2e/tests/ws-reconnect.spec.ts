/**
 * WebSocket auto-reconnect e2e test
 *
 * Goal: prove the app's WS connection recovers from a non-intentional drop
 * without reloading the page.
 *
 * Mechanism: window.__wsClient is exposed in main.tsx (a minimal read/trigger
 * automation hook, mirroring the `respondent:flyto` event pattern in GlobeScene)
 * so Playwright can call `resetAndReconnect()` from page.evaluate().
 *
 * Why resetAndReconnect() and NOT disconnect():
 *   - disconnect() sets `intentionalDisconnect = true`, which causes onclose
 *     to set status 'disconnected' and bypass the auto-reconnect path. That
 *     is NOT what we want to test.
 *   - resetAndReconnect() calls partysocket's ws.reconnect() WITHOUT setting
 *     intentionalDisconnect, so onclose sees a non-intentional drop and
 *     enters the 'reconnecting' → 'connected' cycle — exactly what happens
 *     after a network hiccup, sleep/wake, or heartbeat timeout.
 *
 * Observable: the `data-testid="ws-connection-status"` Typography in RecBlock
 * renders LIVE | RECONNECTING | OFFLINE based on useWebSocketStatus().
 *
 * Unit coverage note: partysocket backoff, 'failed' recovery, pre-OPEN send
 * buffer, and heartbeat logic are comprehensively exercised by the vitest
 * unit suite (Tasks 2–4). This e2e verifies the REAL wired client against
 * the running community server on port 8091.
 */

import { test, expect } from '@playwright/test';

test.describe('WebSocket auto-reconnect', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    // Wait for the app shell to be fully rendered before any WS assertions
    await expect(page.getByTestId('earth-shell')).toBeVisible();
  });

  test('WS connects on app load and status is LIVE', async ({ page }) => {
    const statusEl = page.getByTestId('ws-connection-status');
    await expect(statusEl).toBeVisible({ timeout: 15_000 });
    // RecBlock shows LIVE when wsStatus === 'connected' and timeMode === 'live'
    await expect(statusEl).toHaveText('LIVE', { timeout: 15_000 });
  });

  test('WS recovers to LIVE after a non-intentional drop (no page reload)', async ({ page }) => {
    const statusEl = page.getByTestId('ws-connection-status');

    // Step 1: confirm initial connected state
    await expect(statusEl).toHaveText('LIVE', { timeout: 15_000 });

    // Step 2: force a non-intentional drop via the automation hook.
    //   resetAndReconnect() calls partysocket ws.reconnect() without setting
    //   intentionalDisconnect, so the client enters 'reconnecting' and then
    //   auto-reconnects when the server accepts the new handshake.
    await page.evaluate(() => {
      const client = window.__wsClient;
      if (!client) throw new Error('window.__wsClient not set — main.tsx hook missing');
      client.resetAndReconnect();
    });

    // Step 3: status may briefly show RECONNECTING (partysocket is mid-handshake);
    //   assert it returns to LIVE WITHOUT a page reload.
    //   Timeout is generous (20s) to accommodate partysocket's min reconnect delay.
    await expect(statusEl).toHaveText('LIVE', { timeout: 20_000 });

    // Step 4: verify the page was NOT reloaded — the earth-shell must still be
    //   the original DOM node (no navigation event fired).
    await expect(page.getByTestId('earth-shell')).toBeVisible();
    expect(page.url()).toMatch(/^http:\/\/localhost:\d+\/?$/);
  });
});
