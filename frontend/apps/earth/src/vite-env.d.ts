/// <reference types="vite/client" />
/// <reference types="@testing-library/jest-dom" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string;
  readonly VITE_WS_URL: string;
  readonly VITE_CESIUM_ION_TOKEN?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

// ---------------------------------------------------------------------------
// E2E automation hook (read-only access to the WS client singleton)
// ---------------------------------------------------------------------------
interface Window {
  /**
   * Exposed ONLY for Playwright e2e tests so they can observe connection state
   * and trigger a non-intentional reconnect (simulating network recovery).
   * Do NOT call disconnect() from tests — it sets intentionalDisconnect=true
   * and bypasses the auto-reconnect path.
   */
  __wsClient?: import('@respondent/core').WebSocketClient;
  /**
   * Exposed ONLY for Playwright e2e tests — returns a read-only snapshot of
   * the UI store state so tests can assert on selection, time, watchlist, and
   * display settings without coupling to React internals.
   */
  __store?: () => {
    selectedEntityId: string | null;
    selectedLayerId: string | null;
    selectedEntities: Array<{ entityId: string; layerId: string }>;
    viewMode: import('@respondent/core').ViewMode;
    watchlistEntities: import('@respondent/core').WatchlistEntity[];
    findMode: boolean;
    timeMode: import('@respondent/core').TimeMode;
    timeFrom: string | null;
    timeTo: string | null;
    timePreset: import('@respondent/core').TimePreset | null;
    showOccluded: boolean;
    showGeoLabels: boolean;
    show3DBuildings: boolean;
    smoothMotion: boolean;
    cinematicDrift: boolean;
    spatialAggregation: boolean;
    activeCluster: import('./stores/createClusterSlice').ActiveCluster | null;
    maxEntities: number;
  };
  /**
   * Exposed ONLY for Playwright e2e tests — seeds entity selection so tests
   * can drive the selection state without requiring a canvas click.
   */
  __store_setSelected?: (entityId: string | null, layerId: string | null) => void;
  /**
   * Exposed ONLY for Playwright e2e tests — opens a cluster's member list,
   * mirroring the Cesium cluster-marker pick (not DOM-addressable).
   */
  __store_setActiveCluster?: (cluster: import('./stores/createClusterSlice').ActiveCluster) => void;
  /**
   * Exposed ONLY for Playwright e2e tests — enters entity-view (viewMode ===
   * 'entity') deterministically, mirroring the Cesium double-click path which is
   * not DOM-addressable. Selects the entity, then flips viewMode to 'entity'.
   */
  __store_enterEntityView?: (entityId: string, layerId: string) => void;
}
