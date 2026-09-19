/**
 * Tests for new icon draw functions: warning, radiation, fire
 *
 * Each icon draws on a 32x32 canvas. Tests verify:
 * - The draw function executes without error
 * - Canvas context receives expected drawing calls
 * - Icons are visually distinct (different path operations)
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { drawWarningIcon } from './warningIcon';
import { drawRadiationIcon } from './radiationIcon';
import { drawFireIcon } from './fireIcon';

function createMockContext(): CanvasRenderingContext2D {
  return {
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 0,
    lineJoin: 'miter',
    lineCap: 'butt',
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    bezierCurveTo: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(),
    save: vi.fn(),
    restore: vi.fn(),
    translate: vi.fn(),
    rotate: vi.fn(),
    fillRect: vi.fn(),
  } as unknown as CanvasRenderingContext2D;
}

describe('drawWarningIcon', () => {
  let ctx: CanvasRenderingContext2D;

  beforeEach(() => {
    ctx = createMockContext();
  });

  it('draws without error', () => {
    expect(() => drawWarningIcon(ctx, '#ff0000')).not.toThrow();
  });

  it('draws filled outer and inner triangles plus exclamation', () => {
    drawWarningIcon(ctx, '#ff0000');
    // Outer triangle fill + inner cutout fill + stem fill + dot fill = 4
    expect(ctx.fill).toHaveBeenCalledTimes(4);
    expect(ctx.moveTo).toHaveBeenCalled();
    expect(ctx.lineTo).toHaveBeenCalled();
    expect(ctx.closePath).toHaveBeenCalled();
  });

  it('draws the exclamation dot via arc', () => {
    drawWarningIcon(ctx, '#ff0000');
    expect(ctx.arc).toHaveBeenCalledTimes(1);
  });

  it('uses provided color for fills', () => {
    drawWarningIcon(ctx, '#ff0000');
    // Last fillStyle set is the provided color (exclamation dot)
    expect(ctx.fillStyle).toBe('#ff0000');
  });
});

describe('drawRadiationIcon', () => {
  let ctx: CanvasRenderingContext2D;

  beforeEach(() => {
    ctx = createMockContext();
  });

  it('draws without error', () => {
    expect(() => drawRadiationIcon(ctx, '#00ff00')).not.toThrow();
  });

  it('draws three blades via arc calls', () => {
    drawRadiationIcon(ctx, '#00ff00');
    // 1 outer filled circle + 1 black cutout circle + 3 blades (each with 2 arcs: outer + inner) + 1 center dot = 9 arc calls
    expect(ctx.arc).toHaveBeenCalledTimes(9);
  });

  it('fills six times (outer circle + black cutout + 3 blades + 1 center dot)', () => {
    drawRadiationIcon(ctx, '#00ff00');
    expect(ctx.fill).toHaveBeenCalledTimes(6);
  });

  it('uses provided color for fill', () => {
    drawRadiationIcon(ctx, '#00ff00');
    expect(ctx.fillStyle).toBe('#00ff00');
  });
});

describe('drawFireIcon', () => {
  let ctx: CanvasRenderingContext2D;

  beforeEach(() => {
    ctx = createMockContext();
  });

  it('draws without error', () => {
    expect(() => drawFireIcon(ctx, '#ff4500')).not.toThrow();
  });

  it('uses bezier curves for the flame shape', () => {
    drawFireIcon(ctx, '#ff4500');
    expect(ctx.bezierCurveTo).toHaveBeenCalled();
  });

  it('draws two shapes (outer flame + inner flame)', () => {
    drawFireIcon(ctx, '#ff4500');
    expect(ctx.beginPath).toHaveBeenCalledTimes(2);
    expect(ctx.fill).toHaveBeenCalledTimes(2);
  });

  it('uses black for the inner flame cutout', () => {
    drawFireIcon(ctx, '#ff4500');
    // Inner flame is a black cutout — the last fillStyle set by the draw function
    expect(ctx.fillStyle).toBe('#000000');
  });
});
