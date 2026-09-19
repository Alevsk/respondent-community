/**
 * entityUtils Unit Tests
 *
 * Tests for color parsing, size helpers, heading conversion, backward-motion
 * detection, and billboard scanning utilities.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock cesium before importing the module under test.
// BillboardCollection is defined as a real class so that `instanceof` checks
// inside findEntityBillboard work correctly against objects created in tests.
vi.mock('cesium', () => {
  const WHITE = { r: 1, g: 1, b: 1, alpha: 1, _isWhite: true };
  const MockColor = {
    WHITE,
    fromCssColorString: vi.fn((css: string) => {
      if (css === 'invalid-color') throw new Error('Invalid CSS color');
      return { r: 0, g: 0, b: 1, alpha: 1, _css: css };
    }),
  };

  // Cartographic.fromCartesian: MockCartesian3 stores longitude in x (degrees)
  // and latitude in y (degrees), so convert to radians here to match the
  // real Cesium Cartographic convention (values in radians).
  const MockCartographic = {
    fromCartesian: vi.fn((pos: { x: number; y: number; z?: number }) => ({
      longitude: (pos.x * Math.PI) / 180,
      latitude: (pos.y * Math.PI) / 180,
      height: pos.z ?? 0,
    })),
  };

  const MockCesiumMath = {
    toRadians: vi.fn((deg: number) => (deg * Math.PI) / 180),
    toDegrees: vi.fn((rad: number) => (rad * 180) / Math.PI),
  };

  // BillboardCollection is a class so that `instanceof BillboardCollection`
  // returns true for objects constructed with `new MockBillboardCollection(...)`.
  class MockBillboardCollection {
    private _billboards: MockBillboardData[];
    private _destroyed: boolean;

    constructor(billboards: MockBillboardData[] = [], destroyed = false) {
      this._billboards = billboards;
      this._destroyed = destroyed;
    }

    get length() {
      return this._billboards.length;
    }

    get(i: number) {
      return this._billboards[i];
    }

    isDestroyed() {
      return this._destroyed;
    }
  }

  return {
    Color: MockColor,
    Cartographic: MockCartographic,
    Math: MockCesiumMath,
    BillboardCollection: MockBillboardCollection,
  };
});

import {
  getEntityColor,
  getEntitySize,
  extractHeadingRadians,
  isBackwardMotion,
  findEntityBillboard,
} from './entityUtils';
import {
  Color,
  Math as CesiumMath,
  BillboardCollection,
  type Viewer,
  type Cartesian3,
} from 'cesium';

type MockBillboardData = { id: { entityId: string } };

type MockBBCtor = new (billboards: MockBillboardData[], destroyed?: boolean) => BillboardCollection;

describe('entityUtils', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getEntityColor', () => {
    it('returns parsed Color for a valid CSS color string', () => {
      const result = getEntityColor('satellites', '#ff0000');
      expect(Color.fromCssColorString).toHaveBeenCalledWith('#ff0000');
      expect((result as unknown as { _css: string })._css).toBe('#ff0000');
    });

    it('returns Color.WHITE when no color is provided', () => {
      const result = getEntityColor('satellites');
      expect(Color.fromCssColorString).not.toHaveBeenCalled();
      expect(result).toBe(Color.WHITE);
    });

    it('returns Color.WHITE when color string is empty', () => {
      const result = getEntityColor('satellites', '');
      expect(Color.fromCssColorString).not.toHaveBeenCalled();
      expect(result).toBe(Color.WHITE);
    });

    it('returns Color.WHITE when CSS color string is invalid', () => {
      const result = getEntityColor('satellites', 'invalid-color');
      expect(Color.fromCssColorString).toHaveBeenCalledWith('invalid-color');
      // Should catch the error thrown by the mock and fall through to default
      expect(result).toBe(Color.WHITE);
    });

    it('ignores layerType and uses color argument', () => {
      getEntityColor('flights_commercial', 'rgb(0,255,0)');
      expect(Color.fromCssColorString).toHaveBeenCalledWith('rgb(0,255,0)');
    });
  });

  describe('getEntitySize', () => {
    it('returns pointSize when it is a positive number', () => {
      expect(getEntitySize('satellites', 12)).toBe(12);
    });

    it('returns pointSize of 1 (minimum positive)', () => {
      expect(getEntitySize('satellites', 1)).toBe(1);
    });

    it('returns default size (8) when pointSize is undefined', () => {
      expect(getEntitySize('satellites')).toBe(8);
    });

    it('returns default size (8) when pointSize is 0', () => {
      expect(getEntitySize('satellites', 0)).toBe(8);
    });

    it('returns default size (8) when pointSize is negative', () => {
      expect(getEntitySize('satellites', -5)).toBe(8);
    });

    it('ignores layerType for size calculation', () => {
      expect(getEntitySize('earthquakes', 16)).toBe(16);
      expect(getEntitySize('flights_commercial', 16)).toBe(16);
    });
  });

  describe('extractHeadingRadians', () => {
    it('converts 0 degrees (north) to 0 radians', () => {
      const result = extractHeadingRadians(0);
      expect(CesiumMath.toRadians).toHaveBeenCalledWith(0);
      expect(result).toBe(-0);
    });

    it('converts 90 degrees (east) to -PI/2 radians', () => {
      const result = extractHeadingRadians(90);
      expect(CesiumMath.toRadians).toHaveBeenCalledWith(90);
      expect(result).toBeCloseTo(-Math.PI / 2, 10);
    });

    it('converts 180 degrees (south) to -PI radians', () => {
      const result = extractHeadingRadians(180);
      expect(result).toBeCloseTo(-Math.PI, 10);
    });

    it('converts 270 degrees (west) to -3*PI/2 radians', () => {
      const result = extractHeadingRadians(270);
      expect(result).toBeCloseTo((-3 * Math.PI) / 2, 10);
    });

    it('negates the angle (clockwise degrees to counter-clockwise radians)', () => {
      const result = extractHeadingRadians(45);
      expect(result).toBeCloseTo(-(45 * Math.PI) / 180, 10);
    });
  });

  // ---------------------------------------------------------------------------
  // isBackwardMotion
  // ---------------------------------------------------------------------------
  // Helper: build a mock Cartesian3 whose x = longitude (degrees) and
  // y = latitude (degrees). Cartographic.fromCartesian in the mock converts
  // these to radians, which is what the real Cesium convention uses.
  // ---------------------------------------------------------------------------
  describe('isBackwardMotion', () => {
    // Shorthand to create a position object understood by the mock.
    // The real signature wants a Cartesian3 instance (with .clone/.equals etc.),
    // but the mocked Cartographic.fromCartesian only reads x/y/z — the double
    // cast is the minimum needed to satisfy the nominal Cartesian3 class type
    // without constructing a real Cesium object in a unit test.
    function pos(lonDeg: number, latDeg: number): Cartesian3 {
      return { x: lonDeg, y: latDeg, z: 0 } as unknown as Cartesian3;
    }

    it('returns false when heading change exceeds 30° (turning flight)', () => {
      // Current heading: north (0 rad), new heading: 45° (PI/4 rad) — delta = PI/4 > PI/6
      const result = isBackwardMotion(
        pos(0, 0),
        pos(0, -0.01), // position doesn't matter — heading check fires first
        0,
        Math.PI / 4,
      );
      expect(result).toBe(false);
    });

    it('returns false when new position is ahead of current along heading (northbound)', () => {
      // Heading: north (0 rad). Moving north means increasing latitude.
      // new position is ~111 km north of current → well ahead.
      const currentHeading = 0; // north
      const result = isBackwardMotion(
        pos(0, 0), // current at equator
        pos(0, 1), // new server pos 1° north (~111 km ahead)
        currentHeading,
        currentHeading,
      );
      expect(result).toBe(false);
    });

    it('returns true when new position is >100 m behind along heading (northbound)', () => {
      // Heading: north (0 rad). Moving backward means decreasing latitude.
      // 0.01° latitude ≈ 1113 m → well past the 100 m threshold.
      const currentHeading = 0; // north
      const result = isBackwardMotion(
        pos(0, 0), // current at equator
        pos(0, -0.01), // new server pos ~1113 m behind (south of current)
        currentHeading,
        currentHeading,
      );
      expect(result).toBe(true);
    });

    it('returns true when new position is >100 m behind along heading (eastbound)', () => {
      // Heading: east (PI/2 rad). Moving backward means decreasing longitude.
      // At equator 1° lon ≈ 111 km; 0.01° ≈ 1113 m — past the 100 m threshold.
      const currentHeading = Math.PI / 2; // east
      const result = isBackwardMotion(
        pos(10, 0), // current position
        pos(9.99, 0), // new server pos ~1113 m west (behind for eastbound)
        currentHeading,
        currentHeading,
      );
      expect(result).toBe(true);
    });

    it('returns false when new position is only slightly behind (<100 m threshold)', () => {
      // Heading: north (0 rad). 0.0005° latitude ≈ 55 m — below the 100 m cutoff.
      const currentHeading = 0; // north
      const result = isBackwardMotion(
        pos(0, 0),
        pos(0, -0.0005), // ~55 m behind — not enough to trigger
        currentHeading,
        currentHeading,
      );
      expect(result).toBe(false);
    });

    it('handles heading wrapping: 350° to 10° is a 20° change, not 340°', () => {
      // Without wrapping normalization a naïve delta would be 10 - 350 = -340°,
      // which maps to ≈ -5.93 rad — far above MAX_HEADING_DELTA_RAD — and the
      // function would return false (turn detected). With correct normalization
      // the delta is +20° = 0.349 rad < PI/6 (0.524 rad) so the position check
      // runs instead.
      const heading350 = (350 * Math.PI) / 180;
      const heading10 = (10 * Math.PI) / 180;
      // Place new server pos 200 m behind a northbound entity so that if the
      // heading check passes the projection check will return true.
      const result = isBackwardMotion(
        pos(0, 0),
        pos(0, -0.002), // ~222 m behind — past threshold if heading check passes
        heading350,
        heading10,
      );
      // Heading delta of 20° is < 30°, so the position check runs.
      // New pos is behind → expect true.
      expect(result).toBe(true);
    });
  });

  // ---------------------------------------------------------------------------
  // findEntityBillboard
  // ---------------------------------------------------------------------------
  // Builds minimal mock Viewer / scene / primitives objects that satisfy the
  // shape the function accesses: viewer.scene.primitives.{length, get(i)}.
  // BillboardCollection objects are created with `new BillboardCollection(...)`
  // so that `instanceof BillboardCollection` resolves correctly inside the SUT.
  // ---------------------------------------------------------------------------
  describe('findEntityBillboard', () => {
    function makeBillboard(entityId: string): MockBillboardData {
      return { id: { entityId } };
    }

    function makeViewer(primitives: unknown[]): Viewer {
      return {
        scene: {
          primitives: {
            length: primitives.length,
            get: (i: number) => primitives[i],
          },
        },
      } as unknown as Viewer;
    }

    it('returns the matching billboard when found', () => {
      const target = makeBillboard('entity-42');
      const collection = new (BillboardCollection as unknown as MockBBCtor)([target]);
      const viewer = makeViewer([collection]);

      const result = findEntityBillboard(viewer, 'entity-42');

      expect(result).toBe(target);
    });

    it('returns null when no billboard matches the entityId', () => {
      const collection = new (BillboardCollection as unknown as MockBBCtor)([
        makeBillboard('entity-1'),
      ]);
      const viewer = makeViewer([collection]);

      const result = findEntityBillboard(viewer, 'entity-999');

      expect(result).toBeNull();
    });

    it('skips the excludeCollection and does not search it', () => {
      const targetBillboard = makeBillboard('entity-42');
      const excludedCollection = new (BillboardCollection as unknown as MockBBCtor)([
        targetBillboard,
      ]);
      const otherCollection = new (BillboardCollection as unknown as MockBBCtor)([]);
      const viewer = makeViewer([excludedCollection, otherCollection]);

      const result = findEntityBillboard(viewer, 'entity-42', excludedCollection);

      // The matching billboard lives only in the excluded collection — should not be found.
      expect(result).toBeNull();
    });

    it('skips destroyed BillboardCollections', () => {
      const targetBillboard = makeBillboard('entity-42');
      const destroyedCollection = new (BillboardCollection as unknown as MockBBCtor)(
        [targetBillboard],
        true,
      );
      const viewer = makeViewer([destroyedCollection]);

      const result = findEntityBillboard(viewer, 'entity-42');

      expect(result).toBeNull();
    });

    it('returns null when scene has no BillboardCollections (only other primitives)', () => {
      // Plain objects are not instances of BillboardCollection and will be skipped.
      const nonBillboardPrimitive = {
        length: 1,
        get: () => makeBillboard('entity-42'),
        isDestroyed: () => false,
      };
      const viewer = makeViewer([nonBillboardPrimitive]);

      const result = findEntityBillboard(viewer, 'entity-42');

      expect(result).toBeNull();
    });
  });
});
