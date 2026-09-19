import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { wsClient } from '@respondent/core';

// ---------------------------------------------------------------------------
// vi.hoisted() — mock factories that must be available before vi.mock() runs
// ---------------------------------------------------------------------------
const { mockSubscribe, mockSetLayerEntities, mockReplaceLayerEntities } = vi.hoisted(() => {
  return {
    mockSubscribe: vi.fn(() => vi.fn()),
    mockSetLayerEntities: vi.fn(),
    mockReplaceLayerEntities: vi.fn(),
  };
});

// Mock the websocket client
vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual('@respondent/core');
  const subscribeLayer = vi.fn();
  const sendTimeRange = vi.fn();
  const unsubscribeLayer = vi.fn();
  const sendViewportUpdate = vi.fn();
  const connect = vi.fn();
  const isConnected = vi.fn(() => true);

  return {
    ...actual,
    wsClient: {
      subscribeLayer,
      sendTimeRange,
      unsubscribeLayer,
      sendViewportUpdate,
      connect,
      isConnected,
      subscribe: mockSubscribe,
    },
  };
});

// Mock the app store so we can control setLayerEntities / replaceLayerEntities
vi.mock('@/app/store', async () => {
  const actual = await vi.importActual('@/app/store');
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const { useUIStore } = actual as any;

  // Wrap useUIStore: intercept selector calls for the action functions
  // Cast to the real store type so static-method assignments (.getState/.setState/
  // .subscribe/.destroy) are well-typed and tsc does not reject them.
  const wrappedUseUIStore = vi.fn((selector: (s: unknown) => unknown) => {
    const state = useUIStore.getState();
    const augmented = {
      ...state,
      setLayerEntities: mockSetLayerEntities,
      replaceLayerEntities: mockReplaceLayerEntities,
    };
    return selector(augmented);
  }) as unknown as typeof useUIStore;

  // Forward static methods from the real store
  Object.assign(wrappedUseUIStore, useUIStore);
  // Override getState to inject mock actions too
  wrappedUseUIStore.getState = () => ({
    ...useUIStore.getState(),
    setLayerEntities: mockSetLayerEntities,
    replaceLayerEntities: mockReplaceLayerEntities,
  });
  wrappedUseUIStore.setState = useUIStore.setState;
  wrappedUseUIStore.subscribe = useUIStore.subscribe;
  wrappedUseUIStore.destroy = useUIStore.destroy;

  return {
    ...actual,
    useUIStore: wrappedUseUIStore,
  };
});

// ---------------------------------------------------------------------------
// Imports — after mocks
// ---------------------------------------------------------------------------
import { useLayerStream } from './useLayerStream';
import { useUIStore } from '@/app/store';

// ---------------------------------------------------------------------------
// Helpers: capture the message handlers registered via wsClient.subscribe
// ---------------------------------------------------------------------------
type MessageHandler = (msg: Record<string, unknown>) => void;

