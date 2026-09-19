import { describe, it, expect } from 'vitest';

// ---------------------------------------------------------------------------
// API types & query keys
// ---------------------------------------------------------------------------

describe('API types module', () => {
  it('exports ApiLayer interface (compile-time check via usage)', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys).toBeDefined();
  });

  it('queryKeys.layers is a static tuple', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.layers).toEqual(['layers']);
  });

  it('queryKeys.layer returns a tuple with the id', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.layer('abc')).toEqual(['layers', 'abc']);
  });

  it('queryKeys.layerSnapshot includes limit and offset', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.layerSnapshot('x', 100, 10)).toEqual([
      'layers',
      'x',
      'snapshot',
      { limit: 100, offset: 10 },
    ]);
  });

  it('queryKeys.scenes is a static tuple', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.scenes).toEqual(['scenes']);
  });

  it('queryKeys.filterPresets is a static tuple', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.filterPresets).toEqual(['filterPresets']);
  });

  it('queryKeys.entityDetail returns tuple with entity id', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.entityDetail('e1')).toEqual(['entities', 'e1']);
  });

  it('queryKeys.entityObservations includes limit and beforeMs', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.entityObservations('e1', 50, 1000)).toEqual([
      'entities',
      'e1',
      'observations',
      { limit: 50, beforeMs: 1000 },
    ]);
  });

  it('queryKeys.entitySearch includes query, layerType, and limit', async () => {
    const { queryKeys } = await import('./types');
    expect(queryKeys.entitySearch('test', 'aircraft', 10)).toEqual([
      'entities',
      'search',
      { query: 'test', layerType: 'aircraft', limit: 10 },
    ]);
  });
});

// ---------------------------------------------------------------------------
// normalizeProtoEnum
// ---------------------------------------------------------------------------

describe('normalizeProtoEnum', () => {
  it('strips prefix and lowercases the value', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum('RENDERING_MODE_INDICATOR', 'RENDERING_MODE')).toBe('indicator');
  });

  it('returns undefined for UNSPECIFIED values', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum('RENDERING_MODE_UNSPECIFIED', 'RENDERING_MODE')).toBeUndefined();
  });

  it('returns undefined for undefined input', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum(undefined, 'RENDERING_MODE')).toBeUndefined();
  });

  it('returns undefined for empty string input', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum('', 'RENDERING_MODE')).toBeUndefined();
  });

  it('returns raw value when prefix does not match', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum('SOMETHING_ELSE', 'RENDERING_MODE')).toBe('SOMETHING_ELSE');
  });

  it('normalizes FILTERING_MODE_VIEWPORT', async () => {
    const { normalizeProtoEnum } = await import('./types');
    expect(normalizeProtoEnum('FILTERING_MODE_VIEWPORT', 'FILTERING_MODE')).toBe('viewport');
  });
});

// ---------------------------------------------------------------------------
// normalizeObservationHistory
// ---------------------------------------------------------------------------

