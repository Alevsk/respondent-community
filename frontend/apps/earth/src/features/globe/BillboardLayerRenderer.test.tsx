/**
 * BillboardLayerRenderer Component Tests
 *
 * Tests the headless globe renderer component including:
 * - BillboardCollection lifecycle (create/destroy)
 * - Non-flight full rebuild path
 * - Flight animated diff path (add/update/remove via full rebuild)
 * - O(delta) incremental update path (dirtyEntityIds)
 * - POS_EPSILON: skip no-op animations for tiny position deltas
 * - Interpolation loop registration via scene.preUpdate
 * - Occlusion effect listener registration/cleanup
 * - Data normalization for various backend payload shapes
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, act } from '@testing-library/react';

interface MockBillboard {
  id: unknown;
  show: boolean;
  position: { x: number; y: number; z: number };
  imageId: string;
  image: HTMLCanvasElement | null;
  color: { r: number; g: number; b: number; alpha: number };
  scale: number;
  rotation: number;
  alignedAxis: { x: number; y: number; z: number } | null;
  disableDepthTestDistance: number;
}

interface MockBillboardAddOptions {
  id?: unknown;
  position?: { x: number; y: number; z: number };
  imageId?: string;
  image?: HTMLCanvasElement | null;
  color?: { r: number; g: number; b: number; alpha: number };
  scale?: number;
  rotation?: number;
  alignedAxis?: { x: number; y: number; z: number } | null;
  disableDepthTestDistance?: number;
}

interface MockColorLike {
  r: number;
  g: number;
  b: number;
  alpha: number;
  [key: string]: unknown;
}

interface TestEntity {
  id?: string;
  name?: string;
  layerType?: string;
  source?: string;
}

interface TestObservation {
  entityId: string;
  position: { lon: number | string; lat: number | string };
  altitudeM?: number | string;
  velocity?: { heading?: number | string; ground_speed?: number };
  metadata?: Record<string, string>;
}

// ---------------------------------------------------------------------------
// vi.hoisted() — variables declared here are available inside vi.mock() factories
// because vi.mock() calls are hoisted to the top of the file by the transform.
// ---------------------------------------------------------------------------

const {
  MockCartesian3,
  MockBillboardCollectionCtor,
  MockColor,
  MockEllipsoidalOccluder,
  MockEllipsoid,
  MockCesiumMath,
  mockUseLayerEntities,
  getLastCollectionInstance,
  getLastOccluderInstance,
} = vi.hoisted(() => {
  // ---------------------------------------------------------------------------
  // Cartesian3 mock
  // ---------------------------------------------------------------------------
  class MockCartesian3Impl {
    x: number;
    y: number;
    z: number;

    constructor(x = 0, y = 0, z = 0) {
      this.x = x;
      this.y = y;
      this.z = z;
    }

    static fromDegrees = vi.fn(
      (lon: number, lat: number, alt: number) => new MockCartesian3Impl(lon, lat, alt),
    );

    static clone = vi.fn((c: MockCartesian3Impl) => new MockCartesian3Impl(c.x, c.y, c.z));

    static lerp = vi.fn(
      (
        start: MockCartesian3Impl,
        end: MockCartesian3Impl,
        t: number,
        result: MockCartesian3Impl,
      ) => {
        result.x = start.x + (end.x - start.x) * t;
        result.y = start.y + (end.y - start.y) * t;
        result.z = start.z + (end.z - start.z) * t;
        return result;
      },
    );

    static UNIT_Z = new MockCartesian3Impl(0, 0, 1);
    static ZERO = new MockCartesian3Impl(0, 0, 0);
  }

  // ---------------------------------------------------------------------------
  // BillboardCollection mock
  // ---------------------------------------------------------------------------
  let _lastCollectionInstance: MockBillboardCollectionImpl | null = null;

  class MockBillboardCollectionImpl {
    _billboards: MockBillboard[] = [];
    _destroyed = false;

    // Keep `opts` required (not optional): the real component always supplies
    // options, and the tests assert on specific fields via
    // `collection.add.mock.calls[0][0]`. An optional param would make that
    // access `T | undefined` and force `!` assertions at every call site.
    add = vi.fn((opts: MockBillboardAddOptions) => {
      const bb: MockBillboard = {
        id: opts.id,
        show: true,
        position: opts.position ?? new MockCartesian3Impl(),
        imageId: opts.imageId ?? '',
        image: opts.image ?? null,
        color: opts.color ?? { r: 1, g: 1, b: 1, alpha: 1 },
        scale: opts.scale ?? 1,
        rotation: opts.rotation ?? 0,
        alignedAxis: opts.alignedAxis ?? null,
        disableDepthTestDistance: opts.disableDepthTestDistance ?? 0,
      };
      this._billboards.push(bb);
      return bb;
    });

    remove = vi.fn((bb: MockBillboard) => {
      const idx = this._billboards.indexOf(bb);
      if (idx !== -1) this._billboards.splice(idx, 1);
      return idx !== -1;
    });

    removeAll = vi.fn(() => {
      this._billboards = [];
    });

    get = vi.fn((i: number) => this._billboards[i]);

    get length() {
      return this._billboards.length;
    }

    isDestroyed = vi.fn(() => this._destroyed);

    destroy = vi.fn(() => {
      this._destroyed = true;
    });
  }

  const MockBillboardCollectionCtorImpl = vi.fn(function () {
    const instance = new MockBillboardCollectionImpl();
    _lastCollectionInstance = instance;
    return instance;
  });

  // ---------------------------------------------------------------------------
  // Color mock — must be constructable because the component does `new Color()`
  // ---------------------------------------------------------------------------
  const mockColorWhite = { r: 1, g: 1, b: 1, alpha: 1 };

  class MockColorImpl {
    r: number;
    g: number;
    b: number;
    alpha: number;

    constructor(r = 0, g = 0, b = 0, alpha = 1) {
      this.r = r;
      this.g = g;
      this.b = b;
      this.alpha = alpha;
    }

    // Mirrors Cesium's Color.prototype.withAlpha — returns a copy with a new
    // alpha, preserving any extra own props (e.g. _css from fromCssColorString).
    withAlpha(alpha: number): MockColorImpl {
      const copy = new MockColorImpl(this.r, this.g, this.b, alpha);
      for (const key of Object.keys(this)) {
        if (!(key in copy)) {
          (copy as unknown as Record<string, unknown>)[key] = (
            this as unknown as Record<string, unknown>
          )[key];
        }
      }
      return copy;
    }

    static WHITE = mockColorWhite;

    static fromCssColorString = vi.fn((css: string) => {
      if (css === 'invalid') throw new Error('bad color');
      return Object.assign(new MockColorImpl(0, 0, 1, 1), { _css: css });
    });

    static clone = vi.fn((src: MockColorLike, dest?: MockColorImpl) => {
      // When called with a single argument (Color.clone(src)), return a new Color copy
      // that includes all own enumerable properties of src (e.g. _css from fromCssColorString).
      // When called with two arguments (Color.clone(src, dest)), mutate dest in-place.
      if (!dest) {
        const copy = new MockColorImpl(src.r ?? 0, src.g ?? 0, src.b ?? 0, src.alpha ?? 1);
        // Copy any extra own properties (e.g. _css) that the source carries
        for (const key of Object.keys(src)) {
          if (!(key in MockColorImpl.prototype) && !(key in copy)) {
            (copy as unknown as Record<string, unknown>)[key] = src[key];
          }
        }
        return copy;
      }
      dest.r = src.r;
      dest.g = src.g;
      dest.b = src.b;
      dest.alpha = src.alpha;
      return dest;
    });
  }

  // Fix the fromCssColorString implementation to correctly return an object with _css
  MockColorImpl.fromCssColorString = vi.fn((css: string) => {
    if (css === 'invalid') throw new Error('bad color');
    return Object.assign(new MockColorImpl(0, 0, 1, 1), { _css: css });
  });

  // ---------------------------------------------------------------------------
  // EllipsoidalOccluder mock
  // ---------------------------------------------------------------------------
  let _lastOccluderInstance: MockEllipsoidalOccluderInstance | null = null;

  class MockEllipsoidalOccluderInstance {
    cameraPosition: MockCartesian3Impl | null = null;
    isPointVisible = vi.fn(() => true);
  }

  const MockEllipsoidalOccluderCtor = vi.fn(function () {
    const instance = new MockEllipsoidalOccluderInstance();
    _lastOccluderInstance = instance;
    return instance;
  });

  // ---------------------------------------------------------------------------
  // Ellipsoid mock
  // ---------------------------------------------------------------------------
  const MockEllipsoidImpl = {
    WGS84: { _isWGS84: true },
  };

  // ---------------------------------------------------------------------------
  // CesiumMath mock
  // ---------------------------------------------------------------------------
  const MockCesiumMathImpl = {
    lerp: vi.fn((a: number, b: number, t: number) => a + (b - a) * t),
    toRadians: vi.fn((deg: number) => (deg * Math.PI) / 180),
  };

  // ---------------------------------------------------------------------------
  // useLayerEntities mock
  // ---------------------------------------------------------------------------
  const mockUseLayerEntitiesImpl = vi.fn();

  return {
    MockCartesian3: MockCartesian3Impl,
    MockBillboardCollectionCtor: MockBillboardCollectionCtorImpl,
    MockColor: MockColorImpl,
    MockEllipsoidalOccluder: MockEllipsoidalOccluderCtor,
    MockEllipsoid: MockEllipsoidImpl,
    MockCesiumMath: MockCesiumMathImpl,
    mockUseLayerEntities: mockUseLayerEntitiesImpl,
    getLastCollectionInstance: () => _lastCollectionInstance,
    getLastOccluderInstance: () => _lastOccluderInstance,
  };
});

// ---------------------------------------------------------------------------
// Module mocks — factories can safely reference hoisted variables
// ---------------------------------------------------------------------------

vi.mock('cesium', () => ({
  BillboardCollection: MockBillboardCollectionCtor,
  Cartesian3: MockCartesian3,
  Color: MockColor,
  EllipsoidalOccluder: MockEllipsoidalOccluder,
  Ellipsoid: MockEllipsoid,
  Math: MockCesiumMath,
}));

vi.mock('../../shared/api/useLayerStream', () => ({
  useLayerEntities: (layerId: string) => mockUseLayerEntities(layerId),
}));

const mockIconCanvas = { _isCanvas: true } as unknown as HTMLCanvasElement;

vi.mock('./icons/iconRegistry', () => ({
  getIconCanvas: vi.fn(() => mockIconCanvas),
  supportsRotation: vi.fn(() => false),
  getIconScale: vi.fn(() => 1.0),
}));

vi.mock('./entityUtils', () => ({
  extractHeadingRadians: vi.fn((deg: number) => -(deg * Math.PI) / 180),
  isBackwardMotion: vi.fn(() => false),
  projectPosition: vi.fn((fromPos: unknown) => fromPos),
  KNOTS_TO_MPS: 0.514444,
  MAX_EXTRAPOLATE_SEC: 60,
}));

vi.mock('./interpolatedPositions', () => ({
  interpolatedPositions: new Map(),
}));

// ---------------------------------------------------------------------------
// Import subject under test — comes after all vi.mock() declarations
// ---------------------------------------------------------------------------

import {
  BillboardLayerRenderer,
  BillboardLayerRendererProps,
  resolveColorByHex,
} from './BillboardLayerRenderer';
import { getIconCanvas, supportsRotation, getIconScale } from './icons/iconRegistry';
import { extractHeadingRadians, isBackwardMotion, projectPosition } from './entityUtils';
import { useUIStore } from '@/app/store';

// ---------------------------------------------------------------------------
// Viewer factory helper
// ---------------------------------------------------------------------------

function createMockViewer() {
  const preUpdateListeners: Array<() => void> = [];
  const postRenderListeners: Array<() => void> = [];
  const cameraChangedListeners: Array<() => void> = [];

  const viewer = {
    isDestroyed: vi.fn(() => false),
    scene: {
      primitives: {
        add: vi.fn(),
        remove: vi.fn(),
      },
      requestRender: vi.fn(),
      preUpdate: {
        addEventListener: vi.fn((fn: () => void) => preUpdateListeners.push(fn)),
        removeEventListener: vi.fn((fn: () => void) => {
          const idx = preUpdateListeners.indexOf(fn);
          if (idx !== -1) preUpdateListeners.splice(idx, 1);
        }),
        _listeners: preUpdateListeners,
      },
      postRender: {
        addEventListener: vi.fn((fn: () => void) => postRenderListeners.push(fn)),
        removeEventListener: vi.fn((fn: () => void) => {
          const idx = postRenderListeners.indexOf(fn);
          if (idx !== -1) postRenderListeners.splice(idx, 1);
        }),
        _listeners: postRenderListeners,
      },
    },
    camera: {
      positionWC: new MockCartesian3(1000, 0, 0),
      changed: {
        addEventListener: vi.fn((fn: () => void) => cameraChangedListeners.push(fn)),
        removeEventListener: vi.fn((fn: () => void) => {
          const idx = cameraChangedListeners.indexOf(fn);
          if (idx !== -1) cameraChangedListeners.splice(idx, 1);
        }),
        _listeners: cameraChangedListeners,
      },
    },
    _firePreUpdate() {
      preUpdateListeners.forEach((fn) => fn());
    },
    _firePostRender() {
      postRenderListeners.forEach((fn) => fn());
    },
    _fireCameraChanged() {
      cameraChangedListeners.forEach((fn) => fn());
    },
  };
  return viewer;
}

type MockViewer = ReturnType<typeof createMockViewer>;

// ---------------------------------------------------------------------------
// Data format helpers
//
// useLayerEntities now returns the Map-based format:
//   { entityMap, obsMap, dirtyEntityIds, config, hasData, totalCount, version, maxEntities }
//
// - entityMap: Map<string, Entity> keyed by entity.id
// - obsMap:    Map<string, Observation> keyed by obs.entityId
// - dirtyEntityIds: Set<string> of entity IDs changed since last consumer read
// - version:   monotonically increasing integer (incremented per store update)
// - maxEntities: upper bound on rendered entities (default 2000)
// ---------------------------------------------------------------------------

let _versionCounter = 0;

/**
 * Build the empty-data return value (no layer data in store).
 * hasData=false signals the component to bail out and flag needsFullRebuild.
 */
