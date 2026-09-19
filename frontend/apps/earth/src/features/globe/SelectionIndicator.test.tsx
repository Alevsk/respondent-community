/**
 * SelectionIndicator Component Tests
 *
 * Tests the headless globe component that renders tactical corner brackets
 * around selected and pinned watchlist entities, including:
 * - BillboardCollection + LabelCollection lifecycle (create/destroy)
 * - Incremental bracket management (the flicker-fix — NO removeAll)
 * - Pinned entity bracket alpha differentiation
 * - Label updates (fillColor, text)
 * - preUpdate position tracking (interpolatedPositions + store fallback)
 * - Isolation mode filtering
 * - Entity deduplication (selected + watchlist overlap)
 */

import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from 'vitest';
import { render, act } from '@testing-library/react';

// ---------------------------------------------------------------------------
// vi.hoisted() — variables declared here are available inside vi.mock() factories
// because vi.mock() calls are hoisted to the top of the file by the transform.
// ---------------------------------------------------------------------------

const {
  MockCartesian3,
  MockCartesian2,
  MockCartographic,
  MockBillboardCollectionCtor,
  MockLabelCollectionCtor,
  MockColor,
  MockCesiumMath,
  MockLabelStyle,
  MockVerticalOrigin,
  MockHorizontalOrigin,
  getLastBillboardCollectionInstance,
  getLastLabelCollectionInstance,
  mockInterpolatedPositions,
} = vi.hoisted(() => {
  interface MockBillboard {
    position: MockCartesian3Impl;
    image: HTMLCanvasElement | null;
    color: { alpha: number };
    scale: number;
    disableDepthTestDistance: number;
    id: string | null;
  }

  interface MockLabel {
    position: MockCartesian3Impl;
    text: string;
    font: string;
    fillColor: { alpha: number };
    outlineColor: { alpha: number };
    outlineWidth: number;
    style: string | null;
    verticalOrigin: string | null;
    horizontalOrigin: string | null;
    pixelOffset: MockCartesian2Impl | null;
    disableDepthTestDistance: number;
    id: string | null;
  }

  interface MockColorInstance {
    r: number;
    g: number;
    b: number;
    alpha: number;
    _css?: string;
    // Use the actual function shape (not `ReturnType<typeof vi.fn>`, which
    // resolves to the fully-generic `Mock<any[], unknown>` — invariant in
    // vitest 1.x — and rejects narrower typed mocks like `Mock<[number], X>`).
    withAlpha: (a: number) => MockColorInstance;
  }

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
      (lon: number, lat: number, alt = 0) => new MockCartesian3Impl(lon, lat, alt),
    );

    static clone = vi.fn((c: MockCartesian3Impl) => new MockCartesian3Impl(c.x, c.y, c.z));
  }

  // ---------------------------------------------------------------------------
  // Cartesian2 mock
  // ---------------------------------------------------------------------------
  class MockCartesian2Impl {
    x: number;
    y: number;
    constructor(x = 0, y = 0) {
      this.x = x;
      this.y = y;
    }
  }

  // ---------------------------------------------------------------------------
  // Cartographic mock — fromCartesian returns an object with latitude/longitude
  // in radians so that CesiumMath.toDegrees converts them to degree values.
  // ---------------------------------------------------------------------------
  const MockCartographicImpl = {
    fromCartesian: vi.fn((pos: MockCartesian3Impl) => ({
      // Store lon/lat in radians — toDegrees will convert back
      latitude: (pos.y * Math.PI) / 180,
      longitude: (pos.x * Math.PI) / 180,
    })),
  };

  // ---------------------------------------------------------------------------
  // BillboardCollection mock
  // ---------------------------------------------------------------------------
  let _lastBillboardCollectionInstance: MockBillboardCollectionImpl | null = null;

  class MockBillboardCollectionImpl {
    _billboards: MockBillboard[] = [];
    _destroyed = false;

    add = vi.fn((opts: Partial<MockBillboard>): MockBillboard => {
      const bb: MockBillboard = {
        position: opts?.position ?? new MockCartesian3Impl(),
        image: opts?.image ?? null,
        color: opts?.color ?? { alpha: 1 },
        scale: opts?.scale ?? 1,
        disableDepthTestDistance: opts?.disableDepthTestDistance ?? 0,
        id: opts?.id ?? null,
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
    _lastBillboardCollectionInstance = instance;
    return instance;
  });

  // ---------------------------------------------------------------------------
  // LabelCollection mock
  // ---------------------------------------------------------------------------
  let _lastLabelCollectionInstance: MockLabelCollectionImpl | null = null;

  class MockLabelCollectionImpl {
    _labels: MockLabel[] = [];
    _destroyed = false;

    add = vi.fn((opts: Partial<MockLabel>): MockLabel => {
      const lb: MockLabel = {
        position: opts?.position ?? new MockCartesian3Impl(),
        text: opts?.text ?? '',
        font: opts?.font ?? '',
        fillColor: opts?.fillColor ?? { alpha: 1 },
        outlineColor: opts?.outlineColor ?? { alpha: 1 },
        outlineWidth: opts?.outlineWidth ?? 0,
        style: opts?.style ?? null,
        verticalOrigin: opts?.verticalOrigin ?? null,
        horizontalOrigin: opts?.horizontalOrigin ?? null,
        pixelOffset: opts?.pixelOffset ?? null,
        disableDepthTestDistance: opts?.disableDepthTestDistance ?? 0,
        id: opts?.id ?? null,
      };
      this._labels.push(lb);
      return lb;
    });

    remove = vi.fn((lb: MockLabel) => {
      const idx = this._labels.indexOf(lb);
      if (idx !== -1) this._labels.splice(idx, 1);
      return idx !== -1;
    });

    removeAll = vi.fn(() => {
      this._labels = [];
    });

    get = vi.fn((i: number) => this._labels[i]);

    get length() {
      return this._labels.length;
    }

    isDestroyed = vi.fn(() => this._destroyed);

    destroy = vi.fn(() => {
      this._destroyed = true;
    });
  }

  const MockLabelCollectionCtorImpl = vi.fn(function () {
    const instance = new MockLabelCollectionImpl();
    _lastLabelCollectionInstance = instance;
    return instance;
  });

  // ---------------------------------------------------------------------------
  // Color mock
  // ---------------------------------------------------------------------------
  class MockColorImpl {
    r: number;
    g: number;
    b: number;
    alpha: number;
    _css?: string;

    constructor(r = 0, g = 0, b = 0, alpha = 1) {
      this.r = r;
      this.g = g;
      this.b = b;
      this.alpha = alpha;
    }

    withAlpha = vi.fn((a: number) => {
      return Object.assign(new MockColorImpl(this.r, this.g, this.b, a), { _css: this._css });
    });

    static BLACK = new MockColorImpl(0, 0, 0, 1);

    static fromCssColorString = vi.fn((css: string) => {
      return Object.assign(new MockColorImpl(0, 1, 0.6, 1), { _css: css });
    });

    static clone = vi.fn((src: MockColorInstance, dest?: MockColorInstance) => {
      if (!dest) {
        return Object.assign(
          new MockColorImpl(src.r ?? 0, src.g ?? 0, src.b ?? 0, src.alpha ?? 1),
          { _css: src._css },
        );
      }
      dest.r = src.r;
      dest.g = src.g;
      dest.b = src.b;
      dest.alpha = src.alpha;
      return dest;
    });
  }

  // Fix withAlpha on BLACK singleton
  (MockColorImpl.BLACK as MockColorInstance).withAlpha = vi.fn(
    (a: number) => new MockColorImpl(0, 0, 0, a),
  );

  // ---------------------------------------------------------------------------
  // CesiumMath mock
  // ---------------------------------------------------------------------------
  const MockCesiumMathImpl = {
    toDegrees: vi.fn((rad: number) => (rad * 180) / Math.PI),
    toRadians: vi.fn((deg: number) => (deg * Math.PI) / 180),
  };

  // ---------------------------------------------------------------------------
  // Cesium enum mocks
  // ---------------------------------------------------------------------------
  const MockLabelStyleImpl = { FILL_AND_OUTLINE: 'FILL_AND_OUTLINE' };
  const MockVerticalOriginImpl = { TOP: 'TOP', CENTER: 'CENTER', BOTTOM: 'BOTTOM' };
  const MockHorizontalOriginImpl = { CENTER: 'CENTER', LEFT: 'LEFT', RIGHT: 'RIGHT' };

  // ---------------------------------------------------------------------------
  // interpolatedPositions mock (mutable map, reset between tests)
  // ---------------------------------------------------------------------------
  const mockInterpolatedPositionsImpl = new Map<string, MockCartesian3Impl>();

  return {
    MockCartesian3: MockCartesian3Impl,
    MockCartesian2: MockCartesian2Impl,
    MockCartographic: MockCartographicImpl,
    MockBillboardCollectionCtor: MockBillboardCollectionCtorImpl,
    MockLabelCollectionCtor: MockLabelCollectionCtorImpl,
    MockColor: MockColorImpl,
    MockCesiumMath: MockCesiumMathImpl,
    MockLabelStyle: MockLabelStyleImpl,
    MockVerticalOrigin: MockVerticalOriginImpl,
    MockHorizontalOrigin: MockHorizontalOriginImpl,
    getLastBillboardCollectionInstance: () => _lastBillboardCollectionInstance,
    getLastLabelCollectionInstance: () => _lastLabelCollectionInstance,
    mockInterpolatedPositions: mockInterpolatedPositionsImpl,
  };
});