function getMessageHandler(eventType: string): MessageHandler {
  // mockSubscribe is called as: wsClient.subscribe('layer.batch_update', handleMessage)
  const calls = mockSubscribe.mock.calls as unknown as Array<[string, MessageHandler]>;
  const match = calls.find(([type]) => type === eventType);
  if (!match) throw new Error(`No subscribe call found for event type: ${eventType}`);
  return match[1];
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
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
// Tests: basic wsClient mock availability
// ---------------------------------------------------------------------------

describe('useLayerStream live mode behavior', () => {
  it('subscribeLayer is available on wsClient', () => {
    expect(typeof wsClient.subscribeLayer).toBe('function');
  });

  it('sendTimeRange is available on wsClient', () => {
    expect(typeof wsClient.sendTimeRange).toBe('function');
  });
});

// ---------------------------------------------------------------------------
// Tests: rAF coalescing for bulk writes (Task 4)
// ---------------------------------------------------------------------------

describe('useLayerStream — rAF bulk coalescing (Task 4)', () => {
  it('coalesces two layer.batch_update messages for the same layer into ONE store write per frame', () => {
    const { unmount } = renderHook(() => useLayerStream());

    const handler = getMessageHandler('layer.batch_update');

    act(() => {
      // Two batch_update messages for the same layer within one frame
      handler({
        type: 'layer.batch_update',
        layer_id: 'layer-abc',
        data: {
          updates: [{ entity: { id: 'e1', name: 'Entity 1' } }],
        },
      });
      handler({
        type: 'layer.batch_update',
        layer_id: 'layer-abc',
        data: {
          updates: [{ entity: { id: 'e2', name: 'Entity 2' } }],
        },
      });
      // No rAF flush yet — store setter should NOT have been called
    });

    // Store must not be written synchronously
    expect(mockSetLayerEntities).not.toHaveBeenCalled();
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();

    // Flush the rAF
    act(() => {
      vi.runAllTimers();
    });

    // After flush: setLayerEntities called ONCE (not twice), replaceLayerEntities not at all
    expect(mockSetLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();
    // The single call must include both entities coalesced
    const [calledLayerId, calledData] = mockSetLayerEntities.mock.calls[0];
    expect(calledLayerId).toBe('layer-abc');
    expect(calledData.entities).toHaveLength(2);

    unmount();
  });

  it('calls replaceLayerEntities (not setLayerEntities) for a layer.snapshot message on flush', () => {
    const { unmount } = renderHook(() => useLayerStream());

    const handler = getMessageHandler('layer.snapshot');

    act(() => {
      handler({
        type: 'layer.snapshot',
        layer_id: 'layer-snap',
        data: {
          entities: [
            { id: 's1', name: 'Snap 1' },
            { id: 's2', name: 'Snap 2' },
          ],
          observations: [],
        },
      });
    });

    // Must not write synchronously
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();
    expect(mockSetLayerEntities).not.toHaveBeenCalled();

    act(() => {
      vi.runAllTimers();
    });

    // Must call replaceLayerEntities — atomic snapshot, never merge
    expect(mockReplaceLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockSetLayerEntities).not.toHaveBeenCalled();
    const [layerId, data] = mockReplaceLayerEntities.mock.calls[0];
    expect(layerId).toBe('layer-snap');
    expect(data.entities).toHaveLength(2);

    unmount();
  });

  it('calls replaceLayerEntities when a snapshot chunk is mixed with batch_update chunks in one frame', () => {
    const { unmount } = renderHook(() => useLayerStream());

    const batchHandler = getMessageHandler('layer.batch_update');
    const snapHandler = getMessageHandler('layer.snapshot');

    act(() => {
      batchHandler({
        type: 'layer.batch_update',
        layer_id: 'layer-mixed',
        data: { updates: [{ entity: { id: 'e1' } }] },
      });
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-mixed',
        data: { entities: [{ id: 's1' }], observations: [] },
      });
    });

    act(() => {
      vi.runAllTimers();
    });

    // Snapshot wins — must replace, not merge
    expect(mockReplaceLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockSetLayerEntities).not.toHaveBeenCalled();

    unmount();
  });

  it('does NOT buffer layer.batch_update when timeMode is range (frozen guard)', () => {
    useUIStore.setState({ timeMode: 'range', timeTo: '2025-01-01T08:00:00.000Z' });

    const { unmount } = renderHook(() => useLayerStream());

    const handler = getMessageHandler('layer.batch_update');

    act(() => {
      handler({
        type: 'layer.batch_update',
        layer_id: 'layer-frozen',
        data: { updates: [{ entity: { id: 'e1' } }] },
      });
    });

    act(() => {
      vi.runAllTimers();
    });

    // Frozen guard: no store write at all
    expect(mockSetLayerEntities).not.toHaveBeenCalled();
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();

    unmount();
  });
});

// ---------------------------------------------------------------------------
// Tests: persistent viewport loading (Task 2)
// ---------------------------------------------------------------------------

import { setLayerSnapshotDisposition } from './useLayerStream';

