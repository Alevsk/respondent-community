/**
 * Shared mobile layout constants (px values).
 *
 * Centralizes the magic numbers used by the bottom-anchored HUD elements
 * so that MobileBottomNav, WatchlistBar, MobileEntityPanel, and
 * EntitySearchBar all agree on spacing.
 */

/** Height of MobileBottomNav (icon 44px + padding + safe area handled separately). */
export const MOBILE_NAV_HEIGHT = 56;

/** Vertical gap between the bottom nav and the watchlist bar. */
export const MOBILE_NAV_GAP = 16;

/** Height of the mobile HUD header (RESPONDENT label + connection status). */
export const MOBILE_HEADER_HEIGHT = 40;

/** Gap between stacked bottom elements (entity panel above watchlist). */
export const MOBILE_STACK_GAP = 8;

/** Horizontal margin (px) for floating mobile panels (watchlist, entity detail). */
export const MOBILE_PANEL_MARGIN = 8;
