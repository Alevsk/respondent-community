import React from 'react';
import { Box, Typography, Button } from '@mui/material';
import { semanticColors } from '../theme';

const pulseKeyframes = {
  '@keyframes pulse': {
    '0%, 100%': { opacity: 1 },
    '50%': { opacity: 0.4 },
  },
};

export interface AppLoadingOverlayProps {
  label: string;
  error: boolean;
  onRetry: () => void;
}

const AppLoadingOverlay: React.FC<AppLoadingOverlayProps> = ({ label, error, onRetry }) => {
  return (
    <Box
      data-testid="app-loading-overlay"
      sx={{
        position: 'fixed',
        inset: 0,
        bgcolor: '#000000',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
        ...pulseKeyframes,
      }}
    >
      <Typography
        sx={{
          fontSize: '0.7rem',
          fontFamily: 'monospace',
          fontWeight: 700,
          letterSpacing: '0.15em',
          color: error ? 'secondary.main' : 'primary.main',
          mb: 1.5,
        }}
      >
        {error ? 'CONNECTION FAILED' : 'INITIALIZING'}
      </Typography>

      <Typography
        sx={{
          fontSize: '0.6rem',
          fontFamily: 'monospace',
          letterSpacing: '0.1em',
          color: error ? 'secondary.main' : 'primary.main',
          animation: error ? 'none' : 'pulse 1.5s ease-in-out infinite',
        }}
      >
        {label}
      </Typography>

      {error && (
        <Button
          onClick={onRetry}
          aria-label="retry"
          sx={{
            mt: 3,
            fontSize: '0.65rem',
            fontFamily: 'monospace',
            fontWeight: 700,
            letterSpacing: '0.15em',
            color: 'primary.main',
            border: '1px solid',
            borderColor: 'primary.main',
            textTransform: 'none',
            px: 3,
            py: 0.5,
            '&:hover': {
              bgcolor: semanticColors.primary.alpha10,
            },
          }}
        >
          RETRY
        </Button>
      )}
    </Box>
  );
};

export default AppLoadingOverlay;
