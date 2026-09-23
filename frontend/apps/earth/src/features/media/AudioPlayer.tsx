/**
 * AudioPlayer — the persistent transport bar for the one audio session.
 *
 * It outlives the entity panel on purpose: a listener who closes the detail
 * view keeps hearing the station and keeps a visible way to stop it. Disabling
 * the station's layer, or pressing Stop, releases the element.
 */

import React, { useEffect } from 'react';
import { useStore } from 'zustand';
import { Box, IconButton, Slider, Tooltip, Typography } from '@mui/material';
import { Pause, Play, Radio, X } from 'lucide-react';
import {
  alpha,
  theme,
  DASHBOARD_TYPOGRAPHY,
  useResponsive,
  DESKTOP_BOTTOM_STACK_BASE,
  MOBILE_NAV_HEIGHT,
  MOBILE_NAV_GAP,
  MOBILE_STACK_GAP,
  MOBILE_PANEL_MARGIN,
} from '@respondent/core';
import { useUIStore } from '@/app/store';
import { useMediaContext } from './MediaProvider';

export const AudioPlayer: React.FC = () => {
  const { audio } = useMediaContext();
  const state = useStore(audio.store);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const watchlistBarHeight = useUIStore((s) => s.watchlistBarHeight);
  const { isMobile } = useResponsive();

  const stationLayer = state.station?.layerId;
  useEffect(() => {
    // Turning a layer off withdraws its media along with its entities.
    if (stationLayer && !enabledLayers.includes(stationLayer)) audio.stop();
  }, [stationLayer, enabledLayers, audio]);

  if (!state.station) return null;

  const { station, status, error, volume } = state;
  const playing = status === 'playing' || status === 'loading';

  // The player is a bottom-anchored HUD element, so it stacks with the others
  // instead of choosing its own offset. Anchoring it at the toolbar's own slot
  // put it on top of every toolbar button and swallowed their clicks.
  const stackBase = isMobile ? MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP : DESKTOP_BOTTOM_STACK_BASE;
  const bottomOffset =
    watchlistBarHeight > 0 ? stackBase + watchlistBarHeight + MOBILE_STACK_GAP : stackBase;

  return (
    <Box
      data-testid="media-audio-player"
      sx={{
        position: 'fixed',
        left: '50%',
        transform: 'translateX(-50%)',
        bottom: `calc(${bottomOffset}px + var(--sab, 0px))`,
        zIndex: 1200,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 1.25,
        py: 0.75,
        maxWidth: `min(520px, calc(100vw - ${MOBILE_PANEL_MARGIN * 2}px))`,
        borderRadius: 2,
        border: `1px solid ${alpha(theme.palette.primary.main, 0.25)}`,
        backgroundColor: alpha(theme.palette.background.paper, 0.94),
        backdropFilter: 'blur(6px)',
      }}
      role="region"
      aria-label="Audio player"
    >
      <Radio size={14} color={theme.palette.primary.main} />
      {/* The stream is always live, whatever time range the globe is showing. */}
      <Typography
        variant="caption"
        sx={{
          fontSize: 9,
          fontWeight: 700,
          letterSpacing: '0.08em',
          px: 0.5,
          borderRadius: 0.5,
          color: 'common.black',
          backgroundColor: theme.palette.primary.main,
        }}
      >
        LIVE
      </Typography>

      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Typography
          variant="caption"
          noWrap
          sx={{
            display: 'block',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            fontWeight: 600,
          }}
        >
          {station.name}
        </Typography>
        <Typography
          variant="caption"
          noWrap
          sx={{ display: 'block', fontSize: 10, color: error ? 'error.main' : 'text.secondary' }}
        >
          {error || station.attribution || (status === 'loading' ? 'Connecting…' : 'Live stream')}
        </Typography>
      </Box>

      <Tooltip title={playing ? 'Pause' : 'Play'} placement="top" arrow>
        <IconButton
          size="small"
          aria-label={playing ? 'Pause audio' : 'Play audio'}
          onClick={() => (playing ? audio.pause() : audio.resume())}
        >
          {playing ? <Pause size={14} /> : <Play size={14} />}
        </IconButton>
      </Tooltip>

      <Slider
        size="small"
        aria-label="Volume"
        value={volume}
        min={0}
        max={1}
        step={0.05}
        onChange={(_, value) => audio.setVolume(Array.isArray(value) ? value[0] : value)}
        sx={{ width: 72, display: { xs: 'none', sm: 'block' } }}
      />

      <Tooltip title="Stop" placement="top" arrow>
        <IconButton size="small" aria-label="Stop audio" onClick={() => audio.stop()}>
          <X size={14} />
        </IconButton>
      </Tooltip>
    </Box>
  );
};

export default AudioPlayer;