const makeEmptyData = () => ({
  entityMap: null as Map<string, TestEntity> | null,
  obsMap: null as Map<string, TestObservation> | null,
  dirtyEntityIds: null as Set<string> | null,
  config: {},
  hasData: false,
  totalCount: 0,
  version: ++_versionCounter,
  maxEntities: 2000,
});

/**
 * Build the populated-data return value.
 *
 * All entity IDs are placed in dirtyEntityIds (simulating the first-time
 * "all dirty" state produced by setLayerEntities).  Each subsequent call
 * to makeData creates a fresh Set so the component sees new dirty IDs.
 */
const makeData = (entities: TestEntity[], observations: TestObservation[]) => {
  const entityMap = new Map<string, TestEntity>();
  const obsMap = new Map<string, TestObservation>();
  const dirtyEntityIds = new Set<string>();

  for (const entity of entities) {
    if (entity && entity.id != null) {
      entityMap.set(entity.id, entity);
      dirtyEntityIds.add(entity.id);
    }
  }
  for (const obs of observations) {
    if (obs) obsMap.set(obs.entityId, obs);
  }

  return {
    entityMap,
    obsMap,
    dirtyEntityIds,
    config: {},
    hasData: true,
    totalCount: entityMap.size,
    version: ++_versionCounter,
    maxEntities: 2000,
  };
};

// ---------------------------------------------------------------------------
// Default prop helpers
// ---------------------------------------------------------------------------

const makeDefaultProps = (
  viewer: MockViewer,
  overrides: Partial<BillboardLayerRendererProps> = {},
): BillboardLayerRendererProps => ({
  layerId: 'layer-1',
  layerType: 'satellites',
  viewerRef: { current: viewer } as unknown as BillboardLayerRendererProps['viewerRef'],
  ...overrides,
});

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Flight layer display config seed
//
// supportsInterpolation() reads displayConfig.icon.interpolation from the
// Zustand store (declarative path only — no layer-name fallback). Tests that
// exercise the flight-layer path must seed the store before rendering.
// ---------------------------------------------------------------------------

/** Minimal Layer shape with interpolation enabled, keyed by layerType. */
function makeFlightLayerConfig(type: string) {
  return {
    id: type,
    name: type,
    type,
    enabled: true,
    mode: 'live',
    density: 1,
    source: type,
    lastUpdate: 0,
    count: 0,
    color: '#ffffff',
    pointSize: 8,
    displayConfig: {
      icon: { shape: 'flight', rotatable: true, interpolation: true, scale: 1.0 },
      trail: { color: '', width: 1.5, opacity: 0.7 },
      style: { color: '', pointSize: 8 },
      fieldRenderers: [],
    },
  };
}

