import type { SliceCreator } from './types';
import type { AppMode } from '../models/ui';

const STORAGE_KEY = 'respondent:activeMode';
const VALID_MODES: AppMode[] = ['dashboard', 'immersive'];
const DEFAULT_MODE: AppMode = 'dashboard';

function readPersistedMode(): AppMode {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored && VALID_MODES.includes(stored as AppMode)) {
      return stored as AppMode;
    }
  } catch {
    // localStorage unavailable (SSR, privacy mode)
  }
  return DEFAULT_MODE;
}

function persistMode(mode: AppMode): void {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // localStorage unavailable
  }
}

export interface ModeSlice {
  activeMode: AppMode;
  setMode: (mode: AppMode) => void;
  toggleMode: () => void;
}

export const createModeSlice: SliceCreator<ModeSlice> = (set) => ({
  activeMode: readPersistedMode(),
  setMode: (mode) => {
    persistMode(mode);
    set({ activeMode: mode });
  },
  toggleMode: () =>
    set((state) => {
      const next: AppMode = state.activeMode === 'dashboard' ? 'immersive' : 'dashboard';
      persistMode(next);
      return { activeMode: next };
    }),
});