// ---------------------------------------------------------------------------
// Canvas mock — jsdom does not implement CanvasRenderingContext2D.
// Patch the prototype before any module imports so createBracketCanvas()
// receives a no-op context and does not throw.
// Must use beforeAll since HTMLCanvasElement isn't defined at module scope
// until after jsdom environment setup.
// ---------------------------------------------------------------------------

beforeAll(() => {
  HTMLCanvasElement.prototype.getContext = vi.fn(() => ({
    strokeStyle: '',
    lineWidth: 0,
    shadowColor: '',
    shadowBlur: 0,
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    stroke: vi.fn(),
  })) as unknown as typeof HTMLCanvasElement.prototype.getContext;
});

// ---------------------------------------------------------------------------
// Module mocks — factories can safely reference hoisted variables
// ---------------------------------------------------------------------------

vi.mock('cesium', () => ({
  BillboardCollection: MockBillboardCollectionCtor,
  LabelCollection: MockLabelCollectionCtor,
  Cartesian3: MockCartesian3,
  Cartesian2: MockCartesian2,
  Cartographic: MockCartographic,
  Color: MockColor,
  Math: MockCesiumMath,
  LabelStyle: MockLabelStyle,
  VerticalOrigin: MockVerticalOrigin,
  HorizontalOrigin: MockHorizontalOrigin,
}));

