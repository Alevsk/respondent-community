/**
 * EntityTrailRenderer — renders historical trail polylines on the globe
 * for all selected entities.
 *
 * Each entity gets a polyline drawn from its oldest → newest observation.
 * A highlight point primitive marks the currently selected timeline event.
 *
 * Uses PolylineCollection for batched GPU rendering.
 */

import { useEffect, useRef, useMemo } from 'react';
import {
  type Viewer,
  Cartesian3,
  Color,
  Polyline,
  PolylineCollection,
  PointPrimitive,
  PointPrimitiveCollection,
  Material,
} from 'cesium';
import { useUIStore } from '@/app/store';
import type { TrailPoint } from '@respondent/core';
import { theme } from '@respondent/core';
import { interpolatedPositions } from './interpolatedPositions';
import { splitTrailSegments, segmentOpacity, interpolateTrailGeographic } from './trailUtils';

const DEFAULT_TRAIL_COLOR = theme.palette.primary.main;

/** Stable empty array to avoid re-renders when no trail points exist. */
const EMPTY_POINTS: TrailPoint[] = [];

const DEFAULT_TRAIL_WIDTH = 1.5;
const DEFAULT_TRAIL_OPACITY = 0.7;
const LEADER_ALPHA = 0.4;
const HIGHLIGHT_POINT_SIZE = 10;
const HIGHLIGHT_COLOR = '#ffffff';

interface TrailStyle {
  color: Color;
  width: number;
}

/**
 * Resolves trail rendering style for a layer type.
 * Reads trail config from the layer's displayConfig (populated by the declarative
 * source YAML via the GetLayers API). Layers without a trail config use the
 * generic defaults — no layer-name fallback.
 *
 * NOTE: Reads a snapshot via getState() (not reactive). This is intentional —
 * layer metadata is loaded once at app startup and is stable for the lifetime
 * of the session, so a reactive subscription would add overhead with no benefit.
 */
function getTrailStyle(layerType: string): TrailStyle {
  const layer = useUIStore.getState().layers[layerType];
  const trailConfig = layer?.displayConfig?.trail;

  if (trailConfig) {
    const hex = trailConfig.color || DEFAULT_TRAIL_COLOR;
    const opacity = trailConfig.opacity > 0 ? trailConfig.opacity : DEFAULT_TRAIL_OPACITY;
    const width = trailConfig.width > 0 ? trailConfig.width : DEFAULT_TRAIL_WIDTH;
    return {
      color: Color.fromCssColorString(hex).withAlpha(opacity),
      width,
    };
  }

  return {
    color: Color.fromCssColorString(DEFAULT_TRAIL_COLOR).withAlpha(DEFAULT_TRAIL_OPACITY),
    width: DEFAULT_TRAIL_WIDTH,
  };
}

/** Converts trail points to smoothed Cartesian3 positions via geographic-space Catmull-Rom. */
function trailPointsToCartesian3(points: TrailPoint[]): Cartesian3[] {
  const geo = interpolateTrailGeographic(points);
  return geo.map((p) => Cartesian3.fromDegrees(p.lon, p.lat, p.alt));
}

interface EntityTrailRendererProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady: boolean;
}

/**
 * Renders a single entity's trail polyline and highlight point.
 * Reads trail points from the store (written by useEntityTrail in HistoryTab).
 */
