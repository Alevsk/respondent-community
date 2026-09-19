import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createGlobeDisplaySlice, type GlobeDisplaySlice } from './createGlobeDisplaySlice';

const makeStore = () => createStore<GlobeDisplaySlice>()(createGlobeDisplaySlice);

describe('createGlobeDisplaySlice', () => {
  it('has correct defaults', () => {
    const store = makeStore();
    const s = store.getState();
    expect(s.showOccluded).toBe(true);
    expect(s.showGeoLabels).toBe(true);
    expect(s.show3DBuildings).toBe(false);
    expect(s.smoothMotion).toBe(true);
    expect(s.cinematicDrift).toBe(false);
    expect(s.spatialAggregation).toBe(false);
  });

  it('setShowOccluded toggles occluded entities', () => {
    const store = makeStore();
    store.getState().setShowOccluded(false);
    expect(store.getState().showOccluded).toBe(false);
  });

  it('setShowGeoLabels toggles geo labels', () => {
    const store = makeStore();
    store.getState().setShowGeoLabels(false);
    expect(store.getState().showGeoLabels).toBe(false);
  });

  it('setShow3DBuildings toggles 3D buildings', () => {
    const store = makeStore();
    store.getState().setShow3DBuildings(true);
    expect(store.getState().show3DBuildings).toBe(true);
  });

  it('setSmoothMotion toggles predictive interpolation', () => {
    const store = makeStore();
    store.getState().setSmoothMotion(false);
    expect(store.getState().smoothMotion).toBe(false);
  });

  it('setCinematicDrift toggles cinematic drift', () => {
    const store = makeStore();
    store.getState().setCinematicDrift(true);
    expect(store.getState().cinematicDrift).toBe(true);
  });

  it('setSpatialAggregation toggles spatial aggregation', () => {
    const store = makeStore();
    store.getState().setSpatialAggregation(true);
    expect(store.getState().spatialAggregation).toBe(true);
  });
});
