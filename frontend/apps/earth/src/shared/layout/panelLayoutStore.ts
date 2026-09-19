import { create } from 'zustand';

export type AnchorZone = 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left';

export interface PanelEntry {
  id: string;
  zone: AnchorZone;
  width: number;
  visible: boolean;
  order: number;
}

interface PanelLayoutState {
  panels: Record<string, PanelEntry>;
  zStack: string[];
  _nextOrder: number;
  register: (id: string, zone: AnchorZone, width: number) => void;
  unregister: (id: string) => void;
  setVisible: (id: string, visible: boolean) => void;
  bringToFront: (id: string) => void;
}

/**
 * Computes the CSS offset for a panel based on preceding visible panels in the same zone.
 * Formula: baseMargin + SUM(preceding_visible_width + gap)
 */
export function selectPanelOffset(
  panels: Record<string, PanelEntry>,
  panelId: string,
  gap: number = 16,
  baseMargin: number = 16,
): number {
  const panel = panels[panelId];
  if (!panel) return baseMargin;

  const preceding = Object.values(panels).filter(
    (p) => p.zone === panel.zone && p.visible && p.order < panel.order,
  );

  return baseMargin + preceding.reduce((sum, p) => sum + p.width + gap, 0);
}

/** Base z-index for normal floating panels. */
const PANEL_Z_BASE = 1100;

/**
 * Computes the z-index for a panel based on its position in the focus stack.
 * Panels not in the stack get the base value; stacked panels get base + position + 1.
 */
export function selectPanelZIndex(zStack: string[], panelId: string): number {
  const idx = zStack.indexOf(panelId);
  return idx === -1 ? PANEL_Z_BASE : PANEL_Z_BASE + idx + 1;
}

export const usePanelLayoutStore = create<PanelLayoutState>()((set) => ({
  panels: {},
  zStack: [],
  _nextOrder: 0,

  register: (id, zone, width) =>
    set((state) => {
      // Idempotent: if already registered, update zone/width but keep order
      if (state.panels[id]) {
        return {
          panels: {
            ...state.panels,
            [id]: { ...state.panels[id], zone, width },
          },
        };
      }
      return {
        panels: {
          ...state.panels,
          [id]: { id, zone, width, visible: false, order: state._nextOrder },
        },
        _nextOrder: state._nextOrder + 1,
      };
    }),

  unregister: (id) =>
    set((state) => {
      const rest = { ...state.panels };
      delete rest[id];
      return { panels: rest, zStack: state.zStack.filter((pid) => pid !== id) };
    }),

  bringToFront: (id) =>
    set((state) => {
      if (state.zStack[state.zStack.length - 1] === id) return state;
      const filtered = state.zStack.filter((pid) => pid !== id);
      return { zStack: [...filtered, id] };
    }),

  setVisible: (id, visible) =>
    set((state) => {
      if (!state.panels[id]) return state;
      return {
        panels: {
          ...state.panels,
          [id]: { ...state.panels[id], visible },
        },
      };
    }),
}));
