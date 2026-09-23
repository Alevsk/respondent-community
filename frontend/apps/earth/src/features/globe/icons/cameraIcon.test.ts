/**
 * The camera shape backs declarative CCTV layers, which select it by name from
 * YAML. These tests hold both the draw function and its registry entry in place.
 */

import { describe, it, expect, afterAll, vi } from 'vitest';
import { drawCameraIcon } from './cameraIcon';

function createMockContext(): CanvasRenderingContext2D {
  return {
    fillStyle: '',
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
  } as unknown as CanvasRenderingContext2D;
}

describe('drawCameraIcon', () => {
  it('draws the housing, lens, mount, plate and sight in the requested color', () => {
    const ctx = createMockContext();
    drawCameraIcon(ctx, '#00ff9d');
    expect(ctx.fillStyle).toBe('#00ff9d');
    expect(ctx.fill).toHaveBeenCalledTimes(5);
    expect(ctx.arc).toHaveBeenCalledTimes(1);
  });
});

describe('camera shape registration', () => {
  // jsdom returns null from getContext, so the registry needs a stand-in for
  // the draw function to be reached at all.
  const mockCtx = {
    scale: vi.fn(),
    fillStyle: '',
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
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
    registerDynamicIcon('cctv_austin', { shape: 'camera', rotatable: false, scale: 1.0 });
    // A shape the registry does not know falls back to the default icon, so a
    // canvas with fills proves 'camera' resolved to a real draw function.
    expect(getIconCanvas('cctv_austin')).toBeInstanceOf(HTMLCanvasElement);
    expect(mockCtx.fill).toHaveBeenCalled();
  });
});
