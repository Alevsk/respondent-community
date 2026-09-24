// React hook for streaming layer data via WebSocket

import { useEffect, useRef, useCallback, useMemo } from 'react';
import { wsClient, type WSMessage, type LayerSnapshot } from '@respondent/core';
import { useUIStore, Entity, Observation, IndicatorSnapshot, DataSource } from '@/app/store';
import { useViewerStore } from '../../features/globe/store';

/**
 * Returns true if the given layer uses viewport-based progressive loading.
 * Derived dynamically from the backend's filtering_mode field — no hardcoded list.
 */
function isSpatialLayer(layerId: string): boolean {
  const layer = useUIStore.getState().layers[layerId];
  return layer?.filteringMode === 'viewport';
}

/**
 * Per-layer disposition for the NEXT inbound snapshot.
 *
 * The server replies to `subscribe`, `viewport_update`, and `time_range` with
 * an identical `layer.snapshot` message — it cannot tell the client which one
 * to merge vs replace. Only the client knows what it last requested, so the
 * client owns the decision:
 *   - `subscribe` / `time_range` → 'replace' (true snapshot: initial load or a
 *     changed time window invalidates the prior working set → atomic wipe).
 *   - `viewport_update` (pan) → 'merge' (preserve the prior viewport's entities,
 *     bounded by LRU eviction in the store).
 *
 * Module-level so `useViewportSync` (which sends viewport_update) and
 * `useLayerStream` (which subscribes and consumes snapshots) share one source
 * of truth.
 *
 * The explicit disposition is one-shot (consumed per snapshot). When none is set,
 * the DEFAULT depends on the layer's loading model, because the server pushes an
 * ONGOING stream of snapshots per viewport (geo cache → backfill → on-demand
 * re-poll ticker), not a single reply:
 *   - viewport-progressive layers default to 'merge' so those follow-up snapshots
 *     accumulate (bounded by LRU) instead of each one wiping the prior viewports.
 *   - load-everything layers default to 'replace' — each snapshot is the full
 *     authoritative set.
 * The first snapshot after a subscribe / time_range still REPLACES because those
 * paths set 'replace' explicitly; subsequent snapshots fall through to the default.
 */
type SnapshotDisposition = 'replace' | 'merge';
const snapshotDisposition = new Map<string, SnapshotDisposition>();

/** Set how the next inbound snapshot for a layer should be applied. */
export function setLayerSnapshotDisposition(
  layerId: string,
  disposition: SnapshotDisposition,
): void {
  snapshotDisposition.set(layerId, disposition);
}

/**
 * Read the disposition for a layer's next snapshot. An explicit value (set by a
 * subscribe / viewport_update / time_range) wins; otherwise the default is
 * layer-aware so the server's follow-up snapshots accumulate for viewport-
 * progressive layers and replace for load-everything layers.
 */
function takeLayerSnapshotDisposition(layerId: string): SnapshotDisposition {
  const explicit = snapshotDisposition.get(layerId);
  if (explicit) return explicit;
  return isSpatialLayer(layerId) ? 'merge' : 'replace';
}

// --- Wire format → store normalization ---
// The Go backend sends snake_case JSON (domain structs with json tags).
// The frontend store uses camelCase TypeScript interfaces.
// These functions are the single boundary where the conversion happens.

interface RawEntity {
  id?: string;
  external_id?: string;
  externalId?: string;
  layer_type?: string;
  layerType?: string;
  name?: string;
  metadata?: Record<string, string>;
  ai_metadata?: Record<string, unknown>;
  aiMetadataJson?: string;
  ai_metadata_json?: string;
  source?: string;
}

interface RawObservation {
  entity_id?: string;
  entityId?: string;
  position?: { lat?: number; lon?: number; alt?: number };
  altitude_m?: number;
  altitudeM?: number;
  timestamp?: string;
  velocity?: Record<string, number>;
  metadata?: Record<string, string>;
  source?: string;
}