describe('normalizeObservationHistory', () => {
  it('converts string ts and altitudeM to numbers', async () => {
    const { normalizeObservationHistory } = await import('./types');
    const raw = {
      observations: [
        {
          entityId: 'e1',
          ts: '1700000000000',
          position: { lat: 1, lon: 2, altM: 100 },
          altitudeM: '500',
        },
      ],
      hasMore: true,
    };
    const result = normalizeObservationHistory(raw);
    expect(result.observations[0].ts).toBe(1700000000000);
    expect(result.observations[0].altitudeM).toBe(500);
    expect(result.hasMore).toBe(true);
  });

  it('defaults to empty observations and hasMore false', async () => {
    const { normalizeObservationHistory } = await import('./types');
    const result = normalizeObservationHistory({});
    expect(result.observations).toEqual([]);
    expect(result.hasMore).toBe(false);
  });

  it('defaults altitudeM to 0 when absent', async () => {
    const { normalizeObservationHistory } = await import('./types');
    const raw = {
      observations: [{ entityId: 'e1', ts: '100', position: { lat: 0, lon: 0, altM: 0 } }],
    };
    const result = normalizeObservationHistory(raw);
    expect(result.observations[0].altitudeM).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// normalizeEntityDetail
// ---------------------------------------------------------------------------

describe('normalizeEntityDetail', () => {
  it('converts string ts and altitudeM in latestObservation', async () => {
    const { normalizeEntityDetail } = await import('./types');
    const raw = {
      entity: { id: 'e1', externalId: 'ext', layerType: 'test', name: 'Test', metadata: {} },
      latestObservation: {
        entityId: 'e1',
        ts: '1700000000000',
        position: { lat: 1, lon: 2, altM: 100 },
        altitudeM: '500',
      },
    };
    const result = normalizeEntityDetail(raw);
    expect(result.latestObservation!.ts).toBe(1700000000000);
    expect(result.latestObservation!.altitudeM).toBe(500);
  });

  it('handles missing latestObservation', async () => {
    const { normalizeEntityDetail } = await import('./types');
    const raw = {
      entity: { id: 'e1', externalId: 'ext', layerType: 'test', name: 'Test', metadata: {} },
    };
    const result = normalizeEntityDetail(raw);
    expect(result.latestObservation).toBeUndefined();
  });

  it('defaults altitudeM to 0 when absent', async () => {
    const { normalizeEntityDetail } = await import('./types');
    const raw = {
      entity: { id: 'e1', externalId: 'ext', layerType: 'test', name: 'Test', metadata: {} },
      latestObservation: {
        entityId: 'e1',
        ts: '100',
        position: { lat: 0, lon: 0, altM: 0 },
      },
    };
    const result = normalizeEntityDetail(raw);
    expect(result.latestObservation!.altitudeM).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// client.ts — endpoints & api helper
// ---------------------------------------------------------------------------

describe('API client', () => {
  it('exports endpoints object with expected keys', async () => {
    const { endpoints } = await import('./client');
    expect(endpoints).toHaveProperty('layers');
    expect(endpoints).toHaveProperty('entities');
    expect(endpoints).toHaveProperty('scenes');
    expect(endpoints).toHaveProperty('filters');
    expect(endpoints).toHaveProperty('cctv');
    expect(endpoints).toHaveProperty('aiInsights');
    expect(endpoints).toHaveProperty('aiNotificationFilters');
  });

  it('exports WS_URL string', async () => {
    const { WS_URL } = await import('./client');
    expect(typeof WS_URL).toBe('string');
    expect(WS_URL).toContain('/ws');
  });

  it('exports api object with HTTP methods', async () => {
    const { api } = await import('./client');
    expect(typeof api.get).toBe('function');
    expect(typeof api.post).toBe('function');
    expect(typeof api.put).toBe('function');
    expect(typeof api.delete).toBe('function');
  });

  it('api.get calls fetch and returns parsed JSON', async () => {
    const mockResponse = { data: 'test' };
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const { api } = await import('./client');
    const result = await api.get<{ data: string }>('http://example.com/test');
    expect(result).toEqual(mockResponse);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      'http://example.com/test',
      expect.objectContaining({
        headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
      }),
    );
  });

  it('api.post sends JSON body with POST method', async () => {
    const mockResponse = { id: '1' };
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const { api } = await import('./client');
    const result = await api.post<{ id: string }>('http://example.com/test', { name: 'test' });
    expect(result).toEqual(mockResponse);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      'http://example.com/test',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'test' }),
      }),
    );
  });

  it('api.get throws on non-OK response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValueOnce({
      ok: false,
      statusText: 'Not Found',
    });

    const { api } = await import('./client');
    await expect(api.get('http://example.com/missing')).rejects.toThrow('API error: Not Found');
  });
});

// ---------------------------------------------------------------------------
// websocket.ts — WebSocketClient
// ---------------------------------------------------------------------------

describe('WebSocketClient', () => {
  it('exports WebSocketClient class', async () => {
    const { WebSocketClient } = await import('./websocket');
    expect(typeof WebSocketClient).toBe('function');
    const client = new WebSocketClient();
    expect(client).toBeDefined();
    expect(typeof client.connect).toBe('function');
    expect(typeof client.disconnect).toBe('function');
    expect(typeof client.subscribe).toBe('function');
    expect(typeof client.send).toBe('function');
  });

  it('exports wsClient singleton', async () => {
    const { wsClient } = await import('./websocket');
    expect(wsClient).toBeDefined();
    expect(wsClient.status).toBe('disconnected');
  });

  it('exports useWebSocketStatus hook', async () => {
    const { useWebSocketStatus } = await import('./websocket');
    expect(typeof useWebSocketStatus).toBe('function');
  });

  it('initial status is disconnected', async () => {
    const { WebSocketClient } = await import('./websocket');
    const client = new WebSocketClient();
    expect(client.status).toBe('disconnected');
    expect(client.isConnected()).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Barrel re-exports
// ---------------------------------------------------------------------------

describe('API barrel exports', () => {
  it('re-exports all public API from index', async () => {
    const barrel = await import('./index');
    // types
    expect(barrel.queryKeys).toBeDefined();
    expect(typeof barrel.normalizeProtoEnum).toBe('function');
    expect(typeof barrel.normalizeObservationHistory).toBe('function');
    expect(typeof barrel.normalizeEntityDetail).toBe('function');
    // client
    expect(barrel.endpoints).toBeDefined();
    expect(barrel.api).toBeDefined();
    expect(typeof barrel.WS_URL).toBe('string');
    // websocket
    expect(barrel.WebSocketClient).toBeDefined();
    expect(barrel.wsClient).toBeDefined();
    expect(typeof barrel.useWebSocketStatus).toBe('function');
  });
});
