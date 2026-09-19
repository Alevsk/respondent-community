import React, { useState, useEffect, useCallback } from 'react';
import { Box, Typography, Popover, Divider } from '@mui/material';
import { StatusDotIcon, ArrowDropDownIcon } from '../icons';
import { useWebSocketStatus, type WSConnectionStatus } from '@respondent/core';
import { useUIStore, TimePreset } from '@/app/store';
import CustomRangePicker from './CustomRangePicker';

/** Derive the most permissive history limits from all known layers.
 *  Uses every loaded layer (not just enabled ones) because the time range
 *  is global — the user may enable a historical layer after picking a range. */
function useHistoryLimits(): { maxLookbackHours: number; maxRangeSpanHours: number } {
  const layers = useUIStore((s) => s.layers);

  let maxLookback = 48; // server default
  let maxSpan = 24; // server default

  // layers is keyed by both id and type; use a Set to avoid double-counting.
  const seen = new Set<string>();
  for (const [, layer] of Object.entries(layers)) {
    if (!layer?.historyConfig || seen.has(layer.id)) continue;
    seen.add(layer.id);
    const hc = layer.historyConfig;
    if (hc.maxLookbackHours > maxLookback) maxLookback = hc.maxLookbackHours;
    if (hc.maxRangeSpanHours > maxSpan) maxSpan = hc.maxRangeSpanHours;
  }

  return { maxLookbackHours: maxLookback, maxRangeSpanHours: maxSpan };
}

const STATUS_CONFIG: Record<WSConnectionStatus, { label: string; color: string; pulse: boolean }> =
  {
    connected: { label: 'LIVE', color: 'success.main', pulse: true },
    disconnected: { label: 'OFFLINE', color: 'error.main', pulse: false },
    reconnecting: { label: 'RECONNECTING', color: 'warning.main', pulse: true },
    failed: { label: 'OFFLINE', color: 'error.main', pulse: false },
  };

const PRESET_LABELS: Record<string, string> = {
  live: 'LIVE',
  '1h': 'LAST 1H',
  '8h': 'LAST 8H',
  '24h': 'LAST 24H',
  custom: 'RANGE',
};

function formatUTC(): string {
  return new Date().toISOString().replace('T', ' ').slice(0, 19) + 'Z';
}

function formatRangeDisplay(from: string | null, to: string | null): string {
  if (!from) return '';
  const fmtFrom = from.replace('T', ' ').slice(0, 16) + 'Z';
  const fmtTo = to ? to.replace('T', ' ').slice(0, 16) + 'Z' : 'now';
  return `${fmtFrom} → ${fmtTo}`;
}

const menuItemSx = {
  px: 2,
  py: 1,
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  gap: 1,
  '&:hover': {
    bgcolor: 'rgba(255,255,255,0.05)',
  },
  borderRadius: 0.5,
} as const;

