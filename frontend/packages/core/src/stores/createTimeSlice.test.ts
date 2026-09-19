import { describe, it, expect } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createTimeSlice, type TimeSlice } from './createTimeSlice';

const makeStore = () => createStore<TimeSlice>()(createTimeSlice);

describe('createTimeSlice', () => {
  it('defaults to live mode', () => {
    const store = makeStore();
    const s = store.getState();
    expect(s.timeMode).toBe('live');
    expect(s.timeFrom).toBeNull();
    expect(s.timeTo).toBeNull();
    expect(s.timePreset).toBeNull();
  });

  it('setTimePreset("1h") switches to range mode', () => {
    const store = makeStore();
    store.getState().setTimePreset('1h');
    const s = store.getState();
    expect(s.timeMode).toBe('range');
    expect(s.timePreset).toBe('1h');
    expect(s.timeFrom).toBeTruthy();
    expect(s.timeTo).toBeNull();
  });

  it('setTimePreset("custom") is a no-op', () => {
    const store = makeStore();
    store.getState().setTimePreset('custom');
    const s = store.getState();
    expect(s.timeMode).toBe('live');
    expect(s.timePreset).toBeNull();
  });

  it('setCustomTimeRange sets explicit from/to', () => {
    const store = makeStore();
    store
      .getState()
      .setCustomTimeRange({ from: '2024-01-01T00:00:00Z', to: '2024-01-02T00:00:00Z' });
    const s = store.getState();
    expect(s.timeMode).toBe('range');
    expect(s.timeFrom).toBe('2024-01-01T00:00:00Z');
    expect(s.timeTo).toBe('2024-01-02T00:00:00Z');
    expect(s.timePreset).toBe('custom');
  });

  it('returnToLive resets to live mode', () => {
    const store = makeStore();
    store.getState().setTimePreset('8h');
    store.getState().returnToLive();
    const s = store.getState();
    expect(s.timeMode).toBe('live');
    expect(s.timeFrom).toBeNull();
    expect(s.timeTo).toBeNull();
    expect(s.timePreset).toBeNull();
  });
});
