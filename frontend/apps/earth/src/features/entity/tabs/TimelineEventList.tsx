/**
 * TimelineEventList — vertical timeline with spine line and event cards
 * for entity observation history.
 *
 * Normal mode: click toggles highlight, double-click highlights + flies to point.
 * Tracking mode: single click highlights + flies to point.
 *
 * Uses MouseEvent.detail to distinguish single from double clicks:
 * detail=1 is a genuine first click, detail=2 is the second click of a
 * double-click gesture (ignored — the dblclick handler takes over).
 */

import React, { useCallback, useRef, useEffect, useMemo } from 'react';
import { keyframes } from '@emotion/react';
import { Box, Typography, Button, Skeleton } from '@mui/material';
import type { TrailPoint } from '../hooks/useEntityTrail';
import {
  formatUtcTime,
  formatUtcDate,
  alpha,
  theme,
  semanticColors,
  FONT_FAMILY,
  DASHBOARD_TYPOGRAPHY,
} from '@respondent/core';

const pulseDot = keyframes`
  0%, 100% { opacity: 0.6; transform: translateY(-50%) scale(1); }
  50%       { opacity: 1;   transform: translateY(-50%) scale(1.15); }
`;

export interface TimelineEventListProps {
  /** Trail points ordered oldest-first. Displayed newest-first (reversed). */
  points: TrailPoint[];
  /** Timestamp (ms) of the currently highlighted event, or null. */
  highlightedTs: number | null;
  /** Called when user clicks an event. Passes timestamp or null to deselect. */
  onHighlight: (ts: number | null) => void;
  /** Called when user double-clicks or single-clicks (tracking mode) an event. */
  onFlyTo: (point: TrailPoint) => void;
  /** When true, single click triggers both highlight and flyTo. */
  trackingMode?: boolean;
  /** Whether more pages can be loaded. */
  hasMore: boolean;
  /** Whether data is currently loading. */
  isLoading: boolean;
  /** Fetch the next page of older observations. */
  onLoadMore: () => void;
}

function formatAltitude(altitudeM: number): string {
  return `${Math.round(altitudeM).toLocaleString()}m`;
}

function formatSpeed(speed?: number): string | null {
  if (speed === undefined || speed === null) return null;
  return `${speed.toFixed(1)} m/s`;
}

