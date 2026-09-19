/**
 * trailUtils — unit tests for trail segment splitting, opacity logic,
 * Catmull-Rom interpolation, and geographic trail interpolation.
 *
 * These tests guard against two regressions:
 * 1. Trail altitude forced to 0 — satellites/flights must use actual altitudeM
 * 2. Spline smoothing removed — Catmull-Rom must operate in geographic space
 *    (not ECEF) to avoid overshoot through the Earth's interior
 */

import { describe, it, expect } from 'vitest';
import type { TrailPoint } from '@respondent/core';
import {
  splitTrailSegments,
  segmentOpacity,
  GAP_THRESHOLD_MS,
  HISTORY_OPACITY_FACTOR,
  SAMPLES_PER_SEGMENT,
  catmullRom,
  interpolateTrailGeographic,
} from './trailUtils';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Build a minimal TrailPoint at a given timestamp (ms). */
function pt(ts: number, lat = 40, lon = 32, altitudeM = 10000): TrailPoint {
  return { ts, lat, lon, altitudeM };
}

/** One hour in milliseconds — the gap threshold boundary. */
const ONE_HOUR = 60 * 60 * 1000;

// ---------------------------------------------------------------------------
// splitTrailSegments
// ---------------------------------------------------------------------------

describe('splitTrailSegments', () => {
  it('returns empty array for empty input', () => {
    expect(splitTrailSegments([])).toEqual([]);
  });

  it('returns a single segment when all points are within the gap threshold', () => {
    const points = [pt(1000), pt(2000), pt(3000)];
    const segments = splitTrailSegments(points);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveLength(3);
  });

  it('returns a single segment for one point', () => {
    const segments = splitTrailSegments([pt(1000)]);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveLength(1);
  });

  it('splits at a gap exactly exceeding the threshold', () => {
    const t0 = 0;
    const t1 = ONE_HOUR + 1; // just over 1 hour gap
    const points = [pt(t0), pt(t1)];
    const segments = splitTrailSegments(points);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toHaveLength(1);
    expect(segments[1]).toHaveLength(1);
  });

  it('does not split at a gap exactly equal to the threshold', () => {
    const t0 = 0;
    const t1 = ONE_HOUR; // exactly 1 hour — not greater than threshold
    const points = [pt(t0), pt(t1)];
    const segments = splitTrailSegments(points);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toHaveLength(2);
  });

  it('splits multi-day flight history into separate segments', () => {
    const day1Flight = [pt(1000), pt(2000), pt(3000)];
    // 4-day gap
    const day5Flight = [
      pt(3000 + 4 * 24 * ONE_HOUR),
      pt(3000 + 4 * 24 * ONE_HOUR + 1000),
      pt(3000 + 4 * 24 * ONE_HOUR + 2000),
    ];
    const points = [...day1Flight, ...day5Flight];
    const segments = splitTrailSegments(points);
    expect(segments).toHaveLength(2);
    expect(segments[0]).toHaveLength(3);
    expect(segments[1]).toHaveLength(3);
  });

  it('handles multiple gaps creating 3+ segments', () => {
    const base = 0;
    const gap = ONE_HOUR + 1;
    const seg1 = [pt(base), pt(base + 1000)];
    const seg2Start = base + 1000 + gap;
    const seg2 = [pt(seg2Start), pt(seg2Start + 1000)];
    const seg3Start = seg2Start + 1000 + gap;
    const seg3 = [pt(seg3Start), pt(seg3Start + 1000)];
    const points = [...seg1, ...seg2, ...seg3];
    const segments = splitTrailSegments(points);
    expect(segments).toHaveLength(3);
    expect(segments[0]).toHaveLength(2);
    expect(segments[1]).toHaveLength(2);
    expect(segments[2]).toHaveLength(2);
  });

  it('preserves point data through segmentation', () => {
    const p1 = pt(0, 35.5, 33.2);
    const p2 = pt(ONE_HOUR + 1, 40.0, 32.8);
    const segments = splitTrailSegments([p1, p2]);
    expect(segments[0][0]).toBe(p1);
    expect(segments[1][0]).toBe(p2);
  });

  it('exports GAP_THRESHOLD_MS as 1 hour in milliseconds', () => {
    expect(GAP_THRESHOLD_MS).toBe(ONE_HOUR);
  });
});

// ---------------------------------------------------------------------------
// segmentOpacity
// ---------------------------------------------------------------------------

