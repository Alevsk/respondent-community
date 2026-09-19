import { describe, it, expect, vi } from 'vitest';
import { getRuntimeConfig, loadRuntimeConfig } from './runtimeConfig';

describe('runtimeConfig', () => {
  it('resolves without throwing when the fetch fails (startup must not block)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));
    await expect(loadRuntimeConfig()).resolves.toBeUndefined();
  });

  it('caches client config from a successful /config.json fetch', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ cesiumIonToken: 'tok-123' }) }),
    );
    await loadRuntimeConfig();
    expect(getRuntimeConfig().cesiumIonToken).toBe('tok-123');
  });

  it('ignores a non-OK response (keeps prior config)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }));
    await loadRuntimeConfig();
    // The previous successful load remains the source of truth.
    expect(getRuntimeConfig().cesiumIonToken).toBe('tok-123');
  });
});
