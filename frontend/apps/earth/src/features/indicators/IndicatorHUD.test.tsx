import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import IndicatorHUD from './IndicatorHUD';
import { timeAgo } from './indicatorUtils';
import { useUIStore, IndicatorSnapshot, Layer } from '@/app/store';

// ─── Module mock for useResponsive ───────────────────────────────────────────
const mockUseResponsive = vi.fn(() => ({ isMobile: false, isTablet: false, isDesktop: true }));
vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@respondent/core');
  return { ...actual, useResponsive: () => mockUseResponsive() };
});

function makeSnapshot(overrides: Partial<IndicatorSnapshot> = {}): IndicatorSnapshot {
  return {
    layerId: 'space_weather',
    layerName: 'Space Weather',
    timestampMs: Date.now(),
    values: [
      { key: 'r_scale', value: '2', label: 'Radio Blackout', unit: '', level: 2 },
      { key: 's_scale', value: '0', label: 'Solar Radiation', unit: '', level: 0 },
      { key: 'g_scale', value: '0', label: 'Geomagnetic Storm', unit: '', level: 0 },
    ],
    summary: 'Moderate',
    overallLevel: 2,
    ...overrides,
  };
}

function makeLayer(overrides: Partial<Layer> = {}): Layer {
  return {
    id: 'space_weather',
    name: 'Space Weather',
    type: 'space_weather',
    enabled: true,
    mode: 'live',
    density: 100,
    source: 'feeder',
    lastUpdate: Date.now(),
    count: 1,
    color: '#ffcc00',
    pointSize: 1,
    renderingMode: 'indicator',
    ...overrides,
  };
}

function setIndicatorState(overrides: Record<string, unknown> = {}) {
  useUIStore.setState({
    indicators: { space_weather: makeSnapshot() },
    enabledLayers: ['space_weather'],
    layers: { space_weather: makeLayer() },
    indicatorLayerIds: new Set(['space_weather']),
    ...overrides,
  });
}

describe('timeAgo', () => {
  it('returns "just now" for timestamps within 60s', () => {
    expect(timeAgo(Date.now() - 30_000)).toBe('just now');
    expect(timeAgo(Date.now())).toBe('just now');
  });

  it('returns minutes for timestamps under 1h', () => {
    expect(timeAgo(Date.now() - 5 * 60_000)).toBe('5m ago');
    expect(timeAgo(Date.now() - 59 * 60_000)).toBe('59m ago');
  });

  it('returns hours for timestamps over 1h', () => {
    expect(timeAgo(Date.now() - 90 * 60_000)).toBe('1h ago');
    expect(timeAgo(Date.now() - 3 * 60 * 60_000)).toBe('3h ago');
  });
});

describe('IndicatorHUD (desktop)', () => {
  beforeEach(() => {
    mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });
    useUIStore.setState({
      indicators: {},
      enabledLayers: [],
      layers: {},
      indicatorLayerIds: new Set<string>(),
    });
  });

  it('renders nothing when no indicators', () => {
    const { container } = render(<IndicatorHUD />);
    expect(container.querySelector('[data-testid^="indicator-panel-"]')).toBeNull();
  });

  it('renders nothing when indicator layer is not enabled', () => {
    setIndicatorState({ enabledLayers: [] });
    const { container } = render(<IndicatorHUD />);
    expect(container.querySelector('[data-testid^="indicator-panel-"]')).toBeNull();
  });

  it('renders a separate panel per indicator layer', () => {
    useUIStore.setState({
      indicators: {
        space_weather: makeSnapshot(),
        markets: makeSnapshot({
          layerId: 'markets',
          layerName: 'Markets',
          summary: 'Volatile',
          overallLevel: 3,
          values: [{ key: 'vix', value: '25', label: 'VIX', unit: '', level: 3 }],
        }),
      },
      enabledLayers: ['space_weather', 'markets'],
      layers: {
        space_weather: makeLayer(),
        markets: makeLayer({ id: 'markets', name: 'Markets', type: 'markets' }),
      },
      indicatorLayerIds: new Set(['space_weather', 'markets']),
    });
    render(<IndicatorHUD />);
    expect(screen.getByTestId('indicator-panel-space_weather')).toBeTruthy();
    expect(screen.getByTestId('indicator-panel-markets')).toBeTruthy();
  });

  it('renders panel with layer name as title', () => {
    setIndicatorState();
    render(<IndicatorHUD />);
    expect(screen.getByText('SPACE WEATHER')).toBeTruthy();
  });

  it('renders individual gauge values', () => {
    setIndicatorState();
    render(<IndicatorHUD />);
    expect(screen.getByTestId('indicator-gauge-r_scale')).toBeTruthy();
    expect(screen.getByTestId('indicator-gauge-s_scale')).toBeTruthy();
    expect(screen.getByTestId('indicator-gauge-g_scale')).toBeTruthy();
  });

  it('shows summary text', () => {
    setIndicatorState({
      indicators: { space_weather: makeSnapshot({ summary: 'Severe' }) },
    });
    render(<IndicatorHUD />);
    expect(screen.getByText('Severe')).toBeTruthy();
  });

  it('hides indicator panel when layer is disabled', () => {
    setIndicatorState();
    const { rerender } = render(<IndicatorHUD />);
    expect(screen.getByTestId('indicator-panel-space_weather')).toBeTruthy();

    useUIStore.setState({ enabledLayers: [] });
    rerender(<IndicatorHUD />);
    expect(screen.queryByTestId('indicator-panel-space_weather')).toBeNull();
  });

  it('does not render non-indicator layers even if in indicators map', () => {
    setIndicatorState({ indicatorLayerIds: new Set<string>() });
    const { container } = render(<IndicatorHUD />);
    expect(container.querySelector('[data-testid^="indicator-panel-"]')).toBeNull();
  });

  it('renders timestamp text', () => {
    const ts = Date.now() - 5 * 60_000;
    setIndicatorState({
      indicators: { space_weather: makeSnapshot({ timestampMs: ts }) },
    });
    render(<IndicatorHUD />);
    expect(screen.getByText('Updated 5m ago')).toBeTruthy();
  });

  it('close button calls toggleLayer to disable the layer', () => {
    const toggleLayer = vi.fn();
    setIndicatorState({ toggleLayer });
    render(<IndicatorHUD />);
    const panel = screen.getByTestId('indicator-panel-space_weather');
    const closeBtn = panel.querySelector('[data-testid="config-panel-close"]');
    expect(closeBtn).toBeTruthy();
    fireEvent.click(closeBtn!);
    expect(toggleLayer).toHaveBeenCalledWith('space_weather');
  });
});

