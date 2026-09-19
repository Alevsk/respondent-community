/**
 * useEntityTrail — progressively fetches observation history and accumulates
 * trail points for rendering on the globe and in the timeline.
 *
 * Points are returned oldest-first (ascending by timestamp) so Cesium
 * polylines draw from past → present.
 */

import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { useEntityObservations } from '../../../shared/api/queries';
import { useUIStore } from '@/app/store';

const DEFAULT_PAGE_SIZE = 100;
const DEFAULT_MAX_POINTS = 500;

// Re-export from core so existing consumers don't need to update imports.
export type { TrailPoint } from '@respondent/core';
import type { TrailPoint } from '@respondent/core';

interface UseEntityTrailOptions {
  pageSize?: number;
  maxPoints?: number;
  /** If false, disables fetching. Defaults to true. */
  enabled?: boolean;
}

interface UseEntityTrailResult {
  /** Trail points ordered oldest-first (ascending by ts). */
  points: TrailPoint[];
  /** Whether more pages are available from the API. */
  hasMore: boolean;
  /** Whether a fetch is currently in progress. */
  isLoading: boolean;
  /** Fetch the next page of older observations. */
  fetchMore: () => void;
}

/**
 * Extracts speed from velocity map if available.
 * Velocity map typically has keys like "speed_mps" or "ground_speed".
 */
function extractSpeed(velocity?: Record<string, number>): number | undefined {
  if (!velocity) return undefined;
  return velocity.speed_mps ?? velocity.ground_speed ?? velocity.speed ?? undefined;
}

export function useEntityTrail(
  entityId: string,
  options: UseEntityTrailOptions = {},
): UseEntityTrailResult {
  const { pageSize = DEFAULT_PAGE_SIZE, maxPoints = DEFAULT_MAX_POINTS, enabled = true } = options;

  // Cursor for pagination — oldest timestamp in accumulated points
  const [beforeMs, setBeforeMs] = useState<number | undefined>(undefined);
  // Accumulated pages of points (newest-first from API, we reverse at output)
  const [accumulatedPages, setAccumulatedPages] = useState<TrailPoint[][]>([]);
  const [serverHasMore, setServerHasMore] = useState(true);

  const totalAccumulated = useMemo(
    () => accumulatedPages.reduce((sum, page) => sum + page.length, 0),
    [accumulatedPages],
  );

  const reachedMax = totalAccumulated >= maxPoints;

  const { data, isLoading } = useEntityObservations(enabled ? entityId : null, pageSize, beforeMs);

  // Track previous data ref to detect new pages
  const lastProcessedKey = useRef<string | null>(null);

  useEffect(() => {
    if (!data || data.observations.length === 0) {
      if (data) setServerHasMore(data.hasMore);
      return;
    }

    // Build a cache key from the first and last observation timestamps
    const first = data.observations[0];
    const last = data.observations[data.observations.length - 1];
    const key = `${first.ts}-${last.ts}-${data.observations.length}`;

    if (key === lastProcessedKey.current) return;
    lastProcessedKey.current = key;

    const page: TrailPoint[] = data.observations.map((obs) => ({
      ts: obs.ts,
      lon: obs.position.lon,
      lat: obs.position.lat,
      altitudeM: obs.altitudeM,
      speed: extractSpeed(obs.velocity),
      metadata: obs.metadata,
    }));

    setAccumulatedPages((prev) => [...prev, page]);
    setServerHasMore(data.hasMore);
  }, [data]);

  // Reset when entityId changes
  const prevEntityId = useRef(entityId);
  useEffect(() => {
    if (prevEntityId.current !== entityId) {
      prevEntityId.current = entityId;
      setBeforeMs(undefined);
      setAccumulatedPages([]);
      setServerHasMore(true);
      lastProcessedKey.current = null;
    }
  }, [entityId]);

  const fetchMore = useCallback(() => {
    if (!serverHasMore || reachedMax || isLoading) return;
    // The API returns newest-first, so the last observation in the last page
    // is the oldest — use its timestamp as the cursor
    const lastPage = accumulatedPages[accumulatedPages.length - 1];
    if (!lastPage || lastPage.length === 0) return;
    const oldestInLastPage = lastPage[lastPage.length - 1];
    setBeforeMs(oldestInLastPage.ts);
  }, [serverHasMore, reachedMax, isLoading, accumulatedPages]);

  // Flatten and reverse: API gives newest-first, we want oldest-first
  const points = useMemo(() => {
    const flat = accumulatedPages.flat();
    // Cap to maxPoints (keep the newest maxPoints observations)
    const capped = flat.length > maxPoints ? flat.slice(0, maxPoints) : flat;
    // Reverse: API newest-first → oldest-first for polyline rendering
    return capped.slice().reverse();
  }, [accumulatedPages, maxPoints]);

  // Sync accumulated points to the store so the trail renderer shares the
  // same data without its own fetch cycle. Only writes when points change.
  useEffect(() => {
    if (enabled && points.length > 0) {
      useUIStore.getState().setTrailPoints(entityId, points);
    }
  }, [entityId, points, enabled]);

  const hasMore = serverHasMore && !reachedMax;

  return { points, hasMore, isLoading, fetchMore };
}
