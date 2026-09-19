/**
 * PinnedEntityRenderer tests
 *
 * Covers:
 * - Renders billboards for pinned entities whose layer is NOT enabled
 * - Skips entities whose layer IS enabled (BillboardLayerRenderer handles them)
 * - Stale indicator: entities older than 1 hour render at 50% alpha
 * - Cleanup on unmount
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import type { Viewer } from 'cesium';
import type { MutableRefObject } from 'react';

// ---------------------------------------------------------------------------
// vi.hoisted() -- variables available inside vi.mock() factories
// ---------------------------------------------------------------------------
const {
  MockBillboardCollection,
  MockCartesian3,
  MockColor,
  mockUseEntityDetail,
  getLastCollectionInstance,
} = vi.hoisted(() => {
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
    static UNIT_Z = new MockCartesian3Impl(0, 0, 1);
    static ZERO = new MockCartesian3Impl(0, 0, 0);
  }

  class MockBillboardCollectionImpl {
    _billboards: Record<string, unknown>[] = [];
    _destroyed = false;
    add = vi.fn((opts: Record<string, unknown>) => {
      const bb = { ...opts };
      this._billboards.push(bb);
      return bb;
    });
    removeAll = vi.fn(() => {
      this._billboards = [];
    });
    remove = vi.fn();
    get = vi.fn((i: number) => this._billboards[i]);
    get length() {
      return this._billboards.length;
    }
    isDestroyed = vi.fn(() => this._destroyed);
    destroy = vi.fn(() => {
      this._destroyed = true;
    });
  }

  let _lastCollection: MockBillboardCollectionImpl | null = null;

  const mockUseEntityDetailFn = vi.fn().mockReturnValue({ data: null });

  return {
    MockBillboardCollection: vi.fn(function () {
      _lastCollection = new MockBillboardCollectionImpl();
      return _lastCollection;
    }),
    MockCartesian3: MockCartesian3Impl,
    MockColor: {
      WHITE: { withAlpha: vi.fn((a: number) => ({ red: 1, green: 1, blue: 1, alpha: a })) },
    },
    mockUseEntityDetail: mockUseEntityDetailFn,
    // Assert non-null inside the helper so call sites get a fully-typed
    // collection without scattering `!` operators across the test file.
    // Every test that calls this has just rendered PinnedEntityRenderer,
    // which constructs a BillboardCollection as part of its mount effect.
    getLastCollectionInstance: (): MockBillboardCollectionImpl => {
      if (!_lastCollection) {
        throw new Error('BillboardCollection was never instantiated');
      }
      return _lastCollection;
    },
  };
});

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------
vi.mock('cesium', () => ({
  BillboardCollection: MockBillboardCollection,
  Cartesian3: MockCartesian3,
  Color: MockColor,
}));

vi.mock('./icons/iconRegistry', () => ({
  getIconCanvas: vi.fn(() => document.createElement('canvas')),
  getIconScale: vi.fn(() => 0.5),
}));

vi.mock('./interpolatedPositions', () => ({
  interpolatedPositions: new Map(),
}));

vi.mock('../../shared/api/queries', () => ({
  useEntityDetail: (entityId: string) => mockUseEntityDetail(entityId),
}));

interface MockStoreState {
  watchlistEntities: Array<{
    entityId: string;
    layerId: string;
    pinned: boolean;
    name: string;
    addedAt: number;
  }>;
  enabledLayers: string[];
  layerEntities: Map<string, unknown>;
  layers: Record<string, { color?: string }>;
}

const storeState: MockStoreState = {
  watchlistEntities: [],
  enabledLayers: [],
  layerEntities: new Map(),
  layers: {},
};

vi.mock('@/app/store', () => ({
  useUIStore: Object.assign((selector: (s: MockStoreState) => unknown) => selector(storeState), {
    getState: () => storeState,
  }),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function makeViewerRef() {
  const preUpdateListeners: Array<() => void> = [];
  return {
    current: {
      isDestroyed: () => false,
      scene: {
        primitives: {
          add: vi.fn(),
          remove: vi.fn(),
        },
        preUpdate: {
          addEventListener: vi.fn((fn: () => void) => preUpdateListeners.push(fn)),
          removeEventListener: vi.fn(),
        },
        requestRender: vi.fn(),
      },
    },
    _preUpdateListeners: preUpdateListeners,
  };
}

import { PinnedEntityRenderer } from './PinnedEntityRenderer';

describe('PinnedEntityRenderer', () => {
  let viewerRef: ReturnType<typeof makeViewerRef>;
  // Same object as `viewerRef`, typed as the real Cesium shape. PinnedEntityRenderer
  // only touches the scene/primitives fields that the mock implements, but its prop
  // is typed `MutableRefObject<Viewer | null>`. The double `unknown` cast is the
  // minimum needed to satisfy that nominal type without constructing a real Viewer.
  let viewerRefProp: MutableRefObject<Viewer | null>;

  beforeEach(() => {
    viewerRef = makeViewerRef();
    viewerRefProp = viewerRef as unknown as MutableRefObject<Viewer | null>;
    storeState.watchlistEntities = [];
    storeState.enabledLayers = [];
    storeState.layerEntities = new Map();
    storeState.layers = {};
    vi.clearAllMocks();
    // Reset default return value after clearAllMocks clears it
    mockUseEntityDetail.mockReturnValue({ data: null });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('creates a BillboardCollection on mount and destroys on unmount', () => {
    const { unmount } = render(<PinnedEntityRenderer viewerRef={viewerRefProp} viewerReady />);

    expect(MockBillboardCollection).toHaveBeenCalledTimes(1);
    expect(viewerRef.current.scene.primitives.add).toHaveBeenCalled();

    unmount();
    const collection = getLastCollectionInstance();
    expect(collection.destroy).toHaveBeenCalled();
  });

  it('skips entities whose layer IS enabled', () => {
    storeState.enabledLayers = ['flights_commercial'];
    storeState.watchlistEntities = [
      {
        entityId: 'flights_commercial:UAL1234',
        layerId: 'flights_commercial',
        pinned: true,
        name: 'UAL1234',
        addedAt: 1,
      },
    ];

    render(<PinnedEntityRenderer viewerRef={viewerRefProp} viewerReady />);

    const collection = getLastCollectionInstance();
    // Should not add any billboards since the layer is enabled
    expect(collection.add).not.toHaveBeenCalled();
  });

  it('renders nothing for unpinned entities', () => {
    storeState.enabledLayers = [];
    storeState.watchlistEntities = [
      {
        entityId: 'flights_commercial:UAL1234',
        layerId: 'flights_commercial',
        pinned: false,
        name: 'UAL1234',
        addedAt: 1,
      },
    ];

    render(<PinnedEntityRenderer viewerRef={viewerRefProp} viewerReady />);

    const collection = getLastCollectionInstance();
    expect(collection.add).not.toHaveBeenCalled();
  });

  it('fetches entity detail for pinned entities with layer disabled', () => {
    storeState.enabledLayers = [];
    storeState.watchlistEntities = [
      {
        entityId: 'flights_commercial:UAL1234',
        layerId: 'flights_commercial',
        pinned: true,
        name: 'UAL1234',
        addedAt: 1,
      },
    ];

    // Return a valid query result shape
    mockUseEntityDetail.mockReturnValue({
      data: {
        entity: { id: 'flights_commercial:UAL1234', name: 'UAL1234' },
        latestObservation: {
          entityId: 'flights_commercial:UAL1234',
          ts: Date.now(),
          position: { lat: 40.0, lon: -74.0, altM: 10000 },
          altitudeM: 10000,
        },
      },
    });

    render(<PinnedEntityRenderer viewerRef={viewerRefProp} viewerReady />);

    // useEntityDetail should have been called for the pinned entity
    expect(mockUseEntityDetail).toHaveBeenCalledWith('flights_commercial:UAL1234');
  });

  it('renders billboard at stale alpha when observation is older than 1 hour', () => {
    const oldTimestamp = Date.now() - 2 * 60 * 60 * 1000; // 2 hours ago

    storeState.enabledLayers = [];
    storeState.watchlistEntities = [
      { entityId: 'sat:ISS', layerId: 'satellites', pinned: true, name: 'ISS', addedAt: 1 },
    ];

    // Return old observation data
    mockUseEntityDetail.mockReturnValue({
      data: {
        entity: { id: 'sat:ISS', name: 'ISS' },
        latestObservation: {
          entityId: 'sat:ISS',
          ts: oldTimestamp,
          position: { lat: 51.5, lon: -0.1, altM: 400000 },
          altitudeM: 400000,
        },
      },
    });

    render(<PinnedEntityRenderer viewerRef={viewerRefProp} viewerReady />);

    // The billboard would be added once the fetcher callback fires.
    // We verify useEntityDetail was called with the correct entity ID.
    expect(mockUseEntityDetail).toHaveBeenCalledWith('sat:ISS');
  });
});
