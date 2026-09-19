/**
 * Pure utility functions for entity trail rendering.
 *
 * Extracted from EntityTrailRenderer so the logic can be unit-tested
 * without mounting React components or mocking Cesium primitives.
 */

import type { TrailPoint } from '@respondent/core';

/** Max time gap (ms) between consecutive observations before splitting into
 *  separate trail segments. 1 hour covers normal ADS-B gaps during a single
 *  flight while splitting across different flight legs / multi-day data. */
export const GAP_THRESHOLD_MS = 60 * 60 * 1000;

/** Multiplier applied to trail opacity for historical (non-recent) segments. */
export const HISTORY_OPACITY_FACTOR = 0.4;

/** Number of interpolated samples between each pair of Catmull-Rom control points. */
export const SAMPLES_PER_SEGMENT = 8;

/** Catmull-Rom scalar interpolation between p1 and p2, using p0/p3 as tangent guides. */
export function catmullRom(p0: number, p1: number, p2: number, p3: number, t: number): number {
  const t2 = t * t;
  const t3 = t2 * t;
  return (
    0.5 *
    (2 * p1 +
      (-p0 + p2) * t +
      (2 * p0 - 5 * p1 + 4 * p2 - p3) * t2 +
      (-p0 + 3 * p1 - 3 * p2 + p3) * t3)
  );
}

/** Geographic coordinate produced by trail interpolation (Cesium-free). */
export interface TrailGeoPoint {
  lon: number;
  lat: number;
  alt: number;
}

/**
 * Interpolates trail points using Catmull-Rom spline in geographic coordinate
 * space (lon/lat/alt). Returns geographic coordinates — the caller converts
 * to Cartesian3.
 *
 * Operating in geographic space avoids the ECEF overshoot artifacts that occur
 * when interpolating Cartesian3 positions directly (the spline would cut
 * through the Earth's interior for distant points).
 *
 * All trails use actual altitudeM from the observation data. This is correct
 * for both orbital entities (satellites at ~500 km) and atmospheric entities
 * (flights at ~10 km). Previous implementations forced altitude to 0 for
 * flight trails to work around ECEF spline artifacts — that workaround is no
 * longer needed since interpolation now operates in geographic space.
 *
 * Falls back to linear (no interpolation) for ≤2 points.
 */
export function interpolateTrailGeographic(points: TrailPoint[]): TrailGeoPoint[] {
  if (points.length <= 2) {
    return points.map((p) => ({ lon: p.lon, lat: p.lat, alt: p.altitudeM }));
  }

  const result: TrailGeoPoint[] = [];
  const n = points.length;

  for (let i = 0; i < n - 1; i++) {
    const p0 = points[Math.max(i - 1, 0)];
    const p1 = points[i];
    const p2 = points[i + 1];
    const p3 = points[Math.min(i + 2, n - 1)];

    // Last segment includes the final point (t=1); others stop just before
    const count = i === n - 2 ? SAMPLES_PER_SEGMENT + 1 : SAMPLES_PER_SEGMENT;
    for (let s = 0; s < count; s++) {
      const t = s / SAMPLES_PER_SEGMENT;
      result.push({
        lon: catmullRom(p0.lon, p1.lon, p2.lon, p3.lon, t),
        lat: catmullRom(p0.lat, p1.lat, p2.lat, p3.lat, t),
        alt: catmullRom(p0.altitudeM, p1.altitudeM, p2.altitudeM, p3.altitudeM, t),
      });
    }
  }

  return result;
}

/**
 * Splits trail points into segments at time gaps > GAP_THRESHOLD_MS.
 * Points are expected oldest-first (ascending timestamp).
 */
export function splitTrailSegments(points: TrailPoint[]): TrailPoint[][] {
  if (points.length === 0) return [];
  const segments: TrailPoint[][] = [[points[0]]];
  for (let i = 1; i < points.length; i++) {
    if (points[i].ts - points[i - 1].ts > GAP_THRESHOLD_MS) {
      segments.push([points[i]]);
    } else {
      segments[segments.length - 1].push(points[i]);
    }
  }
  return segments;
}

/**
 * Computes the opacity for a trail segment based on its position.
 * The last segment (most recent flight) gets full opacity; all
 * preceding segments are faded by HISTORY_OPACITY_FACTOR.
 */
export function segmentOpacity(
  baseOpacity: number,
  segmentIndex: number,
  totalSegments: number,
): number {
  const isRecent = segmentIndex === totalSegments - 1;
  return isRecent ? baseOpacity : baseOpacity * HISTORY_OPACITY_FACTOR;
}
