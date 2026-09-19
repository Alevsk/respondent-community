/**
 * useWatchlistBarHeight Hook Tests
 *
 * Tests that the hook correctly reads the watchlist bar DOM element,
 * writes its measured height into the Zustand store via setWatchlistBarHeight,
 * and resets the height back to 0 on unmount or when visible becomes false.
 *
 * ResizeObserver and requestAnimationFrame are mocked because jsdom does
 * not implement them.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useUIStore } from '@/app/store';
import { useWatchlistBarHeight } from './useWatchlistBarHeight';

// ─── ResizeObserver mock ──────────────────────────────────────────────────────

class MockResizeObserver {
  static instances: MockResizeObserver[] = [];
  callback: ResizeObserverCallback;
  observedElements: Element[] = [];

  constructor(callback: ResizeObserverCallback) {
    this.callback = callback;
    MockResizeObserver.instances.push(this);
  }

  observe(el: Element) {
    this.observedElements.push(el);
  }

  unobserve(el: Element) {
    this.observedElements = this.observedElements.filter((e) => e !== el);
  }

  disconnect() {
    this.observedElements = [];
  }
}

// ─── requestAnimationFrame mock ───────────────────────────────────────────────

let rafCallbacks: Array<FrameRequestCallback> = [];

function mockRaf(cb: FrameRequestCallback): number {
  rafCallbacks.push(cb);
  return rafCallbacks.length;
}

function flushRaf() {
  const cbs = [...rafCallbacks];
  rafCallbacks = [];
  cbs.forEach((cb) => cb(performance.now()));
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function mountWatchlistBar(height = 80): { el: HTMLElement; cleanup: () => void } {
  const el = document.createElement('div');
  el.setAttribute('data-testid', 'watchlist-bar');
  Object.defineProperty(el, 'getBoundingClientRect', {
    value: () => ({ height, width: 375, top: 0, left: 0, right: 375, bottom: height }) as DOMRect,
    configurable: true,
  });
  document.body.appendChild(el);
  return {
    el,
    cleanup: () => {
      if (el.parentNode) el.parentNode.removeChild(el);
    },
  };
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('useWatchlistBarHeight', () => {
  beforeEach(() => {
    MockResizeObserver.instances = [];
    rafCallbacks = [];
    vi.stubGlobal('ResizeObserver', MockResizeObserver);
    vi.stubGlobal('requestAnimationFrame', mockRaf);
    vi.stubGlobal('cancelAnimationFrame', vi.fn());
    useUIStore.setState({ watchlistBarHeight: 0 });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('when visible is true and element is present', () => {
    it('writes the measured height to the store after rAF fires', () => {
      const { cleanup } = mountWatchlistBar(80);
      renderHook(() => useWatchlistBarHeight(true));

      // Before rAF fires, store should still be 0
      expect(useUIStore.getState().watchlistBarHeight).toBe(0);

      act(() => flushRaf());

      expect(useUIStore.getState().watchlistBarHeight).toBe(80);
      cleanup();
    });

    it('attaches a ResizeObserver to the watchlist bar element', () => {
      const { el, cleanup } = mountWatchlistBar(80);
      renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());

      const observer = MockResizeObserver.instances[0];
      expect(observer).toBeDefined();
      expect(observer.observedElements).toContain(el);
      cleanup();
    });

    it('updates the store height when the ResizeObserver fires with a new size', () => {
      const { el, cleanup } = mountWatchlistBar(80);
      renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());

      // Simulate resize
      Object.defineProperty(el, 'getBoundingClientRect', {
        value: () =>
          ({ height: 120, width: 375, top: 0, left: 0, right: 375, bottom: 120 }) as DOMRect,
        configurable: true,
      });
      const observer = MockResizeObserver.instances[0];
      observer.callback([], observer as unknown as ResizeObserver);

      expect(useUIStore.getState().watchlistBarHeight).toBe(120);
      cleanup();
    });

    it('resets watchlistBarHeight to 0 on unmount', () => {
      const { cleanup } = mountWatchlistBar(80);
      const { unmount } = renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());
      expect(useUIStore.getState().watchlistBarHeight).toBe(80);

      unmount();
      expect(useUIStore.getState().watchlistBarHeight).toBe(0);
      cleanup();
    });

    it('disconnects the ResizeObserver on unmount', () => {
      const { cleanup } = mountWatchlistBar(80);
      const { unmount } = renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());

      const observer = MockResizeObserver.instances[0];
      const disconnectSpy = vi.spyOn(observer, 'disconnect');

      unmount();
      expect(disconnectSpy).toHaveBeenCalledOnce();
      cleanup();
    });
  });

  describe('when visible is false', () => {
    it('sets watchlistBarHeight to 0 in the store', () => {
      useUIStore.setState({ watchlistBarHeight: 200 });
      renderHook(() => useWatchlistBarHeight(false));

      expect(useUIStore.getState().watchlistBarHeight).toBe(0);
    });

    it('does not create a ResizeObserver', () => {
      renderHook(() => useWatchlistBarHeight(false));
      expect(MockResizeObserver.instances).toHaveLength(0);
    });

    it('does not schedule a requestAnimationFrame', () => {
      renderHook(() => useWatchlistBarHeight(false));
      expect(rafCallbacks).toHaveLength(0);
    });
  });

  describe('when visible transitions from false to true', () => {
    it('attaches observer after the transition', () => {
      const { cleanup } = mountWatchlistBar(150);
      const { rerender } = renderHook(
        ({ visible }: { visible: boolean }) => useWatchlistBarHeight(visible),
        { initialProps: { visible: false } },
      );

      expect(useUIStore.getState().watchlistBarHeight).toBe(0);

      rerender({ visible: true });
      act(() => flushRaf());

      expect(useUIStore.getState().watchlistBarHeight).toBe(150);
      cleanup();
    });
  });

  describe('when watchlist bar element is absent', () => {
    it('does not write to the store even when visible is true', () => {
      renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());
      expect(useUIStore.getState().watchlistBarHeight).toBe(0);
    });

    it('does not throw when unmounted', () => {
      const { unmount } = renderHook(() => useWatchlistBarHeight(true));
      act(() => flushRaf());
      expect(() => unmount()).not.toThrow();
      expect(useUIStore.getState().watchlistBarHeight).toBe(0);
    });
  });
});
