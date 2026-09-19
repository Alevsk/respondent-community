/**
 * Shared mutable map of entity billboard positions updated every frame by
 * BillboardLayerRenderer during lerp/dead-reckoning animation.
 *
 * Consumers (SelectionIndicator, EntityTrailRenderer) read from this map
 * for O(1) position lookups instead of scanning scene primitives.
 *
 * This is NOT reactive state — it's a frame-synchronous data channel
 * written in scene.preUpdate and read in the same or subsequent preUpdate
 * listeners within the same frame.
 */
import type { Cartesian3 } from 'cesium';

/** entityId → current interpolated Cartesian3 billboard position */
export const interpolatedPositions = new Map<string, Cartesian3>();