function normalizeEntity(raw: RawEntity): Entity {
  // Parse AI metadata from either WS path (object) or REST path (JSON string)
  let aiMetadata: Record<string, unknown> | undefined;
  if (
    raw.ai_metadata &&
    typeof raw.ai_metadata === 'object' &&
    Object.keys(raw.ai_metadata).length > 0
  ) {
    aiMetadata = raw.ai_metadata;
  } else {
    // `||` (not `??`) so an empty string falls through to the sibling field
    // and to `undefined` if both are absent — JSON.parse requires a non-empty string.
    const rawJson = raw.aiMetadataJson || raw.ai_metadata_json;
    if (rawJson) {
      try {
        const parsed = JSON.parse(rawJson);
        if (parsed && typeof parsed === 'object' && Object.keys(parsed).length > 0) {
          aiMetadata = parsed;
        }
      } catch {
        /* ignore malformed JSON */
      }
    }
  }
  return {
    id: raw.id ?? '',
    externalId: raw.external_id ?? raw.externalId ?? '',
    layerType: raw.layer_type ?? raw.layerType ?? '',
    name: raw.name ?? '',
    metadata: raw.metadata ?? {},
    aiMetadata,
    source: toDataSource(raw.source),
  };
}

// Wire data is untrusted — narrow the loose `string` source field to the
// discriminated `DataSource` union used by the store. Unknown values fall
// back to 'live' so the renderer treats them as current.
function toDataSource(s: string | undefined): DataSource {
  return s === 'stale' || s === 'historical' ? s : 'live';
}

function normalizeObservation(raw: RawObservation): Observation {
  const pos = raw.position ?? {};
  return {
    entityId: raw.entity_id ?? raw.entityId ?? '',
    position: { lat: pos.lat ?? 0, lon: pos.lon ?? 0 },
    altitudeM: raw.altitude_m ?? raw.altitudeM ?? pos.alt ?? 0,
    timestamp: raw.timestamp ?? '',
    velocity: raw.velocity,
    metadata: raw.metadata,
    source: toDataSource(raw.source),
  };
}

function normalizeEntities(raw: RawEntity[]): Entity[] {
  return raw.map(normalizeEntity);
}

function normalizeObservations(raw: RawObservation[]): Observation[] {
  return raw.map(normalizeObservation);
}

interface LayerUpdatePayload {
  entities: RawEntity[];
  observations: RawObservation[];
}

/**
 * Hook to manage WebSocket subscriptions for layer data streaming.
 * Bulk messages (snapshots) are coalesced into a per-layer rAF flush.
 * Per-entity `layer.update` messages are batched per animation frame
 * to avoid hundreds of store writes + re-renders per second.
 */
