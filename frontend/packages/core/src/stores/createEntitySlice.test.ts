import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createEntitySlice, type EntitySlice } from './createEntitySlice';
import { createWatchlistSlice, type WatchlistSlice } from './createWatchlistSlice';
import type { Entity, Observation } from '../models/entity';

// Combined store type for cross-slice testing
type TestStore = EntitySlice & WatchlistSlice;

const makeStore = () =>
  createStore<TestStore>()((...args) => ({
    ...createEntitySlice<TestStore>(...args),
    ...createWatchlistSlice(...(args as Parameters<typeof createWatchlistSlice>)),
  }));

const mockEntity: Entity = {
  id: 'e1',
  externalId: 'ext1',
  layerType: 'flights',
  name: 'Flight 123',
  metadata: {},
};

const mockObs: Observation = {
  entityId: 'e1',
  position: { lat: 0, lon: 0 },
  altitudeM: 10000,
  timestamp: '2024-01-01T00:00:00Z',
};

describe('createEntitySlice', () => {
  it('defaults to no selection', () => {
    const store = makeStore();
    const s = store.getState();
    expect(s.selectedEntityId).toBeNull();
    expect(s.selectedLayerId).toBeNull();
    expect(s.selectedEntities).toEqual([]);
    expect(s.viewMode).toBe('globe');
  });

  it('setSelectedEntity selects and adds to watchlist', () => {
    const store = makeStore();
    store.getState().setSelectedEntity('e1', 'flights');
    const s = store.getState();
    expect(s.selectedEntityId).toBe('e1');
    expect(s.selectedLayerId).toBe('flights');
    expect(s.selectedEntities).toEqual([{ entityId: 'e1', layerId: 'flights' }]);
    // Should add unpinned entry to watchlist
    expect(s.watchlistEntities).toHaveLength(1);
    expect(s.watchlistEntities[0].pinned).toBe(false);
  });

  it('setSelectedEntity resolves name from layerEntities', () => {
    const store = makeStore();
    // Pre-populate entity data
    store.getState().setLayerEntities('flights', {
      entities: [mockEntity],
      observations: [mockObs],
    });
    store.getState().setSelectedEntity('e1', 'flights');
    expect(store.getState().watchlistEntities[0].name).toBe('Flight 123');
  });

  it('setSelectedEntity(null) clears selection, keeps pinned', () => {
    const store = makeStore();
    store.getState().addToWatchlist({
      entityId: 'pinned1',
      layerId: 'flights',
      name: 'Pinned',
      pinned: true,
      addedAt: Date.now(),
    });
    store.getState().setSelectedEntity('e1', 'flights');
    store.getState().setSelectedEntity(null, null);
    const s = store.getState();
    expect(s.selectedEntityId).toBeNull();
    expect(s.watchlistEntities).toHaveLength(1);
    expect(s.watchlistEntities[0].entityId).toBe('pinned1');
  });

  it('setSelectedEntity keeps pinned entry if already pinned', () => {
    const store = makeStore();
    store.getState().addToWatchlist({
      entityId: 'e1',
      layerId: 'flights',
      name: 'Flight 123',
      pinned: true,
      addedAt: Date.now(),
    });
    store.getState().setSelectedEntity('e1', 'flights');
    // Should keep only pinned entries (e1 is pinned)
    expect(store.getState().watchlistEntities).toHaveLength(1);
    expect(store.getState().watchlistEntities[0].pinned).toBe(true);
  });

  it('addSelectedEntity appends to selection and watchlist', () => {
    const store = makeStore();
    store.getState().setSelectedEntity('e1', 'flights');
    store.getState().addSelectedEntity('e2', 'ships');
    const s = store.getState();
    expect(s.selectedEntities).toHaveLength(2);
    expect(s.selectedEntityId).toBe('e2');
    expect(s.watchlistEntities).toHaveLength(2);
  });

  it('addSelectedEntity is no-op if already selected', () => {
    const store = makeStore();
    store.getState().setSelectedEntity('e1', 'flights');
    store.getState().addSelectedEntity('e1', 'flights');
    expect(store.getState().selectedEntities).toHaveLength(1);
  });

  it('addSelectedEntity respects maxSelectedEntities', () => {
    const store = makeStore();
    for (let i = 0; i < 5; i++) {
      store.getState().addSelectedEntity(`e${i}`, 'flights');
    }
    store.getState().addSelectedEntity('e-extra', 'flights');
    expect(store.getState().selectedEntities).toHaveLength(5);
  });

  it('addSelectedEntity skips watchlist add if already present', () => {
    const store = makeStore();
    store.getState().addToWatchlist({
      entityId: 'e1',
      layerId: 'flights',
      name: 'Flight 123',
      pinned: true,
      addedAt: Date.now(),
    });
    store.getState().addSelectedEntity('e1', 'flights');
    expect(store.getState().watchlistEntities).toHaveLength(1);
  });

  it('selectMultipleEntities caps at maxSelectedEntities', () => {
    const store = makeStore();
    const entities = Array.from({ length: 10 }, (_, i) => ({
      entityId: `e${i}`,
      layerId: 'flights',
    }));
    store.getState().selectMultipleEntities(entities);
    expect(store.getState().selectedEntities).toHaveLength(5);
    expect(store.getState().selectedEntityId).toBe('e0');
  });

  it('selectMultipleEntities preserves pinned watchlist entries', () => {
    const store = makeStore();
    store.getState().addToWatchlist({
      entityId: 'pinned1',
      layerId: 'flights',
      name: 'Pinned',
      pinned: true,
      addedAt: Date.now(),
    });
    store.getState().selectMultipleEntities([{ entityId: 'e1', layerId: 'flights' }]);
    const wl = store.getState().watchlistEntities;
    expect(wl[0].entityId).toBe('pinned1');
    expect(wl[0].pinned).toBe(true);
  });

  it('selectMultipleEntities is no-op for empty array', () => {
    const store = makeStore();
    store.getState().setSelectedEntity('e1', 'flights');
    store.getState().selectMultipleEntities([]);
    expect(store.getState().selectedEntityId).toBe('e1');
  });

  it('removeSelectedEntity removes and cleans up view state', () => {
    const store = makeStore();
    store.getState().addSelectedEntity('e1', 'flights');
    store.getState().addSelectedEntity('e2', 'ships');
    store.getState().updateEntityViewState('e1', { activeTab: 'history' });
    store.getState().removeSelectedEntity('e1');
    const s = store.getState();
    expect(s.selectedEntities).toHaveLength(1);
    expect(s.entityViewState['e1']).toBeUndefined();
    expect(s.selectedEntityId).toBe('e2');
  });

  it('removeSelectedEntity is no-op for unknown entity', () => {
    const store = makeStore();
    store.getState().addSelectedEntity('e1', 'flights');
    store.getState().removeSelectedEntity('unknown');
    expect(store.getState().selectedEntities).toHaveLength(1);
  });

  it('clearSelection resets and removes unpinned from watchlist', () => {
    const store = makeStore();
    store.getState().addToWatchlist({
      entityId: 'pinned1',
      layerId: 'flights',
      name: 'Pinned',
      pinned: true,
      addedAt: Date.now(),
    });
    store.getState().setSelectedEntity('e1', 'flights');
    store.getState().clearSelection();
    const s = store.getState();
    expect(s.selectedEntityId).toBeNull();
    expect(s.selectedEntities).toEqual([]);
    expect(s.viewMode).toBe('globe');
    expect(s.watchlistEntities).toHaveLength(1);
    expect(s.watchlistEntities[0].entityId).toBe('pinned1');
  });

  it('setViewMode updates view mode', () => {
    const store = makeStore();
    store.getState().setViewMode('entity');
    expect(store.getState().viewMode).toBe('entity');
  });

  it('updateEntityViewState creates default then merges patch', () => {
    const store = makeStore();
    store.getState().updateEntityViewState('e1', { activeTab: 'history' });
    const vs = store.getState().entityViewState['e1'];
    expect(vs.activeTab).toBe('history');
    expect(vs.trailHighlight).toBeNull(); // from default
  });

  it('setTrailPoints stores trail for an entity', () => {
    const store = makeStore();
    const points = [{ ts: 1, lon: 0, lat: 0, altitudeM: 100 }];
    store.getState().setTrailPoints('e1', points);
    expect(store.getState().trailPoints['e1']).toEqual(points);
  });

  it('setLayerEntities upserts entities and observations', () => {
    const store = makeStore();
    store.getState().setLayerEntities('flights', {
      entities: [mockEntity],
      observations: [mockObs],
    });
    const ld = store.getState().layerEntities.get('flights')!;
    expect(ld.entityMap.get('e1')).toEqual(mockEntity);
    expect(ld.obsMap.get('e1')).toEqual(mockObs);
    expect(store.getState().layerVersions['flights']).toBe(1);
  });

  it('setLayerEntities skips stale observations', () => {
    const store = makeStore();
    const fresh: Observation = { ...mockObs, timestamp: '2024-01-02T00:00:00Z' };
    const stale: Observation = { ...mockObs, timestamp: '2024-01-01T00:00:00Z' };
    store.getState().setLayerEntities('flights', { entities: [mockEntity], observations: [fresh] });
    store.getState().setLayerEntities('flights', { entities: [], observations: [stale] });
    const ld = store.getState().layerEntities.get('flights')!;
    expect(ld.obsMap.get('e1')!.timestamp).toBe('2024-01-02T00:00:00Z');
  });

  it('clearLayerEntities removes layer data and version', () => {
    const store = makeStore();
    store
      .getState()
      .setLayerEntities('flights', { entities: [mockEntity], observations: [mockObs] });
    store.getState().clearLayerEntities('flights');
    expect(store.getState().layerEntities.has('flights')).toBe(false);
    expect(store.getState().layerVersions['flights']).toBeUndefined();
  });

  // --- Task 2: bounded merge for viewport snapshots ---

  const makeEntity = (id: string): Entity => ({
    id,
    externalId: id,
    layerType: 'flights',
    name: id,
    metadata: {},
  });
  const makeObs = (id: string, timestamp: string): Observation => ({
    entityId: id,
    position: { lat: 0, lon: 0 },
    altitudeM: 0,
    timestamp,
  });

  it('merging a second viewport preserves the first viewport entities (no wipe)', () => {
    const store = makeStore();
    // First viewport
    store.getState().setLayerEntities('flights', {
      entities: [makeEntity('a'), makeEntity('b')],
      observations: [makeObs('a', '2024-01-01T00:00:00Z'), makeObs('b', '2024-01-01T00:00:00Z')],
    });
    // Second viewport (pan) — merge, not replace
    store.getState().setLayerEntities('flights', {
      entities: [makeEntity('c'), makeEntity('d')],
      observations: [makeObs('c', '2024-01-01T00:00:00Z'), makeObs('d', '2024-01-01T00:00:00Z')],
    });
    const ld = store.getState().layerEntities.get('flights')!;
    expect([...ld.entityMap.keys()].sort()).toEqual(['a', 'b', 'c', 'd']);
    expect([...ld.obsMap.keys()].sort()).toEqual(['a', 'b', 'c', 'd']);
  });

  it('bounds the working set — exceeding the cap evicts the LEAST-recently-updated entities', () => {
    const store = makeStore();
    const cap = 3;
    // 'old1','old2' have the oldest observation timestamps → should be evicted first.
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('old1'), makeEntity('old2')],
        observations: [
          makeObs('old1', '2024-01-01T00:00:00Z'),
          makeObs('old2', '2024-01-01T00:01:00Z'),
        ],
      },
      cap,
    );
    // Add 3 newer entities → total 5 > cap 3 → evict the 2 oldest (old1, old2).
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('new1'), makeEntity('new2'), makeEntity('new3')],
        observations: [
          makeObs('new1', '2024-01-02T00:00:00Z'),
          makeObs('new2', '2024-01-02T00:01:00Z'),
          makeObs('new3', '2024-01-02T00:02:00Z'),
        ],
      },
      cap,
    );
    const ld = store.getState().layerEntities.get('flights')!;
    expect(ld.entityMap.size).toBe(cap);
    // Newest retained, oldest evicted from BOTH maps.
    expect([...ld.entityMap.keys()].sort()).toEqual(['new1', 'new2', 'new3']);
    expect(ld.entityMap.has('old1')).toBe(false);
    expect(ld.entityMap.has('old2')).toBe(false);
    expect(ld.obsMap.has('old1')).toBe(false);
    expect(ld.obsMap.has('old2')).toBe(false);
    expect(ld.dirtyEntityIds.has('old1')).toBe(false);
  });

  it('eviction sets needsRebuild=true so the renderer full-rebuild removes lingering billboards', () => {
    const store = makeStore();
    const cap = 2;
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('old1'), makeEntity('old2')],
        observations: [
          makeObs('old1', '2024-01-01T00:00:00Z'),
          makeObs('old2', '2024-01-01T00:01:00Z'),
        ],
      },
      cap,
    );
    // Add a third entity to trigger eviction
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('new1')],
        observations: [makeObs('new1', '2024-01-02T00:00:00Z')],
      },
      cap,
    );
    const ld = store.getState().layerEntities.get('flights')!;
    expect(ld.needsRebuild).toBe(true);
  });

  it('non-evicting merge does NOT force needsRebuild=true', () => {
    const store = makeStore();
    const cap = 5;
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('a')],
        observations: [makeObs('a', '2024-01-01T00:00:00Z')],
      },
      cap,
    );
    const ld = store.getState().layerEntities.get('flights')!;
    // No eviction occurred — needsRebuild must not be forced true
    expect(ld.needsRebuild).not.toBe(true);
  });

  it('eviction bumps the layer version', () => {
    const store = makeStore();
    const cap = 2;
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('a'), makeEntity('b')],
        observations: [makeObs('a', '2024-01-01T00:00:00Z'), makeObs('b', '2024-01-01T00:01:00Z')],
      },
      cap,
    );
    const versionBefore = store.getState().layerVersions['flights'];
    store.getState().setLayerEntities(
      'flights',
      {
        entities: [makeEntity('c')],
        observations: [makeObs('c', '2024-01-02T00:00:00Z')],
      },
      cap,
    );
    expect(store.getState().layerVersions['flights']).toBeGreaterThan(versionBefore);
  });

  it('replaceLayerEntities still atomically replaces (true snapshot)', () => {
    const store = makeStore();
    store.getState().setLayerEntities('flights', {
      entities: [makeEntity('a'), makeEntity('b')],
      observations: [makeObs('a', '2024-01-01T00:00:00Z'), makeObs('b', '2024-01-01T00:00:00Z')],
    });
    store.getState().replaceLayerEntities('flights', {
      entities: [makeEntity('z')],
      observations: [makeObs('z', '2024-01-02T00:00:00Z')],
    });
    const ld = store.getState().layerEntities.get('flights')!;
    expect([...ld.entityMap.keys()]).toEqual(['z']);
    expect([...ld.obsMap.keys()]).toEqual(['z']);
    expect(ld.needsRebuild).toBe(true);
  });
});
