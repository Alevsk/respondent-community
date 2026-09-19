// Runtime frontend config.
//
// A compiled SPA can't read the container's environment, so the server hands
// client-safe values to the browser at runtime via GET /config.json. We fetch it
// ONCE at startup (before the app module graph is loaded — see main.tsx) and cache
// it in this module singleton, so consumers can read it synchronously like they
// would build-time env, but it's configurable per deployment without a rebuild.
//
// Consume with the fallback chain: runtime config -> build-time env -> default, e.g.
//   getRuntimeConfig().cesiumIonToken ?? import.meta.env.VITE_CESIUM_ION_TOKEN

export interface RuntimeConfig {
  /** Cesium ion CLIENT token (server env RESPONDENT_FRONTEND_CESIUM_ION_TOKEN). */
  cesiumIonToken?: string;
}

let current: RuntimeConfig = {};

/** Returns the runtime config loaded at startup (empty object until loaded). */
export function getRuntimeConfig(): RuntimeConfig {
  return current;
}

/**
 * Fetches /config.json (same-origin) and caches it. Resolves even on failure —
 * a missing/failed config must not block app startup; consumers fall back to their
 * build-time defaults. Call once, before importing the app, in main.tsx.
 */
export async function loadRuntimeConfig(): Promise<void> {
  try {
    const res = await fetch('/config.json', { headers: { Accept: 'application/json' } });
    if (res.ok) {
      current = (await res.json()) as RuntimeConfig;
    }
  } catch {
    // Keep defaults; the frontend falls back to build-time env / hardcoded defaults.
  }
}
