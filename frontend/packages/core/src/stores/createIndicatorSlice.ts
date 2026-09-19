import type { IndicatorSnapshot } from '../models/indicator';
import type { SliceCreator } from './types';

/** State + actions for indicator snapshots. */
export interface IndicatorSlice {
  indicators: Record<string, IndicatorSnapshot>;
  setIndicator: (layerId: string, snapshot: IndicatorSnapshot) => void;
}

export const createIndicatorSlice: SliceCreator<IndicatorSlice> = (set) => ({
  indicators: {},
  setIndicator: (layerId, snapshot) =>
    set((state) => ({
      indicators: { ...state.indicators, [layerId]: snapshot },
    })),
});
