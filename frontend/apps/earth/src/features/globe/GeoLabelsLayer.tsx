/**
 * GeoLabelsLayer — toggleable geographic labels overlay (country/city names).
 *
 * Adds a labels-only tile imagery layer on top of the base imagery when
 * enabled via the Settings panel. Uses CARTO Dark Matter Labels
 * (free, no API key required). Fades in/out over FADE_DURATION_MS.
 */

import { useEffect, useRef } from 'react';
import { type Viewer, type ImageryLayer, UrlTemplateImageryProvider, Credit } from 'cesium';
import { useUIStore } from '@/app/store';

/** Labels-only tile URL — transparent background with white text labels. */
const LABELS_TILE_URL = 'https://services.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Reference/MapServer/tile/{z}/{y}/{x}';

/** Target alpha when fully visible. */
const LABELS_ALPHA = 0.85;

/** Fade transition duration in milliseconds. */
const FADE_DURATION_MS = 400;

/** Animate an imagery layer's alpha with ease-out quad. */
function fadeAlpha(
  layer: ImageryLayer,
  from: number,
  to: number,
  rafRef: React.MutableRefObject<number>,
  viewerRef: React.MutableRefObject<Viewer | null>,
  onDone?: () => void,
) {
  cancelAnimationFrame(rafRef.current);
  const start = performance.now();

  const tick = (now: number) => {
    const t = Math.min((now - start) / FADE_DURATION_MS, 1);
    const eased = 1 - (1 - t) * (1 - t);
    layer.alpha = from + (to - from) * eased;
    viewerRef.current?.scene.requestRender();

    if (t < 1) {
      rafRef.current = requestAnimationFrame(tick);
    } else {
      onDone?.();
    }
  };

  rafRef.current = requestAnimationFrame(tick);
}

interface GeoLabelsLayerProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady: boolean;
}

export const GeoLabelsLayer: React.FC<GeoLabelsLayerProps> = ({ viewerRef, viewerReady }) => {
  const showGeoLabels = useUIStore((s) => s.showGeoLabels);
  const layerRef = useRef<ImageryLayer | null>(null);
  const rafRef = useRef<number>(0);

  // Add/remove layer with fade. No cleanup returned — removal is handled
  // explicitly in the fade-out branch to avoid cleanup killing the animation.
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewerReady || !viewer || viewer.isDestroyed()) return;

    if (showGeoLabels && !layerRef.current) {
      const provider = new UrlTemplateImageryProvider({
        url: LABELS_TILE_URL,
        credit: new Credit('Powered by Esri', false),
        maximumLevel: 16,
      });
      const layer = viewer.imageryLayers.addImageryProvider(provider);
      layer.alpha = 0;
      layerRef.current = layer;
      fadeAlpha(layer, 0, LABELS_ALPHA, rafRef, viewerRef);
    }

    if (!showGeoLabels && layerRef.current) {
      const layer = layerRef.current;
      layerRef.current = null; // clear ref before fade to prevent double-handling
      fadeAlpha(layer, layer.alpha, 0, rafRef, viewerRef, () => {
        if (!viewer.isDestroyed()) viewer.imageryLayers.remove(layer);
      });
    }
  }, [showGeoLabels, viewerReady, viewerRef]);

  // Cleanup on unmount only
  useEffect(() => {
    const raf = rafRef;
    const viewer = viewerRef;
    const layer = layerRef;
    return () => {
      cancelAnimationFrame(raf.current);
      const v = viewer.current;
      if (layer.current && v && !v.isDestroyed()) {
        v.imageryLayers.remove(layer.current);
      }
    };
  }, [viewerRef]);

  return null;
};
