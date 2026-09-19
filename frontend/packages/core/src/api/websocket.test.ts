/**
 * websocket.ts — WebSocketClient unit tests (partysocket adoption)
 *
 * Verifies the partysocket ReconnectingWebSocket wiring: reconnection is no
 * longer capped at the old 5-attempt hard death, the subscribedLayers replay
 * fires the correct message type per mode on (re)open, the StrictMode connect
 * guard prevents duplicate sockets, and resetAndReconnect() recovers from the
 * 'failed' terminal state.
 *
 * Strategy: mock `partysocket/ws` so each ReconnectingWebSocket is a fake we
 * drive directly (simulateOpen/Close, retryCount). This tests OUR wiring
 * without depending on partysocket's async connect/backoff internals.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// ---------------------------------------------------------------------------
// Mock partysocket's ReconnectingWebSocket
// ---------------------------------------------------------------------------

interface MockRWS {
  url: string;
  options: Record<string, unknown> | undefined;
  readyState: number;
  retryCount: number;
  send: ReturnType<typeof vi.fn>;
  close: ReturnType<typeof vi.fn>;
  reconnect: ReturnType<typeof vi.fn>;
  onopen: ((event: Event) => void) | null;
  onclose: ((event: Event) => void) | null;
  onmessage: ((event: MessageEvent) => void) | null;
  onerror: ((event: Event) => void) | null;
  // helpers
  simulateOpen: () => void;
  simulateClose: () => void;
  simulateMessage: (data: unknown) => void;
}

let mockInstances: MockRWS[] = [];

vi.mock('partysocket/ws', () => {
  // WebSocket readyState constants the client reads (WebSocket.OPEN etc.).
  // jsdom provides a real WebSocket, but be explicit so the mock is self-contained.
  const RWS = vi.fn(function (
    this: MockRWS,
    url: string,
    _protocols: unknown,
    options: Record<string, unknown> | undefined,
  ) {
    this.url = url;
    this.options = options;
    this.readyState = 0; // CONNECTING
    this.retryCount = 0;
    this.onopen = null;
    this.onclose = null;
    this.onmessage = null;
    this.onerror = null;
    this.send = vi.fn();
    this.close = vi.fn(() => {
      this.readyState = 3; // CLOSED
    }) as ReturnType<typeof vi.fn>;
    this.reconnect = vi.fn(() => {
      this.readyState = 0; // CONNECTING
      this.retryCount = 0;
    }) as ReturnType<typeof vi.fn>;
    this.simulateOpen = () => {
      this.readyState = 1; // OPEN
      this.onopen?.(new Event('open'));
    };
    this.simulateClose = () => {
      this.readyState = 3; // CLOSED
      this.onclose?.(new Event('close'));
    };
    this.simulateMessage = (data: unknown) => {
      this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }));
    };
    mockInstances.push(this);
  });
  return { default: RWS };
});

// Fresh module per test to avoid singleton cross-test pollution.
async function importFresh() {
  vi.resetModules();
  mockInstances = [];
  return import('./websocket');
}

beforeEach(() => {
  mockInstances = [];
});

// ---------------------------------------------------------------------------
// partysocket adoption: infinite retry (no 5-attempt cap)
// ---------------------------------------------------------------------------

describe('WebSocketClient — partysocket reconnection (no 5-attempt cap)', () => {
  it('uses ReconnectingWebSocket, not raw WebSocket', async () => {
    const RWS = (await import('partysocket/ws')).default;
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    expect(mockInstances).toHaveLength(1);
    expect(RWS).toHaveBeenCalledWith(expect.any(String), undefined, expect.any(Object));
  });

  it('does not permanently die after >5 closes — keeps a live socket', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // partysocket owns the reconnect loop; closes below the configured max
    // stay in 'reconnecting' (never the old terminal 'disconnected' at 5).
    for (let i = 1; i <= 4; i++) {
      ws.retryCount = i;
      ws.simulateClose();
      expect(client.status).toBe('reconnecting');
    }
  });

  it('enters the failed state only once partysocket exhausts its retries', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Below max → reconnecting.
    ws.retryCount = 3;
    ws.simulateClose();
    expect(client.status).toBe('reconnecting');

    // At/over max → failed (recoverable via resetAndReconnect).
    ws.retryCount = 5;
    ws.simulateClose();
    expect(client.status).toBe('failed');
  });

  it('configures partysocket for retry with backoff (maxRetries + startClosed false)', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const opts = mockInstances[0].options;
    expect(opts).toMatchObject({ startClosed: false });
    expect(typeof opts?.maxRetries).toBe('number');
    expect(opts?.maxRetries as number).toBeGreaterThanOrEqual(5);
  });
});

// ---------------------------------------------------------------------------
// subscribedLayers replay on (re)open
// ---------------------------------------------------------------------------

describe('WebSocketClient — subscribedLayers replay on open', () => {
  it('replays a plain viewport layer as subscribe with viewport', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();

    const viewport = { west: -90, south: 25, east: -60, north: 50 };
    client.subscribeLayer('layer-1', viewport);

    // Reconnect: close → partysocket creates a fresh socket → open replays.
    ws1.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();

    expect(ws2.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', layer_id: 'layer-1', data: { viewport } }),
    );
  });

  it('replays a range-mode layer as time_range', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();

    client.sendTimeRange('layer-1', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z');

    ws1.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();

    expect(ws2.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: 'time_range',
        layer_id: 'layer-1',
        data: { from: '2024-01-01T00:00:00Z', to: '2024-01-02T00:00:00Z', viewport: undefined },
      }),
    );
  });

  it('replays a global layer as subscribe with global mode', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();

    client.subscribeLayerGlobal('layer-1');

    ws1.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();

    expect(ws2.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'subscribe', layer_id: 'layer-1', data: { mode: 'global' } }),
    );
  });

  it('replays a global + time_range layer as time_range global', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();

    client.sendTimeRangeGlobal('layer-1', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z');

    ws1.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();

    expect(ws2.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: 'time_range',
        layer_id: 'layer-1',
        data: { from: '2024-01-01T00:00:00Z', to: '2024-01-02T00:00:00Z', mode: 'global' },
      }),
    );
  });

  it('replays a pending notification filter on open', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();

    const filter = { min_attention: 'high', insight_types: ['anomaly'] };
    client.sendNotificationFilter(filter);

    ws1.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();

    expect(ws2.send).toHaveBeenCalledWith(
      JSON.stringify({ type: 'notification_filter', data: filter }),
    );
  });
});

// ---------------------------------------------------------------------------
// StrictMode connect guard
// ---------------------------------------------------------------------------

describe('WebSocketClient — StrictMode connect guard', () => {
  it('does not open a second socket when already connecting', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    // readyState is CONNECTING (0) — second connect() must be a no-op.
    client.connect();
    expect(mockInstances).toHaveLength(1);
  });

  it('does not open a second socket when already open', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    mockInstances[0].simulateOpen();
    client.connect();
    expect(mockInstances).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// disconnect intentional-close semantics
// ---------------------------------------------------------------------------

describe('WebSocketClient — disconnect', () => {
  it('intentional disconnect → disconnected, no reconnecting on close', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();
    client.disconnect();
    expect(client.status).toBe('disconnected');
    expect(ws.close).toHaveBeenCalled();
  });

  it('clears subscribed layers on disconnect', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws1 = mockInstances[0];
    ws1.simulateOpen();
    client.subscribeLayer('layer-1');
    client.disconnect();

    // Reconnect with a fresh socket — nothing should replay.
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.send.mockClear();
    ws2.simulateOpen();
    expect(ws2.send).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// resetAndReconnect (failed-state recovery)
// ---------------------------------------------------------------------------

describe('WebSocketClient — resetAndReconnect', () => {
  it('exists as a public method', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    expect(typeof client.resetAndReconnect).toBe('function');
  });

  it('forces a fresh connection and clears the failed state', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Drive to failed.
    ws.retryCount = 5;
    ws.simulateClose();
    expect(client.status).toBe('failed');

    client.resetAndReconnect();
    expect(ws.reconnect).toHaveBeenCalled();
    expect(client.status).toBe('reconnecting');

    // A subsequent open recovers to connected.
    ws.simulateOpen();
    expect(client.status).toBe('connected');
  });

  it('falls back to connect() when ws is null (Fix 1 — silent no-op hardening)', async () => {
    // Without the fix, resetAndReconnect() on a null ws was a complete no-op:
    // the caller expected recovery but got silence. With the fix, connect() is
    // called, creating a new socket and beginning the reconnect lifecycle.
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    // ws is null — never called connect().
    expect(mockInstances).toHaveLength(0);

    client.resetAndReconnect();
    // connect() must have been called, creating a new socket instance.
    expect(mockInstances).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// Pre-OPEN send buffer (Task 3)
// ---------------------------------------------------------------------------

describe('WebSocketClient — pre-OPEN send buffer', () => {
  it('buffers a send() while CONNECTING and flushes it once on open', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    // socket is CONNECTING (readyState=0) — send should not call ws.send yet
    client.send({
      type: 'viewport_update',
      layer_id: 'l1',
      data: { west: 0, south: 0, east: 1, north: 1 },
    });
    expect(ws.send).not.toHaveBeenCalled();

    ws.simulateOpen();
    // after open the buffered message must have been sent
    expect(ws.send).toHaveBeenCalledWith(
      JSON.stringify({
        type: 'viewport_update',
        layer_id: 'l1',
        data: { west: 0, south: 0, east: 1, north: 1 },
      }),
    );
    // and sent exactly once
    expect(ws.send).toHaveBeenCalledTimes(1);
  });

  it('buffer is bounded at 256 — the 257th message drops the oldest (FIFO)', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];

    // enqueue 257 viewport_update messages while CONNECTING
    for (let i = 0; i < 257; i++) {
      client.send({ type: 'viewport_update', layer_id: `l${i}`, data: { idx: i } });
    }

    ws.simulateOpen();

    // only 256 should have been flushed (oldest i=0 was dropped)
    const calls = ws.send.mock.calls.map((c: string[]) => JSON.parse(c[0]));
    expect(calls).toHaveLength(256);
    // first flushed message is i=1 (i=0 was evicted)
    expect(calls[0]).toMatchObject({ type: 'viewport_update', layer_id: 'l1' });
    // last flushed message is i=256
    expect(calls[255]).toMatchObject({ type: 'viewport_update', layer_id: 'l256' });
  });

  it('buffered messages flush AFTER the subscribedLayers replay (ordering)', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // subscribe a layer while connected
    client.subscribeLayer('layer-A', { west: -1, south: -1, east: 1, north: 1 });
    ws.send.mockClear();

    // close and reconnect — during reconnect send a viewport_update BEFORE open
    ws.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];

    // queue a viewport_update before open fires
    client.send({
      type: 'viewport_update',
      layer_id: 'layer-A',
      data: { west: -2, south: -2, east: 2, north: 2 },
    });

    ws2.simulateOpen();

    const calls = ws2.send.mock.calls.map((c: string[]) => JSON.parse(c[0]));
    // subscribe replay must come before the buffered viewport_update
    const subscribeIdx = calls.findIndex((m: { type: string }) => m.type === 'subscribe');
    const vpIdx = calls.findIndex((m: { type: string }) => m.type === 'viewport_update');
    expect(subscribeIdx).toBeGreaterThanOrEqual(0);
    expect(vpIdx).toBeGreaterThan(subscribeIdx);
  });

  it('replay-owned types (subscribe/unsubscribe/time_range) are NOT buffered and NOT double-sent', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    client.subscribeLayer('layer-X');
    client.sendTimeRange('layer-X', '2024-01-01T00:00:00Z', '2024-01-02T00:00:00Z');
    ws.send.mockClear();

    ws.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];

    // While CONNECTING, manually call send() with replay-owned types
    client.send({ type: 'subscribe', layer_id: 'layer-X', data: {} });
    client.send({ type: 'unsubscribe', layer_id: 'layer-X', data: {} });
    client.send({ type: 'time_range', layer_id: 'layer-X', data: {} });

    ws2.simulateOpen();

    const calls = ws2.send.mock.calls.map((c: string[]) => JSON.parse(c[0]));
    // The replay sends exactly one time_range (the last sendTimeRange call
    // overwrote the subscribe in subscribedLayers).
    const subscribeCalls = calls.filter((m: { type: string }) => m.type === 'subscribe');
    const unsubscribeCalls = calls.filter((m: { type: string }) => m.type === 'unsubscribe');
    const timeRangeCalls = calls.filter(
      (m: { type: string; layer_id?: string }) =>
        m.type === 'time_range' && m.layer_id === 'layer-X',
    );
    // unsubscribe should never be buffered or replayed (layer was re-subscribed)
    expect(unsubscribeCalls).toHaveLength(0);
    // subscribe should NOT appear (sendTimeRange replaced it in subscribedLayers)
    expect(subscribeCalls).toHaveLength(0);
    // time_range should appear exactly once (from replay, NOT double-buffered)
    expect(timeRangeCalls).toHaveLength(1);
  });

  it('notification_filter issued while CONNECTING is NOT buffered — canonical re-send on open yields exactly one frame', async () => {
    // RED without fix: sendNotificationFilter stores to pendingNotificationFilter AND
    // send() buffered the raw message → two notification_filter frames on open.
    // GREEN with fix: notification_filter is bypass-listed; only the canonical
    // pendingNotificationFilter re-send in onopen fires.
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    // socket is still CONNECTING (readyState=0)
    const filter = { min_attention: 'high', insight_types: ['anomaly'] };
    client.sendNotificationFilter(filter);
    // Must NOT have sent anything yet
    expect(ws.send).not.toHaveBeenCalled();

    ws.simulateOpen();

    const calls = ws.send.mock.calls.map((c: string[]) => JSON.parse(c[0]));
    const filterCalls = calls.filter((m: { type: string }) => m.type === 'notification_filter');
    // Exactly one notification_filter frame — the canonical re-send, not a buffered duplicate.
    expect(filterCalls).toHaveLength(1);
    expect(filterCalls[0]).toEqual({ type: 'notification_filter', data: filter });
  });
});

// ---------------------------------------------------------------------------
// Part A — App-level heartbeat (ping / pong watchdog), Task 4
// ---------------------------------------------------------------------------

describe('WebSocketClient — heartbeat (ping/pong watchdog)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('sends a ping ~30s after open', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();
    ws.send.mockClear();

    // No ping yet just before the interval elapses.
    vi.advanceTimersByTime(29_000);
    expect(ws.send).not.toHaveBeenCalled();

    // Ping fires at 30s.
    vi.advanceTimersByTime(1_000);
    const pings = ws.send.mock.calls
      .map((c: string[]) => JSON.parse(c[0]))
      .filter((m: { type: string }) => m.type === 'ping');
    expect(pings).toHaveLength(1);
  });

  it('an inbound pong clears the watchdog — no reconnect', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Ping fires, watchdog armed.
    vi.advanceTimersByTime(30_000);
    // Pong arrives within the 10s window.
    vi.advanceTimersByTime(5_000);
    ws.simulateMessage({ type: 'pong', data: {} });
    // Let the rest of the watchdog window elapse.
    vi.advanceTimersByTime(10_000);

    expect(ws.reconnect).not.toHaveBeenCalled();
  });

  it('forces a reconnect when NO pong arrives within 10s of the ping', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Ping fires at 30s; watchdog armed for 10s.
    vi.advanceTimersByTime(30_000);
    expect(ws.reconnect).not.toHaveBeenCalled();

    // No pong → watchdog fires at +10s, forcing a reconnect.
    vi.advanceTimersByTime(10_000);
    expect(ws.reconnect).toHaveBeenCalled();
  });

  it('does NOT dispatch pong to layer/handler subscribers (internal transport)', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    const pongHandler = vi.fn();
    client.subscribe('pong', pongHandler);

    ws.simulateMessage({ type: 'pong', data: {} });
    expect(pongHandler).not.toHaveBeenCalled();
  });

  it('clears heartbeat timers on disconnect — no ping after close', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();
    ws.send.mockClear();

    client.disconnect();

    // Advance well past the ping interval — no ping must fire.
    vi.advanceTimersByTime(60_000);
    const pings = ws.send.mock.calls
      .map((c: string[]) => JSON.parse(c[0]))
      .filter((m: { type: string }) => m.type === 'ping');
    expect(pings).toHaveLength(0);
  });

  it('clears the watchdog on close so no reconnect fires after disconnect', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Arm the watchdog with a ping.
    vi.advanceTimersByTime(30_000);
    // Disconnect before the pong window elapses.
    client.disconnect();

    vi.advanceTimersByTime(10_000);
    expect(ws.reconnect).not.toHaveBeenCalled();
  });

  it('missed-pong path clears the watchdog timer (no leaked duplicate watchdog)', async () => {
    // Verifies Fix 3 + Fix 4a: after a missed-pong-driven reconnect() the
    // heartbeatTimeoutId is null, so no stale watchdog can double-fire.
    // Without clearHeartbeatTimeout() at the top of sendPing() a second ping
    // in the same interval could arm a second concurrent timer.
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // First ping fires + watchdog armed.
    vi.advanceTimersByTime(30_000);
    // Let watchdog expire (missed pong) → reconnect() is called.
    vi.advanceTimersByTime(10_000);
    expect(ws.reconnect).toHaveBeenCalledTimes(1);

    // Simulate the reconnect closing the socket (onclose fires, stopHeartbeat runs).
    ws.simulateClose();
    // Advance well beyond another watchdog window — no second stale fire.
    vi.advanceTimersByTime(20_000);
    // reconnect must not have been called a second time from a leaked watchdog.
    expect(ws.reconnect).toHaveBeenCalledTimes(1);
  });

  it('heartbeat interval is re-armed exactly once across open→close→reopen', async () => {
    // Fix 4b: verifies no duplicate interval stacking after a reconnect.
    // startHeartbeat() calls stopHeartbeat() first (idempotent), so only one
    // interval should be active at any time — exactly one ping per 30s window.
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();
    ws.send.mockClear();

    // Close and reopen (simulates a reconnect cycle).
    ws.simulateClose();
    client.connect();
    const ws2 = mockInstances[mockInstances.length - 1];
    ws2.simulateOpen();
    ws2.send.mockClear();

    // Advance exactly one 30s window.
    vi.advanceTimersByTime(30_000);

    const pings = ws2.send.mock.calls
      .map((c: string[]) => JSON.parse(c[0]))
      .filter((m: { type: string }) => m.type === 'ping');
    // Exactly one ping — not two from a stacked interval.
    expect(pings).toHaveLength(1);
  });
});

// ---------------------------------------------------------------------------
// Part B — Auto-recovery from the 'failed' state, Task 4
// ---------------------------------------------------------------------------

describe('WebSocketClient — failed-state auto-recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('automatically resetAndReconnect()s after the cooldown once failed', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // Drive into failed.
    ws.retryCount = 5;
    ws.simulateClose();
    expect(client.status).toBe('failed');
    expect(ws.reconnect).not.toHaveBeenCalled();

    // Cooldown elapses → automatic recovery without a reload.
    vi.advanceTimersByTime(30_000);
    expect(ws.reconnect).toHaveBeenCalled();
    expect(client.status).toBe('reconnecting');
  });

  it('a disconnect() during the cooldown cancels the scheduled recovery', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    ws.retryCount = 5;
    ws.simulateClose();
    expect(client.status).toBe('failed');

    // Intentional disconnect mid-cooldown must cancel the pending recovery.
    client.disconnect();
    vi.advanceTimersByTime(30_000);
    expect(ws.reconnect).not.toHaveBeenCalled();
    expect(client.status).toBe('disconnected');
  });

  it('a successful reconnect clears the pending recovery timer', async () => {
    const { WebSocketClient } = await importFresh();
    const client = new WebSocketClient();
    client.connect();
    const ws = mockInstances[0];
    ws.simulateOpen();

    // failed → schedules recovery.
    ws.retryCount = 5;
    ws.simulateClose();
    expect(client.status).toBe('failed');

    // Recover before the cooldown via an open event.
    ws.simulateOpen();
    expect(client.status).toBe('connected');

    // The previously-scheduled recovery must NOT fire now.
    vi.advanceTimersByTime(30_000);
    expect(ws.reconnect).not.toHaveBeenCalled();
  });
});
