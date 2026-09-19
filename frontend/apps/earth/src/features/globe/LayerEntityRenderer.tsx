import React, { useEffect, useRef } from 'react';
import { Viewer, Cartesian3, Color, ConstantPositionProperty } from 'cesium';
import { useLayerEntities } from '../../shared/api/useLayerStream';
import { getEntityColor, getEntitySize } from './entityUtils';

export interface LayerEntityRendererProps {
  layerId: string;
  viewerRef: React.MutableRefObject<Viewer | null>;
  color?: string;
  pointSize?: number;
}

export const LayerEntityRenderer: React.FC<LayerEntityRendererProps> = ({
  layerId,
  viewerRef,
  color,
  pointSize,
}) => {
  const { entityMap, obsMap, hasData, version } = useLayerEntities(layerId);
  const addedIds = useRef<Set<string>>(new Set());

  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || !hasData || !entityMap || !obsMap) return;

    viewer.entities.suspendEvents();

    for (const [entityId, rawEntity] of entityMap) {
      const rawObs = obsMap.get(entityId);
      if (!rawObs) continue;

      const pos = rawObs.position;
      if (!pos) continue;
      const lon = Number(pos.lon);
      const lat = Number(pos.lat);
      if (isNaN(lon) || isNaN(lat)) continue;
      const alt = Number(rawObs.altitudeM ?? 0);

      const cartesianPos = Cartesian3.fromDegrees(lon, lat, alt);
      const existingEntity = viewer.entities.getById(entityId);

      if (!existingEntity) {
        viewer.entities.add({
          id: entityId,
          name: rawEntity.name || 'Unknown',
          show: true,
          position: new ConstantPositionProperty(cartesianPos),
          point: {
            show: true,
            pixelSize: getEntitySize(rawEntity.layerType || 'unknown', pointSize),
            color: getEntityColor(rawEntity.layerType || 'unknown', color),
            outlineColor: Color.WHITE,
            outlineWidth: 1,
            disableDepthTestDistance: Number.POSITIVE_INFINITY,
          },
        });
        addedIds.current.add(entityId);
      } else {
        existingEntity.position = new ConstantPositionProperty(cartesianPos);
      }
    }

    viewer.entities.resumeEvents();
  }, [version, hasData, layerId, color, entityMap, obsMap, pointSize, viewerRef]);

  useEffect(() => {
    const viewer = viewerRef;
    const ids = addedIds;
    return () => {
      const v = viewer.current;
      if (v) {
        for (const id of ids.current) {
          v.entities.removeById(id);
        }
        ids.current.clear();
      }
    };
  }, [viewerRef]);

  return null;
};
