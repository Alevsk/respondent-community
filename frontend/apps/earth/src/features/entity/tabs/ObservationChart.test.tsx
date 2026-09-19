/**
 * ObservationChart Component Tests
 *
 * Covers:
 * - Renders "No numeric fields available" when no numeric fields detected
 * - Detects numeric fields from metadata across observations
 * - Renders pill buttons for each detected field
 * - Active pill has highlighted style
 * - Clicking a pill toggles it on/off
 * - At least one pill remains active (can't deselect all)
 * - Renders recharts ResponsiveContainer (mocked)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import type { TrailPoint } from '@respondent/core';

// Mock recharts — all components render simple divs.
// Line exposes its stroke and dataKey props as data attributes so tests can
// assert which palette color each field receives.
vi.mock('recharts', () => ({
  LineChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="recharts-line-chart">{children}</div>
  ),
  Line: ({ stroke, dataKey }: { stroke?: string; dataKey?: string }) => (
    <div data-testid="recharts-line" data-stroke={stroke} data-datakey={dataKey} />
  ),
  XAxis: () => <div data-testid="recharts-xaxis" />,
  YAxis: () => <div data-testid="recharts-yaxis" />,
  CartesianGrid: () => <div data-testid="recharts-cartesian-grid" />,
  Tooltip: () => <div data-testid="recharts-tooltip" />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="recharts-responsive-container">{children}</div>
  ),
}));

import ObservationChart from './ObservationChart';

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

describe('ObservationChart', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Empty / No Chartable Fields', () => {
    it('renders "No numeric fields available for charting" when there are no points', () => {
      renderWithTheme(<ObservationChart points={[]} />);
      expect(screen.getByText('No numeric fields available for charting')).toBeInTheDocument();
    });

    it('renders "No numeric fields available for charting" when metadata values are all non-numeric and altitude is zero', () => {
      const points = [
        makePoint({ altitudeM: 0, metadata: { status: 'active' } }),
        makePoint({ altitudeM: 0, metadata: { status: 'inactive' } }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('No numeric fields available for charting')).toBeInTheDocument();
    });

    it('detects numeric fields even with a single observation', () => {
      renderWithTheme(<ObservationChart points={[makePoint({ altitudeM: 500 })]} />);
      expect(screen.getByText('Altitude (m)')).toBeInTheDocument();
    });

    it('detects altitude even when all values are identical across observations', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 500 })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Altitude (m)')).toBeInTheDocument();
    });
  });

  describe('Chartable Field Detection', () => {
    it('detects altitudeM as a chartable field when values vary', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Altitude (m)')).toBeInTheDocument();
    });

    it('detects speed as a chartable field when values vary', () => {
      const points = [makePoint({ speed: 100 }), makePoint({ speed: 200 })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Speed (m/s)')).toBeInTheDocument();
    });

    it('detects speed even with only one speed observation', () => {
      const points = [makePoint({ speed: 100 }), makePoint({ speed: undefined })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Speed (m/s)')).toBeInTheDocument();
    });

    it('detects numeric metadata field as chartable when values vary', () => {
      const points = [
        makePoint({ altitudeM: 500, metadata: { signal_strength: '80' } }),
        makePoint({ altitudeM: 600, metadata: { signal_strength: '90' } }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Signal Strength')).toBeInTheDocument();
    });

    it('detects metadata field even when all values are identical', () => {
      const points = [
        makePoint({ altitudeM: 500, metadata: { frequency: '100' } }),
        makePoint({ altitudeM: 600, metadata: { frequency: '100' } }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Frequency')).toBeInTheDocument();
    });

    it('converts metadata key snake_case to Title Case for pill label', () => {
      const points = [
        makePoint({ altitudeM: 500, metadata: { battery_level: '50' } }),
        makePoint({ altitudeM: 600, metadata: { battery_level: '75' } }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByText('Battery Level')).toBeInTheDocument();
    });
  });

  describe('Pill Buttons', () => {
    it('renders a pill button for each detected field', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100 }),
        makePoint({ altitudeM: 600, speed: 200 }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      const altitudePill = screen.getByText('Altitude (m)');
      const speedPill = screen.getByText('Speed (m/s)');
      expect(altitudePill).toBeInTheDocument();
      expect(speedPill).toBeInTheDocument();
    });

    it('renders pill button as a button element', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);
      const pill = screen.getByText('Altitude (m)');
      expect(pill.tagName.toLowerCase()).toBe('button');
    });

    it('first pill is active by default', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);
      const altitudePill = screen.getByText('Altitude (m)');
      // Active pill has fontWeight 600
      expect(altitudePill).toHaveStyle({ fontWeight: 600 });
    });

    it('inactive pill has fontWeight 400', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100 }),
        makePoint({ altitudeM: 600, speed: 200 }),
      ];
      renderWithTheme(<ObservationChart points={points} />);
      const speedPill = screen.getByText('Speed (m/s)');
      // Speed is the second pill, not active by default
      expect(speedPill).toHaveStyle({ fontWeight: 400 });
    });

    it('clicking an inactive pill activates it', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100 }),
        makePoint({ altitudeM: 600, speed: 200 }),
      ];
      renderWithTheme(<ObservationChart points={points} />);

      const speedPill = screen.getByText('Speed (m/s)');
      fireEvent.click(speedPill);

      // After clicking, Speed should be active (fontWeight 600)
      expect(speedPill).toHaveStyle({ fontWeight: 600 });
    });

    it('clicking the only active pill does not deselect it', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);

      const altitudePill = screen.getByText('Altitude (m)');
      // Click the only active pill
      fireEvent.click(altitudePill);

      // Should remain active — at least one must always be selected
      expect(altitudePill).toHaveStyle({ fontWeight: 600 });
    });

    it('can deselect a pill when multiple are active', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100 }),
        makePoint({ altitudeM: 600, speed: 200 }),
      ];
      renderWithTheme(<ObservationChart points={points} />);

      const altitudePill = screen.getByText('Altitude (m)');
      const speedPill = screen.getByText('Speed (m/s)');

      // Activate speed pill
      fireEvent.click(speedPill);
      // Now deselect altitude (both are active)
      fireEvent.click(altitudePill);

      // Altitude should now be inactive
      expect(altitudePill).toHaveStyle({ fontWeight: 400 });
      // Speed should remain active
      expect(speedPill).toHaveStyle({ fontWeight: 600 });
    });
  });

  describe('Chart Rendering', () => {
    it('renders ResponsiveContainer when chartable fields exist', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByTestId('recharts-responsive-container')).toBeInTheDocument();
    });

    it('renders LineChart inside ResponsiveContainer', () => {
      const points = [makePoint({ altitudeM: 500 }), makePoint({ altitudeM: 600 })];
      renderWithTheme(<ObservationChart points={points} />);
      expect(screen.getByTestId('recharts-line-chart')).toBeInTheDocument();
    });

    it('does not render recharts when no points', () => {
      renderWithTheme(<ObservationChart points={[]} />);
      expect(screen.queryByTestId('recharts-responsive-container')).not.toBeInTheDocument();
    });

    it('renders recharts for a single observation with numeric fields', () => {
      renderWithTheme(<ObservationChart points={[makePoint({ altitudeM: 500 })]} />);
      expect(screen.getByTestId('recharts-responsive-container')).toBeInTheDocument();
    });
  });

  // -------------------------------------------------------------------------
  // Line stroke color — uses findIndex (original palette index), not the
  // filtered-array index.
  //
  // Palette: ['#00ff9d', '#ff006e', '#00e5ff', '#ffbe0b', '#8338ec']
  //           idx 0         idx 1     idx 2      idx 3      idx 4
  // -------------------------------------------------------------------------

  describe('Line stroke color matches original palette index', () => {
    /**
     * Returns a map of dataKey → stroke for all rendered Line mocks.
     */
    function getLineStrokes(): Map<string, string> {
      const lines = screen.getAllByTestId('recharts-line');
      const map = new Map<string, string>();
      for (const line of lines) {
        const key = line.getAttribute('data-datakey') ?? '';
        const stroke = line.getAttribute('data-stroke') ?? '';
        map.set(key, stroke);
      }
      return map;
    }

    it('first active field uses palette index 0 (#00ff9d)', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100, metadata: { signal: '80' } }),
        makePoint({ altitudeM: 600, speed: 200, metadata: { signal: '90' } }),
      ];
      // Fields: [__altitudeM (idx 0), __speed (idx 1), signal (idx 2)]
      // Only __altitudeM is active by default.
      renderWithTheme(<ObservationChart points={points} />);

      const strokes = getLineStrokes();
      expect(strokes.get('__altitudeM')).toBe('#00ff9d');
    });

    it('when only a subset of fields are active, each Line uses the field original palette index', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100, metadata: { signal: '80' } }),
        makePoint({ altitudeM: 600, speed: 200, metadata: { signal: '90' } }),
      ];
      // Fields ordered: [__altitudeM (idx 0), __speed (idx 1), signal (idx 2)]
      renderWithTheme(<ObservationChart points={points} />);

      // Activate all three fields.
      fireEvent.click(screen.getByText('Speed (m/s)')); // activate idx 1
      fireEvent.click(screen.getByText('Signal')); // activate idx 2

      const strokes = getLineStrokes();
      // Each field must use its original palette slot, not the position within
      // the filtered active subset.
      expect(strokes.get('__altitudeM')).toBe('#00ff9d'); // palette[0]
      expect(strokes.get('__speed')).toBe('#ff006e'); // palette[1]
      expect(strokes.get('signal')).toBe('#00e5ff'); // palette[2]
    });

    it('deselecting the first field does not shift remaining fields to lower palette indices', () => {
      const points = [
        makePoint({ altitudeM: 500, speed: 100, metadata: { signal: '80' } }),
        makePoint({ altitudeM: 600, speed: 200, metadata: { signal: '90' } }),
      ];
      // Fields: [__altitudeM (idx 0), __speed (idx 1), signal (idx 2)]
      renderWithTheme(<ObservationChart points={points} />);

      // First activate speed and signal so we can then deselect altitude.
      fireEvent.click(screen.getByText('Speed (m/s)'));
      fireEvent.click(screen.getByText('Signal'));
      // Now deselect altitude (idx 0) — speed and signal must keep their original colors.
      fireEvent.click(screen.getByText('Altitude (m)'));

      const strokes = getLineStrokes();
      // Only speed and signal are active; their original palette indices must be preserved.
      expect(strokes.has('__altitudeM')).toBe(false);
      expect(strokes.get('__speed')).toBe('#ff006e'); // palette[1] — not shifted to palette[0]
      expect(strokes.get('signal')).toBe('#00e5ff'); // palette[2] — not shifted to palette[1]
    });

    it('a field at palette index 4 uses #8338ec regardless of which other fields are active', () => {
      // Build five fields: altitude(0), speed(1), a(2), b(3), c(4)
      const points = [
        makePoint({
          altitudeM: 500,
          speed: 100,
          metadata: { a: '1', b: '2', c: '3' },
        }),
        makePoint({
          altitudeM: 600,
          speed: 200,
          metadata: { a: '4', b: '5', c: '6' },
        }),
      ];
      renderWithTheme(<ObservationChart points={points} />);

      // Activate all five fields.
      fireEvent.click(screen.getByText('Speed (m/s)'));
      fireEvent.click(screen.getByText('A'));
      fireEvent.click(screen.getByText('B'));
      fireEvent.click(screen.getByText('C'));

      const strokes = getLineStrokes();
      // Field 'c' is at original index 4 → palette[4] = '#8338ec'.
      expect(strokes.get('c')).toBe('#8338ec');
    });
  });
});
