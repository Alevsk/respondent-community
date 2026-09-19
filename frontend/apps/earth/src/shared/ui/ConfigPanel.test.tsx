/**
 * ConfigPanel Component Tests
 *
 * Tests for the foundational floating panel component used across the application.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import ConfigPanel, { ConfigPanelProps, ConfigPanelDisplayMode } from './ConfigPanel';
import { usePanelLayoutStore } from '../layout/panelLayoutStore';

// ─── Module mock for useResponsive ───────────────────────────────────────────
const mockUseResponsive = vi.fn(() => ({ isMobile: false, isTablet: false, isDesktop: true }));
vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@respondent/core');
  return { ...actual, useResponsive: () => mockUseResponsive() };
});

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

describe('ConfigPanel', () => {
  const defaultProps: ConfigPanelProps = {
    open: true,
    onClose: vi.fn(),
    title: 'Test Panel',
    children: <div>Panel Content</div>,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render when open is true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
      expect(screen.getByText('Test Panel')).toBeInTheDocument();
    });

    it('should be hidden (but still in DOM) when open is false due to Fade behavior', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} open={false} />);
      // MUI Fade doesn't remove from DOM, just hides with style
      const panel = screen.getByTestId('config-panel');
      expect(panel).toHaveStyle({ opacity: 0, visibility: 'hidden' });
    });

    it('should render children content', () => {
      renderWithTheme(
        <ConfigPanel {...defaultProps}>
          <div data-testid="child-content">Custom Content</div>
        </ConfigPanel>,
      );
      expect(screen.getByTestId('child-content')).toBeInTheDocument();
      expect(screen.getByText('Custom Content')).toBeInTheDocument();
    });

    it('should render with custom data-testid', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} data-testid="custom-panel" />);
      expect(screen.getByTestId('custom-panel')).toBeInTheDocument();
    });
  });

  describe('Header', () => {
    it('should render title', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} title="Custom Title" />);
      const title = screen.getByTestId('config-panel-title');
      // Note: textTransform: uppercase is CSS, not applied to text content
      expect(title).toHaveTextContent('Custom Title');
    });

    it('should render icon when provided', () => {
      renderWithTheme(
        <ConfigPanel {...defaultProps} icon={<span data-testid="panel-icon">Icon</span>} />,
      );
      expect(screen.getByTestId('config-panel-icon')).toBeInTheDocument();
    });

    it('should not render icon when not provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.queryByTestId('config-panel-icon')).not.toBeInTheDocument();
    });

    it('should render close button by default', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.getByTestId('config-panel-close')).toBeInTheDocument();
    });

    it('should hide close button when hideCloseButton is true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} hideCloseButton />);
      expect(screen.queryByTestId('config-panel-close')).not.toBeInTheDocument();
    });
  });

  describe('Status Bar', () => {
    it('should render status bar when statusLabel and statusValue are provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} statusLabel="STATUS" statusValue="Active" />);
      expect(screen.getByTestId('config-panel-status')).toBeInTheDocument();
      expect(screen.getByText('STATUS')).toBeInTheDocument();
      expect(screen.getByTestId('config-panel-status-value')).toHaveTextContent('Active');
    });

    it('should not render status bar when statusLabel is missing', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} statusValue="Active" />);
      expect(screen.queryByTestId('config-panel-status')).not.toBeInTheDocument();
    });

    it('should not render status bar when statusValue is missing', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} statusLabel="STATUS" />);
      expect(screen.queryByTestId('config-panel-status')).not.toBeInTheDocument();
    });
  });

  describe('Footer', () => {
    it('should render footer when provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} footer="Help text here" />);
      expect(screen.getByTestId('config-panel-footer')).toBeInTheDocument();
      expect(screen.getByText('Help text here')).toBeInTheDocument();
    });

    it('should not render footer when not provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.queryByTestId('config-panel-footer')).not.toBeInTheDocument();
    });
  });

  describe('Interactions', () => {
    it('should call onClose when close button is clicked', async () => {
      const onClose = vi.fn();
      renderWithTheme(<ConfigPanel {...defaultProps} onClose={onClose} />);

      await userEvent.click(screen.getByTestId('config-panel-close'));
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('should have accessible close button with aria-label', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      const closeButton = screen.getByLabelText('Close panel');
      expect(closeButton).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('should have role="dialog"', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should have aria-labelledby pointing to title', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} title="Accessible Panel" />);
      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('aria-labelledby', 'config-panel-title');
    });

    it('should have aria-modal="false"', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('aria-modal', 'false');
    });
  });

  describe('Props', () => {
    it('should accept width prop', () => {
      // MUI sx props are processed differently, just verify no error
      renderWithTheme(<ConfigPanel {...defaultProps} width={400} />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });

    it('should accept positioning props', () => {
      // MUI sx props are processed differently, just verify no error
      renderWithTheme(<ConfigPanel {...defaultProps} top={100} right={20} />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });

    it('should accept maxHeight prop', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} maxHeight="500px" />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });

    it('should hide header divider when hideHeaderDivider is true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} hideHeaderDivider />);
      // Component should render without errors
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });
  });

  describe('Animation', () => {
    it('should accept custom TransitionProps', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} TransitionProps={{ timeout: 500 }} />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });
  });

  describe('Custom Styling', () => {
    it('should accept sx prop without error', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} sx={{ border: '1px solid red' }} />);
      expect(screen.getByTestId('config-panel')).toBeInTheDocument();
    });
  });

  describe('Minimize/Maximize Button Rendering', () => {
    it('should not render minimize button by default', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.queryByTestId('config-panel-minimize')).not.toBeInTheDocument();
    });

    it('should not render maximize button by default', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      expect(screen.queryByTestId('config-panel-maximize')).not.toBeInTheDocument();
    });

    it('should render minimize button when minimizable is true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable />);
      expect(screen.getByTestId('config-panel-minimize')).toBeInTheDocument();
    });

    it('should render maximize button when maximizable is true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} maximizable />);
      expect(screen.getByTestId('config-panel-maximize')).toBeInTheDocument();
    });

    it('should render both buttons when both minimizable and maximizable are true', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable maximizable />);
      expect(screen.getByTestId('config-panel-minimize')).toBeInTheDocument();
      expect(screen.getByTestId('config-panel-maximize')).toBeInTheDocument();
    });
  });

  describe('Minimize/Maximize Interactions', () => {
    it('should call onDisplayModeChange with "minimized" when minimize is clicked in normal mode', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-minimize'));
      expect(onDisplayModeChange).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).toHaveBeenCalledWith('minimized');
    });

    it('should call onDisplayModeChange with "normal" when minimize is clicked while already minimized', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-minimize'));
      expect(onDisplayModeChange).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).toHaveBeenCalledWith('normal');
    });

    it('should call onDisplayModeChange with "maximized" when maximize is clicked in normal mode', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          maximizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-maximize'));
      expect(onDisplayModeChange).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).toHaveBeenCalledWith('maximized');
    });

    it('should call onDisplayModeChange with "normal" when maximize is clicked while already maximized', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          maximizable
          displayMode="maximized"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-maximize'));
      expect(onDisplayModeChange).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).toHaveBeenCalledWith('normal');
    });
  });

  describe('Minimized Display Mode', () => {
    it('should hide content area when displayMode is "minimized"', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable displayMode="minimized" />);
      expect(screen.queryByTestId('config-panel-content')).not.toBeInTheDocument();
    });

    it('should hide status bar when displayMode is "minimized"', () => {
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          statusLabel="STATUS"
          statusValue="Active"
        />,
      );
      expect(screen.queryByTestId('config-panel-status')).not.toBeInTheDocument();
    });

    it('should hide footer when displayMode is "minimized"', () => {
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          footer="Help text here"
        />,
      );
      expect(screen.queryByTestId('config-panel-footer')).not.toBeInTheDocument();
    });

    it('should still show header when displayMode is "minimized"', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable displayMode="minimized" />);
      expect(screen.getByTestId('config-panel-title')).toBeInTheDocument();
    });

    it('should still show icon when displayMode is "minimized"', () => {
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          icon={<span data-testid="panel-icon">Icon</span>}
        />,
      );
      expect(screen.getByTestId('config-panel-icon')).toBeInTheDocument();
    });

    it('should show minimizedTitle in title when displayMode is "minimized" and minimizedTitle is provided', () => {
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          minimizedTitle="Minimized Label"
        />,
      );
      expect(screen.getByTestId('config-panel-title')).toHaveTextContent('Minimized Label');
    });

    it('should show regular title when displayMode is "minimized" and minimizedTitle is not provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable displayMode="minimized" />);
      expect(screen.getByTestId('config-panel-title')).toHaveTextContent('Test Panel');
    });

    it('should apply text truncation styles to title when minimized with long minimizedTitle', () => {
      const longTitle =
        'Flood Warning issued March 11 at 11:04AM CDT until March 12 at 8:12PM CDT by NWS Paducah KY';
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          minimizedTitle={longTitle}
        />,
      );
      const title = screen.getByTestId('config-panel-title');
      expect(title).toHaveTextContent(longTitle);
      expect(title).toHaveStyle({
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
      });
    });

    it('should display long statusValue fully with word wrapping', () => {
      const longName =
        'Flood Warning issued March 11 at 11:04AM CDT until March 12 at 8:12PM CDT by NWS Paducah KY';
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          statusLabel="SELECTED"
          statusValue={longName}
        />,
      );
      const statusValue = screen.getByTestId('config-panel-status-value');
      expect(statusValue).toHaveTextContent(longName);
    });

    it('should show regular title when displayMode is "normal" even if minimizedTitle is provided', () => {
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          minimizedTitle="Minimized Label"
        />,
      );
      expect(screen.getByTestId('config-panel-title')).toHaveTextContent('Test Panel');
    });
  });

  describe('Minimize/Maximize Aria Labels', () => {
    it('should have aria-label "Minimize panel" on minimize button in normal mode', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable displayMode="normal" />);
      expect(screen.getByLabelText('Minimize panel')).toBeInTheDocument();
    });

    it('should have aria-label "Restore panel" on minimize button when already minimized', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} minimizable displayMode="minimized" />);
      expect(screen.getByLabelText('Restore panel')).toBeInTheDocument();
    });

    it('should have aria-label "Maximize panel" on maximize button in normal mode', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} maximizable displayMode="normal" />);
      expect(screen.getByLabelText('Maximize panel')).toBeInTheDocument();
    });

    it('should have aria-label "Restore panel" on maximize button when already maximized', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} maximizable displayMode="maximized" />);
      expect(screen.getByLabelText('Restore panel')).toBeInTheDocument();
    });
  });

  describe('DisplayMode Type', () => {
    it('should accept all valid ConfigPanelDisplayMode values without error', () => {
      const modes: ConfigPanelDisplayMode[] = ['normal', 'minimized', 'maximized'];
      modes.forEach((mode) => {
        const { unmount } = renderWithTheme(
          <ConfigPanel {...defaultProps} minimizable maximizable displayMode={mode} />,
        );
        expect(screen.getByTestId('config-panel')).toBeInTheDocument();
        unmount();
      });
    });
  });

  describe('Mobile header tap-to-minimize', () => {
    beforeEach(() => {
      mockUseResponsive.mockReturnValue({ isMobile: true, isTablet: false, isDesktop: false });
    });

    afterEach(() => {
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });
    });

    it('should call onDisplayModeChange with "minimized" when header is tapped on mobile', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      // Click the header area (the title text is inside the header)
      await userEvent.click(screen.getByTestId('config-panel-title'));
      expect(onDisplayModeChange).toHaveBeenCalledWith('minimized');
    });

    it('should call onDisplayModeChange with "normal" when minimized header is tapped on mobile', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="minimized"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-title'));
      expect(onDisplayModeChange).toHaveBeenCalledWith('normal');
    });

    it('should not trigger header tap when clicking close button on mobile', async () => {
      const onDisplayModeChange = vi.fn();
      const onClose = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
          onClose={onClose}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-close'));
      // Close should fire, but minimize should NOT fire (stopPropagation)
      expect(onClose).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).not.toHaveBeenCalled();
    });

    it('should not trigger header tap when clicking minimize button on mobile', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-minimize'));
      // Only the button handler should fire once, not doubled by header tap
      expect(onDisplayModeChange).toHaveBeenCalledTimes(1);
      expect(onDisplayModeChange).toHaveBeenCalledWith('minimized');
    });

    it('should not make header tappable when minimizable is false on mobile', async () => {
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable={false}
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-title'));
      expect(onDisplayModeChange).not.toHaveBeenCalled();
    });

    it('should not make header tappable on desktop even when minimizable', async () => {
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });
      const onDisplayModeChange = vi.fn();
      renderWithTheme(
        <ConfigPanel
          {...defaultProps}
          minimizable
          displayMode="normal"
          onDisplayModeChange={onDisplayModeChange}
        />,
      );
      await userEvent.click(screen.getByTestId('config-panel-title'));
      expect(onDisplayModeChange).not.toHaveBeenCalled();
    });
  });

  describe('Drag and Focus (panelId)', () => {
    beforeEach(() => {
      usePanelLayoutStore.setState({ panels: {}, _nextOrder: 0, zStack: [] });
    });

    it('applies dynamic z-index from zStack when panelId is provided', () => {
      usePanelLayoutStore.getState().bringToFront('test-panel');
      usePanelLayoutStore.getState().bringToFront('other-panel');
      usePanelLayoutStore.getState().bringToFront('test-panel');

      renderWithTheme(<ConfigPanel {...defaultProps} panelId="test-panel" />);

      const panel = screen.getByTestId('config-panel');
      // test-panel is at index 1 in zStack ['other-panel', 'test-panel']
      // z-index = 1100 + 1 + 1 = 1102
      expect(panel.style.zIndex).toBe('1102');
    });

    it('uses default z-index 1100 when panelId is not provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      const panel = screen.getByTestId('config-panel');
      expect(panel.style.zIndex).toBe('1100');
    });

    it('calls bringToFront on mousedown when panelId is provided', async () => {
      renderWithTheme(<ConfigPanel {...defaultProps} panelId="focus-test" />);

      const panel = screen.getByTestId('config-panel');
      await userEvent.pointer({ target: panel, keys: '[MouseLeft>]' });

      const { zStack } = usePanelLayoutStore.getState();
      expect(zStack).toContain('focus-test');
    });

    it('shows grab cursor on header when panelId is provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} panelId="cursor-test" />);

      const header = screen.getByTestId('config-panel-header');
      expect(header.style.cursor).toBe('grab');
    });

    it('does not show grab cursor when panelId is not provided', () => {
      renderWithTheme(<ConfigPanel {...defaultProps} />);
      const header = screen.getByTestId('config-panel-header');
      expect(header.style.cursor).not.toBe('grab');
    });
  });
});
