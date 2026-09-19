/**
 * useWatchlistPersistence — syncs pinned watchlist entities to/from localStorage.
 *
 * On mount: loads pinned entities from localStorage and merges into store.
 * On change: persists only pinned entities to localStorage.
 *
 * Schema is versioned for forward compatibility. Corrupt data is discarded
 * gracefully and the store starts fresh.
 */

import { useEffect, useRef } from 'react';
import { useUIStore } from '@/app/store';
import type { WatchlistEntity } from '@/app/store';

export const WATCHLIST_STORAGE_KEY = 'respondent:watchlist';
export const WATCHLIST_SCHEMA_VERSION = 1;

export interface WatchlistStorageSchema {
  version: number;
  pinnedEntities: Array<{
    entityId: string;
    layerId: string;
    name: string;
    addedAt: number;
  }>;
}

/** Parse and validate localStorage data. Returns null on any failure. */
export function parseWatchlistStorage(raw: string | null): WatchlistStorageSchema | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    if (
      typeof parsed !== 'object' ||
      parsed === null ||
      parsed.version !== WATCHLIST_SCHEMA_VERSION ||
      !Array.isArray(parsed.pinnedEntities)
    ) {
      return null;
    }
    // Validate each entity has required fields
    const valid = parsed.pinnedEntities.every(
      (e: Record<string, unknown>) =>
        typeof e.entityId === 'string' &&
        typeof e.layerId === 'string' &&
        typeof e.name === 'string' &&
        typeof e.addedAt === 'number',
    );
    if (!valid) return null;
    return parsed as WatchlistStorageSchema;
  } catch {
    return null;
  }
}

/** Serialize pinned entities for localStorage. */
export function serializeWatchlist(entities: WatchlistEntity[]): string {
  const pinned = entities.filter((e) => e.pinned);
  const schema: WatchlistStorageSchema = {
    version: WATCHLIST_SCHEMA_VERSION,
    pinnedEntities: pinned.map(({ entityId, layerId, name, addedAt }) => ({
      entityId,
      layerId,
      name,
      addedAt,
    })),
  };
  return JSON.stringify(schema);
}

export function useWatchlistPersistence() {
  const initialized = useRef(false);

  // Load from localStorage on mount (once)
  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;

    const raw = localStorage.getItem(WATCHLIST_STORAGE_KEY);
    const data = parseWatchlistStorage(raw);
    if (!data || data.pinnedEntities.length === 0) return;

    const addToWatchlist = useUIStore.getState().addToWatchlist;
    for (const entity of data.pinnedEntities) {
      addToWatchlist({
        entityId: entity.entityId,
        layerId: entity.layerId,
        name: entity.name,
        pinned: true,
        addedAt: entity.addedAt,
      });
    }
  }, []);

  // Subscribe to watchlist changes and persist pinned entities.
  // Zustand v4 subscribe() takes (listener) — listener receives (state, prevState).
  useEffect(() => {
    let prev = useUIStore.getState().watchlistEntities;
    const unsub = useUIStore.subscribe((state) => {
      const next = state.watchlistEntities;
      if (next === prev) return;
      prev = next;
      const pinned = next.filter((e) => e.pinned);
      if (pinned.length === 0) {
        localStorage.removeItem(WATCHLIST_STORAGE_KEY);
      } else {
        localStorage.setItem(WATCHLIST_STORAGE_KEY, serializeWatchlist(next));
      }
    });
    return unsub;
  }, []);
}
