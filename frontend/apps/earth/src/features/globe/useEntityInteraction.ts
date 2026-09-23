/**
 * useEntityInteraction — attaches click, double-click, and hover handlers
 * to the Cesium viewer for entity billboard interaction.
 *
 * - Hover: changes cursor to pointer when over a billboard
 * - Single click: selects entity, plays audio cue
 * - Double click: selects entity + enters entity view mode (camera tracking)
 * - Click on empty space: clears selection
 */

import { useEffect, useCallback, useRef } from 'react';
import {
  type Viewer,
  Cartesian2,
  ScreenSpaceEventHandler,
  ScreenSpaceEventType,
  defined,
} from 'cesium';
import { useUIStore, type ViewMode } from '@/app/store';
import { useViewerStore, type CameraState } from './store';
import { playSelectSound } from '../../shared/audio/selectSound';
import { isMultiSelectModifier } from '../../shared/input';
import { isBillboardId, isClusterId, type BillboardPickId, type ClusterPickId } from './pickIds';

export function useEntityInteraction(
  viewerRef: React.MutableRefObject<Viewer | null>,
  viewerReady = false,
): void {
  const handlerRef = useRef<ScreenSpaceEventHandler | null>(null);
  const lastPointerEventRef = useRef<PointerEvent | null>(null);

  // Use refs for store actions to avoid effect re-subscriptions
  const setSelectedEntity = useUIStore((s) => s.setSelectedEntity);
  const addSelectedEntity = useUIStore((s) => s.addSelectedEntity);
  const removeSelectedEntity = useUIStore((s) => s.removeSelectedEntity);
  const setViewMode = useUIStore((s) => s.setViewMode);
  const clearSelection = useUIStore((s) => s.clearSelection);
  const setActiveCluster = useUIStore((s) => s.setActiveCluster);
  const clearActiveCluster = useUIStore((s) => s.clearActiveCluster);
  const setSavedCamera = useViewerStore((s) => s.setSavedCamera);

  const getCameraState = useCallback((): CameraState | null => {
    return useViewerStore.getState().camera;
  }, []);

  const getViewMode = useCallback((): ViewMode => {
    return useUIStore.getState().viewMode;
  }, []);

  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const handler = new ScreenSpaceEventHandler(viewer.scene.canvas);
    handlerRef.current = handler;

    // Capture latest pointer event for modifier key detection
    // (Cesium's ScreenSpaceEventHandler doesn't expose modifier keys)
    const onPointerDown = (e: PointerEvent) => {
      lastPointerEventRef.current = e;
    };
    viewer.canvas.addEventListener('pointerdown', onPointerDown);

    // Pick the id payload of a billboard or cluster marker at a screen position.
    // Both live in BillboardCollections; the discriminants route to the right handler.
    const pickPrimitive = (position: {
      x: number;
      y: number;
    }): BillboardPickId | ClusterPickId | null => {
      const cartesian = new Cartesian2(position.x, position.y);
      const picked = viewer.scene.pick(cartesian);
      const candidates = [
        defined(picked) ? picked?.primitive?.id : undefined,
        defined(picked) ? picked?.id : undefined,
      ];
      for (const id of candidates) {
        if (isClusterId(id)) return id;
        if (isBillboardId(id)) return id;
      }
      return null;
    };

    // Opens the cluster member list anchored to the click position.
    const openClusterList = (cluster: ClusterPickId, position: { x: number; y: number }) => {
      setActiveCluster({
        clusterId: cluster.clusterId,
        layerId: cluster.layerId,
        entityIds: cluster.entityIds,
        count: cluster.count,
        truncated: cluster.truncated,
        screenX: position.x,
        screenY: position.y,
      });
      playSelectSound();
    };

    // HOVER — cursor: pointer over a billboard or cluster
    handler.setInputAction((movement: { endPosition: { x: number; y: number } }) => {
      const picked = pickPrimitive(movement.endPosition);
      viewer.canvas.style.cursor = picked ? 'pointer' : 'default';
    }, ScreenSpaceEventType.MOUSE_MOVE);

    // SINGLE CLICK — cluster opens its list; entity selects (multi-select aware)
    handler.setInputAction((click: { position: { x: number; y: number } }) => {
      const picked = pickPrimitive(click.position);
      if (picked && isClusterId(picked)) {
        openClusterList(picked, click.position);
        return;
      }
      if (picked) {
        const pe = lastPointerEventRef.current;
        if (pe && isMultiSelectModifier(pe)) {
          // Multi-select: toggle entity in selection set
          const selected = useUIStore.getState().selectedEntities;
          const isAlreadySelected = selected.some((e) => e.entityId === picked.entityId);
          if (isAlreadySelected) {
            removeSelectedEntity(picked.entityId);
          } else {
            addSelectedEntity(picked.entityId, picked.layerId);
          }
        } else {
          // Plain click: replace selection (existing behavior)
          setSelectedEntity(picked.entityId, picked.layerId);
        }
        playSelectSound();
      } else {
        // Click on empty space — clear both selection and any open cluster list
        clearSelection();
        clearActiveCluster();
      }
    }, ScreenSpaceEventType.LEFT_CLICK);

    // DOUBLE CLICK — entity enters entity-view; cluster just opens its list
    handler.setInputAction((click: { position: { x: number; y: number } }) => {
      const picked = pickPrimitive(click.position);
      if (!picked) return;

      if (isClusterId(picked)) {
        openClusterList(picked, click.position);
        return;
      }

      // Save current camera before flying
      const camera = getCameraState();
      if (camera) {
        setSavedCamera(camera);
      }

      setSelectedEntity(picked.entityId, picked.layerId);
      setViewMode('entity');
      playSelectSound();
    }, ScreenSpaceEventType.LEFT_DOUBLE_CLICK);

    return () => {
      viewer.canvas.removeEventListener('pointerdown', onPointerDown);
      if (!handler.isDestroyed()) {
        handler.destroy();
      }
      handlerRef.current = null;
      // Restore cursor
      if (viewer.canvas) {
        viewer.canvas.style.cursor = 'default';
      }
    };
  }, [
    viewerRef,
    viewerReady,
    setSelectedEntity,
    addSelectedEntity,
    removeSelectedEntity,
    setViewMode,
    clearSelection,
    setActiveCluster,
    clearActiveCluster,
    setSavedCamera,
    getCameraState,
    getViewMode,
  ]);
}
