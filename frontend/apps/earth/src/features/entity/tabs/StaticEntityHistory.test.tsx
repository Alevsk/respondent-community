/**
 * StaticEntityHistory Component Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import StaticEntityHistory from './StaticEntityHistory';
import type { TrailPoint } from '../hooks/useEntityTrail';
import type { EntityDetailResponse } from '../../../shared/api/queries';

// Mock the fieldRenderers module so we control resolveFields output
vi.mock('./overview/fieldRenderers', () => ({
  resolveFields: vi.fn((metadata: Record<string, string>) => {
    const fields: Array<{ label: string; value: string; priority: number }> = [];
    if (metadata.events) fields.push({ label: 'EVENTS', value: metadata.events, priority: 0 });
    if (metadata.fatalities)
      fields.push({ label: 'FATALITIES', value: metadata.fatalities, priority: 1 });
    if (metadata.status) fields.push({ label: 'STATUS', value: metadata.status, priority: 2 });
    return fields;
  }),
}));

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

function makePoint(overrides: Partial<TrailPoint> = {}): TrailPoint {
  return {
    ts: 1700000000000,
    lon: -70.897,
    lat: -16.345,
    altitudeM: 5608,
    metadata: { status: 'Historical' },
    ...overrides,
  };
}

function makeDetail(overrides: Partial<EntityDetailResponse> = {}): EntityDetailResponse {
  return {
    entity: {
      id: 'test-entity',
      externalId: 'ext-1',
      layerType: 'volcanoes',
      name: 'Ubinas',
      metadata: { country: 'Peru' },
    },
    latestObservation: {
      entityId: 'test-entity',
      ts: 1700000000000,
      position: { lat: -16.345, lon: -70.897, altM: 5608 },
      altitudeM: 5608,
      metadata: { status: 'Historical' },
    },
    ...overrides,
  };
}

const defaultProps = {
  entityId: 'test-entity',
  layerType: 'volcanoes',
  detail: makeDetail(),
  points: [] as TrailPoint[],
  hasMore: false,
  isLoading: false,
  onLoadMore: vi.fn(),
};

describe('StaticEntityHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Loading & Empty States', () => {
    it('should show skeletons when loading with no data', () => {
      renderWithTheme(<StaticEntityHistory {...defaultProps} isLoading={true} />);
      expect(screen.queryByText('No observation history available')).not.toBeInTheDocument();
    });

    it('should show empty message when not loading and no points', () => {
      renderWithTheme(<StaticEntityHistory {...defaultProps} />);
      expect(screen.getByText('No observation history available')).toBeInTheDocument();
    });
  });

  describe('Summary Card', () => {
    it('should render position line from latest observation', () => {
      const points = [makePoint()];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      expect(screen.getByTestId('static-history-position')).toHaveTextContent(
        /16\.345.*70\.897.*5,608/,
      );
    });

    it('should render resolved fields in the event summary', () => {
      const points = [makePoint({ metadata: { status: 'Historical', events: '5' } })];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      expect(screen.getAllByText('STATUS').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('EVENTS').length).toBeGreaterThanOrEqual(1);
    });

    it('should render LOCATION section header above position', () => {
      const points = [makePoint()];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      expect(screen.getByText('LOCATION')).toBeInTheDocument();
    });
  });

  describe('Row Selection', () => {
    it('should highlight clicked changelog row', async () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3' } }),
        makePoint({ ts: 1700000001000, metadata: { events: '5' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      const rows = screen.getAllByTestId('changelog-entry');
      await userEvent.click(rows[0]);
      expect(screen.getByTestId('changelog-entry-selected')).toBeInTheDocument();
    });

    it('should deselect row when clicking the same row again', async () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3' } }),
        makePoint({ ts: 1700000001000, metadata: { events: '5' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      const rows = screen.getAllByTestId('changelog-entry');
      await userEvent.click(rows[0]);
      expect(screen.getByTestId('changelog-entry-selected')).toBeInTheDocument();
      await userEvent.click(screen.getByTestId('changelog-entry-selected'));
      expect(screen.queryByTestId('changelog-entry-selected')).not.toBeInTheDocument();
    });
  });

  describe('Changelog', () => {
    it('should render changelog rows for each point', () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3' } }),
        makePoint({ ts: 1700000001000, metadata: { events: '5' } }),
        makePoint({ ts: 1700000002000, metadata: { events: '7' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      const rows = screen.getAllByTestId('changelog-entry');
      expect(rows).toHaveLength(3);
    });

    it('should display newest entries first', () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3' } }),
        makePoint({ ts: 1700000002000, metadata: { events: '7' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      const rows = screen.getAllByTestId('changelog-entry');
      // First displayed row should be newest (events: 7)
      expect(rows[0]).toHaveTextContent('7');
      // Second displayed row should be oldest (events: 3)
      expect(rows[1]).toHaveTextContent('3');
    });

    it('should highlight changed field values with primary color', () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3', fatalities: '0' } }),
        makePoint({ ts: 1700000001000, metadata: { events: '5', fatalities: '0' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      // The newer row (events changed 3→5) should have a changed marker
      const changedValues = screen.getAllByTestId('field-value-changed');
      expect(changedValues.length).toBeGreaterThan(0);
      // "events" changed from 3 to 5
      expect(changedValues.some((el) => el.textContent?.includes('5'))).toBe(true);
    });

    it('should not highlight unchanged fields', () => {
      const points = [
        makePoint({ ts: 1700000000000, metadata: { events: '3', fatalities: '0' } }),
        makePoint({ ts: 1700000001000, metadata: { events: '5', fatalities: '0' } }),
      ];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      // fatalities stayed 0 → should be unchanged
      const unchangedValues = screen.getAllByTestId('field-value-unchanged');
      expect(unchangedValues.some((el) => el.textContent?.includes('0'))).toBe(true);
    });

    it('should render first (oldest) observation with all fields in default style', () => {
      const points = [makePoint({ ts: 1700000000000, metadata: { events: '3' } })];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} />);
      // Single observation — no previous to compare, so no changed markers
      expect(screen.queryAllByTestId('field-value-changed')).toHaveLength(0);
    });
  });

  describe('Load More', () => {
    it('should show Load More button when hasMore is true', () => {
      const points = [makePoint()];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} hasMore={true} />);
      expect(screen.getByText('Load More')).toBeInTheDocument();
    });

    it('should not show Load More button when hasMore is false', () => {
      const points = [makePoint()];
      renderWithTheme(<StaticEntityHistory {...defaultProps} points={points} hasMore={false} />);
      expect(screen.queryByText('Load More')).not.toBeInTheDocument();
    });

    it('should call onLoadMore when clicked', async () => {
      const onLoadMore = vi.fn();
      const points = [makePoint()];
      renderWithTheme(
        <StaticEntityHistory
          {...defaultProps}
          points={points}
          hasMore={true}
          onLoadMore={onLoadMore}
        />,
      );
      await userEvent.click(screen.getByText('Load More'));
      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });
  });
});
