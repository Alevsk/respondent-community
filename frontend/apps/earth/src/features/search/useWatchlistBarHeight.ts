/**
 * useWatchlistBarHeight — measures the watchlist bar and reports its height
 * to the Zustand store.
 *
 * Called exclusively from WatchlistBar. Attaches a ResizeObserver to the
 * rendered ConfigPanel element (found via data-testid) and writes the
 * measured height into `useUIStore.watchlistBarHeight`.
 *
 * Sibling components (e.g. MobileEntityPanel) read `watchlistBarHeight`
 * from the store to position themselves above the watchlist — no direct
 * DOM queries from consumers.
 *
 * @param visible - whether the watchlist bar is actually rendering content
 *   (i.e. not hidden by an early return). The effect re-runs when this
 *   changes so it can attach/detach the ResizeObserver.
 */

import { useEffect } from 'react';
import { useUIStore } from '@/app/store';

const WATCHLIST_SELECTOR = '[data-testid="watchlist-bar"]';

export function useWatchlistBarHeight(visible: boolean) {
  useEffect(() => {
    if (!visible) {
      useUIStore.getState().setWatchlistBarHeight(0);
      return;
    }

    // Use rAF to ensure the DOM has been painted after the render that
    // made `visible` true (ConfigPanel may not be in the DOM yet during
    // the synchronous phase of the commit).
    const raf = requestAnimationFrame(() => {
      const el = document.querySelector(WATCHLIST_SELECTOR);
      if (!el) return;

      const update = () => {
        const height = el.getBoundingClientRect().height;
        useUIStore.getState().setWatchlistBarHeight(height);
      };
      update();

      const observer = new ResizeObserver(update);
      observer.observe(el);

      // Store cleanup ref on the effect's closure
      cleanupRef.observer = observer;
    });

    const cleanupRef: { observer: ResizeObserver | null } = { observer: null };

    return () => {
      cancelAnimationFrame(raf);
      cleanupRef.observer?.disconnect();
      useUIStore.getState().setWatchlistBarHeight(0);
    };
  }, [visible]);
}
