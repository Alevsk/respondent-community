import { describe, it, expect } from 'vitest';
import { SURFACE, BORDER, HOVER, FOCUS, SELECTED, SEVERITY } from './surfaces';

describe('surface tokens', () => {
  it('exports base surface with bg, border, shadow', () => {
    expect(SURFACE.base.bg).toBe('rgba(255, 255, 255, 0.01)');
    expect(SURFACE.base.border).toBe('rgba(255, 255, 255, 0.08)');
    expect(SURFACE.base.shadow).toBe('none');
  });

  it('exports raised surface with glow shadow', () => {
    expect(SURFACE.raised.bg).toBe('rgba(255, 255, 255, 0.02)');
    expect(SURFACE.raised.shadow).toContain('0 0 12px');
  });

  it('exports overlay surface', () => {
    expect(SURFACE.overlay.bg).toBe('rgba(255, 255, 255, 0.03)');
    expect(SURFACE.overlay.shadow).toContain('0 0 16px');
  });

  it('exports flat header surfaces', () => {
    expect(SURFACE.header.bg).toBe('rgba(255, 255, 255, 0.02)');
    expect(SURFACE.headerCollapsed.bg).toBe('rgba(255, 255, 255, 0.015)');
  });

  it('exports zebra stripe tokens', () => {
    expect(SURFACE.zebra.even).toBe('transparent');
    expect(SURFACE.zebra.odd).toBe('rgba(255, 255, 255, 0.02)');
  });
});

describe('border tokens', () => {
  it('exports all semantic border levels', () => {
    expect(BORDER.subtle).toBe('rgba(255, 255, 255, 0.04)');
    expect(BORDER.default).toBe('rgba(255, 255, 255, 0.08)');
    expect(BORDER.strong).toBe('rgba(255, 255, 255, 0.14)');
    expect(BORDER.accent).toBe('#00ff9d');
  });
});

describe('interaction tokens', () => {
  it('exports hover tokens', () => {
    expect(HOVER.row).toBe('rgba(255, 255, 255, 0.03)');
    expect(HOVER.button).toBe('rgba(255, 255, 255, 0.05)');
    expect(HOVER.intense).toBe('rgba(255, 255, 255, 0.08)');
  });

  it('exports focus ring', () => {
    expect(FOCUS.ring).toContain('rgba(0, 255, 157, 0.3)');
  });

  it('exports selected row tokens', () => {
    expect(SELECTED.row).toBe('rgba(0, 255, 157, 0.08)');
    expect(SELECTED.rowBorder).toBe('2px solid #00ff9d');
  });
});

describe('severity tokens', () => {
  it('exports all severity levels with bg, text, border', () => {
    for (const level of ['critical', 'high', 'medium', 'low', 'info'] as const) {
      expect(SEVERITY[level]).toHaveProperty('bg');
      expect(SEVERITY[level]).toHaveProperty('text');
      expect(SEVERITY[level]).toHaveProperty('border');
    }
  });

  it('critical is red-tinted', () => {
    expect(SEVERITY.critical.text).toBe('#ff5555');
  });

  it('info is blue-tinted', () => {
    expect(SEVERITY.info.text).toBe('#00aaff');
  });
});
