/**
 * ObservationTable Component Tests
 *
 * Covers:
 * - Renders table headers (Time, Lat, Lon, Alt)
 * - Renders metadata columns from metaKeys
 * - Headers are left-aligned
 * - Renders correct number of rows
 * - Formats numbers: integers with toLocaleString, floats to max 4dp
 * - Formats lat/lon to 4 decimal places
 * - Alt+click on a cell copies to clipboard (mock navigator.clipboard)
 * - Right-click opens context menu with "Copy Row as JSON" and "Copy Row as CSV"
 * - Row hover changes background
 * - Headers are sticky (position: sticky, top: 0)
 * - Title case formatting: snake_case → Title Case
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import type { TrailPoint } from '@respondent/core';
import ObservationTable from './ObservationTable';

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

describe('ObservationTable', () => {
  beforeEach(() => {
    // Mock navigator.clipboard
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      writable: true,
      configurable: true,
    });
  });

  describe('Header Rendering', () => {
    it('renders Time header', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      expect(screen.getByText('Time')).toBeInTheDocument();
    });

    it('renders Lat header', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      expect(screen.getByText('Lat')).toBeInTheDocument();
    });

    it('renders Lon header', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      expect(screen.getByText('Lon')).toBeInTheDocument();
    });

    it('renders Alt (m) header', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      expect(screen.getByText('Alt (m)')).toBeInTheDocument();
    });

    it('renders metadata column headers from metaKeys', () => {
      renderWithTheme(
        <ObservationTable points={[makePoint()]} metaKeys={['signal_strength', 'frequency']} />,
      );
      expect(screen.getByText('Signal Strength')).toBeInTheDocument();
      expect(screen.getByText('Frequency')).toBeInTheDocument();
    });

    it('converts snake_case keys to Title Case', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={['altitude_km']} />);
      expect(screen.getByText('Altitude Km')).toBeInTheDocument();
    });

    it('applies sticky positioning to headers', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      const timeHeader = screen.getByText('Time');
      expect(timeHeader).toHaveStyle({ position: 'sticky' });
      expect(timeHeader).toHaveStyle({ top: '0px' });
    });

    it('applies left text-align to headers', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      const timeHeader = screen.getByText('Time');
      expect(timeHeader).toHaveStyle({ textAlign: 'left' });
    });

    it('applies uppercase textTransform to headers', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      const timeHeader = screen.getByText('Time');
      expect(timeHeader).toHaveStyle({ textTransform: 'uppercase' });
    });
  });

  describe('Row Rendering', () => {
    it('renders the correct number of rows for single point', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      const rows = screen.getAllByRole('row');
      // 1 header row + 1 data row
      expect(rows).toHaveLength(2);
    });

    it('renders the correct number of rows for multiple points', () => {
      const points = [makePoint({ ts: 1 }), makePoint({ ts: 2 }), makePoint({ ts: 3 })];
      renderWithTheme(<ObservationTable points={points} metaKeys={[]} />);
      const rows = screen.getAllByRole('row');
      // 1 header row + 3 data rows
      expect(rows).toHaveLength(4);
    });

    it('renders lat formatted to 4 decimal places', () => {
      renderWithTheme(<ObservationTable points={[makePoint({ lat: 37.7749 })]} metaKeys={[]} />);
      expect(screen.getByText('37.7749')).toBeInTheDocument();
    });

    it('renders lon formatted to 4 decimal places', () => {
      renderWithTheme(<ObservationTable points={[makePoint({ lon: -122.4194 })]} metaKeys={[]} />);
      expect(screen.getByText('-122.4194')).toBeInTheDocument();
    });

    it('renders altitude rounded and with locale formatting for large integers', () => {
      renderWithTheme(
        <ObservationTable points={[makePoint({ altitudeM: 408000.7 })]} metaKeys={[]} />,
      );
      expect(screen.getByText('408,001')).toBeInTheDocument();
    });

    it('renders small integer altitude without comma', () => {
      renderWithTheme(<ObservationTable points={[makePoint({ altitudeM: 500 })]} metaKeys={[]} />);
      expect(screen.getByText('500')).toBeInTheDocument();
    });
  });

  describe('Number Formatting in Metadata Cells', () => {
    it('formats integer metadata values with toLocaleString', () => {
      const point = makePoint({ metadata: { population: '1000000' } });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={['population']} />);
      expect(screen.getByText('1,000,000')).toBeInTheDocument();
    });

    it('formats float metadata values to max 4 decimal places', () => {
      const point = makePoint({ metadata: { ratio: '0.123456789' } });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={['ratio']} />);
      expect(screen.getByText('0.1235')).toBeInTheDocument();
    });

    it('renders non-numeric metadata values as-is', () => {
      const point = makePoint({ metadata: { status: 'active' } });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={['status']} />);
      expect(screen.getByText('active')).toBeInTheDocument();
    });

    it('renders null/undefined metadata values as em dash', () => {
      const point = makePoint({ metadata: {} });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={['missing_key']} />);
      expect(screen.getByText('—')).toBeInTheDocument();
    });

    it('trims trailing zeros from float values', () => {
      const point = makePoint({ metadata: { ratio: '1.50000' } });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={['ratio']} />);
      // 1.5 formatted — trailing zeros trimmed
      expect(screen.getByText('1.5')).toBeInTheDocument();
    });
  });

  describe('Alt+Click to Copy Cell', () => {
    it('calls navigator.clipboard.writeText with lat value on Alt+click', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      renderWithTheme(<ObservationTable points={[makePoint({ lat: 37.7749 })]} metaKeys={[]} />);

      const latCell = screen.getByText('37.7749');
      fireEvent.click(latCell, { altKey: true });
      expect(writeText).toHaveBeenCalledWith('37.7749');
    });

    it('does not call clipboard when clicking without Alt key', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      renderWithTheme(<ObservationTable points={[makePoint({ lat: 37.7749 })]} metaKeys={[]} />);

      const latCell = screen.getByText('37.7749');
      fireEvent.click(latCell, { altKey: false });
      expect(writeText).not.toHaveBeenCalled();
    });

    it('calls clipboard with lon value on Alt+click', () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      renderWithTheme(<ObservationTable points={[makePoint({ lon: -122.4194 })]} metaKeys={[]} />);

      const lonCell = screen.getByText('-122.4194');
      fireEvent.click(lonCell, { altKey: true });
      expect(writeText).toHaveBeenCalledWith('-122.4194');
    });
  });

  describe('Right-Click Context Menu', () => {
    it('opens context menu on right-click of a row', () => {
      const points = [makePoint()];
      renderWithTheme(<ObservationTable points={points} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      const dataRow = rows[1]; // First data row after header
      fireEvent.contextMenu(dataRow);

      expect(screen.getByText('Copy Row as JSON')).toBeInTheDocument();
      expect(screen.getByText('Copy Row as CSV')).toBeInTheDocument();
    });

    it('calls clipboard with JSON string when "Copy Row as JSON" is clicked', () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      const point = makePoint({ ts: 1700000000000, lat: 37.7749, lon: -122.4194, altitudeM: 500 });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      fireEvent.contextMenu(rows[1]);
      fireEvent.click(screen.getByText('Copy Row as JSON'));

      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('"lat"'));
      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('"lon"'));
    });

    it('calls clipboard with CSV string when "Copy Row as CSV" is clicked', () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      const point = makePoint({ ts: 1700000000000, lat: 37.7749, lon: -122.4194, altitudeM: 500 });
      renderWithTheme(<ObservationTable points={[point]} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      fireEvent.contextMenu(rows[1]);
      fireEvent.click(screen.getByText('Copy Row as CSV'));

      // CSV should be comma-separated with lat and lon
      expect(writeText).toHaveBeenCalledWith(expect.stringContaining('37.7749'));
    });

    it('closes context menu after clicking a menu item', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.defineProperty(navigator, 'clipboard', {
        value: { writeText },
        writable: true,
        configurable: true,
      });

      const points = [makePoint()];
      renderWithTheme(<ObservationTable points={points} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      fireEvent.contextMenu(rows[1]);

      expect(screen.getByText('Copy Row as JSON')).toBeInTheDocument();

      fireEvent.click(screen.getByText('Copy Row as JSON'));

      // MUI Menu hides the Popover by setting aria-hidden; wait for it to complete
      await waitFor(() => {
        const popover = document.querySelector('[role="presentation"]');
        // Either the presentation container is hidden or removed from the DOM
        if (popover) {
          expect(popover).toHaveAttribute('aria-hidden', 'true');
        } else {
          // Already removed
          expect(popover).toBeNull();
        }
      });
    });
  });

  describe('Row Hover Behavior', () => {
    it('changes background on mouseEnter', () => {
      const points = [makePoint()];
      renderWithTheme(<ObservationTable points={points} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      const dataRow = rows[1];

      fireEvent.mouseEnter(dataRow);
      expect(dataRow.style.backgroundColor).toBe('rgba(255, 255, 255, 0.03)');
    });

    it('resets background on mouseLeave', () => {
      const points = [makePoint()];
      renderWithTheme(<ObservationTable points={points} metaKeys={[]} />);

      const rows = screen.getAllByRole('row');
      const dataRow = rows[1];

      fireEvent.mouseEnter(dataRow);
      fireEvent.mouseLeave(dataRow);
      expect(dataRow.style.backgroundColor).toBe('transparent');
    });
  });

  describe('Observation Table Container', () => {
    it('renders with data-testid="observation-table"', () => {
      renderWithTheme(<ObservationTable points={[makePoint()]} metaKeys={[]} />);
      expect(screen.getByTestId('observation-table')).toBeInTheDocument();
    });

    it('renders empty table with no rows when points is empty', () => {
      renderWithTheme(<ObservationTable points={[]} metaKeys={[]} />);
      const rows = screen.getAllByRole('row');
      // Only the header row
      expect(rows).toHaveLength(1);
    });
  });
});
