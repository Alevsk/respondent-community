/**
 * PinnedEntityRenderer -- renders billboards on the globe for watchlist entities
 * (pinned OR selected) whose layer is NOT currently active/enabled. When an
 * entity's layer IS active, the normal BillboardLayerRenderer handles it.
 *
 * This covers two scenarios:
 *  1. Pinned entities persisted across sessions (e.g. ISS)
 *  2. Entities selected from notifications without enabling the full layer
 *
 * Uses the same Cesium primitive patterns as SelectionIndicator:
 *  - BillboardCollection added to viewer.scene.primitives
 *  - scene.preUpdate listener for real-time position tracking
 *  - proper cleanup on unmount
 *
 * Stale indicator: entities with last observation older than 1 hour render
 * at 50% opacity.
 */

import { useEffect, useRef, useMemo, useCallback, useState } from 'react';
import { type Viewer, type Billboard, BillboardCollection, Cartesian3, Color } from 'cesium';
import { useUIStore } from '@/app/store';
import { useEntityDetail } from '../../shared/api/queries';
import { getIconCanvas, getIconScale } from './icons/iconRegistry';
import { interpolatedPositions } from './interpolatedPositions';

/** One hour in milliseconds -- threshold for stale indicator. */
const STALE_THRESHOLD_MS = 60 * 60 * 1000;

interface PinnedEntityRendererProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady?: boolean;
}

/**
 * Fetches entity detail for a single pinned entity whose layer is off.
 * Renders nothing -- purely a data-fetching component.
 */
const PinnedEntityFetcher: React.FC<{
  entityId: string;
  onData: (entityId: string, lat: number, lon: number, alt: number, ts: number) => void;
}> = ({ entityId, onData }) => {
  const { data } = useEntityDetail(entityId);

  useEffect(() => {
    if (!data?.latestObservation) return;
    const obs = data.latestObservation;
    onData(entityId, obs.position.lat, obs.position.lon, obs.altitudeM ?? 0, obs.ts);
  }, [data, entityId, onData]);

  return null;
};

interface PinnedPosition {
  lat: number;
  lon: number;
  alt: number;
  ts: number;
}

export const PinnedEntityRenderer: React.FC<PinnedEntityRendererProps> = ({
  viewerRef,
  viewerReady = false,
}) => {
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const layers = useUIStore((s) => s.layers);

  // Filter to watchlist entities (pinned or selected) whose layer is NOT enabled
  const pinnedOffLayer = useMemo(() => {
    const enabledSet = new Set(enabledLayers);
    return watchlistEntities.filter((e) => !enabledSet.has(e.layerId));
  }, [watchlistEntities, enabledLayers]);

  // Build a stable color map string for pinned-off-layer entities so the
  // billboard rebuild effect re-runs when layer metadata (colors) arrives.
  const pinnedColorKey = useMemo(
    () => pinnedOffLayer.map((e) => `${e.layerId}:${layers[e.layerId]?.color ?? ''}`).join(','),
    [pinnedOffLayer, layers],
  );

  const collectionRef = useRef<BillboardCollection | null>(null);
  // Cache of fetched positions keyed by entityId
  const positionCacheRef = useRef<Map<string, PinnedPosition>>(new Map());
  // Version counter to trigger billboard rebuild when new position data arrives
  const [positionVersion, setPositionVersion] = useState(0);
  // Track which billboards are in the collection by entityId
  const billboardMapRef = useRef<Map<string, Billboard>>(new Map());

  // Create/destroy the BillboardCollection
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const collection = new BillboardCollection();
    viewer.scene.primitives.add(collection);
    collectionRef.current = collection;
    const billboardMap = billboardMapRef.current;

    return () => {
      collectionRef.current = null;
      billboardMap.clear();
      if (!viewer.isDestroyed()) {
        viewer.scene.primitives.remove(collection);
      }
      if (!collection.isDestroyed()) {
        collection.destroy();
      }
    };
  }, [viewerRef, viewerReady]);

  // Handle fetched position data -- update cache and bump version to trigger rebuild
  const handlePositionData = useCallback(
    (entityId: string, lat: number, lon: number, alt: number, ts: number) => {
      positionCacheRef.current.set(entityId, { lat, lon, alt, ts });
      setPositionVersion((v) => v + 1);
    },
    [],
  );

  // Rebuild billboards when pinned-off-layer list changes or positions update
  useEffect(() => {
    const collection = collectionRef.current;
    if (!collection || collection.isDestroyed()) return;

    collection.removeAll();
    billboardMapRef.current.clear();

    const now = Date.now();

    for (const entity of pinnedOffLayer) {
      const cached = positionCacheRef.current.get(entity.entityId);
      if (!cached) continue;

      const position = Cartesian3.fromDegrees(cached.lon, cached.lat, cached.alt);
      const isStale = now - cached.ts > STALE_THRESHOLD_MS;
      const alpha = isStale ? 0.5 : 1.0;

      // Use the reactively-subscribed layers (line 73) to resolve the configured
      // color. The pinnedColorKey dependency ensures this effect re-runs when
      // layer metadata arrives from the API.
      const layerColor = layers[entity.layerId]?.color;

      const iconCanvas = getIconCanvas(entity.layerId, layerColor);
      const scale = getIconScale(entity.layerId);

      const bb = collection.add({
        id: { entityId: entity.entityId, layerId: entity.layerId },
        position,
        image: iconCanvas,
        color: Color.WHITE.withAlpha(alpha),
        scale,
        disableDepthTestDistance: Number.POSITIVE_INFINITY,
      });

      billboardMapRef.current.set(entity.entityId, bb);
    }

    viewerRef.current?.scene.requestRender();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pinnedOffLayer, viewerRef, positionVersion, pinnedColorKey]);

  // Track positions every frame -- update from layerEntities if available
  // (entity may have cached data even if its layer was recently disabled)
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed() || pinnedOffLayer.length === 0) return;

    const onPreUpdate = () => {
      const collection = collectionRef.current;
      if (!collection || collection.isDestroyed() || collection.length === 0) return;

      let moved = false;

      for (let i = 0; i < collection.length; i++) {
        const bb = collection.get(i);
        const bbId = bb.id as { entityId: string } | undefined;
        if (!bbId?.entityId) continue;

        // Check interpolated positions first (from BillboardLayerRenderer)
        let pos = interpolatedPositions.get(bbId.entityId);

        // Fallback: check store's layerEntities
        if (!pos) {
          const entities = useUIStore.getState().layerEntities;
          for (const [, layerData] of entities) {
            const obs = layerData.obsMap.get(bbId.entityId);
            if (obs) {
              pos = Cartesian3.fromDegrees(obs.position.lon, obs.position.lat, obs.altitudeM ?? 0);
              break;
            }
          }
        }

        if (!pos) continue;

        bb.position = pos;
        moved = true;
      }

      if (moved) {
        viewer.scene.requestRender();
      }
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
      }
    };
  }, [viewerRef, pinnedOffLayer]);

  return (
    <>
      {pinnedOffLayer.map((entity) => (
        <PinnedEntityFetcher
          key={entity.entityId}
          entityId={entity.entityId}
          onData={handlePositionData}
        />
      ))}
    </>
  );
};
