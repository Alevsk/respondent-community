import React, { useEffect, useRef, useCallback, useState } from 'react';
import { Box } from '@mui/material';
import {
  Viewer,
  Cartesian2,
  Cartesian3,
  Cartographic,
  Math as CesiumMath,
  Ion,
  ImageryLayer,
  UrlTemplateImageryProvider,
  Credit,
  Rectangle,
} from 'cesium';
import { useViewerStore, loadPersistedCamera, persistCamera } from './store';
import PostProcessOverlay from '../../shared/hud/PostProcessOverlay';
import { useUIStore, isIndicatorLayer } from '@/app/store';
import { getRuntimeConfig } from '@respondent/core';
import { useLayerStream, useViewportSync } from '../../shared/api/useLayerStream';
import { useLayers } from '../../shared/api/queries';
import { BillboardLayerRenderer } from './BillboardLayerRenderer';
import { ClusterLayerRenderer } from './ClusterLayerRenderer';
import { SelectionIndicator } from './SelectionIndicator';
import { PinnedEntityRenderer } from './PinnedEntityRenderer';
import { FindModeFilter } from './FindModeFilter';
import { EntityTrailRenderer } from './EntityTrailRenderer';
import { GeoLabelsLayer } from './GeoLabelsLayer';
import { BuildingsLayer } from './BuildingsLayer';
import { CinematicDrift } from './CinematicDrift';
import { useEntityInteraction } from './useEntityInteraction';
import { useEntityTracking } from './useEntityTracking';
import { useSearchKeyboard } from './useSearchKeyboard';
import 'cesium/Build/Cesium/Widgets/widgets.css';

// Cesium ion token: prefer the server-provided runtime config (per-deployment via
// GET /config.json, loaded before this module by main.tsx) over the build-time env,
// so the token is configurable without a rebuild. Configured at module level, before
// any Cesium Ion resources load.
const CESIUM_ION_TOKEN = getRuntimeConfig().cesiumIonToken ?? import.meta.env.VITE_CESIUM_ION_TOKEN;
if (CESIUM_ION_TOKEN) {
  Ion.defaultAccessToken = CESIUM_ION_TOKEN;
}

// When no Ion token is configured, Cesium's default Viewer tries to load the
// Cesium Ion "World Imagery" layer (asset 2 = Bing Maps Aerial) using a
// baked-in default access token that is frequently revoked — resulting in a
// cascade of 401s to `api.cesium.com/v1/assets/2/endpoint`. Fall back to
// CARTO Dark Matter tiles so the globe matches the app's dark
// theme without requiring a Cesium Ion account.
//
// IMPORTANT: This must be a factory, not a module-level constant. When React
// StrictMode double-invokes effects, the first Viewer.destroy() also destroys
// the ImageryLayer; a shared instance would be invalid on the second mount.
function createFallbackBaseLayer(): ImageryLayer | undefined {
  return new ImageryLayer(
    new UrlTemplateImageryProvider({
      url: 'https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}',
      credit: new Credit(
        'Powered by Esri — Source: Esri, Maxar, Earthstar Geographics, and the GIS User Community',
        false,
      ),
      maximumLevel: 16,
    }),
    {},
  );
}

const CESIUM_VIEWER_BASE_OPTIONS = {
  timeline: false,
  animation: false,
  geocoder: false,
  homeButton: false,
  sceneModePicker: false,
  baseLayerPicker: false,
  navigationHelpButton: false,
  fullscreenButton: false,
  vrButton: false,
  scene3DOnly: true,
  infoBox: false,
  selectionIndicator: false,
  shadows: false,
  shouldAnimate: true,
  requestRenderMode: true,
  // Safety-net: auto-render at least every 500ms of simulation time even if
  // an explicit requestRender() call is missed (e.g. React StrictMode
  // double-effect-invocation timing gap).  Most frames are still driven by
  // explicit requestRender() calls in BillboardLayerRenderer effects.
  maximumRenderTimeChange: 0.5,
  targetFrameRate: 60,
};

