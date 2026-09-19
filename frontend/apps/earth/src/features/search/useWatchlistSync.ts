/**
 * useWatchlistSync — ensures watchlist entities always have observation data
 * available in layerEntities.obsMap, even when outside the viewport query.
 *
 * Problem: The WebSocket viewport stream only sends entities within the
 * camera's visible area. Pinned watchlist entities may be on the other side
 * of the globe (or stale), so they're missing from obsMap. This breaks
 * SelectionIndicator brackets and fly-to positioning.
 *
 * Solution: Calls the batch entity endpoint (POST /v1/entities/batch) to
 * fetch latest observations for all watchlist entities and injects them
 * into layerEntities.obsMap. Re-injects after each viewport clear to
 * survive the WebSocket stream's clearLayerEntities cycle.
 */

import { useEffect, useRef } from 'react';
import { useUIStore } from '@/app/store';
import type { Entity, Observation } from '@/app/store';
import { api, endpoints } from '@respondent/core';

/** Cached batch response for re-injection after viewport clears. */
interface CachedEntityData {
  entity: Entity;
  observation: Observation | null;
  layerId: string;
}

/**
 * Fetch latest observations for a batch of entity IDs via the backend
 * batch endpoint (POST /v1/entities/batch).
 */
async function fetchBatchObservations(entityIds: string[]): Promise<CachedEntityData[]> {
  if (entityIds.length === 0) return [];

  try {
    const response = await api.post<{
      entities?: Array<{
        entity?: {
          id: string;
          externalId: string;
          layerType: string;
          name: string;
          metadata: Record<string, string>;
        };
        latestObservation?: {
          entityId: string;
          ts: number;
          position: { lat: number; lon: number; altM?: number };
          altitudeM: number;
          velocity?: Record<string, number>;
        };
      }>;
    }>(`${endpoints.entities}/batch`, { entityIds });

    const results: CachedEntityData[] = [];
    for (const item of response.entities ?? []) {
      if (!item.entity) continue;

      const compositeId = `${item.entity.layerType}:${item.entity.externalId}`;
      const entity: Entity = {
        id: compositeId,
        externalId: item.entity.externalId,
        layerType: item.entity.layerType,
        name: item.entity.name,
        metadata: item.entity.metadata ?? {},
        source: 'stale',
      };

      let observation: Observation | null = null;
      if (item.latestObservation?.position) {
        observation = {
          entityId: compositeId,
          position: {
            lat: item.latestObservation.position.lat,
            lon: item.latestObservation.position.lon,
          },
          altitudeM: item.latestObservation.altitudeM ?? item.latestObservation.position.altM ?? 0,
          timestamp: item.latestObservation.ts
            ? new Date(Number(item.latestObservation.ts)).toISOString()
            : new Date().toISOString(),
          velocity: item.latestObservation.velocity,
          source: 'stale',
        };
      }

      results.push({ entity, observation, layerId: item.entity.layerType });
    }
    return results;
  } catch (err) {
    console.error('[useWatchlistSync] batch fetch failed:', err);
    return [];
  }
}

/**
 * Inject cached entity/observation data into the store's layerEntities.
 * Groups by layerId for efficient batch upserts.
 */
function injectIntoStore(data: CachedEntityData[]): void {
  const byLayer = new Map<string, { entities: Entity[]; observations: Observation[] }>();

  for (const item of data) {
    let bucket = byLayer.get(item.layerId);
    if (!bucket) {
      bucket = { entities: [], observations: [] };
      byLayer.set(item.layerId, bucket);
    }
    bucket.entities.push(item.entity);
    if (item.observation) {
      bucket.observations.push(item.observation);
    }
  }

  const { setLayerEntities } = useUIStore.getState();
  for (const [layerId, payload] of byLayer) {
    setLayerEntities(layerId, payload);
  }
}

/**
 * Hook that keeps watchlist entity observations synced in the store.
 * Mount this alongside the WatchlistBar component.
 */
export function useWatchlistSync(): void {
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const cacheRef = useRef<CachedEntityData[]>([]);
  const syncingRef = useRef(false);

  // Fetch batch data when watchlist changes.
  // Track the fetched entity IDs to detect if the watchlist changed during an
  // in-flight request, and re-trigger if needed.
  const pendingIdsRef = useRef<string | null>(null);

  useEffect(() => {
    if (watchlistEntities.length === 0) {
      cacheRef.current = [];
      return;
    }

    const entityIds = watchlistEntities.map((e) => e.entityId);
    const idsKey = entityIds.join(',');

    // Skip if already fetching the same set of IDs
    if (syncingRef.current) {
      pendingIdsRef.current = idsKey;
      return;
    }
    syncingRef.current = true;
    pendingIdsRef.current = null;

    fetchBatchObservations(entityIds)
      .then((data) => {
        cacheRef.current = data;
        injectIntoStore(data);
      })
      .finally(() => {
        syncingRef.current = false;
        // If watchlist changed while we were fetching, re-trigger
        if (pendingIdsRef.current !== null && pendingIdsRef.current !== idsKey) {
          const nextIds = pendingIdsRef.current.split(',');
          pendingIdsRef.current = null;
          fetchBatchObservations(nextIds).then((data) => {
            cacheRef.current = data;
            injectIntoStore(data);
          });
        }
      });
  }, [watchlistEntities]);

  // Re-inject cached data whenever layerVersions change (viewport clear + rebuild).
  // This ensures watchlist observation data survives clearLayerEntities calls.
  // NOTE: Always subscribe — the cache is populated async by the fetch effect above,
  // so checking cacheRef.current.length in the effect body would miss re-injection
  // when the subscription is needed (cache fills after mount).
  useEffect(() => {
    const unsub = useUIStore.subscribe((state, prev) => {
      if (state.layerVersions !== prev.layerVersions && cacheRef.current.length > 0) {
        // Check if any watchlist entities are missing from obsMap
        const { layerEntities } = state;
        let needsReinjection = false;

        for (const cached of cacheRef.current) {
          if (!cached.observation) continue;
          const layerData = layerEntities.get(cached.layerId);
          if (!layerData || !layerData.obsMap.has(cached.observation.entityId)) {
            needsReinjection = true;
            break;
          }
        }

        if (needsReinjection) {
          // Defer to next microtask to avoid recursive set() during subscribe
          queueMicrotask(() => injectIntoStore(cacheRef.current));
        }
      }
    });

    return unsub;
  }, []);
}
