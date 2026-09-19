import React from 'react';
import { Paper, Box, IconButton, Collapse, SxProps, Theme } from '@mui/material';
import { alpha, theme, semanticColors } from '@respondent/core';
import { ChevronLeftIcon, ChevronRightIcon, iconSizes } from '../icons';

interface PanelShellProps {
  children: React.ReactNode;
  title?: string;
  icon?: React.ReactNode;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
  sx?: SxProps<Theme>;
}

const PanelShell: React.FC<PanelShellProps> = ({
  children,
  title,
  icon,
  collapsed = false,
  onToggleCollapse,
  sx,
}) => {
  return (
    <Paper
      elevation={3}
      sx={{
        position: 'absolute',
        borderRadius: 2,
        overflow: 'hidden',
        transition: 'all 0.2s ease-out',
        border: `1px solid ${semanticColors.primary.alpha20}`,
        '&:hover': {
          border: `1px solid ${alpha(theme.palette.primary.main, 0.4)}`,
          boxShadow: `0 0 20px ${semanticColors.primary.alpha10}`,
        },
        ...sx,
      }}
    >
      {/* Header */}
      {title && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            px: 2,
            py: 1,
            borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
            bgcolor: alpha(theme.palette.primary.main, 0.05),
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            {icon}
            <Box
              sx={{
                fontSize: '0.75rem',
                fontWeight: 700,
                letterSpacing: '0.05em',
                textTransform: 'uppercase',
                color: 'primary.main',
              }}
            >
              {title}
            </Box>
          </Box>
          {onToggleCollapse && (
            <IconButton
              size="small"
              onClick={onToggleCollapse}
              sx={{
                color: 'text.secondary',
                '&:hover': { color: 'primary.main' },
              }}
            >
              {collapsed ? (
                <ChevronRightIcon size={iconSizes.md} />
              ) : (
                <ChevronLeftIcon size={iconSizes.md} />
              )}
            </IconButton>
          )}
        </Box>
      )}

      {/* Content */}
      <Collapse in={!collapsed} timeout={200}>
        <Box sx={{ p: 2 }}>{children}</Box>
      </Collapse>
    </Paper>
  );
};

export default PanelShell;
