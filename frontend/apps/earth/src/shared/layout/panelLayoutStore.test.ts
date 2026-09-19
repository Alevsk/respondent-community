import { describe, it, expect, beforeEach } from 'vitest';
import { usePanelLayoutStore, selectPanelOffset, selectPanelZIndex } from './panelLayoutStore';

describe('panelLayoutStore', () => {
  beforeEach(() => {
    usePanelLayoutStore.setState({ panels: {}, _nextOrder: 0, zStack: [] });
  });

  it('registers a panel with monotonic order', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);
    register('b', 'top-right', 320);

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.a).toMatchObject({
      id: 'a',
      zone: 'top-right',
      width: 280,
      visible: false,
      order: 0,
    });
    expect(panels.b).toMatchObject({
      id: 'b',
      zone: 'top-right',
      width: 320,
      visible: false,
      order: 1,
    });
  });

  it('idempotent registration preserves order', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);
    register('a', 'top-right', 300); // re-register with different width

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.a.width).toBe(300);
    expect(panels.a.order).toBe(0);
  });

  it('unregisters a panel', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);

    usePanelLayoutStore.getState().unregister('a');
    expect(usePanelLayoutStore.getState().panels.a).toBeUndefined();
  });

  it('sets visibility', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);

    usePanelLayoutStore.getState().setVisible('a', true);
    expect(usePanelLayoutStore.getState().panels.a.visible).toBe(true);

    usePanelLayoutStore.getState().setVisible('a', false);
    expect(usePanelLayoutStore.getState().panels.a.visible).toBe(false);
  });

  it('setVisible on unknown panel is a no-op', () => {
    const before = usePanelLayoutStore.getState();
    usePanelLayoutStore.getState().setVisible('nonexistent', true);
    expect(usePanelLayoutStore.getState()).toBe(before);
  });

  it('unregister unknown panel does not throw', () => {
    usePanelLayoutStore.getState().unregister('nonexistent');
    expect(usePanelLayoutStore.getState().panels).toEqual({});
  });

  it('idempotent registration can update zone', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);
    register('a', 'bottom-right', 280);

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.a.zone).toBe('bottom-right');
    expect(panels.a.order).toBe(0); // order preserved
  });

  it('order counter increments independently of unregistration', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100); // order 0
    register('b', 'top-right', 200); // order 1

    usePanelLayoutStore.getState().unregister('a');
    register('c', 'top-right', 150); // order 2, NOT 0

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.c.order).toBe(2);
    expect(panels.b.order).toBe(1);
  });

  it('new panels default to visible: false', () => {
    usePanelLayoutStore.getState().register('a', 'top-right', 280);
    expect(usePanelLayoutStore.getState().panels.a.visible).toBe(false);
  });

  it('registers panels across different zones', () => {
    const { register } = usePanelLayoutStore.getState();
    register('tr', 'top-right', 100);
    register('tl', 'top-left', 200);
    register('br', 'bottom-right', 300);
    register('bl', 'bottom-left', 400);

    const { panels } = usePanelLayoutStore.getState();
    expect(panels.tr.zone).toBe('top-right');
    expect(panels.tl.zone).toBe('top-left');
    expect(panels.br.zone).toBe('bottom-right');
    expect(panels.bl.zone).toBe('bottom-left');
  });
});

