/**
 * Respondent MUI theme.
 *
 * Assembled from:
 *   - palettes.ts  (color palette + semantic alpha tokens)
 *   - tokens.ts    (spacing scale, typography, elevation)
 *
 * Usage:
 *   import { theme } from './theme'
 *   <ThemeProvider theme={theme}> ... </ThemeProvider>
 *
 *   // Density-aware variant (for dynamic theming):
 *   import { buildTheme } from './theme'
 *   const densityTheme = useMemo(() => buildTheme(density), [density]);
 */
import { createTheme } from '@mui/material/styles';
import { palette, semanticColors } from './palettes';
import { typography, FONT_FAMILY, DENSITY_SCALES, DASHBOARD_TYPOGRAPHY } from './tokens';
import type { Density } from './tokens';
import { SURFACE } from './surfaces';

/**
 * Creates a MUI theme for the given UI density.
 *
 * All component overrides are included regardless of density — the density
 * parameter only influences the dashboard typography variant sizes so that
 * the entire component tree reflects the user's preferred information density.
 */
export function buildTheme(density: Density = 'default') {
  const scale = DENSITY_SCALES[density];

  return createTheme({
    breakpoints: {
      values: {
        xs: 0,
        sm: 600,
        md: 900,
        lg: 1200,
        xl: 1536,
      },
    },
    palette,
    typography: {
      ...typography,
      dashboardXxs: scale.dashboardXxs,
      dashboardXs: scale.dashboardXs,
      dashboardSm: scale.dashboardSm,
      dashboardBase: scale.dashboardBase,
      dashboardMd: scale.dashboardMd,
      dashboardLg: scale.dashboardLg,
    },
    components: {
      MuiPaper: {
        styleOverrides: {
          root: {
            backgroundImage: 'none',
            backgroundColor: semanticColors.background.paperTranslucent,
            backdropFilter: 'blur(8px)',
            border: `1px solid ${SURFACE.raised.border}`,
          },
        },
      },
      MuiTooltip: {
        defaultProps: {
          arrow: true,
          placement: 'top' as const,
        },
        styleOverrides: {
          tooltip: {
            backgroundColor: semanticColors.background.tooltipBg,
            border: `1px solid ${semanticColors.primary.alpha20}`,
            backdropFilter: 'blur(8px)',
            color: '#ffffff',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            fontFamily: FONT_FAMILY,
            fontWeight: 600,
            letterSpacing: '0.05em',
            padding: '4px 8px',
          },
          arrow: {
            color: semanticColors.background.tooltipBg,
            '&::before': {
              border: `1px solid ${semanticColors.primary.alpha20}`,
            },
          },
        },
      },
      MuiIconButton: {
        styleOverrides: {
          root: {
            '&:hover': {
              backgroundColor: semanticColors.primary.alpha10,
            },
          },
        },
      },
      MuiTableSortLabel: {
        styleOverrides: {
          root: {
            '&.Mui-active': {
              color: (palette.primary as { main: string }).main,
            },
          },
          icon: {
            color: `${(palette.primary as { main: string }).main} !important`,
          },
        },
      },
    },
  });
}

/** Static default-density theme — single source of truth, derived from buildTheme. */
export const theme = buildTheme('default');

// Re-export palette helpers and tokens for consumer convenience
export { alpha, semanticColors } from './palettes';
export {
  DESIGN_TOKENS,
  SPACING,
  FONT_FAMILY,
  FONT_WEIGHTS,
  ELEVATION,
  DASHBOARD_TYPOGRAPHY,
  COMPONENT_SIZES,
  DENSITY_SCALES,
} from './tokens';
export type { Density } from './tokens';
export { SURFACE, BORDER, HOVER, FOCUS, SELECTED, SEVERITY } from './surfaces';
