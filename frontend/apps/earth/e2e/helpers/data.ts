import { APIRequestContext } from '@playwright/test';

// Polls /v1/layers then each layer's snapshot until at least one entity is
// present. Returns the total entity count found, or 0 if the timeout elapses.
//
// Mirrors the global-setup.ts style: same @playwright/test `request` API,
// same deadline-loop shape (for/while with Date.now() + deadline guard,
// catch-safe fetch, 2 s sleep between polls).
//
// JSON shape confirmed against live server (Task 5 Step 1):
//   GET /v1/layers          → { layers: [{ id: string, ... }] }
//   GET /v1/layers/:id/snapshot → { entities: [...] }
export async function waitForEntityData(
  api: APIRequestContext,
  timeoutMs = 90_000,
): Promise<number> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const layersRes = await api.get('/v1/layers');
      if (layersRes.ok()) {
        const body = await layersRes.json();
        const layers: Array<{ id?: string; layerId?: string }> = body.layers ?? body ?? [];
        for (const layer of layers) {
          const id = layer.id ?? layer.layerId;
          if (!id) continue;
          const snap = await api.get(`/v1/layers/${id}/snapshot`);
          if (snap.ok()) {
            const s = await snap.json();
            const entities: unknown[] = s.entities ?? s.observations ?? [];
            if (entities.length > 0) return entities.length;
          }
        }
      }
    } catch {
      // server not reachable yet — keep polling
    }
    await new Promise((r) => setTimeout(r, 2000));
  }
  return 0;
}

/**
 * Find a spatial (viewport-filtered) layer ID from /v1/layers. Returns the
 * first layer whose ID matches known spatial layer types, or falls back to the
 * first available layer. Returns null if no layers are registered.
 *
 * Moved here from progressive-loading.spec.ts so it can be shared across specs.
 */
export async function findSpatialLayerId(api: APIRequestContext): Promise<string | null> {
  try {
    const res = await api.get('/v1/layers');
    if (!res.ok()) return null;
    const body = await res.json();
    const layers: Array<{ id?: string; layerId?: string }> = body.layers ?? body ?? [];
    // Prefer known spatial layer types (adsb_lol, flights_commercial, ships)
    const SPATIAL_LAYER_PREFIXES = [
      'flights_commercial',
      'ships',
      'adsb',
      'earthquakes',
      'lightning',
    ];
    for (const prefix of SPATIAL_LAYER_PREFIXES) {
      const match = layers.find((l) => (l.id ?? l.layerId ?? '').startsWith(prefix));
      if (match) return match.id ?? match.layerId ?? null;
    }
    // Fallback: first available layer
    if (layers.length > 0) return layers[0].id ?? layers[0].layerId ?? null;
    return null;
  } catch {
    return null;
  }
}
