import { describe, it, expect, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDraggable } from '@respondent/core';

/** Fires a native mouse event on document at (clientX, clientY). */
function fireMouseEvent(type: 'mousemove' | 'mouseup', clientX: number, clientY: number) {
  const event = new MouseEvent(type, { bubbles: true, clientX, clientY });
  document.dispatchEvent(event);
}

describe('useDraggable', () => {
  it('returns null offset initially', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));
    expect(result.current.offset).toBeNull();
    expect(result.current.isDragging).toBe(false);
  });

  it('tracks drag offset from mousedown through mousemove', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    expect(result.current.isDragging).toBe(true);

    act(() => {
      fireMouseEvent('mousemove', 130, 70);
    });

    expect(result.current.offset).toEqual({ x: 30, y: 20 });
  });

  it('stops dragging on mouseup', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    act(() => {
      fireMouseEvent('mousemove', 130, 70);
    });

    act(() => {
      fireMouseEvent('mouseup', 130, 70);
    });

    expect(result.current.isDragging).toBe(false);
    expect(result.current.offset).toEqual({ x: 30, y: 20 });
  });

  it('does not drag when enabled is false', () => {
    const { result } = renderHook(() => useDraggable({ enabled: false }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    expect(result.current.isDragging).toBe(false);

    act(() => {
      fireMouseEvent('mousemove', 200, 200);
    });

    expect(result.current.offset).toBeNull();
  });

  it('resetPosition clears offset', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    act(() => {
      fireMouseEvent('mousemove', 200, 150);
      fireMouseEvent('mouseup', 200, 150);
    });

    expect(result.current.offset).toEqual({ x: 100, y: 100 });

    act(() => {
      result.current.resetPosition();
    });

    expect(result.current.offset).toBeNull();
  });

  it('accumulates offset across multiple drags', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));

    // First drag: move (30, 20)
    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });
    act(() => {
      fireMouseEvent('mousemove', 130, 70);
      fireMouseEvent('mouseup', 130, 70);
    });

    expect(result.current.offset).toEqual({ x: 30, y: 20 });

    // Second drag: move another (10, 5)
    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 200,
        clientY: 100,
        preventDefault: () => {},
      } as React.MouseEvent);
    });
    act(() => {
      fireMouseEvent('mousemove', 210, 105);
      fireMouseEvent('mouseup', 210, 105);
    });

    expect(result.current.offset).toEqual({ x: 40, y: 25 });
  });

  it('sets cursor grab when enabled, default when disabled', () => {
    const { result, rerender } = renderHook(({ enabled }) => useDraggable({ enabled }), {
      initialProps: { enabled: true },
    });

    expect(result.current.handleProps.style.cursor).toBe('grab');

    rerender({ enabled: false });
    expect(result.current.handleProps.style.cursor).toBe('default');
  });

  it('sets cursor to grabbing while dragging', () => {
    const { result } = renderHook(() => useDraggable({ enabled: true }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    expect(result.current.handleProps.style.cursor).toBe('grabbing');
  });

  it('cleans up document listeners on unmount', () => {
    const addSpy = vi.spyOn(document, 'addEventListener');
    const removeSpy = vi.spyOn(document, 'removeEventListener');

    const { result, unmount } = renderHook(() => useDraggable({ enabled: true }));

    act(() => {
      result.current.handleProps.onMouseDown({
        clientX: 100,
        clientY: 50,
        preventDefault: () => {},
      } as React.MouseEvent);
    });

    expect(addSpy).toHaveBeenCalledWith('mousemove', expect.any(Function));
    expect(addSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));

    unmount();

    expect(removeSpy).toHaveBeenCalledWith('mousemove', expect.any(Function));
    expect(removeSpy).toHaveBeenCalledWith('mouseup', expect.any(Function));

    addSpy.mockRestore();
    removeSpy.mockRestore();
  });
});