vi.mock('./interpolatedPositions', () => ({
  interpolatedPositions: mockInterpolatedPositions,
}));

vi.mock('@/app/selectors', () => ({
  selectIsolatedEntityId: (s: {
    entityViewState: Record<string, { isolateEntity?: boolean }>;
    selectedEntities: Array<{ entityId: string }>;
  }) => {
    for (const [eid, vs] of Object.entries(s.entityViewState)) {
      if (vs.isolateEntity && s.selectedEntities.some((e) => e.entityId === eid)) {
        return eid;
      }
    }
    return null;
  },
}));

// ---------------------------------------------------------------------------
// Import subject under test — comes after all vi.mock() declarations
// ---------------------------------------------------------------------------

import { SelectionIndicator } from './SelectionIndicator';
import { useUIStore, type Entity, type Observation, type LayerData } from '@/app/store';

// ---------------------------------------------------------------------------
// Viewer factory helper
// ---------------------------------------------------------------------------

function createMockViewer() {
  const preUpdateListeners: Array<() => void> = [];
  const postRenderListeners: Array<() => void> = [];

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
    _firePreUpdate() {
      preUpdateListeners.forEach((fn) => fn());
    },
    _firePostRender() {
      postRenderListeners.forEach((fn) => fn());
    },
  };
  return viewer;
}