export function useLayerStream() {
  // Use granular selectors to avoid re-rendering on every store change.
  // Action references are stable in Zustand, so selecting them individually
  // prevents this component from subscribing to the entire store.
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const setLayerEntities = useUIStore((s) => s.setLayerEntities);
  const clearLayerEntities = useUIStore((s) => s.clearLayerEntities);
  const replaceLayerEntities = useUIStore((s) => s.replaceLayerEntities);
  const setIndicator = useUIStore((s) => s.setIndicator);
  // Seeded with the already-enabled layers so the diff effect does NOT treat
  // them as "new" on mount (which would clear them). The dedicated mount effect
  // below re-subscribes them WITHOUT clearing — entities persist across
  // earth↔dashboard remounts.
  const prevEnabledLayersRef = useRef<Set<string>>(new Set(useUIStore.getState().enabledLayers));

  // Batch buffer for per-entity updates (legacy layer.update path), keyed by layerId
  const batchRef = useRef<Map<string, { entities: Entity[]; observations: Observation[] }>>(
    new Map(),
  );

  // Bulk buffer for snapshot / layer.snapshot / layer.batch_update messages.
  // Design choice: sibling buffer (not extending batchRef) so flushBatch for
  // single-entity updates stays clean. Both buffers share the same rafRef so
  // only one rAF is ever pending at a time, and both are drained in one frame.
  // isSnapshot=true on any chunk for a layer causes replaceLayerEntities (atomic
  // replace) on flush; otherwise setLayerEntities (merge) is used.
  const bulkBatchRef = useRef<
    Map<string, { entities: Entity[]; observations: Observation[]; isSnapshot: boolean }>
  >(new Map());

  const rafRef = useRef<number>(0);

  const flushBatch = useCallback(() => {
    rafRef.current = 0;

    // Cap the bounded per-layer working set so panning (merge) can't grow
    // memory unboundedly — store evicts LRU past this bound.
    const maxWorkingSet = useUIStore.getState().maxEntities;

    // Drain legacy per-entity buffer (layer.update path)
    const batch = batchRef.current;
    if (batch.size > 0) {
      for (const [layerId, data] of batch) {
        setLayerEntities(layerId, data, maxWorkingSet);
      }
      batch.clear();
    }

    // Drain bulk buffer (snapshot / layer.snapshot / layer.batch_update).
    // If any chunk arriving within the same frame was a snapshot, we perform
    // an atomic replace for that layer — a partial snapshot must never render.
    const bulkBatch = bulkBatchRef.current;
    if (bulkBatch.size > 0) {
      for (const [layerId, data] of bulkBatch) {
        if (data.isSnapshot) {
          replaceLayerEntities(layerId, {
            entities: data.entities,
            observations: data.observations,
          });
        } else {
          setLayerEntities(
            layerId,
            { entities: data.entities, observations: data.observations },
            maxWorkingSet,
          );
        }
      }
      bulkBatch.clear();
    }
  }, [setLayerEntities, replaceLayerEntities]);

  const scheduleBatch = useCallback(
    (layerId: string, entity: Entity, observation: Observation | null) => {
      let entry = batchRef.current.get(layerId);
      if (!entry) {
        entry = { entities: [], observations: [] };
        batchRef.current.set(layerId, entry);
      }
      entry.entities.push(entity);
      if (observation) entry.observations.push(observation);

      if (!rafRef.current) {
        rafRef.current = requestAnimationFrame(flushBatch);
      }
    },
    [flushBatch],
  );

  // scheduleBulk accumulates bulk chunks (snapshot / layer.snapshot / layer.batch_update)
  // into the per-layer bulkBatchRef. On the next animation frame, ONE store write is
  // performed per layer: replaceLayerEntities if any chunk was a snapshot, else setLayerEntities.
  const scheduleBulk = useCallback(
    (
      layerId: string,
      data: { entities: Entity[]; observations: Observation[] },
      isSnapshot: boolean,
    ) => {
      let entry = bulkBatchRef.current.get(layerId);
      if (!entry) {
        entry = { entities: [], observations: [], isSnapshot: false };
        bulkBatchRef.current.set(layerId, entry);
      }
      entry.entities.push(...data.entities);
      entry.observations.push(...data.observations);
      // Once any chunk marks this layer as a snapshot, the flag is latched —
      // a partial snapshot must never be rendered as a merge.
      if (isSnapshot) entry.isSnapshot = true;

      if (!rafRef.current) {
        rafRef.current = requestAnimationFrame(flushBatch);
      }
    },
    [flushBatch],
  );

  const handleIndicatorMessage = useCallback(
    (message: WSMessage) => {
      if (message.type === 'indicator.update' && message.layer_id) {
        const data = message.data as {
          timestamp_ms?: number;
          timestampMs?: number;
          values?: Array<{
            key: string;
            value: string;
            label: string;
            unit?: string;
            level: number;
            change_pct?: number;
            changePct?: number;
            change_abs?: number;
            changeAbs?: number;
            format?: string;
            precision?: number;
            prefix?: string;
          }>;
          summary?: string;
          overall_level?: number;
          overallLevel?: number;
        };
        const layerId = message.layer_id;
        const layers = useUIStore.getState().layers;
        const layer = layers[layerId];
        const snapshot: IndicatorSnapshot = {
          layerId,
          layerName: layer?.name ?? layerId,
          timestampMs: data.timestampMs ?? data.timestamp_ms ?? Date.now(),
          values: (data.values ?? []).map((v) => ({
            key: v.key,
            value: v.value,
            label: v.label,
            unit: v.unit ?? '',
            level: v.level,
            changePct: v.change_pct ?? v.changePct ?? 0,
            changeAbs: v.change_abs ?? v.changeAbs ?? 0,
            format: v.format ?? 'scale',
            precision: v.precision ?? 0,
            prefix: v.prefix ?? '',
          })),
          summary: data.summary ?? '',
          overallLevel: data.overallLevel ?? data.overall_level ?? 0,
        };
        setIndicator(layerId, snapshot);
      }
    },
    [setIndicator],
  );

  const handleMessage = useCallback(
    (message: WSMessage) => {
      if (message.type === 'layer_update' || message.type === 'snapshot') {
        // Bulk payload — route through rAF bulk buffer.
        // snapshot → atomic replace; layer_update → merge.
        const payload = message.data as LayerUpdatePayload;
        if (message.layer_id && payload.entities) {
          scheduleBulk(
            message.layer_id,
            {
              entities: normalizeEntities(payload.entities),
              observations: normalizeObservations(payload.observations || []),
            },
            message.type === 'snapshot',
          );
        }
      } else if (message.type === 'layer.snapshot') {
        // Snapshot disposition is client-owned: a viewport-pan snapshot MERGES
        // (preserve the prior working set), a true snapshot (subscribe /
        // time_range) REPLACES atomically. Consume the flag so subsequent
        // snapshots default back to replace until another viewport_update.
        const payload = message.data as LayerSnapshot;
        if (message.layer_id) {
          const replace = takeLayerSnapshotDisposition(message.layer_id) === 'replace';
          snapshotDisposition.delete(message.layer_id);
          scheduleBulk(
            message.layer_id,
            {
              entities: normalizeEntities(payload.entities || []),
              observations: normalizeObservations(payload.observations || []),
            },
            replace /* isSnapshot → atomic replace */,
          );
        }
      } else if (message.type === 'layer.batch_update') {
        // Batched Pub/Sub updates — server accumulates updates and sends them as one message.
        // timeMode==='range' guard: frozen playback must not receive live updates.
        const { timeMode, timeTo } = useUIStore.getState();
        const isFrozen = timeMode === 'range' && timeTo !== null;
        if (isFrozen) return;

        const payload = message.data as {
          updates: Array<{ entity?: RawEntity; observation?: RawObservation }>;
        };
        if (message.layer_id && payload.updates?.length) {
          const entities: Entity[] = [];
          const observations: Observation[] = [];
          for (const update of payload.updates) {
            if (update.entity) {
              entities.push(normalizeEntity(update.entity));
            }
            if (update.observation) {
              observations.push(normalizeObservation(update.observation));
            }
          }
          if (entities.length > 0) {
            scheduleBulk(message.layer_id, { entities, observations }, false /* isSnapshot */);
          }
        }
      } else if (message.type === 'layer.update') {
        // Legacy single-entity update (kept for backward compatibility).
        const { timeMode, timeTo } = useUIStore.getState();
        const isFrozen = timeMode === 'range' && timeTo !== null;
        if (isFrozen) return;

        // Per-entity update — batch into next animation frame
        const payload = message.data as { entity: RawEntity; observation?: RawObservation };
        if (message.layer_id && payload.entity) {
          scheduleBatch(
            message.layer_id,
            normalizeEntity(payload.entity),
            payload.observation ? normalizeObservation(payload.observation) : null,
          );
        }
      }
    },
    [scheduleBulk, scheduleBatch],
  );

  useEffect(() => {
    // Connect WebSocket if not connected
    if (!wsClient.isConnected()) {
      wsClient.connect();
    }

    // Subscribe to messages
    const unsubUpdate = wsClient.subscribe('layer_update', handleMessage);
    const unsubSnapshot = wsClient.subscribe('snapshot', handleMessage);
    const unsubLayerSnapshot = wsClient.subscribe('layer.snapshot', handleMessage);
    const unsubLayerUpdate = wsClient.subscribe('layer.update', handleMessage);
    const unsubBatchUpdate = wsClient.subscribe('layer.batch_update', handleMessage);
    const unsubIndicator = wsClient.subscribe('indicator.update', handleIndicatorMessage);

    return () => {
      unsubUpdate();
      unsubSnapshot();
      unsubLayerSnapshot();
      unsubLayerUpdate();
      unsubBatchUpdate();
      unsubIndicator();
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
      snapshotDisposition.clear();
    };
  }, [handleMessage, handleIndicatorMessage]);

  useEffect(() => {
    const prevSet = prevEnabledLayersRef.current;
    const currentSet = new Set(enabledLayers);

    // Find newly enabled layers — O(n) with Set lookup instead of O(n²) with includes.
    const newLayers = enabledLayers.filter((id) => !prevSet.has(id));
    // Find newly disabled layers
    const removedLayers = [...prevSet].filter((id) => !currentSet.has(id));

    // Get current viewport for spatial layers
    const viewport = useViewerStore.getState().viewport;
    const { timeMode, timeFrom, timeTo } = useUIStore.getState();

    // Subscribe to new layers
    newLayers.forEach((layerId) => {
      clearLayerEntities(layerId);
      // Initial load for this layer — its snapshot must REPLACE atomically.
      setLayerSnapshotDisposition(layerId, 'replace');
      // Only spatial (viewport-filtered) layers need a viewport — non-spatial
      // layers must subscribe without one so the backend uses the full cache/DB fallback.
      const layerViewport = isSpatialLayer(layerId) ? (viewport ?? undefined) : undefined;
      if (timeMode === 'range' && timeFrom) {
        // Range mode — send time_range to query Postgres with time bounds.
        const effectiveTo = timeTo ?? new Date().toISOString();
        wsClient.sendTimeRange(layerId, timeFrom, effectiveTo, layerViewport);
      } else {
        // Live mode — use subscribe to read the current snapshot (no time filtering).
        // The backend's handleSubscribe sends the current cache snapshot and
        // enables Pub/Sub for real-time updates.
        wsClient.subscribeLayer(layerId, layerViewport);
      }
    });

    // Unsubscribe from removed layers and clear their data
    removedLayers.forEach((layerId) => {
      wsClient.unsubscribeLayer(layerId);
      clearLayerEntities(layerId);
    });

    // Update ref for next comparison
    prevEnabledLayersRef.current = currentSet;
  }, [enabledLayers, clearLayerEntities]);

  // Time range mode — react to timeMode/timeFrom/timeTo changes
  const timeMode = useUIStore((s) => s.timeMode);
  const timeFrom = useUIStore((s) => s.timeFrom);
  const timeTo = useUIStore((s) => s.timeTo);
  const timeModeInitializedRef = useRef(false);

  useEffect(() => {
    // Skip the initial run: on mount the dedicated re-subscribe effect already
    // re-establishes subscriptions WITHOUT clearing. Clearing here would wipe
    // the persisted working set on every remount. Only an actual time-window
    // CHANGE (which invalidates old data) should clear + replace.
    if (!timeModeInitializedRef.current) {
      timeModeInitializedRef.current = true;
      return;
    }

    const { enabledLayers } = useUIStore.getState();
    const viewport = useViewerStore.getState().viewport;

    if (timeMode === 'live') {
      // Live mode — use subscribe to read the current snapshot (no time filtering).
      enabledLayers.forEach((layerId) => {
        clearLayerEntities(layerId);
        // Time window changed → snapshot must REPLACE (old data is invalid).
        setLayerSnapshotDisposition(layerId, 'replace');
        const lv = isSpatialLayer(layerId) ? (viewport ?? undefined) : undefined;
        wsClient.subscribeLayer(layerId, lv);
      });
      return;
    }

    // Range mode — send time_range for each enabled layer
    if (!timeFrom) return;
    const effectiveTo = timeTo ?? new Date().toISOString();

    enabledLayers.forEach((layerId) => {
      clearLayerEntities(layerId);
      // Time window changed → snapshot must REPLACE (old data is invalid).
      setLayerSnapshotDisposition(layerId, 'replace');
      const lv = isSpatialLayer(layerId) ? (viewport ?? undefined) : undefined;
      wsClient.sendTimeRange(layerId, timeFrom, effectiveTo, lv);
    });
  }, [timeMode, timeFrom, timeTo, clearLayerEntities]);

  // --- Re-subscribe on mount for already-enabled layers (m910) ---
  // Re-establishes WS subscriptions when the hook remounts (e.g. switching
  // back from dashboard to earth) WITHOUT clearing entity data, so the working
  // set persists across mode switches. The snapshot the server sends in reply
  // MERGES into the retained entities rather than wiping them.
  useEffect(() => {
    const layers = useUIStore.getState().enabledLayers;
    if (layers.length === 0) return;

    const { timeMode: tm, timeFrom: tf, timeTo: tt } = useUIStore.getState();
    const viewport = useViewerStore.getState().viewport;

    layers.forEach((layerId) => {
      // Merge: the remount's snapshot must not wipe entities already in the store.
      setLayerSnapshotDisposition(layerId, 'merge');
      const lv = isSpatialLayer(layerId) ? (viewport ?? undefined) : undefined;
      if (tm === 'range' && tf) {
        const effectiveTo = tt ?? new Date().toISOString();
        wsClient.sendTimeRange(layerId, tf, effectiveTo, lv);
      } else {
        wsClient.subscribeLayer(layerId, lv);
      }
    });
  }, []);
}

