import { describe, it, expect, beforeEach, vi } from 'vitest';
import { create } from 'zustand';
import { createModeSlice, type ModeSlice } from './createModeSlice';

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      store = {};
    }),
  };
})();
Object.defineProperty(globalThis, 'localStorage', {
  value: localStorageMock,
  writable: true,
  configurable: true,
});

function createTestStore() {
  return create<ModeSlice>()((...args) => createModeSlice(...args));
}

describe('createModeSlice', () => {
  beforeEach(() => {
    localStorageMock.clear();
    vi.clearAllMocks();
  });

  it('defaults to dashboard mode', () => {
    const store = createTestStore();
    expect(store.getState().activeMode).toBe('dashboard');
  });

  it('reads persisted mode from localStorage', () => {
    localStorageMock.setItem('respondent:activeMode', 'immersive');
    const store = createTestStore();
    expect(store.getState().activeMode).toBe('immersive');
  });

  it('ignores invalid localStorage values', () => {
    localStorageMock.setItem('respondent:activeMode', 'invalid');
    const store = createTestStore();
    expect(store.getState().activeMode).toBe('dashboard');
  });

  it('setMode updates state and persists to localStorage', () => {
    const store = createTestStore();
    store.getState().setMode('immersive');
    expect(store.getState().activeMode).toBe('immersive');
    expect(localStorageMock.setItem).toHaveBeenCalledWith('respondent:activeMode', 'immersive');
  });

  it('toggleMode switches between modes', () => {
    const store = createTestStore();
    expect(store.getState().activeMode).toBe('dashboard');
    store.getState().toggleMode();
    expect(store.getState().activeMode).toBe('immersive');
    store.getState().toggleMode();
    expect(store.getState().activeMode).toBe('dashboard');
  });
});