describe('segmentOpacity', () => {
  it('returns full opacity for the last (most recent) segment', () => {
    expect(segmentOpacity(0.7, 2, 3)).toBe(0.7);
  });

  it('returns reduced opacity for historical segments', () => {
    const result = segmentOpacity(0.7, 0, 3);
    expect(result).toBeCloseTo(0.7 * HISTORY_OPACITY_FACTOR);
  });

  it('returns full opacity when there is only one segment', () => {
    expect(segmentOpacity(0.7, 0, 1)).toBe(0.7);
  });

  it('applies HISTORY_OPACITY_FACTOR to all non-last segments', () => {
    const base = 0.8;
    // 4 segments: indices 0, 1, 2 are historical; index 3 is recent
    expect(segmentOpacity(base, 0, 4)).toBeCloseTo(base * HISTORY_OPACITY_FACTOR);
    expect(segmentOpacity(base, 1, 4)).toBeCloseTo(base * HISTORY_OPACITY_FACTOR);
    expect(segmentOpacity(base, 2, 4)).toBeCloseTo(base * HISTORY_OPACITY_FACTOR);
    expect(segmentOpacity(base, 3, 4)).toBe(base);
  });

  it('works with opacity of 1.0', () => {
    expect(segmentOpacity(1.0, 0, 2)).toBeCloseTo(HISTORY_OPACITY_FACTOR);
    expect(segmentOpacity(1.0, 1, 2)).toBe(1.0);
  });

  it('exports HISTORY_OPACITY_FACTOR as 0.4', () => {
    expect(HISTORY_OPACITY_FACTOR).toBe(0.4);
  });
});

// ---------------------------------------------------------------------------
// catmullRom
// ---------------------------------------------------------------------------

describe('catmullRom', () => {
  it('returns p1 at t=0', () => {
    expect(catmullRom(0, 10, 20, 30, 0)).toBe(10);
  });

  it('returns p2 at t=1', () => {
    expect(catmullRom(0, 10, 20, 30, 1)).toBe(20);
  });

  it('returns midpoint at t=0.5 for uniformly spaced values', () => {
    expect(catmullRom(0, 10, 20, 30, 0.5)).toBeCloseTo(15);
  });

  it('produces smooth values between control points', () => {
    const t025 = catmullRom(0, 10, 20, 30, 0.25);
    const t050 = catmullRom(0, 10, 20, 30, 0.5);
    const t075 = catmullRom(0, 10, 20, 30, 0.75);
    // Values should be monotonically increasing for uniformly spaced inputs
    expect(t025).toBeGreaterThan(10);
    expect(t050).toBeGreaterThan(t025);
    expect(t075).toBeGreaterThan(t050);
    expect(t075).toBeLessThan(20);
  });

  it('handles constant values (all points equal)', () => {
    expect(catmullRom(5, 5, 5, 5, 0.5)).toBe(5);
  });

  it('handles negative values', () => {
    expect(catmullRom(-30, -20, -10, 0, 0)).toBe(-20);
    expect(catmullRom(-30, -20, -10, 0, 1)).toBe(-10);
  });

  it('exports SAMPLES_PER_SEGMENT as 8', () => {
    expect(SAMPLES_PER_SEGMENT).toBe(8);
  });
});

// ---------------------------------------------------------------------------
// interpolateTrailGeographic
// ---------------------------------------------------------------------------

