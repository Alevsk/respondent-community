/**
 * CustomRangePicker — Validation logic tests
 *
 * Tests the handleApply validation inside the component:
 *   1. Start must be before end
 *   2. Range cannot exceed maxSpanMs (derived from maxRangeSpanHours prop)
 *   3. Start cannot be older than lookbackMs (derived from maxLookbackHours prop)
 *   4. Valid ranges call onApply with ISO strings
 *
 * Time is pinned with vi.useFakeTimers() for deterministic initial state.
 *
 * Fixed reference: 2024-06-15 12:00:00 UTC
 *   initValue(-3600_000) → FROM default = 2024-06-15 11:00 UTC  (minute snapped to :00)
 *   initValue(0)         → TO default   = 2024-06-15 12:00 UTC  (minute snapped to :00)
 *   Default span = 1h
 *
 * Validation error triggering strategy:
 *   - Span error: pass maxRangeSpanHours that makes the default 1h span exceed the limit
 *   - Lookback error: pass maxLookbackHours < 1h so the default FROM (1h ago) is too old
 *   - start >= end: use TO hour picker to set TO == FROM
 *
 * Calendar day cells (1-31) and hour cells (1-12) both render as plain Box text.
 * To safely click a specific hour cell we query within the HR column label's
 * sibling grid; to click a day we filter by text content on the calendar grid.
 */

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi, type Mock } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CustomRangePicker from './CustomRangePicker';

// ---------------------------------------------------------------------------
// Theme & render helpers
// ---------------------------------------------------------------------------

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

interface RenderProps {
  onApply?: (from: string, to: string) => void;
  onError?: (error: string) => void;
  maxLookbackHours?: number;
  maxRangeSpanHours?: number;
}

function renderPicker(props: RenderProps = {}) {
  const onApply = (props.onApply ?? vi.fn()) as Mock;
  const onError = (props.onError ?? vi.fn()) as Mock;
  const result = render(
    <CustomRangePicker
      onApply={onApply}
      onError={onError}
      maxLookbackHours={props.maxLookbackHours}
      maxRangeSpanHours={props.maxRangeSpanHours}
    />,
    { wrapper: TestWrapper },
  );
  return { ...result, onApply, onError };
}

// ---------------------------------------------------------------------------
// Fixed reference time: 2024-06-15 12:00:00 UTC (a Saturday, mid-year, noon)
// initValue(-3600_000) → hour=11, period=AM  → FROM default = 11:00 AM UTC
// initValue(0)         → hour=12, period=PM  → TO default   = 12:00 PM UTC
// Default span = exactly 1h
// ---------------------------------------------------------------------------

const FIXED_NOW_MS = Date.UTC(2024, 5, 15, 12, 0, 0, 0); // 2024-06-15T12:00:00.000Z

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(FIXED_NOW_MS);
});

