export const iconSizes = {
  xs: 12,
  sm: 16,
  md: 20,
  lg: 24,
} as const;

export type IconSize = (typeof iconSizes)[keyof typeof iconSizes];
