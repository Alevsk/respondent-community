/**
 * useEntityTracking — Camera tracking loop for entity view mode.
 *
 * When viewMode === 'entity', locks the camera onto the selected entity
 * using camera.lookAt with a Cartesian3 offset. The user can freely
 * orbit (rotate, zoom) around the entity while it moves.
 *
 * On exit (ESC key, "Exit Entity View" button, or click-on-empty),
 * unlocks the camera and restores the previously saved camera state.
 *
 * Key design choice: the tracking loop preserves the camera's local-frame
 * offset (viewer.camera.position) so user orbit adjustments are never
 * overwritten. Only the lookAt target moves.
 */

import { useEffect, useRef } from 'react';
import { type Viewer, Cartesian3, HeadingPitchRange, Matrix4, Math as CesiumMath } from 'cesium';
import { useUIStore } from '@/app/store';
import { useViewerStore } from './store';

const DEFAULT_PITCH = CesiumMath.toRadians(-45);
const MIN_RANGE = 100_000; // 100 km

function getEntityPosition(entityId: string): Cartesian3 | null {
  const { layerEntities } = useUIStore.getState();

  for (const [, layerData] of layerEntities) {
    const obs = layerData.obsMap.get(entityId);
    if (obs) {
      return Cartesian3.fromDegrees(obs.position.lon, obs.position.lat, obs.altitudeM ?? 0);
    }
  }
  return null;
}

/**
 * Unlocks camera from lookAt and optionally restores saved camera position.
 */
function exitEntityView(viewer: Viewer): void {
  if (viewer.isDestroyed()) return;

  // Unlock camera from lookAt constraint
  viewer.camera.lookAtTransform(Matrix4.IDENTITY);

  const savedCamera = useViewerStore.getState().savedCamera;
  if (savedCamera) {
    viewer.camera.flyTo({
      destination: Cartesian3.fromDegrees(savedCamera.lon, savedCamera.lat, savedCamera.altitude),
      orientation: {
        heading: CesiumMath.toRadians(savedCamera.heading),
        pitch: CesiumMath.toRadians(savedCamera.pitch),
        roll: CesiumMath.toRadians(savedCamera.roll),
      },
      duration: 1.0,
    });

    useViewerStore.getState().setSavedCamera(null);
  }
}

export function useEntityTracking(viewerRef: React.MutableRefObject<Viewer | null>): void {
  const viewMode = useUIStore((s) => s.viewMode);
  const selectedEntityId = useUIStore((s) => s.selectedEntityId);
  const clearSelection = useUIStore((s) => s.clearSelection);

  // Whether tracking loop is active (set after initial framing)
  const trackingRef = useRef(false);
  // Track previous viewMode so we can detect entity→globe transitions
  const prevViewModeRef = useRef(viewMode);

  // Effect 1: Restore camera when exiting entity view (any exit path)
  useEffect(() => {
    const wasEntity = prevViewModeRef.current === 'entity';
    prevViewModeRef.current = viewMode;

    if (wasEntity && viewMode === 'globe') {
      const viewer = viewerRef.current;
      if (viewer) {
        exitEntityView(viewer);
      }
      // Clear any tracking override from history timeline
      useViewerStore.getState().setTrackingOverridePosition(null);
    }
  }, [viewMode, viewerRef]);

  // Effect 2: Frame entity when entering entity view mode
  useEffect(() => {
    trackingRef.current = false;

    if (viewMode !== 'entity' || !selectedEntityId) return;

    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const entityPos = getEntityPosition(selectedEntityId);
    if (!entityPos) return;

    // Compute a range based on entity altitude
    let alt = 0;
    const { layerEntities } = useUIStore.getState();
    for (const [, ld] of layerEntities) {
      const obs = ld.obsMap.get(selectedEntityId);
      if (obs) {
        alt = obs.altitudeM ?? 0;
        break;
      }
    }
    const range = Math.max(alt * 0.15, MIN_RANGE);

    // Frame the entity immediately with lookAt — no flyTo animation
    // to avoid flyTo and the tracking loop fighting each other.
    viewer.camera.lookAt(entityPos, new HeadingPitchRange(0, DEFAULT_PITCH, range));

    trackingRef.current = true;
  }, [viewMode, selectedEntityId, viewerRef]);

  // Effect 3: Camera tracking loop — re-lock lookAt target each frame
  // as the entity moves. Preserves the camera's local-frame offset
  // (viewer.camera.position) so user orbit adjustments are kept intact.
  useEffect(() => {
    if (viewMode !== 'entity' || !selectedEntityId) return;

    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const onPreUpdate = () => {
      if (!trackingRef.current) return;

      // Use override position (from history timeline click) if set,
      // otherwise follow the entity's live position.
      const overridePos = useViewerStore.getState().trackingOverridePosition;
      const targetPos = overridePos ?? getEntityPosition(selectedEntityId);
      if (!targetPos) return;

      // camera.position is the offset from the lookAt target in the local
      // reference frame. Cloning and re-applying it preserves the user's
      // orbit (heading, pitch, distance) while only moving the target.
      const offset = viewer.camera.position.clone();
      viewer.camera.lookAt(targetPos, offset);
      viewer.scene.requestRender();
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
        // Unlock camera from lookAt constraint
        viewer.camera.lookAtTransform(Matrix4.IDENTITY);
      }
      trackingRef.current = false;
    };
  }, [viewerRef, viewMode, selectedEntityId]);

  // Effect 4: ESC key handler
  useEffect(() => {
    if (viewMode !== 'entity') return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        clearSelection();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [viewMode, clearSelection]);
}