describe('useLayerStream — persistent viewport loading (Task 2)', () => {
  it('routes a viewport-pan snapshot through MERGE (prior entities retained)', () => {
    const { unmount } = renderHook(() => useLayerStream());

    // Simulate a viewport_update having been sent for this layer — the next
    // snapshot for it must MERGE so the prior viewport's working set persists.
    setLayerSnapshotDisposition('layer-pan', 'merge');

    const snapHandler = getMessageHandler('layer.snapshot');
    act(() => {
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-pan',
        data: { entities: [{ id: 'p1' }, { id: 'p2' }], observations: [] },
      });
    });
    act(() => {
      vi.runAllTimers();
    });

    // Merge path — setLayerEntities, NOT replaceLayerEntities.
    expect(mockSetLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();
    expect(mockSetLayerEntities.mock.calls[0][0]).toBe('layer-pan');

    unmount();
  });

  it('routes an initial-subscribe snapshot through REPLACE (atomic, default disposition)', () => {
    const { unmount } = renderHook(() => useLayerStream());

    // No viewport_update sent → disposition defaults to replace.
    const snapHandler = getMessageHandler('layer.snapshot');
    act(() => {
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-init',
        data: { entities: [{ id: 'i1' }], observations: [] },
      });
    });
    act(() => {
      vi.runAllTimers();
    });

    expect(mockReplaceLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockSetLayerEntities).not.toHaveBeenCalled();
    expect(mockReplaceLayerEntities.mock.calls[0][0]).toBe('layer-init');

    unmount();
  });

  it('defaults a viewport-progressive layer to MERGE so server follow-up snapshots accumulate', () => {
    // Regression: viewport layers receive an ONGOING stream of per-viewport
    // snapshots (geo cache, backfill, on-demand re-poll ticker). The explicit
    // 'merge' set on pan is one-shot; after it is consumed, follow-up snapshots
    // fall through to the default. That default MUST be 'merge' for viewport
    // layers — otherwise each follow-up snapshot wipes the prior viewport's
    // entities and panning loses the original region.
    useUIStore.setState((s) => ({
      layers: {
        ...s.layers,
        'layer-vp': {
          id: 'layer-vp',
          name: 'layer-vp',
          type: 'layer-vp',
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
    }));

    const { unmount } = renderHook(() => useLayerStream());

    // No explicit disposition set → the layer-aware default applies.
    const snapHandler = getMessageHandler('layer.snapshot');
    act(() => {
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-vp',
        data: { entities: [{ id: 'v1' }], observations: [] },
      });
    });
    act(() => {
      vi.runAllTimers();
    });

    // Merge path — accumulate, never wipe the prior viewport.
    expect(mockSetLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockReplaceLayerEntities).not.toHaveBeenCalled();
    expect(mockSetLayerEntities.mock.calls[0][0]).toBe('layer-vp');

    unmount();
  });

  it('routes a time_range snapshot through REPLACE even after a prior viewport merge', () => {
    const { unmount } = renderHook(() => useLayerStream());

    // A pan set merge…
    setLayerSnapshotDisposition('layer-tr', 'merge');
    // …then a time_range request resets disposition to replace (window changed).
    setLayerSnapshotDisposition('layer-tr', 'replace');

    const snapHandler = getMessageHandler('layer.snapshot');
    act(() => {
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-tr',
        data: { entities: [{ id: 't1' }], observations: [] },
      });
    });
    act(() => {
      vi.runAllTimers();
    });

    expect(mockReplaceLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockSetLayerEntities).not.toHaveBeenCalled();

    unmount();
  });

  it('snapshotDisposition.clear() is called on WS effect cleanup — stale disposition does not survive disconnect', () => {
    // Set a pending 'merge' disposition before mounting
    setLayerSnapshotDisposition('layer-stale', 'merge');

    const { unmount } = renderHook(() => useLayerStream());

    // Unmount (simulates WS disconnect / component teardown)
    unmount();

    // After cleanup, mounting again and receiving a snapshot should use the
    // default 'replace' disposition — the stale 'merge' was cleared.
    const { unmount: unmount2 } = renderHook(() => useLayerStream());
    const snapHandler = getMessageHandler('layer.snapshot');

    act(() => {
      snapHandler({
        type: 'layer.snapshot',
        layer_id: 'layer-stale',
        data: { entities: [{ id: 'x1' }], observations: [] },
      });
    });
    act(() => {
      vi.runAllTimers();
    });

    // Default 'replace' must be used — not the stale 'merge'
    expect(mockReplaceLayerEntities).toHaveBeenCalledTimes(1);
    expect(mockSetLayerEntities).not.toHaveBeenCalled();

    unmount2();
  });

  it('re-subscribes on mount for already-enabled layers WITHOUT clearing entity data', () => {
    useUIStore.setState({ enabledLayers: ['layer-persist'], timeMode: 'live' });
    const subscribeLayer = wsClient.subscribeLayer as ReturnType<typeof vi.fn>;
    const clearLayerEntities = useUIStore.getState().clearLayerEntities;
    const clearSpy = vi.fn();
    useUIStore.setState({ clearLayerEntities: clearSpy });

    const { unmount } = renderHook(() => useLayerStream());

    // Mount must re-establish the subscription for the already-enabled layer…
    const subscribedLayerIds = subscribeLayer.mock.calls.map((c) => c[0]);
    expect(subscribedLayerIds).toContain('layer-persist');
    // …but must NOT clear the already-loaded entities for an existing layer.
    expect(clearSpy).not.toHaveBeenCalledWith('layer-persist');

    useUIStore.setState({ clearLayerEntities });
    unmount();
  });
});