const SingleEntityTrail: React.FC<{
  entityId: string;
  layerType: string;
  polylineCollection: PolylineCollection;
  pointCollection: PointPrimitiveCollection;
  viewerRef: React.MutableRefObject<Viewer | null>;
}> = ({ entityId, layerType, polylineCollection, pointCollection, viewerRef }) => {
  // Trail visibility is driven by the store value written when History tab mounts.
  // Value is absent (undefined) until the user visits History tab → trails hidden.
  // Once set, it persists across tab switches until panel close clears it.
  const showTrails = useUIStore((s) => s.entityViewState[entityId]?.showTrails);
  const trailVisible = showTrails === true;
  // Read trail points from the store — written by the useEntityTrail hook in HistoryTab.
  // This ensures Load More in the timeline immediately extends the rendered polyline.
  const points = useUIStore((s) => s.trailPoints[entityId] ?? EMPTY_POINTS);
  const highlightTs = useUIStore((s) => s.entityViewState[entityId]?.trailHighlight ?? null);

  const polylinesRef = useRef<Polyline[]>([]);
  const leaderRef = useRef<Polyline | null>(null);
  const pointRef = useRef<PointPrimitive | null>(null);
  // Cache the last trail point position for O(1) leader line updates
  const lastTrailPosRef = useRef<Cartesian3 | null>(null);

  // Update polylines when points change or history tab visibility changes
  useEffect(() => {
    if (polylineCollection.isDestroyed()) return;

    // Remove previous polylines for this entity
    for (const pl of polylinesRef.current) {
      polylineCollection.remove(pl);
    }
    polylinesRef.current = [];

    if (!trailVisible || points.length < 2) {
      lastTrailPosRef.current = null;
      return;
    }

    const trailStyle = getTrailStyle(layerType);

    // Split into segments at time gaps (separate flight legs / multi-day data)
    const segments = splitTrailSegments(points);

    for (let i = 0; i < segments.length; i++) {
      const segment = segments[i];
      if (segment.length < 2) continue;
      const positions = trailPointsToCartesian3(segment);
      // Recent segment gets full opacity; historical segments are faded
      const alpha = segmentOpacity(trailStyle.color.alpha, i, segments.length);
      const segmentColor = trailStyle.color.withAlpha(alpha);
      const pl = polylineCollection.add({
        positions,
        width: trailStyle.width,
        material: Material.fromType('Color', { color: segmentColor }),
      });
      polylinesRef.current.push(pl);
    }

    // Cache the last trail point for the leader line
    const lastPt = points[points.length - 1];
    lastTrailPosRef.current = Cartesian3.fromDegrees(lastPt.lon, lastPt.lat, lastPt.altitudeM);

    const viewer = viewerRef.current;
    viewer?.scene.requestRender();

    return () => {
      if (!polylineCollection.isDestroyed()) {
        for (const pl of polylinesRef.current) {
          polylineCollection.remove(pl);
        }
        polylinesRef.current = [];
        viewer?.scene.requestRender();
      }
    };
  }, [points, layerType, polylineCollection, trailVisible, viewerRef]);

  // Leader line: connects last trail point to entity's current interpolated position.
  // Created once when trail becomes visible; updated every frame via preUpdate.
  useEffect(() => {
    if (polylineCollection.isDestroyed()) return;

    // Remove previous leader
    if (leaderRef.current) {
      polylineCollection.remove(leaderRef.current);
      leaderRef.current = null;
    }

    if (!trailVisible || points.length < 1) return;

    const lastPt = points[points.length - 1];
    const lastPos = Cartesian3.fromDegrees(lastPt.lon, lastPt.lat, lastPt.altitudeM);
    const trailStyle = getTrailStyle(layerType);
    const leaderColor = trailStyle.color.withAlpha(LEADER_ALPHA);

    // Initialize with zero-length segment; preUpdate will extend it
    leaderRef.current = polylineCollection.add({
      positions: [lastPos, lastPos],
      width: trailStyle.width,
      material: Material.fromType('Color', { color: leaderColor }),
    });

    return () => {
      if (!polylineCollection.isDestroyed() && leaderRef.current) {
        polylineCollection.remove(leaderRef.current);
        leaderRef.current = null;
      }
    };
  }, [points, layerType, polylineCollection, trailVisible]);

  // Update leader line end-point every frame to track interpolated billboard position
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed() || !trailVisible) return;

    const onPreUpdate = () => {
      const leader = leaderRef.current;
      const lastTrailPos = lastTrailPosRef.current;
      if (!leader || !lastTrailPos) return;

      const entityPos = interpolatedPositions.get(entityId);
      if (!entityPos) return;

      // Update leader line: [lastTrailPoint → current billboard position]
      leader.positions = [lastTrailPos, entityPos];
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
      }
    };
  }, [viewerRef, entityId, trailVisible]);

  // Update highlight point when selection changes
  useEffect(() => {
    if (pointCollection.isDestroyed()) return;

    // Remove previous highlight
    if (pointRef.current) {
      pointCollection.remove(pointRef.current);
      pointRef.current = null;
    }

    if (!trailVisible || highlightTs === null) return;

    // Find point by timestamp (stable across Load More pagination)
    const point = points.find((p) => p.ts === highlightTs);
    if (!point) return;

    const viewer = viewerRef.current;
    pointRef.current = pointCollection.add({
      position: Cartesian3.fromDegrees(point.lon, point.lat, point.altitudeM),
      pixelSize: HIGHLIGHT_POINT_SIZE,
      color: Color.fromCssColorString(HIGHLIGHT_COLOR),
      outlineColor: Color.fromCssColorString(DEFAULT_TRAIL_COLOR),
      outlineWidth: 2,
      disableDepthTestDistance: Number.POSITIVE_INFINITY,
    });
    viewer?.scene.requestRender();

    return () => {
      if (!pointCollection.isDestroyed() && pointRef.current) {
        pointCollection.remove(pointRef.current);
        pointRef.current = null;
        viewer?.scene.requestRender();
      }
    };
  }, [highlightTs, points, pointCollection, trailVisible, viewerRef]);

  return null;
};

