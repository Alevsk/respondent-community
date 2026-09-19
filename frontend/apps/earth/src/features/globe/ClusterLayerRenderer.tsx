/**
 * ClusterLayerRenderer — client-side spatial aggregation for one layer.
 *
 * A sibling of BillboardLayerRenderer, mounted only when Settings → Visibility →
 * "Spatial Aggregation" is on. On camera-idle (and on data-version change) it
 * projects the layer's loaded working set to screen space, bins nearby entities
 * into pixel cells (`screenSpaceCluster`), and renders one marker per multi-point
 * cell into its OWN BillboardCollection. It publishes the set of clustered member
 * ids so BillboardLayerRenderer can hide those individual billboards.
 *
 * Honest-by-construction: the projector excludes points that are off-screen or
 * behind the globe (via EllipsoidalOccluder), so a cluster only ever groups
 * entities the user can actually see.
 */

import { useEffect, useRef } from 'react';
import {
  type Viewer,
  type Billboard,
  BillboardCollection,
  Cartesian2,
  Cartesian3,
  Color,
  Ellipsoid,
  SceneTransforms,
} from 'cesium';
// EllipsoidalOccluder exists at runtime but is missing from Cesium's bundled .d.ts
import * as Cesium from 'cesium';
const EllipsoidalOccluder = (Cesium as Record<string, unknown>).EllipsoidalOccluder as new (
  ellipsoid: typeof Ellipsoid.WGS84,
  cameraPosition: Cartesian3,
) => {
  cameraPosition: Cartesian3;
  isPointVisible(position: Cartesian3): boolean;
};
import { useUIStore } from '@/app/store';
import { useLayerEntities } from '../../shared/api/useLayerStream';
import { screenSpaceCluster, type ScreenProjector } from './clustering';
import {
  collectClusterInputs,
  createClusterCountCanvas,
  DEFAULT_CLUSTER_COLOR,
} from './clusterRendererUtils';
import type { ClusterPickId } from './pickIds';

/** Debounce window approximating "camera has stopped moving". */
const RECOMPUTE_DEBOUNCE_MS = 250;
/** Screen cell size in CSS pixels for binning. */
const CELL_PX = 64;

export interface ClusterLayerRendererProps {
  layerId: string;
  viewerRef: React.MutableRefObject<Viewer | null>;
  /** The layer's configured color; cluster markers are tinted with it. */
  color?: string;
}

interface MarkerEntry {
  billboard: Billboard;
  count: number;
}

