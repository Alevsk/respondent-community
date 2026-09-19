import type { TimeMode, TimePreset, TimeRange } from '../models/time';
import type { SliceCreator } from './types';

/** State + actions for time range control. */
export interface TimeSlice {
  timeMode: TimeMode;
  timeFrom: string | null;
  timeTo: string | null;
  timePreset: TimePreset | null;
  setTimePreset: (preset: TimePreset) => void;
  setCustomTimeRange: (range: TimeRange) => void;
  returnToLive: () => void;
}

export const createTimeSlice: SliceCreator<TimeSlice> = (set) => ({
  timeMode: 'live' as TimeMode,
  timeFrom: null,
  timeTo: null,
  timePreset: null,
  setTimePreset: (preset) =>
    set(() => {
      const now = new Date();
      let from: string;
      switch (preset) {
        case '1h':
          from = new Date(now.getTime() - 1 * 3600_000).toISOString();
          break;
        case '8h':
          from = new Date(now.getTime() - 8 * 3600_000).toISOString();
          break;
        case '24h':
          from = new Date(now.getTime() - 24 * 3600_000).toISOString();
          break;
        case 'custom':
          return {}; // no-op, custom uses setCustomTimeRange
      }
      return {
        timeMode: 'range' as TimeMode,
        timeFrom: from,
        timeTo: null,
        timePreset: preset,
      };
    }),
  setCustomTimeRange: (range) =>
    set({
      timeMode: 'range' as TimeMode,
      timeFrom: range.from,
      timeTo: range.to,
      timePreset: 'custom' as TimePreset,
    }),
  returnToLive: () =>
    set({
      timeMode: 'live' as TimeMode,
      timeFrom: null,
      timeTo: null,
      timePreset: null,
    }),
});
