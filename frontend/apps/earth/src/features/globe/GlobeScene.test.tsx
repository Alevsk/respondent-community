/**
 * GlobeScene — viewport computation logic tests
 *
 * GlobeScene's updateCameraState callback contains two distinct concerns:
 *   1. Camera state extraction (lat/lon/altitude/heading/pitch/roll)
 *   2. Viewport bounding-box computation, with a synthetic viewport branch
 *      that activates at high altitude ("full-globe" view).
 *
 * Because GlobeScene mounts a real Cesium Viewer (WebGL, canvas, workers) that
 * cannot run in jsdom, we test the viewport-computation logic as a pure
 * function extracted verbatim from the component.  This is the recommended
 * pattern in this codebase: isolate the pure decision logic and unit-test it
 * without rendering the full component.
 *
 * Covered cases:
 * - Normal viewport: passthrough unchanged (east-west span ≤ 350°)
 * - Full-globe viewport + successful pickEllipsoid → synthetic ±30° box
 * - Full-globe viewport + failed pickEllipsoid (returns undefined) → raw fallback
 * - Synthetic viewport clamping at ±180 lon / ±90 lat boundaries
 * - Exact threshold boundary: (east-west) exactly 350° is NOT full-globe
 * - Exact threshold boundary: (east-west) > 350° AND (north-south) > 170° → full-globe
 * - Full-globe with only width exceeding threshold → not full-globe (height check)
 */

import { describe, it, expect } from 'vitest';
import type { ViewportBBox } from './store';

// ---------------------------------------------------------------------------
// Pure viewport-computation function
//
// This mirrors the exact logic inside updateCameraState (GlobeScene.tsx lines
// 96-134) so that the decision rules can be exercised without Cesium.
// If the source logic changes, update this mirror to match.
// ---------------------------------------------------------------------------

interface RawViewRect {
  west: number; // degrees
  south: number; // degrees
  east: number; // degrees
  north: number; // degrees
}

interface PickEllipsoidResult {
  longitude: number; // degrees
  latitude: number; // degrees
}

/**
 * Computes the viewport bounding box that GlobeScene.updateCameraState
 * would store via setViewport, given the raw camera view-rectangle and an
 * optional look-at point from pickEllipsoid.
 *
 * @param viewRect    Raw west/south/east/north in degrees from computeViewRectangle()
 * @param lookAt      Result of camera.pickEllipsoid converted to degrees, or
 *                    undefined when the ray misses the ellipsoid.
 * @returns           The ViewportBBox that would be passed to setViewport.
 */
