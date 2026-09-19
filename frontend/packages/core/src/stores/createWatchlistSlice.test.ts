import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createWatchlistSlice, type WatchlistSlice } from './createWatchlistSlice';
import type { WatchlistEntity } from '../models/watchlist';

const makeStore = () => createStore<WatchlistSlice>()(createWatchlistSlice);

const mockEntity: WatchlistEntity = {
  entityId: 'entity-1',
  layerId: 'flights',
  name: 'Flight 123',
  pinned: false,
  addedAt: Date.now(),
};

describe('createWatchlistSlice', () => {
  it('defaults to empty watchlist', () => {
    const store = makeStore();
    expect(store.getState().watchlistEntities).toEqual([]);
    expect(store.getState().watchlistPanelOpen).toBe(true);
  });

  it('addToWatchlist adds an entity', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    expect(store.getState().watchlistEntities).toHaveLength(1);
    expect(store.getState().watchlistEntities[0].entityId).toBe('entity-1');
  });

  it('addToWatchlist updates existing entity', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    store.getState().addToWatchlist({ ...mockEntity, name: 'Updated Name' });
    expect(store.getState().watchlistEntities).toHaveLength(1);
    expect(store.getState().watchlistEntities[0].name).toBe('Updated Name');
  });

  it('removeFromWatchlist removes by entityId', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    store.getState().removeFromWatchlist('entity-1');
    expect(store.getState().watchlistEntities).toHaveLength(0);
  });

  it('togglePin pins an unpinned entity', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    store.getState().togglePin('entity-1');
    expect(store.getState().watchlistEntities[0].pinned).toBe(true);
  });

  it('togglePin unpins a pinned entity', () => {
    const store = makeStore();
    store.getState().addToWatchlist({ ...mockEntity, pinned: true });
    store.getState().togglePin('entity-1');
    expect(store.getState().watchlistEntities[0].pinned).toBe(false);
  });

  it('pinEntity enforces MAX_PINNED_ENTITIES cap', () => {
    const store = makeStore();
    // Add 21 entities, pin the first 20
    for (let i = 0; i < 21; i++) {
      store.getState().addToWatchlist({
        ...mockEntity,
        entityId: `e-${i}`,
        pinned: i < 20,
      });
    }
    // Try to pin the 21st — should be a no-op
    store.getState().pinEntity('e-20');
    expect(store.getState().watchlistEntities[20].pinned).toBe(false);
  });

  it('clearUnpinned keeps only pinned entities', () => {
    const store = makeStore();
    store.getState().addToWatchlist({ ...mockEntity, pinned: true });
    store.getState().addToWatchlist({ ...mockEntity, entityId: 'e-2', pinned: false });
    store.getState().clearUnpinned();
    expect(store.getState().watchlistEntities).toHaveLength(1);
    expect(store.getState().watchlistEntities[0].entityId).toBe('entity-1');
  });

  it('clearWatchlist empties everything', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    store.getState().clearWatchlist();
    expect(store.getState().watchlistEntities).toEqual([]);
    expect(store.getState().watchlistPanelOpen).toBe(false);
    expect(store.getState().findMode).toBe(false);
  });

  it('toggleFindMode does nothing without pinned entities', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity); // unpinned
    store.getState().toggleFindMode();
    expect(store.getState().findMode).toBe(false);
  });

  it('toggleFindMode activates with pinned entities', () => {
    const store = makeStore();
    store.getState().addToWatchlist({ ...mockEntity, pinned: true });
    store.getState().toggleFindMode();
    expect(store.getState().findMode).toBe(true);
  });

  it('toggleSearch reopens watchlist panel when entities exist', () => {
    const store = makeStore();
    store.getState().addToWatchlist(mockEntity);
    store.getState().setWatchlistPanelOpen(false);
    store.getState().toggleSearch();
    expect(store.getState().searchOpen).toBe(true);
    expect(store.getState().watchlistPanelOpen).toBe(true);
  });
});
