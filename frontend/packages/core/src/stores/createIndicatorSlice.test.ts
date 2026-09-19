import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createIndicatorSlice, type IndicatorSlice } from './createIndicatorSlice';
import type { IndicatorSnapshot } from '../models/indicator';

const makeStore = () => createStore<IndicatorSlice>()(createIndicatorSlice);

const mockSnapshot: IndicatorSnapshot = {
  layerId: 'threat-index',
  layerName: 'Threat Index',
  timestampMs: Date.now(),
  values: [{ key: 'level', value: '3', label: 'Level', unit: '', level: 3 }],
  summary: 'Elevated threat level',
  overallLevel: 3,
};

describe('createIndicatorSlice', () => {
  it('defaults to empty indicators', () => {
    const store = makeStore();
    expect(store.getState().indicators).toEqual({});
  });

  it('setIndicator stores a snapshot by layerId', () => {
    const store = makeStore();
    store.getState().setIndicator('threat-index', mockSnapshot);
    expect(store.getState().indicators['threat-index']).toEqual(mockSnapshot);
  });

  it('setIndicator replaces an existing snapshot', () => {
    const store = makeStore();
    store.getState().setIndicator('threat-index', mockSnapshot);
    const updated = { ...mockSnapshot, overallLevel: 5 };
    store.getState().setIndicator('threat-index', updated);
    expect(store.getState().indicators['threat-index'].overallLevel).toBe(5);
  });

  it('setIndicator preserves other layer snapshots', () => {
    const store = makeStore();
    store.getState().setIndicator('layer-a', mockSnapshot);
    const snapshotB = { ...mockSnapshot, layerId: 'layer-b' };
    store.getState().setIndicator('layer-b', snapshotB);
    expect(store.getState().indicators['layer-a']).toEqual(mockSnapshot);
    expect(store.getState().indicators['layer-b']).toEqual(snapshotB);
  });
});