describe('BillboardLayerRenderer', () => {
  let viewer: MockViewer;

  beforeEach(() => {
    vi.clearAllMocks();
    _versionCounter = 0;
    viewer = createMockViewer();
    // Default: no data
    mockUseLayerEntities.mockReturnValue(makeEmptyData());
    // Default icon registry behaviour
    vi.mocked(supportsRotation).mockReturnValue(false);
    vi.mocked(getIconScale).mockReturnValue(1.0);
    vi.mocked(getIconCanvas).mockReturnValue(mockIconCanvas);
    // Seed the store with flight layer displayConfig so supportsInterpolation()
    // returns true for the declarative flight-layer types used in tests.
    useUIStore
      .getState()
      .setLayers([
        makeFlightLayerConfig('flights_commercial'),
        makeFlightLayerConfig('flights_military'),
      ]);
  });

  afterEach(() => {
    vi.useRealTimers();
    // Clear seeded layers so store is pristine between test suites
    useUIStore.getState().setLayers([]);
  });

  // -------------------------------------------------------------------------
  // Effect 1: BillboardCollection lifecycle
  // -------------------------------------------------------------------------

  describe('Effect 1 — BillboardCollection lifecycle', () => {
    it('creates a BillboardCollection and adds it to scene.primitives on mount', () => {
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      expect(MockBillboardCollectionCtor).toHaveBeenCalledTimes(1);
      expect(viewer.scene.primitives.add).toHaveBeenCalledTimes(1);
      expect(viewer.scene.primitives.add).toHaveBeenCalledWith(getLastCollectionInstance());
    });

    it('removes and destroys the collection on unmount', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      const collection = getLastCollectionInstance()!;
      expect(collection).not.toBeNull();

      unmount();

      expect(viewer.scene.primitives.remove).toHaveBeenCalledWith(collection);
      expect(collection.destroy).toHaveBeenCalledTimes(1);
    });

    it('does not remove collection if viewer is destroyed at cleanup time', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      viewer.isDestroyed.mockReturnValue(true);

      unmount();

      expect(viewer.scene.primitives.remove).not.toHaveBeenCalled();
    });

    it('does not destroy collection if it is already destroyed at cleanup time', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      const collection = getLastCollectionInstance()!;
      collection._destroyed = true;

      unmount();

      expect(collection.destroy).not.toHaveBeenCalled();
    });

    it('skips collection creation when viewer is null', () => {
      const nullViewerRef = { current: null };
      render(
        <BillboardLayerRenderer
          layerId="layer-1"
          layerType="satellites"
          viewerRef={nullViewerRef as unknown as BillboardLayerRendererProps['viewerRef']}
        />,
      );
      expect(MockBillboardCollectionCtor).not.toHaveBeenCalled();
    });

    it('skips collection creation when viewer is already destroyed', () => {
      viewer.isDestroyed.mockReturnValue(true);
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      expect(MockBillboardCollectionCtor).not.toHaveBeenCalled();
    });

    it('renders null (headless component)', () => {
      const { container } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      expect(container.firstChild).toBeNull();
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2: Non-flight layer full rebuild
  // -------------------------------------------------------------------------

  describe('Effect 2 — Non-flight layer full rebuild', () => {
    const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites' };
    const satObs = {
      entityId: 'sat-1',
      position: { lon: '10.5', lat: '20.5' },
      altitudeM: '500',
    };

    it('does NOT call removeAll on data update for non-flight layers (O(delta) reuse)', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      // Non-flight layers reuse billboards incrementally — never bulk-destroy
      expect(collection.removeAll).not.toHaveBeenCalled();
      expect(collection.add).toHaveBeenCalledTimes(1);
    });

    it('calls collection.add with correct position from observation', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);
      expect(MockCartesian3.fromDegrees).toHaveBeenCalledWith(10.5, 20.5, 500);
    });

    it('passes image canvas to collection.add', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'satellites' })} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      expect(addCall.image).toBe(mockIconCanvas);
    });

    it('passes color to getIconCanvas and uses WHITE for billboard', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { color: '#00ff00' })} />);

      const collection = getLastCollectionInstance()!;
      // Color is baked into the canvas, billboard uses WHITE (alpha-only)
      expect(vi.mocked(getIconCanvas)).toHaveBeenCalledWith('satellites', '#00ff00');
      const addCall = collection.add.mock.calls[0][0];
      expect(addCall.color).toMatchObject({ r: 1, g: 1, b: 1 });
    });

    it('color_by layer: draws the icon canvas WHITE and applies the per-entity color', () => {
      // Regression: a color_by layer resolves a per-entity color applied via
      // billboard.color, which Cesium MULTIPLIES with the icon texture. If the
      // canvas were baked in the layer base color, the icon would render as
      // base×entity — darkening e.g. a coal plant (#4d4d4d) to near-black and
      // making it effectively invisible on the dark globe. The canvas must be
      // WHITE so the color_by color renders exactly.
      useUIStore.getState().setLayers([
        {
          id: 'power_plants',
          name: 'power_plants',
          type: 'power_plants',
          enabled: true,
          mode: 'live',
          density: 1,
          source: 'power_plants',
          lastUpdate: 0,
          count: 0,
          color: '#ffb300',
          pointSize: 8,
          displayConfig: {
            icon: { shape: 'powerplant', rotatable: false, interpolation: false, scale: 1.0 },
            trail: { color: '', width: 1, opacity: 0.5 },
            style: { color: '#ffb300', pointSize: 8 },
            fieldRenderers: [],
            colorBy: {
              field: 'primary_fuel',
              values: { Coal: '#4d4d4d' },
              defaultColor: '#888888',
            },
          },
        },
      ]);
      const ent = { id: 'pp-1', name: 'Coal Plant', layerType: 'power_plants' };
      const obs = {
        entityId: 'pp-1',
        position: { lon: '5', lat: '5' },
        altitudeM: '0',
        metadata: { primary_fuel: 'Coal' },
      };
      mockUseLayerEntities.mockReturnValue(makeData([ent], [obs]));
      render(
        <BillboardLayerRenderer
          {...makeDefaultProps(viewer, { layerType: 'power_plants', color: '#ffb300' })}
        />,
      );

      // Canvas is drawn WHITE (not the layer base color) for color_by layers.
      expect(vi.mocked(getIconCanvas)).toHaveBeenCalledWith('power_plants', '#ffffff');
      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      // Billboard carries the exact color_by color (mock records it as _css).
      expect((addCall.color as unknown as { _css?: string })._css).toBe('#4d4d4d');
    });

    it('falls back to WHITE when color prop is invalid CSS', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { color: 'invalid' })} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      // When Color.fromCssColorString throws, cesiumColor stays as Color.WHITE.
      // The component then calls Color.clone(Color.WHITE) to produce entityColor,
      // so the color values match WHITE but is a distinct clone instance.
      expect(addCall.color).toMatchObject({ r: 1, g: 1, b: 1 });
    });

    it('passes scale from getIconScale to collection.add', () => {
      vi.mocked(getIconScale).mockReturnValue(1.5);
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      expect(addCall.scale).toBe(1.5);
    });

    it('skips entity when no matching observation is found', () => {
      mockUseLayerEntities.mockReturnValue(
        makeData([satEntity], [{ entityId: 'sat-99', position: { lon: '0', lat: '0' } }]),
      );
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).not.toHaveBeenCalled();
    });

    it('skips entity with no matching observation (Map-based: entity in map, obs missing)', () => {
      // entityMap has sat-1 but obsMap has a different key — sat-1 has no obs
      const data = makeData([satEntity], []);
      mockUseLayerEntities.mockReturnValue(data);
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).not.toHaveBeenCalled();
    });

    it('skips entity missing id field', () => {
      const badEntity = { name: 'NoId', layerType: 'satellites' };
      // makeData will skip entities with no id, so the Map will be empty
      mockUseLayerEntities.mockReturnValue(makeData([badEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).not.toHaveBeenCalled();
    });

    it('does not set rotation when supportsRotation returns false', () => {
      vi.mocked(supportsRotation).mockReturnValue(false);
      const obsWithHeading = { ...satObs, velocity: { heading: '90' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [obsWithHeading]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      expect(addCall.rotation).toBe(0);
    });

    it('sets rotation via extractHeadingRadians when supportsRotation is true', () => {
      vi.mocked(supportsRotation).mockReturnValue(true);
      vi.mocked(extractHeadingRadians).mockReturnValue(-1.5708);
      const entityWithHeading = { id: 'fl-1', name: 'Flight', layerType: 'satellites' };
      const obsWithHeading = {
        entityId: 'fl-1',
        position: { lon: '10', lat: '20' },
        velocity: { heading: '90' },
      };
      mockUseLayerEntities.mockReturnValue(makeData([entityWithHeading], [obsWithHeading]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'satellites' })} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      expect(extractHeadingRadians).toHaveBeenCalledWith(90);
      expect(addCall.rotation).toBe(-1.5708);
    });

    it('never calls removeAll across re-renders with new data (incremental add)', () => {
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      const props = makeDefaultProps(viewer);
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.removeAll).not.toHaveBeenCalled();
      expect(collection.add).toHaveBeenCalledTimes(1);

      // Incremental update: a new entity becomes dirty; sat-1 stays (not dirty,
      // not departed via full rebuild) so the incremental path just adds sat-2.
      const newEntity = { id: 'sat-2', name: 'ISS', layerType: 'satellites' };
      const newObs = { entityId: 'sat-2', position: { lon: '5', lat: '10' } };
      mockUseLayerEntities.mockReturnValue(makeData([newEntity], [newObs]));
      rerender(<BillboardLayerRenderer {...props} />);

      // Still no bulk destroy — sat-2 added incrementally
      expect(collection.removeAll).not.toHaveBeenCalled();
      expect(collection.add).toHaveBeenCalledTimes(2);
    });

    it('does not add billboards when hasData is false', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).not.toHaveBeenCalled();
    });

    it('handles NaN longitude gracefully (skips observation)', () => {
      const badObs = { entityId: 'sat-1', position: { lon: 'not-a-number', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [badObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2: Data normalization (camelCase vs PascalCase)
  // -------------------------------------------------------------------------

  describe('Effect 2 — data normalization', () => {
    // Note: Multi-casing normalization (PascalCase, snake_case) is handled at
    // the WebSocket ingestion boundary in useLayerStream.ts. By the time data
    // reaches BillboardLayerRenderer, all fields are canonical camelCase.

    it('reads altitude from altitudeM field', () => {
      const entity = { id: 'e-1', name: 'Test', layerType: 'satellites' };
      const obs = { entityId: 'e-1', position: { lon: '1', lat: '2' }, altitudeM: 10000 };
      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      expect(MockCartesian3.fromDegrees).toHaveBeenCalledWith(1, 2, 10000);
    });

    it('reads heading from velocity.heading', () => {
      vi.mocked(supportsRotation).mockReturnValue(true);
      const entity = { id: 'fl-1', name: 'Flight', layerType: 'satellites' };
      const obs = {
        entityId: 'fl-1',
        position: { lon: '1', lat: '2' },
        velocity: { heading: 180 },
      };
      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'satellites' })} />);

      expect(extractHeadingRadians).toHaveBeenCalledWith(180);
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2: Flight layer animated diff path
  //
  // For flight layers the component uses O(delta) incremental updates on
  // subsequent renders.  The FIRST render always goes through the full rebuild
  // path (needsFullRebuildRef starts true).
  //
  // To exercise the full rebuild path on a re-render (for depart/remove tests)
  // we insert a hasData=false render between data renders, which resets
  // needsFullRebuildRef to true.
  // -------------------------------------------------------------------------

  describe('Effect 2 — flight layer animated diff path', () => {
    const flightEntity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
    const flightObs = {
      entityId: 'fl-1',
      position: { lon: '10', lat: '20' },
      velocity: { heading: '90' },
    };

    const makeFlightProps = () => makeDefaultProps(viewer, { layerType: 'flights_commercial' });

    it('does NOT call removeAll for flight layers', () => {
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.removeAll).not.toHaveBeenCalled();
    });

    it('calls collection.add for a new flight entity', () => {
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);
    });

    it('image canvas is set for flight layer', () => {
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      expect(addCall.image).toBe(mockIconCanvas);
    });

    it('updates existing billboard color/scale on second render instead of re-adding', () => {
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // Second render with same entity at updated position (incremental path)
      const updatedObs = { ...flightObs, position: { lon: '11', lat: '21' } };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [updatedObs]));
      rerender(<BillboardLayerRenderer {...props} />);

      // Still just 1 add call; the existing billboard was mutated in-place
      expect(collection.add).toHaveBeenCalledTimes(1);
      expect(collection.removeAll).not.toHaveBeenCalled();
    });

    it('uses WHITE for billboard color even when color prop changes', () => {
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      const existingBb = collection._billboards[0];

      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      rerender(<BillboardLayerRenderer {...{ ...props, color: '#ff0000' }} />);

      // Billboard color is always WHITE — color is baked into the canvas
      expect(existingBb.color).toMatchObject({ r: 1, g: 1, b: 1 });
    });

    it('removes billboard for departed entity via full rebuild after data reset', () => {
      // Render with the entity present (full rebuild path — needsFullRebuildRef starts true)
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection._billboards).toHaveLength(1);

      // Simulate clearLayerEntities: hasData=false resets needsFullRebuildRef to true
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      // Re-render with empty entity set — triggers full rebuild, departed entity is removed
      mockUseLayerEntities.mockReturnValue(makeData([], []));
      rerender(<BillboardLayerRenderer {...props} />);

      expect(collection.remove).toHaveBeenCalledTimes(1);
      expect(collection._billboards).toHaveLength(0);
    });

    it('adds new entity and removes departed entity in same full rebuild update', () => {
      const entity2 = { id: 'fl-2', name: 'UA200', layerType: 'flights_commercial' };
      const obs2 = { entityId: 'fl-2', position: { lon: '5', lat: '5' } };

      // First render: add fl-1 (full rebuild)
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // Reset needsFullRebuildRef via hasData=false
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      // Full rebuild with fl-2 only — fl-1 should be removed, fl-2 added
      mockUseLayerEntities.mockReturnValue(makeData([entity2], [obs2]));
      rerender(<BillboardLayerRenderer {...props} />);

      expect(collection.add).toHaveBeenCalledTimes(2); // fl-1 (render 1) + fl-2 (render 3)
      expect(collection.remove).toHaveBeenCalledTimes(1); // fl-1 removed
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2b: Interpolation loop
  // -------------------------------------------------------------------------

  describe('Effect 2b — interpolation loop', () => {
    it('registers scene.preUpdate listener for flight layers', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      render(
        <BillboardLayerRenderer
          {...makeDefaultProps(viewer, { layerType: 'flights_commercial' })}
        />,
      );
      expect(viewer.scene.preUpdate.addEventListener).toHaveBeenCalledTimes(1);
    });

    it('does NOT register scene.preUpdate listener for non-flight layers', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'satellites' })} />);
      expect(viewer.scene.preUpdate.addEventListener).not.toHaveBeenCalled();
    });

    it('removes scene.preUpdate listener on unmount for flight layers', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      const { unmount } = render(
        <BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'flights_military' })} />,
      );
      unmount();
      expect(viewer.scene.preUpdate.removeEventListener).toHaveBeenCalledTimes(1);
    });

    it('does not remove preUpdate listener if viewer is destroyed at cleanup', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      const { unmount } = render(
        <BillboardLayerRenderer
          {...makeDefaultProps(viewer, { layerType: 'flights_commercial' })}
        />,
      );
      viewer.isDestroyed.mockReturnValue(true);
      unmount();
      expect(viewer.scene.preUpdate.removeEventListener).not.toHaveBeenCalled();
    });

    it('interpolation loop fires Cartesian3.lerp during active animation', () => {
      vi.useFakeTimers();

      const flightEntity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs1 = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));

      const props = makeDefaultProps(viewer, { layerType: 'flights_commercial' });
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // Second position triggers animation state update for existing entity
      const obs2 = { entityId: 'fl-1', position: { lon: '11', lat: '21' } };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs2]));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance time partway through the 2000ms animation window
      vi.advanceTimersByTime(1000);

      // Fire preUpdate — should interpolate
      act(() => {
        viewer._firePreUpdate();
      });

      expect(MockCartesian3.lerp).toHaveBeenCalled();
    });

    it('interpolation skips converged entities (past animation duration)', () => {
      vi.useFakeTimers();

      const flightEntity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs1 = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));

      const props = makeDefaultProps(viewer, { layerType: 'flights_commercial' });
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const obs2 = { entityId: 'fl-1', position: { lon: '11', lat: '21' } };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs2]));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past the 2000ms animation window
      vi.advanceTimersByTime(3000);

      act(() => {
        viewer._firePreUpdate();
      });

      // lerp should NOT have been called because animation is converged
      expect(MockCartesian3.lerp).not.toHaveBeenCalled();
    });

    it('does NOT interpolate a clustered flight member (no drift, no render forcing)', () => {
      vi.useFakeTimers();
      const flightEntity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      mockUseLayerEntities.mockReturnValue(
        makeData([flightEntity], [{ entityId: 'fl-1', position: { lon: '10', lat: '20' } }]),
      );
      const props = makeDefaultProps(viewer, { layerType: 'flights_commercial' });
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // Mark fl-1 as clustered BEFORE the position update that would animate it.
      act(() => {
        useUIStore.getState().setClusteredMembers('layer-1', new Set(['fl-1']));
      });

      mockUseLayerEntities.mockReturnValue(
        makeData([flightEntity], [{ entityId: 'fl-1', position: { lon: '11', lat: '21' } }]),
      );
      rerender(<BillboardLayerRenderer {...props} />);
      vi.advanceTimersByTime(1000);

      MockCartesian3.lerp.mockClear();
      act(() => {
        viewer._firePreUpdate();
      });

      // Clustered member is skipped → no interpolation.
      expect(MockCartesian3.lerp).not.toHaveBeenCalled();

      // Clearing membership lets it animate again.
      act(() => {
        useUIStore.getState().setClusteredMembers('layer-1', new Set());
      });
      mockUseLayerEntities.mockReturnValue(
        makeData([flightEntity], [{ entityId: 'fl-1', position: { lon: '12', lat: '22' } }]),
      );
      rerender(<BillboardLayerRenderer {...props} />);
      vi.advanceTimersByTime(500);
      act(() => {
        viewer._firePreUpdate();
      });
      expect(MockCartesian3.lerp).toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Effect 4: Cluster member suppression (single owner of `show`)
  // -------------------------------------------------------------------------

  describe('Effect 4 — cluster member suppression', () => {
    const ids = (bb: { id: unknown }) => (bb.id as { entityId: string }).entityId;

    function renderTwo() {
      mockUseLayerEntities.mockReturnValue(
        makeData(
          [
            { id: 'a', name: 'A', layerType: 'satellites' },
            { id: 'b', name: 'B', layerType: 'satellites' },
          ],
          [
            { entityId: 'a', position: { lon: '10', lat: '20' } },
            { entityId: 'b', position: { lon: '30', lat: '40' } },
          ],
        ),
      );
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      return getLastCollectionInstance()!;
    }

    it('hides a clustered member and restores it when membership clears', () => {
      const collection = renderTwo();
      expect(collection._billboards.every((bb) => bb.show)).toBe(true);

      act(() => {
        useUIStore.getState().setClusteredMembers('layer-1', new Set(['a']));
      });
      const a = collection._billboards.find((bb) => ids(bb) === 'a')!;
      const b = collection._billboards.find((bb) => ids(bb) === 'b')!;
      expect(a.show).toBe(false);
      expect(b.show).toBe(true);

      act(() => {
        useUIStore.getState().setClusteredMembers('layer-1', new Set());
      });
      expect(a.show).toBe(true);
      expect(b.show).toBe(true);
    });

    it('isolation wins over clustering (isolated entity stays visible)', () => {
      const collection = renderTwo();
      act(() => {
        // Clustering both, but isolate 'a' — 'a' must remain shown.
        useUIStore.getState().setClusteredMembers('layer-1', new Set(['a', 'b']));
        useUIStore.getState().setSelectedEntity('a', 'layer-1');
        useUIStore.getState().updateEntityViewState('a', { isolateEntity: true });
      });
      const a = collection._billboards.find((bb) => ids(bb) === 'a')!;
      const b = collection._billboards.find((bb) => ids(bb) === 'b')!;
      expect(a.show).toBe(true);
      expect(b.show).toBe(false);
      act(() => {
        useUIStore.getState().clearSelection();
        useUIStore.getState().setClusteredMembers('layer-1', new Set());
      });
    });
  });

  // -------------------------------------------------------------------------
  // Effect 3: Occlusion listeners
  // -------------------------------------------------------------------------

  describe('Effect 3 — occlusion effect listeners', () => {
    it('registers camera.changed listener on mount', () => {
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      expect(viewer.camera.changed.addEventListener).toHaveBeenCalledTimes(1);
    });

    it('registers scene.postRender listener on mount', () => {
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      expect(viewer.scene.postRender.addEventListener).toHaveBeenCalledTimes(1);
    });

    it('removes camera.changed listener on unmount', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      unmount();
      expect(viewer.camera.changed.removeEventListener).toHaveBeenCalledTimes(1);
    });

    it('removes scene.postRender listener on unmount', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      unmount();
      expect(viewer.scene.postRender.removeEventListener).toHaveBeenCalledTimes(1);
    });

    it('does not remove occlusion listeners if viewer is destroyed at cleanup', () => {
      const { unmount } = render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);
      viewer.isDestroyed.mockReturnValue(true);
      unmount();
      expect(viewer.camera.changed.removeEventListener).not.toHaveBeenCalled();
      expect(viewer.scene.postRender.removeEventListener).not.toHaveBeenCalled();
    });

    it('updates billboard alpha to VISIBLE when camera changes and billboard is visible', () => {
      const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites' };
      const satObs = { entityId: 'sat-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // Set alpha to something different so the update triggers. MockBillboard.color
      // is fully typed so all four channels must be supplied — only alpha matters here.
      bb.color = { r: 1, g: 1, b: 1, alpha: 0.1 };
      getLastOccluderInstance()!.isPointVisible.mockReturnValue(true);

      act(() => {
        // onCameraChanged now only marks dirty; postRender performs the O(N) scan
        viewer._fireCameraChanged();
        viewer._firePostRender();
      });

      // alpha should now reflect VISIBLE_ALPHA (1.0)
      expect(bb.color.alpha).toBe(1.0);
    });

    it('updates billboard alpha to occluded value when billboard is behind globe', () => {
      const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites' };
      const satObs = { entityId: 'sat-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      bb.color = { r: 1, g: 1, b: 1, alpha: 1.0 };
      getLastOccluderInstance()!.isPointVisible.mockReturnValue(false);

      act(() => {
        // onCameraChanged now only marks dirty; postRender performs the O(N) scan
        viewer._fireCameraChanged();
        viewer._firePostRender();
      });

      expect(bb.color.alpha).toBe(0.12);
    });

    it('fires occlusion update via postRender dirty flag', () => {
      const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites' };
      const satObs = { entityId: 'sat-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];
      bb.color = { r: 1, g: 1, b: 1, alpha: 0.0 };
      getLastOccluderInstance()!.isPointVisible.mockReturnValue(true);

      act(() => {
        viewer._firePostRender();
      });

      expect(bb.color.alpha).toBe(1.0);
    });

    it('does not run occlusion twice on postRender (dirty flag cleared after first fire)', () => {
      const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites' };
      const satObs = { entityId: 'sat-1', position: { lon: '10', lat: '20' } };
      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      getLastOccluderInstance()!.isPointVisible.mockReturnValue(true);

      act(() => {
        viewer._firePostRender(); // first fire: processes dirty flag, clears it
        viewer._firePostRender(); // second fire: dirty flag is false, should skip
      });

      // isPointVisible called only once (on first postRender)
      expect(getLastOccluderInstance()!.isPointVisible).toHaveBeenCalledTimes(1);
    });

    it('skips occlusion update when collection is empty', () => {
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer)} />);

      act(() => {
        viewer._fireCameraChanged();
      });

      // No billboards — isPointVisible should not be called
      expect(getLastOccluderInstance()?.isPointVisible).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2 — O(delta) incremental update path
  //
  // These tests verify the optimized incremental rendering path where only
  // dirty entities (those changed since the last consumer read) are processed.
  // -------------------------------------------------------------------------

  describe('Effect 2 — O(delta) incremental updates', () => {
    const makeFlightProps = () => makeDefaultProps(viewer, { layerType: 'flights_commercial' });

    it('only processes dirty entities on incremental update', () => {
      // Initial full render with 4 entities
      const entities = [
        { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' },
        { id: 'fl-2', name: 'UA200', layerType: 'flights_commercial' },
        { id: 'fl-3', name: 'DL300', layerType: 'flights_commercial' },
        { id: 'fl-4', name: 'SW400', layerType: 'flights_commercial' },
      ];
      const observations = entities.map((e) => ({
        entityId: e.id,
        position: { lon: '10', lat: '20' },
      }));

      mockUseLayerEntities.mockReturnValue(makeData(entities, observations));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(4);
      collection.add.mockClear();

      // Incremental update: only fl-1 and fl-2 are dirty
      const entityMap = new Map<string, TestEntity>(
        entities.map((e) => [e.id!, e] as [string, TestEntity]),
      );
      const obsMap = new Map<string, TestObservation>(
        observations.map((o) => [o.entityId, o] as [string, TestObservation]),
      );
      const dirtyEntityIds = new Set(['fl-1', 'fl-2']);

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: entityMap.size,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      // No new adds — fl-1 and fl-2 exist already and were updated in-place
      expect(collection.add).not.toHaveBeenCalled();
      // No removes — incremental path never removes departed entities
      expect(collection.remove).not.toHaveBeenCalled();
    });

    it('adds only newly discovered entities from dirtyEntityIds', () => {
      // Initial render with fl-1
      const entity1 = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs1 = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      mockUseLayerEntities.mockReturnValue(makeData([entity1], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // Incremental update: fl-2 is new (dirty), fl-1 also dirty but already has a billboard
      const entity2 = { id: 'fl-2', name: 'UA200', layerType: 'flights_commercial' };
      const obs2 = { entityId: 'fl-2', position: { lon: '5', lat: '10' } };

      const entityMap = new Map<string, TestEntity>([
        ['fl-1', entity1],
        ['fl-2', entity2],
      ] as [string, TestEntity][]);
      const obsMap = new Map<string, TestObservation>([
        ['fl-1', obs1],
        ['fl-2', obs2],
      ] as [string, TestObservation][]);
      // Only fl-2 is newly dirty
      const dirtyEntityIds = new Set(['fl-2']);

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: entityMap.size,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      // Only fl-2 should have triggered a new add call
      expect(collection.add).toHaveBeenCalledTimes(2);
    });

    it('dirtyEntityIds.clear() is called after processing incremental update', () => {
      const entity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      // Initial render (full rebuild)
      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // Incremental update with a trackable dirtyEntityIds Set
      const dirtyEntityIds = new Set(['fl-1']);
      const clearSpy = vi.spyOn(dirtyEntityIds, 'clear');

      const entityMap = new Map<string, TestEntity>([['fl-1', entity]] as [string, TestEntity][]);
      const obsMap = new Map<string, TestObservation>([['fl-1', obs]] as [
        string,
        TestObservation,
      ][]);

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      expect(clearSpy).toHaveBeenCalledTimes(1);
    });

    it('skips animation (no Cartesian3.lerp) when position delta is below POS_EPSILON', () => {
      vi.useFakeTimers();

      const entity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      // Use numeric positions to avoid string-to-number parsing variation
      const obs1 = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // Position that is within POS_EPSILON (1e-6) of the existing billboard position.
      // MockCartesian3.fromDegrees(lon, lat, alt) creates {x: lon, y: lat, z: alt}.
      // The billboard position after first render is {x: 10.0, y: 20.0, z: 0}.
      // A sub-epsilon nudge: delta = 1e-8 (well below 1e-6 threshold).
      const nudge = 1e-8;
      const obs2 = { entityId: 'fl-1', position: { lon: 10.0 + nudge, lat: 20.0 } };

      const entityMap = new Map<string, TestEntity>([['fl-1', entity]] as [string, TestEntity][]);
      const obsMap = new Map<string, TestObservation>([['fl-1', obs2]] as [
        string,
        TestObservation,
      ][]);
      const dirtyEntityIds = new Set(['fl-1']);

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      // Advance time to mid-animation window
      vi.advanceTimersByTime(1000);
      act(() => {
        viewer._firePreUpdate();
      });

      // No animation state should have been set — lerp must not be called
      expect(MockCartesian3.lerp).not.toHaveBeenCalled();
    });

    it('creates animation state (Cartesian3.lerp called) when position delta exceeds POS_EPSILON', () => {
      vi.useFakeTimers();

      const entity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs1 = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // Position change well above POS_EPSILON (1e-6): lon shifts by 1.0
      const obs2 = { entityId: 'fl-1', position: { lon: 11.0, lat: 20.0 } };

      const entityMap = new Map<string, TestEntity>([['fl-1', entity]] as [string, TestEntity][]);
      const obsMap = new Map<string, TestObservation>([['fl-1', obs2]] as [
        string,
        TestObservation,
      ][]);
      const dirtyEntityIds = new Set(['fl-1']);

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      // Advance time to mid-animation window
      vi.advanceTimersByTime(1000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Animation state was set — lerp must be called
      expect(MockCartesian3.lerp).toHaveBeenCalled();
    });

    it('full rebuild path runs when needsFullRebuildRef is true (after data reset)', () => {
      const entity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      // First render: full rebuild (needsFullRebuildRef starts true)
      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // hasData=false triggers needsFullRebuildRef = true
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      collection.add.mockClear();

      // Next render must go through full rebuild (not incremental)
      const entity2 = { id: 'fl-2', name: 'UA200', layerType: 'flights_commercial' };
      const obs2 = { entityId: 'fl-2', position: { lon: '5', lat: '10' } };
      mockUseLayerEntities.mockReturnValue(makeData([entity2], [obs2]));
      rerender(<BillboardLayerRenderer {...props} />);

      // fl-2 added fresh via full rebuild (fl-1 had no billboard in collection
      // at this point so remove count is 0; the old bb was from before the clear)
      expect(collection.add).toHaveBeenCalledTimes(1);
    });

    it('does not process entities when dirtyEntityIds is empty and needsFullRebuild is false', () => {
      const entity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };
      const obs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      // First render fills everything
      mockUseLayerEntities.mockReturnValue(makeData([entity], [obs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);
      collection.add.mockClear();

      // Second render: dirtyEntityIds is empty — nothing should change
      const entityMap = new Map<string, TestEntity>([['fl-1', entity]] as [string, TestEntity][]);
      const obsMap = new Map<string, TestObservation>([['fl-1', obs]] as [
        string,
        TestObservation,
      ][]);
      const dirtyEntityIds = new Set<string>(); // empty — no dirty entities

      mockUseLayerEntities.mockReturnValue({
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      expect(collection.add).not.toHaveBeenCalled();
      expect(collection.remove).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Backward motion catchup pause
  //
  // When dead-reckoning over-predicts (entity extrapolated past the real
  // position) and a new server update arrives behind the current billboard,
  // the component must:
  //   1. Snap to the server position (no backward lerp).
  //   2. Pause dead-reckoning for CATCHUP_PAUSE_MS so the backend can catch up.
  //   3. Resume dead-reckoning after the pause expires.
  //
  // The isBackwardMotion function (mocked from ./entityUtils) determines
  // whether a new position is behind the extrapolated billboard.
  // -------------------------------------------------------------------------

  describe('backward motion catchup pause', () => {
    const makeFlightProps = () => makeDefaultProps(viewer, { layerType: 'flights_commercial' });

    const flightEntity = { id: 'fl-1', name: 'AA100', layerType: 'flights_commercial' };

    // Helper: build an incremental-update payload with a single dirty entity
    const makeIncrementalData = (entity: TestEntity, obs: TestObservation) => {
      const entityMap = new Map<string, TestEntity>([[entity.id!, entity]] as [
        string,
        TestEntity,
      ][]);
      const obsMap = new Map<string, TestObservation>([[obs.entityId, obs]] as [
        string,
        TestObservation,
      ][]);
      const dirtyEntityIds = new Set([entity.id]);
      return {
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      };
    };

    it('snaps billboard to server position when backward motion is detected (no lerp)', () => {
      vi.useFakeTimers();

      // Initial render — places the billboard at lon=10, lat=20
      const obs1 = {
        entityId: 'fl-1',
        position: { lon: 10.0, lat: 20.0 },
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(1);

      // First position update — starts lerp animation
      const obs2 = {
        entityId: 'fl-1',
        position: { lon: 11.0, lat: 20.0 },
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past INTERP_DURATION_MS (2000ms) so dead-reckoning phase is active
      vi.advanceTimersByTime(3000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Now a server update arrives behind the extrapolated billboard.
      // Configure isBackwardMotion to return true for this scenario.
      vi.mocked(isBackwardMotion).mockReturnValueOnce(true);

      const obs3 = {
        entityId: 'fl-1',
        position: { lon: 10.5, lat: 20.0 }, // behind the extrapolated position
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs3));
      rerender(<BillboardLayerRenderer {...props} />);

      const bb = collection._billboards[0];
      // The billboard must snap directly to the server position — no lerp
      // MockCartesian3.fromDegrees(10.5, 20.0, 0) produces {x:10.5, y:20, z:0}
      expect(bb.position).toMatchObject({ x: 10.5, y: 20.0, z: 0 });

      // After the backward snap, for live entities the animation uses
      // duration=INTERP_DURATION_MS so the alpha pulse can fade. Position
      // lerp still occurs but is a no-op (fromPos===toPos). Verify position
      // remains at the snapped server position regardless.
      vi.advanceTimersByTime(500);
      act(() => {
        viewer._firePreUpdate();
      });
      // Billboard held at snapped server position — no backward animation
      expect(bb.position).toMatchObject({ x: 10.5, y: 20.0, z: 0 });
    });

    it('billboard remains stationary during the catchup pause (no extrapolation)', () => {
      vi.useFakeTimers();

      // Render initial billboard
      const obs1 = {
        entityId: 'fl-1',
        position: { lon: 10.0, lat: 20.0 },
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;

      // First update to establish lerp animation
      const obs2 = {
        entityId: 'fl-1',
        position: { lon: 10.1, lat: 20.0 },
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past lerp phase into dead-reckoning
      vi.advanceTimersByTime(3000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Trigger backward motion detection — component snaps and starts pause
      vi.mocked(isBackwardMotion).mockReturnValueOnce(true);
      const obs3 = {
        entityId: 'fl-1',
        position: { lon: 9.9, lat: 20.0 }, // behind current position
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs3));
      rerender(<BillboardLayerRenderer {...props} />);

      const bb = collection._billboards[0];

      // Record the position immediately after snap
      const posAfterSnap = { x: bb.position.x, y: bb.position.y, z: bb.position.z };

      // Advance halfway through the 5000ms catchup pause
      vi.advanceTimersByTime(2500);
      vi.mocked(projectPosition).mockClear();
      act(() => {
        viewer._firePreUpdate();
      });

      // Billboard must not have moved during the pause
      expect(bb.position.x).toBe(posAfterSnap.x);
      expect(bb.position.y).toBe(posAfterSnap.y);
      expect(bb.position.z).toBe(posAfterSnap.z);

      // Confirm that projectPosition was not called during the pause
      expect(vi.mocked(projectPosition)).not.toHaveBeenCalled();
    });

    it('dead-reckoning resumes after the catchup pause expires', () => {
      vi.useFakeTimers();

      // Render initial billboard
      const obs1 = {
        entityId: 'fl-1',
        position: { lon: 10.0, lat: 20.0 },
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // First update: establish lerp animation with speed data
      const obs2 = {
        entityId: 'fl-1',
        position: { lon: 10.1, lat: 20.0 },
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past lerp into dead-reckoning
      vi.advanceTimersByTime(3000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Trigger backward motion detection
      vi.mocked(isBackwardMotion).mockReturnValueOnce(true);
      const obs3 = {
        entityId: 'fl-1',
        position: { lon: 9.9, lat: 20.0 },
        velocity: { heading: 0, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs3));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past CATCHUP_PAUSE_MS (5000ms) — pause should be over
      vi.advanceTimersByTime(6000);

      vi.mocked(projectPosition).mockClear();

      act(() => {
        viewer._firePreUpdate();
      });

      // After the pause expires, projectPosition should be called to resume
      // dead-reckoning extrapolation
      expect(vi.mocked(projectPosition)).toHaveBeenCalled();
    });

    it('normal update with isBackwardMotion returning false still uses lerp animation', () => {
      vi.useFakeTimers();

      // Ensure isBackwardMotion returns false (the default mock value)
      vi.mocked(isBackwardMotion).mockReturnValue(false);

      const obs1 = {
        entityId: 'fl-1',
        position: { lon: 10.0, lat: 20.0 },
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // First update to establish animation state
      const obs2 = {
        entityId: 'fl-1',
        position: { lon: 11.0, lat: 20.0 },
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance past lerp phase so the existing anim qualifies as "was extrapolating"
      vi.advanceTimersByTime(3000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Second position update — backward motion NOT detected
      const obs3 = {
        entityId: 'fl-1',
        position: { lon: 12.0, lat: 20.0 }, // forward of extrapolated position
        velocity: { heading: 90, ground_speed: 400 },
      };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs3));
      rerender(<BillboardLayerRenderer {...props} />);

      // Advance into the new lerp window
      vi.advanceTimersByTime(1000);
      MockCartesian3.lerp.mockClear();
      act(() => {
        viewer._firePreUpdate();
      });

      // Normal lerp animation must be active (no snap, no pause)
      expect(MockCartesian3.lerp).toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Pulse-on-update alpha system
  //
  // Flight layers use a two-level alpha scheme for live entities:
  //   LIVE_IDLE_ALPHA  (1.0) — resting brightness between position updates
  //   LIVE_PULSE_ALPHA (1.0) — flash brightness when a new position arrives
  //
  // Both constants are currently 1.0, so the pulse system is a no-op for
  // live entities (alpha never changes between idle and pulse states).
  //
  // All sources now render at full alpha (1.0) — stale/historical provenance
  // is indicated in the Entity Detail panel, not via billboard dimming.
  // -------------------------------------------------------------------------

  describe('Pulse-on-update alpha system', () => {
    const makeFlightProps = () => makeDefaultProps(viewer, { layerType: 'flights_commercial' });

    // Helper to build an incremental-update payload for a single entity
    const makeIncrementalData = (entity: TestEntity, obs: TestObservation) => {
      const entityMap = new Map<string, TestEntity>([[entity.id!, entity]] as [
        string,
        TestEntity,
      ][]);
      const obsMap = new Map<string, TestObservation>([[obs.entityId, obs]] as [
        string,
        TestObservation,
      ][]);
      const dirtyEntityIds = new Set([entity.id]);
      return {
        entityMap,
        obsMap,
        dirtyEntityIds,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
      };
    };

    // 1. Flight layer initial render uses LIVE_IDLE_ALPHA (1.0)
    it('flight layer initial render uses LIVE_IDLE_ALPHA (1.0) for live entities', () => {
      const flightEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'live',
      };
      const flightObs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // LIVE_IDLE_ALPHA is 1.0 — live entities on flight layers render at full brightness
      expect(bb.color.alpha).toBe(1.0);
    });

    // 2. Non-flight layer initial render still uses full alpha (1.0)
    it('non-flight layer initial render uses full alpha (1.0) for live satellite entities', () => {
      const satEntity = { id: 'sat-1', name: 'Hubble', layerType: 'satellites', source: 'live' };
      const satObs = { entityId: 'sat-1', position: { lon: '10', lat: '20' } };

      mockUseLayerEntities.mockReturnValue(makeData([satEntity], [satObs]));
      render(<BillboardLayerRenderer {...makeDefaultProps(viewer, { layerType: 'satellites' })} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // Non-flight layers use SOURCE_ALPHA.live = 1.0 with no idle dimming
      expect(bb.color.alpha).toBe(1.0);
    });

    // 3. Flight entity position update stays at LIVE_PULSE_ALPHA (1.0)
    it('flight entity position update sets billboard alpha to LIVE_PULSE_ALPHA (1.0)', () => {
      const flightEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'live',
      };
      const obs1 = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      // Initial render — billboard starts at LIVE_IDLE_ALPHA=1.0
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];
      expect(bb.color.alpha).toBe(1.0);

      // Position update — incremental path sets alpha to LIVE_PULSE_ALPHA=1.0
      const obs2 = { entityId: 'fl-1', position: { lon: 11.0, lat: 21.0 } };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Billboard color alpha must be set to LIVE_PULSE_ALPHA=1.0 on the first frame
      expect(bb.color.alpha).toBe(1.0);
    });

    // 4. Stale entities render at full alpha (1.0) — no dimming
    it('stale flight entity renders at full alpha (1.0)', () => {
      const staleEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'stale',
      };
      const staleObs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      mockUseLayerEntities.mockReturnValue(makeData([staleEntity], [staleObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // All sources render at full alpha — provenance shown in Entity Detail panel
      expect(bb.color.alpha).toBe(1.0);
    });

    // 5. Historical entities render at full alpha (1.0) — no dimming
    it('historical flight entity renders at full alpha (1.0)', () => {
      const historicalEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'historical',
      };
      const historicalObs = { entityId: 'fl-1', position: { lon: '10', lat: '20' } };

      mockUseLayerEntities.mockReturnValue(makeData([historicalEntity], [historicalObs]));
      render(<BillboardLayerRenderer {...makeFlightProps()} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // All sources render at full alpha — provenance shown in Entity Detail panel
      expect(bb.color.alpha).toBe(1.0);
    });

    // 6. Animation is a no-op when LIVE_PULSE_ALPHA == LIVE_IDLE_ALPHA (both 1.0)
    it('animation skips alpha lerp when LIVE_PULSE_ALPHA equals LIVE_IDLE_ALPHA (both 1.0)', () => {
      vi.useFakeTimers();

      const flightEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'live',
      };
      const obs1 = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [obs1]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // Position update: fromAlpha=1.0, toAlpha=1.0 — no lerp branch executed
      const obs2 = { entityId: 'fl-1', position: { lon: 11.0, lat: 21.0 } };
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, obs2));
      rerender(<BillboardLayerRenderer {...props} />);

      // Pulse is applied on render (LIVE_PULSE_ALPHA=1.0)
      expect(bb.color.alpha).toBe(1.0);

      // Advance to midpoint of the 2000ms animation window (t ≈ 0.5)
      vi.advanceTimersByTime(1000);
      act(() => {
        viewer._firePreUpdate();
      });

      // Since fromAlpha === toAlpha (both 1.0), the alpha lerp branch is skipped.
      // Alpha remains at 1.0 throughout the animation.
      expect(bb.color.alpha).toBe(1.0);
    });

    // 7. New flight entity starts at LIVE_IDLE_ALPHA (1.0)
    it('newly added flight entity starts at LIVE_IDLE_ALPHA (1.0), not below pulse', () => {
      const flightEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'live',
      };
      const flightObs = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      // Initial render with no entities
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      // New entity arrives in an incremental update
      mockUseLayerEntities.mockReturnValue(makeIncrementalData(flightEntity, flightObs));
      rerender(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      const bb = collection._billboards[0];

      // A brand-new entity has no prior position — it is added directly at LIVE_IDLE_ALPHA=1.0
      expect(bb.color.alpha).toBe(1.0);
    });

    // 8. entityIdleAlphaRef cleanup on entity departure and re-add
    it('re-added entity after removal still renders at LIVE_IDLE_ALPHA (1.0)', () => {
      const flightEntity = {
        id: 'fl-1',
        name: 'AA100',
        layerType: 'flights_commercial',
        source: 'live',
      };
      const flightObs = { entityId: 'fl-1', position: { lon: 10.0, lat: 20.0 } };

      // First render: entity added at LIVE_IDLE_ALPHA=1.0
      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      const props = makeFlightProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection._billboards).toHaveLength(1);
      expect(collection._billboards[0].color.alpha).toBe(1.0);

      // Simulate entity departure: hasData=false resets full rebuild flag
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      // Full rebuild with empty set removes the entity
      mockUseLayerEntities.mockReturnValue(makeData([], []));
      rerender(<BillboardLayerRenderer {...props} />);

      expect(collection.remove).toHaveBeenCalledTimes(1);
      expect(collection._billboards).toHaveLength(0);

      // Entity re-appears: another hasData=false to force full rebuild, then re-add
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      mockUseLayerEntities.mockReturnValue(makeData([flightEntity], [flightObs]));
      rerender(<BillboardLayerRenderer {...props} />);

      // Re-added entity should start at LIVE_IDLE_ALPHA=1.0 (entityIdleAlphaRef was cleaned up on removal)
      expect(collection._billboards).toHaveLength(1);
      expect(collection._billboards[0].color.alpha).toBe(1.0);
    });
  });

  // -------------------------------------------------------------------------
  // Eviction → needsRebuild → full-rebuild removes lingering billboards
  //
  // When the store LRU-evicts entities it sets needsRebuild=true on the
  // LayerData. The renderer must detect this flag and run the full-rebuild
  // path so departed billboards are actually removed from the collection.
  // Without the fix, evicted IDs are deleted from dirtyEntityIds so the
  // incremental path never touches them — billboards linger on the globe.
  // -------------------------------------------------------------------------

  describe('Effect 2 — eviction → needsRebuild → billboard removal', () => {
    const makeFlightProps = () => makeDefaultProps(viewer, { layerType: 'flights_commercial' });

    afterEach(() => {
      // Clean up store state between tests
      useUIStore.setState({ layerEntities: new Map(), layerVersions: {} });
    });

    it('eviction sets needsRebuild on LayerData — renderer full-rebuild removes the billboard (RED→GREEN)', () => {
      const layerId = 'layer-evict';
      const flightEntity = { id: 'fl-keep', name: 'Keep', layerType: 'flights_commercial' };
      const evictedEntity = { id: 'fl-evict', name: 'Evict', layerType: 'flights_commercial' };

      const flightObs = { entityId: 'fl-keep', position: { lon: '10', lat: '20' } };
      const evictedObs = { entityId: 'fl-evict', position: { lon: '5', lat: '5' } };

      const props = { ...makeFlightProps(), layerId };

      // Step 1: render both entities via the incremental path (full rebuild first)
      const entityMap1 = new Map([
        ['fl-keep', flightEntity],
        ['fl-evict', evictedEntity],
      ] as [string, typeof flightEntity][]);
      const obsMap1 = new Map([
        ['fl-keep', flightObs],
        ['fl-evict', evictedObs],
      ] as [string, typeof flightObs][]);
      const dirtyIds1 = new Set(['fl-keep', 'fl-evict']);

      // Seed the real store so needsRebuild starts false
      useUIStore.setState((s) => {
        const m = new Map(s.layerEntities);
        m.set(layerId, {
          entityMap: entityMap1 as unknown as Map<string, import('@/app/store').Entity>,
          obsMap: obsMap1 as unknown as Map<string, import('@/app/store').Observation>,
          dirtyEntityIds: dirtyIds1,
          needsRebuild: false,
        });
        return { layerEntities: m };
      });

      mockUseLayerEntities.mockReturnValue({
        entityMap: entityMap1,
        obsMap: obsMap1,
        dirtyEntityIds: dirtyIds1,
        config: {},
        hasData: true,
        totalCount: 2,
        version: ++_versionCounter,
        maxEntities: 2000,
        needsRebuild: false,
      });

      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection._billboards).toHaveLength(2);

      // Step 2: simulate eviction — only fl-keep survives, store has needsRebuild=true
      const entityMap2 = new Map([['fl-keep', flightEntity]] as [string, typeof flightEntity][]);
      const obsMap2 = new Map([['fl-keep', flightObs]] as [string, typeof flightObs][]);
      const dirtyIds2 = new Set(['fl-keep']); // evicted ID removed from dirty set

      useUIStore.setState((s) => {
        const m = new Map(s.layerEntities);
        m.set(layerId, {
          entityMap: entityMap2 as unknown as Map<string, import('@/app/store').Entity>,
          obsMap: obsMap2 as unknown as Map<string, import('@/app/store').Observation>,
          dirtyEntityIds: dirtyIds2,
          needsRebuild: true, // set by eviction fix in createEntitySlice
        });
        return { layerEntities: m };
      });

      mockUseLayerEntities.mockReturnValue({
        entityMap: entityMap2,
        obsMap: obsMap2,
        dirtyEntityIds: dirtyIds2,
        config: {},
        hasData: true,
        totalCount: 1,
        version: ++_versionCounter,
        maxEntities: 2000,
        needsRebuild: true,
      });

      rerender(<BillboardLayerRenderer {...props} />);

      // Full-rebuild path must have removed the evicted entity's billboard
      expect(collection._billboards).toHaveLength(1);
      expect(collection.remove).toHaveBeenCalledTimes(1);
    });
  });

  describe('resolveColorByHex (declarative metadata→color)', () => {
    // Mirrors the nuclear_facilities color_by rule declared in the source YAML.
    const colorBy = {
      field: 'status',
      values: {
        operational: '#00ff9d',
        under_construction: '#ffcc00',
        shutdown: '#ff9900',
        decommissioned: '#ff4444',
      },
      defaultColor: '#888888',
    };

    it('maps a known field value to its configured color', () => {
      expect(resolveColorByHex(colorBy, { status: 'operational' })).toBe('#00ff9d');
      expect(resolveColorByHex(colorBy, { status: 'under_construction' })).toBe('#ffcc00');
      expect(resolveColorByHex(colorBy, { status: 'shutdown' })).toBe('#ff9900');
      expect(resolveColorByHex(colorBy, { status: 'decommissioned' })).toBe('#ff4444');
    });

    it('falls back to defaultColor for an unknown or missing value', () => {
      expect(resolveColorByHex(colorBy, { status: 'anything_else' })).toBe('#888888');
      expect(resolveColorByHex(colorBy, {})).toBe('#888888');
      expect(resolveColorByHex(colorBy, undefined)).toBe('#888888');
    });

    it('returns undefined when no colorBy rule is configured', () => {
      expect(resolveColorByHex(undefined, { status: 'operational' })).toBeUndefined();
    });

    it('returns undefined when value is unknown and no default is set', () => {
      const noDefault = { field: 'status', values: { operational: '#00ff9d' }, defaultColor: '' };
      expect(resolveColorByHex(noDefault, { status: 'mystery' })).toBeUndefined();
    });
  });

  // -------------------------------------------------------------------------
  // Effect 2 — Non-flight layer O(delta) incremental updates + cleared path
  //
  // Non-flight layers must NOT call collection.removeAll() on every update.
  // Instead they reuse existing billboards (mutate position/color/scale/rotation
  // in place), add newly discovered entities, and (on full rebuild) remove
  // departed ones. When data clears (hasData=false) all billboards are removed.
  // -------------------------------------------------------------------------

  describe('Effect 2 — non-flight O(delta) incremental updates', () => {
    const makeQuakeProps = () => makeDefaultProps(viewer, { layerType: 'earthquakes' });

    // Build an incremental-update payload with explicit dirtyEntityIds.
    const makeIncrementalData = (
      entities: TestEntity[],
      observations: TestObservation[],
      dirty: string[],
    ) => {
      const entityMap = new Map<string, TestEntity>();
      const obsMap = new Map<string, TestObservation>();
      for (const e of entities) {
        if (e && e.id != null) entityMap.set(e.id, e);
      }
      for (const o of observations) {
        if (o) obsMap.set(o.entityId, o);
      }
      return {
        entityMap,
        obsMap,
        dirtyEntityIds: new Set(dirty),
        config: {},
        hasData: true,
        totalCount: entityMap.size,
        version: ++_versionCounter,
        maxEntities: 2000,
      };
    };

    const entityA = { id: 'eq-a', name: 'QuakeA', layerType: 'earthquakes' };
    const entityB = { id: 'eq-b', name: 'QuakeB', layerType: 'earthquakes' };
    const entityC = { id: 'eq-c', name: 'QuakeC', layerType: 'earthquakes' };
    const obsA = { entityId: 'eq-a', position: { lon: '10', lat: '20' } };
    const obsB = { entityId: 'eq-b', position: { lon: '30', lat: '40' } };
    const obsAMoved = { entityId: 'eq-a', position: { lon: '11', lat: '21' } };
    const obsC = { entityId: 'eq-c', position: { lon: '50', lat: '60' } };

    it('does NOT call removeAll for non-flight layers on initial full rebuild', () => {
      mockUseLayerEntities.mockReturnValue(makeData([entityA, entityB], [obsA, obsB]));
      render(<BillboardLayerRenderer {...makeQuakeProps()} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.removeAll).not.toHaveBeenCalled();
      expect(collection.add).toHaveBeenCalledTimes(2);
      expect(collection._billboards).toHaveLength(2);
    });

    it('reuses A in place, adds C, leaves B untouched, and never calls removeAll on incremental update', () => {
      // Initial full render with {A, B} → 2 billboards
      mockUseLayerEntities.mockReturnValue(makeData([entityA, entityB], [obsA, obsB]));
      const props = makeQuakeProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection.add).toHaveBeenCalledTimes(2);
      const bbA = collection._billboards[0];
      const bbB = collection._billboards[1];
      collection.add.mockClear();

      // Incremental update: A moved, C added; dirty={A, C}; entityMap={A', B, C}
      mockUseLayerEntities.mockReturnValue(
        makeIncrementalData([entityA, entityB, entityC], [obsAMoved, obsB, obsC], ['eq-a', 'eq-c']),
      );
      rerender(<BillboardLayerRenderer {...props} />);

      // removeAll never called — billboards reused, not destroyed/recreated
      expect(collection.removeAll).not.toHaveBeenCalled();
      // Only C triggers a new add (A reused in place)
      expect(collection.add).toHaveBeenCalledTimes(1);
      // A reused (same billboard instance) and mutated to the new position
      expect(collection._billboards[0]).toBe(bbA);
      expect(bbA.position).toMatchObject({ x: 11, y: 21 });
      // B untouched (same instance, original position)
      expect(collection._billboards[1]).toBe(bbB);
      expect(bbB.position).toMatchObject({ x: 30, y: 40 });
      // Total count reflects the add of C → 3
      expect(collection._billboards).toHaveLength(3);
    });

    it('removes existing billboards when data clears (hasData=false) → count 0', () => {
      // Initial render with {A, B}
      mockUseLayerEntities.mockReturnValue(makeData([entityA, entityB], [obsA, obsB]));
      const props = makeQuakeProps();
      const { rerender } = render(<BillboardLayerRenderer {...props} />);

      const collection = getLastCollectionInstance()!;
      expect(collection._billboards).toHaveLength(2);

      // Data clears
      mockUseLayerEntities.mockReturnValue(makeEmptyData());
      rerender(<BillboardLayerRenderer {...props} />);

      // Existing billboards removed individually → count 0
      expect(collection.remove).toHaveBeenCalledTimes(2);
      expect(collection._billboards).toHaveLength(0);
    });

    it('colors entities by a declarative metadata→color rule (displayConfig.colorBy)', () => {
      // Seed the layer's color_by rule the way GetLayers would (from source YAML).
      useUIStore.getState().setLayers([
        {
          id: 'nuclear_facilities',
          name: 'nuclear_facilities',
          type: 'nuclear_facilities',
          enabled: true,
          mode: 'live',
          density: 1,
          source: 'nuclear_facilities',
          lastUpdate: 0,
          count: 0,
          color: '#ffffff',
          pointSize: 8,
          displayConfig: {
            icon: { shape: 'radiation', rotatable: false, interpolation: false, scale: 1.0 },
            trail: { color: '', width: 1.0, opacity: 0.6 },
            style: { color: '#ff9900', pointSize: 6 },
            fieldRenderers: [],
            colorBy: {
              field: 'status',
              values: { operational: '#00ff9d', shutdown: '#ff9900' },
              defaultColor: '#888888',
            },
          },
        },
      ]);

      const nukeEntity = { id: 'nk-1', name: 'Reactor', layerType: 'nuclear_facilities' };
      const nukeObs = {
        entityId: 'nk-1',
        position: { lon: '5', lat: '5' },
        metadata: { status: 'shutdown' },
      } as unknown as TestObservation;

      mockUseLayerEntities.mockReturnValue(makeData([nukeEntity], [nukeObs]));
      render(
        <BillboardLayerRenderer
          {...makeDefaultProps(viewer, { layerType: 'nuclear_facilities' })}
        />,
      );

      const collection = getLastCollectionInstance()!;
      const addCall = collection.add.mock.calls[0][0];
      // status "shutdown" → orange (#ff9900) via fromCssColorString (mock stamps _css)
      expect((addCall.color as unknown as { _css?: string })._css).toBe('#ff9900');
    });
  });
});