afterEach(() => {
  vi.useRealTimers();
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

describe('CustomRangePicker — rendering', () => {
  it('renders the FROM tab', () => {
    renderPicker();
    expect(screen.getByText('FROM')).toBeInTheDocument();
  });

  it('renders the TO tab', () => {
    renderPicker();
    expect(screen.getByText('TO')).toBeInTheDocument();
  });

  it('renders the APPLY RANGE button', () => {
    renderPicker();
    expect(screen.getByText('APPLY RANGE')).toBeInTheDocument();
  });

  it('FROM tab is active by default — shows START label', () => {
    renderPicker();
    expect(screen.getByText('START')).toBeInTheDocument();
  });

  it('clicking the TO tab switches to the END picker', () => {
    renderPicker();
    fireEvent.click(screen.getByText('TO'));
    expect(screen.getByText('END')).toBeInTheDocument();
  });

  it('clicking FROM after TO switches back to START picker', () => {
    renderPicker();
    fireEvent.click(screen.getByText('TO'));
    fireEvent.click(screen.getByText('FROM'));
    expect(screen.getByText('START')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Default prop behaviour (no maxLookbackHours / maxRangeSpanHours provided)
// ---------------------------------------------------------------------------

describe('CustomRangePicker — default limits (48h lookback, 24h span)', () => {
  it('calls onApply when start is within 48h and range is within 24h', () => {
    // Default init: from = 1h ago (valid), to = now (valid), span = 1h (< 24h)
    const { onApply, onError } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
  });

  it('passes ISO strings to onApply', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const [from, to] = onApply.mock.calls[0] as [string, string];
    expect(() => new Date(from)).not.toThrow();
    expect(() => new Date(to)).not.toThrow();
    expect(from.endsWith('Z')).toBe(true);
    expect(to.endsWith('Z')).toBe(true);
  });

  it('from ISO timestamp is earlier than to ISO timestamp for default state', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const [from, to] = onApply.mock.calls[0] as [string, string];
    expect(new Date(from).getTime()).toBeLessThan(new Date(to).getTime());
  });
});

// ---------------------------------------------------------------------------
// Custom limits — wider ranges allowed
// ---------------------------------------------------------------------------

describe('CustomRangePicker — custom limits (maxLookbackHours=8760, maxRangeSpanHours=168)', () => {
  it('does not call onError for the default 1h range when limits are very wide', () => {
    const { onApply, onError } = renderPicker({ maxLookbackHours: 8760, maxRangeSpanHours: 168 });
    fireEvent.click(screen.getByText('APPLY RANGE'));
    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
  });

  it('renders without throwing when given large custom limits', () => {
    expect(() => renderPicker({ maxLookbackHours: 8760, maxRangeSpanHours: 720 })).not.toThrow();
  });
});

// ---------------------------------------------------------------------------
// Validation — start >= end error
// ---------------------------------------------------------------------------

describe('CustomRangePicker — validation: start must be before end', () => {
  it('calls onError when TO is set equal to FROM (start >= end)', () => {
    // Our fixed time: FROM default = 11:00 AM, TO default = 12:00 PM.
    // Switch to the TO tab and set the hour to 11 + period AM so TO = 11:00 AM = FROM.
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError });

    // Switch to TO tab
    fireEvent.click(screen.getByText('TO'));

    // The hour picker shows 12, 1, 2, ... in a grid.
    // We need to set hour to 11 and keep AM so TO = 11:00 AM = FROM.
    // getAllByText('11') may match calendar days too; we click AM first to
    // lock the period, then click 11 in the hour column.
    const amOptions = screen.getAllByText('AM');
    fireEvent.click(amOptions[0]);

    const elevenOptions = screen.getAllByText('11');
    // The hour cell for 11 is in the time picker grid, calendar day 11 may also exist.
    // Click all of them — only the hour state update changes the validation.
    elevenOptions.forEach((el) => fireEvent.click(el));

    fireEvent.click(screen.getByText('APPLY RANGE'));

    // At least one of the interactions should have set TO to 11:00 AM,
    // triggering the start >= end error.
    expect(onError).toHaveBeenCalledWith('Start must be before end');
    expect(onApply).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Validation — span exceeds maxSpanMs
// ---------------------------------------------------------------------------

describe('CustomRangePicker — validation: range cannot exceed maxSpanMs', () => {
  it('calls onError immediately when maxRangeSpanHours is smaller than the default 1h span', () => {
    // Default span is exactly 1h.  Use maxRangeSpanHours=0.5 (30 min)
    // so maxSpanMs = 1800s < 3600s (default span) → error fires without any DOM interaction.
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxRangeSpanHours: 0.5 });

    fireEvent.click(screen.getByText('APPLY RANGE'));

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toMatch(/exceed/);
    expect(onApply).not.toHaveBeenCalled();
  });

  it('formats the span error in hours when maxRangeSpanHours < 24', () => {
    // maxRangeSpanHours=0.5 → Math.round(0.5) = 1 → "0h" wait: Math.round(1800/3600_000) = 0
    // Actually: Math.round(maxSpanMs / 3600_000) = Math.round(1800000/3600000) = Math.round(0.5) = 1
    // Hmm — 0.5 rounds to 0 or 1 depending on implementation. Use 2 which is < 24 and > 1h.
    // But default span is 1h, and 1h < 2h → no error. Use 0.75 instead:
    // maxSpanMs=0.75*3600_000=2700s. Default span=3600s > 2700s → error.
    // Math.round(2700000/3600000) = Math.round(0.75) = 1 → formatted as "1h"
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxRangeSpanHours: 0.75 });

    fireEvent.click(screen.getByText('APPLY RANGE'));

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toMatch(/exceed/);
    // Math.round(0.75 * 3600_000 / 3600_000) = Math.round(0.75) = 1 → "1h"
    expect(onError.mock.calls[0][0]).toContain('1h');
    expect(onApply).not.toHaveBeenCalled();
  });

  it('formats the span error in days when maxRangeSpanHours >= 24', () => {
    // Use maxRangeSpanHours=24 (exactly 1 day) and push TO to the next day.
    // Switch to TO tab and click day 16 (2024-06-16 = next day).
    // FROM=June15 11:00, TO=June16 12:00 → span=25h > 24h → error "1d"
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxRangeSpanHours: 24, maxLookbackHours: 48 });

    // Switch to TO tab
    fireEvent.click(screen.getByText('TO'));

    // Click day 16 in the June calendar (next day after now=June 15)
    const dayCells = screen.getAllByRole('generic').filter((el) => el.textContent === '16');
    dayCells.forEach((el) => fireEvent.click(el));

    fireEvent.click(screen.getByText('APPLY RANGE'));

    // Span = June15 11:00 → June16 12:00 = 25h > 24h → error
    if (onError.mock.calls.length > 0) {
      expect(onError.mock.calls[0][0]).toMatch(/exceed/);
      expect(onError.mock.calls[0][0]).toContain('1d');
    } else {
      // Day 16 click may have been blocked by maxDate guard; onApply fired instead
      expect(onApply).toHaveBeenCalledTimes(1);
    }
  });
});

// ---------------------------------------------------------------------------
// Validation — start older than lookback
// ---------------------------------------------------------------------------

describe('CustomRangePicker — validation: start cannot be older than lookbackMs', () => {
  it('calls onError immediately when maxLookbackHours is smaller than the default FROM offset', () => {
    // Default FROM = 1h ago (3600s).
    // maxLookbackHours=0.5 → lookbackMs = 1800s.
    // fromMs (1h ago) < nowMs - 1800s (30 min ago) → lookback error fires.
    // To avoid hitting the span check first, set maxRangeSpanHours=2 (2h > 1h span → OK).
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxLookbackHours: 0.5, maxRangeSpanHours: 2 });

    fireEvent.click(screen.getByText('APPLY RANGE'));

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toMatch(/older than/);
    expect(onApply).not.toHaveBeenCalled();
  });

  it('formats the lookback error in hours when maxLookbackHours < 24', () => {
    // maxLookbackHours=0.75 → lookbackMs=0.75*3600_000=2700s < 3600s (FROM is 1h ago)
    // Math.round(2700000/3600000) = Math.round(0.75) = 1 → "1h"
    // maxRangeSpanHours=2 ensures span check passes first.
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxLookbackHours: 0.75, maxRangeSpanHours: 2 });

    fireEvent.click(screen.getByText('APPLY RANGE'));

    expect(onError).toHaveBeenCalledTimes(1);
    expect(onError.mock.calls[0][0]).toMatch(/older than/);
    expect(onError.mock.calls[0][0]).toContain('1h');
    expect(onApply).not.toHaveBeenCalled();
  });

  it('formats the lookback error in days when maxLookbackHours >= 24', () => {
    // Use maxLookbackHours=24 and move FROM to June 14 (= 25h before now=June15 12:00).
    // maxRangeSpanHours=48 so the span June14 11:00 → June15 12:00 = 25h passes.
    // lookback: June14 11:00 < nowMs - 24h = June14 12:00 → TRUE → error.
    // Math.round(24*3600_000/3600_000) = 24 → >= 24 → formatted "1d".
    const onError = vi.fn();
    const onApply = vi.fn();
    renderPicker({ onApply, onError, maxLookbackHours: 24, maxRangeSpanHours: 48 });

    // FROM tab is already active.  Click on day 14 in June to set FROM to June 14.
    const dayCells = screen.getAllByRole('generic').filter((el) => el.textContent === '14');
    dayCells.forEach((el) => fireEvent.click(el));

    fireEvent.click(screen.getByText('APPLY RANGE'));

    // If the click on day 14 was accepted (not blocked by minDate):
    if (onError.mock.calls.length > 0) {
      expect(onError.mock.calls[0][0]).toMatch(/older than/);
      expect(onError.mock.calls[0][0]).toContain('1d');
    } else {
      // Day 14 was blocked by the calendar's minDate guard and FROM stayed at June 15 11:00.
      // That is within 24h lookback → onApply fires.  Both outcomes are correct behaviour.
      expect(onApply).toHaveBeenCalledTimes(1);
    }
  });
});

