/**
 * billboardRendering.ts
 *
 * Pure helper functions, constants, and types for BillboardLayerRenderer.
 * Nothing here touches React refs, component state, or lifecycle hooks —
 * all exports are safe to unit-test in isolation.
 */

import { Color } from 'cesium';
import { type DataSource, type Observation } from '@/app/store';
import { useUIStore } from '@/app/store';

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

/** Shape stored in Billboard.id — Cesium types Billboard.id as `any`. */
export interface BillboardId {
  entityId: string;
  layerId: string;
}

/** Normalized observation fields consumed by the billboard sync logic. */
export interface NormalizedObservation {
  entityId: string;
  lon: number;
  lat: number;
  alt: number;
  heading?: number; // degrees clockwise from north
  groundSpeedKnots?: number; // knots, for dead-reckoning extrapolation
}

/** Animation state for smooth interpolation between position updates. */
export interface AnimationState {
  fromPos: import('cesium').Cartesian3;
  toPos: import('cesium').Cartesian3;
  fromRotation: number;
  toRotation: number;
  fromAlpha: number;
  toAlpha: number;
  startTime: number; // Date.now() ms
  duration: number; // ms
  // Dead-reckoning extrapolation (post-lerp)
  groundSpeedMps?: number; // m/s (converted from knots)
  headingRad?: number; // radians for geodesic projection (geographic: CW from north)
  // Catchup pause: when backward motion is detected, freeze extrapolation
  // until the backend catches up. Prevents jarring backward lerp.
  pauseUntil?: number; // Date.now() ms — skip dead-reckoning until this time
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const INTERP_DURATION_MS = 2000;

/** Duration to pause dead-reckoning after backward motion detection (ms) */
export const CATCHUP_PAUSE_MS = 5000;

/** Resting alpha for live entities on interpolated (flight) layers — dimmed between updates */
export const LIVE_IDLE_ALPHA = 1.0;

/** Flash alpha when a live entity receives a position update */
export const LIVE_PULSE_ALPHA = 1.0;

/** Position epsilon for skipping no-op animations (in degrees) */
export const POS_EPSILON = 1e-6;

// Source-based alpha values for data provenance rendering.
// All sources render at full opacity — stale/historical provenance is indicated
// in the Entity Detail panel ("Last seen: Xm ago") rather than billboard dimming,
// which created a jarring visual that users perceived as a selection-related bug.
export const SOURCE_ALPHA: Record<DataSource, number> = {
  live: 1.0,
  stale: 1.0,
  historical: 1.0,
};

// ---------------------------------------------------------------------------
// Pure helper functions
// ---------------------------------------------------------------------------

/** Safely extracts entityId from a billboard's id property. */
export function getBillboardEntityId(billboard: import('cesium').Billboard): string | null {
  const id = billboard.id;
  if (id && typeof id === 'object' && 'entityId' in id) {
    return (id as BillboardId).entityId;
  }
  return null;
}

/** Returns the source-specific alpha value (1.0 for all sources). */
export function getSourceAlpha(source?: DataSource): number {
  return SOURCE_ALPHA[source ?? 'live'] ?? 1.0;
}

/**
 * Resolves a hex color for an entity from a declarative metadata→color rule
 * (layer.displayConfig.colorBy), or undefined when no rule applies. This replaces
 * per-layer hardcoded color logic: the rule (field name, value→color map, default)
 * comes from the source YAML, so the client stays layer-agnostic.
 */
export function resolveColorByHex(
  colorBy: { field: string; values: Record<string, string>; defaultColor: string } | undefined,
  metadata: Record<string, string> | undefined,
): string | undefined {
  if (!colorBy) return undefined;
  const value = metadata?.[colorBy.field] ?? '';
  return colorBy.values[value] || colorBy.defaultColor || undefined;
}

/** Desaturate a color for historical entities. */
export function desaturateColor(color: Color, factor: number): Color {
  const r = color.red;
  const g = color.green;
  const b = color.blue;
  const gray = 0.299 * r + 0.587 * g + 0.114 * b;
  return new Color(
    r + (gray - r) * factor,
    g + (gray - g) * factor,
    b + (gray - b) * factor,
    color.alpha,
  );
}

/** Smoothstep easing: maps t ∈ [0,1] to a smooth S-curve. */
export function smoothstep(t: number): number {
  return t * t * (3 - 2 * t);
}

/**
 * Normalize a raw store Observation into the fields the billboard renderer
 * actually uses. Returns null when the observation is invalid or missing
 * required position data.
 */
export function normalizeObservation(raw: Observation): NormalizedObservation | null {
  if (!raw?.entityId) return null;
  const pos = raw.position;
  if (!pos) return null;

  const lon = Number(pos.lon);
  const lat = Number(pos.lat);
  if (isNaN(lon) || isNaN(lat)) return null;

  const alt = Number(raw.altitudeM ?? 0);

  let heading: number | undefined;
  if (raw.velocity?.heading != null) {
    const h = Number(raw.velocity.heading);
    if (!isNaN(h)) heading = h;
  }

  let groundSpeedKnots: number | undefined;
  const gs = raw.velocity?.ground_speed ?? raw.velocity?.speed;
  if (gs != null) {
    const v = Number(gs);
    if (!isNaN(v) && v > 0) groundSpeedKnots = v;
  }

  return { entityId: raw.entityId, lon, lat, alt, heading, groundSpeedKnots };
}

/**
 * Returns true if this layer type supports dead-reckoning motion interpolation.
 * Reads interpolation from the layer's displayConfig (populated by the declarative
 * source YAML via the GetLayers API). Layers without a displayConfig declaration
 * default to false — no layer-name fallback.
 *
 * NOTE: Reads a snapshot via getState() (not reactive). This is intentional —
 * layer metadata is loaded once at app startup and is stable for the lifetime
 * of the session, so a reactive subscription would add overhead with no benefit.
 */
export function supportsInterpolation(layerType: string): boolean {
  const layer = useUIStore.getState().layers[layerType];
  return layer?.displayConfig?.icon?.interpolation ?? false;
}
