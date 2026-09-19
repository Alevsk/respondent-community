import React from 'react';
import { Box, Typography } from '@mui/material';
import { DASHBOARD_TYPOGRAPHY } from '@respondent/core';

const SEVERITY_COLORS: Record<string, { bg: string; text: string }> = {
  low: { bg: 'rgba(76, 175, 80, 0.15)', text: '#4caf50' },
  moderate: { bg: 'rgba(255, 152, 0, 0.15)', text: '#ff9800' },
  high: { bg: 'rgba(244, 67, 54, 0.15)', text: '#f44336' },
  critical: { bg: 'rgba(211, 47, 47, 0.25)', text: '#d32f2f' },
};

interface SeverityBadgeProps {
  severity: string;
}

const SeverityBadge: React.FC<SeverityBadgeProps> = ({ severity }) => {
  const normalized = severity.toLowerCase();
  const colors = SEVERITY_COLORS[normalized] ?? {
    bg: 'rgba(255,255,255,0.08)',
    text: 'text.secondary',
  };

  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        px: 1,
        py: 0.25,
        borderRadius: '4px',
        bgcolor: colors.bg,
      }}
    >
      <Typography
        component="span"
        sx={{
          fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
          fontWeight: 600,
          color: colors.text,
          textTransform: 'capitalize',
          lineHeight: 1,
        }}
      >
        {severity}
      </Typography>
    </Box>
  );
};

export default SeverityBadge;
