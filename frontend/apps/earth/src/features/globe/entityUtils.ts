import {
  type Viewer,
  type Billboard,
  BillboardCollection,
  Cartesian3,
  Cartographic,
  Color,
  Math as CesiumMath,
} from 'cesium';

const DEFAULT_COLOR = Color.WHITE;
const DEFAULT_SIZE = 8;

// Entity styling helpers that use backend-provided configuration.
// The color/pointSize come from the Layer API response, with sensible defaults.

export function getEntityColor(_layerType: string, color?: string): Color {
  if (color) {
    try {
      return Color.fromCssColorString(color);
    } catch {
      // Fall through to default
    }
  }
  return DEFAULT_COLOR;
}

export function getEntitySize(_layerType: string, pointSize?: number): number {
  return pointSize && pointSize > 0 ? pointSize : DEFAULT_SIZE;
}

/**
 * Converts a heading in degrees (clockwise from north) to Cesium billboard
 * rotation in radians (counter-clockwise from east/screen-up).
 *
 * Backend sends heading as: 0=N, 90=E, 180=S, 270=W (clockwise degrees)
 * Cesium billboard.rotation expects: counter-clockwise radians from screen-up
 *
 * The conversion negates the angle (CW -> CCW) and converts to radians.
 */
export function extractHeadingRadians(headingDegrees: number): number {
  return -CesiumMath.toRadians(headingDegrees);
}

/** Conversion factor: 1 knot = 0.514444 m/s */
export const KNOTS_TO_MPS = 0.514444;

/** Maximum extrapolation time in seconds before freezing position */
export const MAX_EXTRAPOLATE_SEC = 60;

/**
 * Project a Cartesian3 position along a heading by a given distance (meters).
 * Uses spherical approximation which is accurate to within meters for
 * distances under ~15km (60s of flight at typical speeds).
 *
 * @param fromPos  Starting position in Cartesian3
 * @param headingRad  Heading in radians, clockwise from north (geographic convention)
 * @param distanceM  Distance to project in meters
 * @returns New Cartesian3 position
 */
export function projectPosition(
  fromPos: Cartesian3,
  headingRad: number,
  distanceM: number,
): Cartesian3 {
  const carto = Cartographic.fromCartesian(fromPos);
  // Approximate: at Earth's surface, 1 degree latitude ≈ 111,320 meters
  const dLatDeg = (distanceM * Math.cos(headingRad)) / 111320;
  const dLonDeg = (distanceM * Math.sin(headingRad)) / (111320 * Math.cos(carto.latitude));
  return Cartesian3.fromRadians(
    carto.longitude + CesiumMath.toRadians(dLonDeg),
    carto.latitude + CesiumMath.toRadians(dLatDeg),
    carto.height,
  );
}

/**
 * Find an entity's billboard by scanning all BillboardCollections in the scene.
 * Skips the provided excludeCollection (e.g. bracket overlays).
 * Returns the billboard with its current interpolated/extrapolated position.
 */
export function findEntityBillboard(
  viewer: Viewer,
  entityId: string,
  excludeCollection?: BillboardCollection,
): Billboard | null {
  const primitives = viewer.scene.primitives;
  for (let p = 0; p < primitives.length; p++) {
    const prim = primitives.get(p);
    if (!(prim instanceof BillboardCollection) || prim === excludeCollection) continue;
    if (prim.isDestroyed()) continue;

    for (let i = 0; i < prim.length; i++) {
      const bb = prim.get(i);
      const bbId = (bb as unknown as { id?: { entityId?: string } }).id;
      if (bbId?.entityId === entityId) return bb;
    }
  }
  return null;
}

/** Minimum backward distance (meters) to trigger the catchup pause heuristic. */
const BACKWARD_THRESHOLD_M = 100;

/** Maximum heading change (radians) before disabling backward detection (turns). */
const MAX_HEADING_DELTA_RAD = Math.PI / 6; // 30°

/**
 * Detect if a new server position is "behind" the current extrapolated billboard
 * position along the entity's heading direction. Used to avoid jarring backward
 * lerp animations when dead-reckoning over-predicted.
 *
 * Only triggers when:
 * - The heading hasn't changed significantly (< 30°, entity flying straight)
 * - The new position is at least 100m behind the current position along the heading
 */
export function isBackwardMotion(
  currentPos: Cartesian3,
  newServerPos: Cartesian3,
  currentHeadingRad: number,
  newHeadingRad: number,
): boolean {
  // Normalize heading delta to [-π, π]
  let headingDelta = newHeadingRad - currentHeadingRad;
  if (headingDelta > Math.PI) headingDelta -= 2 * Math.PI;
  if (headingDelta < -Math.PI) headingDelta += 2 * Math.PI;
  if (Math.abs(headingDelta) > MAX_HEADING_DELTA_RAD) return false;

  const currentCarto = Cartographic.fromCartesian(currentPos);
  const newCarto = Cartographic.fromCartesian(newServerPos);

  // Delta in meters (spherical approximation)
  const dLatM =
    (CesiumMath.toDegrees(newCarto.latitude) - CesiumMath.toDegrees(currentCarto.latitude)) *
    111320;
  const dLonM =
    (CesiumMath.toDegrees(newCarto.longitude) - CesiumMath.toDegrees(currentCarto.longitude)) *
    111320 *
    Math.cos(currentCarto.latitude);

  // Project displacement onto heading vector (heading: CW from north)
  const hx = Math.sin(currentHeadingRad);
  const hy = Math.cos(currentHeadingRad);
  const projection = dLonM * hx + dLatM * hy;

  return projection < -BACKWARD_THRESHOLD_M;
}
