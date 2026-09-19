import React, { useCallback } from 'react';
import { Box, Typography } from '@mui/material';
import { alpha, theme } from '@respondent/core';
import {
  SearchIcon,
  LayersIcon,
  SettingsIcon,
  ExploreIcon,
  VideocamIcon,
  iconSizes,
} from '../icons';
import { useUIStore } from '@/app/store';
import type { MobileDrawerType } from '@/app/store';

interface NavItemProps {
  icon: React.ReactNode;
  label: string;
  active?: boolean;
  badge?: boolean;
  onClick: () => void;
}

const NavItem: React.FC<NavItemProps> = React.memo(({ icon, label, active, badge, onClick }) => (
  <Box
    onClick={onClick}
    role="button"
    aria-label={label}
    sx={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 0.25,
      minWidth: 44,
      minHeight: 44,
      flex: 1,
      cursor: 'pointer',
      color: active ? 'primary.main' : 'text.disabled',
      transition: 'color 0.2s ease',
      WebkitTapHighlightColor: 'transparent',
      '&:active': {
        color: 'primary.main',
      },
    }}
  >
    <Box sx={{ position: 'relative', fontSize: '1.25rem', display: 'flex' }}>
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
    <Typography
      variant="caption"
      sx={{
        fontSize: '0.5rem',
        fontWeight: 600,
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
      }}
    >
      {label}
    </Typography>
  </Box>
));

NavItem.displayName = 'NavItem';

const MobileBottomNav: React.FC = React.memo(() => {
  const activeMobileDrawer = useUIStore((s) => s.activeMobileDrawer);
  const openMobileDrawer = useUIStore((s) => s.openMobileDrawer);
  const closeMobileDrawer = useUIStore((s) => s.closeMobileDrawer);
  const toggleRecordingMode = useUIStore((s) => s.toggleRecordingMode);
  const recordingMode = useUIStore((s) => s.recordingMode);
  const searchOpen = useUIStore((s) => s.searchOpen);
  const toggleSearch = useUIStore((s) => s.toggleSearch);
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const hasWatchlistItems = watchlistEntities.some((e) => e.pinned);

  const handleNav = useCallback(
    (drawer: MobileDrawerType) => {
      if (activeMobileDrawer === drawer) {
        closeMobileDrawer();
      } else {
        openMobileDrawer(drawer);
      }
    },
    [activeMobileDrawer, closeMobileDrawer, openMobileDrawer],
  );

  const handleSearchClick = useCallback(() => toggleSearch(), [toggleSearch]);
  const handleLayersClick = useCallback(() => handleNav('layers'), [handleNav]);
  const handleSettingsClick = useCallback(() => handleNav('settings'), [handleNav]);
  const handleNavClick = useCallback(() => handleNav('nav'), [handleNav]);

  if (recordingMode) return null;

  return (
    <Box
      data-testid="mobile-bottom-nav"
      sx={{
        position: 'absolute',
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: 1000,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-around',
        bgcolor: 'rgba(5, 5, 5, 0.95)',
        backdropFilter: 'blur(16px)',
        borderTop: `1px solid ${alpha(theme.palette.primary.main, 0.15)}`,
        pt: 0.5,
        pb: 'calc(var(--sab) + 4px)',
        px: 1,
      }}
    >
      <NavItem
        icon={<SearchIcon size={iconSizes.md} />}
        label="Search"
        active={searchOpen}
        badge={hasWatchlistItems}
        onClick={handleSearchClick}
      />
      <NavItem
        icon={<LayersIcon size={iconSizes.md} />}
        label="Layers"
        active={activeMobileDrawer === 'layers'}
        onClick={handleLayersClick}
      />
      <NavItem
        icon={<SettingsIcon size={iconSizes.md} />}
        label="Settings"
        active={activeMobileDrawer === 'settings'}
        onClick={handleSettingsClick}
      />
      <NavItem
        icon={<ExploreIcon size={iconSizes.md} />}
        label="Nav"
        active={activeMobileDrawer === 'nav'}
        onClick={handleNavClick}
      />
      <NavItem
        icon={<VideocamIcon size={iconSizes.md} />}
        label="Record"
        onClick={toggleRecordingMode}
      />
    </Box>
  );
});

MobileBottomNav.displayName = 'MobileBottomNav';

export default MobileBottomNav;
