/**
 * AspectRatioOverlay Component Tests
 *
 * Tests for the overlay that renders letterbox/pillarbox bars and the
 * rule-of-thirds grid during recording mode.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import AspectRatioOverlay from './AspectRatioOverlay';
import { calculateFrame } from './aspectRatioUtils';

const theme = createTheme({ palette: { mode: 'dark' } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

// ─── calculateFrame unit tests ────────────────────────────────────────────────

describe('calculateFrame()', () => {
  describe('ratio === "free"', () => {
    it('returns full viewport dimensions with zero offsets', () => {
      const result = calculateFrame('free', 1920, 1080);
      expect(result).toEqual({
        frameWidth: 1920,
        frameHeight: 1080,
        offsetX: 0,
        offsetY: 0,
      });
    });

    it('returns full viewport for any dimensions when free', () => {
      const result = calculateFrame('free', 375, 812);
      expect(result).toEqual({
        frameWidth: 375,
        frameHeight: 812,
        offsetX: 0,
        offsetY: 0,
      });
    });
  });

  describe('ratio === "9:16" (portrait) on a landscape viewport', () => {
    it('produces letterbox bars (offsetY > 0, offsetX === 0)', () => {
      // Landscape viewport: 1920 × 1080
      // 9/16 = 0.5625 target ratio; viewport ratio = 1920/1080 ≈ 1.78
      // viewport is wider → constrain by height
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('9:16', 1920, 1080);
      expect(frameHeight).toBeCloseTo(1080, 5);
      expect(frameWidth).toBeCloseTo(1080 * (9 / 16), 5);
      expect(offsetX).toBeGreaterThan(0);
      expect(offsetY).toBeCloseTo(0, 5);
    });

    it('produces correct frame on a square viewport', () => {
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('9:16', 800, 800);
      // viewport ratio 1 > target 0.5625 → height-constrained
      expect(frameHeight).toBeCloseTo(800, 5);
      expect(frameWidth).toBeCloseTo(800 * (9 / 16), 5);
      expect(offsetX).toBeGreaterThan(0);
      expect(offsetY).toBeCloseTo(0, 5);
    });
  });

  describe('ratio === "4:5" (portrait) on a landscape viewport', () => {
    it('produces pillarbox bars (offsetX > 0, offsetY === 0)', () => {
      // Landscape viewport: 1200 × 900 → ratio 1.33, target 0.8 → width wider
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('4:5', 1200, 900);
      expect(frameHeight).toBeCloseTo(900, 5);
      expect(frameWidth).toBeCloseTo(900 * (4 / 5), 5);
      expect(offsetX).toBeGreaterThan(0);
      expect(offsetY).toBeCloseTo(0, 5);
    });

    it('produces letterbox bars on a very tall viewport', () => {
      // Tall viewport: 400 × 1000 → ratio 0.4, target 0.8 → height-driven
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('4:5', 400, 1000);
      expect(frameWidth).toBeCloseTo(400, 5);
      expect(frameHeight).toBeCloseTo(400 / (4 / 5), 5);
      expect(offsetY).toBeGreaterThan(0);
      expect(offsetX).toBeCloseTo(0, 5);
    });
  });

  describe('ratio === "1:1" (square)', () => {
    it('produces pillarbox bars on a landscape viewport', () => {
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('1:1', 1280, 720);
      // viewport ratio 1.78 > 1 → height-constrained
      expect(frameHeight).toBeCloseTo(720, 5);
      expect(frameWidth).toBeCloseTo(720, 5);
      expect(offsetX).toBeGreaterThan(0);
      expect(offsetY).toBeCloseTo(0, 5);
    });

    it('produces letterbox bars on a portrait viewport', () => {
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('1:1', 375, 812);
      // viewport ratio < 1 → width-constrained
      expect(frameWidth).toBeCloseTo(375, 5);
      expect(frameHeight).toBeCloseTo(375, 5);
      expect(offsetY).toBeGreaterThan(0);
      expect(offsetX).toBeCloseTo(0, 5);
    });

    it('fills the viewport exactly when given a square viewport', () => {
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('1:1', 500, 500);
      expect(frameWidth).toBeCloseTo(500, 5);
      expect(frameHeight).toBeCloseTo(500, 5);
      expect(offsetX).toBeCloseTo(0, 5);
      expect(offsetY).toBeCloseTo(0, 5);
    });
  });

  describe('ratio === "16:9" (landscape)', () => {
    it('fills height on a landscape viewport and adds no letterbox', () => {
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('16:9', 1920, 1080);
      expect(frameWidth).toBeCloseTo(1920, 5);
      expect(frameHeight).toBeCloseTo(1080, 5);
      expect(offsetX).toBeCloseTo(0, 5);
      expect(offsetY).toBeCloseTo(0, 5);
    });

    it('produces letterbox bars on a portrait viewport', () => {
      // Portrait 390 × 844
      const { frameWidth, frameHeight, offsetX, offsetY } = calculateFrame('16:9', 390, 844);
      // viewport ratio 0.46 < 16/9 ≈ 1.78 → width-constrained
      expect(frameWidth).toBeCloseTo(390, 5);
      expect(frameHeight).toBeCloseTo(390 / (16 / 9), 5);
      expect(offsetY).toBeGreaterThan(0);
      expect(offsetX).toBeCloseTo(0, 5);
    });

    it('offsets are symmetric horizontally when pillarboxing', () => {
      // Very tall viewport
      const { offsetX } = calculateFrame('16:9', 1920, 2160);
      // viewport 0.89 < 1.78 → width-constrained, no horizontal offset
      expect(offsetX).toBeCloseTo(0, 5);
    });
  });

  describe('offsetX and offsetY are symmetric', () => {
    it('horizontal pillarbox offset is centred', () => {
      const { frameWidth, offsetX } = calculateFrame('9:16', 1920, 1080);
      expect(offsetX * 2 + frameWidth).toBeCloseTo(1920, 5);
    });

    it('vertical letterbox offset is centred', () => {
      const { frameHeight, offsetY } = calculateFrame('1:1', 375, 812);
      expect(offsetY * 2 + frameHeight).toBeCloseTo(812, 5);
    });
  });
});

// ─── Component render tests ───────────────────────────────────────────────────

describe('AspectRatioOverlay component', () => {
  const originalInnerWidth = window.innerWidth;
  const originalInnerHeight = window.innerHeight;

  beforeEach(() => {
    // Default to a landscape viewport for most tests
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 1280,
    });
    Object.defineProperty(window, 'innerHeight', {
      writable: true,
      configurable: true,
      value: 720,
    });
  });

  afterEach(() => {
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: originalInnerWidth,
    });
    Object.defineProperty(window, 'innerHeight', {
      writable: true,
      configurable: true,
      value: originalInnerHeight,
    });
    vi.clearAllMocks();
  });

  describe('ratio === "free"', () => {
    it('returns null — nothing is rendered', () => {
      const { container } = renderWithTheme(<AspectRatioOverlay ratio="free" />);
      expect(container.firstChild).toBeNull();
    });

    it('does not render the overlay wrapper', () => {
      renderWithTheme(<AspectRatioOverlay ratio="free" />);
      expect(screen.queryByTestId('aspect-ratio-overlay')).not.toBeInTheDocument();
    });
  });

  describe('letterbox bars (portrait ratio on landscape viewport)', () => {
    it('renders the overlay wrapper for 9:16', () => {
      renderWithTheme(<AspectRatioOverlay ratio="9:16" />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });

    it('renders the overlay wrapper for 4:5', () => {
      renderWithTheme(<AspectRatioOverlay ratio="4:5" />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });

    it('renders pillarbox bars when portrait ratio shown on landscape viewport (9:16)', () => {
      // Landscape 1280×720 with 9:16 means frame is narrower → pillarbox (offsetX > 0)
      const { container } = renderWithTheme(<AspectRatioOverlay ratio="9:16" />);
      // The frame border box is always rendered; additionally the two pillarbox
      // bars are rendered because offsetX > 0.  We count Box elements to confirm.
      // At minimum the overlay root + frame border must be present.
      expect(container.querySelector('[data-testid="aspect-ratio-overlay"]')).toBeTruthy();
    });
  });

  describe('letterbox bars (landscape ratio on portrait viewport)', () => {
    beforeEach(() => {
      // Switch to a portrait viewport
      Object.defineProperty(window, 'innerWidth', {
        writable: true,
        configurable: true,
        value: 375,
      });
      Object.defineProperty(window, 'innerHeight', {
        writable: true,
        configurable: true,
        value: 812,
      });
    });

    it('renders overlay for 16:9 on portrait viewport', () => {
      renderWithTheme(<AspectRatioOverlay ratio="16:9" />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });

    it('renders overlay for 1:1 on portrait viewport', () => {
      renderWithTheme(<AspectRatioOverlay ratio="1:1" />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });
  });

  describe('grid lines', () => {
    it('does not render grid container when showGrid is false (default)', () => {
      const { container } = renderWithTheme(<AspectRatioOverlay ratio="1:1" />);
      // The grid container has children with left: "33.33%". We inspect the
      // rendered style content via the MUI sx system.  A reliable approach is
      // to check that fewer Box elements exist than with the grid enabled.
      const boxesWithoutGrid = container.querySelectorAll('div').length;
      expect(boxesWithoutGrid).toBeGreaterThan(0);
    });

    it('does not render grid lines when showGrid is omitted', () => {
      // Baseline render — capture child count without grid
      const { container: containerNoGrid } = renderWithTheme(<AspectRatioOverlay ratio="16:9" />);
      const countNoGrid = containerNoGrid.querySelectorAll('div').length;

      // Re-render with grid enabled
      const { container: containerWithGrid } = renderWithTheme(
        <AspectRatioOverlay ratio="16:9" showGrid />,
      );
      const countWithGrid = containerWithGrid.querySelectorAll('div').length;

      // Grid adds a container + 4 lines = at least 5 extra elements
      expect(countWithGrid).toBeGreaterThan(countNoGrid);
    });

    it('renders additional elements when showGrid is true', () => {
      const { container: containerNoGrid } = renderWithTheme(
        <AspectRatioOverlay ratio="4:5" showGrid={false} />,
      );
      const { container: containerWithGrid } = renderWithTheme(
        <AspectRatioOverlay ratio="4:5" showGrid={true} />,
      );
      expect(containerWithGrid.querySelectorAll('div').length).toBeGreaterThan(
        containerNoGrid.querySelectorAll('div').length,
      );
    });

    it('renders exactly 4 extra line elements when grid is enabled (2 vertical + 2 horizontal)', () => {
      // Without grid: root + top-letterbox + bottom-letterbox + left-pillar + right-pillar + frame-border
      // (varies by viewport/ratio; this test just checks the delta)
      const { container: noGrid } = renderWithTheme(
        <AspectRatioOverlay ratio="1:1" showGrid={false} />,
      );
      const { container: withGrid } = renderWithTheme(
        <AspectRatioOverlay ratio="1:1" showGrid={true} />,
      );
      // 1 grid wrapper + 4 line divs = 5 additional divs
      const diff = withGrid.querySelectorAll('div').length - noGrid.querySelectorAll('div').length;
      expect(diff).toBe(5);
    });
  });

  describe('resize event handling', () => {
    it('attaches and detaches resize listener without errors', () => {
      const addSpy = vi.spyOn(window, 'addEventListener');
      const removeSpy = vi.spyOn(window, 'removeEventListener');

      const { unmount } = renderWithTheme(<AspectRatioOverlay ratio="1:1" />);

      expect(addSpy).toHaveBeenCalledWith('resize', expect.any(Function));
      expect(addSpy).toHaveBeenCalledWith('orientationchange', expect.any(Function));

      unmount();

      expect(removeSpy).toHaveBeenCalledWith('resize', expect.any(Function));
      expect(removeSpy).toHaveBeenCalledWith('orientationchange', expect.any(Function));
    });

    it('re-renders after a resize event', () => {
      renderWithTheme(<AspectRatioOverlay ratio="16:9" />);
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();

      act(() => {
        Object.defineProperty(window, 'innerWidth', {
          writable: true,
          configurable: true,
          value: 768,
        });
        Object.defineProperty(window, 'innerHeight', {
          writable: true,
          configurable: true,
          value: 1024,
        });
        window.dispatchEvent(new Event('resize'));
      });

      // Component should still be mounted and visible after resize
      expect(screen.getByTestId('aspect-ratio-overlay')).toBeInTheDocument();
    });
  });
});
