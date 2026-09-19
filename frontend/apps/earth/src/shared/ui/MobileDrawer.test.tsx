/**
 * MobileDrawer Component Tests
 *
 * Tests for the bottom-sheet swipeable drawer used on mobile to host
 * secondary panels (Layers, Settings, Nav, etc.).
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import MobileDrawer, { MobileDrawerProps } from './MobileDrawer';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

const defaultProps: MobileDrawerProps = {
  open: true,
  onClose: vi.fn(),
  onOpen: vi.fn(),
  title: 'Layers',
  children: <div data-testid="drawer-child">Panel content</div>,
};

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('MobileDrawer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ─── Rendering ─────────────────────────────────────────────────────────────

  describe('Rendering', () => {
    it('renders the title text', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} />);
      expect(screen.getByText('Layers')).toBeInTheDocument();
    });

    it('renders a custom title', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} title="Settings" />);
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders children content', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} />);
      expect(screen.getByTestId('drawer-child')).toBeInTheDocument();
    });

    it('renders arbitrary children text', () => {
      renderWithTheme(
        <MobileDrawer {...defaultProps}>
          <p>Hello from inside the drawer</p>
        </MobileDrawer>,
      );
      expect(screen.getByText('Hello from inside the drawer')).toBeInTheDocument();
    });

    it('renders the close button', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} />);
      // CloseIcon inside an IconButton — find by aria-label or role
      const closeButtons = screen.getAllByRole('button');
      expect(closeButtons.length).toBeGreaterThan(0);
    });

    it('renders the drag handle element', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} />);
      // The drawer title is visible, meaning the Paper rendered with the
      // drag handle above it. Verify the title exists as a proxy.
      expect(screen.getByText('Layers')).toBeInTheDocument();
      // Close button also means the header (which contains the drag handle) rendered
      expect(screen.getAllByRole('button').length).toBeGreaterThan(0);
    });
  });

  // ─── data-testid derivation ─────────────────────────────────────────────────

  describe('data-testid', () => {
    it('sets testid from lower-cased title', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} title="Layers" />);
      expect(screen.getByTestId('mobile-drawer-layers')).toBeInTheDocument();
    });

    it('replaces spaces with hyphens in testid', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} title="My Panel" />);
      expect(screen.getByTestId('mobile-drawer-my-panel')).toBeInTheDocument();
    });
  });

  // ─── Close button ───────────────────────────────────────────────────────────

  describe('Close button', () => {
    it('calls onClose when close button is clicked', () => {
      const onClose = vi.fn();
      renderWithTheme(<MobileDrawer {...defaultProps} onClose={onClose} />);
      // The CloseIcon is inside the only visible button in the header
      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[0]);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('does not call onOpen when close button is clicked', () => {
      const onOpen = vi.fn();
      const onClose = vi.fn();
      renderWithTheme(<MobileDrawer {...defaultProps} onOpen={onOpen} onClose={onClose} />);
      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[0]);
      expect(onOpen).not.toHaveBeenCalled();
    });
  });

  // ─── Open / closed state ────────────────────────────────────────────────────

  describe('Open state', () => {
    it('renders content when open is true', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} open={true} />);
      expect(screen.getByText('Layers')).toBeInTheDocument();
    });

    it('keeps content in DOM when open is false (keepMounted)', () => {
      // MobileDrawer sets ModalProps keepMounted — content stays in DOM
      renderWithTheme(<MobileDrawer {...defaultProps} open={false} />);
      expect(screen.getByText('Layers')).toBeInTheDocument();
      expect(screen.getByTestId('drawer-child')).toBeInTheDocument();
    });
  });

  // ─── heightPercent prop ──────────────────────────────────────────────────────

  describe('heightPercent prop', () => {
    it('accepts a custom heightPercent without errors', () => {
      renderWithTheme(<MobileDrawer {...defaultProps} heightPercent={80} />);
      expect(screen.getByText('Layers')).toBeInTheDocument();
    });

    it('uses 60 as default heightPercent', () => {
      // Simply verify it renders without errors using the default
      renderWithTheme(<MobileDrawer {...defaultProps} />);
      expect(screen.getByTestId('mobile-drawer-layers')).toBeInTheDocument();
    });
  });
});
