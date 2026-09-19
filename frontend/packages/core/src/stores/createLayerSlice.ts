import type { Layer, LayerConfig } from '../models/layer';
import type { SliceCreator } from './types';

/** State + actions for layer management. */
export interface LayerSlice {
  enabledLayers: string[];
  stashedLayers: string[] | null;
  layers: Record<string, Layer>;
  indicatorLayerIds: Set<string>;
  maxEntities: number;
  layerConfigs: Record<string, LayerConfig>;
  toggleLayer: (layerId: string) => void;
  clearAllLayers: () => void;
  toggleLayersVisibility: () => void;
  setLayers: (layers: Layer[]) => void;
  setMaxEntities: (max: number) => void;
  setLayerConfig: (layerId: string, config: LayerConfig) => void;
}

export const createLayerSlice: SliceCreator<LayerSlice> = (set) => ({
  enabledLayers: [],
  stashedLayers: null,
  layers: {},
  indicatorLayerIds: new Set<string>(),
  maxEntities: 2000,
  layerConfigs: {},
  toggleLayer: (layerId) =>
    set((state) => {
      const isEnabled = state.enabledLayers.includes(layerId);
      return {
        enabledLayers: isEnabled
          ? state.enabledLayers.filter((id) => id !== layerId)
          : [...state.enabledLayers, layerId],
        stashedLayers: null,
      };
    }),
  clearAllLayers: () => set({ enabledLayers: [], stashedLayers: null }),
  toggleLayersVisibility: () =>
    set((state) => {
      if (state.stashedLayers) {
        return { enabledLayers: state.stashedLayers, stashedLayers: null };
      }
      return { stashedLayers: [...state.enabledLayers], enabledLayers: [] };
    }),
  setLayers: (layerList) =>
    set(() => {
      const record: Record<string, Layer> = {};
      const indicatorIds = new Set<string>();
      for (const layer of layerList) {
        record[layer.type] = layer;
        record[layer.id] = layer;
        if (layer.renderingMode === 'indicator') {
          indicatorIds.add(layer.id);
          indicatorIds.add(layer.type);
        }
      }
      return { layers: record, indicatorLayerIds: indicatorIds };
    }),
  setMaxEntities: (max) => set({ maxEntities: max }),
  setLayerConfig: (layerId, config) =>
    set((state) => ({
      layerConfigs: {
        ...state.layerConfigs,
        [layerId]: { ...state.layerConfigs[layerId], ...config },
      },
    })),
});
