/**
 * iconRegistry Unit Tests
 *
 * Tests for icon canvas creation/caching, rotation support, and scale calculation.
 * All per-layer rendering config must come from registerDynamicIcon (declarative
 * path); unregistered layers fall back to generic defaults only (no layer-name maps).
 */

import { describe, it, expect, vi, beforeEach, afterAll } from 'vitest';

// Mock icon draw functions before importing the registry
vi.mock('./flightIcon', () => ({ drawFlightIcon: vi.fn() }));
vi.mock('./flightAltBIcon', () => ({ drawFlightAltBIcon: vi.fn() }));
vi.mock('./diamondIcon', () => ({ drawDiamondIcon: vi.fn() }));
vi.mock('./earthquakeIcon', () => ({ drawEarthquakeIcon: vi.fn() }));
vi.mock('./defaultIcon', () => ({ drawDefaultIcon: vi.fn() }));

// Provide a mock canvas context so drawFn actually gets called (jsdom returns null for getContext)
const mockCtx = { scale: vi.fn() } as unknown as CanvasRenderingContext2D;
const originalGetContext = HTMLCanvasElement.prototype.getContext;
// @ts-expect-error -- overriding for test
HTMLCanvasElement.prototype.getContext = function (id: string) {
  if (id === '2d') return mockCtx;
  return originalGetContext.call(this, id);
};

import { supportsRotation, getIconScale } from './iconRegistry';

afterAll(() => {
  HTMLCanvasElement.prototype.getContext = originalGetContext;
});

describe('iconRegistry', () => {
  describe('supportsRotation', () => {
    it('returns false for unregistered layer types (generic default)', () => {
      expect(supportsRotation('satellites')).toBe(false);
      expect(supportsRotation('earthquakes')).toBe(false);
      expect(supportsRotation('ships')).toBe(false);
      expect(supportsRotation('')).toBe(false);
      expect(supportsRotation('FLIGHTS_COMMERCIAL')).toBe(false);
    });
  });

  describe('getIconScale', () => {
    // All expected values are halved because canvas renders at 2x resolution
    // (CANVAS_SCALE = 2) so the billboard scale compensates.
    it('returns base scale of 0.5 when pointSize equals default (8) for unregistered layer', () => {
      expect(getIconScale('satellites', 8)).toBe(0.5);
    });

    it('returns base scale of 1.0 when pointSize is 16', () => {
      expect(getIconScale('satellites', 16)).toBe(1.0);
    });

    it('returns base scale of 0.25 when pointSize is 4', () => {
      expect(getIconScale('satellites', 4)).toBe(0.25);
    });

    it('uses default pointSize of 8 when pointSize is undefined', () => {
      expect(getIconScale('satellites', undefined)).toBe(0.5);
    });

    it('applies multiplier of 1.0 for unregistered layer types (generic default)', () => {
      expect(getIconScale('earthquakes', 8)).toBe(0.5);
      expect(getIconScale('flights_commercial', 8)).toBe(0.5);
      expect(getIconScale('unknown_type', 8)).toBe(0.5);
    });
  });

  describe('getIconCanvas', () => {
    // Reset module registry before each test so the module-level canvasCache is cleared.
    // vi.clearAllMocks() resets call counts on all spies so assertions stay isolated.
    beforeEach(() => {
      vi.resetModules();
      vi.clearAllMocks();
    });

    it('returns an HTMLCanvasElement', async () => {
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const canvas = freshGetIconCanvas('some_layer');
      expect(canvas).toBeInstanceOf(HTMLCanvasElement);
    });

    it('canvas has correct icon dimensions (64x64 for 2x crisp rendering)', async () => {
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const canvas = freshGetIconCanvas('some_layer');
      expect(canvas.width).toBe(64);
      expect(canvas.height).toBe(64);
    });

    it('calls drawDefaultIcon for unregistered layer types', async () => {
      const { drawDefaultIcon: mockDrawDefault } = await import('./defaultIcon');
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      freshGetIconCanvas('flights_commercial');
      freshGetIconCanvas('satellites');
      freshGetIconCanvas('earthquakes');
      freshGetIconCanvas('unknown_ships');
      expect(mockDrawDefault).toHaveBeenCalledTimes(4);
    });

    it('caches the canvas and does not recreate it on second call', async () => {
      const { drawDefaultIcon: mockDrawDefault } = await import('./defaultIcon');
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const canvas1 = freshGetIconCanvas('some_layer');
      const canvas2 = freshGetIconCanvas('some_layer');
      expect(canvas1).toBe(canvas2);
      expect(mockDrawDefault).toHaveBeenCalledTimes(1);
    });

    it('caches default icon per layer type and color', async () => {
      const { drawDefaultIcon: mockDrawDefault } = await import('./defaultIcon');
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const canvas1 = freshGetIconCanvas('ships');
      const canvas1b = freshGetIconCanvas('ships');
      // Same layer type + same default color = same canvas instance
      expect(canvas1).toBe(canvas1b);
      // Different layer type = different cache key, separate canvas
      const canvas2 = freshGetIconCanvas('buoys');
      expect(canvas1).not.toBe(canvas2);
      expect(mockDrawDefault).toHaveBeenCalledTimes(2);
    });

    it('returns different canvases for same type with different colors', async () => {
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const redCanvas = freshGetIconCanvas('some_layer', '#ff0000');
      const blueCanvas = freshGetIconCanvas('some_layer', '#00bfff');
      // Same layer type but different colors must produce separate canvases
      expect(redCanvas).not.toBe(blueCanvas);
      // Re-requesting the same type+color returns the cached instance
      const redAgain = freshGetIconCanvas('some_layer', '#ff0000');
      expect(redAgain).toBe(redCanvas);
    });

    it('treats undefined color as white and caches separately from explicit colors', async () => {
      const { getIconCanvas: freshGetIconCanvas } = await import('./iconRegistry');
      const noColor = freshGetIconCanvas('some_layer');
      const whiteExplicit = freshGetIconCanvas('some_layer', '#ffffff');
      const pinkExplicit = freshGetIconCanvas('some_layer', '#ff006e');
      // undefined color resolves to #ffffff, so same cache key
      expect(noColor).toBe(whiteExplicit);
      // Explicit different color is a separate canvas
      expect(noColor).not.toBe(pinkExplicit);
    });

    it('registered layer uses its draw function while unregistered uses default', async () => {
      const { drawFlightIcon: mockDrawFlight } = await import('./flightIcon');
      const { drawDefaultIcon: mockDrawDefault } = await import('./defaultIcon');
      const { getIconCanvas: freshGetIconCanvas, registerDynamicIcon } =
        await import('./iconRegistry');
      registerDynamicIcon('my_flights', { shape: 'flight', rotatable: true, scale: 1.0 });
      freshGetIconCanvas('my_flights');
      freshGetIconCanvas('unregistered_layer');
      expect(mockDrawFlight).toHaveBeenCalledTimes(1);
      expect(mockDrawDefault).toHaveBeenCalledTimes(1);
    });
  });
});
