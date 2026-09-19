import React, { useMemo, useState, useEffect } from 'react';
import { Box } from '@mui/material';
import { semanticColors, alpha, theme } from '@respondent/core';
import type { AspectRatioKey } from '@respondent/core';
import { calculateFrame } from './aspectRatioUtils';

interface AspectRatioOverlayProps {
  ratio: AspectRatioKey;
  showGrid?: boolean;
}

const AspectRatioOverlay: React.FC<AspectRatioOverlayProps> = ({ ratio, showGrid = false }) => {
  // Resize counter to trigger recalculation on viewport changes
  const [tick, setTick] = useState(0);

  useEffect(() => {
    const handleResize = () => setTick((t) => t + 1);
    window.addEventListener('resize', handleResize);
    window.addEventListener('orientationchange', handleResize);
    return () => {
      window.removeEventListener('resize', handleResize);
      window.removeEventListener('orientationchange', handleResize);
    };
  }, []);

  const currentFrame = useMemo(
    () => calculateFrame(ratio, window.innerWidth, window.innerHeight),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [ratio, tick],
  );

  if (ratio === 'free') return null;

  const { frameWidth, frameHeight, offsetX, offsetY } = currentFrame;

  return (
    <Box
      data-testid="aspect-ratio-overlay"
      sx={{
        position: 'absolute',
        inset: 0,
        zIndex: 1100,
        pointerEvents: 'none',
      }}
    >
      {/* Top letterbox */}
      {offsetY > 0 && (
        <Box
          sx={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            height: offsetY,
            bgcolor: 'rgba(0, 0, 0, 0.75)',
          }}
        />
      )}
      {/* Bottom letterbox */}
      {offsetY > 0 && (
        <Box
          sx={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            height: offsetY,
            bgcolor: 'rgba(0, 0, 0, 0.75)',
          }}
        />
      )}
      {/* Left pillarbox */}
      {offsetX > 0 && (
        <Box
          sx={{
            position: 'absolute',
            top: offsetY,
            left: 0,
            width: offsetX,
            height: frameHeight,
            bgcolor: 'rgba(0, 0, 0, 0.75)',
          }}
        />
      )}
      {/* Right pillarbox */}
      {offsetX > 0 && (
        <Box
          sx={{
            position: 'absolute',
            top: offsetY,
            right: 0,
            width: offsetX,
            height: frameHeight,
            bgcolor: 'rgba(0, 0, 0, 0.75)',
          }}
        />
      )}

      {/* Frame border */}
      <Box
        sx={{
          position: 'absolute',
          top: offsetY,
          left: offsetX,
          width: frameWidth,
          height: frameHeight,
          border: `1px solid ${semanticColors.primary.alpha30}`,
        }}
      />

      {/* Rule of thirds grid */}
      {showGrid && (
        <Box
          data-testid="aspect-ratio-grid"
          sx={{
            position: 'absolute',
            top: offsetY,
            left: offsetX,
            width: frameWidth,
            height: frameHeight,
          }}
        >
          {/* Vertical lines */}
          <Box
            sx={{
              position: 'absolute',
              top: 0,
              left: '33.33%',
              width: 1,
              height: '100%',
              bgcolor: alpha(theme.palette.primary.main, 0.15),
            }}
          />
          <Box
            sx={{
              position: 'absolute',
              top: 0,
              left: '66.66%',
              width: 1,
              height: '100%',
              bgcolor: alpha(theme.palette.primary.main, 0.15),
            }}
          />
          {/* Horizontal lines */}
          <Box
            sx={{
              position: 'absolute',
              top: '33.33%',
              left: 0,
              width: '100%',
              height: 1,
              bgcolor: alpha(theme.palette.primary.main, 0.15),
            }}
          />
          <Box
            sx={{
              position: 'absolute',
              top: '66.66%',
              left: 0,
              width: '100%',
              height: 1,
              bgcolor: alpha(theme.palette.primary.main, 0.15),
            }}
          />
        </Box>
      )}
    </Box>
  );
};

export default AspectRatioOverlay;
