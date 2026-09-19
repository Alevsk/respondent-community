import { useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useRef } from 'react';
import { api, endpoints, normalizeInsight, buildFilterParams } from '@respondent/core';
import type { AIInsightNotification, AIInsightsResponse } from '@respondent/core';
import { useUIStore } from '@/app/store';

const NOTIFICATION_PAGE_SIZE = 20;

/**
 * Backfills notifications from REST on mount and provides a loadMore function
 * for pagination. Aligns REST queries with the active notification filter.
 * Re-fetches when filter changes.
 */
export function useNotificationBackfill() {
  const mergeNotifications = useUIStore((s) => s.mergeNotifications);
  const clearNotifications = useUIStore((s) => s.clearNotifications);
  const setNotificationsLoading = useUIStore((s) => s.setNotificationsLoading);
  const notificationFilter = useUIStore((s) => s.notificationFilter);

  // Track filter changes to clear store before new data arrives.
  const filterKeyRef = useRef(
    `${notificationFilter.minAttention}|${notificationFilter.insightTypes.join(',')}`,
  );
  const filterKey = `${notificationFilter.minAttention}|${notificationFilter.insightTypes.join(',')}`;

  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      filterKeyRef.current = filterKey;
      clearNotifications();
    }
  }, [filterKey, clearNotifications]);

  const { data } = useQuery({
    queryKey: [
      'ai-insights-backfill',
      notificationFilter.minAttention,
      notificationFilter.insightTypes.join(','),
    ],
    queryFn: async () => {
      const qs = buildFilterParams(notificationFilter, NOTIFICATION_PAGE_SIZE, 0);
      return api.get<AIInsightsResponse>(`${endpoints.aiInsights}?${qs}`);
    },
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
  });

  // Sync query result to Zustand store. Runs when data changes (fresh fetch
  // or cache hit from a previously-seen filter).
  useEffect(() => {
    if (data) {
      const items: AIInsightNotification[] = (data.insights ?? []).map(normalizeInsight);
      mergeNotifications(items, data.totalCount ?? 0);
    }
  }, [data, mergeNotifications]);

  const loadMore = useCallback(async () => {
    const { notifications, notificationFilter: filter } = useUIStore.getState();
    const offset = notifications.length;
    setNotificationsLoading(true);
    try {
      const qs = buildFilterParams(filter, NOTIFICATION_PAGE_SIZE, offset);
      const resp = await api.get<AIInsightsResponse>(`${endpoints.aiInsights}?${qs}`);
      const items: AIInsightNotification[] = (resp.insights ?? []).map(normalizeInsight);
      mergeNotifications(items, resp.totalCount ?? 0);
    } finally {
      setNotificationsLoading(false);
    }
  }, [mergeNotifications, setNotificationsLoading]);

  return { loadMore };
}
