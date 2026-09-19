/**
 * pickIds.ts — the Cesium billboard `id` payload contract and its discriminants.
 *
 * Both individual entity billboards and cluster markers live in
 * `BillboardCollection`s and surface through `scene.pick()`. These guards route
 * a pick to the right handler. The cluster payload carries an explicit
 * `kind: 'cluster'` discriminant and deliberately omits `entityId`, so a cluster
 * can never be mistaken for a single entity even if the shapes drift.
 */

export interface BillboardPickId {
  entityId: string;
  layerId: string;
}

export interface ClusterPickId {
  kind: 'cluster';
  clusterId: string;
  entityIds: string[];
  /** True bin size (may exceed `entityIds.length` when capped). */
  count: number;
  /** The layer's loaded working set was at capacity → cluster reflects a subset. */
  truncated: boolean;
  layerId: string;
}

export function isClusterId(value: unknown): value is ClusterPickId {
  return (
    typeof value === 'object' && value !== null && (value as { kind?: unknown }).kind === 'cluster'
  );
}

export function isBillboardId(value: unknown): value is BillboardPickId {
  return (
    typeof value === 'object' &&
    value !== null &&
    !isClusterId(value) &&
    'entityId' in value &&
    'layerId' in value
  );
}
