import React from 'react';
import { Box, Slider, Typography } from '@mui/material';

interface EffectSliderProps {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  defaultValue?: number;
  suffix?: string;
}

const EffectSlider: React.FC<EffectSliderProps> = ({
  label,
  value,
  onChange,
  min = 0,
  max = 100,
  step = 1,
  defaultValue,
  suffix = '%',
}) => {
  return (
    <Box sx={{ mb: 2 }}>
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          mb: 1,
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
          }}
        >
          {label}
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'primary.main',
            fontWeight: 700,
            fontFamily: 'monospace',
          }}
        >
          {value}
          {suffix}
        </Typography>
      </Box>
      <Box sx={{ position: 'relative', px: 1 }}>
        {/* Default marker line */}
        {defaultValue !== undefined && (
          <Box
            sx={{
              position: 'absolute',
              top: '50%',
              left: `calc(${((defaultValue - min) / (max - min)) * 100}% + 11px)`,
              transform: 'translateX(-50%)',
              width: 1,
              height: 12,
              bgcolor: 'rgba(255, 255, 255, 0.3)',
              zIndex: 0,
              pointerEvents: 'none',
            }}
          />
        )}
        <Slider
          value={value}
          onChange={(_, newValue) => onChange(newValue as number)}
          min={min}
          max={max}
          step={step}
          sx={{
            height: 4,
            '& .MuiSlider-thumb': {
              width: 16,
              height: 16,
              transition: 'transform 0.12s ease-out',
              '&:hover': {
                transform: 'scale(1.1)',
              },
            },
            '& .MuiSlider-track': {
              border: 'none',
            },
            '& .MuiSlider-rail': {
              opacity: 0.3,
            },
          }}
        />
      </Box>
    </Box>
  );
};

export default EffectSlider;
