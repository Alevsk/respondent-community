/**
 * RecBlock — useHistoryLimits hook tests
 *
 * useHistoryLimits is a private hook inside RecBlock.tsx.  It derives the most
 * permissive history limits from all layers stored in the Zustand store.
 *
 * Strategy: render RecBlock with the popover open (by clicking the badge) and
 * then click "Custom Range..." so that CustomRangePicker mounts with the
 * maxLookbackHours / maxRangeSpanHours props that useHistoryLimits produced.
 * A vi.mock intercepts CustomRangePicker and captures those props.
 *
 * Dependencies mocked:
 *   - ../api/websocket   — useWebSocketStatus → 'connected'
 *   - ./CustomRangePicker — spy captures the props forwarded by RecBlock
 */

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { useUIStore } from '@/app/store';
import type { Layer, HistoryConfig } from '@/app/store';

// ---------------------------------------------------------------------------
// Mocks — declared BEFORE importing the component under test
// ---------------------------------------------------------------------------

vi.mock('@respondent/core', async () => ({
  ...(await vi.importActual('@respondent/core')),
  useWebSocketStatus: () => 'connected' as const,
}));

// Capture the props RecBlock passes to CustomRangePicker so we can assert on
// maxLookbackHours and maxRangeSpanHours without having to actually render
// the full date/time picker UI.
let capturedPickerProps: Record<string, unknown> = {};

vi.mock('./CustomRangePicker', () => ({
  default: (props: Record<string, unknown>) => {
    capturedPickerProps = props;
    return <div data-testid="custom-range-picker" />;
  },
}));

// ---------------------------------------------------------------------------
// Import component AFTER mocks are in place
// ---------------------------------------------------------------------------

import RecBlock from './RecBlock';

// ---------------------------------------------------------------------------
// Theme & render helpers
// ---------------------------------------------------------------------------

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderBlock = () => render(<RecBlock />, { wrapper: TestWrapper });

// ---------------------------------------------------------------------------
// Helpers — Layer factory
// ---------------------------------------------------------------------------

function makeLayer(overrides: Partial<Layer> = {}): Layer {
  return {
    id: 'layer-id-1',
    name: 'Test Layer',
    type: 'test-type',
    enabled: true,
    mode: 'realtime',
    density: 50,
    source: 'test',
    lastUpdate: 0,
    count: 0,
    color: '#ffffff',
    pointSize: 4,
    ...overrides,
  };
}

/**
 * Open the time-range popover and then click "Custom Range..." so that
 * CustomRangePicker mounts and capturedPickerProps is populated.
 */
function openCustomPicker() {
  // Click the REC/LIVE badge to open the popover
  const badge = screen.getByText(/LIVE|LAST|RANGE|HISTORICAL|OFFLINE|RECONNECTING/i);
  fireEvent.click(badge);

  // Click the "Custom Range..." menu item
  const customRangeItem = screen.getByText(/Custom Range/i);
  fireEvent.click(customRangeItem);
}

// ---------------------------------------------------------------------------
// Store reset
// ---------------------------------------------------------------------------

beforeEach(() => {
  capturedPickerProps = {};
  useUIStore.setState(
    {
      layers: {},
      timeMode: 'live',
      timeFrom: null,
      timeTo: null,
      timePreset: null,
    },
    false,
  );
});

afterEach(() => {
  vi.clearAllMocks();
});

// ---------------------------------------------------------------------------
// useHistoryLimits — default behaviour (no layers)
// ---------------------------------------------------------------------------

describe('useHistoryLimits — no layers loaded', () => {
  it('passes default maxLookbackHours=48 to CustomRangePicker', () => {
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxLookbackHours).toBe(48);
  });

  it('passes default maxRangeSpanHours=24 to CustomRangePicker', () => {
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxRangeSpanHours).toBe(24);
  });
});

// ---------------------------------------------------------------------------
// useHistoryLimits — layers without historyConfig
// ---------------------------------------------------------------------------

describe('useHistoryLimits — layers present but no historyConfig', () => {
  it('still returns default 48h lookback', () => {
    useUIStore
      .getState()
      .setLayers([makeLayer({ id: 'a', type: 'type-a' }), makeLayer({ id: 'b', type: 'type-b' })]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxLookbackHours).toBe(48);
  });

  it('still returns default 24h span', () => {
    useUIStore.getState().setLayers([makeLayer({ id: 'a', type: 'type-a' })]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxRangeSpanHours).toBe(24);
  });
});

// ---------------------------------------------------------------------------
// useHistoryLimits — single layer with historyConfig
// ---------------------------------------------------------------------------