/**
 * Hook that syncs the camera viewport to the WebSocket server for spatial layers.
 * Debounced at 300ms to avoid flooding the server on rapid camera movements.
 *
 * Driven by useViewerStore.subscribe() in a mount-only ([]-dep) effect so that
 * camera-driven viewport writes (~60/sec) do NOT re-render the host component.
 * All state is read via getState() inside the callback to avoid stale closures.
 */
export function useViewportSync() {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastSentRef = useRef<string | null>(null);

  useEffect(() => {
    const unsubscribe = useViewerStore.subscribe(
      (state) => state.viewport,
      (viewport) => {
        if (!viewport) return;

        const key = JSON.stringify(viewport);
        if (key === lastSentRef.current) return;

        // Skip minor drift at high altitude: if viewport center moved less than
        // ~2° from the last sent viewport, suppress the update to avoid flooding
        // during orbital fly-overs.
        if (lastSentRef.current) {
          const prevVp = JSON.parse(lastSentRef.current);
          const prevCenterLat = (prevVp.north + prevVp.south) / 2;
          const prevCenterLon = (prevVp.east + prevVp.west) / 2;
          const newCenterLat = (viewport.north + viewport.south) / 2;
          const newCenterLon = (viewport.east + viewport.west) / 2;
          if (
            Math.abs(newCenterLat - prevCenterLat) < 2 &&
            Math.abs(newCenterLon - prevCenterLon) < 2
          )
            return;
        }

        if (timerRef.current) clearTimeout(timerRef.current);

        timerRef.current = setTimeout(() => {
          lastSentRef.current = key;
          // Read fresh state inside the callback to avoid stale closures —
          // layers or time mode may have changed during the 300ms debounce.
          const {
            timeMode,
            timeFrom,
            timeTo,
            enabledLayers: currentLayers,
          } = useUIStore.getState();

          if (timeMode === 'range' && timeFrom) {
            // In range mode, send time_range requests with the viewport. Same time
            // window, only the viewport panned → MERGE so prior entities persist.
            const effectiveTo = timeTo ?? new Date().toISOString();
            currentLayers.forEach((layerId) => {
              if (isSpatialLayer(layerId)) {
                setLayerSnapshotDisposition(layerId, 'merge');
                wsClient.sendTimeRange(layerId, timeFrom, effectiveTo, viewport);
              }
            });
          } else {
            // Live mode — send lightweight viewport_update (server-side spatial query)
            // instead of time_range (which always hits Postgres). Pan → MERGE.
            currentLayers.forEach((layerId) => {
              if (isSpatialLayer(layerId)) {
                setLayerSnapshotDisposition(layerId, 'merge');
                wsClient.sendViewportUpdate(layerId, viewport);
              }
            });
          }
        }, 300);
      },
    );

    return () => {
      unsubscribe();
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);
}

/**
 * Hook to get entities for a specific layer from the store.
 * Returns direct Map references and a dirty-ID set for O(delta) rendering.
 * Uses granular selectors so only updates for THIS layer trigger re-renders.
 */
export function useLayerEntities(layerId: string) {
  // Use layerVersions as the change signal instead of the Map reference
  const version = useUIStore((s) => s.layerVersions[layerId] ?? 0);
  const maxEntities = useUIStore((s) => s.maxEntities);
  const config = useUIStore((s) => s.layerConfigs[layerId]);

  return useMemo(() => {
    const data = useUIStore.getState().layerEntities.get(layerId);
    if (!data) {
      return {
        entityMap: null as Map<string, Entity> | null,
        obsMap: null as Map<string, Observation> | null,
        dirtyEntityIds: null as Set<string> | null,
        config: config || {},
        hasData: false,
        totalCount: 0,
        version,
        maxEntities,
        needsRebuild: false as const,
      };
    }
    return {
      entityMap: data.entityMap,
      obsMap: data.obsMap,
      dirtyEntityIds: data.dirtyEntityIds,
      config: config || {},
      hasData: true,
      totalCount: data.entityMap.size,
      version,
      maxEntities,
      needsRebuild: data.needsRebuild ?? false,
    };
  }, [version, layerId, maxEntities, config]);
}
