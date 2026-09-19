// React Query hooks for API calls

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, endpoints } from '@respondent/core';
import { useUIStore } from '@/app/store';
import type { FieldRendererConfig } from '@respondent/core';
import { registerDynamicIcon } from '../../features/globe/icons/iconRegistry';
import { registerDynamicRenderers } from '../../features/entity/tabs/overview/fieldRenderers';

// Re-export API types and utilities from @respondent/core for backward compatibility.
// Consumers that import from './queries' continue to work without changes.
export type {
  ApiLayer,
  ApiEntity as Entity,
  ApiObservation as Observation,
  LayerSnapshotResponse,
  LayerToggleRequest,
  Scene,
  ApiFilterPreset as FilterPreset,
  EntityDetailResponse,
  ObservationHistoryResponse,
} from '@respondent/core';

export {
  normalizeProtoEnum,
  normalizeObservationHistory,
  normalizeEntityDetail,
  queryKeys,
} from '@respondent/core';

// Import what we need locally for the hook implementations
import type { ApiLayer } from '@respondent/core';
import {
  normalizeProtoEnum,
  normalizeObservationHistory,
  normalizeEntityDetail,
  queryKeys,
} from '@respondent/core';

// Layers hooks
export function useLayers() {
  return useQuery({
    queryKey: queryKeys.layers,
    queryFn: async () => {
      const response = await api.get<{ layers: ApiLayer[] }>(endpoints.layers);
      const layers = response.layers ?? [];

      // Initialize dynamic registrations for declarative source layers.
      // This is idempotent — re-registering with the same config is safe.
      for (const layer of layers) {
        const dc = layer.displayConfig;
        if (!dc) continue;

        // Register dynamic icon draw function and rotation/scale config
        if (dc.icon?.shape) {
          registerDynamicIcon(layer.type, {
            shape: dc.icon.shape,
            rotatable: dc.icon.rotatable ?? false,
            scale: dc.icon.scale > 0 ? dc.icon.scale : 1.0,
          });
        }

        // Register dynamic field renderers
        if (dc.fieldRenderers && dc.fieldRenderers.length > 0) {
          registerDynamicRenderers(layer.type, dc.fieldRenderers as FieldRendererConfig[]);
        }
      }

      // Sync layer metadata into the store so renderers can look up displayConfig
      // via useUIStore.getState().layers[layerType]
      const setLayers = useUIStore.getState().setLayers;
      // Map API Layer shape to store Layer shape (snake_case → camelCase where needed)
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
  });
}

export function useLayer(layerId: string) {
  return useQuery({
    queryKey: queryKeys.layer(layerId),
    queryFn: () => api.get<ApiLayer>(`${endpoints.layers}/${layerId}`),
    enabled: !!layerId,
  });
}

export function useLayerSnapshot(layerId: string, limit = 500, offset = 0) {
  return useQuery({
    queryKey: queryKeys.layerSnapshot(layerId, limit, offset),
    queryFn: () =>
      api.get<import('@respondent/core').LayerSnapshotResponse>(
        `${endpoints.layers}/${layerId}/snapshot?limit=${limit}&offset=${offset}`,
      ),
    enabled: !!layerId,
  });
}

export function useToggleLayer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (toggle: import('@respondent/core').LayerToggleRequest) =>
      api.put<ApiLayer>(`${endpoints.layers}/${toggle.layer_id}`, {
        enabled: toggle.enabled,
        mode: toggle.mode,
        density: toggle.density,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.layers });
    },
  });
}

// Scenes hooks
export function useScenes() {
  return useQuery({
    queryKey: queryKeys.scenes,
    queryFn: () => api.get<import('@respondent/core').Scene[]>(endpoints.scenes),
  });
}

export function useCreateScene() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (scene: Omit<import('@respondent/core').Scene, 'id'>) =>
      api.post<import('@respondent/core').Scene>(endpoints.scenes, scene),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.scenes });
    },
  });
}

export function useUpdateScene() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, ...scene }: import('@respondent/core').Scene) =>
      api.put<import('@respondent/core').Scene>(`${endpoints.scenes}/${id}`, scene),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.scenes });
    },
  });
}

export function useDeleteScene() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => api.delete(`${endpoints.scenes}/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.scenes });
    },
  });
}

// Filter presets hooks
export function useFilterPresets() {
  return useQuery({
    queryKey: queryKeys.filterPresets,
    queryFn: () => api.get<import('@respondent/core').ApiFilterPreset[]>(endpoints.filters),
  });
}

// Entity detail hooks
export function useEntityDetail(entityId: string | null) {
  return useQuery({
    queryKey: queryKeys.entityDetail(entityId ?? ''),
    queryFn: async () => {
      const raw = await api.get(
        `${endpoints.entities}/detail?entity_id=${encodeURIComponent(entityId!)}`,
      );
      return normalizeEntityDetail(raw);
    },
    enabled: !!entityId,
  });
}

export function useEntityObservations(entityId: string | null, limit = 50, beforeMs?: number) {
  return useQuery({
    queryKey: queryKeys.entityObservations(entityId ?? '', limit, beforeMs),
    queryFn: async () => {
      let url = `${endpoints.entities}/observations?entity_id=${encodeURIComponent(entityId!)}&limit=${limit}`;
      if (beforeMs) url += `&before_ms=${beforeMs}`;
      const raw = await api.get(url);
      return normalizeObservationHistory(raw);
    },
    enabled: !!entityId,
  });
}
