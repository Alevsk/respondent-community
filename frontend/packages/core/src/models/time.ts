// Time range control types
export type TimeMode = 'live' | 'range';
export type TimePreset = '1h' | '8h' | '24h' | 'custom';

export interface TimeRange {
  from: string; // ISO 8601
  to: string; // ISO 8601
}

export const TIME_PRESETS: TimePreset[] = ['1h', '8h', '24h', 'custom'];
