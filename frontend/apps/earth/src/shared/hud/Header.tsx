import React from 'react';
import { Box, Typography } from '@mui/material';
import { alpha, theme } from '@respondent/core';

const Header: React.FC = () => {
  return (
    <Box
      sx={{
        position: 'absolute',
        top: 16,
        left: 72,
        zIndex: 100,
      }}
    >
      <Typography
        variant="h6"
        sx={{
          fontWeight: 700,
          letterSpacing: '0.15em',
          color: 'primary.main',
          textShadow: `0 0 10px ${alpha(theme.palette.primary.main, 0.5)}`,
        }}
      >
        RESPONDENT
      </Typography>
      <Typography
        variant="caption"
        sx={{
          display: 'block',
          fontSize: '0.65rem',
          letterSpacing: '0.1em',
          color: 'text.secondary',
          textTransform: 'uppercase',
        }}
      >
        Geospatial Intelligence
      </Typography>
    </Box>
  );
};

export default Header;
