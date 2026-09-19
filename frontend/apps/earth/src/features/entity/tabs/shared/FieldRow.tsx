/**
 * FieldRow — Shared label-value row used across entity detail tabs.
 *
 * Adapts layout based on value length:
 *   - Short values: side-by-side (label left, value right)
 *   - Long values:  stacked (label above, value below with full width)
 *
 * All values route through SmartValue for consistent URL detection,
 * HTML tag stripping, and optional truncation.
 *
 * Two style variants:
 *   - `overview`: larger non-monospace labels (used by OverviewTab)
 *   - `detail`:   smaller monospace labels with minWidth (used by MetadataTab)
 */

import React from 'react';
import { Box, Typography } from '@mui/material';
import { BORDER, HOVER, FONT_FAMILY, DASHBOARD_TYPOGRAPHY } from '@respondent/core';
import SmartValue from './SmartValue';
import { useDetailVariant } from './DetailVariantContext';

/** Convert a string to sentence case (first letter upper, rest lower). */
function sentenceCase(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1).toLowerCase();
}

/** Default character threshold above which a field switches to stacked layout. */
const DEFAULT_STACK_THRESHOLD = 60;

interface FieldRowProps {
  label: string;
  value: string;
  /** Label style variant. Default: 'detail'. */
  variant?: 'overview' | 'detail';
  /** Override value color (e.g., for severity-coded values). */
  color?: string;
  /** Override value font weight. Defaults: 500 for overview, 500 for detail. */
  fontWeight?: number;
  /** Override value font size. */
  fontSize?: string;
  /** Character threshold for stacked layout. Default: 60. */
  stackThreshold?: number;
}

const LABEL_STYLES = {
  overview: {
    color: 'text.secondary',
    fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize,
    fontWeight: 500,
    letterSpacing: '0.04em',
    flexShrink: 0,
  },
  detail: {
    color: 'text.secondary',
    fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
    fontWeight: 500,
    letterSpacing: '0.03em',
    fontFamily: FONT_FAMILY,
    minWidth: 80,
    flexShrink: 0,
  },
} as const;

const FieldRow: React.FC<FieldRowProps> = ({
  label,
  value,
  variant = 'detail',
  color,
  fontWeight,
  fontSize,
  stackThreshold = DEFAULT_STACK_THRESHOLD,
}) => {
  const detailVariant = useDetailVariant();
  const isClean = detailVariant === 'clean';
  const isLong = value.length > stackThreshold;
  const resolvedFontSize =
    fontSize ??
    (variant === 'overview'
      ? DASHBOARD_TYPOGRAPHY.dashboardBase.fontSize
      : DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize);
  const resolvedFontWeight = fontWeight ?? 500;
  const displayLabel = isClean ? sentenceCase(label) : label;

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: isLong ? 'column' : 'row',
        justifyContent: isLong ? 'flex-start' : 'space-between',
        alignItems: isLong ? 'flex-start' : variant === 'overview' ? 'center' : 'flex-start',
        py: isClean ? 0.5 : 0.75,
        ...(isClean
          ? {
              borderRadius: '4px',
              px: 0.5,
              '&:hover': { bgcolor: HOVER.fieldRow },
            }
          : {
              borderBottom: `1px solid ${BORDER.subtle}`,
            }),
        gap: isLong ? 0.5 : 0,
      }}
    >
      <Typography variant="caption" sx={LABEL_STYLES[variant]}>
        {displayLabel}
      </Typography>
      <SmartValue
        value={value}
        fontSize={resolvedFontSize}
        color={color}
        fontWeight={resolvedFontWeight}
        textAlign={isLong ? 'left' : 'right'}
        truncateAt={isLong ? undefined : 0}
      />
    </Box>
  );
};

export default FieldRow;