describe('selectPanelOffset', () => {
  beforeEach(() => {
    usePanelLayoutStore.setState({ panels: {}, _nextOrder: 0, zStack: [] });
  });

  it('returns baseMargin when no preceding visible panels', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 280);
    setVisible('a', true);

    const { panels } = usePanelLayoutStore.getState();
    expect(selectPanelOffset(panels, 'a', 16, 16)).toBe(16);
  });

  it('returns baseMargin for unknown panel', () => {
    expect(selectPanelOffset({}, 'unknown', 16, 16)).toBe(16);
  });

  it('offsets based on one preceding visible panel', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('effects', 'top-right', 280);
    register('entity', 'top-right', 320);
    setVisible('effects', true);
    setVisible('entity', true);

    const { panels } = usePanelLayoutStore.getState();
    // effects (order 0): 16
    expect(selectPanelOffset(panels, 'effects', 16, 16)).toBe(16);
    // entity (order 1): 16 + (280 + 16) = 312
    expect(selectPanelOffset(panels, 'entity', 16, 16)).toBe(312);
  });

  it('offsets based on two preceding visible panels', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100);
    register('b', 'top-right', 200);
    register('c', 'top-right', 150);
    setVisible('a', true);
    setVisible('b', true);
    setVisible('c', true);

    const { panels } = usePanelLayoutStore.getState();
    // c (order 2): 16 + (100 + 16) + (200 + 16) = 348
    expect(selectPanelOffset(panels, 'c', 16, 16)).toBe(348);
  });

  it('hidden panel does not contribute to offset', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('effects', 'top-right', 280);
    register('entity', 'top-right', 320);
    setVisible('effects', false); // hidden
    setVisible('entity', true);

    const { panels } = usePanelLayoutStore.getState();
    // entity (order 1): effects is hidden, so offset = 16
    expect(selectPanelOffset(panels, 'entity', 16, 16)).toBe(16);
  });

  it('different zones do not affect each other', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('top-panel', 'top-right', 280);
    register('bottom-panel', 'bottom-right', 300);
    setVisible('top-panel', true);
    setVisible('bottom-panel', true);

    const { panels } = usePanelLayoutStore.getState();
    // bottom-panel is alone in bottom-right zone
    expect(selectPanelOffset(panels, 'bottom-panel', 16, 16)).toBe(16);
  });

  it('uses custom gap and baseMargin', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'bottom-left', 200);
    register('b', 'bottom-left', 150);
    setVisible('a', true);
    setVisible('b', true);

    const { panels } = usePanelLayoutStore.getState();
    // b: baseMargin(80) + (200 + gap(8)) = 288
    expect(selectPanelOffset(panels, 'b', 8, 80)).toBe(288);
  });

  it('uses default gap=16 and baseMargin=16 when omitted', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100);
    register('b', 'top-right', 200);
    setVisible('a', true);
    setVisible('b', true);

    const { panels } = usePanelLayoutStore.getState();
    // b: default baseMargin(16) + (100 + default gap(16)) = 132
    expect(selectPanelOffset(panels, 'b')).toBe(132);
  });

  it('only counts preceding panels (lower order), not following', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100); // order 0
    register('b', 'top-right', 200); // order 1
    register('c', 'top-right', 300); // order 2
    setVisible('a', true);
    setVisible('b', true);
    setVisible('c', true);

    const { panels } = usePanelLayoutStore.getState();
    // b only counts a (order 0), not c (order 2)
    expect(selectPanelOffset(panels, 'b', 16, 16)).toBe(16 + 100 + 16); // 132
  });

  it('hidden middle panel collapses gap', () => {
    const { register, setVisible } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100); // order 0
    register('b', 'top-right', 200); // order 1
    register('c', 'top-right', 300); // order 2
    setVisible('a', true);
    setVisible('b', false); // hidden
    setVisible('c', true);

    const { panels } = usePanelLayoutStore.getState();
    // c only counts a (b is hidden): 16 + (100 + 16) = 132
    expect(selectPanelOffset(panels, 'c', 16, 16)).toBe(132);
  });

  it('all panels hidden yields baseMargin for any panel', () => {
    const { register } = usePanelLayoutStore.getState();
    register('a', 'top-right', 100);
    register('b', 'top-right', 200);
    // Neither visible (default false)

    const { panels } = usePanelLayoutStore.getState();
    expect(selectPanelOffset(panels, 'b', 16, 16)).toBe(16);
  });
});

describe('z-index stack', () => {
  beforeEach(() => {
    usePanelLayoutStore.setState({ panels: {}, _nextOrder: 0, zStack: [] });
  });

  it('bringToFront adds panel to end of zStack', () => {
    usePanelLayoutStore.getState().bringToFront('a');
    expect(usePanelLayoutStore.getState().zStack).toEqual(['a']);
  });

  it('bringToFront moves existing panel to end', () => {
    const { bringToFront } = usePanelLayoutStore.getState();
    bringToFront('a');
    bringToFront('b');
    bringToFront('a');
    expect(usePanelLayoutStore.getState().zStack).toEqual(['b', 'a']);
  });

  it('bringToFront is idempotent when panel is already on top', () => {
    const { bringToFront } = usePanelLayoutStore.getState();
    bringToFront('a');
    bringToFront('b');
    bringToFront('b');
    expect(usePanelLayoutStore.getState().zStack).toEqual(['a', 'b']);
  });

  it('unregister removes panel from zStack', () => {
    const { bringToFront } = usePanelLayoutStore.getState();
    bringToFront('a');
    bringToFront('b');
    bringToFront('c');
    usePanelLayoutStore.getState().register('b', 'top-right', 100);
    usePanelLayoutStore.getState().unregister('b');
    expect(usePanelLayoutStore.getState().zStack).toEqual(['a', 'c']);
  });

  it('selectPanelZIndex returns base for unknown panel', () => {
    expect(selectPanelZIndex([], 'unknown')).toBe(1100);
  });

  it('selectPanelZIndex returns base + position + 1', () => {
    expect(selectPanelZIndex(['a', 'b', 'c'], 'a')).toBe(1101);
    expect(selectPanelZIndex(['a', 'b', 'c'], 'b')).toBe(1102);
    expect(selectPanelZIndex(['a', 'b', 'c'], 'c')).toBe(1103);
  });

  it('selectPanelZIndex top panel has highest z-index', () => {
    const { bringToFront } = usePanelLayoutStore.getState();
    bringToFront('x');
    bringToFront('y');
    bringToFront('z');
    const { zStack } = usePanelLayoutStore.getState();
    expect(selectPanelZIndex(zStack, 'z')).toBeGreaterThan(selectPanelZIndex(zStack, 'y'));
    expect(selectPanelZIndex(zStack, 'y')).toBeGreaterThan(selectPanelZIndex(zStack, 'x'));
  });
});
