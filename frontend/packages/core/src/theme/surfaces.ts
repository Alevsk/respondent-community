/**
 * Surface, border, interaction, and severity tokens.
 *
 * These formalize the primary-tinted glow design system into reusable constants.
 * Components import these instead of hardcoding rgba values inline.
 */

// ---------------------------------------------------------------------------
// Surface elevation levels
// ---------------------------------------------------------------------------
export const SURFACE = {
  /** Nested containers, grouped fields inside panels */
  base: {
    bg: 'rgba(255, 255, 255, 0.01)',
    border: 'rgba(255, 255, 255, 0.08)',
    shadow: 'none',
  },
  /** Panel containers, cards */
  raised: {
    bg: 'rgba(255, 255, 255, 0.02)',
    border: 'rgba(255, 255, 255, 0.10)',
    shadow: '0 0 12px rgba(0, 0, 0, 0.2)',
  },
  /** Overlays, floating panels, modals */
  overlay: {
    bg: 'rgba(255, 255, 255, 0.03)',
    border: 'rgba(255, 255, 255, 0.12)',
    shadow: '0 0 16px rgba(0, 0, 0, 0.3)',
  },
  /** Panel header — flat subtle surface */
  header: {
    bg: 'rgba(255, 255, 255, 0.02)',
  },
  /** Collapsed panel header — dimmed flat surface */
  headerCollapsed: {
    bg: 'rgba(255, 255, 255, 0.015)',
  },
  /** Table row zebra striping */
  zebra: {
    even: 'transparent',
    odd: 'rgba(255, 255, 255, 0.02)',
  },
} as const;

// ---------------------------------------------------------------------------
// Border tokens
// ---------------------------------------------------------------------------
export const BORDER = {
  /** Row dividers, nested containers */
  subtle: 'rgba(255, 255, 255, 0.04)',
  /** Panel edges, table header bottom */
  default: 'rgba(255, 255, 255, 0.08)',
  /** Active/focused elements */
  strong: 'rgba(255, 255, 255, 0.14)',
  /** Selection indicators, active tabs (solid) */
  accent: '#00ff9d',
} as const;

// ---------------------------------------------------------------------------
// Interaction tokens
// ---------------------------------------------------------------------------
export const HOVER = {
  /** Table rows, sidebar items */
  row: 'rgba(255, 255, 255, 0.03)',
  /** Icon buttons, tab items */
  button: 'rgba(255, 255, 255, 0.05)',
  /** Active states, pressed */
  intense: 'rgba(255, 255, 255, 0.08)',
  /** FieldRow hover in clean variant */
  fieldRow: 'rgba(255, 255, 255, 0.04)',
} as const;

export const FOCUS = {
  /** Keyboard focus outline */
  ring: '0 0 0 1px rgba(0, 255, 157, 0.3)',
} as const;

export const SELECTED = {
  /** Selected table row background */
  row: 'rgba(0, 255, 157, 0.08)',
  /** Selected row left border */
  rowBorder: '2px solid #00ff9d',
} as const;

// ---------------------------------------------------------------------------
// Severity color tokens
// ---------------------------------------------------------------------------
export const SEVERITY = {
  critical: { bg: 'rgba(255, 80, 80, 0.15)', text: '#ff5555', border: '#ff5555' },
  high: { bg: 'rgba(255, 165, 0, 0.12)', text: '#ffaa00', border: '#ffaa00' },
  medium: { bg: 'rgba(0, 255, 157, 0.10)', text: '#00ff9d', border: '#00ff9d' },
  low: {
    bg: 'rgba(0, 255, 157, 0.06)',
    text: 'rgba(0, 255, 157, 0.5)',
    border: 'rgba(0, 255, 157, 0.3)',
  },
  info: { bg: 'rgba(0, 170, 255, 0.10)', text: '#00aaff', border: '#00aaff' },
} as const;
