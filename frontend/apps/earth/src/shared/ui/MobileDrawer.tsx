import React from 'react';
import { Box, SwipeableDrawer, Typography, IconButton } from '@mui/material';
import { alpha, theme } from '@respondent/core';
import { CloseIcon, iconSizes } from '../icons';

export interface MobileDrawerProps {
  open: boolean;
  onClose: () => void;
  onOpen: () => void;
  title: string;
  children: React.ReactNode;
  heightPercent?: number;
  /**
   * When true, tapping the dim backdrop does NOT close the drawer (it still
   * closes via the close button, swipe-down, Escape, or programmatically).
   *
   * Required for drawers opened by a Cesium CANVAS gesture: the browser's
   * touch→mouse compatibility "ghost click" fired ~300ms after the tap lands on
   * the Modal backdrop and would otherwise close the just-opened drawer. Leave
   * false (default) for drawers opened by an on-screen button, where the open
   * happens on the click itself and there is no stray trailing event.
   */
  disableBackdropClose?: boolean;
}

const MobileDrawer: React.FC<MobileDrawerProps> = ({
  open,
  onClose,
  onOpen,
  title,
  children,
  heightPercent = 60,
  disableBackdropClose = false,
}) => {
  const slug = title.toLowerCase().replace(/\s+/g, '-');
  // SwipeableDrawer types onClose as a single-arg handler but passes the close
  // `reason` as the 2nd runtime arg (via the underlying Modal). Declaring reason
  // optional keeps this assignable while letting us ignore backdrop clicks.
  const handleClose = (
    _event: React.SyntheticEvent,
    reason?: 'backdropClick' | 'escapeKeyDown',
  ) => {
    if (disableBackdropClose && reason === 'backdropClick') return;
    onClose();
  };
  return (
    <SwipeableDrawer
      anchor="bottom"
      open={open}
      onClose={handleClose}
      onOpen={onOpen}
      disableSwipeToOpen
      swipeAreaWidth={0}
      data-testid={`mobile-drawer-${slug}`}
      ModalProps={{ keepMounted: true }}
      PaperProps={{
        sx: {
          height: `${heightPercent}vh`,
          maxHeight: '85vh',
          bgcolor: 'rgba(5, 5, 5, 0.95)',
          backdropFilter: 'blur(16px)',
          borderTopLeftRadius: 16,
          borderTopRightRadius: 16,
          border: `1px solid ${alpha(theme.palette.primary.main, 0.15)}`,
          borderBottom: 'none',
          overflow: 'hidden',
          paddingBottom: 'var(--sab)',
        },
      }}
      slotProps={{
        backdrop: {
          sx: { backgroundColor: 'rgba(0, 0, 0, 0.4)' },
        },
      }}
    >
      {/* Drag handle */}
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          pt: 1,
          pb: 0.5,
        }}
      >
        <Box
          sx={{
            width: 40,
            height: 4,
            borderRadius: 2,
            bgcolor: 'rgba(255, 255, 255, 0.3)',
          }}
        />
      </Box>

      {/* Header */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 2,
          pb: 1.5,
          borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 700,
            fontSize: '0.75rem',
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'primary.main',
          }}
        >
          {title}
        </Typography>
        <IconButton
          onClick={onClose}
          size="small"
          aria-label={`Close ${title}`}
          data-testid={`mobile-drawer-close-${slug}`}
          sx={{ color: 'text.secondary', minWidth: 44, minHeight: 44 }}
        >
          <CloseIcon size={iconSizes.md} />
        </IconButton>
      </Box>

      {/* Scrollable content */}
      <Box
        sx={{
          flex: 1,
          overflow: 'auto',
          px: 2,
          py: 1.5,
        }}
      >
        {children}
      </Box>
    </SwipeableDrawer>
  );
};

export default MobileDrawer;