type MockViewer = ReturnType<typeof createMockViewer>;

// ---------------------------------------------------------------------------
// Store helpers
// ---------------------------------------------------------------------------

/** Reset the store to a clean baseline state before each test. */
function resetStore() {
  useUIStore.setState({
    selectedEntities: [],
    watchlistEntities: [],
    layerEntities: new Map(),
    layerVersions: {},
    entityViewState: {},
  });
}

/** Build a LayerData map entry from flat arrays. */
function makeLayerData(
  observations: Array<{ entityId: string; lat: number; lon: number; altitudeM?: number }>,
): LayerData {
  const entityMap = new Map<string, Entity>();
  const obsMap = new Map<string, Observation>();
  const dirtyEntityIds = new Set<string>();
  for (const obs of observations) {
    obsMap.set(obs.entityId, {
      entityId: obs.entityId,
      position: { lat: obs.lat, lon: obs.lon },
      altitudeM: obs.altitudeM ?? 0,
      timestamp: new Date().toISOString(),
    });
    // Include layerType + metadata so this satisfies the real Entity contract
    // used by `LayerData` — a previous minimal shape silently broke once the
    // store tightened its types.
    entityMap.set(obs.entityId, {
      id: obs.entityId,
      externalId: obs.entityId,
      layerType: '',
      name: obs.entityId,
      metadata: {},
    });
    dirtyEntityIds.add(obs.entityId);
  }
  return { entityMap, obsMap, dirtyEntityIds };
}

/** Populate the store with a single layer containing the given observations. */
function setLayerObs(
  layerId: string,
  observations: Array<{ entityId: string; lat: number; lon: number; altitudeM?: number }>,
) {
  const data = makeLayerData(observations);
  useUIStore.setState({
    layerEntities: new Map([[layerId, data]]),
    layerVersions: { [layerId]: 1 },
  });
}

// ---------------------------------------------------------------------------
// Default prop factory
// ---------------------------------------------------------------------------

import type { SelectionIndicatorProps } from './SelectionIndicator';

