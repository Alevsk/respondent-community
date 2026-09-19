import React from 'react';
import { Box, BoxProps } from '@mui/material';
import { alpha, theme, semanticColors } from '@respondent/core';
import { useUIStore, FilterPreset } from '@/app/store';

interface PresetButtonProps {
  preset: FilterPreset;
  label: string;
  icon: React.ReactNode;
  active: boolean;
  onClick: () => void;
}

const PresetButton: React.FC<PresetButtonProps> = ({ label, icon, active, onClick }) => {
  return (
    <Box
      onClick={onClick}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 0.75,
        px: 2.5,
        py: 1.5,
        borderRadius: 2,
        cursor: 'pointer',
        transition: 'all 0.2s cubic-bezier(0.4, 0, 0.2, 1)',
        border: '2px solid transparent',
        minWidth: 70,
        position: 'relative',
        overflow: 'hidden',
        '&::before': {
          content: '""',
          position: 'absolute',
          inset: 0,
          background: active
            ? `linear-gradient(180deg, ${semanticColors.primary.alpha20} 0%, ${alpha(theme.palette.primary.main, 0.05)} 100%)`
            : 'transparent',
          transition: 'background 0.2s ease',
        },
        '&:hover': {
          bgcolor: alpha(theme.palette.primary.main, 0.08),
          transform: 'translateY(-2px)',
          borderColor: semanticColors.primary.alpha30,
        },
        '&:active': {
          transform: 'translateY(0)',
        },
        ...(active && {
          borderColor: 'primary.main',
          bgcolor: alpha(theme.palette.primary.main, 0.12),
          boxShadow: `
            0 0 20px ${alpha(theme.palette.primary.main, 0.25)},
            inset 0 1px 0 rgba(255, 255, 255, 0.1)
          `,
          transform: 'scale(1.05)',
          '&::after': {
            content: '""',
            position: 'absolute',
            bottom: 0,
            left: '50%',
            transform: 'translateX(-50%)',
            width: '60%',
            height: 2,
            bgcolor: 'primary.main',
            borderRadius: 1,
            boxShadow: `0 0 8px ${alpha(theme.palette.primary.main, 0.6)}`,
          },
        }),
      }}
    >
      <Box
        sx={{
          color: active ? 'primary.main' : 'rgba(255, 255, 255, 0.6)',
          fontSize: '1.4rem',
          transition: 'all 0.2s ease',
          filter: active
            ? `drop-shadow(0 0 6px ${alpha(theme.palette.primary.main, 0.5)})`
            : 'none',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {icon}
      </Box>
      <Box
        sx={{
          fontSize: '0.6rem',
          fontWeight: 700,
          letterSpacing: '0.1em',
          textTransform: 'uppercase',
          color: active ? 'primary.main' : 'rgba(255, 255, 255, 0.5)',
          transition: 'color 0.2s ease',
          textShadow: active ? `0 0 10px ${alpha(theme.palette.primary.main, 0.5)}` : 'none',
          position: 'relative',
          zIndex: 1,
        }}
      >
        {label}
      </Box>
    </Box>
  );
};

type StylePresetBarProps = BoxProps;

const StylePresetBar: React.FC<StylePresetBarProps> = (props) => {
  const activePreset = useUIStore((s) => s.activePreset);
  const setActivePreset = useUIStore((s) => s.setActivePreset);

  const presets: { preset: FilterPreset; label: string; icon: string }[] = [
    { preset: 'NORMAL', label: 'Normal', icon: '◉' },
    { preset: 'CRT', label: 'CRT', icon: '|RF|' },
    { preset: 'NVG', label: 'NVG', icon: '◐' },
    { preset: 'FLIR', label: 'FLIR', icon: '▦' },
  ];

  return (
    <Box
      sx={{
        position: 'absolute',
        bottom: 80,
        left: '50%',
        transform: 'translateX(-50%)',
        display: 'flex',
        gap: 1.5,
        bgcolor: 'rgba(5, 5, 5, 0.92)',
        p: 1.25,
        borderRadius: 3,
        backdropFilter: 'blur(16px)',
        border: `1px solid ${alpha(theme.palette.primary.main, 0.15)}`,
        boxShadow: `
          0 4px 24px rgba(0, 0, 0, 0.4),
          0 0 40px ${alpha(theme.palette.primary.main, 0.05)},
          inset 0 1px 0 rgba(255, 255, 255, 0.05)
        `,
        '&::before': {
          content: '""',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: 1,
          background: `linear-gradient(90deg, transparent, ${semanticColors.primary.alpha30}, transparent)`,
        },
        ...props.sx,
      }}
      {...props}
    >
      {presets.map(({ preset, label, icon }) => (
        <PresetButton
          key={preset}
          preset={preset}
          label={label}
          icon={icon}
          active={activePreset === preset}
          onClick={() => setActivePreset(preset)}
        />
      ))}
    </Box>
  );
};

export default StylePresetBar;