const GlobeScene: React.FC = () => {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewerRef = useRef<Viewer | null>(null);
  const [viewerReady, setViewerReady] = useState(false);
  const setCamera = useViewerStore((s) => s.setCamera);
  const setViewport = useViewerStore((s) => s.setViewport);
  const setViewer = useViewerStore((s) => s.setViewer);
  const activePreset = useUIStore((s) => s.activePreset);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const spatialAggregation = useUIStore((s) => s.spatialAggregation);
  const { data: layers } = useLayers();

  // Initialize WebSocket streaming for layer data
  useLayerStream();
  // Sync viewport to server for spatial layers (debounced 300ms)
  useViewportSync();
  // Entity click/hover/double-click interaction
  useEntityInteraction(viewerRef, viewerReady);
  // Camera tracking for entity view mode
  useEntityTracking(viewerRef);
  // Keyboard shortcuts for search/find mode
  useSearchKeyboard();

  // Update camera state from viewer
  const updateCameraState = useCallback(() => {
    if (!viewerRef.current) return;

    const camera = viewerRef.current.camera;
    const position = camera.positionCartographic;

    const state = {
      lat: CesiumMath.toDegrees(position.latitude),
      lon: CesiumMath.toDegrees(position.longitude),
      altitude: position.height,
      heading: CesiumMath.toDegrees(camera.heading),
      pitch: CesiumMath.toDegrees(camera.pitch),
      roll: CesiumMath.toDegrees(camera.roll),
    };
    setCamera(state);
    persistCamera(state);

    // Compute viewport bounding box from camera view rectangle
    const viewRect = camera.computeViewRectangle();
    if (viewRect) {
      const west = CesiumMath.toDegrees(viewRect.west);
      const south = CesiumMath.toDegrees(viewRect.south);
      const east = CesiumMath.toDegrees(viewRect.east);
      const north = CesiumMath.toDegrees(viewRect.north);

      // Detect full-globe viewport (high altitude / orbital zoom)
      const isFullGlobe = east - west > 350 && north - south > 170;

      if (isFullGlobe) {
        // Compute synthetic viewport from camera look-at point so that
        // viewport_update messages reflect where the user is actually looking,
        // not [-180,-90,180,90] which defeats spatial dedup.
        const lookAt = camera.pickEllipsoid(
          new Cartesian2(
            viewerRef.current!.canvas.clientWidth / 2,
            viewerRef.current!.canvas.clientHeight / 2,
          ),
        );
        if (lookAt) {
          const carto = Cartographic.fromCartesian(lookAt);
          const centerLon = CesiumMath.toDegrees(carto.longitude);
          const centerLat = CesiumMath.toDegrees(carto.latitude);
          // ~60° window centered on look-at point (covers ~6,600 km)
          const halfSpan = 30;
          setViewport({
            west: Math.max(centerLon - halfSpan, -180),
            south: Math.max(centerLat - halfSpan, -90),
            east: Math.min(centerLon + halfSpan, 180),
            north: Math.min(centerLat + halfSpan, 90),
          });
        } else {
          setViewport({ west, south, east, north });
        }
      } else {
        setViewport({ west, south, east, north });
      }
    }
  }, [setCamera, setViewport]);

  // Initialize Cesium Viewer
  useEffect(() => {
    if (!containerRef.current || viewerRef.current) return;

    const fallbackBaseLayer = createFallbackBaseLayer();
    const viewer = new Viewer(containerRef.current, {
      ...CESIUM_VIEWER_BASE_OPTIONS,
      ...(fallbackBaseLayer ? { baseLayer: fallbackBaseLayer } : {}),
    });

    viewerRef.current = viewer;
    setViewer(viewer);
    // Expose viewer for perf tooling and e2e automation (harmless in prod — read-only access).
    (window as unknown as Record<string, unknown>).__cesiumViewer = viewer;
    setViewerReady(true);

    // Restore persisted camera or use full-globe default
    const initial = loadPersistedCamera();
    viewer.camera.setView({
      destination: Cartesian3.fromDegrees(initial.lon, initial.lat, initial.altitude),
      orientation: {
        heading: CesiumMath.toRadians(initial.heading),
        pitch: CesiumMath.toRadians(initial.pitch),
        roll: CesiumMath.toRadians(initial.roll),
      },
    });

    // Listen for camera changes
    viewer.camera.changed.addEventListener(updateCameraState);
    // Set initial viewport immediately so layer subscribes have a bbox
    updateCameraState();

    // Listen for fly-to requests from other components (e.g. notification panel).
    // Supports single-point ({ lat, lon, alt }) and bounding-box ({ bounds }) modes.
    type FlyToDetail =
      | { lat: number; lon: number; alt?: number; duration?: number }
      | { bounds: { west: number; south: number; east: number; north: number }; duration?: number };
    const handleFlyTo = (e: Event) => {
      const detail = (e as CustomEvent<FlyToDetail>).detail;
      if ('bounds' in detail) {
        const { west, south, east, north } = detail.bounds;
        viewer.camera.flyTo({
          destination: Rectangle.fromDegrees(west, south, east, north),
          duration: detail.duration ?? 2.0,
        });
      } else {
        viewer.camera.flyTo({
          destination: Cartesian3.fromDegrees(detail.lon, detail.lat, detail.alt ?? 50_000),
          duration: detail.duration ?? 2.0,
        });
      }
    };
    window.addEventListener('respondent:flyto', handleFlyTo);

    // Cleanup
    return () => {
      viewer.camera.changed.removeEventListener(updateCameraState);
      window.removeEventListener('respondent:flyto', handleFlyTo);
      viewer.destroy();
      viewerRef.current = null;
      setViewer(null);
      setViewerReady(false);
    };
  }, [setViewer, updateCameraState]);

  // Render entities for each enabled layer
  return (
    <>
      {enabledLayers.map((layerId) => {
        // Skip indicator layers — they render as HUD overlays, not globe entities
        if (isIndicatorLayer(layerId)) return null;
        const layer = layers?.find((l) => l.id === layerId);
        return (
          <React.Fragment key={layerId}>
            <BillboardLayerRenderer
              layerId={layerId}
              layerType={layer?.type ?? 'unknown'}
              viewerRef={viewerRef}
              color={layer?.color}
              pointSize={layer?.pointSize}
            />
            {spatialAggregation && (
              <ClusterLayerRenderer layerId={layerId} viewerRef={viewerRef} color={layer?.color} />
            )}
          </React.Fragment>
        );
      })}

      {/* Geographic labels overlay (country/city names) */}
      <GeoLabelsLayer viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* 3D OSM Buildings (altitude-gated) */}
      <BuildingsLayer viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Entity trail polylines */}
      <EntityTrailRenderer viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Selection bracket indicator (also renders brackets for pinned entities) */}
      <SelectionIndicator viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Render billboards for pinned entities whose layers are not active */}
      <PinnedEntityRenderer viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Find Mode: control visibility/opacity of non-pinned entities */}
      <FindModeFilter viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Cinematic drift — slow heading rotation + pitch oscillation for recording */}
      <CinematicDrift viewerRef={viewerRef} viewerReady={viewerReady} />

      {/* Post-process wrapper — encapsulates all preset logic (CSS filters +
           overlay layers) so GlobeScene never needs preset-specific knowledge */}
      <PostProcessOverlay preset={activePreset} data-testid="globe-overlay">
        {/* Cesium Viewer Container */}
        <Box
          ref={containerRef}
          data-testid="globe-container"
          sx={{
            position: 'absolute',
            top: 0,
            left: 0,
            width: '100%',
            height: '100%',
            '& .cesium-viewer': {
              background: '#000000',
            },
            '& .cesium-viewer-bottom': {
              display: 'none',
            },
          }}
        />
      </PostProcessOverlay>
    </>
  );
};

export default GlobeScene;
