import { describe, it, expect } from 'vitest';

describe('core theme', () => {
  it('exports a MUI theme with dark mode', async () => {
    const { theme } = await import('./index');
    expect(theme).toBeDefined();
    expect(theme.palette.mode).toBe('dark');
    expect(theme.palette.primary.main).toBe('#00ff9d');
  });

  it('buildTheme("compact") has dashboardBase fontSize 0.75rem', async () => {
    const { buildTheme } = await import('./index');
    const t = buildTheme('compact');
    expect(t.typography.dashboardBase?.fontSize).toBe('0.75rem');
  });

  it('buildTheme("comfortable") has dashboardLg fontSize 1.05rem', async () => {
    const { buildTheme } = await import('./index');
    const t = buildTheme('comfortable');
    expect(t.typography.dashboardLg?.fontSize).toBe('1.05rem');
  });

  it('buildTheme() with no arg is equivalent to buildTheme("default")', async () => {
    const { buildTheme } = await import('./index');
    const noArg = buildTheme();
    const withDefault = buildTheme('default');
    expect(noArg.typography.dashboardBase?.fontSize).toBe(
      withDefault.typography.dashboardBase?.fontSize,
    );
    expect(noArg.typography.dashboardLg?.fontSize).toBe(
      withDefault.typography.dashboardLg?.fontSize,
    );
  });

  it('static theme export still has palette.mode === "dark"', async () => {
    const { theme } = await import('./index');
    expect(theme.palette.mode).toBe('dark');
  });

  it('exports semanticColors', async () => {
    const { semanticColors } = await import('./index');
    expect(semanticColors.background.paperTranslucent).toBe('rgba(10, 10, 10, 0.85)');
  });

  it('exports design tokens', async () => {
    const { DESIGN_TOKENS, FONT_FAMILY } = await import('./index');
    expect(DESIGN_TOKENS.spacing.compact).toBe(1);
    // FONT_FAMILY now includes JetBrains Mono as the first choice
    expect(FONT_FAMILY).toContain('JetBrains Mono');
  });

  it('exports semanticColors with primary-tinted border', async () => {
    const { semanticColors } = await import('./index');
    expect(semanticColors.border.subtle).toBe('rgba(0, 255, 157, 0.06)');
  });

  it('exports design tokens with upgraded font family', async () => {
    const { FONT_FAMILY } = await import('./index');
    expect(FONT_FAMILY).toContain('JetBrains Mono');
  });

  it('exports surface tokens', async () => {
    const { SURFACE, BORDER, HOVER } = await import('./index');
    expect(SURFACE.raised.bg).toBeDefined();
    expect(BORDER.accent).toBe('#00ff9d');
    expect(HOVER.row).toBeDefined();
  });

  it('exports COMPONENT_SIZES with correct values', async () => {
    const { COMPONENT_SIZES } = await import('./index');
    expect(COMPONENT_SIZES.rowHeight.default).toBe(30);
    expect(COMPONENT_SIZES.rowHeight.compact).toBe(24);
    expect(COMPONENT_SIZES.rowHeight.comfortable).toBe(36);
    expect(COMPONENT_SIZES.headerHeight).toBe(28);
    expect(COMPONENT_SIZES.tabBarHeight).toBe(32);
  });

  it('exports DENSITY_SCALES with correct font sizes', async () => {
    const { DENSITY_SCALES } = await import('./index');
    expect(DENSITY_SCALES.default.dashboardBase.fontSize).toBe('0.8rem');
    expect(DENSITY_SCALES.default.dashboardXs.fontSize).toBe('0.65rem');
    expect(DENSITY_SCALES.default.dashboardSm.fontSize).toBe('0.75rem');
    expect(DENSITY_SCALES.default.dashboardMd.fontSize).toBe('0.875rem');
    expect(DENSITY_SCALES.default.dashboardLg.fontSize).toBe('0.95rem');
    expect(DENSITY_SCALES.default.rowHeight).toBe(30);
  });

  it('DENSITY_SCALES compact uses smaller font sizes than default', async () => {
    const { DENSITY_SCALES } = await import('./index');
    expect(DENSITY_SCALES.compact.dashboardBase.fontSize).toBe('0.75rem');
    expect(DENSITY_SCALES.compact.rowHeight).toBe(24);
  });

  it('DENSITY_SCALES comfortable uses larger font sizes than default', async () => {
    const { DENSITY_SCALES } = await import('./index');
    expect(DENSITY_SCALES.comfortable.dashboardBase.fontSize).toBe('0.875rem');
    expect(DENSITY_SCALES.comfortable.rowHeight).toBe(36);
  });
});
