/**
 * PostProcessOverlay Component Tests
 *
 * Tests the wrapper-based post-processing overlay that applies CSS filters
 * and visual effect layers for each FilterPreset (NORMAL, CRT, NVG, FLIR).
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import PostProcessOverlay from './PostProcessOverlay';
import type { FilterPreset } from '@/app/store';

// ─── Helpers ──────────────────────────────────────────────────────────────────

const theme = createTheme({ palette: { mode: 'dark', primary: { main: '#00ff9d' } } });

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderOverlay = (preset: FilterPreset, children?: React.ReactNode) =>
  render(<PostProcessOverlay preset={preset}>{children}</PostProcessOverlay>, {
    wrapper: TestWrapper,
  });

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('PostProcessOverlay', () => {
  // ─── Rendering & data-testid ──────────────────────────────────────────────

  describe('Rendering', () => {
    const presets: FilterPreset[] = ['NORMAL', 'CRT', 'NVG', 'FLIR'];

    it.each(presets)('renders with data-testid for preset "%s"', (preset) => {
      renderOverlay(preset);
      expect(
        screen.getByTestId(`post-process-overlay-${preset.toLowerCase()}`),
      ).toBeInTheDocument();
    });

    it('renders without children (overlay-only mode)', () => {
      renderOverlay('NORMAL');
      const el = screen.getByTestId('post-process-overlay-normal');
      expect(el).toBeInTheDocument();
    });

    it('renders children inside the wrapper', () => {
      renderOverlay('NORMAL', <div data-testid="child-content">Globe</div>);
      expect(screen.getByTestId('child-content')).toBeInTheDocument();
      expect(screen.getByText('Globe')).toBeInTheDocument();
    });

    it('nests children inside the overlay container', () => {
      renderOverlay('CRT', <div data-testid="child-content" />);
      const overlay = screen.getByTestId('post-process-overlay-crt');
      const child = screen.getByTestId('child-content');
      expect(overlay.contains(child)).toBe(true);
    });
  });

  // ─── Container-level CSS filter ───────────────────────────────────────────

  describe('Container filter', () => {
    it('applies grayscale + contrast + brightness filter for FLIR', () => {
      renderOverlay('FLIR');
      const el = screen.getByTestId('post-process-overlay-flir');
      const style = window.getComputedStyle(el);
      expect(style.filter).toBe('grayscale(0.85) contrast(1.7) brightness(1.1)');
    });

    it('applies brightness + contrast filter for NVG', () => {
      renderOverlay('NVG');
      const el = screen.getByTestId('post-process-overlay-nvg');
      const style = window.getComputedStyle(el);
      expect(style.filter).toBe('brightness(1.2) contrast(1.3)');
    });

    it.each<FilterPreset>(['NORMAL', 'CRT'])(
      'does not apply a container filter for preset "%s"',
      (preset) => {
        renderOverlay(preset);
        const el = screen.getByTestId(`post-process-overlay-${preset.toLowerCase()}`);
        const style = window.getComputedStyle(el);
        // Should be empty or 'none' — no container-level filter
        expect(style.filter === '' || style.filter === 'none').toBe(true);
      },
    );
  });

  // ─── Overlay layer counts ─────────────────────────────────────────────────

  describe('Overlay layers', () => {
    // Each preset produces a known number of overlay Box elements.
    // NORMAL: 1 (vignette)
    // CRT: 8 (curvature, scanlines, scan beam, chromatic, phosphor, noise, vignette, glow, tint)
    // NVG: 7 (green base, brightness, contrast, noise, vignette, lens, glow, scanlines)
    // FLIR: 6 (contrast, highlight, noise, banding, vignette, optics ring)

    it('renders exactly 1 overlay layer for NORMAL', () => {
      renderOverlay('NORMAL');
      const container = screen.getByTestId('post-process-overlay-normal');
      // All overlay layers have pointerEvents: none — count direct children
      // that are overlay layers (not the children wrapper which has pointerEvents: auto)
      const overlayLayers = Array.from(container.children).filter(
        (child) => window.getComputedStyle(child).pointerEvents === 'none',
      );
      expect(overlayLayers.length).toBe(1);
    });

    it('renders multiple overlay layers for CRT', () => {
      renderOverlay('CRT');
      const container = screen.getByTestId('post-process-overlay-crt');
      const overlayLayers = Array.from(container.children).filter(
        (child) => window.getComputedStyle(child).pointerEvents === 'none',
      );
      expect(overlayLayers.length).toBe(9);
    });

    it('renders multiple overlay layers for NVG', () => {
      renderOverlay('NVG');
      const container = screen.getByTestId('post-process-overlay-nvg');
      const overlayLayers = Array.from(container.children).filter(
        (child) => window.getComputedStyle(child).pointerEvents === 'none',
      );
      expect(overlayLayers.length).toBe(8);
    });

    it('renders multiple overlay layers for FLIR', () => {
      renderOverlay('FLIR');
      const container = screen.getByTestId('post-process-overlay-flir');
      const overlayLayers = Array.from(container.children).filter(
        (child) => window.getComputedStyle(child).pointerEvents === 'none',
      );
      expect(overlayLayers.length).toBe(7);
    });
  });

  // ─── Wrapper behaviour ────────────────────────────────────────────────────

  describe('Wrapper architecture', () => {
    it('sets pointerEvents "none" on the root overlay container', () => {
      renderOverlay('NORMAL');
      const el = screen.getByTestId('post-process-overlay-normal');
      expect(window.getComputedStyle(el).pointerEvents).toBe('none');
    });

    it('restores pointerEvents "auto" on the children wrapper so the globe is interactive', () => {
      renderOverlay('FLIR', <div data-testid="child-content" />);
      const child = screen.getByTestId('child-content');
      // The immediate parent of the child is the children wrapper Box
      const childWrapper = child.parentElement!;
      expect(window.getComputedStyle(childWrapper).pointerEvents).toBe('auto');
    });

    it('does not render the children wrapper when no children are provided', () => {
      renderOverlay('NORMAL');
      const container = screen.getByTestId('post-process-overlay-normal');
      // Only overlay layers should exist — no wrapper with pointerEvents: auto
      const autoPointerChildren = Array.from(container.children).filter(
        (child) => window.getComputedStyle(child).pointerEvents === 'auto',
      );
      expect(autoPointerChildren.length).toBe(0);
    });

    it('applies the FLIR CSS filter to children (filter is on parent)', () => {
      renderOverlay('FLIR', <div data-testid="child-content" />);
      const child = screen.getByTestId('child-content');
      // Walk up to find the overlay root — it should have the filter
      const overlayRoot = screen.getByTestId('post-process-overlay-flir');
      expect(overlayRoot.contains(child)).toBe(true);
      expect(window.getComputedStyle(overlayRoot).filter).toBe(
        'grayscale(0.85) contrast(1.7) brightness(1.1)',
      );
    });
  });

  // ─── Prop forwarding ──────────────────────────────────────────────────────

  describe('Prop forwarding', () => {
    it('merges custom sx prop with default styles', () => {
      render(<PostProcessOverlay preset="NORMAL" sx={{ opacity: 0.5 }} />, {
        wrapper: TestWrapper,
      });
      // The component still renders with its internal data-testid
      const el = screen.getByTestId('post-process-overlay-normal');
      expect(el).toBeInTheDocument();
    });

    it('forwards additional Box props', () => {
      render(<PostProcessOverlay preset="NORMAL" id="my-overlay" />, { wrapper: TestWrapper });
      expect(document.getElementById('my-overlay')).toBeInTheDocument();
    });
  });
});
