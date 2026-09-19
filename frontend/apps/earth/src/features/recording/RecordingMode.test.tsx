/**
 * RecordingMode Component Tests
 *
 * Tests for the composition component that mounts AspectRatioOverlay and
 * AspectRatioPicker when recording mode is active and calls clearSelection
 * on mount.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import RecordingMode from './RecordingMode';
import { useUIStore } from '@/app/store';
import type { AspectRatioKey } from '@respondent/core';

// Stub child components so tests are isolated from their implementation details.
vi.mock('./AspectRatioOverlay', () => ({
  default: ({ ratio, showGrid }: { ratio: string; showGrid: boolean }) => (
    <div data-testid="aspect-ratio-overlay" data-ratio={ratio} data-show-grid={showGrid} />
  ),
}));

vi.mock('./AspectRatioPicker', () => ({
  default: () => <div data-testid="aspect-ratio-picker" />,
}));

const theme = createTheme({ palette: { mode: 'dark' } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

describe('RecordingMode', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useUIStore.setState({
      recordingMode: false,
      recordingAspectRatio: '9:16' as AspectRatioKey,
      recordingShowGrid: false,
      selectedEntityId: null,
      selectedLayerId: null,
      selectedEntities: [],
      viewMode: 'globe',
      entityViewState: {},
      trailPoints: {},
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  // ─── Guard render ───────────────────────────────────────────────────────────

  describe('when recordingMode is false', () => {
    it('renders nothing', () => {
      const { container } = renderWithTheme(<RecordingMode />);
      expect(container.firstChild).toBeNull();
    });

    it('does not mount AspectRatioOverlay', () => {
      renderWithTheme(<RecordingMode />);
      expect(screen.queryByTestId('aspect-ratio-overlay')).not.toBeInTheDocument();
    });

    it('does not mount AspectRatioPicker', () => {
      renderWithTheme(<RecordingMode />);
      expect(screen.queryByTestId('aspect-ratio-picker')).not.toBeInTheDocument();
    });
  });

  // ─── Active render ──────────────────────────────────────────────────────────

  describe('when recordingMode is true', () => {
    beforeEach(() => {
      useUIStore.setState({ recordingMode: true });
    });

    it('renders AspectRatioOverlay', () => {
      renderWithTheme(<RecordingMode />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });

    it('renders AspectRatioPicker', () => {
      renderWithTheme(<RecordingMode />);
      expect(screen.getByTestId('aspect-ratio-picker')).toBeInTheDocument();
    });

    it('passes the current ratio to AspectRatioOverlay', () => {
      useUIStore.setState({ recordingAspectRatio: '16:9' });
      renderWithTheme(<RecordingMode />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toHaveAttribute('data-ratio', '16:9');
    });

    it('passes showGrid=true to AspectRatioOverlay when grid is enabled', () => {
      useUIStore.setState({ recordingShowGrid: true });
      renderWithTheme(<RecordingMode />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toHaveAttribute('data-show-grid', 'true');
    });

    it('passes showGrid=false to AspectRatioOverlay when grid is disabled', () => {
      useUIStore.setState({ recordingShowGrid: false });
      renderWithTheme(<RecordingMode />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toHaveAttribute('data-show-grid', 'false');
    });
  });

  // ─── clearSelection side-effect ─────────────────────────────────────────────

  describe('clearSelection on mount', () => {
    it('calls clearSelection when recordingMode is true on mount', () => {
      // Plant some selection state before mounting with recordingMode on
      useUIStore.setState({
        recordingMode: true,
        selectedEntityId: 'entity-123',
        selectedLayerId: 'layer-abc',
        selectedEntities: [{ entityId: 'entity-123', layerId: 'layer-abc' }],
      });

      renderWithTheme(<RecordingMode />);

      const state = useUIStore.getState();
      expect(state.selectedEntityId).toBeNull();
      expect(state.selectedLayerId).toBeNull();
      expect(state.selectedEntities).toHaveLength(0);
    });

    it('does not call clearSelection when recordingMode is false on mount', () => {
      useUIStore.setState({
        recordingMode: false,
        selectedEntityId: 'entity-456',
        selectedLayerId: 'layer-xyz',
        selectedEntities: [{ entityId: 'entity-456', layerId: 'layer-xyz' }],
      });

      renderWithTheme(<RecordingMode />);

      // Selection should be preserved because recordingMode is false
      const state = useUIStore.getState();
      expect(state.selectedEntityId).toBe('entity-456');
      expect(state.selectedEntities).toHaveLength(1);
    });

    it('resets viewMode to "globe" on clearSelection', () => {
      useUIStore.setState({
        recordingMode: true,
        viewMode: 'entity',
        selectedEntityId: 'entity-789',
        selectedLayerId: 'layer-def',
        selectedEntities: [{ entityId: 'entity-789', layerId: 'layer-def' }],
      });

      renderWithTheme(<RecordingMode />);

      expect(useUIStore.getState().viewMode).toBe('globe');
    });
  });
});
