/**
 * iconRegistry dynamic registration unit tests
 *
 * Covers:
 * - registerDynamicIcon() registration and retrieval
 * - Dynamic registrations drive icon draw fn, rotation, and scale
 * - supportsRotation() and getIconScale() reflect dynamic config
 * - Canvas cache invalidation on re-registration
 * - Generic defaults for unregistered layer types (no layer-name fallback maps)
 */

import { describe, it, expect, beforeEach, afterAll, vi } from 'vitest';

// Mock all icon draw functions so we can track calls without DOM canvas rendering
vi.mock('./flightIcon', () => ({ drawFlightIcon: vi.fn() }));
vi.mock('./earthquakeIcon', () => ({ drawEarthquakeIcon: vi.fn() }));
vi.mock('./defaultIcon', () => ({ drawDefaultIcon: vi.fn() }));
vi.mock('./diamondIcon', () => ({ drawDiamondIcon: vi.fn() }));
vi.mock('./radioIcon', () => ({ drawRadioIcon: vi.fn() }));
vi.mock('./warningIcon', () => ({ drawWarningIcon: vi.fn() }));
vi.mock('./radiationIcon', () => ({ drawRadiationIcon: vi.fn() }));
vi.mock('./fireIcon', () => ({ drawFireIcon: vi.fn() }));

// Provide a mock canvas context so drawFn actually gets called (jsdom returns null for getContext)
const mockCtx = { scale: vi.fn() } as unknown as CanvasRenderingContext2D;
const originalGetContext = HTMLCanvasElement.prototype.getContext;
// @ts-expect-error -- overriding for test
HTMLCanvasElement.prototype.getContext = function (id: string) {
  if (id === '2d') return mockCtx;
  return originalGetContext.call(this, id);
};

afterAll(() => {
  HTMLCanvasElement.prototype.getContext = originalGetContext;
});

import {
  registerDynamicIcon,
  getIconCanvas,
  supportsRotation,
  getIconScale,
  _resetDynamicRegistry,
} from './iconRegistry';

import { drawFlightIcon } from './flightIcon';
import { drawDiamondIcon } from './diamondIcon';
import { drawRadioIcon } from './radioIcon';
import { drawDefaultIcon } from './defaultIcon';
import { drawWarningIcon } from './warningIcon';
import { drawRadiationIcon } from './radiationIcon';
import { drawFireIcon } from './fireIcon';

describe('registerDynamicIcon', () => {
  beforeEach(() => {
    _resetDynamicRegistry();
    vi.clearAllMocks();
  });

  it('registers a new layer type and returns an HTMLCanvasElement', () => {
    registerDynamicIcon('custom_ships', { shape: 'diamond', rotatable: false, scale: 1.0 });
    const canvas = getIconCanvas('custom_ships');
    expect(canvas).toBeInstanceOf(HTMLCanvasElement);
  });

  it('uses the draw function mapped from the shape name', () => {
    registerDynamicIcon('my_radio', { shape: 'radio', rotatable: false, scale: 1.0 });
    getIconCanvas('my_radio');
    expect(drawRadioIcon).toHaveBeenCalledTimes(1);
  });

  it('maps "flight" shape to drawFlightIcon', () => {
    registerDynamicIcon('military_drones', { shape: 'flight', rotatable: true, scale: 1.0 });
    getIconCanvas('military_drones');
    expect(drawFlightIcon).toHaveBeenCalledTimes(1);
  });

  it('maps "diamond" shape to drawDiamondIcon', () => {
    registerDynamicIcon('buoys', { shape: 'diamond', rotatable: false, scale: 1.2 });
    getIconCanvas('buoys');
    expect(drawDiamondIcon).toHaveBeenCalledTimes(1);
  });

  it('maps "warning" shape to drawWarningIcon', () => {
    registerDynamicIcon('alerts', { shape: 'warning', rotatable: false, scale: 1.4 });
    getIconCanvas('alerts');
    expect(drawWarningIcon).toHaveBeenCalledTimes(1);
  });

  it('maps "radiation" shape to drawRadiationIcon', () => {
    registerDynamicIcon('nuke_sensors', { shape: 'radiation', rotatable: false, scale: 1.0 });
    getIconCanvas('nuke_sensors');
    expect(drawRadiationIcon).toHaveBeenCalledTimes(1);
  });

  it('maps "fire" shape to drawFireIcon', () => {
    registerDynamicIcon('wildfires', { shape: 'fire', rotatable: false, scale: 1.0 });
    getIconCanvas('wildfires');
    expect(drawFireIcon).toHaveBeenCalledTimes(1);
  });

  it('falls back to drawDefaultIcon for unknown shape name', () => {
    registerDynamicIcon('unknown_layer', {
      shape: 'nonexistent_shape',
      rotatable: false,
      scale: 1.0,
    });
    getIconCanvas('unknown_layer');
    expect(drawDefaultIcon).toHaveBeenCalledTimes(1);
  });

  it('unregistered layer type uses drawDefaultIcon (no hardcoded layer-name maps)', () => {
    // flights_commercial, satellites, earthquakes — no prior registerDynamicIcon call
    getIconCanvas('flights_commercial');
    getIconCanvas('satellites');
    getIconCanvas('earthquakes');
    expect(drawDefaultIcon).toHaveBeenCalledTimes(3);
    expect(drawFlightIcon).not.toHaveBeenCalled();
    expect(drawDiamondIcon).not.toHaveBeenCalled();
  });

  it('dynamic registration overrides default for a layer type that was previously unregistered', () => {
    // Without registration: uses drawDefaultIcon
    getIconCanvas('flights_commercial');
    expect(drawDefaultIcon).toHaveBeenCalledTimes(1);
    expect(drawFlightIcon).not.toHaveBeenCalled();

    // After registration: uses the registered draw function
    _resetDynamicRegistry();
    vi.clearAllMocks();
    registerDynamicIcon('flights_commercial', { shape: 'flight', rotatable: true, scale: 1.0 });
    getIconCanvas('flights_commercial');
    expect(drawFlightIcon).toHaveBeenCalledTimes(1);
    expect(drawDefaultIcon).not.toHaveBeenCalled();
  });

  it('invalidates canvas cache so re-registration takes effect', () => {
    // First registration: radio shape
    registerDynamicIcon('my_layer', { shape: 'radio', rotatable: false, scale: 1.0 });
    getIconCanvas('my_layer'); // prime cache
    expect(drawRadioIcon).toHaveBeenCalledTimes(1);

    // Re-register with diamond shape — cache should be cleared
    registerDynamicIcon('my_layer', { shape: 'diamond', rotatable: false, scale: 1.0 });
    getIconCanvas('my_layer');
    expect(drawDiamondIcon).toHaveBeenCalledTimes(1);
    // Radio should not have been called a second time
    expect(drawRadioIcon).toHaveBeenCalledTimes(1);
  });

  it('caches the canvas after first call for dynamic type', () => {
    registerDynamicIcon('cached_layer', { shape: 'diamond', rotatable: false, scale: 1.0 });
    const canvas1 = getIconCanvas('cached_layer');
    const canvas2 = getIconCanvas('cached_layer');
    expect(canvas1).toBe(canvas2);
    expect(drawDiamondIcon).toHaveBeenCalledTimes(1);
  });
});

