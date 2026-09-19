import { describe, it, expect, beforeEach } from 'vitest';
import { create } from 'zustand';
import { createClusterSlice, type ClusterSlice, type ActiveCluster } from './createClusterSlice';

function makeStore() {
  return create<ClusterSlice>()((...args) => createClusterSlice(...args));
}

const SAMPLE: ActiveCluster = {
  clusterId: 'cluster-1:2',
  layerId: 'flights',
  entityIds: ['a', 'b', 'c'],
  count: 3,
  truncated: false,
  screenX: 120,
  screenY: 340,
};

describe('createClusterSlice', () => {
  let store: ReturnType<typeof makeStore>;

  beforeEach(() => {
    store = makeStore();
  });

  it('has correct initial state', () => {
    const s = store.getState();
    expect(s.activeCluster).toBeNull();
    expect(s.clusteredMemberIds).toEqual({});
  });

  it('setActiveCluster stores the active cluster', () => {
    store.getState().setActiveCluster(SAMPLE);
    expect(store.getState().activeCluster).toEqual(SAMPLE);
  });

  it('clearActiveCluster resets the active cluster to null', () => {
    store.getState().setActiveCluster(SAMPLE);
    store.getState().clearActiveCluster();
    expect(store.getState().activeCluster).toBeNull();
  });

  it('setClusteredMembers stores a per-layer member set', () => {
    const ids = new Set(['a', 'b']);
    store.getState().setClusteredMembers('flights', ids);
    expect(store.getState().clusteredMemberIds.flights).toBe(ids);
  });

  it('setClusteredMembers keeps layers independent', () => {
    store.getState().setClusteredMembers('flights', new Set(['a']));
    store.getState().setClusteredMembers('ships', new Set(['x', 'y']));
    expect([...store.getState().clusteredMemberIds.flights]).toEqual(['a']);
    expect([...store.getState().clusteredMemberIds.ships]).toEqual(['x', 'y']);
  });

  it('setClusteredMembers with an empty set clears that layer membership', () => {
    store.getState().setClusteredMembers('flights', new Set(['a']));
    store.getState().setClusteredMembers('flights', new Set());
    expect(store.getState().clusteredMemberIds.flights.size).toBe(0);
  });
});
