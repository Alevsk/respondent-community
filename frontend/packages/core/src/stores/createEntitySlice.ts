import type { Entity, Observation, TrailPoint, EntityViewState, LayerData } from '../models/entity';
import { DEFAULT_ENTITY_VIEW_STATE, entityKey, obsKey } from '../models/entity';
import type { ViewMode } from '../models/ui';
import type { WatchlistEntity } from '../models/watchlist';
import type { SliceCreator } from './types';

/** State + actions for entity selection, data, and view state. */
export interface EntitySlice {
  selectedEntityId: string | null;
  selectedLayerId: string | null;
  selectedEntities: Array<{ entityId: string; layerId: string }>;
  maxSelectedEntities: number;
  viewMode: ViewMode;
  entityViewState: Record<string, EntityViewState>;
  trailPoints: Record<string, TrailPoint[]>;
  /**
   * Entity data keyed by layer ID.
   *
   * **WARNING: Mutated in-place** by `setLayerEntities` / `clearLayerEntities`
   * for performance (avoids copying large Maps on every WebSocket frame).
   * The Map reference is stable — subscribing via `useStore(s => s.layerEntities)`
   * will NOT trigger rerenders on data changes. Use `layerVersions[layerId]`
   * as the reactive change signal instead.
   */
  layerEntities: Map<string, LayerData>;
  /** Per-layer counter incremented by `setLayerEntities`. Use as a reactive dependency. */
  layerVersions: Record<string, number>;
  setSelectedEntity: (entityId: string | null, layerId: string | null) => void;
  addSelectedEntity: (entityId: string, layerId: string) => void;
  removeSelectedEntity: (entityId: string) => void;
  selectMultipleEntities: (entities: Array<{ entityId: string; layerId: string }>) => void;
  setViewMode: (mode: ViewMode) => void;
  clearSelection: () => void;
  updateEntityViewState: (entityId: string, patch: Partial<EntityViewState>) => void;
  setTrailPoints: (entityId: string, points: TrailPoint[]) => void;
  setLayerEntities: (
    layerId: string,
    data: { entities: Entity[]; observations: Observation[] },
    maxWorkingSet?: number,
  ) => void;
  clearLayerEntities: (layerId: string) => void;
  replaceLayerEntities: (
    layerId: string,
    data: { entities: Entity[]; observations: Observation[] },
  ) => void;
}

/**
 * Cross-slice state that the entity slice reads/writes.
 * The entity selection actions need access to watchlistEntities and layerEntities
 * from the full composed store.
 */
interface CrossSliceAccess {
  watchlistEntities: WatchlistEntity[];
  layerEntities: Map<string, LayerData>;
}

/**
 * Creates the entity slice. Uses a generic TStore parameter to access
 * cross-slice state (watchlistEntities) via get().
 *
 * `_get` and `_api` are unused here but must stay in the signature so it
 * matches SliceCreator<EntitySlice, TStore> for callers.
 */
