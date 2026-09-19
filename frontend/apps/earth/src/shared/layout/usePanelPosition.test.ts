import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { usePanelPosition } from './usePanelPosition';
import { usePanelLayoutStore } from './panelLayoutStore';

describe('usePanelPosition', () => {
  beforeEach(() => {
    usePanelLayoutStore.setState({ panels: {}, _nextOrder: 0 });
  });

  it('registers panel on mount and unregisters on unmount', () => {
    const { unmount } = renderHook(() =>
      usePanelPosition({ id: 'test', zone: 'top-right', width: 280, visible: true }),
    );

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.test).toBeDefined();
    expect(panels.test).toMatchObject({ id: 'test', zone: 'top-right', width: 280 });

    unmount();
    expect(usePanelLayoutStore.getState().panels.test).toBeUndefined();
  });

  it('returns { right } for top-right zone', () => {
    const { result } = renderHook(() =>
      usePanelPosition({ id: 'test', zone: 'top-right', width: 280, visible: true }),
    );

    expect(result.current).toEqual({ right: 16 });
    expect(result.current.left).toBeUndefined();
  });

  it('returns { right } for bottom-right zone', () => {
    const { result } = renderHook(() =>
      usePanelPosition({ id: 'test', zone: 'bottom-right', width: 300, visible: true }),
    );

    expect(result.current).toEqual({ right: 16 });
  });

  it('returns { left } for top-left zone', () => {
    const { result } = renderHook(() =>
      usePanelPosition({ id: 'test', zone: 'top-left', width: 200, visible: true }),
    );

    expect(result.current).toEqual({ left: 16 });
    expect(result.current.right).toBeUndefined();
  });

  it('returns { left } for bottom-left zone', () => {
    const { result } = renderHook(() =>
      usePanelPosition({
        id: 'test',
        zone: 'bottom-left',
        width: 300,
        visible: true,
        baseMargin: 80,
      }),
    );

    expect(result.current).toEqual({ left: 80 });
  });

  it('syncs visibility on prop change', () => {
    const { rerender } = renderHook(
      ({ visible }) => usePanelPosition({ id: 'test', zone: 'top-right', width: 280, visible }),
      { initialProps: { visible: false } },
    );

    expect(usePanelLayoutStore.getState().panels.test.visible).toBe(false);

    rerender({ visible: true });
    expect(usePanelLayoutStore.getState().panels.test.visible).toBe(true);

    rerender({ visible: false });
    expect(usePanelLayoutStore.getState().panels.test.visible).toBe(false);
  });

  it('computes offset with one preceding visible panel', () => {
    // Register first panel (effects) before rendering the hook for the second
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('effects', 'top-right', 280);
    setVisible('effects', true);

    const { result } = renderHook(() =>
      usePanelPosition({ id: 'entity', zone: 'top-right', width: 320, visible: true }),
    );

    // entity offset = 16 + (280 + 16) = 312
    expect(result.current).toEqual({ right: 312 });
  });

  it('recalculates offset when preceding panel hides', () => {
    // Set up two panels manually, then hook into the second
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('effects', 'top-right', 280);
    setVisible('effects', true);

    const { result } = renderHook(() =>
      usePanelPosition({ id: 'entity', zone: 'top-right', width: 320, visible: true }),
    );

    expect(result.current).toEqual({ right: 312 });

    // Hide the preceding panel
    act(() => {
      usePanelLayoutStore.getState().setVisible('effects', false);
    });

    expect(result.current).toEqual({ right: 16 });
  });

  it('uses custom gap parameter', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100);
    setVisible('a', true);

    const { result } = renderHook(() =>
      usePanelPosition({ id: 'b', zone: 'top-right', width: 200, visible: true, gap: 8 }),
    );

    // offset = 16 + (100 + 8) = 124
    expect(result.current).toEqual({ right: 124 });
  });

  it('uses custom baseMargin parameter', () => {
    const { result } = renderHook(() =>
      usePanelPosition({
        id: 'test',
        zone: 'bottom-left',
        width: 300,
        visible: true,
        baseMargin: 80,
      }),
    );

    expect(result.current).toEqual({ left: 80 });
  });

  it('multiple hooks in same zone compute correct offsets', () => {
    const hookA = renderHook(() =>
      usePanelPosition({ id: 'panel-a', zone: 'top-right', width: 100, visible: true }),
    );
    const hookB = renderHook(() =>
      usePanelPosition({ id: 'panel-b', zone: 'top-right', width: 200, visible: true }),
    );

    expect(hookA.result.current).toEqual({ right: 16 });
    // panel-b: 16 + (100 + 16) = 132
    expect(hookB.result.current).toEqual({ right: 132 });
  });

  it('panel in different zone is unaffected by other zones', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('top-panel', 'top-right', 280);
    setVisible('top-panel', true);

    const { result } = renderHook(() =>
      usePanelPosition({ id: 'bottom-panel', zone: 'bottom-right', width: 300, visible: true }),
    );

    // bottom-panel is alone in bottom-right, offset = 16
    expect(result.current).toEqual({ right: 16 });
  });

  it('hidden panel does not claim space', () => {
    // Register panel-a as hidden so it doesn't claim space
    renderHook(() =>
      usePanelPosition({ id: 'panel-a', zone: 'top-right', width: 100, visible: false }),
    );
    const hookB = renderHook(() =>
      usePanelPosition({ id: 'panel-b', zone: 'top-right', width: 200, visible: true }),
    );

    // panel-a is hidden, so panel-b gets baseMargin
    expect(hookB.result.current).toEqual({ right: 16 });
  });

  it('re-registers with updated width on prop change', () => {
    const { rerender } = renderHook(
      ({ width }) => usePanelPosition({ id: 'test', zone: 'top-right', width, visible: true }),
      { initialProps: { width: 280 } },
    );

    expect(usePanelLayoutStore.getState().panels.test.width).toBe(280);

    rerender({ width: 350 });
    expect(usePanelLayoutStore.getState().panels.test.width).toBe(350);
    // Panel remains registered with the same id
    expect(usePanelLayoutStore.getState().panels.test.id).toBe('test');
    expect(usePanelLayoutStore.getState().panels.test.zone).toBe('top-right');
  });
});
