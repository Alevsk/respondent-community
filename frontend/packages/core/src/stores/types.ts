import type { StateCreator } from 'zustand';

/**
 * Generic slice creator type for Zustand store composition.
 * TSlice = the state + actions this slice provides.
 * TStore = the full composed store type (defaults to TSlice for standalone use).
 */
export type SliceCreator<TSlice, TStore = TSlice> = StateCreator<TStore, [], [], TSlice>;
