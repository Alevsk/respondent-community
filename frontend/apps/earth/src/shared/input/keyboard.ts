/**
 * Centralized keyboard utilities for modifier key detection.
 *
 * Pure functions with no React dependency — usable from any handler.
 */

// The User-Agent Client Hints API (navigator.userAgentData) is not in the
// default Navigator lib types yet. Type it here rather than widening to
// `Record<string, unknown>` (which loses the property names we care about).
interface NavigatorUAData {
  platform?: string;
}
interface NavigatorWithUAData extends Navigator {
  userAgentData?: NavigatorUAData;
}

/** True when running on a Mac platform. */
export const isMac: boolean =
  typeof navigator !== 'undefined' &&
  (/Mac|iPod|iPhone|iPad/.test(navigator.platform) ||
    (navigator as NavigatorWithUAData).userAgentData?.platform === 'macOS');

/**
 * Returns true if the multi-select modifier key is pressed:
 * - Cmd (Meta) on Mac
 * - Ctrl on Windows/Linux
 */
export function isMultiSelectModifier(e: { ctrlKey: boolean; metaKey: boolean }): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}
