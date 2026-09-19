import { useEffect, type MutableRefObject } from 'react';
import { Viewer, Math as CesiumMath } from 'cesium';
import { useUIStore } from '@/app/store';

/** Radians per second of heading rotation (~3.6°/s → full revolution in ~100s). */
const HEADING_RATE = CesiumMath.toRadians(3.6);

interface CinematicDriftProps {
  viewerRef: MutableRefObject<Viewer | null>;
  viewerReady: boolean;
}

/**
 * Slowly rotates the camera heading to produce a cinematic "orbit" effect
 * suitable for recording demo footage.
 *
 * Controlled by the `cinematicDrift` toggle in the GlobeDisplaySlice store.
 * All user input (mouse, touch, keyboard) remains active — dragging the
 * camera will work normally, and the drift resumes from wherever the user
 * leaves off.
 */
export const CinematicDrift: React.FC<CinematicDriftProps> = ({ viewerRef, viewerReady }) => {
  const enabled = useUIStore((s) => s.cinematicDrift);

  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || !viewerReady || !enabled) return;

    const onTick = () => {
      if (viewer.isDestroyed()) return;

      const dt = viewer.clock.multiplier / 60;
      viewer.camera.rotateRight(HEADING_RATE * dt);
      viewer.scene.requestRender();
    };

    viewer.clock.onTick.addEventListener(onTick);

    // Ensure continuous ticking while drift is active — override
    // requestRenderMode so the clock fires every frame.
    const wasRequestRenderMode = viewer.scene.requestRenderMode;
    viewer.scene.requestRenderMode = false;

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.clock.onTick.removeEventListener(onTick);
        viewer.scene.requestRenderMode = wasRequestRenderMode;
      }
    };
  }, [viewerRef, viewerReady, enabled]);

  return null;
};