export const createEntitySlice = <TStore extends EntitySlice & CrossSliceAccess>(
  set: Parameters<SliceCreator<EntitySlice, TStore>>[0],
  _get: Parameters<SliceCreator<EntitySlice, TStore>>[1],
  _api: Parameters<SliceCreator<EntitySlice, TStore>>[2],
): EntitySlice => ({
  selectedEntityId: null,
  selectedLayerId: null,
  selectedEntities: [],
  maxSelectedEntities: 5,
  viewMode: 'globe',
  entityViewState: {},
  trailPoints: {},
  layerEntities: new Map(),
  layerVersions: {},
  setSelectedEntity: (entityId, layerId) =>
    set((state) => {
      const base = {
        selectedEntityId: entityId,
        selectedLayerId: layerId,
        selectedEntities: entityId && layerId ? [{ entityId, layerId }] : [],
      } as Partial<TStore>;
      // Strip all unpinned watchlist entries (except the newly selected one)
      const pinned = state.watchlistEntities.filter((e) => e.pinned);
      if (entityId && layerId) {
        const alreadyPinned = pinned.some((e) => e.entityId === entityId);
        if (alreadyPinned) {
          return { ...base, watchlistEntities: pinned } as Partial<TStore>;
        }
        // Resolve display name from entity data
        const layerData = state.layerEntities.get(layerId);
        const entity = layerData?.entityMap.get(entityId);
        const displayName = entity?.name || entityId;
        return {
          ...base,
          watchlistEntities: [
            ...pinned,
            { entityId, layerId, name: displayName, pinned: false, addedAt: Date.now() },
          ],
        } as Partial<TStore>;
      }
      // No entity selected — keep only pinned
      return { ...base, watchlistEntities: pinned } as Partial<TStore>;
    }),
  addSelectedEntity: (entityId, layerId) =>
    set((state) => {
      // No-op if already selected or at max
      if (state.selectedEntities.some((e) => e.entityId === entityId)) return state;
      if (state.selectedEntities.length >= state.maxSelectedEntities) return state;
      // Append to watchlist as unpinned if not already present
      const alreadyInWatchlist = state.watchlistEntities.some((e) => e.entityId === entityId);
      if (alreadyInWatchlist) {
        return {
          selectedEntities: [...state.selectedEntities, { entityId, layerId }],
          selectedEntityId: entityId,
          selectedLayerId: layerId,
        } as Partial<TStore>;
      }
      // Resolve display name from entity data
      const layerData = state.layerEntities.get(layerId);
      const entity = layerData?.entityMap.get(entityId);
      const displayName = entity?.name || entityId;
      return {
        selectedEntities: [...state.selectedEntities, { entityId, layerId }],
        selectedEntityId: entityId,
        selectedLayerId: layerId,
        watchlistEntities: [
          ...state.watchlistEntities,
          { entityId, layerId, name: displayName, pinned: false, addedAt: Date.now() },
        ],
      } as Partial<TStore>;
    }),
  selectMultipleEntities: (entities) =>
    set((state) => {
      if (entities.length === 0) return state;
      const capped = entities.slice(0, state.maxSelectedEntities);
      const primary = capped[0];
      // Keep pinned watchlist entries, add new unpinned entries for each entity
      const pinned = state.watchlistEntities.filter((e) => e.pinned);
      const pinnedIds = new Set(pinned.map((e) => e.entityId));
      const newWatchlist = [...pinned];
      const now = Date.now();
      for (const { entityId, layerId } of capped) {
        if (pinnedIds.has(entityId)) continue;
        const layerData = state.layerEntities.get(layerId);
        const entity = layerData?.entityMap.get(entityId);
        const displayName = entity?.name || entityId;
        newWatchlist.push({ entityId, layerId, name: displayName, pinned: false, addedAt: now });
      }
      return {
        selectedEntityId: primary.entityId,
        selectedLayerId: primary.layerId,
        selectedEntities: capped,
        watchlistEntities: newWatchlist,
        entityViewState: {},
        trailPoints: {},
      } as Partial<TStore>;
    }),
  removeSelectedEntity: (entityId) =>
    set((state) => {
      const filtered = state.selectedEntities.filter((e) => e.entityId !== entityId);
      if (filtered.length === state.selectedEntities.length) return state;
      const last = filtered.length > 0 ? filtered[filtered.length - 1] : null;
      const remainingViewState = { ...state.entityViewState };
      delete remainingViewState[entityId];
      const remainingTrailPoints = { ...state.trailPoints };
      delete remainingTrailPoints[entityId];
      return {
        selectedEntities: filtered,
        selectedEntityId:
          state.selectedEntityId === entityId ? (last?.entityId ?? null) : state.selectedEntityId,
        selectedLayerId:
          state.selectedEntityId === entityId ? (last?.layerId ?? null) : state.selectedLayerId,
        entityViewState: remainingViewState,
        trailPoints: remainingTrailPoints,
      } as Partial<TStore>;
    }),
  setViewMode: (mode) => set({ viewMode: mode } as Partial<TStore>),
  clearSelection: () =>
    set(
      (state) =>
        ({
          selectedEntityId: null,
          selectedLayerId: null,
          selectedEntities: [],
          viewMode: 'globe',
          entityViewState: {},
          trailPoints: {},
          // `as unknown as` needed: watchlistEntities belongs to WatchlistSlice, not EntitySlice
          watchlistEntities: state.watchlistEntities.filter((e) => e.pinned),
        }) as unknown as Partial<TStore>,
    ),
  updateEntityViewState: (entityId, patch) =>
    set(
      (state) =>
        ({
          entityViewState: {
            ...state.entityViewState,
            [entityId]: {
              ...(state.entityViewState[entityId] ?? DEFAULT_ENTITY_VIEW_STATE),
              ...patch,
            },
          },
        }) as Partial<TStore>,
    ),
  setTrailPoints: (entityId, points) =>
    set(
      (state) =>
        ({
          trailPoints: { ...state.trailPoints, [entityId]: points },
        }) as Partial<TStore>,
    ),
  setLayerEntities: (layerId, data, maxWorkingSet) =>
    set((state) => {
      const existing = state.layerEntities.get(layerId);

      const eMap = existing ? existing.entityMap : new Map<string, Entity>();
      const oMap = existing ? existing.obsMap : new Map<string, Observation>();
      const dirtyIds = existing ? existing.dirtyEntityIds : new Set<string>();

      for (const entity of data.entities) {
        const key = entityKey(entity);
        dirtyIds.add(key);
        eMap.set(key, entity);
      }
      for (const obs of data.observations) {
        const key = obsKey(obs);
        const prev = oMap.get(key);
        if (prev && prev.timestamp && obs.timestamp) {
          if (prev.timestamp > obs.timestamp) continue;
        }
        oMap.set(key, obs);
      }

      // Bound the merged working set so panning can't grow memory unboundedly.
      // Evict LRU by last-observation timestamp (oldest first), preserving the
      // most-recently-active entities. Removes from entityMap, obsMap, and the
      // dirty set consistently. Entities with no observation are treated as
      // oldest (timestamp 0) so they don't pin a slot indefinitely.
      let evictCount = 0;
      if (maxWorkingSet && maxWorkingSet > 0 && eMap.size > maxWorkingSet) {
        const byRecency = [...eMap.keys()].sort((a, b) => {
          const ta = Date.parse(oMap.get(a)?.timestamp ?? '') || 0;
          const tb = Date.parse(oMap.get(b)?.timestamp ?? '') || 0;
          return ta - tb;
        });
        evictCount = eMap.size - maxWorkingSet;
        for (let i = 0; i < evictCount; i++) {
          const key = byRecency[i];
          eMap.delete(key);
          oMap.delete(key);
          dirtyIds.delete(key);
        }
      }

      // Mutate the outer Map in-place — no new Map() copy
      state.layerEntities.set(layerId, {
        entityMap: eMap,
        obsMap: oMap,
        dirtyEntityIds: dirtyIds,
        needsRebuild: evictCount > 0 ? true : existing?.needsRebuild,
      });

      // Increment only this layer's version counter to signal selectors
      const prevVersion = state.layerVersions[layerId] ?? 0;
      return {
        layerVersions: { ...state.layerVersions, [layerId]: prevVersion + 1 },
      } as Partial<TStore>;
    }),
  clearLayerEntities: (layerId) =>
    set((state) => {
      state.layerEntities.delete(layerId);
      const rest = { ...state.layerVersions };
      delete rest[layerId];
      return { layerVersions: rest } as Partial<TStore>;
    }),
  replaceLayerEntities: (layerId, data) =>
    set((state) => {
      const eMap = new Map<string, Entity>();
      const oMap = new Map<string, Observation>();
      const dirtyIds = new Set<string>();

      for (const entity of data.entities) {
        const key = entityKey(entity);
        dirtyIds.add(key);
        eMap.set(key, entity);
      }
      for (const obs of data.observations) {
        oMap.set(obsKey(obs), obs);
      }

      state.layerEntities.set(layerId, {
        entityMap: eMap,
        obsMap: oMap,
        dirtyEntityIds: dirtyIds,
        needsRebuild: true,
      });

      const prevVersion = state.layerVersions[layerId] ?? 0;
      return {
        layerVersions: { ...state.layerVersions, [layerId]: prevVersion + 1 },
      } as Partial<TStore>;
    }),
});