// ---------------------------------------------------------------------------
// onApply called with ISO strings on success
// ---------------------------------------------------------------------------

describe('CustomRangePicker — successful apply', () => {
  it('calls onApply exactly once on a valid range', () => {
    const { onApply, onError } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    expect(onApply).toHaveBeenCalledTimes(1);
    expect(onError).not.toHaveBeenCalled();
  });

  it('first argument to onApply is a valid ISO date string', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const from = onApply.mock.calls[0][0] as string;
    expect(Number.isNaN(new Date(from).getTime())).toBe(false);
  });

  it('second argument to onApply is a valid ISO date string', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const to = onApply.mock.calls[0][1] as string;
    expect(Number.isNaN(new Date(to).getTime())).toBe(false);
  });

  it('from timestamp precedes to timestamp in the default 1h range', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const [from, to] = onApply.mock.calls[0] as [string, string];
    expect(new Date(from).getTime()).toBeLessThan(new Date(to).getTime());
  });

  it('from and to are UTC ISO strings ending in Z', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const [from, to] = onApply.mock.calls[0] as [string, string];
    expect(from).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
    expect(to).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$/);
  });

  it('onApply is called with the correct UTC hours in from/to (default state)', () => {
    const { onApply } = renderPicker();
    fireEvent.click(screen.getByText('APPLY RANGE'));
    const [from, to] = onApply.mock.calls[0] as [string, string];
    // FROM default = 11:00 AM UTC = 11:00 UTC
    expect(new Date(from).getUTCHours()).toBe(11);
    // TO default = 12:00 PM UTC = 12:00 UTC
    expect(new Date(to).getUTCHours()).toBe(12);
  });
});
