/**
 * useSearchKeyboard -- keyboard shortcuts for Entity Search & Find Mode.
 *
 * Shortcuts:
 *  - Ctrl+F / Cmd+F: Toggle Find Mode (prevents default browser find)
 *  - Ctrl+K / Cmd+K: Toggle search bar open
 *  - Escape: Close search bar if open, exit Find Mode if active
 */

import { useEffect } from 'react';
import { useUIStore } from '@/app/store';

export function useSearchKeyboard(): void {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const isMod = e.metaKey || e.ctrlKey;

      // Ctrl+F / Cmd+F -- toggle Find Mode
      if (isMod && e.key === 'f') {
        e.preventDefault();
        e.stopPropagation();
        useUIStore.getState().toggleFindMode();
        return;
      }

      // Ctrl+K / Cmd+K -- toggle search bar
      if (isMod && e.key === 'k') {
        e.preventDefault();
        e.stopPropagation();
        useUIStore.getState().toggleSearch();
        return;
      }

      // Escape -- close search bar, then exit Find Mode
      if (e.key === 'Escape') {
        const state = useUIStore.getState();
        if (state.searchOpen) {
          useUIStore.getState().toggleSearch();
          return;
        }
        if (state.findMode) {
          useUIStore.getState().toggleFindMode();
          return;
        }
      }
    };

    // Use capture phase to intercept before browser's built-in Ctrl+F
    window.addEventListener('keydown', onKeyDown, true);

    return () => {
      window.removeEventListener('keydown', onKeyDown, true);
    };
  }, []);
}
