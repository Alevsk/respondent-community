/**
 * The camera shape backs declarative CCTV layers, which select it by name from
 * YAML. These tests hold both the draw function and its registry entry in place.
 */

import { describe, it, expect, afterAll, vi } from 'vitest';
import { drawCameraIcon } from './cameraIcon';
import { drawBroadcastIcon } from './broadcastIcon';

function createMockContext(): CanvasRenderingContext2D {
  return {
    fillStyle: '',
    strokeStyle: '',
    lineWidth: 0,
    globalAlpha: 1,
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    rect: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(),
  } as unknown as CanvasRenderingContext2D;
}

describe('drawCameraIcon', () => {
  it('draws the view cone, mount, body and lens in the requested color', () => {
    const ctx = createMockContext();
    drawCameraIcon(ctx, '#00ff9d');
    expect(ctx.fillStyle).toBe('#00ff9d');
    expect(ctx.fill).toHaveBeenCalledTimes(5);
  });

  it('restores full opacity after the translucent view cone', () => {
    // The cone is drawn at reduced alpha. Leaving it set would wash out every
    // icon rendered after this one onto the same context.
    const ctx = createMockContext();
    drawCameraIcon(ctx, '#00ff9d');
    expect(ctx.globalAlpha).toBe(1);
  });
});

describe('drawBroadcastIcon', () => {
  it('draws a mast and radiating waves in the requested color', () => {
    const ctx = createMockContext();
    drawBroadcastIcon(ctx, '#e040fb');
    expect(ctx.fillStyle).toBe('#e040fb');
    expect(ctx.strokeStyle).toBe('#e040fb');
    // Mast, base, brace and emitter are filled; the four wave arcs are stroked.
    expect(ctx.fill).toHaveBeenCalledTimes(4);
    expect(ctx.stroke).toHaveBeenCalledTimes(4);
  });
});

describe('camera shape registration', () => {
  // jsdom returns null from getContext, so the registry needs a stand-in for
  // the draw function to be reached at all.
  // Same stand-in the draw tests use, plus the scale() the registry calls, so
  // there is one definition of what a canvas context looks like here.
  const mockCtx = {
    ...createMockContext(),
    scale: vi.fn(),
  } as unknown as CanvasRenderingContext2D;
  const originalGetContext = HTMLCanvasElement.prototype.getContext;
  // @ts-expect-error -- overriding for test
  HTMLCanvasElement.prototype.getContext = function (id: string) {
    if (id === '2d') return mockCtx;
    return originalGetContext.call(this, id);
  };
  afterAll(() => {
    HTMLCanvasElement.prototype.getContext = originalGetContext;
  });

  it('is reachable from YAML through the "camera" shape name', async () => {
    const { registerDynamicIcon, getIconCanvas, _resetDynamicRegistry } =
      await import('./iconRegistry');
    _resetDynamicRegistry();
    registerDynamicIcon('cctv', { shape: 'camera', rotatable: false, scale: 1.0 });
    // A shape the registry does not know falls back to the default icon, so a
    // canvas with fills proves 'camera' resolved to a real draw function.
    expect(getIconCanvas('cctv')).toBeInstanceOf(HTMLCanvasElement);
    expect(mockCtx.fill).toHaveBeenCalled();
  });
});
