import { describe, it, expect } from 'vitest';
import { WS_TIMEOUT_MS, STEP_DEFS, ATTENTION_ENUM } from './constants';

describe('bootstrap constants', () => {
  it('WS_TIMEOUT_MS is 10 seconds', () => {
    expect(WS_TIMEOUT_MS).toBe(10_000);
  });

  it('STEP_DEFS has 3 steps with correct keys', () => {
    expect(STEP_DEFS).toHaveLength(3);
    expect(STEP_DEFS.map((s) => s.key)).toEqual(['layers', 'notifications', 'websocket']);
  });

  it('STEP_DEFS labels match spec', () => {
    expect(STEP_DEFS.map((s) => s.label)).toEqual([
      'LOADING LAYERS...',
      'LOADING NOTIFICATIONS...',
      'CONNECTING STREAM...',
    ]);
  });

  it('ATTENTION_ENUM maps severity strings to numbers', () => {
    expect(ATTENTION_ENUM).toEqual({
      info: 1,
      low: 2,
      medium: 3,
      high: 4,
      critical: 5,
    });
  });
});
