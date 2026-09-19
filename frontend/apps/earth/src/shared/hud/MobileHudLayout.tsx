import React, { useMemo } from 'react';
import { Box, Typography } from '@mui/material';
import { StatusDotIcon } from '../icons';
import { useViewerStore } from '../../features/globe/store';
import { useUIStore } from '@/app/store';
import { useWebSocketStatus, alpha, theme } from '@respondent/core';
import NotificationBell from '../notifications/NotificationBell';

function formatCoord(lat: number, lon: number): string {
  const ns = lat >= 0 ? 'N' : 'S';
  const ew = lon >= 0 ? 'E' : 'W';
  return `${Math.abs(lat).toFixed(2)}${ns} ${Math.abs(lon).toFixed(2)}${ew}`;
}

function formatAlt(altitudeM: number): string {
  if (altitudeM >= 1_000_000) return `${(altitudeM / 1_000_000).toFixed(1)}M m`;
  if (altitudeM >= 1_000) return `${(altitudeM / 1_000).toFixed(1)} km`;
  return `${Math.round(altitudeM)} m`;
}

const MobileHudLayout: React.FC = React.memo(() => {
  const camera = useViewerStore((s) => s.camera);
  const wsStatus = useWebSocketStatus();
  const timeMode = useUIStore((s) => s.timeMode);
  const recordingMode = useUIStore((s) => s.recordingMode);

  const { statusColor, statusLabel } = useMemo(() => {
    const color =
      wsStatus === 'connected'
        ? 'success.main'
        : wsStatus === 'reconnecting'
          ? 'warning.main'
          : 'error.main';
    const label =
      wsStatus === 'connected'
        ? timeMode === 'live'
          ? 'LIVE'
          : 'RANGE'
        : wsStatus === 'reconnecting'
          ? 'RECONN'
          : 'OFFLINE';
    return { statusColor: color, statusLabel: label };
  }, [wsStatus, timeMode]);

  if (recordingMode) return null;

  return (
    <>
      {/* Top bar */}
      <Box
        data-testid="mobile-hud-top"
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 100,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          px: 2,
          pt: 'calc(var(--sat) + 8px)',
          pb: 1,
          background:
            'linear-gradient(180deg, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.4) 70%, transparent 100%)',
          pointerEvents: 'none',
        }}
      >
        {/* Logo */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, pointerEvents: 'auto' }}>
          <Typography
            variant="caption"
            sx={{
              fontWeight: 700,
              fontSize: '0.75rem',
              letterSpacing: '0.12em',
              color: 'primary.main',
              textShadow: `0 0 8px ${alpha(theme.palette.primary.main, 0.4)}`,
            }}
          >
            RESPONDENT
          </Typography>
        </Box>

        {/* Right side: notification bell + connection status */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, pointerEvents: 'auto' }}>
          <NotificationBell />

          {/* Connection status */}
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 0.5,
              bgcolor: 'rgba(0, 0, 0, 0.6)',
              px: 1,
              py: 0.5,
              borderRadius: 1,
            }}
          >
            <Box
              component="span"
              sx={{
                display: 'inline-flex',
                color: statusColor,
                animation: wsStatus === 'connected' ? 'pulse 1s infinite' : 'none',
                '@keyframes pulse': {
                  '0%, 100%': { opacity: 1 },
                  '50%': { opacity: 0.3 },
                },
              }}
            >
              <StatusDotIcon size={8} fill="currentColor" strokeWidth={0} />
            </Box>
            <Typography
              variant="caption"
              sx={{
                fontWeight: 700,
                fontSize: '0.6rem',
                letterSpacing: '0.08em',
                color: statusColor,
              }}
            >
              {statusLabel}
            </Typography>
          </Box>
        </Box>
      </Box>

      {/* Bottom telemetry strip */}
      <Box
        data-testid="mobile-hud-telemetry"
        sx={{
          position: 'absolute',
          bottom: 72,
          left: 0,
          right: 0,
          zIndex: 100,
          display: 'flex',
          justifyContent: 'center',
          gap: 2,
          px: 2,
          py: 0.5,
          pointerEvents: 'none',
        }}
      >
        <Box
          sx={{
            display: 'flex',
            gap: 2,
            bgcolor: 'rgba(0, 0, 0, 0.6)',
            px: 1.5,
            py: 0.5,
            borderRadius: 1,
          }}
        >
          <Typography
            variant="caption"
            sx={{
              color: 'primary.main',
              fontWeight: 600,
              fontSize: '0.6rem',
              fontFamily: 'monospace',
            }}
          >
            {camera ? formatCoord(camera.lat, camera.lon) : '---'}
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: 'text.secondary',
              fontWeight: 600,
              fontSize: '0.6rem',
              fontFamily: 'monospace',
            }}
          >
            ALT: {camera ? formatAlt(camera.altitude) : '---'}
          </Typography>
        </Box>
      </Box>
    </>
  );
});

MobileHudLayout.displayName = 'MobileHudLayout';

export default MobileHudLayout;
