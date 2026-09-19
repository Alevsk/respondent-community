import { describe, it, expect } from 'vitest';

describe('core utils', () => {
  it('exports formatRelativeTime', async () => {
    const utils = await import('./formatTime');
    expect(typeof utils.formatRelativeTime).toBe('function');
  });

  it('formats recent timestamps', async () => {
    const { formatRelativeTime } = await import('./formatTime');
    expect(formatRelativeTime(Date.now())).toBe('just now');
  });

  it('exports formatUtcTime', async () => {
    const { formatUtcTime } = await import('./formatTime');
    expect(formatUtcTime(0)).toBe('—');
  });
});
