/**
 * useAppBootstrap — single entry-point hook that orchestrates all critical
 * data loads required before the application shell can render.
 *
 * Coordinates:
 * 1. Layer metadata fetch (GET /v1/layers) with store sync + icon registration
 * 2. Notification backfill fetch (GET /v1/ai/insights) with store sync
 * 3. WebSocket connection establishment with a 10-second timeout
 *
 * Exposes a BootstrapState consumed by BootstrapGate to show/hide the
 * loading overlay.
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  api,
  endpoints,
  normalizeInsight,
  wsClient,
  useWebSocketStatus,
  normalizeProtoEnum,
} from '@respondent/core';
import type { ApiLayer, FieldRendererConfig } from '@respondent/core';
import { useUIStore } from '@/app/store';
import { registerDynamicIcon } from '../../features/globe/icons/iconRegistry';
import { registerDynamicRenderers } from '../../features/entity/tabs/overview/fieldRenderers';

import type { BootstrapStep, BootstrapState, AIInsightsResponse } from '@respondent/core';
import { WS_TIMEOUT_MS, STEP_DEFS, buildFilterParams } from '@respondent/core';

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useAppBootstrap(): BootstrapState {
  const queryClient = useQueryClient();
  const notificationFilter = useUIStore((s) => s.notificationFilter);
  const mergeNotifications = useUIStore((s) => s.mergeNotifications);
  const addNotification = useUIStore((s) => s.addNotification);

  // --- WebSocket ---
  const wsStatus = useWebSocketStatus();
  const [wsTimedOut, setWsTimedOut] = useState(false);
  const wsTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hasBootstrappedRef = useRef(false);

  // Connect WS on mount
  useEffect(() => {
    wsClient.connect();
  }, []);

  // 10-second WS timeout
  useEffect(() => {
    if (wsStatus === 'connected') {
      // Connected — clear any pending timeout and reset timedOut flag
      if (wsTimeoutRef.current) {
        clearTimeout(wsTimeoutRef.current);
        wsTimeoutRef.current = null;
      }
      setWsTimedOut(false);
      return;
    }

    // Not connected — start timeout if not already running and not already timed out
    if (!wsTimeoutRef.current && !wsTimedOut) {
      wsTimeoutRef.current = setTimeout(() => {
        wsTimeoutRef.current = null;
        setWsTimedOut(true);
      }, WS_TIMEOUT_MS);
    }

    return () => {
      if (wsTimeoutRef.current) {
        clearTimeout(wsTimeoutRef.current);
        wsTimeoutRef.current = null;
      }
    };
  }, [wsStatus, wsTimedOut]);

  // --- Notification stream subscription (replaces useNotificationStream in AppShell) ---

  // Send notification filter on connect and whenever it changes
  useEffect(() => {
    wsClient.sendNotificationFilter({
      min_attention: notificationFilter.minAttention,
      insight_types: notificationFilter.insightTypes,
    });
  }, [notificationFilter]);

  // Subscribe to ai_insight WebSocket messages
  useEffect(() => {
    const unsubscribe = wsClient.subscribe('ai_insight', (message) => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const raw = (message as any).payload ?? message.data ?? message;
      const notification = normalizeInsight(raw);
      if (notification.id) {
        addNotification(notification);
      }
    });
    return unsubscribe;
  }, [addNotification]);

  // --- Layers query ---
  const layersQuery = useQuery({
    queryKey: ['bootstrap', 'layers'],
    queryFn: async () => {
      const response = await api.get<{ layers: ApiLayer[] }>(endpoints.layers);
      const layers = response.layers ?? [];

      // Register dynamic icons and field renderers (idempotent)
      for (const layer of layers) {
        const dc = layer.displayConfig;
        if (!dc) continue;
        if (dc.icon?.shape) {
          registerDynamicIcon(layer.type, {
            shape: dc.icon.shape,
            rotatable: dc.icon.rotatable ?? false,
            scale: dc.icon.scale > 0 ? dc.icon.scale : 1.0,
          });
        }
        if (dc.fieldRenderers && dc.fieldRenderers.length > 0) {
          registerDynamicRenderers(layer.type, dc.fieldRenderers as FieldRendererConfig[]);
        }
      }

      // Sync to Zustand store
      const setLayers = useUIStore.getState().setLayers;
      setLayers(
        layers.map((l) => ({
          id: l.id,
          name: l.name,
          type: l.type,
          enabled: l.enabled,
          mode: l.mode,
          density: l.density,
          source: l.source,
          lastUpdate: l.last_update,
          count: l.count,
          color: l.color,
          pointSize: l.pointSize,
          displayConfig: l.displayConfig,
          historyConfig: l.historyConfig ?? l.history_config,
          renderingMode: normalizeProtoEnum(l.renderingMode ?? l.rendering_mode, 'RENDERING_MODE'),
          filteringMode: normalizeProtoEnum(l.filteringMode ?? l.filtering_mode, 'FILTERING_MODE'),
        })),
      );

      return layers;
    },
    staleTime: Infinity,
    retry: 3,
    retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30_000),
  });

  // --- Notifications backfill query ---
  const notificationsQuery = useQuery({
    queryKey: [
      'bootstrap',
      'notifications',
      notificationFilter.minAttention,
      notificationFilter.insightTypes.join(','),
    ],
    queryFn: async () => {
      const qs = buildFilterParams(notificationFilter, 20, 0);
      const data = await api.get<AIInsightsResponse>(`${endpoints.aiInsights}?${qs}`);

      // Sync to Zustand store
      const items = (data.insights ?? []).map(normalizeInsight);
      mergeNotifications(items, data.totalCount ?? 0);

      return data;
    },
    staleTime: Infinity,
    retry: 3,
    retryDelay: (attemptIndex) => Math.min(1000 * 2 ** attemptIndex, 30_000),
  });

  // --- Cache seeding ---
  // Seed the `useLayers()` query cache so downstream consumers get a cache hit
  useEffect(() => {
    if (layersQuery.data) {
      queryClient.setQueryData(['layers'], layersQuery.data);
    }
  }, [layersQuery.data, queryClient]);

  // Seed the `useNotificationBackfill()` query cache so it doesn't re-fetch
  useEffect(() => {
    if (notificationsQuery.data) {
      queryClient.setQueryData(
        [
          'ai-insights-backfill',
          notificationFilter.minAttention,
          notificationFilter.insightTypes.join(','),
        ],
        notificationsQuery.data,
      );
    }
  }, [notificationsQuery.data, queryClient, notificationFilter]);

  // --- Derive step statuses ---
  const steps: BootstrapStep[] = useMemo(() => {
    function queryStatus(query: {
      isLoading: boolean;
      isFetching: boolean;
      isSuccess: boolean;
      isError: boolean;
    }): BootstrapStep['status'] {
      if (query.isError) return 'error';
      if (query.isSuccess) return 'success';
      if (query.isFetching || query.isLoading) return 'loading';
      return 'pending';
    }

    function wsStepStatus(): BootstrapStep['status'] {
      if (wsTimedOut) return 'error';
      if (wsStatus === 'connected') return 'success';
      return 'loading';
    }

    return STEP_DEFS.map((def) => {
      let status: BootstrapStep['status'];
      switch (def.key) {
        case 'layers':
          status = queryStatus(layersQuery);
          break;
        case 'notifications':
          status = queryStatus(notificationsQuery);
          break;
        case 'websocket':
          status = wsStepStatus();
          break;
        default:
          status = 'pending';
      }
      return { key: def.key, label: def.label, status };
    });
  }, [
    layersQuery.isLoading,
    layersQuery.isFetching,
    layersQuery.isSuccess,
    layersQuery.isError,
    notificationsQuery.isLoading,
    notificationsQuery.isFetching,
    notificationsQuery.isSuccess,
    notificationsQuery.isError,
    wsStatus,
    wsTimedOut,
  ]);

  // Latch: once bootstrap succeeds, stay ready. Prevents filter changes
  // (which create new React Query keys → refetch → isFetching=true) from
  // re-triggering the loading overlay via BootstrapGate.
  const rawReady = steps.every((s) => s.status === 'success');
  if (rawReady) hasBootstrappedRef.current = true;
  const ready = hasBootstrappedRef.current;
  const error = !ready && steps.some((s) => s.status === 'error');

  const currentLabel = useMemo(() => {
    // Prefer the first loading step; fall back to first error step
    const loading = steps.find((s) => s.status === 'loading');
    if (loading) return loading.label;
    const errStep = steps.find((s) => s.status === 'error');
    if (errStep) return errStep.label;
    // All success — return the last label
    return steps[steps.length - 1].label;
  }, [steps]);

  // --- Retry ---
  const retry = useCallback(() => {
    if (layersQuery.isError) {
      queryClient.invalidateQueries({ queryKey: ['bootstrap', 'layers'] });
    }
    if (notificationsQuery.isError) {
      queryClient.invalidateQueries({ queryKey: ['bootstrap', 'notifications'] });
    }
    if (wsTimedOut) {
      setWsTimedOut(false);
      wsClient.connect();
    }
  }, [queryClient, layersQuery.isError, notificationsQuery.isError, wsTimedOut]);

  return { steps, ready, error, currentLabel, retry };
}
