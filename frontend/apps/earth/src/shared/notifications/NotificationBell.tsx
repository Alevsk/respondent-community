import React from 'react';
import { Box, IconButton, Badge } from '@mui/material';
import { NotificationsIcon } from '../icons';
import { useUIStore } from '@/app/store';
import { useResponsive, theme, semanticColors } from '@respondent/core';

const NotificationBell: React.FC = React.memo(() => {
  const { isMobile } = useResponsive();
  const cleanUI = useUIStore((s) => s.cleanUI);
  const recordingMode = useUIStore((s) => s.recordingMode);
  const unreadCount = useUIStore((s) => s.unreadCount);
  const toggleNotificationPanel = useUIStore((s) => s.toggleNotificationPanel);

  if (cleanUI || recordingMode) return null;

  const size = isMobile ? 40 : 32;

  return (
    <Box
      data-testid="notification-bell"
      onClick={toggleNotificationPanel}
      sx={
        isMobile
          ? { display: 'flex', alignItems: 'center' }
          : { position: 'absolute', top: 20, right: 140, zIndex: 200 }
      }
    >
      <IconButton
        sx={{
          width: size,
          height: size,
          color: 'primary.main',
          '&:hover': {
            bgcolor: semanticColors.primary.alpha10,
            filter: `drop-shadow(0 0 6px ${semanticColors.primary.alpha30})`,
          },
        }}
      >
        <Badge
          badgeContent={unreadCount}
          max={99}
          invisible={unreadCount === 0}
          sx={{
            '& .MuiBadge-badge': {
              bgcolor: theme.palette.secondary.main,
              color: '#fff',
              fontSize: '0.6rem',
              fontWeight: 700,
              fontFamily: 'monospace',
              minWidth: 16,
              height: 16,
            },
          }}
        >
          <NotificationsIcon size={isMobile ? 24 : 20} />
        </Badge>
      </IconButton>
    </Box>
  );
});

NotificationBell.displayName = 'NotificationBell';

export default NotificationBell;
