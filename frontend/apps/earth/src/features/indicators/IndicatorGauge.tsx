import React from 'react';
import { Box, Typography } from '@mui/material';
import type { IndicatorValue } from '@/app/store';
import { levelColor } from './indicatorColors';
import { alpha, theme } from '@respondent/core';

/** Format a numeric indicator value for display. */
export function formatIndicatorValue(v: IndicatorValue): string {
  const fmt = v.format ?? 'scale';
  if (fmt === 'scale') {
    return v.unit ? `${v.value} ${v.unit}` : v.value;
  }
  const num = parseFloat(v.value);
  if (isNaN(num)) return v.value;
  const formatted = num.toLocaleString('en-US', {
    minimumFractionDigits: v.precision ?? 0,
    maximumFractionDigits: v.precision ?? 0,
  });
  return `${v.prefix ?? ''}${formatted}`;
}

/** Color for change % indicator — green positive, red negative, neutral zero. */
export function changeColor(changePct: number): string {
  if (changePct === 0) return 'rgba(255,255,255,0.4)';
  const intensity = Math.min(Math.abs(changePct) / 5, 1);
  const opacity = 0.4 + intensity * 0.6;
  return changePct > 0
    ? alpha(theme.palette.primary.main, opacity)
    : alpha(theme.palette.secondary.main, opacity);
}

interface IndicatorGaugeProps {
  value: IndicatorValue;
}

const IndicatorGauge: React.FC<IndicatorGaugeProps> = ({ value }) => {
  const color = levelColor(value.level);
  const fmt = value.format ?? 'scale';
  const isExtended = fmt !== 'scale';

  if (isExtended) {
    const displayValue = formatIndicatorValue(value);
    const pct = value.changePct ?? 0;
    const pctStr =
      pct > 0 ? `+${pct.toFixed(value.precision ?? 2)}%` : `${pct.toFixed(value.precision ?? 2)}%`;
    const pctColor = changeColor(pct);
    const shown = fmt === 'percent' && value.unit ? `${displayValue}${value.unit}` : displayValue;

    return (
      <Box
        data-testid={`indicator-gauge-${value.key}`}
        sx={{
          display: 'flex',
          flexDirection: 'column',
          minWidth: 52,
          px: 0.75,
          py: 0.5,
          borderRadius: '4px',
          border: '1px solid',
          borderColor: `${color}33`,
          background: `${color}0d`,
        }}
      >
        <Typography
          sx={{
            fontSize: '0.5rem',
            color: 'text.secondary',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
            lineHeight: 1.3,
            mb: 0.25,
          }}
        >
          {value.label}
        </Typography>
        <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.5 }}>
          <Typography
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.85rem',
              fontWeight: 700,
              color,
              lineHeight: 1.2,
            }}
          >
            {shown}
          </Typography>
          {pct !== 0 && (
            <Typography
              sx={{
                fontFamily: 'monospace',
                fontSize: '0.55rem',
                fontWeight: 600,
                color: pctColor,
                lineHeight: 1.2,
              }}
            >
              {pctStr}
            </Typography>
          )}
        </Box>
      </Box>
    );
  }

  // Scale format (default) — original rendering
  return (
    <Box
      data-testid={`indicator-gauge-${value.key}`}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        minWidth: 52,
        px: 0.75,
        py: 0.5,
        borderRadius: '4px',
        border: '1px solid',
        borderColor: `${color}33`,
        background: `${color}0d`,
      }}
    >
      <Typography
        sx={{
          fontFamily: 'monospace',
          fontSize: '0.85rem',
          fontWeight: 700,
          color,
          lineHeight: 1.2,
        }}
      >
        {value.value}
        {value.unit ? ` ${value.unit}` : ''}
      </Typography>
      <Typography
        sx={{
          fontSize: '0.5rem',
          color: 'text.secondary',
          letterSpacing: '0.05em',
          textTransform: 'uppercase',
          lineHeight: 1.3,
          textAlign: 'center',
          mt: 0.25,
        }}
      >
        {value.label}
      </Typography>
    </Box>
  );
};

export default IndicatorGauge;
