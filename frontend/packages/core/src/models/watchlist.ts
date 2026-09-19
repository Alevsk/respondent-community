// Watchlist entity for persistent entity tracking across sessions
export interface WatchlistEntity {
  entityId: string; // composite: "layerType:externalId"
  layerId: string; // layer type
  name: string; // display name
  pinned: boolean; // persists through deselection and page reload
  addedAt: number; // timestamp for ordering
}

/** Maximum number of pinned entities allowed in the watchlist. */
export const MAX_PINNED_ENTITIES = 20;
