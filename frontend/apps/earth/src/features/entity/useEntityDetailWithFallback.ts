/**
 * useEntityDetailWithFallback — returns entity detail from the REST API,
 * falling back to in-memory WebSocket store data when the API hasn't
 * persisted the entity yet (e.g., on-demand fetches pending async Postgres
 * persist).
 *
 * Shared between EntityDetailPanel (desktop) and MobileEntityPanel.
 */

import { useMemo, useCallback } from 'react';
import { useUIStore } from '@/app/store';
import { useEntityDetail, type EntityDetailResponse } from '../../shared/api/queries';

export function useEntityDetailWithFallback(entityId: string, layerId: string) {
  const { data: apiDetail, isLoading } = useEntityDetail(entityId);

  // Subscribe to version changes for this layer so fallback data stays fresh
  const layerVersion = useUIStore(useCallback((s) => s.layerVersions[layerId] ?? 0, [layerId]));

  const storeDetail = useMemo((): EntityDetailResponse | undefined => {
    if (apiDetail?.entity) return undefined; // API has data — no fallback needed
    const layerData = useUIStore.getState().layerEntities.get(layerId);
    if (!layerData) return undefined;
    const entity = layerData.entityMap.get(entityId);
    if (!entity) return undefined;
    const obs = layerData.obsMap.get(entityId);
    const result: EntityDetailResponse = {
      entity: {
        id: entity.id,
        externalId: entity.externalId,
        layerType: entity.layerType,
        name: entity.name,
        metadata: entity.metadata,
        // Pass through parsed AI metadata from WS store as JSON string
        // to match the REST API shape (aiMetadataJson)
        ...(entity.aiMetadata && Object.keys(entity.aiMetadata).length > 0
          ? { aiMetadataJson: JSON.stringify(entity.aiMetadata) }
          : {}),
      },
    };
    if (obs) {
      result.latestObservation = {
        entityId: obs.entityId,
        ts: obs.timestamp ? new Date(obs.timestamp).getTime() : Date.now(),
        position: { lat: obs.position.lat, lon: obs.position.lon, altM: obs.altitudeM },
        altitudeM: obs.altitudeM,
        velocity: obs.velocity,
        metadata: obs.metadata,
      };
    }
    return result;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [apiDetail, entityId, layerId, layerVersion]);

  const detail = apiDetail?.entity ? apiDetail : storeDetail;

  return { detail, isLoading };
}
