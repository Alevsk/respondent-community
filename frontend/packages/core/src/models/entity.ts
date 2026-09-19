// Data provenance — where the entity/observation data came from
export type DataSource = 'live' | 'stale' | 'historical';

// Entity types from API
export interface Entity {
  id: string;
  externalId: string;
  layerType: string;
  name: string;
  metadata: Record<string, string>;
  aiMetadata?: Record<string, unknown>; // AI enrichment data (parsed from JSON)
  source?: DataSource; // data provenance
}

// Observation types from API
export interface Observation {
  entityId: string;
  position: {
    lat: number;
    lon: number;
  };
  altitudeM: number;
  timestamp: string;
  velocity?: Record<string, number>;
  metadata?: Record<string, string>;
  source?: DataSource; // data provenance
}

/**
 * Shared entity feature types — defined here to avoid circular imports
 * between the store (app layer) and feature-level hooks.
 */
export interface TrailPoint {
  ts: number;
  lon: number;
  lat: number;
  altitudeM: number;
  speed?: number;
  metadata?: Record<string, string>;
}

// Internal map-based storage for O(1) per-entity merges
export interface LayerData {
  entityMap: Map<string, Entity>;
  obsMap: Map<string, Observation>;
  // Entity IDs modified since the last consumer read — enables O(delta) rendering
  dirtyEntityIds: Set<string>;
  needsRebuild?: boolean;
}

/** Per-entity UI view state — consolidated to simplify cleanup on deselection. */
export interface EntityViewState {
  /** Highlighted trail point timestamp in ms (null = no highlight). */
  trailHighlight: number | null;
  /** Active tab id in the entity detail panel. */
  activeTab: string;
  /** Whether trail polylines are visible (undefined = never visited History tab). */
  showTrails?: boolean;
  /** Whether to hide all other entities when viewing this one. */
  isolateEntity?: boolean;
}

export const DEFAULT_ENTITY_VIEW_STATE: EntityViewState = {
  trailHighlight: null,
  activeTab: 'overview',
};

/** Named type for entity + layer identifier pair used in selection and watchlist. */
export interface EntityRef {
  entityId: string;
  layerId: string;
}

// Merge keys for entity and observation maps.
// Data is normalized at the WebSocket ingestion boundary (useLayerStream.ts)
// so only the canonical camelCase fields are used here.
export function entityKey(e: Entity): string {
  return e.id || '';
}

export function obsKey(o: Observation): string {
  return o.entityId || '';
}