function computeViewport(
  viewRect: RawViewRect,
  lookAt: PickEllipsoidResult | undefined,
): ViewportBBox {
  const { west, south, east, north } = viewRect;

  // Detect full-globe viewport (high altitude / orbital zoom)
  const isFullGlobe = east - west > 350 && north - south > 170;

  if (isFullGlobe) {
    if (lookAt) {
      const centerLon = lookAt.longitude;
      const centerLat = lookAt.latitude;
      const halfSpan = 30;
      return {
        west: Math.max(centerLon - halfSpan, -180),
        south: Math.max(centerLat - halfSpan, -90),
        east: Math.min(centerLon + halfSpan, 180),
        north: Math.min(centerLat + halfSpan, 90),
      };
    }
    // pickEllipsoid failed — fall back to raw viewport
    return { west, south, east, north };
  }

  // Normal viewport passes through unchanged
  return { west, south, east, north };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Builds a view-rect that triggers the full-globe branch. */
function fullGlobeRect(overrides: Partial<RawViewRect> = {}): RawViewRect {
  return {
    west: -179,
    south: -89,
    east: 179, // east - west = 358 > 350
    north: 89, // north - south = 178 > 170
    ...overrides,
  };
}

/** Builds a view-rect for a normal (non-full-globe) zoom level. */
function normalRect(overrides: Partial<RawViewRect> = {}): RawViewRect {
  return {
    west: -10,
    south: -5,
    east: 10,
    north: 5,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('computeViewport — normal viewport', () => {
  it('returns the raw rectangle unchanged when the view is not full-globe', () => {
    const rect = normalRect();
    const result = computeViewport(rect, undefined);

    expect(result).toEqual({ west: -10, south: -5, east: 10, north: 5 });
  });

  it('ignores the lookAt point when the view is not full-globe', () => {
    const rect = normalRect();
    const lookAt: PickEllipsoidResult = { longitude: 45, latitude: 20 };
    const result = computeViewport(rect, lookAt);

    // lookAt should have no effect — raw rect is returned as-is
    expect(result).toEqual({ west: -10, south: -5, east: 10, north: 5 });
  });

  it('treats east-west span of exactly 350° as normal (boundary — not full-globe)', () => {
    // east - west = 350 is NOT > 350, so should pass through
    const rect: RawViewRect = { west: -175, south: -89, east: 175, north: 89 };
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result).toEqual({ west: -175, south: -89, east: 175, north: 89 });
  });

  it('is not full-globe when width > 350° but height ≤ 170°', () => {
    // Width 358° but height only 100° → isFullGlobe = false
    const rect: RawViewRect = { west: -179, south: -50, east: 179, north: 50 };
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result).toEqual({ west: -179, south: -50, east: 179, north: 50 });
  });

  it('is not full-globe when height > 170° but width ≤ 350°', () => {
    // Height 178° but width only 20° → isFullGlobe = false
    const rect: RawViewRect = { west: -10, south: -89, east: 10, north: 89 };
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result).toEqual({ west: -10, south: -89, east: 10, north: 89 });
  });
});

describe('computeViewport — full-globe viewport with successful pickEllipsoid', () => {
  it('returns a synthetic ±30° box centered on the look-at point', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 20, latitude: 10 };
    const result = computeViewport(rect, lookAt);

    expect(result).toEqual({
      west: -10, // 20 - 30
      south: -20, // 10 - 30
      east: 50, // 20 + 30
      north: 40, // 10 + 30
    });
  });

  it('produces a box exactly 60° wide and 60° tall when unclamped', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    const width = result.east - result.west;
    const height = result.north - result.south;
    expect(width).toBe(60);
    expect(height).toBe(60);
  });

  it('works correctly when the look-at point is at the prime meridian / equator', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result).toEqual({ west: -30, south: -30, east: 30, north: 30 });
  });

  it('applies the halfSpan = 30 constant and not any other value', () => {
    // Verify the half-span is exactly 30° by checking a known center
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 50, latitude: 50 };
    const result = computeViewport(rect, lookAt);

    expect(result.east - result.west).toBe(60);
    expect(result.north - result.south).toBe(60);
    expect(result.west).toBe(20);
    expect(result.south).toBe(20);
  });
});

describe('computeViewport — full-globe viewport with failed pickEllipsoid', () => {
  it('falls back to the raw view-rectangle when pickEllipsoid returns undefined', () => {
    const rect = fullGlobeRect();
    const result = computeViewport(rect, undefined);

    expect(result).toEqual({
      west: rect.west,
      south: rect.south,
      east: rect.east,
      north: rect.north,
    });
  });

  it('returns the full [-180,-90,180,90] box when the raw rect spans the globe and no lookAt', () => {
    const rect: RawViewRect = { west: -180, south: -90, east: 180, north: 90 };
    const result = computeViewport(rect, undefined);

    expect(result).toEqual({ west: -180, south: -90, east: 180, north: 90 });
  });
});

