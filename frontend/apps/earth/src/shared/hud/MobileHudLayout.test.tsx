/**
 * MobileHudLayout Component Tests
 *
 * Tests for the top bar (branding + connection status) and the bottom
 * telemetry strip of the mobile HUD.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import MobileHudLayout from './MobileHudLayout';
import { useUIStore } from '@/app/store';
import { useViewerStore } from '../../features/globe/store';
import type { WSConnectionStatus } from '@respondent/core';

// ─── Mock dependencies ────────────────────────────────────────────────────────

// useWebSocketStatus is backed by a singleton WebSocket; replace with a
// controllable mock so tests are deterministic and network-free.
const mockWsStatus = vi.fn<() => WSConnectionStatus>(() => 'connected');

vi.mock('@respondent/core', async () => ({
  ...(await vi.importActual('@respondent/core')),
  useWebSocketStatus: () => mockWsStatus(),
}));

// ─── Helpers ──────────────────────────────────────────────────────────────────

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

const defaultCamera = {
  lat: 37.77,
  lon: -122.42,
  altitude: 15_000,
  heading: 0,
  pitch: -90,
  roll: 0,
};

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('MobileHudLayout', () => {
  beforeEach(() => {
    mockWsStatus.mockReturnValue('connected');
    useUIStore.setState({ timeMode: 'live', recordingMode: false });
    useViewerStore.setState({ camera: defaultCamera });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  // ─── Guard render ───────────────────────────────────────────────────────────

  describe('when recordingMode is true', () => {
    it('returns null — nothing is rendered', () => {
      useUIStore.setState({ recordingMode: true });
      const { container } = renderWithTheme(<MobileHudLayout />);
      expect(container.firstChild).toBeNull();
    });

    it('does not render the top bar', () => {
      useUIStore.setState({ recordingMode: true });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.queryByTestId('mobile-hud-top')).not.toBeInTheDocument();
    });

    it('does not render the telemetry strip', () => {
      useUIStore.setState({ recordingMode: true });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.queryByTestId('mobile-hud-telemetry')).not.toBeInTheDocument();
    });
  });

  // ─── Top bar ────────────────────────────────────────────────────────────────

  describe('Top bar', () => {
    it('renders the top bar container', () => {
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByTestId('mobile-hud-top')).toBeInTheDocument();
    });

    it('displays "RESPONDENT" brand text', () => {
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('RESPONDENT')).toBeInTheDocument();
    });
  });

  // ─── Connection status labels ────────────────────────────────────────────────

  describe('Connection status', () => {
    it('shows "LIVE" when connected and timeMode is "live"', () => {
      mockWsStatus.mockReturnValue('connected');
      useUIStore.setState({ timeMode: 'live' });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('LIVE')).toBeInTheDocument();
    });

    it('shows "RANGE" when connected and timeMode is "range"', () => {
      mockWsStatus.mockReturnValue('connected');
      useUIStore.setState({ timeMode: 'range' });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('RANGE')).toBeInTheDocument();
    });

    it('shows "RECONN" when status is "reconnecting"', () => {
      mockWsStatus.mockReturnValue('reconnecting');
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('RECONN')).toBeInTheDocument();
    });

    it('shows "OFFLINE" when status is "disconnected"', () => {
      mockWsStatus.mockReturnValue('disconnected');
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('OFFLINE')).toBeInTheDocument();
    });

    it('does not show "LIVE" when reconnecting', () => {
      mockWsStatus.mockReturnValue('reconnecting');
      renderWithTheme(<MobileHudLayout />);
      expect(screen.queryByText('LIVE')).not.toBeInTheDocument();
    });

    it('does not show "OFFLINE" when connected', () => {
      mockWsStatus.mockReturnValue('connected');
      useUIStore.setState({ timeMode: 'live' });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.queryByText('OFFLINE')).not.toBeInTheDocument();
    });
  });

  // ─── Telemetry strip ────────────────────────────────────────────────────────

  describe('Telemetry strip', () => {
    it('renders the telemetry container', () => {
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByTestId('mobile-hud-telemetry')).toBeInTheDocument();
    });

    it('displays formatted coordinates when camera is set', () => {
      useViewerStore.setState({
        camera: { lat: 37.77, lon: -122.42, altitude: 15_000, heading: 0, pitch: -90, roll: 0 },
      });
      renderWithTheme(<MobileHudLayout />);
      // formatCoord(37.77, -122.42) → "37.77N 122.42W"
      expect(screen.getByText('37.77N 122.42W')).toBeInTheDocument();
    });

    it('displays formatted altitude in metres for low altitudes', () => {
      useViewerStore.setState({
        camera: { ...defaultCamera, altitude: 500 },
      });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText(/ALT:\s*500 m/)).toBeInTheDocument();
    });

    it('displays altitude in km for altitudes >= 1000 m', () => {
      useViewerStore.setState({
        camera: { ...defaultCamera, altitude: 15_000 },
      });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText(/ALT:\s*15\.0 km/)).toBeInTheDocument();
    });

    it('displays altitude in millions for very high altitudes', () => {
      useViewerStore.setState({
        camera: { ...defaultCamera, altitude: 18_000_000 },
      });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText(/ALT:\s*18\.0M m/)).toBeInTheDocument();
    });

    it('shows placeholder "---" when camera is null', () => {
      useViewerStore.setState({ camera: null });
      renderWithTheme(<MobileHudLayout />);
      const dashes = screen.getAllByText('---');
      expect(dashes.length).toBeGreaterThanOrEqual(1);
    });

    it('formats southern latitude with "S" suffix', () => {
      useViewerStore.setState({
        camera: { ...defaultCamera, lat: -33.87, lon: 151.21 },
      });
      renderWithTheme(<MobileHudLayout />);
      expect(screen.getByText('33.87S 151.21E')).toBeInTheDocument();
    });
  });
});