export const EntityTrailRenderer: React.FC<EntityTrailRendererProps> = ({
  viewerRef,
  viewerReady,
}) => {
  const selectedEntities = useUIStore((s) => s.selectedEntities);
  const polylineCollectionRef = useRef<PolylineCollection | null>(null);
  const pointCollectionRef = useRef<PointPrimitiveCollection | null>(null);

  // Create/destroy primitive collections
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const polylines = new PolylineCollection();
    const points = new PointPrimitiveCollection();
    viewer.scene.primitives.add(polylines);
    viewer.scene.primitives.add(points);
    polylineCollectionRef.current = polylines;
    pointCollectionRef.current = points;

    return () => {
      polylineCollectionRef.current = null;
      pointCollectionRef.current = null;
      if (!viewer.isDestroyed()) {
        viewer.scene.primitives.remove(polylines);
        viewer.scene.primitives.remove(points);
      }
      if (!polylines.isDestroyed()) polylines.destroy();
      if (!points.isDestroyed()) points.destroy();
    };
  }, [viewerRef, viewerReady]);

  // Resolve layer type for each selected entity (memoized, reads state directly)
  const entityLayerTypes = useMemo(() => {
    const result: Record<string, string> = {};
    const layerEntities = useUIStore.getState().layerEntities;
    for (const sel of selectedEntities) {
      for (const [, layerData] of layerEntities) {
        const entity = layerData.entityMap.get(sel.entityId);
        if (entity) {
          result[sel.entityId] = entity.layerType;
          break;
        }
      }
      if (!result[sel.entityId]) {
        result[sel.entityId] = sel.layerId;
      }
    }
    return result;
  }, [selectedEntities]);

  if (!polylineCollectionRef.current || !pointCollectionRef.current) return null;

  return (
    <>
      {selectedEntities.map((sel) => (
        <SingleEntityTrail
          key={sel.entityId}
          entityId={sel.entityId}
          layerType={entityLayerTypes[sel.entityId] ?? 'unknown'}
          polylineCollection={polylineCollectionRef.current!}
          pointCollection={pointCollectionRef.current!}
          viewerRef={viewerRef}
        />
      ))}
    </>
  );
};
