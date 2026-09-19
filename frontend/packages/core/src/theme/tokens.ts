/**
 * Design tokens for the Respondent MUI theme.
 *
 * Spacing uses MUI's 8-point grid (theme.spacing multipliers).
 * Typography defines the monospace font stack and semantic weight scale.
 */
import type React from 'react';
import type { TypographyOptions } from '@mui/material/styles/createTypography';

// ---------------------------------------------------------------------------
// MUI module augmentation — makes custom dashboard variants type-safe
// ---------------------------------------------------------------------------
declare module '@mui/material/styles' {
  interface TypographyVariants {
    dashboardXxs: React.CSSProperties;
    dashboardXs: React.CSSProperties;
    dashboardSm: React.CSSProperties;
    dashboardBase: React.CSSProperties;
    dashboardMd: React.CSSProperties;
    dashboardLg: React.CSSProperties;
  }
  interface TypographyVariantsOptions {
    dashboardXxs?: React.CSSProperties;
    dashboardXs?: React.CSSProperties;
    dashboardSm?: React.CSSProperties;
    dashboardBase?: React.CSSProperties;
    dashboardMd?: React.CSSProperties;
    dashboardLg?: React.CSSProperties;
  }
}

declare module '@mui/material/Typography' {
  interface TypographyPropsVariantOverrides {
    dashboardXxs: true;
    dashboardXs: true;
    dashboardSm: true;
    dashboardBase: true;
    dashboardMd: true;
    dashboardLg: true;
  }
}

// ---------------------------------------------------------------------------
// Spacing scale (multipliers for theme.spacing, which defaults to 8px)
// ---------------------------------------------------------------------------
export const SPACING = {
  /** 8px -- compact / dense layouts */
  compact: 1,
  /** 16px -- gap between related elements */
  elementGap: 2,
  /** 24px -- standard page padding / card gap */
  pageContent: 3,
  /** 24px -- gap between cards in grid layouts */
  cardGap: 3,
  /** 32px -- gap between major page sections */
  sectionGap: 4,
  /** 48px -- extra spacing for visual separation */
  loose: 6,
} as const;

// ---------------------------------------------------------------------------
// Typography
// ---------------------------------------------------------------------------

/** Monospace font stack used across the entire UI */
export const FONT_FAMILY =
  "'JetBrains Mono', 'SF Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace";

/** Semantic font-weight scale */
export const FONT_WEIGHTS = {
  regular: 400,
  medium: 500,
  semibold: 600,
  bold: 700,
} as const;

// ---------------------------------------------------------------------------
// Dashboard typography scale
// ---------------------------------------------------------------------------
// Dense, monospace scale for the IDE-style dashboard.  Centralised here so a
// single edit bumps every panel title / table cell / sidebar item at once.

/** Dashboard variant definitions — consumed by createTheme and typed via module augmentation */
export const DASHBOARD_TYPOGRAPHY = {
  /** 0.6rem — ultra-compact badges, icon labels, status indicators */
  dashboardXxs: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.6rem',
    letterSpacing: '0.05em',
    lineHeight: 1.3,
  },
  /** 0.65rem — badge labels, timestamps, tiny metadata */
  dashboardXs: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.65rem',
    letterSpacing: '0.05em',
    lineHeight: 1.4,
  },
  /** 0.75rem — column headers, footer counts, time buttons */
  dashboardSm: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.75rem',
    letterSpacing: '0.05em',
    lineHeight: 1.4,
  },
  /** 0.8rem — panel titles, table cells, filter input (the workhorse) */
  dashboardBase: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.8rem',
    letterSpacing: '0.05em',
    lineHeight: 1.5,
  },
  /** 0.875rem — sidebar items, tab names */
  dashboardMd: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.875rem',
    letterSpacing: '0.05em',
    lineHeight: 1.5,
  },
  /** 0.95rem — empty state messages, subheadings */
  dashboardLg: {
    fontFamily: FONT_FAMILY,
    fontSize: '0.95rem',
    letterSpacing: '0.05em',
    lineHeight: 1.5,
  },
} as const;

/** Full MUI TypographyOptions consumed by createTheme */
export const typography: TypographyOptions = {
  fontFamily: FONT_FAMILY,
  h6: {
    fontWeight: FONT_WEIGHTS.bold,
    letterSpacing: '0.05em',
  },
  ...DASHBOARD_TYPOGRAPHY,
};

// ---------------------------------------------------------------------------
// Elevation / shadow scale (optional, for components that need custom shadows)
// ---------------------------------------------------------------------------
export const ELEVATION = {
  none: 'none',
  xs: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
  sm: '0 1px 3px 0 rgba(0, 0, 0, 0.12)',
  md: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
  lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
  xl: '0 20px 25px -5px rgba(0, 0, 0, 0.15)',
} as const;

// ---------------------------------------------------------------------------
// Component size tokens
// ---------------------------------------------------------------------------
export const COMPONENT_SIZES = {
  rowHeight: { compact: 24, default: 30, comfortable: 36 },
  headerHeight: 28,
  tabBarHeight: 32,
  iconXs: 12,
  iconSm: 14,
  iconMd: 16,
  iconLg: 20,
  iconXl: 24,
} as const;

// ---------------------------------------------------------------------------
// Density scale tokens
// ---------------------------------------------------------------------------
export type Density = 'compact' | 'default' | 'comfortable';

export const DENSITY_SCALES: Record<
  Density,
  {
    dashboardXxs: React.CSSProperties;
    dashboardXs: React.CSSProperties;
    dashboardSm: React.CSSProperties;
    dashboardBase: React.CSSProperties;
    dashboardMd: React.CSSProperties;
    dashboardLg: React.CSSProperties;
    rowHeight: number;
  }
> = {
  compact: {
    dashboardXxs: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.55rem',
      letterSpacing: '0.05em',
      lineHeight: 1.3,
    },
    dashboardXs: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.65rem',
      letterSpacing: '0.05em',
      lineHeight: 1.4,
    },
    dashboardSm: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.7rem',
      letterSpacing: '0.05em',
      lineHeight: 1.4,
    },
    dashboardBase: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.75rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    dashboardMd: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.8rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    dashboardLg: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.85rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    rowHeight: 24,
  },
  default: {
    ...DASHBOARD_TYPOGRAPHY,
    rowHeight: 30,
  },
  comfortable: {
    dashboardXxs: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.65rem',
      letterSpacing: '0.05em',
      lineHeight: 1.3,
    },
    dashboardXs: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.7rem',
      letterSpacing: '0.05em',
      lineHeight: 1.4,
    },
    dashboardSm: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.8rem',
      letterSpacing: '0.05em',
      lineHeight: 1.4,
    },
    dashboardBase: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.875rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    dashboardMd: {
      fontFamily: FONT_FAMILY,
      fontSize: '0.95rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    dashboardLg: {
      fontFamily: FONT_FAMILY,
      fontSize: '1.05rem',
      letterSpacing: '0.05em',
      lineHeight: 1.5,
    },
    rowHeight: 36,
  },
} as const;

// ---------------------------------------------------------------------------
// Aggregate export matching the pattern in component-patterns.md
// ---------------------------------------------------------------------------
export const DESIGN_TOKENS = {
  spacing: SPACING,
  typography: {
    fontFamily: FONT_FAMILY,
    weights: FONT_WEIGHTS,
  },
  elevation: ELEVATION,
} as const;
