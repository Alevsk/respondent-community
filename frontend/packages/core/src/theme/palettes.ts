/**
 * Palette configuration for the Respondent MUI theme.
 *
 * Single source of truth for all color values. Components should reference
 * semantic tokens (e.g. `palette.primary.main`) rather than hardcoding hex
 * or rgba strings.
 */
import { alpha as muiAlpha } from '@mui/material/styles';
import type { PaletteOptions } from '@mui/material/styles';

// Re-export MUI's alpha utility so consumers can do:
//   import { alpha } from '../theme/palettes'
//   alpha(theme.palette.primary.main, 0.1)
export const alpha = muiAlpha;

// ---------------------------------------------------------------------------
// Raw color values (private to this module -- use palette tokens instead)
// ---------------------------------------------------------------------------
const PRIMARY = '#00ff9d';
const SECONDARY = '#ff006e';
const BG_DEFAULT = '#000000';
const BG_PAPER = '#0a0a0a';
const TEXT_PRIMARY = '#ffffff';
const TEXT_SECONDARY = '#888888';

// ---------------------------------------------------------------------------
// Semantic alpha tokens
//
// Pre-computed alpha variants of the palette colors so components can import
// a constant instead of calling `alpha()` inline. These are intentionally
// strings (CSS color values) and match the rgba values already used across
// the codebase.
//
// Components import these tokens or use alpha(theme.palette.primary.main, X)
// for alpha levels not in this table.
// ---------------------------------------------------------------------------
export const semanticColors = {
  primary: {
    /** rgba(0, 255, 157, 0.1) */
    alpha10: muiAlpha(PRIMARY, 0.1),
    /** rgba(0, 255, 157, 0.2) */
    alpha20: muiAlpha(PRIMARY, 0.2),
    /** rgba(0, 255, 157, 0.3) */
    alpha30: muiAlpha(PRIMARY, 0.3),
  },
  secondary: {
    /** rgba(255, 0, 110, 0.1) */
    alpha10: muiAlpha(SECONDARY, 0.1),
    /** rgba(255, 0, 110, 0.2) */
    alpha20: muiAlpha(SECONDARY, 0.2),
    /** rgba(255, 0, 110, 0.3) */
    alpha30: muiAlpha(SECONDARY, 0.3),
  },
  background: {
    /** rgba(10, 10, 10, 0.85) -- paper with transparency */
    paperTranslucent: 'rgba(10, 10, 10, 0.85)',
    /** rgba(5, 5, 5, 0.95) -- tooltip background */
    tooltipBg: 'rgba(5, 5, 5, 0.95)',
  },
  border: {
    /** rgba(255, 255, 255, 0.1) -- subtle border on dark surfaces */
    subtle: 'rgba(0, 255, 157, 0.06)',
  },
} as const;

// ---------------------------------------------------------------------------
// MUI PaletteOptions
// ---------------------------------------------------------------------------
export const palette: PaletteOptions = {
  mode: 'dark',
  background: {
    default: BG_DEFAULT,
    paper: BG_PAPER,
  },
  primary: {
    main: PRIMARY,
  },
  secondary: {
    main: SECONDARY,
  },
  text: {
    primary: TEXT_PRIMARY,
    secondary: TEXT_SECONDARY,
  },
};
