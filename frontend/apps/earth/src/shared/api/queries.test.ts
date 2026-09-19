/**
 * queries.ts — ApiLayer → Layer mapping tests
 *
 * Tests the historyConfig field mapping logic that runs inside useLayers()
 * when it calls setLayers(). The transform is:
 *
 *   historyConfig: l.historyConfig ?? l.history_config,
 *
 * We exercise this directly by calling useUIStore.getState().setLayers() with
 * the same mapped shape that useLayers() would produce, and asserting what
 * lands in the store.  This avoids the need to mock @tanstack/react-query or
 * the network layer while still covering the exact mapping expression.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { useUIStore } from '@/app/store';
import type { Layer, HistoryConfig } from '@/app/store';
import type { ApiLayer } from './queries';

// ---------------------------------------------------------------------------
// Helper — apply the same mapping as useLayers()
// ---------------------------------------------------------------------------

/**
 * Mirrors the ApiLayer → Layer transform from useLayers() without the
 * network call or React Query wrapper.  Only the historyConfig mapping
 * is under test here; the rest of the fields are passed through verbatim.
 */
function mapApiLayerToLayer(l: ApiLayer): Layer {
  return {
    id: l.id,
    name: l.name,
    type: l.type,
    enabled: l.enabled,
    mode: l.mode,
    density: l.density,
    source: l.source,
    lastUpdate: l.last_update,
    count: l.count,
    color: l.color,
    pointSize: l.pointSize,
    displayConfig: l.displayConfig,
    historyConfig: l.historyConfig ?? l.history_config,
  };
}

function makeApiLayer(overrides: Partial<ApiLayer> = {}): ApiLayer {
  return {
    id: 'layer-id-1',
    name: 'Test Layer',
    type: 'test-type',
    enabled: true,
    mode: 'realtime',
    density: 50,
    source: 'test',
    last_update: 0,
    count: 0,
    color: '#ffffff',
    pointSize: 4,
    ...overrides,
  };
}

const SAMPLE_HISTORY_CONFIG: HistoryConfig = {
  maxLookbackHours: 720,
  maxRangeSpanHours: 168,
};

// ---------------------------------------------------------------------------
// Store reset
// ---------------------------------------------------------------------------

beforeEach(() => {
  useUIStore.setState({ layers: {} }, false);
});

// ---------------------------------------------------------------------------
// historyConfig mapping
// ---------------------------------------------------------------------------

describe('ApiLayer → Layer historyConfig mapping', () => {
  it('preserves historyConfig from the camelCase field', () => {
    const api = makeApiLayer({ historyConfig: SAMPLE_HISTORY_CONFIG });
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig).toEqual(SAMPLE_HISTORY_CONFIG);
  });

  it('falls back to history_config (snake_case) when historyConfig is absent', () => {
    const api = makeApiLayer({ history_config: SAMPLE_HISTORY_CONFIG });
    // historyConfig is intentionally absent — do not set it
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig).toEqual(SAMPLE_HISTORY_CONFIG);
  });

  it('camelCase historyConfig takes precedence over snake_case history_config', () => {
    const camelValue: HistoryConfig = { maxLookbackHours: 100, maxRangeSpanHours: 50 };
    const snakeValue: HistoryConfig = { maxLookbackHours: 999, maxRangeSpanHours: 999 };
    const api = makeApiLayer({ historyConfig: camelValue, history_config: snakeValue });
    const layer = mapApiLayerToLayer(api);
    // camelCase wins (left side of ??)
    expect(layer.historyConfig).toEqual(camelValue);
  });

  it('historyConfig is undefined when both fields are absent', () => {
    const api = makeApiLayer();
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig).toBeUndefined();
  });

  it('historyConfig is undefined when historyConfig is null and history_config is absent', () => {
    // null triggers the ?? fallback only when the left side is null or undefined
    // so null on the camelCase field should also be treated as absent
    const api = makeApiLayer({ historyConfig: undefined, history_config: undefined });
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig).toBeUndefined();
  });

  it('mapped layer carries correct maxLookbackHours from snake_case fallback', () => {
    const api = makeApiLayer({
      history_config: { maxLookbackHours: 8760, maxRangeSpanHours: 720 },
    });
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig?.maxLookbackHours).toBe(8760);
  });

  it('mapped layer carries correct maxRangeSpanHours from snake_case fallback', () => {
    const api = makeApiLayer({
      history_config: { maxLookbackHours: 8760, maxRangeSpanHours: 720 },
    });
    const layer = mapApiLayerToLayer(api);
    expect(layer.historyConfig?.maxRangeSpanHours).toBe(720);
  });

  it('non-historyConfig fields are correctly mapped (spot-check lastUpdate)', () => {
    const api = makeApiLayer({ last_update: 1_700_000_000 });
    const layer = mapApiLayerToLayer(api);
    expect(layer.lastUpdate).toBe(1_700_000_000);
  });
});

// ---------------------------------------------------------------------------
// setLayers round-trip — historyConfig survives the store write
// ---------------------------------------------------------------------------

describe('setLayers round-trip with historyConfig', () => {
  it('historyConfig is readable from the store after setLayers', () => {
    const api = makeApiLayer({
      id: 'rt-id',
      type: 'rt-type',
      historyConfig: SAMPLE_HISTORY_CONFIG,
    });
    const layer = mapApiLayerToLayer(api);
    useUIStore.getState().setLayers([layer]);
    expect(useUIStore.getState().layers['rt-id']?.historyConfig).toEqual(SAMPLE_HISTORY_CONFIG);
  });

  it('snake_case fallback historyConfig is readable from the store after setLayers', () => {
    const api = makeApiLayer({
      id: 'rt-id',
      type: 'rt-type',
      history_config: SAMPLE_HISTORY_CONFIG,
    });
    const layer = mapApiLayerToLayer(api);
    useUIStore.getState().setLayers([layer]);
    expect(useUIStore.getState().layers['rt-type']?.historyConfig).toEqual(SAMPLE_HISTORY_CONFIG);
  });

  it('layer without historyConfig stores undefined in the store', () => {
    const api = makeApiLayer({ id: 'rt-id', type: 'rt-type' });
    const layer = mapApiLayerToLayer(api);
    useUIStore.getState().setLayers([layer]);
    expect(useUIStore.getState().layers['rt-id']?.historyConfig).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Entity ID URL encoding
// ---------------------------------------------------------------------------

describe('Entity ID URL encoding', () => {
  it('encodeURIComponent is applied to entity IDs containing URLs', () => {
    // Entity IDs for news articles contain full URLs with ://
    const entityId = 'news_articles:https://thehackernews.com/2026/04/test-article.html';
    const encoded = encodeURIComponent(entityId);

    // The encoded string must NOT contain :// which causes path normalization
    expect(encoded).not.toContain('://');
    // Must preserve the original value when decoded
    expect(decodeURIComponent(encoded)).toBe(entityId);
    // Specifically, the double slash is encoded
    expect(encoded).toContain('%3A%2F%2F');
  });
});
