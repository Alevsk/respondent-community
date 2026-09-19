import { describe, it, expect, vi, afterEach } from 'vitest';
import {
  formatRelativeTime,
  formatAbsoluteTime,
  formatUtcTime,
  formatUtcDate,
} from '@respondent/core';

describe('formatRelativeTime', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockNow(offsetMs: number, baseTs = 1710000000000) {
    vi.spyOn(Date, 'now').mockReturnValue(baseTs + offsetMs);
    return baseTs;
  }

  it('returns "just now" for timestamps < 5 seconds ago', () => {
    const ts = mockNow(0);
    expect(formatRelativeTime(ts)).toBe('just now');

    const ts2 = mockNow(4000);
    expect(formatRelativeTime(ts2)).toBe('just now');
  });

  it('returns seconds for 5s–59s', () => {
    const ts = mockNow(5000);
    expect(formatRelativeTime(ts)).toBe('5s ago');

    const ts2 = mockNow(59000);
    expect(formatRelativeTime(ts2)).toBe('59s ago');
  });

  it('returns minutes for 1m–59m', () => {
    const ts = mockNow(60_000);
    expect(formatRelativeTime(ts)).toBe('1m ago');

    const ts2 = mockNow(59 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('59m ago');
  });

  it('returns hours for 1h–23h', () => {
    const ts = mockNow(60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('1h ago');

    const ts2 = mockNow(23 * 60 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('23h ago');
  });

  it('returns "yesterday" for exactly 1 day', () => {
    const ts = mockNow(24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('yesterday');
  });

  it('returns days for 2–6 days', () => {
    const ts = mockNow(2 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('2 days ago');

    const ts2 = mockNow(6 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('6 days ago');
  });

  it('returns weeks for 7–29 days', () => {
    const ts = mockNow(7 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('1 week ago');

    const ts2 = mockNow(14 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('2 weeks ago');

    const ts3 = mockNow(28 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts3)).toBe('4 weeks ago');

    // day 29 is still 4 weeks
    const ts4 = mockNow(29 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts4)).toBe('4 weeks ago');
  });

  it('transitions cleanly from weeks to months at day 30', () => {
    // day 30 should be "1 month ago" (not "4 weeks ago")
    const ts = mockNow(30 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('1 month ago');
  });

  it('returns months for 30–364 days', () => {
    const ts = mockNow(60 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('1 month ago');

    const ts2 = mockNow(90 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('2 months ago');

    const ts3 = mockNow(335 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts3)).toBe('11 months ago');
  });

  it('returns years for 365+ days', () => {
    const ts = mockNow(366 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts)).toBe('1 year ago');

    const ts2 = mockNow(730 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts2)).toBe('1 year ago');

    const ts3 = mockNow(800 * 24 * 60 * 60_000);
    expect(formatRelativeTime(ts3)).toBe('2 years ago');
  });

  it('returns "just now" for future timestamps (clock skew)', () => {
    const ts = mockNow(-5000);
    expect(formatRelativeTime(ts)).toBe('just now');
  });
});

describe('formatAbsoluteTime', () => {
  it('returns a locale-formatted string with timezone', () => {
    // Just verify it returns a non-empty string containing the year
    const result = formatAbsoluteTime(1710000000000);
    expect(result).toBeTruthy();
    expect(result).toContain('2024');
  });
});

describe('formatUtcTime', () => {
  it('formats as HH:mm:ssZ', () => {
    // 2024-03-09T16:00:00.000Z
    expect(formatUtcTime(1710000000000)).toBe('16:00:00Z');
  });

  it('returns dash for zero or negative', () => {
    expect(formatUtcTime(0)).toBe('—');
    expect(formatUtcTime(-1)).toBe('—');
  });
});

describe('formatUtcDate', () => {
  it('formats as YYYY-MM-DD', () => {
    expect(formatUtcDate(1710000000000)).toBe('2024-03-09');
  });

  it('returns dash for zero or negative', () => {
    expect(formatUtcDate(0)).toBe('—');
    expect(formatUtcDate(-1)).toBe('—');
  });
});
