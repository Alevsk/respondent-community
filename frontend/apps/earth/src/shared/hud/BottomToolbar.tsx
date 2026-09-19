import React from 'react';
import { Box, IconButton, Tooltip } from '@mui/material';
import { alpha, theme, semanticColors } from '@respondent/core';
import {
  SearchIcon,
  LayersIcon,
  SettingsIcon,
  ExploreIcon,
  EditIcon,
  SaveIcon,
  iconSizes,
} from '../icons';
import { useUIStore } from '@/app/store';

interface ToolbarButtonProps {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  badge?: boolean;
  onClick: () => void;
  testId?: string;
}

const ToolbarButton: React.FC<ToolbarButtonProps> = ({
  icon,
  label,
  active,
  badge,
  onClick,
  testId,
}) => (
  <Tooltip title={label} placement="top" arrow>
    <IconButton
      onClick={onClick}
      data-testid={testId}
      sx={{
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 0.5,
        px: 2,
        py: 1,
        borderRadius: 2,
        color: active ? 'primary.main' : 'rgba(255, 255, 255, 0.6)',
        transition: 'all 0.2s ease',
        bgcolor: active ? semanticColors.primary.alpha10 : 'transparent',
        '&:hover': {
          color: 'primary.main',
          bgcolor: semanticColors.primary.alpha10,
          transform: 'translateY(-2px)',
        },
      }}
    >
      <Box sx={{ position: 'relative', fontSize: '1.25rem' }}>
        {icon}
        {badge && (
          <Box
            sx={{
              position: 'absolute',
              top: -2,
              right: -4,
              width: 8,
              height: 8,
              borderRadius: '50%',
              bgcolor: 'primary.main',
              boxShadow: `0 0 6px ${alpha(theme.palette.primary.main, 0.6)}`,
            }}
          />
        )}
      </Box>
      <Box
        sx={{
          fontSize: '0.55rem',
          fontWeight: 600,
          letterSpacing: '0.1em',
          textTransform: 'uppercase',
        }}
      >
        {label}
      </Box>
    </IconButton>
  </Tooltip>
);

const BottomToolbar: React.FC = () => {
  const cleanUI = useUIStore((s) => s.cleanUI);
  const activePreset = useUIStore((s) => s.activePreset);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const searchOpen = useUIStore((s) => s.searchOpen);
  const toggleSearch = useUIStore((s) => s.toggleSearch);
  const desktopPanelOpen = useUIStore((s) => s.desktopPanelOpen);
  const toggleDesktopPanel = useUIStore((s) => s.toggleDesktopPanel);

  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const hasActiveFilter = activePreset !== 'NORMAL';
  const hasActiveLayers = enabledLayers.length > 0;
  const hasWatchlistItems = watchlistEntities.some((e) => e.pinned);

  if (cleanUI) return null;

  return (
    <Box
      data-testid="bottom-toolbar"
      sx={{
        position: 'absolute',
        bottom: 16,
        left: '50%',
        transform: 'translateX(-50%)',
        display: 'flex',
        gap: 0.5,
        bgcolor: 'rgba(5, 5, 5, 0.92)',
        px: 1,
        py: 0.5,
        borderRadius: 3,
        backdropFilter: 'blur(16px)',
        border: `1px solid ${alpha(theme.palette.primary.main, 0.15)}`,
        boxShadow: `
          0 4px 24px rgba(0, 0, 0, 0.4),
          0 0 40px ${alpha(theme.palette.primary.main, 0.05)},
          inset 0 1px 0 rgba(255, 255, 255, 0.05)
        `,
        zIndex: 1000,
        '&::before': {
          content: '""',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: 1,
          background: `linear-gradient(90deg, transparent, ${semanticColors.primary.alpha30}, transparent)`,
        },
      }}
    >
      <ToolbarButton
        icon={<SearchIcon size={iconSizes.md} />}
        label="Search"
        active={searchOpen}
        badge={hasWatchlistItems}
        onClick={toggleSearch}
        testId="toolbar-btn-search"
      />
      <ToolbarButton
        icon={<LayersIcon size={iconSizes.md} />}
        label="Layers"
        active={desktopPanelOpen.layers}
        badge={hasActiveLayers}
        onClick={() => toggleDesktopPanel('layers')}
        testId="toolbar-btn-layers"
      />
      <ToolbarButton
        icon={<SettingsIcon size={iconSizes.md} />}
        label="Settings"
        active={desktopPanelOpen.settings}
        badge={hasActiveFilter}
        onClick={() => toggleDesktopPanel('settings')}
        testId="toolbar-btn-settings"
      />
      <ToolbarButton
        icon={<ExploreIcon size={iconSizes.md} />}
        label="Nav"
        active={desktopPanelOpen.nav}
        onClick={() => toggleDesktopPanel('nav')}
        testId="toolbar-btn-nav"
      />
      <ToolbarButton
        icon={<EditIcon size={iconSizes.md} />}
        label="Annotate"
        active={false}
        onClick={() => {}}
      />
      <ToolbarButton
        icon={<SaveIcon size={iconSizes.md} />}
        label="Save"
        active={false}
        onClick={() => {}}
      />
    </Box>
  );
};

export default BottomToolbar;