describe('supportsRotation (dynamic)', () => {
  beforeEach(() => {
    _resetDynamicRegistry();
  });

  it('returns true when dynamic registration has rotatable: true', () => {
    registerDynamicIcon('rotating_drones', { shape: 'flight', rotatable: true, scale: 1.0 });
    expect(supportsRotation('rotating_drones')).toBe(true);
  });

  it('returns false when dynamic registration has rotatable: false', () => {
    registerDynamicIcon('static_ships', { shape: 'diamond', rotatable: false, scale: 1.0 });
    expect(supportsRotation('static_ships')).toBe(false);
  });

  it('returns false for any unregistered layer type (no hardcoded fallback)', () => {
    // Previously ROTATABLE_TYPES contained these; now unregistered = false
    expect(supportsRotation('flights_commercial')).toBe(false);
    expect(supportsRotation('flights_military')).toBe(false);
    expect(supportsRotation('satellites')).toBe(false);
    expect(supportsRotation('earthquakes')).toBe(false);
    expect(supportsRotation('unknown_xyz')).toBe(false);
  });

  it('dynamic registration overrides the generic default for a given layer type', () => {
    // Register flights_commercial as rotatable
    registerDynamicIcon('flights_commercial', { shape: 'flight', rotatable: true, scale: 1.0 });
    expect(supportsRotation('flights_commercial')).toBe(true);

    // Register satellites as non-rotatable (explicit)
    registerDynamicIcon('satellites', { shape: 'diamond', rotatable: false, scale: 1.0 });
    expect(supportsRotation('satellites')).toBe(false);
  });
});

describe('getIconScale (dynamic)', () => {
  beforeEach(() => {
    _resetDynamicRegistry();
  });

  // All expected values are halved because canvas renders at 2x resolution
  // (CANVAS_SCALE = 2) so the billboard scale compensates.
  it('applies dynamic scale multiplier combined with pointSize ratio', () => {
    registerDynamicIcon('big_ships', { shape: 'diamond', rotatable: false, scale: 2.0 });
    // pointSize 8 → (1.0 × 2.0) / 2 = 1.0
    expect(getIconScale('big_ships', 8)).toBeCloseTo(1.0);
  });

  it('combines dynamic scale with non-default pointSize', () => {
    registerDynamicIcon('small_buoys', { shape: 'dot', rotatable: false, scale: 0.5 });
    // pointSize 16 → (2.0 × 0.5) / 2 = 0.5
    expect(getIconScale('small_buoys', 16)).toBeCloseTo(0.5);
  });

  it('unregistered layer uses scale multiplier of 1.0 (generic default)', () => {
    // Previously SCALE_MAP had earthquakes: 1.2 — now unregistered = 1.0
    // pointSize 8 → (1.0 × 1.0) / 2 = 0.5
    expect(getIconScale('earthquakes', 8)).toBeCloseTo(0.5);
    expect(getIconScale('satellites', 8)).toBeCloseTo(0.5);
    expect(getIconScale('flights_commercial', 8)).toBeCloseTo(0.5);
    expect(getIconScale('unknown_xyz', 8)).toBeCloseTo(0.5);
  });

  it('dynamic registration sets scale for a previously unregistered type', () => {
    // Register earthquakes with 1.2 scale (same as the old hardcoded value)
    registerDynamicIcon('earthquakes', { shape: 'ripple', rotatable: false, scale: 1.2 });
    // (1.0 × 1.2) / 2 = 0.6
    expect(getIconScale('earthquakes', 8)).toBeCloseTo(0.6);
  });

  it('dynamic registration overrides scale for a registered type', () => {
    registerDynamicIcon('earthquakes', { shape: 'ripple', rotatable: false, scale: 0.8 });
    // (1.0 × 0.8) / 2 = 0.4
    expect(getIconScale('earthquakes', 8)).toBeCloseTo(0.4);
  });

  it('uses default pointSize of 8 when undefined', () => {
    registerDynamicIcon('default_test', { shape: 'dot', rotatable: false, scale: 1.5 });
    // (1.0 × 1.5) / 2 = 0.75
    expect(getIconScale('default_test', undefined)).toBeCloseTo(0.75);
  });
});
