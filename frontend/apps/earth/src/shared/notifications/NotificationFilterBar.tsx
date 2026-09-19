import React, { useCallback } from 'react';
import { Box, Typography, Chip, IconButton, Button, Divider } from '@mui/material';
import { TuneIcon } from '../icons';
import { useUIStore } from '@/app/store';
import { DEFAULT_NOTIFICATION_FILTER, theme, alpha, semanticColors } from '@respondent/core';
import type { NotificationFilterOptions } from '@respondent/core';

/** Maps attention levels to their display colors. */
const ATTENTION_PILL_COLORS: Record<string, string> = {
  critical: 'rgba(255, 0, 50, 0.8)',
  high: 'rgba(255, 100, 50, 0.7)',
  medium: 'rgba(255, 200, 50, 0.6)',
  low: alpha(theme.palette.primary.main, 0.5),
  info: semanticColors.primary.alpha20,
};

/** Zustand selector: true when filter differs from defaults. */
function selectHasActiveFilter(s: {
  notificationFilter: { minAttention: string; insightTypes: string[] };
}): boolean {
  return (
    s.notificationFilter.minAttention !== DEFAULT_NOTIFICATION_FILTER.minAttention ||
    s.notificationFilter.insightTypes.length !== DEFAULT_NOTIFICATION_FILTER.insightTypes.length
  );
}

interface NotificationFilterToggleProps {
  open: boolean;
  onToggle: () => void;
}

/** Inline toggle button with active-filter indicator dot. */
export const NotificationFilterToggle: React.FC<NotificationFilterToggleProps> = React.memo(
  ({ open, onToggle }) => {
    const hasActiveFilter = useUIStore(selectHasActiveFilter);

    return (
      <IconButton
        data-testid="notification-filter-toggle"
        onClick={onToggle}
        size="small"
        sx={{
          position: 'relative',
          color: open || hasActiveFilter ? 'primary.main' : 'text.secondary',
          '&:hover': { color: 'primary.main' },
        }}
      >
        <TuneIcon size={14} />
        {/* Active filter indicator dot — matches BottomToolbar badge pattern */}
        {hasActiveFilter && !open && (
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
      </IconButton>
    );
  },
);

NotificationFilterToggle.displayName = 'NotificationFilterToggle';

interface NotificationFilterPanelProps {
  options: NotificationFilterOptions;
}

/**
 * Inline filter panel with attention level pills, insight type chips,
 * and a reset button. Renders in normal document flow so it pushes
 * content below it down.
 */
export const NotificationFilterPanel: React.FC<NotificationFilterPanelProps> = React.memo(
  ({ options }) => {
    const notificationFilter = useUIStore((s) => s.notificationFilter);
    const setNotificationFilter = useUIStore((s) => s.setNotificationFilter);
    const hasActiveFilter = useUIStore(selectHasActiveFilter);

    const handleAttentionToggle = useCallback(
      (level: string) => {
        const current = useUIStore.getState().notificationFilter.minAttention;
        // Deselecting the active pill reverts to the default, not empty string.
        setNotificationFilter({
          minAttention: current === level ? DEFAULT_NOTIFICATION_FILTER.minAttention : level,
        });
      },
      [setNotificationFilter],
    );

    const handleInsightTypeToggle = useCallback(
      (typeValue: string) => {
        const current = useUIStore.getState().notificationFilter.insightTypes;
        const next = current.includes(typeValue)
          ? current.filter((t) => t !== typeValue)
          : [...current, typeValue];
        setNotificationFilter({ insightTypes: next });
      },
      [setNotificationFilter],
    );

    const handleReset = useCallback(() => {
      setNotificationFilter({ ...DEFAULT_NOTIFICATION_FILTER });
    }, [setNotificationFilter]);

    return (
      <Box
        sx={{
          bgcolor: 'rgba(5, 5, 5, 0.95)',
          borderBottom: `1px solid ${semanticColors.primary.alpha10}`,
          px: 1.5,
          py: 1,
        }}
      >
        {/* Attention Level Pills */}
        <Box sx={{ mb: 0.75 }}>
          <Typography
            sx={{
              fontSize: '0.6rem',
              fontFamily: 'monospace',
              color: 'text.secondary',
              letterSpacing: '0.08em',
              mb: 0.5,
            }}
          >
            MIN ATTENTION
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
            {options.attentionLevels.map((level) => {
              const active = notificationFilter.minAttention === level;
              const pillColor = ATTENTION_PILL_COLORS[level] ?? 'rgba(255, 255, 255, 0.1)';
              return (
                <Chip
                  key={level}
                  data-testid={`notif-filter-attention-${level}`}
                  data-active={active}
                  label={level.toUpperCase()}
                  size="small"
                  onClick={() => handleAttentionToggle(level)}
                  sx={{
                    height: 20,
                    fontSize: '0.6rem',
                    fontFamily: 'monospace',
                    letterSpacing: '0.04em',
                    bgcolor: active ? pillColor : 'rgba(255, 255, 255, 0.04)',
                    color: active ? '#fff' : 'text.secondary',
                    border: `1px solid ${active ? pillColor : 'rgba(255, 255, 255, 0.08)'}`,
                    cursor: 'pointer',
                    '&:hover': { bgcolor: active ? pillColor : 'rgba(255, 255, 255, 0.08)' },
                  }}
                />
              );
            })}
          </Box>
        </Box>

        <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.05)', my: 0.75 }} />

        {/* Insight Types */}
        <Box sx={{ mb: 0.5 }}>
          <Typography
            sx={{
              fontSize: '0.6rem',
              fontFamily: 'monospace',
              color: 'text.secondary',
              letterSpacing: '0.08em',
              mb: 0.5,
            }}
          >
            INSIGHT TYPES{' '}
            <span>
              {notificationFilter.insightTypes.length > 0
                ? `(${notificationFilter.insightTypes.length})`
                : '(all)'}
            </span>
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
            {options.insightTypes.map((type) => {
              const selected = notificationFilter.insightTypes.includes(type.value);
              return (
                <Chip
                  key={type.value}
                  data-testid={`notif-filter-insight-${type.value}`}
                  data-active={selected}
                  label={type.displayName}
                  size="small"
                  onClick={() => handleInsightTypeToggle(type.value)}
                  sx={{
                    height: 20,
                    fontSize: '0.6rem',
                    fontFamily: 'monospace',
                    bgcolor: selected
                      ? alpha(theme.palette.primary.main, 0.15)
                      : 'rgba(255, 255, 255, 0.04)',
                    color: selected ? 'primary.main' : 'text.secondary',
                    border: `1px solid ${selected ? semanticColors.primary.alpha30 : 'rgba(255, 255, 255, 0.08)'}`,
                    cursor: 'pointer',
                    '&:hover': {
                      bgcolor: selected
                        ? alpha(theme.palette.primary.main, 0.25)
                        : 'rgba(255, 255, 255, 0.08)',
                    },
                  }}
                />
              );
            })}
          </Box>
        </Box>

        {/* Reset — only show when filter differs from defaults */}
        {hasActiveFilter && (
          <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button
              data-testid="notif-filter-reset"
              size="small"
              onClick={handleReset}
              sx={{
                fontSize: '0.6rem',
                fontFamily: 'monospace',
                letterSpacing: '0.08em',
                color: 'text.secondary',
                textTransform: 'none',
                minWidth: 0,
                py: 0,
                '&:hover': { color: 'primary.main' },
              }}
            >
              RESET
            </Button>
          </Box>
        )}
      </Box>
    );
  },
);

NotificationFilterPanel.displayName = 'NotificationFilterPanel';
