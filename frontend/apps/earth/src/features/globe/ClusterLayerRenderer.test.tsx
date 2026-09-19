/**
 * ClusterLayerRenderer tests — verify gating, idle recompute, membership
 * publication, and lifecycle cleanup against a mocked Cesium viewer.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, act } from '@testing-library/react';
import { useRef } from 'react';

const {
  MockBillboardCollectionCtor,
  MockCartesian3,
  MockCartesian2,
  getLastCollection,
  getCollectionCount,
} = vi.hoisted(() => {
  interface MarkerBB {
    id: unknown;
    position: unknown;
    image: unknown;
    color: unknown;
    show: boolean;
  }
  class MockCartesian3Impl {
    constructor(
      public x = 0,
      public y = 0,
      public z = 0,
    ) {}
    static fromDegrees = vi.fn(
      (lon: number, lat: number, alt = 0) => new MockCartesian3Impl(lon, lat, alt),
    );
    static ZERO = new MockCartesian3Impl(0, 0, 0);
  }
  class MockCartesian2Impl {
    constructor(
      public x = 0,
      public y = 0,
    ) {}
  }
  class MockCollection {
    _bbs: MarkerBB[] = [];
    _destroyed = false;
    add = vi.fn((opts: Partial<MarkerBB>) => {
      const bb: MarkerBB = {
        id: opts.id,
        position: opts.position,
        image: opts.image,
        color: opts.color,
        show: true,
      };
      this._bbs.push(bb);
      return bb;
    });
    remove = vi.fn((bb: MarkerBB) => {
      const i = this._bbs.indexOf(bb);
      if (i !== -1) this._bbs.splice(i, 1);
    });
    get = vi.fn((i: number) => this._bbs[i]);
    get length() {
      return this._bbs.length;
    }
    isDestroyed = vi.fn(() => this._destroyed);
    destroy = vi.fn(() => {
      this._destroyed = true;
    });
  }
  let last: MockCollection | null = null;
  let count = 0;
  const ctor = vi.fn(function () {
    last = new MockCollection();
    count++;
    return last;
  });
  return {
    MockBillboardCollectionCtor: ctor,
    MockCartesian3: MockCartesian3Impl,
    MockCartesian2: MockCartesian2Impl,
    getLastCollection: () => last,
    getCollectionCount: () => count,
  };
});

vi.mock('cesium', () => ({
  BillboardCollection: MockBillboardCollectionCtor,
  Cartesian2: MockCartesian2,
  Cartesian3: MockCartesian3,
  Color: { WHITE: { r: 1, g: 1, b: 1, alpha: 1 } },
  Ellipsoid: { WGS84: { _wgs84: true } },
  SceneTransforms: {
    // cart.x/y were set to lon/lat by fromDegrees → screen coords = (lon, lat).
    worldToWindowCoordinates: vi.fn((_scene: unknown, cart: { x: number; y: number }) => ({
      x: cart.x,
      y: cart.y,
    })),
  },
  EllipsoidalOccluder: class {
    cameraPosition = new MockCartesian3();
    isPointVisible = vi.fn(() => true);
  },
}));

// Spy on the canvas factory (real collectClusterInputs is kept) to assert the
// layer color is threaded into cluster markers.
vi.mock('./clusterRendererUtils', async (importActual) => {
  const actual = await importActual<typeof import('./clusterRendererUtils')>();
  return { ...actual, createClusterCountCanvas: vi.fn(() => document.createElement('canvas')) };
});

import { ClusterLayerRenderer } from './ClusterLayerRenderer';
import { createClusterCountCanvas } from './clusterRendererUtils';
import { useUIStore } from '@/app/store';
import type { Entity, Observation } from '@respondent/core';

const LAYER = 'flights';

function seedLayer(rows: Array<{ id: string; lat: number; lon: number }>) {
  const entities = rows.map((r) => ({ id: r.id, name: r.id, layerType: LAYER }) as Entity);
  const observations = rows.map(
    (r) => ({ entityId: r.id, position: { lat: r.lat, lon: r.lon } }) as unknown as Observation,
  );
  act(() => {
    useUIStore.getState().setLayerEntities(LAYER, { entities, observations });
  });
}

function Harness({ color }: { color?: string }) {
  const viewer = useRef(makeViewer());
  return <ClusterLayerRenderer layerId={LAYER} viewerRef={viewer as never} color={color} />;
}

function makeViewer() {
  const cameraChanged: Array<() => void> = [];
  const postRender: Array<() => void> = [];
  return {
    isDestroyed: () => false,
    canvas: { clientWidth: 2000, clientHeight: 2000 },
    scene: {
      primitives: { add: vi.fn(), remove: vi.fn() },
      requestRender: vi.fn(),
      postRender: {
        addEventListener: (fn: () => void) => postRender.push(fn),
        removeEventListener: vi.fn(),
      },
    },
    camera: {
      positionWC: new MockCartesian3(1000, 0, 0),
      changed: {
        addEventListener: (fn: () => void) => cameraChanged.push(fn),
        removeEventListener: vi.fn(),
      },
    },
  };
}

describe('ClusterLayerRenderer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('requestAnimationFrame', vi.fn());
    MockBillboardCollectionCtor.mockClear();
    vi.mocked(createClusterCountCanvas).mockClear();
    const s = useUIStore.getState();
    s.setSpatialAggregation(false);
    s.clearLayerEntities(LAYER);
    s.setClusteredMembers(LAYER, new Set());
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('does not create a collection when spatial aggregation is off', () => {
    useUIStore.getState().setSpatialAggregation(false);
    seedLayer([
      { id: 'a', lat: 10, lon: 20 },
      { id: 'b', lat: 10, lon: 21 },
    ]);
    render(<Harness />);
    act(() => vi.advanceTimersByTime(300));
    expect(getCollectionCount()).toBe(0);
    expect(useUIStore.getState().clusteredMemberIds[LAYER]?.size ?? 0).toBe(0);
  });

  it('clusters co-located entities and publishes member ids on idle', () => {
    useUIStore.getState().setSpatialAggregation(true);
    seedLayer([
      { id: 'a', lat: 10, lon: 20 },
      { id: 'b', lat: 10, lon: 21 }, // same 64px cell as a
      { id: 'c', lat: 10, lon: 500 }, // far → individual
    ]);
    render(<Harness color="#88cc00" />);
    act(() => vi.advanceTimersByTime(300));

    const col = getLastCollection();
    expect(col).not.toBeNull();
    // One cluster marker (a+b); c stays an individual (not rendered here).
    expect(col!.add).toHaveBeenCalledTimes(1);
    const added = col!.add.mock.calls[0][0] as {
      id: { kind: string; entityIds: string[] };
      color: { r: number; g: number; b: number; alpha: number };
    };
    expect(added.id.kind).toBe('cluster');
    expect(added.id.entityIds.sort()).toEqual(['a', 'b']);
    // Marker is tinted WHITE (colors are baked into the canvas, not transparent).
    expect(added.color).toEqual({ r: 1, g: 1, b: 1, alpha: 1 });
    // The canvas is painted with the LAYER color, not a transparent default.
    expect(createClusterCountCanvas).toHaveBeenCalledWith(2, '#88cc00');

    const members = useUIStore.getState().clusteredMemberIds[LAYER];
    expect([...members].sort()).toEqual(['a', 'b']);
  });

  it('clears membership and destroys its collection on unmount', () => {
    useUIStore.getState().setSpatialAggregation(true);
    seedLayer([
      { id: 'a', lat: 10, lon: 20 },
      { id: 'b', lat: 10, lon: 21 },
    ]);
    const { unmount } = render(<Harness />);
    act(() => vi.advanceTimersByTime(300));
    const col = getLastCollection();
    expect(useUIStore.getState().clusteredMemberIds[LAYER].size).toBe(2);

    act(() => unmount());
    expect(col!.destroy).toHaveBeenCalled();
    expect(useUIStore.getState().clusteredMemberIds[LAYER].size).toBe(0);
  });
});
