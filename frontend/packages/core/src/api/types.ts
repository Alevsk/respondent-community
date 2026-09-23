// API response types and query keys — shared across apps
//
// These types represent the snake_case / mixed-case shapes returned by the
// backend (gRPC-gateway JSON). They are intentionally distinct from the
// camelCase domain models in `@respondent/core/models`.

import type { LayerDisplayConfig, HistoryConfig } from '../models';

// ---------------------------------------------------------------------------
// API response type (snake_case fields from the backend).
// The canonical domain type is Layer in models/layer.ts (camelCase).
// ---------------------------------------------------------------------------

export interface ApiLayer {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  mode: string;
  density: number;
  source: string;
  last_update: number;
  count: number;
  color: string;
  pointSize: number;
  displayConfig?: LayerDisplayConfig;
  historyConfig?: HistoryConfig;
  history_config?: HistoryConfig; // snake_case variant from proto JSON
  rendering_mode?: string; // "map" (default) or "indicator"
  renderingMode?: string; // camelCase variant from proto JSON
  filtering_mode?: string; // "viewport" or ""
  filteringMode?: string; // camelCase variant from proto JSON
}

/**
 * Normalize protobuf enum strings to simple lowercase values.
 * e.g. "RENDERING_MODE_INDICATOR" -> "indicator", "FILTERING_MODE_VIEWPORT" -> "viewport"
 */
export function normalizeProtoEnum(raw: string | undefined, prefix: string): string | undefined {
  if (!raw) return undefined;
  const upper = raw.toUpperCase();
  if (upper === `${prefix}_UNSPECIFIED`) return undefined;
  if (upper.startsWith(`${prefix}_`)) return raw.slice(prefix.length + 1).toLowerCase();
  return raw;
}

// ---------------------------------------------------------------------------
// Entity / Observation (API snake_case shape — NOT the domain model)
// ---------------------------------------------------------------------------

export interface ApiEntity {
  id: string;
  external_id: string;
  layer_type: string;
  name: string;
  metadata: Record<string, string>;
  ai_metadata_json?: string;
}

export interface ApiObservation {
  entity_id: string;
  position: { lat: number; lon: number };
  altitude_m: number;
  timestamp: string;
  velocity?: Record<string, number>;
}

// ---------------------------------------------------------------------------
// Composite API response types
// ---------------------------------------------------------------------------

export interface LayerSnapshotResponse {
  entities: ApiEntity[];
  observations: ApiObservation[];
  total_count: number;
  has_more: boolean;
}

export interface LayerToggleRequest {
  layer_id: string;
  enabled: boolean;
  mode?: string;
  density?: number;
}

// ---------------------------------------------------------------------------
// Media playback notification (POST /v1/media/playback)
// Mirrors v1ReportMediaPlaybackRequest / v1ReportMediaPlaybackResponse in the
// generated OpenAPI spec. The request names a stored entity and one of the
// media slots its layer declares — there is deliberately no field for a URL.
// ---------------------------------------------------------------------------

export interface ReportMediaPlaybackRequest {
  entityId: string;
  mediaId: string;
}

export interface ReportMediaPlaybackResponse {
  /** Whether the source was notified. False is not an error. */
  reported: boolean;
}

export interface Scene {
  id: string;
  name: string;
  camera: {
    position: { lat: number; lon: number; alt_m: number };
    heading: number;
    pitch: number;
    roll: number;
    range: number;
  };
  layers: Array<{
    layer_id: string;
    enabled: boolean;
    mode: string;
    density: number;
  }>;
  filter_preset_id: string;
}

/** API filter preset — distinct from the UI FilterPreset type ('NORMAL' | 'CRT' | ...). */
export interface ApiFilterPreset {
  id: string;
  name: string;
  style: string;
  params: Record<string, number>;
}

// Entity detail types (from GET /v1/entities/:id)
export interface EntityDetailResponse {
  entity: {
    id: string;
    externalId: string;
    layerType: string;
    name: string;
    metadata: Record<string, string>;
    aiMetadataJson?: string;
  };
  latestObservation?: {
    entityId: string;
    ts: number;
    position: { lat: number; lon: number; altM: number };
    altitudeM: number;
    velocity?: Record<string, number>;
    metadata?: Record<string, string>;
  };
}

export interface ObservationHistoryResponse {
  observations: Array<{
    entityId: string;
    ts: number;
    position: { lat: number; lon: number; altM: number };
    altitudeM: number;
    velocity?: Record<string, number>;
    metadata?: Record<string, string>;
  }>;
  hasMore: boolean;
}

// ---------------------------------------------------------------------------
// Query keys
// ---------------------------------------------------------------------------

export const queryKeys = {
  layers: ['layers'] as const,
  layer: (id: string) => ['layers', id] as const,
  layerSnapshot: (id: string, limit?: number, offset?: number) =>
    ['layers', id, 'snapshot', { limit, offset }] as const,
  scenes: ['scenes'] as const,
  scene: (id: string) => ['scenes', id] as const,
  filterPresets: ['filterPresets'] as const,
  filterPreset: (id: string) => ['filterPresets', id] as const,
  entityDetail: (id: string) => ['entities', id] as const,
  entityObservations: (id: string, limit: number, beforeMs?: number) =>
    ['entities', id, 'observations', { limit, beforeMs }] as const,
  entitySearch: (query: string, layerType?: string, limit?: number) =>
    ['entities', 'search', { query, layerType, limit }] as const,
};

// ---------------------------------------------------------------------------
// REST response normalization
// ---------------------------------------------------------------------------
// gRPC-gateway serializes proto int64 as JSON strings to avoid JavaScript
// 53-bit precision loss. Convert at the API boundary so consumers get numbers.

/* eslint-disable @typescript-eslint/no-explicit-any */
export function normalizeObservationHistory(raw: any): ObservationHistoryResponse {
  return {
    observations: (raw.observations ?? []).map((obs: any) => ({
      ...obs,
      ts: Number(obs.ts),
      altitudeM: Number(obs.altitudeM ?? 0),
    })),
    hasMore: raw.hasMore ?? false,
  };
}

export function normalizeEntityDetail(raw: any): EntityDetailResponse {
  const result: EntityDetailResponse = { entity: raw.entity };
  if (raw.latestObservation) {
    result.latestObservation = {
      ...raw.latestObservation,
      ts: Number(raw.latestObservation.ts),
      altitudeM: Number(raw.latestObservation.altitudeM ?? 0),
    };
  }
  return result;
}
/* eslint-enable @typescript-eslint/no-explicit-any */
