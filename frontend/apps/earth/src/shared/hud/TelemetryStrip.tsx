import React from 'react';
import { Box, Typography, Divider } from '@mui/material';
import { semanticColors } from '@respondent/core';
import { useViewerStore } from '../../features/globe/store';

const TelemetryStrip: React.FC = () => {
  const camera = useViewerStore((state) => state.camera);

  return (
    <Box
      sx={{
        position: 'absolute',
        top: 0,
        left: 0,
        height: '100vh',
        width: 60,
        zIndex: 100,
        bgcolor: 'rgba(0, 0, 0, 0.7)',
        borderRight: `1px solid ${semanticColors.primary.alpha20}`,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        py: 2,
        fontSize: '0.6rem',
      }}
    >
      <Box sx={{ position: 'relative', height: '100%', width: '100%' }}>
        {/* Top tick marks */}
        {[...Array(10)].map((_, i) => (
          <Box
            key={`top-${i}`}
            sx={{
              position: 'absolute',
              top: `${80 + i * 8}px`,
              left: 0,
              width: i % 5 === 0 ? 10 : 6,
              height: 1,
              bgcolor: semanticColors.primary.alpha30,
            }}
          />
        ))}

        {/* Coordinate readouts */}
        <Box
          sx={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            textAlign: 'center',
            writingMode: 'vertical-rl',
            textOrientation: 'mixed',
          }}
        >
          <Typography
            variant="caption"
            sx={{
              color: 'primary.main',
              fontWeight: 600,
              fontFamily: 'monospace',
            }}
          >
            {camera ? `${Math.abs(camera.lat).toFixed(4)}°${camera.lat >= 0 ? 'N' : 'S'}` : ''}
          </Typography>
          <Divider
            orientation="horizontal"
            sx={{
              my: 1,
              borderColor: semanticColors.primary.alpha30,
            }}
          />
          <Typography
            variant="caption"
            sx={{
              color: 'primary.main',
              fontWeight: 600,
              fontFamily: 'monospace',
            }}
          >
            {camera ? `${Math.abs(camera.lon).toFixed(4)}°${camera.lon >= 0 ? 'E' : 'W'}` : ''}
          </Typography>
        </Box>

        {/* Bottom tick marks */}
        {[...Array(10)].map((_, i) => (
          <Box
            key={`bottom-${i}`}
            sx={{
              position: 'absolute',
              bottom: `${80 + i * 8}px`,
              left: 0,
              width: i % 5 === 0 ? 10 : 6,
              height: 1,
              bgcolor: semanticColors.primary.alpha30,
            }}
          />
        ))}

        {/* Altitude readout at bottom */}
        <Box
          sx={{
            position: 'absolute',
            bottom: 20,
            left: '50%',
            transform: 'translateX(-50%)',
            textAlign: 'center',
            writingMode: 'vertical-rl',
            textOrientation: 'mixed',
          }}
        >
          <Typography
            variant="caption"
            sx={{
              color: 'primary.main',
              fontWeight: 600,
              fontFamily: 'monospace',
              fontSize: '0.55rem',
            }}
          >
            {camera ? `${Math.round(camera.altitude / 1000)} km` : ''}
          </Typography>
          <Divider
            orientation="horizontal"
            sx={{
              my: 1,
              borderColor: semanticColors.primary.alpha30,
            }}
          />
          <Typography
            variant="caption"
            sx={{
              color: 'text.secondary',
              fontWeight: 600,
              fontFamily: 'monospace',
              fontSize: '0.55rem',
            }}
          >
            ALT
          </Typography>
        </Box>
      </Box>
    </Box>
  );
};

export default TelemetryStrip;
