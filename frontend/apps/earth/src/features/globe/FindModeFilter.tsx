/**
 * FindModeFilter -- controls the visibility/opacity of non-pinned entities
 * when Find Mode is active.
 *
 * Behavior:
 *  - findMode=false: no-op (renders nothing, no listeners)
 *  - findMode=true + findModeDisplay='hidden': non-pinned billboards hidden
 *  - findMode=true + findModeDisplay='dimmed': non-pinned billboards at 20% alpha
 *
 * Uses a scene.preUpdate listener (same pattern as SelectionIndicator) to
 * iterate billboard collections each frame and adjust show/alpha.
 *
 * Performance: caches the Set of pinned entityIds to avoid repeated store reads.
 * On deactivation, restores all billboards to show=true + alpha=1.0.
 */

import { useEffect, useRef } from 'react';
import { type Viewer, Color } from 'cesium';
import { useUIStore } from '@/app/store';

/** Alpha value for dimmed non-pinned entities. */
const DIMMED_ALPHA = 0.2;

interface FindModeFilterProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady?: boolean;
}

/**
 * Extract entity ID from a billboard's id property.
 * BillboardLayerRenderer uses `{ entityId, layerId }` as the billboard id.
 */
function getBillboardEntityId(billboard: {
  id?: { entityId: string } | string | null;
}): string | null {
  const id = billboard?.id;
  if (!id) return null;
  if (typeof id === 'object' && 'entityId' in id) return id.entityId;
  if (typeof id === 'string') return id;
  return null;
}

export const FindModeFilter: React.FC<FindModeFilterProps> = ({
  viewerRef,
  viewerReady = false,
}) => {
  const findMode = useUIStore((s) => s.findMode);
  const findModeDisplay = useUIStore((s) => s.findModeDisplay);
  const prevFindModeRef = useRef(false);

  // Keep refs current for the preUpdate callback to avoid stale closures
  const findModeRef = useRef(findMode);
  findModeRef.current = findMode;
  const displayRef = useRef(findModeDisplay);
  displayRef.current = findModeDisplay;

  // Cache pinned entity IDs for O(1) lookups in the hot path.
  // Subscribe to watchlist changes so the set stays current without
  // rebuilding on every render.
  const pinnedSetRef = useRef<Set<string>>(new Set());

  // Cache the union of clustered member ids (across layers) so Find Mode leaves
  // entities absorbed into a cluster marker hidden, rather than fighting
  // ClusterLayerRenderer over `bb.show`.
  const clusteredSetRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    // Initialize from current state
    const buildSet = () => {
      const entities = useUIStore.getState().watchlistEntities;
      pinnedSetRef.current = new Set(entities.filter((e) => e.pinned).map((e) => e.entityId));
    };
    const buildClusteredSet = () => {
      const byLayer = useUIStore.getState().clusteredMemberIds;
      const union = new Set<string>();
      for (const layerId in byLayer) {
        for (const id of byLayer[layerId]) union.add(id);
      }
      clusteredSetRef.current = union;
    };
    buildSet();
    buildClusteredSet();

    // Re-build when watchlist or cluster membership changes
    const unsub = useUIStore.subscribe((state, prev) => {
      if (state.watchlistEntities !== prev.watchlistEntities) {
        buildSet();
      }
      if (state.clusteredMemberIds !== prev.clusteredMemberIds) {
        buildClusteredSet();
      }
    });
    return unsub;
  }, []);

  // Restore all billboards when Find Mode is deactivated
  useEffect(() => {
    if (prevFindModeRef.current && !findMode) {
      // Find mode just deactivated -- restore all billboards
      const viewer = viewerRef.current;
      if (viewer && !viewer.isDestroyed()) {
        restoreAllBillboards(viewer, clusteredSetRef.current);
        viewer.scene.requestRender();
      }
    }
    prevFindModeRef.current = findMode;
  }, [findMode, viewerRef]);

  // Attach/detach scene.preUpdate listener when Find Mode is active
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed() || !findMode) return;

    const onPreUpdate = () => {
      if (!findModeRef.current) return;

      const pinnedIds = pinnedSetRef.current;
      const clusteredIds = clusteredSetRef.current;
      const display = displayRef.current;
      const primitives = viewer.scene.primitives;

      for (let p = 0; p < primitives.length; p++) {
        const primitive = primitives.get(p);
        // Only process BillboardCollections
        if (
          !primitive ||
          typeof primitive.get !== 'function' ||
          typeof primitive.length !== 'number'
        )
          continue;

        const count = primitive.length;
        for (let i = 0; i < count; i++) {
          const bb = primitive.get(i);
          if (!bb) continue;

          const entityId = getBillboardEntityId(bb);
          if (!entityId) continue;
          // Clustered members are owned by the cluster suppression — leave hidden.
          if (clusteredIds.has(entityId)) continue;

          const isPinned = pinnedIds.has(entityId);

          if (display === 'hidden') {
            bb.show = isPinned;
          } else {
            // dimmed mode
            bb.show = true;
            if (!isPinned) {
              const c = bb.color;
              if (!c) continue;
              if (Math.abs(c.alpha - DIMMED_ALPHA) > 0.01) {
                bb.color = new Color(c.red, c.green, c.blue, DIMMED_ALPHA);
              }
            }
            // Pinned entities keep their current alpha (set by their renderer)
          }
        }
      }
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
      }
    };
  }, [viewerRef, viewerReady, findMode]);

  return null;
};

/**
 * Restore all billboard show/alpha to defaults.
 * Called when Find Mode is deactivated.
 *
 * Note: this resets stale-indicator alphas (0.5) set by PinnedEntityRenderer.
 * The next preUpdate frame from PinnedEntityRenderer will re-apply the stale
 * opacity, so the visual glitch is at most one frame.
 *
 * `clusteredIds` are left untouched so cluster suppression survives Find Mode
 * deactivation (otherwise clustered members would flash back until the next
 * cluster recompute).
 */
function restoreAllBillboards(viewer: Viewer, clusteredIds: Set<string>): void {
  const primitives = viewer.scene.primitives;

  for (let p = 0; p < primitives.length; p++) {
    const primitive = primitives.get(p);
    if (!primitive || typeof primitive.get !== 'function' || typeof primitive.length !== 'number')
      continue;

    const count = primitive.length;
    for (let i = 0; i < count; i++) {
      const bb = primitive.get(i);
      if (!bb) continue;

      const entityId = getBillboardEntityId(bb);
      if (entityId && clusteredIds.has(entityId)) continue;

      bb.show = true;
      const c = bb.color;
      if (c && c.alpha < 1.0) {
        bb.color = new Color(c.red, c.green, c.blue, 1.0);
      }
    }
  }
}