describe('IndicatorHUD (mobile)', () => {
  beforeEach(() => {
    mockUseResponsive.mockReturnValue({ isMobile: true, isTablet: false, isDesktop: false });
    useUIStore.setState({
      indicators: {},
      enabledLayers: [],
      layers: {},
      indicatorLayerIds: new Set<string>(),
    });
  });

  it('renders nothing when no indicators', () => {
    const { container } = render(<IndicatorHUD />);
    expect(container.querySelector('[data-testid="mobile-indicator-chips"]')).toBeNull();
  });

  it('renders compact chips instead of panels on mobile', () => {
    setIndicatorState();
    render(<IndicatorHUD />);
    expect(screen.getByTestId('mobile-indicator-chips')).toBeTruthy();
    expect(screen.getByTestId('indicator-chip-space_weather')).toBeTruthy();
    // Should NOT render desktop panels
    expect(screen.queryByTestId('indicator-panel-space_weather')).toBeNull();
  });

  it('renders a chip for each indicator layer', () => {
    useUIStore.setState({
      indicators: {
        space_weather: makeSnapshot(),
        markets: makeSnapshot({
          layerId: 'markets',
          layerName: 'Markets',
          summary: 'Volatile',
          overallLevel: 3,
          values: [{ key: 'vix', value: '25', label: 'VIX', unit: '', level: 3 }],
        }),
      },
      enabledLayers: ['space_weather', 'markets'],
      layers: {
        space_weather: makeLayer(),
        markets: makeLayer({ id: 'markets', name: 'Markets', type: 'markets' }),
      },
      indicatorLayerIds: new Set(['space_weather', 'markets']),
    });
    render(<IndicatorHUD />);
    expect(screen.getByTestId('indicator-chip-space_weather')).toBeTruthy();
    expect(screen.getByTestId('indicator-chip-markets')).toBeTruthy();
  });

  it('shows layer name and summary in chip', () => {
    setIndicatorState({
      indicators: { space_weather: makeSnapshot({ summary: 'Severe' }) },
    });
    render(<IndicatorHUD />);
    const chip = screen.getByTestId('indicator-chip-space_weather');
    expect(chip.textContent).toContain('Space Weather');
    expect(chip.textContent).toContain('Severe');
  });

  it('tapping a chip opens the drawer with full gauge content', () => {
    setIndicatorState();
    render(<IndicatorHUD />);

    // Tap the chip
    fireEvent.click(screen.getByTestId('indicator-chip-space_weather'));

    // Drawer should render indicator content with gauges
    expect(screen.getByTestId('indicator-content-space_weather')).toBeTruthy();
    expect(screen.getByTestId('indicator-gauge-r_scale')).toBeTruthy();
  });

  it('disable layer action in drawer calls toggleLayer', () => {
    const toggleLayer = vi.fn();
    setIndicatorState({ toggleLayer });
    render(<IndicatorHUD />);

    // Open drawer
    fireEvent.click(screen.getByTestId('indicator-chip-space_weather'));

    // Click "Disable Layer"
    fireEvent.click(screen.getByText('Disable Layer'));
    expect(toggleLayer).toHaveBeenCalledWith('space_weather');
  });
});
