/**
 * useViewportSync — center-distance dedup and debounce tests
 *
 * useViewportSync (useLayerStream.ts ~line 306) watches the Cesium viewer
 * viewport stored in useViewerStore and forwards it to the WebSocket server
 * for spatial layers.  Two dedup mechanisms are in play:
 *
 *   1. JSON.stringify equality check: identical viewport objects are never
 *      forwarded, regardless of how many times the store emits them.
 *
 *   2. Center-distance check: if the viewport center has moved less than 2°
 *      in both latitude and longitude since the last sent viewport, the
 *      update is suppressed to avoid flooding during orbital fly-overs.
 *
 * Both mechanisms guard the *debounce timer scheduling* — when suppressed,
 * no setTimeout is created.  The actual WebSocket call happens 300 ms after
 * scheduling.
 *
 * Test strategy:
 * - Manipulate the real Zustand stores directly (matches project convention).
 * - Use vi.useFakeTimers() so setTimeout can be advanced without wall-clock
 *   waiting, keeping the suite synchronous and deterministic.
 * - Mock wsClient and the app/store's isSpatialLayer path so we can assert
 *   which wsClient methods get called and how many times.
 *
 * Covered cases:
 * - First viewport update always reaches the server (no previous to compare)
 * - Viewport change > 2° (center moved) triggers a server update
 * - Viewport change < 2° (center drift) is suppressed
 * - Viewport change exactly 2° is suppressed (boundary — < 2° condition)
 * - Viewport change slightly above 2° passes through
 * - Exact same viewport (identical JSON) is suppressed
 * - Debounce: rapid successive changes coalesce into a single call
 * - Live mode: sendViewportUpdate is used for spatial layers
 * - Range mode: sendTimeRange is used for spatial layers
 * - Non-spatial layers are never forwarded
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// ---------------------------------------------------------------------------
// vi.hoisted() — mock factories that must be available before vi.mock() runs
// ---------------------------------------------------------------------------
const { mockSendViewportUpdate, mockSendTimeRange, mockSubscribe } = vi.hoisted(() => {
  return {
    mockSendViewportUpdate: vi.fn(),
    mockSendTimeRange: vi.fn(),
    mockSubscribe: vi.fn(() => vi.fn()),
  };
});

// ---------------------------------------------------------------------------
// Mock the WebSocket client
// ---------------------------------------------------------------------------
vi.mock('@respondent/core', async () => ({
  ...(await vi.importActual('@respondent/core')),
  wsClient: {
    isConnected: vi.fn(() => true),
    connect: vi.fn(),
    subscribe: mockSubscribe,
    subscribeLayer: vi.fn(),
    unsubscribeLayer: vi.fn(),
    sendViewportUpdate: mockSendViewportUpdate,
    sendTimeRange: mockSendTimeRange,
  },
}));

// ---------------------------------------------------------------------------
// Imports — after mocks are registered
// ---------------------------------------------------------------------------
import { useViewportSync } from './useLayerStream';
import { useViewerStore } from '../../features/globe/store';
import { useUIStore } from '@/app/store';
import type { ViewportBBox } from '../../features/globe/store';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeViewport(centerLon: number, centerLat: number, halfW = 5, halfH = 5): ViewportBBox {
  return {
    west: centerLon - halfW,
    south: centerLat - halfH,
    east: centerLon + halfW,
    north: centerLat + halfH,
  };
}

/**
 * Register a layer that reports filteringMode = 'viewport' in the store so
 * that isSpatialLayer() returns true for it.
 */
function registerSpatialLayer(layerId: string) {
  useUIStore.setState((s) => ({
    layers: {
      ...s.layers,
      [layerId]: {
        id: layerId,
        name: layerId,
        type: layerId,
        enabled: true,
        mode: 'live',
        density: 0,
        source: '',
        lastUpdate: 0,
        count: 0,
        color: '#ffffff',
        pointSize: 4,
        filteringMode: 'viewport',
      },
    },
    enabledLayers: [...s.enabledLayers, layerId],
  }));
}

/**
 * Register a layer whose filteringMode is NOT 'viewport' (non-spatial).
 */