export const ClusterLayerRenderer: React.FC<ClusterLayerRendererProps> = ({
  layerId,
  viewerRef,
  color,
}) => {
  // `version` is the reactive data-change signal; recompute reads the rest fresh
  // from the store to avoid stale closures.
  const { version } = useLayerEntities(layerId);
  const spatialAggregation = useUIStore((s) => s.spatialAggregation);
  const setClusteredMembers = useUIStore((s) => s.setClusteredMembers);

  const collectionRef = useRef<BillboardCollection | null>(null);
  const markerMapRef = useRef<Map<string, MarkerEntry>>(new Map());
  const recomputeRef = useRef<(() => void) | null>(null);

  // Effect 1: lifecycle + camera-idle recompute + occlusion pass.
  // Re-runs only when the toggle, layer, or viewer changes (NOT on every data
  // frame), so the collection is created once and reused.
  useEffect(() => {
    if (!spatialAggregation) return;
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const markerColor = color || DEFAULT_CLUSTER_COLOR;
    const collection = new BillboardCollection();
    viewer.scene.primitives.add(collection);
    collectionRef.current = collection;
    const markerMap = markerMapRef.current;
    const occluder = new EllipsoidalOccluder(Ellipsoid.WGS84, Cartesian3.ZERO);
    const scratchWin = new Cartesian2();
    const scratchPos = new Cartesian3();

    const recompute = () => {
      const col = collectionRef.current;
      if (!col || col.isDestroyed() || viewer.isDestroyed()) return;

      const { maxEntities, layerEntities } = useUIStore.getState();
      const data = layerEntities.get(layerId);
      if (!data) {
        // No data — clear markers + membership.
        for (const [, entry] of markerMap) col.remove(entry.billboard);
        markerMap.clear();
        setClusteredMembers(layerId, new Set());
        viewer.scene.requestRender();
        return;
      }

      const points = collectClusterInputs(data.entityMap, data.obsMap, maxEntities);
      const truncated = data.entityMap.size >= maxEntities;

      occluder.cameraPosition = viewer.camera.positionWC;
      const width = viewer.canvas.clientWidth;
      const height = viewer.canvas.clientHeight;
      const project: ScreenProjector = (lat, lon) => {
        const cart = Cartesian3.fromDegrees(lon, lat, 0, Ellipsoid.WGS84, scratchPos);
        if (!occluder.isPointVisible(cart)) return null; // behind the globe
        const win = SceneTransforms.worldToWindowCoordinates(viewer.scene, cart, scratchWin);
        if (!win) return null;
        if (win.x < 0 || win.y < 0 || win.x > width || win.y > height) return null; // off-screen
        return { x: win.x, y: win.y };
      };

      const { clusters, memberIds } = screenSpaceCluster(points, project, CELL_PX);

      // Reconcile markers by clusterId.
      const live = new Set<string>();
      for (const cluster of clusters) {
        live.add(cluster.clusterId);
        const position = Cartesian3.fromDegrees(cluster.lon, cluster.lat, 0);
        const id: ClusterPickId = {
          kind: 'cluster',
          clusterId: cluster.clusterId,
          entityIds: cluster.entityIds,
          count: cluster.count,
          truncated,
          layerId,
        };
        const existing = markerMap.get(cluster.clusterId);
        if (existing) {
          existing.billboard.position = position;
          existing.billboard.id = id;
          if (existing.count !== cluster.count) {
            // Cesium's bundled .d.ts types `image` as string, but the setter
            // accepts a canvas at runtime (same as the icon canvases).
            existing.billboard.image = createClusterCountCanvas(
              cluster.count,
              markerColor,
            ) as unknown as string;
            existing.count = cluster.count;
          }
          existing.billboard.show = true;
        } else {
          const billboard = col.add({
            id,
            position,
            image: createClusterCountCanvas(cluster.count, markerColor),
            // Colors are baked into the canvas; tint WHITE so they render unmultiplied.
            color: Color.WHITE,
            disableDepthTestDistance: Number.POSITIVE_INFINITY,
          } as Billboard.ConstructorOptions);
          markerMap.set(cluster.clusterId, { billboard, count: cluster.count });
        }
      }
      // Remove markers whose cell no longer clusters.
      for (const [clusterId, entry] of markerMap) {
        if (!live.has(clusterId)) {
          col.remove(entry.billboard);
          markerMap.delete(clusterId);
        }
      }

      setClusteredMembers(layerId, memberIds);

      viewer.scene.requestRender();
      requestAnimationFrame(() => {
        if (!viewer.isDestroyed()) viewer.scene.requestRender();
      });
    };

    let timer: ReturnType<typeof setTimeout> | null = null;
    const debouncedRecompute = () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(recompute, RECOMPUTE_DEBOUNCE_MS);
    };
    recomputeRef.current = debouncedRecompute;

    // Occlusion: hide markers that rotate behind the globe between recomputes.
    const onPostRender = () => {
      const col = collectionRef.current;
      if (!col || col.isDestroyed() || markerMap.size === 0) return;
      occluder.cameraPosition = viewer.camera.positionWC;
      let changed = false;
      for (const [, entry] of markerMap) {
        const visible = occluder.isPointVisible(entry.billboard.position);
        if (entry.billboard.show !== visible) {
          entry.billboard.show = visible;
          changed = true;
        }
      }
      if (changed) viewer.scene.requestRender();
    };

    viewer.camera.changed.addEventListener(debouncedRecompute);
    viewer.scene.postRender.addEventListener(onPostRender);

    // Initial pass shortly after mount (lets the canvas settle / data arrive).
    debouncedRecompute();

    return () => {
      if (timer) clearTimeout(timer);
      recomputeRef.current = null;
      if (!viewer.isDestroyed()) {
        viewer.camera.changed.removeEventListener(debouncedRecompute);
        viewer.scene.postRender.removeEventListener(onPostRender);
        viewer.scene.primitives.remove(collection);
      }
      if (!collection.isDestroyed()) collection.destroy();
      collectionRef.current = null;
      markerMap.clear();
      // Restore individual billboards by clearing this layer's membership.
      setClusteredMembers(layerId, new Set());
      if (!viewer.isDestroyed()) viewer.scene.requestRender();
    };
  }, [spatialAggregation, layerId, viewerRef, setClusteredMembers, color]);

  // Effect 2: recompute when the layer's data changes (debounced, idle-gated).
  useEffect(() => {
    if (!spatialAggregation) return;
    recomputeRef.current?.();
  }, [version, spatialAggregation]);

  return null;
};
