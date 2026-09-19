/**
 * FindModeFilter tests
 *
 * Covers:
 * - No-op when findMode is false (no listener attached)
 * - Hidden mode: non-pinned billboards hidden, pinned shown
 * - Dimmed mode: non-pinned at 20% alpha, pinned unchanged
 * - Restores all billboards when Find Mode is deactivated
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import type { Viewer } from 'cesium';

// ---------------------------------------------------------------------------
// Hoisted test helpers
// ---------------------------------------------------------------------------
const { MockColor } = vi.hoisted(() => ({
  MockColor: class MockColorImpl {
    red: number;
    green: number;
    blue: number;
    alpha: number;
    constructor(r = 1, g = 1, b = 1, a = 1) {
      this.red = r;
      this.green = g;
      this.blue = b;
      this.alpha = a;
    }
  },
}));

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------
vi.mock('cesium', () => ({
  Color: MockColor,
}));

// Store state
type MockStoreState = {
  findMode: boolean;
  findModeDisplay: 'dimmed' | 'hidden';
  watchlistEntities: Array<{
    entityId: string;
    pinned: boolean;
    layerId: string;
    name: string;
    addedAt: number;
  }>;
  clusteredMemberIds: Record<string, Set<string>>;
};

const storeState: MockStoreState = {
  findMode: false,
  findModeDisplay: 'dimmed',
  watchlistEntities: [],
  clusteredMemberIds: {},
};

vi.mock('@/app/store', () => ({
  useUIStore: Object.assign((selector: (s: MockStoreState) => unknown) => selector(storeState), {
    getState: () => storeState,
    subscribe: vi.fn(() => vi.fn()), // returns unsub no-op
  }),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
interface MockBillboard {
  id: { entityId: string } | string;
  show: boolean;
  color: { red: number; green: number; blue: number; alpha: number };
}

function makeBillboard(entityId: string, alpha = 1.0): MockBillboard {
  return {
    id: { entityId },
    show: true,
    color: new MockColor(1, 1, 1, alpha),
  };
}

function makeBillboardCollection(billboards: MockBillboard[]) {
  return {
    get: (i: number) => billboards[i],
    get length() {
      return billboards.length;
    },
    // BillboardCollection detection
  };
}

function makeViewerRef(billboards: MockBillboard[]) {
  const preUpdateListeners: Array<() => void> = [];
  const collection = makeBillboardCollection(billboards);

  return {
    current: {
      isDestroyed: () => false,
      scene: {
        primitives: {
          length: 1,
          get: (i: number) => (i === 0 ? collection : null),
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

import { FindModeFilter } from './FindModeFilter';

describe('FindModeFilter', () => {
  beforeEach(() => {
    storeState.findMode = false;
    storeState.findModeDisplay = 'dimmed';
    storeState.watchlistEntities = [];
    storeState.clusteredMemberIds = {};
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('does not attach preUpdate listener when findMode is false', () => {
    const viewerRef = makeViewerRef([]);

    render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    expect(viewerRef.current.scene.preUpdate.addEventListener).not.toHaveBeenCalled();
  });

  it('attaches preUpdate listener when findMode is true', () => {
    storeState.findMode = true;
    storeState.watchlistEntities = [
      { entityId: 'e1', pinned: true, layerId: 'l1', name: 'E1', addedAt: 1 },
    ];

    const bb1 = makeBillboard('e1');
    const viewerRef = makeViewerRef([bb1]);

    render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    expect(viewerRef.current.scene.preUpdate.addEventListener).toHaveBeenCalledTimes(1);
  });

  it('hidden mode: hides non-pinned, shows pinned', () => {
    storeState.findMode = true;
    storeState.findModeDisplay = 'hidden';
    storeState.watchlistEntities = [
      { entityId: 'pinned1', pinned: true, layerId: 'l1', name: 'P1', addedAt: 1 },
    ];

    const pinnedBb = makeBillboard('pinned1');
    const unpinnedBb = makeBillboard('other1');
    const viewerRef = makeViewerRef([pinnedBb, unpinnedBb]);

    render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    // Execute the preUpdate callback
    const preUpdateFn = viewerRef._preUpdateListeners[0];
    expect(preUpdateFn).toBeDefined();
    preUpdateFn();

    expect(pinnedBb.show).toBe(true);
    expect(unpinnedBb.show).toBe(false);
  });

  it('dimmed mode: sets non-pinned to 20% alpha', () => {
    storeState.findMode = true;
    storeState.findModeDisplay = 'dimmed';
    storeState.watchlistEntities = [
      { entityId: 'pinned1', pinned: true, layerId: 'l1', name: 'P1', addedAt: 1 },
    ];

    const pinnedBb = makeBillboard('pinned1');
    const unpinnedBb = makeBillboard('other1');
    const viewerRef = makeViewerRef([pinnedBb, unpinnedBb]);

    render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    const preUpdateFn = viewerRef._preUpdateListeners[0];
    preUpdateFn();

    // Pinned entity keeps original alpha
    expect(pinnedBb.show).toBe(true);
    // Non-pinned should be dimmed to 0.2
    expect(unpinnedBb.show).toBe(true);
    expect(unpinnedBb.color.alpha).toBeCloseTo(0.2, 1);
  });

  it('restores all billboards when Find Mode is deactivated', () => {
    storeState.findMode = true;
    storeState.findModeDisplay = 'hidden';
    storeState.watchlistEntities = [
      { entityId: 'pinned1', pinned: true, layerId: 'l1', name: 'P1', addedAt: 1 },
    ];

    const pinnedBb = makeBillboard('pinned1');
    const unpinnedBb = makeBillboard('other1');
    unpinnedBb.show = false; // simulate hidden state
    unpinnedBb.color.alpha = 0.2; // simulate dimmed state

    const viewerRef = makeViewerRef([pinnedBb, unpinnedBb]);

    const { rerender } = render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    // Deactivate Find Mode
    storeState.findMode = false;
    rerender(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    // After deactivation, all billboards should be restored
    expect(unpinnedBb.show).toBe(true);
    expect(unpinnedBb.color.alpha).toBeCloseTo(1.0, 1);
  });

  it('excludes clustered members from find-mode dimming (cluster owns them)', () => {
    storeState.findMode = true;
    storeState.findModeDisplay = 'dimmed';
    storeState.watchlistEntities = [];
    storeState.clusteredMemberIds = { l1: new Set(['clustered1']) };

    const clusteredBb = makeBillboard('clustered1');
    clusteredBb.show = false; // hidden by cluster suppression
    const normalBb = makeBillboard('normal1');
    const viewerRef = makeViewerRef([clusteredBb, normalBb]);

    render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );
    viewerRef._preUpdateListeners[0]();

    // Clustered member is left exactly as cluster suppression set it.
    expect(clusteredBb.show).toBe(false);
    expect(clusteredBb.color.alpha).toBeCloseTo(1.0, 1);
    // A normal non-pinned entity is still dimmed by find mode.
    expect(normalBb.show).toBe(true);
    expect(normalBb.color.alpha).toBeCloseTo(0.2, 1);
  });

  it('does not restore clustered members when Find Mode is deactivated', () => {
    storeState.findMode = true;
    storeState.findModeDisplay = 'hidden';
    storeState.clusteredMemberIds = { l1: new Set(['clustered1']) };

    const clusteredBb = makeBillboard('clustered1');
    clusteredBb.show = false; // hidden by cluster suppression
    const viewerRef = makeViewerRef([clusteredBb]);

    const { rerender } = render(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    storeState.findMode = false;
    rerender(
      <FindModeFilter
        viewerRef={viewerRef as unknown as React.MutableRefObject<Viewer | null>}
        viewerReady
      />,
    );

    // Cluster suppression still owns this billboard — must stay hidden.
    expect(clusteredBb.show).toBe(false);
  });
});