function makeProps(
  viewer: MockViewer,
  overrides: Partial<SelectionIndicatorProps> = {},
): SelectionIndicatorProps {
  return {
    // MockViewer only implements the subset of Viewer that SelectionIndicator
    // touches. Cast via `unknown` because the nominal Viewer type has 40+
    // fields (container, creditDisplay, cesiumWidget, …) we deliberately don't
    // stub. This isolates the cast to a single place instead of 30+ JSX sites.
    viewerRef: { current: viewer } as unknown as SelectionIndicatorProps['viewerRef'],
    viewerReady: true,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe('SelectionIndicator', () => {
  let viewer: MockViewer;

  beforeEach(() => {
    vi.clearAllMocks();
    mockInterpolatedPositions.clear();
    resetStore();
    viewer = createMockViewer();
  });

  afterEach(() => {
    // Ensure clean store state after each test
    resetStore();
  });

  // -------------------------------------------------------------------------
  // Collection lifecycle
  // -------------------------------------------------------------------------

  describe('Collection lifecycle', () => {
    it('creates BillboardCollection and LabelCollection on mount', () => {
      render(<SelectionIndicator {...makeProps(viewer)} />);

      expect(MockBillboardCollectionCtor).toHaveBeenCalledTimes(1);
      expect(MockLabelCollectionCtor).toHaveBeenCalledTimes(1);
      // Both primitives added to scene
      expect(viewer.scene.primitives.add).toHaveBeenCalledTimes(2);
      expect(viewer.scene.primitives.add).toHaveBeenCalledWith(
        getLastBillboardCollectionInstance(),
      );
      expect(viewer.scene.primitives.add).toHaveBeenCalledWith(getLastLabelCollectionInstance());
    });

    it('removes and destroys both collections on unmount', () => {
      const { unmount } = render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;
      const lbCol = getLastLabelCollectionInstance()!;

      unmount();

      expect(viewer.scene.primitives.remove).toHaveBeenCalledWith(bbCol);
      expect(viewer.scene.primitives.remove).toHaveBeenCalledWith(lbCol);
      expect(bbCol.destroy).toHaveBeenCalledTimes(1);
      expect(lbCol.destroy).toHaveBeenCalledTimes(1);
    });

    it('does not remove primitives if viewer is destroyed at cleanup time', () => {
      const { unmount } = render(<SelectionIndicator {...makeProps(viewer)} />);
      viewer.isDestroyed.mockReturnValue(true);

      unmount();

      expect(viewer.scene.primitives.remove).not.toHaveBeenCalled();
    });

    it('does not destroy collection if already destroyed at cleanup time', () => {
      const { unmount } = render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;
      const lbCol = getLastLabelCollectionInstance()!;
      bbCol._destroyed = true;
      lbCol._destroyed = true;

      unmount();

      expect(bbCol.destroy).not.toHaveBeenCalled();
      expect(lbCol.destroy).not.toHaveBeenCalled();
    });

    it('skips collection creation when viewer is null', () => {
      render(<SelectionIndicator viewerRef={{ current: null }} viewerReady={false} />);

      expect(MockBillboardCollectionCtor).not.toHaveBeenCalled();
      expect(MockLabelCollectionCtor).not.toHaveBeenCalled();
    });

    it('skips collection creation when viewer is destroyed', () => {
      viewer.isDestroyed.mockReturnValue(true);
      render(<SelectionIndicator {...makeProps(viewer)} />);

      expect(MockBillboardCollectionCtor).not.toHaveBeenCalled();
      expect(MockLabelCollectionCtor).not.toHaveBeenCalled();
    });

    it('renders null (headless component)', () => {
      const { container } = render(<SelectionIndicator {...makeProps(viewer)} />);
      expect(container.firstChild).toBeNull();
    });
  });

  // -------------------------------------------------------------------------
  // Incremental bracket management (flicker fix)
  // -------------------------------------------------------------------------

  describe('Incremental bracket management', () => {
    it('adds a bracket billboard for a selected entity', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(bbCol.add).toHaveBeenCalledTimes(1);
      expect(bbCol._billboards).toHaveLength(1);
      expect(bbCol._billboards[0].id).toBe('ent-1');
    });

    it('adds a coordinate label below the bracket', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(lbCol.add).toHaveBeenCalledTimes(1);
      expect(lbCol._labels).toHaveLength(1);
      const label = lbCol._labels[0];
      expect(label.id).toBe('ent-1');
      expect(label.text).toContain('32.0000°');
      expect(label.text).toContain('117.0000°');
    });

    it('updates bracket position in-place — does NOT call removeAll', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Now update the layer data (position change) — version bump triggers re-render
      await act(async () => {
        setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 33.0, lon: -118.0 }]);
      });

      // removeAll must NEVER be called (that would cause flicker)
      expect(bbCol.removeAll).not.toHaveBeenCalled();
      // Position should be updated in-place, not via add/remove
      expect(bbCol._billboards).toHaveLength(1);
    });

    it('removes bracket when entity is deselected', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });
      expect(bbCol._billboards).toHaveLength(1);

      await act(async () => {
        useUIStore.setState({ selectedEntities: [] });
      });

      expect(bbCol.remove).toHaveBeenCalledTimes(1);
      expect(bbCol._billboards).toHaveLength(0);
    });

    it('adds a new bracket while keeping existing ones (incremental add)', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-1', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-2', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });
      expect(bbCol._billboards).toHaveLength(1);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [
            { entityId: 'ent-1', layerId: 'layer-1' },
            { entityId: 'ent-2', layerId: 'layer-1' },
          ],
        });
      });

      expect(bbCol._billboards).toHaveLength(2);
      // No removeAll — incremental add only
      expect(bbCol.removeAll).not.toHaveBeenCalled();
    });

    it('does NOT call collection.removeAll() at any point after initial setup', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-1', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-2', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;
      const lbCol = getLastLabelCollectionInstance()!;

      // Select one
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Select another
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [
            { entityId: 'ent-1', layerId: 'layer-1' },
            { entityId: 'ent-2', layerId: 'layer-1' },
          ],
        });
      });

      // Deselect one
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-2', layerId: 'layer-1' }],
        });
      });

      // Clear all
      await act(async () => {
        useUIStore.setState({ selectedEntities: [] });
      });

      // The critical flicker-fix assertion
      expect(bbCol.removeAll).not.toHaveBeenCalled();
      expect(lbCol.removeAll).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Pinned entity brackets
  // -------------------------------------------------------------------------

  describe('Pinned entity brackets', () => {
    it('shows brackets for pinned watchlist entities', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-pinned', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          watchlistEntities: [
            {
              entityId: 'ent-pinned',
              layerId: 'layer-1',
              name: 'Pinned Entity',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      expect(bbCol._billboards).toHaveLength(1);
      expect(bbCol._billboards[0].id).toBe('ent-pinned');
    });

    it('uses PINNED_BRACKET_ALPHA (0.6) for pinned-only entities', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-pinned', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          watchlistEntities: [
            {
              entityId: 'ent-pinned',
              layerId: 'layer-1',
              name: 'Pinned Entity',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      const bb = bbCol._billboards[0];
      expect(bb.color.alpha).toBe(0.6);
    });

    it('uses SELECTED_BRACKET_ALPHA (0.9) for selected entities', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-sel', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-sel', layerId: 'layer-1' }],
        });
      });

      const bb = bbCol._billboards[0];
      expect(bb.color.alpha).toBe(0.9);
    });

    it('updates alpha when entity transitions from pinned to selected', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      // Start as pinned-only
      await act(async () => {
        useUIStore.setState({
          watchlistEntities: [
            {
              entityId: 'ent-1',
              layerId: 'layer-1',
              name: 'Entity 1',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      expect(bbCol._billboards[0].color.alpha).toBe(0.6);

      // Now select it — should transition to SELECTED_BRACKET_ALPHA
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(bbCol._billboards[0].color.alpha).toBe(0.9);
    });
  });

  // -------------------------------------------------------------------------
  // Label updates
  // -------------------------------------------------------------------------

  describe('Label updates', () => {
    it('updates label fillColor when entity transitions between pinned/selected states', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      // Start as pinned-only (label alpha = 0.55)
      await act(async () => {
        useUIStore.setState({
          watchlistEntities: [
            {
              entityId: 'ent-1',
              layerId: 'layer-1',
              name: 'Entity 1',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      expect(lbCol._labels[0].fillColor.alpha).toBe(0.55);

      // Transition to selected (label alpha = 0.85)
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(lbCol._labels[0].fillColor.alpha).toBe(0.85);
    });

    it('label contains correctly formatted lat/lon coordinates', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 34.1234, lon: -118.5678 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const label = lbCol._labels[0];
      expect(label.text).toContain('34.1234°');
      expect(label.text).toContain('N');
      expect(label.text).toContain('118.5678°');
      expect(label.text).toContain('W');
    });

    it('label has correct font and style properties', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const label = lbCol._labels[0];
      expect(label.font).toBe('11px monospace');
      expect(label.style).toBe('FILL_AND_OUTLINE');
      expect(label.verticalOrigin).toBe('TOP');
      expect(label.horizontalOrigin).toBe('CENTER');
    });
  });

  // -------------------------------------------------------------------------
  // preUpdate position tracking
  // -------------------------------------------------------------------------

  describe('preUpdate position tracking', () => {
    it('registers a preUpdate listener when there are entities to track', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(viewer.scene.preUpdate.addEventListener).toHaveBeenCalled();
      expect(viewer.scene.preUpdate._listeners).toHaveLength(1);
    });

    it('reads from interpolatedPositions map (O(1) lookup) when available', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Place an interpolated position into the shared map
      const interpolatedPos = new MockCartesian3(99, 55, 11);
      mockInterpolatedPositions.set('ent-1', interpolatedPos);

      // Fire the preUpdate listener
      act(() => {
        viewer._firePreUpdate();
      });

      // Bracket position should be set to the interpolated value
      expect(bbCol._billboards[0].position).toBe(interpolatedPos);
    });

    it('falls back to store position when interpolatedPositions is empty', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Ensure interpolatedPositions is empty for this entity
      mockInterpolatedPositions.clear();

      act(() => {
        viewer._firePreUpdate();
      });

      // Should have called fromDegrees with store position
      expect(MockCartesian3.fromDegrees).toHaveBeenCalledWith(-117.0, 32.0, 0);
      // Billboard position should be updated
      expect(bbCol._billboards[0].position).toBeDefined();
    });

    it('updates label position on preUpdate', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const interpolatedPos = new MockCartesian3(99, 55, 11);
      mockInterpolatedPositions.set('ent-1', interpolatedPos);

      act(() => {
        viewer._firePreUpdate();
      });

      expect(lbCol._labels[0].position).toBe(interpolatedPos);
    });

    it('calls requestRender when positions moved', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const interpolatedPos = new MockCartesian3(99, 55, 11);
      mockInterpolatedPositions.set('ent-1', interpolatedPos);
      viewer.scene.requestRender.mockClear();

      act(() => {
        viewer._firePreUpdate();
      });

      expect(viewer.scene.requestRender).toHaveBeenCalled();
    });

    it('updates label text via Cartographic fromCartesian on preUpdate', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Set an interpolated position — x=lon, y=lat (our mock convention)
      const interpolatedPos = new MockCartesian3(-74.0, 40.7, 0);
      mockInterpolatedPositions.set('ent-1', interpolatedPos);

      act(() => {
        viewer._firePreUpdate();
      });

      // After preUpdate the label text should reflect the new coordinates
      // fromCartesian converts x→lon, y→lat via our mock
      const label = lbCol._labels[0];
      expect(label.text).toContain('40.7000°');
      expect(label.text).toContain('74.0000°');
    });

    it('removes preUpdate listener on unmount', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      const { unmount } = render(<SelectionIndicator {...makeProps(viewer)} />);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      expect(viewer.scene.preUpdate._listeners).toHaveLength(1);

      unmount();

      expect(viewer.scene.preUpdate._listeners).toHaveLength(0);
    });
  });

  // -------------------------------------------------------------------------
  // Isolation mode
  // -------------------------------------------------------------------------

  describe('Isolation mode', () => {
    it('only shows brackets for the isolated entity when isolatedEntityId is set', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-1', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-2', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      // Select both entities and isolate ent-1
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [
            { entityId: 'ent-1', layerId: 'layer-1' },
            { entityId: 'ent-2', layerId: 'layer-1' },
          ],
          entityViewState: {
            'ent-1': { trailHighlight: null, activeTab: 'overview', isolateEntity: true },
          },
        });
      });

      // Only ent-1 should have a bracket
      expect(bbCol._billboards).toHaveLength(1);
      expect(bbCol._billboards[0].id).toBe('ent-1');
    });

    it('shows all brackets when isolatedEntityId is null', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-1', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-2', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [
            { entityId: 'ent-1', layerId: 'layer-1' },
            { entityId: 'ent-2', layerId: 'layer-1' },
          ],
          // No isolation
          entityViewState: {},
        });
      });

      expect(bbCol._billboards).toHaveLength(2);
    });

    it('removes extra brackets when isolation is enabled', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-1', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-2', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      // First, select both without isolation
      await act(async () => {
        useUIStore.setState({
          selectedEntities: [
            { entityId: 'ent-1', layerId: 'layer-1' },
            { entityId: 'ent-2', layerId: 'layer-1' },
          ],
          entityViewState: {},
        });
      });
      expect(bbCol._billboards).toHaveLength(2);

      // Now isolate ent-1
      await act(async () => {
        useUIStore.setState({
          entityViewState: {
            'ent-1': { trailHighlight: null, activeTab: 'overview', isolateEntity: true },
          },
        });
      });

      // ent-2 bracket should be removed
      expect(bbCol._billboards).toHaveLength(1);
      expect(bbCol._billboards[0].id).toBe('ent-1');
      expect(bbCol.removeAll).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // Entity deduplication
  // -------------------------------------------------------------------------

  describe('Entity deduplication', () => {
    it('deduplicates entities that are both selected and in watchlist', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
          watchlistEntities: [
            {
              entityId: 'ent-1',
              layerId: 'layer-1',
              name: 'Entity 1',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      // Only ONE bracket, not two
      expect(bbCol._billboards).toHaveLength(1);
    });

    it('selected entities take priority (SELECTED_BRACKET_ALPHA) over pinned', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
          watchlistEntities: [
            {
              entityId: 'ent-1',
              layerId: 'layer-1',
              name: 'Entity 1',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      // Selected takes priority → full brightness alpha
      expect(bbCol._billboards[0].color.alpha).toBe(0.9);
    });

    it('shows brackets for multiple unique entities from selected and watchlist', async () => {
      setLayerObs('layer-1', [
        { entityId: 'ent-selected', lat: 32.0, lon: -117.0 },
        { entityId: 'ent-pinned', lat: 40.0, lon: -74.0 },
      ]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-selected', layerId: 'layer-1' }],
          watchlistEntities: [
            {
              entityId: 'ent-pinned',
              layerId: 'layer-1',
              name: 'Pinned',
              pinned: true,
              addedAt: Date.now(),
            },
          ],
        });
      });

      expect(bbCol._billboards).toHaveLength(2);
      const ids = bbCol._billboards.map((b) => b.id);
      expect(ids).toContain('ent-selected');
      expect(ids).toContain('ent-pinned');
    });
  });

  // -------------------------------------------------------------------------
  // Edge cases
  // -------------------------------------------------------------------------

  describe('Edge cases', () => {
    it('does not add bracket when entity has no observation data', async () => {
      // No layer data set — obsMap is empty
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const bbCol = getLastBillboardCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-unknown', layerId: 'layer-1' }],
        });
      });

      expect(bbCol._billboards).toHaveLength(0);
    });

    it('does not crash when viewer is destroyed before preUpdate fires', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const bbCol = getLastBillboardCollectionInstance()!;
      bbCol._destroyed = true;

      // Should not throw when collection is destroyed
      expect(() => {
        act(() => {
          viewer._firePreUpdate();
        });
      }).not.toThrow();
    });

    it('handles southern hemisphere coordinates with S/W suffix', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: -33.8688, lon: 151.2093 }]);
      render(<SelectionIndicator {...makeProps(viewer)} />);
      const lbCol = getLastLabelCollectionInstance()!;

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      const label = lbCol._labels[0];
      expect(label.text).toContain('S');
      expect(label.text).toContain('E');
    });

    it('clears collections refs on unmount so preUpdate is a no-op', async () => {
      setLayerObs('layer-1', [{ entityId: 'ent-1', lat: 32.0, lon: -117.0 }]);
      const { unmount } = render(<SelectionIndicator {...makeProps(viewer)} />);

      await act(async () => {
        useUIStore.setState({
          selectedEntities: [{ entityId: 'ent-1', layerId: 'layer-1' }],
        });
      });

      // Capture collection before unmount
      const bbCol = getLastBillboardCollectionInstance()!;

      unmount();

      // After unmount, firing preUpdate should not interact with the destroyed collection
      bbCol.add.mockClear();
      expect(() => {
        act(() => {
          viewer._firePreUpdate();
        });
      }).not.toThrow();
      // No new billboards should be added after unmount
      expect(bbCol.add).not.toHaveBeenCalled();
    });
  });
});
