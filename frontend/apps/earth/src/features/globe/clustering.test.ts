import { describe, it, expect } from 'vitest';
import {
  screenSpaceCluster,
  MAX_CLUSTER_IDS,
  type ClusterInput,
  type ScreenProjector,
} from './clustering';

/**
 * Builds a projector that maps each entity id to a fixed screen coordinate.
 * Ids absent from the map project to `null` (off-screen / behind the globe).
 */
function projectorFor(coords: Record<string, { x: number; y: number } | null>): ScreenProjector {
  // The real projector keys on lat/lon; tests encode the id in `lat` for lookup.
  return (lat) => {
    const entry = coords[String(lat)];
    return entry ?? null;
  };
}

/** Encodes the id into `lat` so the test projector can resolve it deterministically. */
function pt(id: string): ClusterInput {
  return { id, lat: Number(id), lon: 0 };
}

describe('screenSpaceCluster', () => {
  it('returns empty result for empty input', () => {
    const result = screenSpaceCluster([], () => ({ x: 0, y: 0 }));
    expect(result.clusters).toEqual([]);
    expect(result.individualIds).toEqual([]);
  });

  it('groups points that land in the same screen cell into one cluster', () => {
    const points = [
      { id: '1', lat: 10, lon: 20 },
      { id: '2', lat: 12, lon: 24 },
    ];
    // Both project within a 64px cell (cell 0,0).
    const project: ScreenProjector = (lat) => ({ x: lat === 10 ? 5 : 30, y: 5 });
    const { clusters, individualIds } = screenSpaceCluster(points, project, 64);
    expect(individualIds).toEqual([]);
    expect(clusters).toHaveLength(1);
    expect(clusters[0].clusterId).toBe('cluster-0:0');
    expect(clusters[0].count).toBe(2);
    expect(clusters[0].entityIds).toEqual(['1', '2']);
    // Centroid = mean of member lat/lon.
    expect(clusters[0].lat).toBeCloseTo(11);
    expect(clusters[0].lon).toBeCloseTo(22);
  });

  it('treats a lone point in a cell as an individual, not a cluster', () => {
    const project: ScreenProjector = () => ({ x: 5, y: 5 });
    const { clusters, individualIds } = screenSpaceCluster(
      [{ id: 'solo', lat: 1, lon: 2 }],
      project,
    );
    expect(clusters).toEqual([]);
    expect(individualIds).toEqual(['solo']);
  });

  it('separates points in different cells into distinct individuals', () => {
    const project: ScreenProjector = (lat) => ({ x: lat === 1 ? 5 : 500, y: 5 });
    const { clusters, individualIds } = screenSpaceCluster(
      [
        { id: 'a', lat: 1, lon: 0 },
        { id: 'b', lat: 2, lon: 0 },
      ],
      project,
      64,
    );
    expect(clusters).toEqual([]);
    expect(individualIds.sort()).toEqual(['a', 'b']);
  });

  it('drops points the projector cannot place (off-screen / behind globe)', () => {
    const coords = { '1': { x: 5, y: 5 }, '2': null };
    const { clusters, individualIds } = screenSpaceCluster(
      [pt('1'), pt('2')],
      projectorFor(coords),
    );
    expect(clusters).toEqual([]);
    expect(individualIds).toEqual(['1']);
  });

  it('caps entityIds at MAX_CLUSTER_IDS while count + memberIds reflect the true bin size', () => {
    const points: ClusterInput[] = [];
    for (let i = 0; i < MAX_CLUSTER_IDS + 50; i++) points.push({ id: `e${i}`, lat: i, lon: 0 });
    // Everything lands in cell 0,0.
    const project: ScreenProjector = () => ({ x: 1, y: 1 });
    const { clusters, memberIds } = screenSpaceCluster(points, project, 64);
    expect(clusters).toHaveLength(1);
    expect(clusters[0].count).toBe(MAX_CLUSTER_IDS + 50);
    expect(clusters[0].entityIds).toHaveLength(MAX_CLUSTER_IDS);
    // Suppression set is UNCAPPED so overflow members are hidden too.
    expect(memberIds.size).toBe(MAX_CLUSTER_IDS + 50);
  });

  it('memberIds contains every clustered id but excludes individuals', () => {
    const project: ScreenProjector = (lat) => (lat < 100 ? { x: 1, y: 1 } : { x: 999, y: 999 });
    const { memberIds, individualIds } = screenSpaceCluster(
      [
        { id: 'a', lat: 1, lon: 0 },
        { id: 'b', lat: 2, lon: 0 },
        { id: 'solo', lat: 200, lon: 0 },
      ],
      project,
      64,
    );
    expect([...memberIds].sort()).toEqual(['a', 'b']);
    expect(individualIds).toEqual(['solo']);
  });

  it('uses negative cell indices for negative screen coordinates', () => {
    const project: ScreenProjector = () => ({ x: -10, y: -10 });
    const { clusters } = screenSpaceCluster(
      [
        { id: 'a', lat: 1, lon: 0 },
        { id: 'b', lat: 2, lon: 0 },
      ],
      project,
      64,
    );
    expect(clusters[0].clusterId).toBe('cluster--1:-1');
  });
});
