import React from 'react';
import { Box, Switch, Typography, Chip, Skeleton } from '@mui/material';
import { alpha, theme, semanticColors } from '@respondent/core';
import { CanvasLayerIcon } from '../../features/search/layerIcons';

interface LayerRowProps {
  name: string;
  type: string;
  source: string;
  enabled: boolean;
  count?: number;
  lastUpdate?: string;
  loading?: boolean;
  onToggle: (enabled: boolean) => void;
  icon?: React.ReactNode;
}

const LayerRow: React.FC<LayerRowProps> = ({
  name,
  type,
  source,
  enabled,
  count,
  lastUpdate,
  loading,
  onToggle,
  icon,
}) => {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        py: 1.5,
        px: 1,
        borderRadius: 1,
        transition: 'all 0.12s ease-out',
        '&:hover': {
          bgcolor: alpha(theme.palette.primary.main, 0.05),
        },
      }}
    >
      {/* Left: Icon, Name, Source */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flex: 1 }}>
        <Box sx={{ color: 'primary.main' }}>{icon || <CanvasLayerIcon layerType={type} />}</Box>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            variant="caption"
            sx={{
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
              display: 'block',
            }}
          >
            {name}
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: 'text.secondary',
              fontSize: '0.65rem',
              display: 'block',
            }}
          >
            {source} • {lastUpdate || 'never'}
          </Typography>
        </Box>
      </Box>

      {/* Right: Count Badge, Toggle */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {count !== undefined && count > 0 && (
          <Chip
            label={formatCount(count)}
            size="small"
            sx={{
              height: 20,
              fontSize: '0.65rem',
              fontWeight: 600,
              bgcolor: semanticColors.primary.alpha10,
              color: 'primary.main',
            }}
          />
        )}
        {loading ? (
          <Skeleton variant="rectangular" width={36} height={20} />
        ) : (
          <Switch
            checked={enabled}
            onChange={(_, checked) => onToggle(checked)}
            size="small"
            sx={{
              '& .MuiSwitch-switchBase.Mui-checked': {
                color: 'primary.main',
              },
              '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': {
                backgroundColor: 'primary.main',
              },
            }}
          />
        )}
      </Box>
    </Box>
  );
};

function formatCount(count: number): string {
  if (count >= 1000000) {
    return `${(count / 1000000).toFixed(1)}M`;
  }
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}K`;
  }
  return count.toString();
}

export default LayerRow;