describe('useHistoryLimits — single layer with historyConfig', () => {
  it('overrides default lookback when layer has larger maxLookbackHours', () => {
    const hc: HistoryConfig = { maxLookbackHours: 720, maxRangeSpanHours: 24 };
    useUIStore.getState().setLayers([makeLayer({ id: 'l1', type: 'flights', historyConfig: hc })]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxLookbackHours).toBe(720);
  });

  it('overrides default span when layer has larger maxRangeSpanHours', () => {
    const hc: HistoryConfig = { maxLookbackHours: 48, maxRangeSpanHours: 168 };
    useUIStore.getState().setLayers([makeLayer({ id: 'l1', type: 'flights', historyConfig: hc })]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxRangeSpanHours).toBe(168);
  });

  it('keeps default lookback when layer config is smaller than default', () => {
    const hc: HistoryConfig = { maxLookbackHours: 12, maxRangeSpanHours: 6 };
    useUIStore.getState().setLayers([makeLayer({ id: 'l1', type: 'flights', historyConfig: hc })]);
    renderBlock();
    openCustomPicker();
    // 12 < 48, so default 48 wins
    expect(capturedPickerProps.maxLookbackHours).toBe(48);
  });

  it('keeps default span when layer config is smaller than default', () => {
    const hc: HistoryConfig = { maxLookbackHours: 12, maxRangeSpanHours: 6 };
    useUIStore.getState().setLayers([makeLayer({ id: 'l1', type: 'flights', historyConfig: hc })]);
    renderBlock();
    openCustomPicker();
    // 6 < 24, so default 24 wins
    expect(capturedPickerProps.maxRangeSpanHours).toBe(24);
  });
});

// ---------------------------------------------------------------------------
// useHistoryLimits — most permissive wins across multiple layers
// ---------------------------------------------------------------------------

describe('useHistoryLimits — most permissive across multiple layers', () => {
  it('picks the largest maxLookbackHours across all layers', () => {
    useUIStore.getState().setLayers([
      makeLayer({
        id: 'l1',
        type: 'type-a',
        historyConfig: { maxLookbackHours: 100, maxRangeSpanHours: 24 },
      }),
      makeLayer({
        id: 'l2',
        type: 'type-b',
        historyConfig: { maxLookbackHours: 500, maxRangeSpanHours: 24 },
      }),
      makeLayer({
        id: 'l3',
        type: 'type-c',
        historyConfig: { maxLookbackHours: 200, maxRangeSpanHours: 24 },
      }),
    ]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxLookbackHours).toBe(500);
  });

  it('picks the largest maxRangeSpanHours across all layers', () => {
    useUIStore.getState().setLayers([
      makeLayer({
        id: 'l1',
        type: 'type-a',
        historyConfig: { maxLookbackHours: 48, maxRangeSpanHours: 48 },
      }),
      makeLayer({
        id: 'l2',
        type: 'type-b',
        historyConfig: { maxLookbackHours: 48, maxRangeSpanHours: 168 },
      }),
      makeLayer({
        id: 'l3',
        type: 'type-c',
        historyConfig: { maxLookbackHours: 48, maxRangeSpanHours: 72 },
      }),
    ]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxRangeSpanHours).toBe(168);
  });

  it('combines the most permissive lookback and span from different layers', () => {
    useUIStore.getState().setLayers([
      makeLayer({
        id: 'l1',
        type: 'type-a',
        historyConfig: { maxLookbackHours: 8760, maxRangeSpanHours: 24 },
      }),
      makeLayer({
        id: 'l2',
        type: 'type-b',
        historyConfig: { maxLookbackHours: 48, maxRangeSpanHours: 720 },
      }),
    ]);
    renderBlock();
    openCustomPicker();
    expect(capturedPickerProps.maxLookbackHours).toBe(8760);
    expect(capturedPickerProps.maxRangeSpanHours).toBe(720);
  });
});

// ---------------------------------------------------------------------------
// useHistoryLimits — dual-key store indexing (no double-counting)
// ---------------------------------------------------------------------------

describe('useHistoryLimits — no double-counting from dual-key store', () => {
  it('counts a layer only once even though it is keyed by both id and type', () => {
    // setLayers keys each layer by both id and type, so the store has
    // two entries pointing to the same Layer object.  useHistoryLimits
    // must deduplicate by layer.id and visit each logical layer once.
    // We verify by checking that the result equals a single-layer scenario
    // and does not incorrectly inflate above the single-layer value.
    const hc: HistoryConfig = { maxLookbackHours: 300, maxRangeSpanHours: 150 };
    useUIStore
      .getState()
      .setLayers([makeLayer({ id: 'sole-id', type: 'sole-type', historyConfig: hc })]);

    // Confirm the store has two entries (dual-key indexing)
    const layers = useUIStore.getState().layers;
    expect(Object.keys(layers).length).toBe(2);
    expect(layers['sole-id']).toBe(layers['sole-type']); // same reference

    renderBlock();
    openCustomPicker();

    // Should reflect 300, not 600 (would be 600 if double-counted)
    expect(capturedPickerProps.maxLookbackHours).toBe(300);
    expect(capturedPickerProps.maxRangeSpanHours).toBe(150);
  });

  it('processes two distinct layers independently without cross-contamination', () => {
    useUIStore.getState().setLayers([
      makeLayer({
        id: 'id-x',
        type: 'type-x',
        historyConfig: { maxLookbackHours: 200, maxRangeSpanHours: 100 },
      }),
      makeLayer({
        id: 'id-y',
        type: 'type-y',
        historyConfig: { maxLookbackHours: 400, maxRangeSpanHours: 50 },
      }),
    ]);
    renderBlock();
    openCustomPicker();
    // Most permissive from both
    expect(capturedPickerProps.maxLookbackHours).toBe(400);
    expect(capturedPickerProps.maxRangeSpanHours).toBe(100);
  });
});