const RecBlock: React.FC = () => {
  const wsStatus = useWebSocketStatus();
  const timeMode = useUIStore((s) => s.timeMode);
  const timePreset = useUIStore((s) => s.timePreset);
  const timeFrom = useUIStore((s) => s.timeFrom);
  const timeTo = useUIStore((s) => s.timeTo);
  const setTimePreset = useUIStore((s) => s.setTimePreset);
  const setCustomTimeRange = useUIStore((s) => s.setCustomTimeRange);
  const returnToLive = useUIStore((s) => s.returnToLive);
  const { maxLookbackHours, maxRangeSpanHours } = useHistoryLimits();

  // Determine if the current range is "historical" (beyond the default 48h window)
  const isHistorical =
    timeFrom != null && Date.now() - new Date(timeFrom).getTime() > 48 * 3600_000;

  const [utc, setUtc] = useState(formatUTC);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [showCustom, setShowCustom] = useState(false);
  const [customError, setCustomError] = useState<string | null>(null);

  useEffect(() => {
    const id = setInterval(() => setUtc(formatUTC()), 1000);
    return () => clearInterval(id);
  }, []);

  const handleOpen = useCallback((e: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(e.currentTarget);
  }, []);

  const handleClose = useCallback(() => {
    setAnchorEl(null);
    setShowCustom(false);
  }, []);

  const handlePreset = useCallback(
    (preset: TimePreset | 'live') => {
      if (preset === 'live') {
        returnToLive();
      } else if (preset === 'custom') {
        setShowCustom(true);
        return; // Don't close menu
      } else {
        setTimePreset(preset);
      }
      handleClose();
    },
    [returnToLive, setTimePreset, handleClose],
  );

  const handleApplyCustom = useCallback(
    (from: string, to: string) => {
      setCustomError(null);
      setCustomTimeRange({ from, to });
      handleClose();
    },
    [setCustomTimeRange, handleClose],
  );

  const handleCustomError = useCallback((error: string) => {
    setCustomError(error);
  }, []);

  // Derive display state
  const isOffline = wsStatus !== 'connected';
  const isFrozen = timeMode === 'range' && timeTo !== null;
  const isSliding = timeMode === 'range' && timeTo === null;

  // Determine dot color and label
  let dotColor: string;
  let dotPulse: boolean;
  let label: string;

  if (isOffline) {
    const s = STATUS_CONFIG[wsStatus];
    dotColor = s.color;
    dotPulse = s.pulse;
    label = s.label;
  } else if (isFrozen && isHistorical) {
    dotColor = 'info.main';
    dotPulse = false;
    label = 'HISTORICAL';
  } else if (isFrozen) {
    dotColor = 'warning.main';
    dotPulse = false;
    label = PRESET_LABELS[timePreset ?? 'custom'];
  } else if (isSliding) {
    dotColor = 'success.main';
    dotPulse = true;
    label = PRESET_LABELS[timePreset ?? '1h'];
  } else {
    dotColor = 'success.main';
    dotPulse = true;
    label = 'LIVE';
  }

  const open = Boolean(anchorEl);

  return (
    <Box
      sx={{
        position: 'absolute',
        top: 16,
        right: 16,
        zIndex: 100,
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
        alignItems: 'flex-end',
      }}
    >
      {/* Connection + time range indicator — clickable */}
      <Box
        data-testid="time-range-trigger"
        onClick={handleOpen}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.5,
          bgcolor: 'rgba(0, 0, 0, 0.7)',
          px: 1.5,
          py: 0.5,
          borderRadius: 1,
          cursor: 'pointer',
          userSelect: 'none',
          '&:hover': { bgcolor: 'rgba(0, 0, 0, 0.85)' },
        }}
      >
        <Box
          component="span"
          sx={{
            display: 'inline-flex',
            color: dotColor,
            animation: dotPulse ? 'pulse 1s infinite' : 'none',
            '@keyframes pulse': {
              '0%, 100%': { opacity: 1 },
              '50%': { opacity: 0.3 },
            },
          }}
        >
          <StatusDotIcon size={12} fill="currentColor" strokeWidth={0} />
        </Box>
        <Typography
          variant="caption"
          data-testid="ws-connection-status"
          sx={{
            fontWeight: 700,
            fontSize: '0.7rem',
            letterSpacing: '0.1em',
            color: dotColor,
          }}
        >
          {label}
        </Typography>
        <Box component="span" sx={{ display: 'inline-flex', color: 'text.secondary', ml: -0.5 }}>
          <ArrowDropDownIcon size={16} />
        </Box>
      </Box>

      {/* UTC timestamp or range display */}
      <Box
        sx={{
          bgcolor: 'rgba(0, 0, 0, 0.7)',
          px: 1.5,
          py: 0.5,
          borderRadius: 1,
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            fontSize: '0.65rem',
            letterSpacing: '0.05em',
            color: isFrozen ? 'warning.main' : 'primary.main',
            fontFamily: 'monospace',
          }}
        >
          {isFrozen ? formatRangeDisplay(timeFrom, timeTo) : utc}
        </Typography>
      </Box>

      {/* Time range dropdown menu */}
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{
          paper: {
            sx: {
              bgcolor: 'rgba(5, 5, 5, 0.95)',
              backdropFilter: 'blur(16px)',
              border: '1px solid rgba(255,255,255,0.08)',
              borderRadius: 1.5,
              py: 1,
              minWidth: 200,
            },
          },
        }}
      >
        {/* LIVE option */}
        <Box
          data-testid="time-preset-live"
          data-active={timeMode === 'live'}
          onClick={() => handlePreset('live')}
          sx={{
            ...menuItemSx,
            color: timeMode === 'live' ? 'success.main' : 'text.primary',
          }}
        >
          <Box
            component="span"
            sx={{
              display: 'inline-flex',
              color: timeMode === 'live' ? 'success.main' : 'text.disabled',
            }}
          >
            <StatusDotIcon size={10} fill="currentColor" strokeWidth={0} />
          </Box>
          <Typography variant="caption" sx={{ fontWeight: 600, fontSize: '0.75rem' }}>
            LIVE
          </Typography>
        </Box>

        <Divider sx={{ my: 0.5, borderColor: 'rgba(255,255,255,0.08)' }} />

        {/* Preset options */}
        {(['1h', '8h', '24h'] as TimePreset[]).map((preset) => (
          <Box
            key={preset}
            data-testid={`time-preset-${preset}`}
            data-active={timePreset === preset}
            onClick={() => handlePreset(preset)}
            sx={{
              ...menuItemSx,
              color: timePreset === preset ? 'primary.main' : 'text.primary',
            }}
          >
            <Box
              component="span"
              sx={{
                display: 'inline-flex',
                color: timePreset === preset ? 'primary.main' : 'text.disabled',
              }}
            >
              <StatusDotIcon size={10} fill="currentColor" strokeWidth={0} />
            </Box>
            <Typography variant="caption" sx={{ fontWeight: 600, fontSize: '0.75rem' }}>
              {PRESET_LABELS[preset]}
            </Typography>
          </Box>
        ))}

        <Divider sx={{ my: 0.5, borderColor: 'rgba(255,255,255,0.08)' }} />

        {/* Custom range option */}
        <Box
          data-testid="time-preset-custom"
          data-active={timePreset === 'custom'}
          onClick={() => handlePreset('custom')}
          sx={{
            ...menuItemSx,
            color: timePreset === 'custom' ? 'warning.main' : 'text.primary',
          }}
        >
          <Box
            component="span"
            sx={{
              display: 'inline-flex',
              color: timePreset === 'custom' ? 'warning.main' : 'text.disabled',
            }}
          >
            <StatusDotIcon size={10} fill="currentColor" strokeWidth={0} />
          </Box>
          <Typography variant="caption" sx={{ fontWeight: 600, fontSize: '0.75rem' }}>
            Custom Range...
          </Typography>
        </Box>

        {/* Custom range picker */}
        {showCustom && (
          <Box>
            {customError && (
              <Typography
                variant="caption"
                sx={{ color: 'error.main', fontSize: '0.6rem', display: 'block', px: 2, pt: 0.5 }}
              >
                {customError}
              </Typography>
            )}
            <CustomRangePicker
              onApply={handleApplyCustom}
              onError={handleCustomError}
              maxLookbackHours={maxLookbackHours}
              maxRangeSpanHours={maxRangeSpanHours}
            />
          </Box>
        )}
      </Popover>
    </Box>
  );
};

export default RecBlock;
