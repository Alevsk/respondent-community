/**
 * HistoryTab Branching Tests
 *
 * Verifies that HistoryTab renders TimelineEventList for moving entities
 * and ObservationDataView for stationary entities based on icon.interpolation.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';

// Mock child components to detect which one renders
vi.mock('./TimelineEventList', () => ({
  default: () => <div data-testid="timeline-event-list">TimelineEventList</div>,
}));

vi.mock('./ObservationDataView', () => ({
  default: () => <div data-testid="observation-data-view">ObservationDataView</div>,
  buildCsvString: vi.fn(() => ''),
  buildJsonString: vi.fn(() => ''),
  buildMarkdownString: vi.fn(() => ''),
  collectMetadataKeys: vi.fn(() => []),
}));

// Mock useEntityTrail to return empty data
vi.mock('../hooks/useEntityTrail', () => ({
  useEntityTrail: () => ({
    points: [],
    hasMore: false,
    isLoading: false,
    fetchMore: vi.fn(),
  }),
}));

// Mock cesium
vi.mock('cesium', () => ({
  Cartesian3: { fromDegrees: vi.fn() },
}));

// Mock globe store
vi.mock('../../globe/store', () => ({
  useViewerStore: Object.assign(() => null, {
    getState: () => ({ viewer: null, setTrackingOverridePosition: vi.fn() }),
  }),
}));

// We need to mock the store BEFORE importing HistoryTab
// Use vi.hoisted so mockLayers is available inside the hoisted vi.mock factory
const mockLayers = vi.hoisted(
  () => ({}) as Record<string, { displayConfig?: { icon?: { interpolation?: boolean } } }>,
);

vi.mock('@/app/store', () => {
  const storeState = {
    entityViewState: {},
    viewMode: 'globe',
    layers: mockLayers,
    updateEntityViewState: vi.fn(),
  };
  const useUIStore = Object.assign(
    (selector: (s: typeof storeState) => unknown) => selector(storeState),
    { getState: () => storeState },
  );
  return { useUIStore };
});

// Import AFTER mocks
import HistoryTab from './HistoryTab';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>
    <CssBaseline />
    {children}
  </ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

describe('HistoryTab — branching by interpolation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Clear layers between tests
    for (const key of Object.keys(mockLayers)) delete mockLayers[key];
  });

  it('should render TimelineEventList for moving entities (interpolation: true)', () => {
    mockLayers['iss'] = { displayConfig: { icon: { interpolation: true } } };
    renderWithTheme(
      <HistoryTab entityId="iss-1" layerType="iss" detail={undefined} isLoading={false} />,
    );
    expect(screen.getByTestId('timeline-event-list')).toBeInTheDocument();
    expect(screen.queryByTestId('static-entity-history')).not.toBeInTheDocument();
  });

  it('should render ObservationDataView for stationary entities (interpolation: false)', () => {
    mockLayers['volcanoes'] = { displayConfig: { icon: { interpolation: false } } };
    renderWithTheme(
      <HistoryTab entityId="ubinas-1" layerType="volcanoes" detail={undefined} isLoading={false} />,
    );
    expect(screen.getByTestId('observation-data-view')).toBeInTheDocument();
    expect(screen.queryByTestId('timeline-event-list')).not.toBeInTheDocument();
  });

  it('should default to table view when layer config is missing', () => {
    // No layer config — interpolation defaults to false → table view via ObservationDataView
    renderWithTheme(
      <HistoryTab entityId="unknown-1" layerType="unknown" detail={undefined} isLoading={false} />,
    );
    expect(screen.getByTestId('observation-data-view')).toBeInTheDocument();
  });

  it('should show Trails toggle only for moving entities', () => {
    mockLayers['iss'] = { displayConfig: { icon: { interpolation: true } } };
    renderWithTheme(
      <HistoryTab entityId="iss-1" layerType="iss" detail={undefined} isLoading={false} />,
    );
    expect(screen.getByText('Trails')).toBeInTheDocument();
    expect(screen.getByText('Isolate')).toBeInTheDocument();
  });

  it('should hide Trails and Isolate toggles for stationary entities', () => {
    mockLayers['volcanoes'] = { displayConfig: { icon: { interpolation: false } } };
    renderWithTheme(
      <HistoryTab entityId="ubinas-1" layerType="volcanoes" detail={undefined} isLoading={false} />,
    );
    expect(screen.queryByText('Trails')).not.toBeInTheDocument();
    expect(screen.queryByText('Isolate')).not.toBeInTheDocument();
  });
});
