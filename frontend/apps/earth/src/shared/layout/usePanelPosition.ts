import { useEffect } from 'react';
import { usePanelLayoutStore, selectPanelOffset, type AnchorZone } from './panelLayoutStore';

interface UsePanelPositionOptions {
  id: string;
  zone: AnchorZone;
  width: number;
  visible: boolean;
  gap?: number;
  baseMargin?: number;
}

/**
 * Hook that registers a panel in the layout system and returns its computed position.
 * Returns `{ right: offset }` for right-anchored zones, `{ left: offset }` for left-anchored.
 */
export function usePanelPosition({
  id,
  zone,
  width,
  visible,
  gap = 16,
  baseMargin = 16,
}: UsePanelPositionOptions): { right?: number; left?: number } {
  const register = usePanelLayoutStore((s) => s.register);
  const unregister = usePanelLayoutStore((s) => s.unregister);
  const setVisible = usePanelLayoutStore((s) => s.setVisible);

  useEffect(() => {
    register(id, zone, width);
    return () => unregister(id);
  }, [id, zone, width, register, unregister]);

  useEffect(() => {
    setVisible(id, visible);
  }, [id, visible, setVisible]);

  const offset = usePanelLayoutStore((s) => selectPanelOffset(s.panels, id, gap, baseMargin));

  if (zone.endsWith('-right')) {
    return { right: offset };
  }
  return { left: offset };
}
