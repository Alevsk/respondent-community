import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';

describe('core hooks', () => {
  describe('useResponsive', () => {
    it('exports useResponsive function', async () => {
      const mod = await import('./useResponsive');
      expect(typeof mod.useResponsive).toBe('function');
    });

    it('returns isMobile, isTablet, isDesktop booleans', async () => {
      const { useResponsive } = await import('./useResponsive');
      const { result } = renderHook(() => useResponsive());
      expect(typeof result.current.isMobile).toBe('boolean');
      expect(typeof result.current.isTablet).toBe('boolean');
      expect(typeof result.current.isDesktop).toBe('boolean');
    });

    it('isMobile is false when matchMedia returns no match (default mock)', async () => {
      const { useResponsive } = await import('./useResponsive');
      const { result } = renderHook(() => useResponsive());
      // Default mock returns matches: false for all queries
      expect(result.current.isMobile).toBe(false);
    });
  });

  describe('useDraggable', () => {
    it('exports useDraggable function', async () => {
      const mod = await import('./useDraggable');
      expect(typeof mod.useDraggable).toBe('function');
    });

    it('returns offset null, isDragging false, handleProps, resetPosition initially', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      expect(result.current.offset).toBeNull();
      expect(result.current.isDragging).toBe(false);
      expect(typeof result.current.handleProps.onMouseDown).toBe('function');
      expect(typeof result.current.resetPosition).toBe('function');
    });

    it('handleProps.style has cursor grab when enabled and not dragging', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      expect(result.current.handleProps.style.cursor).toBe('grab');
    });

    it('handleProps.style has cursor default when disabled', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: false }));
      expect(result.current.handleProps.style.cursor).toBe('default');
    });

    it('sets isDragging true on mousedown when enabled', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      act(() => {
        result.current.handleProps.onMouseDown({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
        } as unknown as React.MouseEvent);
      });
      expect(result.current.isDragging).toBe(true);
    });

    it('does not set isDragging when disabled', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: false }));
      act(() => {
        result.current.handleProps.onMouseDown({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
        } as unknown as React.MouseEvent);
      });
      expect(result.current.isDragging).toBe(false);
    });

    it('resetPosition clears offset and isDragging', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      act(() => {
        result.current.handleProps.onMouseDown({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
        } as unknown as React.MouseEvent);
      });
      expect(result.current.isDragging).toBe(true);
      act(() => {
        result.current.resetPosition();
      });
      expect(result.current.offset).toBeNull();
      expect(result.current.isDragging).toBe(false);
    });

    it('updates offset on mousemove while dragging', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      act(() => {
        result.current.handleProps.onMouseDown({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
        } as unknown as React.MouseEvent);
      });
      act(() => {
        document.dispatchEvent(new MouseEvent('mousemove', { clientX: 150, clientY: 250 }));
      });
      expect(result.current.offset).toEqual({ x: 50, y: 50 });
    });

    it('stops dragging on mouseup', async () => {
      const { useDraggable } = await import('./useDraggable');
      const { result } = renderHook(() => useDraggable({ enabled: true }));
      act(() => {
        result.current.handleProps.onMouseDown({
          clientX: 100,
          clientY: 200,
          preventDefault: vi.fn(),
        } as unknown as React.MouseEvent);
      });
      act(() => {
        document.dispatchEvent(new MouseEvent('mouseup'));
      });
      expect(result.current.isDragging).toBe(false);
    });
  });
});