describe('interpolateTrailGeographic', () => {
  describe('altitude preservation (regression: trails must NOT force altitude to 0)', () => {
    it('preserves satellite orbital altitude (~500 km)', () => {
      const satelliteAlt = 514_917;
      const points = [
        pt(0, 40, -80, satelliteAlt),
        pt(1000, 42, -78, satelliteAlt),
        pt(2000, 44, -76, satelliteAlt),
      ];
      const result = interpolateTrailGeographic(points);
      for (const p of result) {
        expect(p.alt).toBeGreaterThan(500_000);
        expect(p.alt).toBeLessThan(530_000);
      }
    });

    it('preserves flight cruise altitude (~10 km)', () => {
      const flightAlt = 10_363;
      const points = [
        pt(0, 40, -80, flightAlt),
        pt(1000, 42, -78, flightAlt),
        pt(2000, 44, -76, flightAlt),
      ];
      const result = interpolateTrailGeographic(points);
      for (const p of result) {
        expect(p.alt).toBeGreaterThan(10_000);
        expect(p.alt).toBeLessThan(11_000);
      }
    });

    it('never produces altitude 0 when input altitudes are nonzero', () => {
      const points = [pt(0, 40, -80, 10_000), pt(1000, 42, -78, 10_500), pt(2000, 44, -76, 11_000)];
      const result = interpolateTrailGeographic(points);
      for (const p of result) {
        expect(p.alt).toBeGreaterThan(0);
      }
    });

    it('interpolates ascending altitude smoothly (no underground values)', () => {
      const points = [
        pt(0, 40, -80, 1000), // climbing
        pt(1000, 41, -79, 5000),
        pt(2000, 42, -78, 10000), // cruise
        pt(3000, 43, -77, 10000),
      ];
      const result = interpolateTrailGeographic(points);
      expect(result[0].alt).toBeCloseTo(1000);
      expect(result[result.length - 1].alt).toBeCloseTo(10000);
      for (const p of result) {
        expect(p.alt).toBeGreaterThanOrEqual(0);
      }
    });

    it('preserves altitude for 2-point linear fallback (no spline)', () => {
      const points = [pt(0, 40, -80, 500_000), pt(1000, 42, -78, 510_000)];
      const result = interpolateTrailGeographic(points);
      expect(result).toHaveLength(2);
      expect(result[0].alt).toBe(500_000);
      expect(result[1].alt).toBe(510_000);
    });

    it('preserves altitude for single-point input', () => {
      const result = interpolateTrailGeographic([pt(0, 40, -80, 500_000)]);
      expect(result).toHaveLength(1);
      expect(result[0].alt).toBe(500_000);
    });
  });

  describe('Catmull-Rom smoothing (regression: spline must be active)', () => {
    it('produces more output points than input for 3+ points', () => {
      const points = [pt(0, 40, -80), pt(1000, 42, -78), pt(2000, 44, -76)];
      const result = interpolateTrailGeographic(points);
      expect(result.length).toBeGreaterThan(points.length);
      expect(result.length).toBe(2 * SAMPLES_PER_SEGMENT + 1);
    });

    it('first interpolated point matches first control point', () => {
      const points = [pt(0, 40, -80, 10000), pt(1000, 42, -78, 10000), pt(2000, 44, -76, 10000)];
      const result = interpolateTrailGeographic(points);
      expect(result[0].lat).toBeCloseTo(40);
      expect(result[0].lon).toBeCloseTo(-80);
    });

    it('last interpolated point matches last control point', () => {
      const points = [pt(0, 40, -80, 10000), pt(1000, 42, -78, 10000), pt(2000, 44, -76, 10000)];
      const result = interpolateTrailGeographic(points);
      const last = result[result.length - 1];
      expect(last.lat).toBeCloseTo(44);
      expect(last.lon).toBeCloseTo(-76);
    });

    it('interpolated values stay within bounding box of control points', () => {
      const points = [
        pt(0, 40, -80, 10000),
        pt(1000, 42, -78, 10000),
        pt(2000, 44, -76, 10000),
        pt(3000, 46, -74, 10000),
      ];
      const result = interpolateTrailGeographic(points);
      for (const p of result) {
        // Allow small overshoot from Catmull-Rom tangents (1 degree tolerance)
        expect(p.lat).toBeGreaterThanOrEqual(39);
        expect(p.lat).toBeLessThanOrEqual(47);
        expect(p.lon).toBeGreaterThanOrEqual(-81);
        expect(p.lon).toBeLessThanOrEqual(-73);
      }
    });

    it('produces correct point count: (N-1) * SAMPLES_PER_SEGMENT + 1', () => {
      for (const n of [3, 5, 10]) {
        const points = Array.from({ length: n }, (_, i) => pt(i * 1000, 40 + i, -80 + i, 10000));
        const result = interpolateTrailGeographic(points);
        const expected = (n - 1) * SAMPLES_PER_SEGMENT + 1;
        expect(result).toHaveLength(expected);
      }
    });

    it('returns empty for empty input', () => {
      expect(interpolateTrailGeographic([])).toEqual([]);
    });
  });

  describe('geographic-space interpolation (regression: no ECEF overshoot)', () => {
    it('distant satellite points stay at orbital altitude (no Earth interior cut-through)', () => {
      // Two points on opposite sides of the globe — in ECEF space a spline
      // would cut through the Earth. Geographic-space must stay at altitude.
      const orbitalAlt = 500_000;
      const points = [
        pt(0, 0, 0, orbitalAlt),
        pt(1000, 0, 60, orbitalAlt),
        pt(2000, 0, 120, orbitalAlt),
      ];
      const result = interpolateTrailGeographic(points);
      for (const p of result) {
        expect(p.alt).toBeGreaterThan(400_000);
      }
    });

    it('lon/lat monotonically increase for a simple ascending track', () => {
      const points = [
        pt(0, -30, -120, 500000),
        pt(300_000, 0, -60, 500000),
        pt(600_000, 30, 0, 500000),
        pt(900_000, 45, 60, 500000),
      ];
      const result = interpolateTrailGeographic(points);
      for (let i = 1; i < result.length; i++) {
        expect(result[i].lon).toBeGreaterThanOrEqual(result[i - 1].lon - 1);
      }
    });
  });
});