describe('computeViewport — synthetic viewport boundary clamping', () => {
  it('clamps west to -180 when center is near the antimeridian (west side)', () => {
    const rect = fullGlobeRect();
    // centerLon - 30 = -170 - 30 = -200, clamped to -180
    const lookAt: PickEllipsoidResult = { longitude: -170, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result.west).toBe(-180);
    expect(result.east).toBe(-140); // -170 + 30
  });

  it('clamps east to 180 when center is near the antimeridian (east side)', () => {
    const rect = fullGlobeRect();
    // centerLon + 30 = 170 + 30 = 200, clamped to 180
    const lookAt: PickEllipsoidResult = { longitude: 170, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result.east).toBe(180);
    expect(result.west).toBe(140); // 170 - 30
  });

  it('clamps south to -90 when look-at is near the south pole', () => {
    const rect = fullGlobeRect();
    // centerLat - 30 = -75 - 30 = -105, clamped to -90
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: -75 };
    const result = computeViewport(rect, lookAt);

    expect(result.south).toBe(-90);
    expect(result.north).toBe(-45); // -75 + 30
  });

  it('clamps north to 90 when look-at is near the north pole', () => {
    const rect = fullGlobeRect();
    // centerLat + 30 = 75 + 30 = 105, clamped to 90
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 75 };
    const result = computeViewport(rect, lookAt);

    expect(result.north).toBe(90);
    expect(result.south).toBe(45); // 75 - 30
  });

  it('clamps all four edges simultaneously at a corner (e.g. north-east pole region)', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 170, latitude: 80 };
    const result = computeViewport(rect, lookAt);

    // west = 170 - 30 = 140 (no clamp)
    // east = 170 + 30 = 200 → clamped to 180
    // south = 80 - 30 = 50 (no clamp)
    // north = 80 + 30 = 110 → clamped to 90
    expect(result.west).toBe(140);
    expect(result.east).toBe(180);
    expect(result.south).toBe(50);
    expect(result.north).toBe(90);
  });

  it('never produces a west value below -180', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: -180, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result.west).toBeGreaterThanOrEqual(-180);
  });

  it('never produces an east value above 180', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 180, latitude: 0 };
    const result = computeViewport(rect, lookAt);

    expect(result.east).toBeLessThanOrEqual(180);
  });

  it('never produces a south value below -90', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: -90 };
    const result = computeViewport(rect, lookAt);

    expect(result.south).toBeGreaterThanOrEqual(-90);
  });

  it('never produces a north value above 90', () => {
    const rect = fullGlobeRect();
    const lookAt: PickEllipsoidResult = { longitude: 0, latitude: 90 };
    const result = computeViewport(rect, lookAt);

    expect(result.north).toBeLessThanOrEqual(90);
  });
});

describe('computeViewport — full-globe threshold boundary conditions', () => {
  it('triggers full-globe when width is 350.001° and height is 170.001°', () => {
    // Both just above the threshold → isFullGlobe = true → synthetic box used
    const rect: RawViewRect = {
      west: -175.0005,
      south: -85.0005,
      east: 175.0005, // east - west ≈ 350.001
      north: 85.0005, // north - south ≈ 170.001
    };
    const lookAt: PickEllipsoidResult = { longitude: 10, latitude: 5 };
    const result = computeViewport(rect, lookAt);

    // Should have used the synthetic box, not the raw rect
    expect(result.west).toBe(-20); // 10 - 30
    expect(result.south).toBe(-25); // 5 - 30
    expect(result.east).toBe(40); // 10 + 30
    expect(result.north).toBe(35); // 5 + 30
  });

  it('does not trigger full-globe when width is 350.001° but height is exactly 170°', () => {
    // Height exactly 170 is NOT > 170, so isFullGlobe = false
    const rect: RawViewRect = {
      west: -175.0005,
      east: 175.0005, // east - west ≈ 350.001 → passes width check
      south: -85,
      north: 85, // north - south = 170 → fails height check (not > 170)
    };
    const lookAt: PickEllipsoidResult = { longitude: 10, latitude: 5 };
    const result = computeViewport(rect, lookAt);

    // Raw rect should be returned since isFullGlobe is false
    expect(result).toEqual({
      west: rect.west,
      south: rect.south,
      east: rect.east,
      north: rect.north,
    });
  });
});
