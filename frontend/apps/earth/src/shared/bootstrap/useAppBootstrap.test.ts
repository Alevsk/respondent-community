/**
 * useAppBootstrap — bootstrap orchestrator hook tests.
 *
 * Verifies the hook coordinates layers, notification backfill, and WebSocket
 * connection into a unified BootstrapState consumed by BootstrapGate.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';

// ---------------------------------------------------------------------------
// Hoisted mock variables — vi.hoisted() ensures these are declared above
// the hoisted vi.mock() calls so factories can reference them.
// ---------------------------------------------------------------------------

const {
  mockConnect,
  mockSendNotificationFilter,
  mockSubscribe,
  mockWsStatusRef,
  mockApiGet,
  mockSetLayers,
  mockMergeNotifications,
  mockAddNotification,
  mockNotificationFilter,
} = vi.hoisted(() => ({
  mockConnect: vi.fn(),
  mockSendNotificationFilter: vi.fn(),
  mockSubscribe: vi.fn(() => vi.fn()),
  mockWsStatusRef: { current: 'disconnected' as 'connected' | 'disconnected' | 'reconnecting' },
  mockApiGet: vi.fn(),
  mockSetLayers: vi.fn(),
  mockMergeNotifications: vi.fn(),
  mockAddNotification: vi.fn(),
  mockNotificationFilter: { minAttention: 'medium', insightTypes: [] as string[] },
}));

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@respondent/core');
  return {
    ...actual,
    wsClient: {
      connect: mockConnect,
      sendNotificationFilter: mockSendNotificationFilter,
      subscribe: mockSubscribe,
    },
    useWebSocketStatus: () => mockWsStatusRef.current,
    api: { get: mockApiGet },
    endpoints: {
      layers: 'http://test/v1/layers',
      aiInsights: 'http://test/v1/ai/insights',
    },
    normalizeInsight: (raw: Record<string, unknown>) => ({ id: raw.id, ...raw }),
  };
});

vi.mock('@/app/store', () => ({
  useUIStore: Object.assign(
    // Selector call form: useUIStore(selector)
    (selector: (s: Record<string, unknown>) => unknown) => {
      const state: Record<string, unknown> = {
        mergeNotifications: mockMergeNotifications,
        addNotification: mockAddNotification,
        notificationFilter: mockNotificationFilter,
      };
      return selector(state);
    },
    {
      getState: () => ({
        setLayers: mockSetLayers,
        notificationFilter: mockNotificationFilter,
      }),
    },
  ),
}));

vi.mock('../../features/globe/icons/iconRegistry', () => ({
  registerDynamicIcon: vi.fn(),
}));

vi.mock('../../features/entity/tabs/overview/fieldRenderers', () => ({
  registerDynamicRenderers: vi.fn(),
}));

// ---------------------------------------------------------------------------
// Import module under test AFTER mocks
// ---------------------------------------------------------------------------

import { useAppBootstrap } from './useAppBootstrap';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useAppBootstrap', () => {
  beforeEach(() => {
    mockWsStatusRef.current = 'disconnected';
    mockConnect.mockClear();
    mockSendNotificationFilter.mockClear();
    mockSubscribe.mockClear().mockReturnValue(vi.fn());
    mockApiGet.mockReset();
    mockSetLayers.mockClear();
    mockMergeNotifications.mockClear();
    mockAddNotification.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('exposes 3 bootstrap steps with correct keys', () => {
    mockApiGet.mockResolvedValue({ layers: [] });
    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    expect(result.current.steps).toHaveLength(3);
    expect(result.current.steps.map((s) => s.key)).toEqual([
      'layers',
      'notifications',
      'websocket',
    ]);
  });

  it('ready=false while queries are loading', () => {
    // Never-resolving promises to keep loading state
    mockApiGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    expect(result.current.ready).toBe(false);
  });

  it('currentLabel matches the first loading step', () => {
    mockApiGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    expect(result.current.currentLabel).toBe('LOADING LAYERS...');
  });

  it('ready=true when all loads succeed and WS is connected', async () => {
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('layers')) return Promise.resolve({ layers: [] });
      return Promise.resolve({ insights: [], totalCount: 0 });
    });
    mockWsStatusRef.current = 'connected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.ready).toBe(true);
    });
    expect(result.current.error).toBe(false);
  });

  it('error=true when a query exhausts retries', async () => {
    vi.useFakeTimers();
    mockApiGet.mockRejectedValue(new Error('network failure'));
    mockWsStatusRef.current = 'connected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    // The hook uses retry: 3 with exponential backoff (1s, 2s, 4s).
    // Advance fake timers through all retry delays so the queries exhaust retries.
    for (let i = 0; i < 5; i++) {
      await vi.advanceTimersByTimeAsync(5_000);
    }

    expect(result.current.error).toBe(true);
    vi.useRealTimers();
  });

  it('retry is a function', () => {
    mockApiGet.mockResolvedValue({ layers: [] });
    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    expect(typeof result.current.retry).toBe('function');
  });

  it('WS disconnected means not ready even when queries succeed', async () => {
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('layers')) return Promise.resolve({ layers: [] });
      return Promise.resolve({ insights: [], totalCount: 0 });
    });
    mockWsStatusRef.current = 'disconnected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    await waitFor(() => {
      // Both queries should succeed
      const layersStep = result.current.steps.find((s) => s.key === 'layers');
      expect(layersStep?.status).toBe('success');
    });

    // But not ready because WS is disconnected
    expect(result.current.ready).toBe(false);
  });

  it('calls wsClient.connect() on mount', () => {
    mockApiGet.mockResolvedValue({ layers: [] });
    renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    expect(mockConnect).toHaveBeenCalled();
  });

  it('calls setLayers on the store when layers query succeeds', async () => {
    const mockLayers = [
      {
        id: 'l1',
        name: 'Layer 1',
        type: 'test',
        enabled: true,
        mode: 'realtime',
        density: 50,
        source: 'test',
        last_update: 0,
        count: 10,
        color: '#fff',
        pointSize: 4,
      },
    ];
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('layers')) return Promise.resolve({ layers: mockLayers });
      return Promise.resolve({ insights: [], totalCount: 0 });
    });
    mockWsStatusRef.current = 'connected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.steps.find((s) => s.key === 'layers')?.status).toBe('success');
    });

    expect(mockSetLayers).toHaveBeenCalled();
  });

  it('calls mergeNotifications on the store when notifications query succeeds', async () => {
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('layers')) return Promise.resolve({ layers: [] });
      return Promise.resolve({
        insights: [{ id: 'n1' }],
        totalCount: 1,
      });
    });
    mockWsStatusRef.current = 'connected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(result.current.steps.find((s) => s.key === 'notifications')?.status).toBe('success');
    });

    expect(mockMergeNotifications).toHaveBeenCalled();
  });

  it('WS timeout sets websocket step to error after 10 seconds', async () => {
    vi.useFakeTimers();

    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('layers')) return Promise.resolve({ layers: [] });
      return Promise.resolve({ insights: [], totalCount: 0 });
    });
    mockWsStatusRef.current = 'disconnected';

    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });

    // Flush the microtask queue so queries resolve
    await vi.advanceTimersByTimeAsync(100);

    // Advance past the 10-second WS timeout
    await vi.advanceTimersByTimeAsync(11_000);

    expect(result.current.steps.find((s) => s.key === 'websocket')?.status).toBe('error');
    expect(result.current.error).toBe(true);

    vi.useRealTimers();
  });

  it('step labels match spec', () => {
    mockApiGet.mockResolvedValue({ layers: [] });
    const { result } = renderHook(() => useAppBootstrap(), { wrapper: createWrapper() });
    const labels = result.current.steps.map((s) => s.label);
    expect(labels).toEqual([
      'LOADING LAYERS...',
      'LOADING NOTIFICATIONS...',
      'CONNECTING STREAM...',
    ]);
  });
});
