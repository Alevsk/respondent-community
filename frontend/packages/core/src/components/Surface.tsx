import { Box, type SxProps } from '@mui/material';
import type { ReactNode } from 'react';
import { SURFACE } from '../theme/surfaces';

interface SurfaceProps {
  level: 'base' | 'raised' | 'overlay';
  children: ReactNode;
  sx?: SxProps;
}

export function Surface({ level, children, sx }: SurfaceProps) {
  const tokens = SURFACE[level];
  return (
    <Box
      sx={{
        bgcolor: tokens.bg,
        border: `1px solid ${tokens.border}`,
        boxShadow: tokens.shadow,
        borderRadius: '4px',
        overflow: 'hidden',
        ...sx,
      }}
    >
      {children}
    </Box>
  );
}
