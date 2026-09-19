/**
 * BuildingsLayer — toggleable 3D OSM Buildings overlay.
 *
 * Loads Cesium OSM Buildings (Ion asset 96188) as a 3D Tileset when enabled
 * via the Settings panel. Visibility is altitude-gated with hysteresis:
 * buildings show below 5 km and hide above 6 km to prevent flickering.
 *
 * Altitude gating uses a Cesium camera event listener (not React state)
 * to avoid React re-renders on every camera move.
 *
 * Uses Cesium Ion's default token; set VITE_CESIUM_ION_TOKEN for custom quota.
 */

import { useEffect, useRef } from 'react';
import {
  type Viewer,
  Cesium3DTileset,
  createWorldTerrainAsync,
  EllipsoidTerrainProvider,
} from 'cesium';
import { useUIStore } from '@/app/store';

/** Cesium Ion asset ID for OSM Buildings. */
const BUILDINGS_ION_ASSET_ID = 96188;

/** Show buildings when camera drops below this altitude (meters). */
const ALTITUDE_SHOW_THRESHOLD = 5_000;

/** Hide buildings when camera rises above this altitude (meters). */
const ALTITUDE_HIDE_THRESHOLD = 6_000;

/** LOD quality — lower = more detail, higher GPU cost. */
const MAXIMUM_SCREEN_SPACE_ERROR = 16;

interface BuildingsLayerProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady: boolean;
}

export const BuildingsLayer: React.FC<BuildingsLayerProps> = ({ viewerRef, viewerReady }) => {
  const show3DBuildings = useUIStore((s) => s.show3DBuildings);
  const tilesetRef = useRef<Cesium3DTileset | null>(null);
  const terrainSetRef = useRef(false);

  // Load/unload the tileset and terrain based on toggle
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewerReady || !viewer || viewer.isDestroyed()) return;

    if (show3DBuildings && !tilesetRef.current) {
      // Enable world terrain for proper building ground placement
      if (!terrainSetRef.current) {
        createWorldTerrainAsync().then((terrain) => {
          if (!viewer.isDestroyed()) {
            viewer.scene.terrainProvider = terrain;
            terrainSetRef.current = true;
            viewer.scene.requestRender();
          }
        });
      }

      // Load OSM Buildings tileset
      let cancelled = false;
      Cesium3DTileset.fromIonAssetId(BUILDINGS_ION_ASSET_ID, {
        maximumScreenSpaceError: MAXIMUM_SCREEN_SPACE_ERROR,
      }).then((tileset) => {
        if (cancelled || viewer.isDestroyed()) {
          tileset.destroy();
          return;
        }
        tileset.show = false; // Start hidden — camera listener will set visibility
        viewer.scene.primitives.add(tileset);
        tilesetRef.current = tileset;
        viewer.scene.requestRender();
      });

      return () => {
        cancelled = true;
      };
    }

    if (!show3DBuildings && tilesetRef.current) {
      if (!viewer.isDestroyed()) {
        viewer.scene.primitives.remove(tilesetRef.current);
        // Revert to default ellipsoid terrain
        if (terrainSetRef.current) {
          viewer.scene.terrainProvider = new EllipsoidTerrainProvider();
          terrainSetRef.current = false;
        }
      }
      if (!tilesetRef.current.isDestroyed()) {
        tilesetRef.current.destroy();
      }
      tilesetRef.current = null;
    }
  }, [show3DBuildings, viewerReady, viewerRef]);

  // Altitude-gated visibility via Cesium camera event (no React re-renders).
  // Uses hysteresis: show < 5km, hide > 6km to prevent flicker at boundary.
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewerReady || !viewer || viewer.isDestroyed() || !show3DBuildings) return;

    const onCameraChange = () => {
      if (!tilesetRef.current) return;
      const altitude = viewer.camera.positionCartographic.height;
      const wasVisible = tilesetRef.current.show;

      if (altitude < ALTITUDE_SHOW_THRESHOLD) {
        tilesetRef.current.show = true;
      } else if (altitude > ALTITUDE_HIDE_THRESHOLD) {
        tilesetRef.current.show = false;
      }
      // Between thresholds: maintain previous state (hysteresis)

      if (tilesetRef.current.show !== wasVisible) {
        viewer.scene.requestRender();
      }
    };

    viewer.camera.changed.addEventListener(onCameraChange);
    onCameraChange(); // Set initial state

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.camera.changed.removeEventListener(onCameraChange);
      }
    };
  }, [show3DBuildings, viewerReady, viewerRef]);

  // Cleanup on unmount only
  useEffect(() => {
    const viewer = viewerRef.current;
    return () => {
      if (tilesetRef.current) {
        if (viewer && !viewer.isDestroyed()) {
          viewer.scene.primitives.remove(tilesetRef.current);
        }
        if (!tilesetRef.current.isDestroyed()) {
          tilesetRef.current.destroy();
        }
        tilesetRef.current = null;
      }
    };
  }, [viewerRef]);

  return null;
};