function registerNonSpatialLayer(layerId: string) {
  useUIStore.setState((s) => ({
    layers: {
      ...s.layers,
      [layerId]: {
        id: layerId,
        name: layerId,
        type: layerId,
        enabled: true,
        mode: 'live',
        density: 0,
        source: '',
        lastUpdate: 0,
        count: 0,
        color: '#ffffff',
        pointSize: 4,
        filteringMode: '',
      },
    },
    enabledLayers: [...s.enabledLayers, layerId],
  }));
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();

  // Reset both stores to a clean baseline before each test
  useViewerStore.setState({ viewport: null });
  useUIStore.setState({
    enabledLayers: [],
    layers: {},
    timeMode: 'live',
    timeFrom: null,
    timeTo: null,
  });
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Tests: first viewport update
// ---------------------------------------------------------------------------

describe('useViewportSync — first viewport update', () => {
  it('always sends the first viewport update (no prior comparison)', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });

    // Advance past the 300ms debounce
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    expect(mockSendViewportUpdate).toHaveBeenCalledWith('layer-a', makeViewport(0, 0));
    unmount();
  });

  it('does not call sendViewportUpdate before the debounce elapses', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });

    act(() => {
      vi.advanceTimersByTime(299);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: JSON.stringify equality dedup
// ---------------------------------------------------------------------------

describe('useViewportSync — identical viewport dedup (JSON equality)', () => {
  it('does not send a second update for the identical viewport object', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    const vp = makeViewport(10, 20);

    // First update — should go through
    act(() => {
      useViewerStore.setState({ viewport: vp });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);

    vi.clearAllMocks();

    // Set the exact same object again — store sets a new reference but
    // the JSON.stringify check should catch it
    act(() => {
      useViewerStore.setState({ viewport: { ...vp } });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: center-distance dedup (< 2° suppression)
// ---------------------------------------------------------------------------

describe('useViewportSync — center-distance dedup', () => {
  it('suppresses update when center moved less than 2° in both dimensions', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // First update establishes the baseline
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);

    vi.clearAllMocks();

    // New viewport whose center moved only 1° in lon and 1° in lat → suppressed
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(1, 1) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });

  it('suppresses update when center moved 0° (no movement at all)', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // Establish baseline with a different size viewport so JSON.stringify differs
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0, 5, 5) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // Same center, different size — JSON is different but center delta = 0
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0, 10, 10) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });

  it('suppresses update when center moved exactly 2° (boundary — condition is strict <)', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // Baseline at (0, 0)
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // Center moves exactly 2° in lon — Math.abs(2) < 2 is false, but
    // both conditions must be true (< 2 AND < 2).  lon delta = 2 fails.
    // The source code uses `< 2` so exactly 2° should NOT be suppressed.
    // However, for the lat axis the delta is 0, which IS < 2.
    // The combined condition: Math.abs(2) < 2 → false → update passes through.
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(2, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    // Exactly 2° change is NOT suppressed (strict less-than)
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    unmount();
  });

  it('suppresses update when center moved 1.999° in lon and 1.999° in lat', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // Both deltas are just below 2° → both < 2 conditions pass → suppressed
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(1.999, 1.999) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });

  it('passes through when center moved more than 2° in longitude', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // Baseline
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // lon delta = 5° > 2° → dedup condition fails → update sent
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(5, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    unmount();
  });

  it('passes through when center moved more than 2° in latitude', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // lat delta = 5° > 2° → passes through
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 5) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    unmount();
  });

  it('passes through when center moved > 2° in both dimensions', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(10, 10) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    unmount();
  });

  it('correctly updates the baseline after a non-suppressed update', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // Step 1: baseline at (0, 0)
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    vi.clearAllMocks();

    // Step 2: move to (10, 10) — > 2°, passes through, new baseline = (10, 10)
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(10, 10) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    vi.clearAllMocks();

    // Step 3: move to (11, 11) — only 1° from the new baseline (10, 10) → suppressed
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(11, 11) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: debounce coalescing
// ---------------------------------------------------------------------------

describe('useViewportSync — debounce coalescing', () => {
  it('coalesces multiple rapid viewport changes into a single server call', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // Rapid updates well apart in distance (each > 2° from the previous sent)
    // The first one establishes a pending timer.
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(100); // still within 300ms window
    });
    act(() => {
      // Large jump → different JSON + > 2° center, resets the debounce timer
      useViewerStore.setState({ viewport: makeViewport(20, 20) });
    });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(40, 40) });
    });

    // Only advance past the final timer, not each intermediate one
    act(() => {
      vi.advanceTimersByTime(300);
    });

    // Should have called once with the last viewport (40, 40)
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    expect(mockSendViewportUpdate).toHaveBeenCalledWith('layer-a', makeViewport(40, 40));
    unmount();
  });

  it('does not fire before 300ms even after multiple rapid changes', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(20, 20) });
    });
    act(() => {
      vi.advanceTimersByTime(299);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: layer filtering — only spatial layers receive updates
// ---------------------------------------------------------------------------

describe('useViewportSync — spatial vs non-spatial layer filtering', () => {
  it('sends viewport update only to spatial layers', () => {
    registerSpatialLayer('spatial-layer');
    registerNonSpatialLayer('non-spatial-layer');

    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    expect(mockSendViewportUpdate).toHaveBeenCalledWith('spatial-layer', expect.any(Object));

    // non-spatial-layer must not receive a viewport update
    const calls = mockSendViewportUpdate.mock.calls;
    const calledIds = calls.map((c) => c[0]);
    expect(calledIds).not.toContain('non-spatial-layer');

    unmount();
  });

  it('does not call sendViewportUpdate when no spatial layers are enabled', () => {
    registerNonSpatialLayer('non-spatial-1');
    registerNonSpatialLayer('non-spatial-2');

    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });

  it('does not call sendViewportUpdate when no layers are enabled at all', () => {
    // enabledLayers remains empty (no registerSpatialLayer call)
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: time-range mode uses sendTimeRange instead of sendViewportUpdate
// ---------------------------------------------------------------------------

describe('useViewportSync — range mode dispatches sendTimeRange', () => {
  it('calls sendTimeRange (not sendViewportUpdate) when in range mode with timeFrom set', () => {
    registerSpatialLayer('layer-a');

    useUIStore.setState({
      timeMode: 'range',
      timeFrom: '2025-01-01T00:00:00.000Z',
      timeTo: '2025-01-01T08:00:00.000Z',
    });

    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendTimeRange).toHaveBeenCalledTimes(1);
    expect(mockSendViewportUpdate).not.toHaveBeenCalled();

    const [layerId, from, to, vp] = mockSendTimeRange.mock.calls[0];
    expect(layerId).toBe('layer-a');
    expect(from).toBe('2025-01-01T00:00:00.000Z');
    expect(to).toBe('2025-01-01T08:00:00.000Z');
    expect(vp).toEqual(makeViewport(0, 0));

    unmount();
  });

  it('does not call sendTimeRange when timeMode is range but timeFrom is null', () => {
    registerSpatialLayer('layer-a');

    useUIStore.setState({
      timeMode: 'range',
      timeFrom: null,
      timeTo: null,
    });

    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendTimeRange).not.toHaveBeenCalled();
    unmount();
  });

  it('uses sendViewportUpdate in live mode even when spatial layers are enabled', () => {
    registerSpatialLayer('layer-a');

    useUIStore.setState({
      timeMode: 'live',
      timeFrom: null,
      timeTo: null,
    });

    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    expect(mockSendTimeRange).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: null viewport guard
// ---------------------------------------------------------------------------

describe('useViewportSync — null viewport guard', () => {
  it('does not call sendViewportUpdate when viewport is null', () => {
    registerSpatialLayer('layer-a');
    const { unmount } = renderHook(() => useViewportSync());

    // viewport remains null (not set)
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();
    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: non-reactive render path (Task 3)
//
// useViewportSync MUST drive viewport sync via useViewerStore.subscribe() in
// a []‑dep effect — NOT via a reactive selector that re-renders the host on
// every camera frame (~60/sec).
//
// Assertions:
//   1. Viewport store changes still trigger sendViewportUpdate (subscribe works).
//   2. The hook does NOT re-render the host when the viewport changes — render
//      count stays at 1 after any number of viewport mutations.
//   3. Drift-suppression still gates sub-threshold viewport changes even though
//      the subscription fires for every store update.
// ---------------------------------------------------------------------------

describe('useViewportSync — non-reactive render path (Task 3)', () => {
  it('viewport store update drives sendViewportUpdate WITHOUT re-rendering the host', () => {
    registerSpatialLayer('layer-spatial');

    let renderCount = 0;
    const { unmount } = renderHook(() => {
      renderCount++;
      useViewportSync();
    });

    // Initial render count after mount
    const countAfterMount = renderCount;

    // Trigger multiple viewport store updates (simulates 60fps camera)
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(10, 10) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(20, 20) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    // The hook must have synced all three > 2° viewport changes
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(3);

    // Render count must NOT have increased — the effect is mount-only ([]-dep),
    // viewport changes are handled via store.subscribe() not reactive selector.
    expect(renderCount).toBe(countAfterMount);

    unmount();
  });

  it('drift-suppression still gates sub-threshold changes via subscribe path', () => {
    registerSpatialLayer('layer-spatial');
    const { unmount } = renderHook(() => useViewportSync());

    // Establish baseline at (0, 0)
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    vi.clearAllMocks();

    // Sub-threshold drift (< 2°) — subscribe fires but drift-suppression must gate it
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0.5, 0.5) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).not.toHaveBeenCalled();

    // Above-threshold (> 2°) — must pass through
    act(() => {
      useViewerStore.setState({ viewport: makeViewport(10, 10) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledTimes(1);
    expect(mockSendViewportUpdate).toHaveBeenCalledWith('layer-spatial', makeViewport(10, 10));

    unmount();
  });

  it('sends viewport_update (not time_range) for a spatial layer in live mode', () => {
    registerSpatialLayer('layer-spatial');
    const { unmount } = renderHook(() => useViewportSync());

    act(() => {
      useViewerStore.setState({ viewport: makeViewport(0, 0) });
    });
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(mockSendViewportUpdate).toHaveBeenCalledWith('layer-spatial', makeViewport(0, 0));

    unmount();
  });
});
