import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Box, Typography, IconButton } from '@mui/material';
import { CloseIcon, GridOnIcon, iconSizes } from '../../shared/icons';
import { useUIStore } from '@/app/store';
import { ASPECT_RATIOS, alpha, theme } from '@respondent/core';
import type { AspectRatioKey } from '@respondent/core';

const RATIO_KEYS: AspectRatioKey[] = ['9:16', '4:5', '1:1', '16:9', 'free'];
const AUTO_HIDE_MS = 3000;

const AspectRatioPicker: React.FC = () => {
  const recordingAspectRatio = useUIStore((s) => s.recordingAspectRatio);
  const setRecordingAspectRatio = useUIStore((s) => s.setRecordingAspectRatio);
  const toggleRecordingMode = useUIStore((s) => s.toggleRecordingMode);
  const recordingShowGrid = useUIStore((s) => s.recordingShowGrid);
  const setRecordingShowGrid = useUIStore((s) => s.setRecordingShowGrid);

  const [visible, setVisible] = useState(true);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  const resetTimer = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setVisible(true);
    timerRef.current = setTimeout(() => setVisible(false), AUTO_HIDE_MS);
  }, []);

  useEffect(() => {
    resetTimer();
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [resetTimer]);

  // Tap anywhere to show controls
  useEffect(() => {
    const handleTap = () => {
      if (!visible) {
        resetTimer();
      }
    };
    window.addEventListener('pointerdown', handleTap);
    return () => window.removeEventListener('pointerdown', handleTap);
  }, [visible, resetTimer]);

  const handleToggleGrid = useCallback(() => {
    setRecordingShowGrid(!recordingShowGrid);
  }, [recordingShowGrid, setRecordingShowGrid]);

  return (
    <Box
      data-testid="aspect-ratio-picker"
      onPointerDown={(e) => {
        e.stopPropagation();
        resetTimer();
      }}
      sx={{
        position: 'absolute',
        bottom: 'calc(var(--sab) + 16px)',
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 1200,
        display: 'flex',
        alignItems: 'center',
        gap: 0.5,
        bgcolor: 'rgba(5, 5, 5, 0.9)',
        backdropFilter: 'blur(12px)',
        px: 1,
        py: 0.5,
        borderRadius: 3,
        border: '1px solid rgba(255, 255, 255, 0.1)',
        opacity: visible ? 1 : 0,
        transition: 'opacity 0.3s ease',
        pointerEvents: visible ? 'auto' : 'none',
      }}
    >
      {RATIO_KEYS.map((key) => (
        <Box
          key={key}
          data-testid={`ratio-${key.replace(':', '-')}`}
          data-active={recordingAspectRatio === key}
          onClick={() => setRecordingAspectRatio(key)}
          sx={{
            px: 1.5,
            py: 0.75,
            borderRadius: 2,
            cursor: 'pointer',
            minWidth: 44,
            minHeight: 44,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            bgcolor:
              recordingAspectRatio === key
                ? alpha(theme.palette.primary.main, 0.15)
                : 'transparent',
            border:
              recordingAspectRatio === key
                ? `1px solid ${alpha(theme.palette.primary.main, 0.4)}`
                : '1px solid transparent',
            transition: 'all 0.15s ease',
            WebkitTapHighlightColor: 'transparent',
          }}
        >
          <Typography
            variant="caption"
            sx={{
              fontWeight: 700,
              fontSize: '0.65rem',
              letterSpacing: '0.05em',
              color: recordingAspectRatio === key ? 'primary.main' : 'text.secondary',
            }}
          >
            {ASPECT_RATIOS[key].label === 'Free' ? 'Free' : key}
          </Typography>
        </Box>
      ))}

      {/* Grid toggle */}
      <IconButton
        onClick={handleToggleGrid}
        size="small"
        aria-label="Toggle rule of thirds grid"
        sx={{
          minWidth: 44,
          minHeight: 44,
          color: recordingShowGrid ? 'primary.main' : 'text.secondary',
        }}
      >
        <GridOnIcon size={iconSizes.md} />
      </IconButton>

      {/* Exit */}
      <IconButton
        onClick={toggleRecordingMode}
        size="small"
        aria-label="Exit recording mode"
        sx={{
          minWidth: 44,
          minHeight: 44,
          color: 'error.main',
        }}
      >
        <CloseIcon size={iconSizes.md} />
      </IconButton>
    </Box>
  );
};

export default AspectRatioPicker;
