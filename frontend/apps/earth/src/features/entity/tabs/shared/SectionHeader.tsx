/**
 * SectionHeader — Shared section header component for entity detail tabs.
 *
 * Renders an optional icon + title row with a subtle green-tinted bottom border.
 * Used across OverviewTab, ObservationDataView, AIAnalysisTab, and MetadataTab.
 */

import React from 'react';
import { Box, Typography } from '@mui/material';
import { alpha, theme, DASHBOARD_TYPOGRAPHY } from '@respondent/core';

interface SectionHeaderProps {
  icon?: React.ReactNode;
  title: string;
}

const SectionHeader: React.FC<SectionHeaderProps> = ({ icon, title }) => (
  <Box
    sx={{
      display: 'flex',
      alignItems: 'center',
      gap: 0.75,
      mt: 1.5,
      mb: 0.75,
      borderBottom: `1px solid ${alpha(theme.palette.primary.main, 0.1)}`,
      pb: 0.5,
    }}
  >
    {icon && (
      <Box
        sx={{
          color: theme.palette.primary.main,
          display: 'flex',
          alignItems: 'center',
          fontSize: 14,
        }}
      >
        {icon}
      </Box>
    )}
    <Typography
      variant="caption"
      sx={{
        fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
        fontWeight: 600,
        color: theme.palette.primary.main,
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
      }}
    >
      {title}
    </Typography>
  </Box>
);

export default SectionHeader;
