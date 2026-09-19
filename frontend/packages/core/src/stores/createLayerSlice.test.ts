import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createLayerSlice, type LayerSlice } from './createLayerSlice';
import type { Layer } from '../models/layer';

const makeStore = () => createStore<LayerSlice>()(createLayerSlice);

const mockLayer: Layer = {
  id: 'layer-1',
  name: 'Test Layer',
  type: 'flights',
  enabled: true,
  mode: 'live',
  density: 50,
  source: 'api',
  lastUpdate: Date.now(),
  count: 100,
  color: '#ff0000',
  pointSize: 4,
};

describe('createLayerSlice', () => {
  it('defaults to empty enabled layers', () => {
    const store = makeStore();
    expect(store.getState().enabledLayers).toEqual([]);
    expect(store.getState().stashedLayers).toBeNull();
  });

  it('toggleLayer enables a layer', () => {
    const store = makeStore();
    store.getState().toggleLayer('flights');
    expect(store.getState().enabledLayers).toEqual(['flights']);
  });

  it('toggleLayer disables an enabled layer', () => {
    const store = makeStore();
    store.getState().toggleLayer('flights');
    store.getState().toggleLayer('flights');
    expect(store.getState().enabledLayers).toEqual([]);
  });

  it('toggleLayer clears stashed layers', () => {
    const store = makeStore();
    store.getState().toggleLayer('flights');
    store.getState().toggleLayersVisibility(); // stash
    store.getState().toggleLayer('ships');
    expect(store.getState().stashedLayers).toBeNull();
  });

  it('clearAllLayers empties enabled and stashed', () => {
    const store = makeStore();
    store.getState().toggleLayer('flights');
    store.getState().clearAllLayers();
    expect(store.getState().enabledLayers).toEqual([]);
    expect(store.getState().stashedLayers).toBeNull();
  });

  it('toggleLayersVisibility stashes and restores', () => {
    const store = makeStore();
    store.getState().toggleLayer('flights');
    store.getState().toggleLayer('ships');
    // Stash
    store.getState().toggleLayersVisibility();
    expect(store.getState().enabledLayers).toEqual([]);
    expect(store.getState().stashedLayers).toEqual(['flights', 'ships']);
    // Restore
    store.getState().toggleLayersVisibility();
    expect(store.getState().enabledLayers).toEqual(['flights', 'ships']);
    expect(store.getState().stashedLayers).toBeNull();
  });

  it('setLayers populates layers record and indicator set', () => {
    const store = makeStore();
    const indicator: Layer = {
      ...mockLayer,
      id: 'ind-1',
      type: 'threat',
      renderingMode: 'indicator',
    };
    store.getState().setLayers([mockLayer, indicator]);
    const s = store.getState();
    expect(s.layers['flights']).toEqual(mockLayer);
    expect(s.layers['layer-1']).toEqual(mockLayer);
    expect(s.indicatorLayerIds.has('ind-1')).toBe(true);
    expect(s.indicatorLayerIds.has('threat')).toBe(true);
    expect(s.indicatorLayerIds.has('flights')).toBe(false);
  });

  it('setMaxEntities updates max', () => {
    const store = makeStore();
    store.getState().setMaxEntities(5000);
    expect(store.getState().maxEntities).toBe(5000);
  });

  it('setLayerConfig merges config for a layer', () => {
    const store = makeStore();
    store.getState().setLayerConfig('flights', { showTrails: true });
    store.getState().setLayerConfig('flights', { color: '#00ff00' });
    expect(store.getState().layerConfigs['flights']).toEqual({
      showTrails: true,
      color: '#00ff00',
    });
  });
});
