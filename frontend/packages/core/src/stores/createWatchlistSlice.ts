import type { WatchlistEntity } from '../models/watchlist';
import { MAX_PINNED_ENTITIES } from '../models/watchlist';
import type { SliceCreator } from './types';

/** State + actions for watchlist management. */
export interface WatchlistSlice {
  watchlistEntities: WatchlistEntity[];
  watchlistPanelOpen: boolean;
  watchlistBarHeight: number;
  findMode: boolean;
  findModeDisplay: 'hidden' | 'dimmed';
  searchOpen: boolean;
  searchQuery: string;
  addToWatchlist: (entity: WatchlistEntity) => void;
  removeFromWatchlist: (entityId: string) => void;
  togglePin: (entityId: string) => void;
  pinEntity: (entityId: string) => void;
  unpinEntity: (entityId: string) => void;
  clearUnpinned: () => void;
  clearWatchlist: () => void;
  toggleFindMode: () => void;
  setFindModeDisplay: (mode: 'hidden' | 'dimmed') => void;
  setWatchlistPanelOpen: (open: boolean) => void;
  setWatchlistBarHeight: (height: number) => void;
  toggleSearch: () => void;
  setSearchQuery: (query: string) => void;
}

export const createWatchlistSlice: SliceCreator<WatchlistSlice> = (set) => ({
  watchlistEntities: [],
  watchlistPanelOpen: true,
  watchlistBarHeight: 0,
  findMode: false,
  findModeDisplay: 'dimmed' as const,
  searchOpen: false,
  searchQuery: '',
  addToWatchlist: (entity) =>
    set((state) => {
      const idx = state.watchlistEntities.findIndex((e) => e.entityId === entity.entityId);
      if (idx >= 0) {
        const updated = [...state.watchlistEntities];
        updated[idx] = { ...updated[idx], name: entity.name, layerId: entity.layerId };
        return { watchlistEntities: updated };
      }
      return { watchlistEntities: [...state.watchlistEntities, entity] };
    }),
  removeFromWatchlist: (entityId) =>
    set((state) => ({
      watchlistEntities: state.watchlistEntities.filter((e) => e.entityId !== entityId),
    })),
  togglePin: (entityId) =>
    set((state) => {
      const entity = state.watchlistEntities.find((e) => e.entityId === entityId);
      if (!entity) return state;
      if (entity.pinned) {
        return {
          watchlistEntities: state.watchlistEntities.map((e) =>
            e.entityId === entityId ? { ...e, pinned: false } : e,
          ),
        };
      }
      const pinnedCount = state.watchlistEntities.filter((e) => e.pinned).length;
      if (pinnedCount >= MAX_PINNED_ENTITIES) return state;
      return {
        watchlistEntities: state.watchlistEntities.map((e) =>
          e.entityId === entityId ? { ...e, pinned: true } : e,
        ),
      };
    }),
  pinEntity: (entityId) =>
    set((state) => {
      const entity = state.watchlistEntities.find((e) => e.entityId === entityId);
      if (!entity || entity.pinned) return state;
      const pinnedCount = state.watchlistEntities.filter((e) => e.pinned).length;
      if (pinnedCount >= MAX_PINNED_ENTITIES) return state;
      return {
        watchlistEntities: state.watchlistEntities.map((e) =>
          e.entityId === entityId ? { ...e, pinned: true } : e,
        ),
      };
    }),
  unpinEntity: (entityId) =>
    set((state) => ({
      watchlistEntities: state.watchlistEntities.map((e) =>
        e.entityId === entityId ? { ...e, pinned: false } : e,
      ),
    })),
  clearUnpinned: () =>
    set((state) => ({
      watchlistEntities: state.watchlistEntities.filter((e) => e.pinned),
    })),
  clearWatchlist: () =>
    set({
      watchlistEntities: [],
      watchlistPanelOpen: false,
      findMode: false,
      findModeDisplay: 'dimmed',
    }),
  toggleFindMode: () =>
    set((state) => {
      if (!state.findMode) {
        const hasPinned = state.watchlistEntities.some((e) => e.pinned);
        if (!hasPinned) return state;
      }
      return { findMode: !state.findMode };
    }),
  setFindModeDisplay: (mode) => set({ findModeDisplay: mode }),
  setWatchlistPanelOpen: (open) => set({ watchlistPanelOpen: open }),
  setWatchlistBarHeight: (height) => set({ watchlistBarHeight: height }),
  toggleSearch: () =>
    set((state) => {
      const next: Partial<WatchlistSlice> = { searchOpen: !state.searchOpen };
      if (!state.searchOpen && state.watchlistEntities.length > 0) {
        next.watchlistPanelOpen = true;
      }
      return next;
    }),
  setSearchQuery: (query) => set({ searchQuery: query }),
});
