import type { SliceCreator } from '@respondent/core';

/**
 * The cluster the user clicked, anchored to the screen position of the click so
 * the desktop list can float beside it. `count` is the true bin size (which may
 * exceed `entityIds.length` when capped); `truncated` flags that the layer's
 * loaded working set was at capacity, so the cluster reflects a loaded subset.
 */
export interface ActiveCluster {
  clusterId: string;
  layerId: string;
  entityIds: string[];
  count: number;
  truncated: boolean;
  screenX: number;
  screenY: number;
}

/**
 * Cluster interaction state. Kept separate from the entity-selection slice on
 * purpose: selection caps at `maxSelectedEntities` (5) and is entangled with the
 * watchlist, which would silently truncate a 100-member cluster.
 */
export interface ClusterSlice {
  /** The cluster whose member list is open, or null. */
  activeCluster: ActiveCluster | null;
  /**
   * Per-layer set of entity ids currently absorbed into a cluster marker.
   * Published by `ClusterLayerRenderer`; consumed by `BillboardLayerRenderer`
   * (to hide members) and `FindModeFilter` (to leave them hidden).
   */
  clusteredMemberIds: Record<string, Set<string>>;
  setActiveCluster: (cluster: ActiveCluster) => void;
  clearActiveCluster: () => void;
  setClusteredMembers: (layerId: string, ids: Set<string>) => void;
}

export const createClusterSlice: SliceCreator<ClusterSlice> = (set) => ({
  activeCluster: null,
  clusteredMemberIds: {},
  setActiveCluster: (cluster) => set({ activeCluster: cluster }),
  clearActiveCluster: () => set({ activeCluster: null }),
  setClusteredMembers: (layerId, ids) =>
    set((state) => ({
      clusteredMemberIds: { ...state.clusteredMemberIds, [layerId]: ids },
    })),
});
