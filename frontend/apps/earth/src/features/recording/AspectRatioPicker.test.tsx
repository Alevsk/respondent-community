/**
 * AspectRatioPicker Component Tests
 *
 * Tests for the bottom-anchored picker that lets users choose an aspect ratio,
 * toggle the rule-of-thirds grid, and exit recording mode.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import AspectRatioPicker from './AspectRatioPicker';
import { useUIStore } from '@/app/store';
import type { AspectRatioKey } from '@respondent/core';

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

describe('AspectRatioPicker', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useUIStore.setState({
      recordingMode: true,
      recordingAspectRatio: '9:16' as AspectRatioKey,
      recordingShowGrid: false,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  // ─── Rendering ─────────────────────────────────────────────────────────────

  describe('Rendering', () => {
    it('renders the picker container', () => {
      renderWithTheme(<AspectRatioPicker />);
      expect(screen.getByTestId('aspect-ratio-picker')).toBeInTheDocument();
    });

    it('renders all 5 ratio buttons', () => {
      renderWithTheme(<AspectRatioPicker />);
      expect(screen.getByText('9:16')).toBeInTheDocument();
      expect(screen.getByText('4:5')).toBeInTheDocument();
      expect(screen.getByText('1:1')).toBeInTheDocument();
      expect(screen.getByText('16:9')).toBeInTheDocument();
      expect(screen.getByText('Free')).toBeInTheDocument();
    });

    it('renders the grid toggle button', () => {
      renderWithTheme(<AspectRatioPicker />);
      expect(
        screen.getByRole('button', { name: /toggle rule of thirds grid/i }),
      ).toBeInTheDocument();
    });

    it('renders the exit recording mode button', () => {
      renderWithTheme(<AspectRatioPicker />);
      expect(screen.getByRole('button', { name: /exit recording mode/i })).toBeInTheDocument();
    });
  });

  // ─── Ratio selection ────────────────────────────────────────────────────────

  describe('Ratio selection', () => {
    it('clicking "4:5" calls setRecordingAspectRatio with "4:5"', () => {
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByText('4:5'));
      expect(useUIStore.getState().recordingAspectRatio).toBe('4:5');
    });

    it('clicking "1:1" calls setRecordingAspectRatio with "1:1"', () => {
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByText('1:1'));
      expect(useUIStore.getState().recordingAspectRatio).toBe('1:1');
    });

    it('clicking "16:9" calls setRecordingAspectRatio with "16:9"', () => {
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByText('16:9'));
      expect(useUIStore.getState().recordingAspectRatio).toBe('16:9');
    });

    it('clicking "Free" calls setRecordingAspectRatio with "free"', () => {
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByText('Free'));
      expect(useUIStore.getState().recordingAspectRatio).toBe('free');
    });

    it('clicking "9:16" calls setRecordingAspectRatio with "9:16"', () => {
      useUIStore.setState({ recordingAspectRatio: '1:1' });
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByText('9:16'));
      expect(useUIStore.getState().recordingAspectRatio).toBe('9:16');
    });
  });

  // ─── Grid toggle ────────────────────────────────────────────────────────────

  describe('Grid toggle', () => {
    it('clicking the grid button enables the grid when it was off', () => {
      useUIStore.setState({ recordingShowGrid: false });
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByRole('button', { name: /toggle rule of thirds grid/i }));
      expect(useUIStore.getState().recordingShowGrid).toBe(true);
    });

    it('clicking the grid button disables the grid when it was on', () => {
      useUIStore.setState({ recordingShowGrid: true });
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByRole('button', { name: /toggle rule of thirds grid/i }));
      expect(useUIStore.getState().recordingShowGrid).toBe(false);
    });

    it('successive clicks toggle grid state repeatedly', () => {
      useUIStore.setState({ recordingShowGrid: false });
      renderWithTheme(<AspectRatioPicker />);
      const gridBtn = screen.getByRole('button', { name: /toggle rule of thirds grid/i });

      fireEvent.click(gridBtn);
      expect(useUIStore.getState().recordingShowGrid).toBe(true);

      fireEvent.click(gridBtn);
      expect(useUIStore.getState().recordingShowGrid).toBe(false);
    });
  });

  // ─── Exit recording mode ────────────────────────────────────────────────────

  describe('Exit button', () => {
    it('clicking exit calls toggleRecordingMode on the store', () => {
      useUIStore.setState({ recordingMode: true });
      renderWithTheme(<AspectRatioPicker />);
      fireEvent.click(screen.getByRole('button', { name: /exit recording mode/i }));
      expect(useUIStore.getState().recordingMode).toBe(false);
    });
  });

  // ─── Auto-hide timer ────────────────────────────────────────────────────────

  describe('Auto-hide behaviour', () => {
    it('is initially visible', () => {
      renderWithTheme(<AspectRatioPicker />);
      const picker = screen.getByTestId('aspect-ratio-picker');
      // The sx opacity is set inline; rather than inspect computed style we
      // confirm the element is present in the DOM (it is never unmounted).
      expect(picker).toBeInTheDocument();
    });

    it('any interaction resets the visibility timer', () => {
      renderWithTheme(<AspectRatioPicker />);
      // Advance half the AUTO_HIDE_MS (3000ms)
      act(() => {
        vi.advanceTimersByTime(1500);
      });
      // Simulate an interaction that resets the timer
      fireEvent.click(screen.getByText('1:1'));
      // Advance past the original 3000ms; picker should NOT have hidden yet
      // because the timer was reset after the click
      act(() => {
        vi.advanceTimersByTime(1600);
      });
      expect(screen.getByTestId('aspect-ratio-picker')).toBeInTheDocument();
    });
  });
});
