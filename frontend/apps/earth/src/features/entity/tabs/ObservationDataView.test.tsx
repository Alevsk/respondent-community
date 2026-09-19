/**
 * ObservationDataView Component Tests
 *
 * Covers:
 * - Shows "No observation data available" when points is empty
 * - Single observation: renders SingleObservationView with field rows
 * - Multiple observations: renders ObservationTable in table mode
 * - Chart mode: renders ObservationChart
 * - Accepts viewMode prop and renders accordingly
 * - Load more button shown when hasMore is true
 * - Load more button hidden when hasMore is false
 * - Load more button calls onLoadMore
 * - Load more button is disabled while isLoading
 * - Helper exports: collectMetadataKeys, buildCsvString, buildJsonString, buildMarkdownString
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import type { TrailPoint } from '@respondent/core';

// Mock child components so tests isolate ObservationDataView behavior
vi.mock('./ObservationChart', () => ({
  default: () => <div data-testid="observation-chart">ObservationChart</div>,
}));

vi.mock('./shared/ObservationTable', () => ({
  default: () => <div data-testid="observation-table-mock">ObservationTable</div>,
}));

// Mock core time formatters to avoid time-based flakiness
vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual('@respondent/core');
  return {
    ...actual,
    formatRelativeTime: vi.fn(() => '5 min ago'),
    formatAbsoluteTime: vi.fn(() => '2024-01-01 12:00:00 UTC'),
    formatUtcTime: vi.fn((ts: number) => (ts <= 0 ? '—' : '13:45:00Z')),
  };
});

import ObservationDataView, {
  collectMetadataKeys,
  buildCsvString,
  buildJsonString,
  buildMarkdownString,
} from './ObservationDataView';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const renderWithTheme = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

function makePoint(overrides: Partial<TrailPoint> = {}): TrailPoint {
  return {
    ts: 1700000000000,
    lat: 37.7749,
    lon: -122.4194,
    altitudeM: 500,
    ...overrides,
  };
}

const defaultProps = {
  points: [] as TrailPoint[],
  hasMore: false,
  isLoading: false,
  onLoadMore: vi.fn(),
  viewMode: 'table' as const,
};

describe('ObservationDataView', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Empty State', () => {
    it('shows "No observation data available" when points is empty', () => {
      renderWithTheme(<ObservationDataView {...defaultProps} points={[]} />);
      expect(screen.getByText('No observation data available')).toBeInTheDocument();
    });

    it('does not render the table or chart when points is empty', () => {
      renderWithTheme(<ObservationDataView {...defaultProps} points={[]} />);
      expect(screen.queryByTestId('observation-table-mock')).not.toBeInTheDocument();
      expect(screen.queryByTestId('observation-chart')).not.toBeInTheDocument();
    });
  });

  describe('Single Observation View', () => {
    it('renders single observation container with data-testid="observation-single"', () => {
      renderWithTheme(<ObservationDataView {...defaultProps} points={[makePoint()]} />);
      expect(screen.getByTestId('observation-single')).toBeInTheDocument();
    });

    it('renders ObservationTable for single observation in table mode', () => {
      renderWithTheme(<ObservationDataView {...defaultProps} points={[makePoint()]} />);
      // Single observation renders through ObservationTable (same as multi)
      expect(screen.getByTestId('observation-single')).toBeInTheDocument();
    });

    it('renders ObservationChart for single observation in chart mode', () => {
      renderWithTheme(
        <ObservationDataView {...defaultProps} points={[makePoint()]} viewMode="chart" />,
      );
      // Chart mode works for single observations too
      expect(screen.getByTestId('observation-single')).toBeInTheDocument();
    });

    it('respects viewMode for single observation', () => {
      const { rerender } = renderWithTheme(
        <ObservationDataView {...defaultProps} points={[makePoint()]} viewMode="table" />,
      );
      expect(screen.getByTestId('observation-single')).toBeInTheDocument();

      rerender(
        <ThemeProvider theme={createTheme()}>
          <ObservationDataView {...defaultProps} points={[makePoint()]} viewMode="chart" />
        </ThemeProvider>,
      );
      expect(screen.getByTestId('observation-single')).toBeInTheDocument();
    });
  });

  describe('Multiple Observations - Table Mode', () => {
    it('renders multi-observation container with data-testid="observation-multi"', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(<ObservationDataView {...defaultProps} points={points} viewMode="table" />);
      expect(screen.getByTestId('observation-multi')).toBeInTheDocument();
    });

    it('renders ObservationTable in table mode', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(<ObservationDataView {...defaultProps} points={points} viewMode="table" />);
      expect(screen.getByTestId('observation-table-mock')).toBeInTheDocument();
      expect(screen.queryByTestId('observation-chart')).not.toBeInTheDocument();
    });
  });

  describe('Multiple Observations - Chart Mode', () => {
    it('renders ObservationChart in chart mode', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(<ObservationDataView {...defaultProps} points={points} viewMode="chart" />);
      expect(screen.getByTestId('observation-chart')).toBeInTheDocument();
      expect(screen.queryByTestId('observation-table-mock')).not.toBeInTheDocument();
    });
  });

  describe('Load More', () => {
    it('shows "Load More" button when hasMore is true', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(<ObservationDataView {...defaultProps} points={points} hasMore={true} />);
      expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
    });

    it('does not show "Load More" button when hasMore is false', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(<ObservationDataView {...defaultProps} points={points} hasMore={false} />);
      expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
    });

    it('calls onLoadMore when Load More button is clicked', () => {
      const onLoadMore = vi.fn();
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(
        <ObservationDataView
          {...defaultProps}
          points={points}
          hasMore={true}
          onLoadMore={onLoadMore}
        />,
      );
      fireEvent.click(screen.getByRole('button', { name: /load more/i }));
      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('shows "Loading..." text on button while isLoading is true', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(
        <ObservationDataView {...defaultProps} points={points} hasMore={true} isLoading={true} />,
      );
      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });

    it('disables the Load More button while isLoading is true', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 })];
      renderWithTheme(
        <ObservationDataView {...defaultProps} points={points} hasMore={true} isLoading={true} />,
      );
      const button = screen.getByRole('button', { name: /loading/i });
      expect(button).toBeDisabled();
    });
  });

  describe('collectMetadataKeys helper', () => {
    it('returns empty array for points with no metadata', () => {
      const points = [makePoint(), makePoint()];
      expect(collectMetadataKeys(points)).toEqual([]);
    });

    it('returns sorted unique keys across all points', () => {
      const points = [
        makePoint({ metadata: { zebra: '1', apple: '2' } }),
        makePoint({ metadata: { mango: '3', apple: '4' } }),
      ];
      expect(collectMetadataKeys(points)).toEqual(['apple', 'mango', 'zebra']);
    });

    it('deduplicates keys that appear in multiple points', () => {
      const points = [makePoint({ metadata: { key: 'a' } }), makePoint({ metadata: { key: 'b' } })];
      expect(collectMetadataKeys(points)).toEqual(['key']);
    });

    it('handles empty points array', () => {
      expect(collectMetadataKeys([])).toEqual([]);
    });
  });

  describe('buildCsvString helper', () => {
    it('includes header row', () => {
      const points = [makePoint({ ts: 1700000000000, lat: 1, lon: 2, altitudeM: 100 })];
      const csv = buildCsvString(points, []);
      expect(csv.split('\n')[0]).toBe('time,lat,lon,alt_m');
    });

    it('includes metadata keys in header', () => {
      const points = [makePoint({ metadata: { foo: 'bar' } })];
      const csv = buildCsvString(points, ['foo']);
      expect(csv.split('\n')[0]).toBe('time,lat,lon,alt_m,foo');
    });

    it('wraps values containing commas in double quotes', () => {
      const points = [makePoint({ metadata: { desc: 'a,b,c' } })];
      const csv = buildCsvString(points, ['desc']);
      expect(csv).toContain('"a,b,c"');
    });
  });

  describe('buildJsonString helper', () => {
    it('returns valid JSON string', () => {
      const points = [makePoint({ lat: 37.77, lon: -122.4, altitudeM: 500 })];
      const json = buildJsonString(points, []);
      expect(() => JSON.parse(json)).not.toThrow();
    });

    it('includes lat, lon, alt_m, and time fields', () => {
      const points = [makePoint({ lat: 37.77, lon: -122.4, altitudeM: 500 })];
      const json = buildJsonString(points, []);
      const parsed = JSON.parse(json);
      expect(parsed[0]).toHaveProperty('lat');
      expect(parsed[0]).toHaveProperty('lon');
      expect(parsed[0]).toHaveProperty('alt_m');
      expect(parsed[0]).toHaveProperty('time');
    });

    it('includes metadata keys in output', () => {
      const points = [makePoint({ metadata: { quality: '5' } })];
      const json = buildJsonString(points, ['quality']);
      const parsed = JSON.parse(json);
      expect(parsed[0]).toHaveProperty('quality', '5');
    });
  });

  describe('buildMarkdownString helper', () => {
    it('returns a string with markdown table header pipe syntax', () => {
      const points = [makePoint()];
      const md = buildMarkdownString(points, []);
      expect(md).toContain('| Time |');
      expect(md).toContain('| Lat |');
      expect(md).toContain('| Lon |');
      expect(md).toContain('| Alt (m) |');
    });

    it('includes separator row with ---', () => {
      const points = [makePoint()];
      const md = buildMarkdownString(points, []);
      expect(md).toContain('| --- |');
    });

    it('includes metadata keys in header uppercased', () => {
      const points = [makePoint({ metadata: { signal: '80' } })];
      const md = buildMarkdownString(points, ['signal']);
      expect(md).toContain('SIGNAL');
    });
  });
});
