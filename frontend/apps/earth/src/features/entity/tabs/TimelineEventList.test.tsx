/**
 * TimelineEventList Component Tests
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import TimelineEventList, { type TimelineEventListProps } from './TimelineEventList';
import type { TrailPoint } from '../hooks/useEntityTrail';

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
    lon: -122.4194,
    lat: 37.7749,
    altitudeM: 500000,
    ...overrides,
  };
}

describe('TimelineEventList', () => {
  const defaultProps: TimelineEventListProps = {
    points: [],
    highlightedTs: null,
    onHighlight: vi.fn(),
    onFlyTo: vi.fn(),
    hasMore: false,
    isLoading: false,
    onLoadMore: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // jsdom doesn't implement scrollIntoView
    Element.prototype.scrollIntoView = vi.fn();
  });

  describe('Empty & Loading States', () => {
    it('should show empty message when no points', () => {
      renderWithTheme(<TimelineEventList {...defaultProps} />);
      expect(screen.getByText('No observation history available')).toBeInTheDocument();
    });

    it('should show skeletons when loading with no data', () => {
      renderWithTheme(<TimelineEventList {...defaultProps} isLoading={true} />);
      expect(screen.queryByText('No observation history available')).not.toBeInTheDocument();
    });
  });

  describe('Rendering Events', () => {
    it('should render event cards for each point', () => {
      const points = [
        makePoint({ ts: 1700000000000 }),
        makePoint({ ts: 1700000001000 }),
        makePoint({ ts: 1700000002000 }),
      ];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      const cards = screen.getAllByTestId('timeline-event-card');
      expect(cards).toHaveLength(3);
    });

    it('should display newest events first', () => {
      const points = [
        makePoint({ ts: 1700000000000, lat: 10.0 }), // oldest (index 0)
        makePoint({ ts: 1700000002000, lat: 30.0 }), // newest (index 2)
      ];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      const cards = screen.getAllByTestId('timeline-event-card');
      // First displayed card should be the newest (lat 30.0)
      expect(cards[0]).toHaveTextContent('30.000');
      // Second displayed card should be the oldest (lat 10.0)
      expect(cards[1]).toHaveTextContent('10.000');
    });

    it('should display position coordinates', () => {
      const points = [makePoint({ lat: 37.775, lon: -122.419 })];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      expect(screen.getByText('37.775, -122.419')).toBeInTheDocument();
    });

    it('should display altitude', () => {
      const points = [makePoint({ altitudeM: 408000 })];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      expect(screen.getByText('408,000m')).toBeInTheDocument();
    });

    it('should display speed when available', () => {
      const points = [makePoint({ speed: 7680.5 })];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      expect(screen.getByText('7680.5 m/s')).toBeInTheDocument();
    });

    it('should not display speed when undefined', () => {
      const points = [makePoint({ speed: undefined })];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      expect(screen.queryByText(/m\/s/)).not.toBeInTheDocument();
    });
  });

  describe('Interactions', () => {
    it('should call onHighlight with timestamp on click', async () => {
      const onHighlight = vi.fn();
      const points = [makePoint({ ts: 1700000000000 }), makePoint({ ts: 1700000001000 })];
      renderWithTheme(
        <TimelineEventList {...defaultProps} points={points} onHighlight={onHighlight} />,
      );
      const cards = screen.getAllByTestId('timeline-event-card');
      // First displayed card = newest (ts 1700000001000)
      await userEvent.click(cards[0]);
      expect(onHighlight).toHaveBeenCalledWith(1700000001000);
    });

    it('should toggle highlight off when clicking already-highlighted event', async () => {
      const onHighlight = vi.fn();
      const points = [makePoint({ ts: 1700000000000 })];
      renderWithTheme(
        <TimelineEventList
          {...defaultProps}
          points={points}
          highlightedTs={1700000000000}
          onHighlight={onHighlight}
        />,
      );
      const cards = screen.getAllByTestId('timeline-event-card');
      await userEvent.click(cards[0]);
      expect(onHighlight).toHaveBeenCalledWith(null);
    });

    it('should call onFlyTo on double-click without toggling highlight off', async () => {
      const onHighlight = vi.fn();
      const onFlyTo = vi.fn();
      const point = makePoint({ ts: 1700000000000, lat: 37.0 });
      renderWithTheme(
        <TimelineEventList
          {...defaultProps}
          points={[point]}
          onHighlight={onHighlight}
          onFlyTo={onFlyTo}
        />,
      );
      const cards = screen.getAllByTestId('timeline-event-card');
      await userEvent.dblClick(cards[0]);
      expect(onFlyTo).toHaveBeenCalledWith(point);
      // Double-click should set highlight (not toggle off)
      expect(onHighlight).toHaveBeenCalledWith(1700000000000);
    });

    it('should call onHighlight + onFlyTo on single click in tracking mode', async () => {
      const onHighlight = vi.fn();
      const onFlyTo = vi.fn();
      const point = makePoint({ ts: 1700000000000, lat: 37.0 });
      renderWithTheme(
        <TimelineEventList
          {...defaultProps}
          points={[point]}
          onHighlight={onHighlight}
          onFlyTo={onFlyTo}
          trackingMode={true}
        />,
      );
      const cards = screen.getAllByTestId('timeline-event-card');
      await userEvent.click(cards[0]);
      expect(onHighlight).toHaveBeenCalledWith(1700000000000);
      expect(onFlyTo).toHaveBeenCalledWith(point);
    });
  });

  describe('Load More', () => {
    it('should show Load More button when hasMore is true', () => {
      const points = [makePoint()];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} hasMore={true} />);
      expect(screen.getByText('Load More')).toBeInTheDocument();
    });

    it('should not show Load More button when hasMore is false', () => {
      const points = [makePoint()];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} hasMore={false} />);
      expect(screen.queryByText('Load More')).not.toBeInTheDocument();
    });

    it('should call onLoadMore when Load More is clicked', async () => {
      const onLoadMore = vi.fn();
      const points = [makePoint()];
      renderWithTheme(
        <TimelineEventList
          {...defaultProps}
          points={points}
          hasMore={true}
          onLoadMore={onLoadMore}
        />,
      );
      await userEvent.click(screen.getByText('Load More'));
      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('should show Loading... text when loading more', () => {
      const points = [makePoint()];
      renderWithTheme(
        <TimelineEventList {...defaultProps} points={points} hasMore={true} isLoading={true} />,
      );
      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });
  });

  describe('Timeline Spine', () => {
    it('should render the timeline container', () => {
      const points = [makePoint()];
      renderWithTheme(<TimelineEventList {...defaultProps} points={points} />);
      expect(screen.getByTestId('timeline-event-list')).toBeInTheDocument();
    });
  });
});
