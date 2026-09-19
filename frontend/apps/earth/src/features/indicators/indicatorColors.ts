import { theme } from '@respondent/core';

/** Severity level → color mapping (matches spec Section 5.3). */
export const LEVEL_COLORS: Record<number, string> = {
  0: theme.palette.primary.main,
  1: '#ffff00',
  2: '#ff9500',
  3: '#ff6b35',
  4: theme.palette.secondary.main,
  5: '#ff0000',
};

export function levelColor(level: number): string {
  return LEVEL_COLORS[Math.min(Math.max(level, 0), 5)] ?? LEVEL_COLORS[0];
}