const TimelineEventList: React.FC<TimelineEventListProps> = ({
  points,
  highlightedTs,
  onHighlight,
  onFlyTo,
  trackingMode = false,
  hasMore,
  isLoading,
  onLoadMore,
}) => {
  const highlightRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (highlightedTs !== null && highlightRef.current) {
      highlightRef.current.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }, [highlightedTs]);

  const handleClick = useCallback(
    (e: React.MouseEvent, point: TrailPoint) => {
      if (e.detail >= 2) return;

      if (trackingMode) {
        onHighlight(point.ts);
        onFlyTo(point);
      } else {
        onHighlight(highlightedTs === point.ts ? null : point.ts);
      }
    },
    [onHighlight, onFlyTo, highlightedTs, trackingMode],
  );

  const handleDoubleClick = useCallback(
    (point: TrailPoint) => {
      onHighlight(point.ts);
      onFlyTo(point);
    },
    [onHighlight, onFlyTo],
  );

  const displayOrder = useMemo(() => [...points].reverse(), [points]);

  if (isLoading && points.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        {[...Array(5)].map((_, i) => (
          <Skeleton key={i} variant="rectangular" height={40} sx={{ mb: 0.5, borderRadius: 1 }} />
        ))}
      </Box>
    );
  }

  if (points.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{ color: 'text.secondary', fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize }}
        >
          No observation history available
        </Typography>
      </Box>
    );
  }

  return (
    <Box data-testid="timeline-event-list" sx={{ position: 'relative' }}>
      {/* Timeline spine — 2px, subtle primary green tint */}
      <Box
        sx={{
          position: 'absolute',
          left: 11,
          top: 0,
          bottom: 0,
          width: '2px',
          bgcolor: alpha(theme.palette.primary.main, 0.15),
        }}
      />

      {displayOrder.map((point, displayIdx) => {
        const isHighlighted = highlightedTs === point.ts;
        const isNewest = displayIdx === 0;
        const speedStr = formatSpeed(point.speed);

        return (
          <Box
            key={`${point.ts}-${displayIdx}`}
            ref={isHighlighted ? highlightRef : undefined}
            data-testid="timeline-event-card"
            onClick={(e) => handleClick(e, point)}
            onDoubleClick={() => handleDoubleClick(point)}
            sx={{
              position: 'relative',
              pl: 3,
              pr: 0.5,
              py: 0.75,
              cursor: 'pointer',
              borderLeft: isHighlighted ? '3px solid' : '2px solid transparent',
              borderColor: isHighlighted ? 'primary.main' : 'transparent',
              borderRadius: isHighlighted ? '4px' : 0,
              borderBottom: '1px solid rgba(255,255,255,0.04)',
              bgcolor: isHighlighted ? alpha(theme.palette.primary.main, 0.06) : 'transparent',
              '&:last-of-type': {
                borderBottom: 'none',
              },
              '&:hover': {
                bgcolor: isHighlighted
                  ? alpha(theme.palette.primary.main, 0.09)
                  : alpha(theme.palette.primary.main, 0.03),
                '& .timeline-dot': {
                  width: isHighlighted ? 10 : 8,
                  height: isHighlighted ? 10 : 8,
                  left: isHighlighted ? 6 : 7,
                },
              },
              transition: 'background-color 0.15s, border-color 0.15s',
            }}
          >
            {/* Spine dot */}
            <Box
              className="timeline-dot"
              sx={{
                position: 'absolute',
                left: isHighlighted ? 6 : 7,
                top: '50%',
                transform: 'translateY(-50%)',
                width: isHighlighted ? 10 : 6,
                height: isHighlighted ? 10 : 6,
                borderRadius: '50%',
                bgcolor: isHighlighted ? 'primary.main' : alpha(theme.palette.primary.main, 0.4),
                border: isHighlighted
                  ? 'none'
                  : `1px solid ${alpha(theme.palette.primary.main, 0.3)}`,
                boxShadow: isHighlighted ? '0 0 6px rgba(0,255,157,0.4)' : 'none',
                transition: 'all 0.15s ease',
                ...(isNewest &&
                  !isHighlighted && {
                    animation: `${pulseDot} 2s ease-in-out infinite`,
                  }),
              }}
            />

            {/* Time + date row */}
            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.25 }}>
              <Typography
                variant="caption"
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm?.fontSize ?? '0.7rem',
                  fontFamily: FONT_FAMILY,
                  color: 'primary.main',
                  fontWeight: 700,
                  letterSpacing: '0.02em',
                }}
              >
                {formatUtcTime(point.ts)}
              </Typography>
              <Typography
                variant="caption"
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                  color: 'text.secondary',
                }}
              >
                {formatUtcDate(point.ts)}
              </Typography>
            </Box>

            {/* Position + altitude + speed row */}
            <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 0.5 }}>
              <Typography
                variant="caption"
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                  fontFamily: FONT_FAMILY,
                  color: 'text.primary',
                }}
              >
                {point.lat.toFixed(3)}, {point.lon.toFixed(3)}
              </Typography>
              <Typography
                variant="caption"
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                  color: alpha(theme.palette.text.secondary, 0.5),
                }}
              >
                ·
              </Typography>
              <Typography
                variant="caption"
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                  fontFamily: FONT_FAMILY,
                  color: 'text.secondary',
                }}
              >
                {formatAltitude(point.altitudeM)}
              </Typography>
              {speedStr && (
                <>
                  <Typography
                    variant="caption"
                    sx={{
                      fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                      color: alpha(theme.palette.text.secondary, 0.5),
                    }}
                  >
                    ·
                  </Typography>
                  <Typography
                    variant="caption"
                    sx={{
                      fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                      fontFamily: FONT_FAMILY,
                      color: 'text.secondary',
                    }}
                  >
                    {speedStr}
                  </Typography>
                </>
              )}
            </Box>
          </Box>
        );
      })}

      {/* Load More */}
      {hasMore && (
        <Button
          data-testid="load-more-observations"
          size="small"
          onClick={onLoadMore}
          disabled={isLoading}
          sx={{
            mt: 1,
            ml: 3,
            width: 'calc(100% - 24px)',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            color: 'primary.main',
            borderColor: semanticColors.primary.alpha20,
            textTransform: 'uppercase',
            letterSpacing: '0.1em',
          }}
          variant="outlined"
        >
          {isLoading ? 'Loading...' : 'Load More'}
        </Button>
      )}
    </Box>
  );
};

export default TimelineEventList;
