import { describe, it, expect } from 'vitest';
import { isBillboardId, isClusterId, type ClusterPickId } from './pickIds';

const CLUSTER: ClusterPickId = {
  kind: 'cluster',
  clusterId: 'cluster-1:2',
  entityIds: ['a', 'b'],
  count: 2,
  truncated: false,
  layerId: 'flights',
};

describe('pickIds discriminants', () => {
  it('isClusterId is true only for a cluster payload', () => {
    expect(isClusterId(CLUSTER)).toBe(true);
    expect(isClusterId({ entityId: 'x', layerId: 'flights' })).toBe(false);
    expect(isClusterId(null)).toBe(false);
    expect(isClusterId('cluster')).toBe(false);
    expect(isClusterId(undefined)).toBe(false);
  });

  it('isBillboardId is true for an entity payload', () => {
    expect(isBillboardId({ entityId: 'x', layerId: 'flights' })).toBe(true);
  });

  it('isBillboardId rejects a cluster payload even though it has layerId', () => {
    // Guards against the fragile implicit contract: a cluster must never be
    // mistaken for a single entity.
    expect(isBillboardId(CLUSTER)).toBe(false);
  });

  it('isBillboardId rejects non-objects and partial payloads', () => {
    expect(isBillboardId(null)).toBe(false);
    expect(isBillboardId({ entityId: 'x' })).toBe(false);
    expect(isBillboardId({ layerId: 'x' })).toBe(false);
  });
});
