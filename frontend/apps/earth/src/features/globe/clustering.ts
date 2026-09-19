/**
 * clustering.ts — pure, dependency-free spatial aggregation for the globe.
 *
 * A faithful screen-space adaptation of the original platform's pixel-grid
 * clustering (`pkg/cluster/grid.go`). Instead of synthesizing a Mercator zoom
 * level — which is unstable on a tilted 3D globe — each entity is projected to
 * window coordinates by an injected `ScreenProjector` and binned into fixed
 * pixel cells. Points the projector cannot place (off-screen or behind the
 * globe) are excluded, so clusters honestly reflect what is actually visible.
 *
 * The module is intentionally Cesium-free: the renderer supplies a projector
 * built from the live `Viewer`, while tests supply a deterministic stub.
 */

/** Maximum member ids retained per cluster. `count` still carries the true size. */
export const MAX_CLUSTER_IDS = 100;

/** Default screen cell size in CSS pixels. */
export const DEFAULT_CELL_PX = 64;

export interface ClusterInput {
  id: string;
  lat: number;
  lon: number;
}

export interface Cluster {
  /** Deterministic id derived from the cell indices: `cluster-<cx>:<cy>`. */
  clusterId: string;
  /** Mean latitude of all members in the cell (uses the full bin, not the capped ids). */
  lat: number;
  /** Mean longitude of all members in the cell. */
  lon: number;
  /** True number of members in the cell (may exceed `entityIds.length`). */
  count: number;
  /** Member ids, capped at {@link MAX_CLUSTER_IDS}. */
  entityIds: string[];
}

export interface ClusterResult {
  clusters: Cluster[];
  /** Ids of cells containing exactly one point — rendered as normal billboards. */
  individualIds: string[];
  /**
   * Every entity id absorbed into a cluster, UNCAPPED. `Cluster.entityIds` is
   * capped at {@link MAX_CLUSTER_IDS} for the popup list, but suppression must
   * hide *all* members or overflow points would render under the marker.
   */
  memberIds: Set<string>;
}

/** Projects a geographic coordinate to screen pixels, or `null` if not visible. */
export type ScreenProjector = (lat: number, lon: number) => { x: number; y: number } | null;

interface Cell {
  ids: string[];
  latSum: number;
  lonSum: number;
  count: number;
}

/**
 * Bins visible points into pixel cells and returns clusters (cells with >1 point)
 * and individuals (cells with exactly 1). O(N) over the input — one projection
 * pass, no spatial index.
 */
export function screenSpaceCluster(
  points: ClusterInput[],
  project: ScreenProjector,
  cellPx: number = DEFAULT_CELL_PX,
): ClusterResult {
  const cells = new Map<string, Cell>();

  for (const p of points) {
    const win = project(p.lat, p.lon);
    if (!win) continue; // off-screen or occluded by the globe

    const cx = Math.floor(win.x / cellPx);
    const cy = Math.floor(win.y / cellPx);
    const key = `${cx}:${cy}`;

    let cell = cells.get(key);
    if (!cell) {
      cell = { ids: [], latSum: 0, lonSum: 0, count: 0 };
      cells.set(key, cell);
    }
    cell.count++;
    cell.latSum += p.lat;
    cell.lonSum += p.lon;
    cell.ids.push(p.id);
  }

  const clusters: Cluster[] = [];
  const individualIds: string[] = [];
  const memberIds = new Set<string>();

  for (const [key, cell] of cells) {
    if (cell.count === 1) {
      individualIds.push(cell.ids[0]);
      continue;
    }
    for (const id of cell.ids) memberIds.add(id);
    clusters.push({
      clusterId: `cluster-${key}`,
      lat: cell.latSum / cell.count,
      lon: cell.lonSum / cell.count,
      count: cell.count,
      entityIds: cell.ids.slice(0, MAX_CLUSTER_IDS),
    });
  }

  return { clusters, individualIds, memberIds };
}
