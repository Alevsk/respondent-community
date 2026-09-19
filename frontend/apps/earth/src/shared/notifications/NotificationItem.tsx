import React, { useState, useCallback, useMemo } from 'react';
import { Box, Typography, Chip } from '@mui/material';
import { useUIStore } from '@/app/store';
import {
  attentionColor,
  extractTitle,
  extractDescription,
  relativeTime,
  theme,
  alpha,
  semanticColors,
  NOTIFICATION_SKIP_KEYS,
} from '@respondent/core';
import type { AIInsightNotification } from '@respondent/core';
import { CanvasLayerIcon } from '../../features/search/layerIcons';

/** Max entity chips shown before "show more" toggle. */
const ENTITY_DISPLAY_LIMIT = 5;

interface NotificationItemProps {
  notification: AIInsightNotification;
  isRead: boolean;
  isMobile: boolean;
  onEntityClick: (
    entityId: string,
    layerType: string | undefined,
    notification: AIInsightNotification,
  ) => void;
  onObservationClick: (observationId: string, layerType?: string) => void;
}

const NotificationItem: React.FC<NotificationItemProps> = React.memo(
  ({ notification, isRead, isMobile, onEntityClick, onObservationClick }) => {
    const [expanded, setExpanded] = useState(false);
    const [showAllEntities, setShowAllEntities] = useState(false);
    const markRead = useUIStore((s) => s.markRead);

    const title = extractTitle(notification);
    const description = extractDescription(notification.result);

    // The result often contains the relevant external ID (human-readable),
    // while entityIds contains internal UUIDs. Show external ID prominently.
    // Guard against scientific notation (e.g. 1.66e+08 → "166217243").
    const externalId = (() => {
      const raw = notification.result.entity_external_id;
      if (raw == null) return undefined;
      const s = String(raw);
      if (/e[+-]?\d/i.test(s)) {
        const n = Number(s);
        if (Number.isFinite(n)) return n.toFixed(0);
      }
      return s;
    })();

    const resultChips = useMemo(() => {
      /** Replace scientific notation (e.g. 1.66e+08) with full integers, even when embedded in text. */
      const fmt = (v: unknown): string =>
        String(v).replace(/-?\d+\.?\d*e[+-]?\d+/gi, (m) => {
          const n = Number(m);
          return Number.isFinite(n) ? n.toFixed(0) : m;
        });
      return Object.entries(notification.result)
        .filter(
          ([k, v]) =>
            !NOTIFICATION_SKIP_KEYS.has(k) && typeof v !== 'object' && String(v).length < 60,
        )
        .map(([k, v]) => [k, fmt(v)] as [string, string])
        .slice(0, 8);
    }, [notification.result]);

    const handleClick = useCallback(() => {
      setExpanded((prev) => !prev);
      if (!isRead) {
        markRead(notification.id);
      }
    }, [isRead, markRead, notification.id]);

    const handleEntityChipClick = useCallback(
      (e: React.MouseEvent, entityId: string, layerType?: string) => {
        e.stopPropagation();
        onEntityClick(entityId, layerType ?? notification.layerType, notification);
      },
      [onEntityClick, notification],
    );

    const handleObservationChipClick = useCallback(
      (e: React.MouseEvent, observationId: string) => {
        e.stopPropagation();
        onObservationClick(observationId, notification.layerType);
      },
      [onObservationClick, notification.layerType],
    );

    const borderColor = attentionColor(notification.attention);
    const chipHeight = isMobile ? 40 : 24;

    return (
      <Box
        data-testid={`notification-item-${notification.id}`}
        onClick={handleClick}
        sx={{
          display: 'flex',
          cursor: 'pointer',
          borderLeft: `3px solid ${borderColor}`,
          bgcolor: isRead ? 'transparent' : alpha(theme.palette.primary.main, 0.05),
          px: 1.5,
          py: 1,
          minHeight: isMobile ? 56 : undefined,
          '&:hover': { bgcolor: alpha(theme.palette.primary.main, 0.08) },
          transition: 'background-color 0.15s',
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {/* Top line: type badge + timestamp */}
          <Box
            sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}
          >
            <Typography
              sx={{
                fontSize: '0.65rem',
                fontWeight: 700,
                fontFamily: 'monospace',
                letterSpacing: '0.1em',
                color: borderColor,
                textTransform: 'uppercase',
              }}
            >
              {notification.insightType.replace(/_/g, ' ')}
            </Typography>
            <Typography
              sx={{
                fontSize: '0.65rem',
                fontFamily: 'monospace',
                color: 'text.secondary',
                letterSpacing: '0.05em',
              }}
            >
              {relativeTime(notification.createdAt)}
            </Typography>
          </Box>

          {/* Title */}
          <Typography
            sx={{
              fontSize: '0.85rem',
              fontWeight: 600,
              fontFamily: 'monospace',
              color: 'text.primary',
              mb: 0.5,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: expanded ? 'normal' : 'nowrap',
            }}
          >
            {title}
          </Typography>

          {/* Description */}
          {description && (
            <Typography
              sx={{
                fontSize: '0.75rem',
                fontFamily: 'monospace',
                color: 'text.secondary',
                lineHeight: 1.4,
                ...(expanded
                  ? {}
                  : {
                      display: '-webkit-box',
                      WebkitLineClamp: 2,
                      WebkitBoxOrient: 'vertical',
                      overflow: 'hidden',
                    }),
              }}
            >
              {description}
            </Typography>
          )}

          {/* Bottom line: key details (collapsed) */}
          {!expanded && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 0.5, flexWrap: 'wrap' }}>
              {notification.layerType && (
                <Typography
                  sx={{
                    fontSize: '0.65rem',
                    fontFamily: 'monospace',
                    color: alpha(theme.palette.primary.main, 0.6),
                    letterSpacing: '0.05em',
                  }}
                >
                  {notification.layerType}
                </Typography>
              )}
              {externalId && (
                <Typography
                  sx={{
                    fontSize: '0.65rem',
                    fontFamily: 'monospace',
                    color: 'primary.main',
                    letterSpacing: '0.05em',
                  }}
                >
                  {externalId}
                </Typography>
              )}
              {notification.attention && (
                <Typography
                  sx={{
                    fontSize: '0.65rem',
                    fontFamily: 'monospace',
                    color: borderColor,
                    letterSpacing: '0.05em',
                    textTransform: 'uppercase',
                  }}
                >
                  {notification.attention}
                </Typography>
              )}
            </Box>
          )}

          {/* Expanded: result details */}
          {expanded && (
            <Box sx={{ mt: 1 }}>
              {/* Key-value pairs from result (excluding long text and internal fields) */}
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                {/* Show standalone ID chip only when no rich entity refs exist */}
                {externalId && notification.entities.length === 0 && (
                  <Chip
                    label={`ID: ${externalId}`}
                    size="small"
                    onClick={(e) =>
                      handleEntityChipClick(
                        e,
                        notification.layerType && externalId
                          ? `${notification.layerType}:${externalId}`
                          : externalId,
                      )
                    }
                    sx={{
                      height: chipHeight,
                      fontSize: '0.7rem',
                      fontFamily: 'monospace',
                      bgcolor: alpha(theme.palette.primary.main, 0.15),
                      color: 'primary.main',
                      border: `1px solid ${semanticColors.primary.alpha30}`,
                      cursor: 'pointer',
                      '&:hover': { bgcolor: alpha(theme.palette.primary.main, 0.25) },
                    }}
                  />
                )}
                {resultChips.map(([key, value]) => (
                  <Chip
                    key={key}
                    label={`${key.replace(/_/g, ' ')}: ${value}`}
                    size="small"
                    sx={{
                      height: chipHeight,
                      fontSize: '0.65rem',
                      fontFamily: 'monospace',
                      bgcolor: 'rgba(255, 255, 255, 0.05)',
                      color: 'text.secondary',
                      border: '1px solid rgba(255, 255, 255, 0.1)',
                    }}
                  />
                ))}
              </Box>
            </Box>
          )}

          {/* Expanded: associated entity chips.
              Prefer rich entity refs from notification.entities; fall back to
              raw entityIds for backwards compat with cached/old notifications.
              Capped at ENTITY_DISPLAY_LIMIT with a "show more" toggle. */}
          {expanded &&
            (notification.entities.length > 0 || notification.entityIds.length > 0) &&
            (() => {
              const totalEntities =
                notification.entities.length > 0
                  ? notification.entities.length
                  : notification.entityIds.length;
              const limit = showAllEntities ? totalEntities : ENTITY_DISPLAY_LIMIT;
              const remaining = totalEntities - ENTITY_DISPLAY_LIMIT;

              return (
                <Box sx={{ mt: 1 }}>
                  <Typography
                    sx={{
                      fontSize: '0.6rem',
                      fontFamily: 'monospace',
                      color: 'text.secondary',
                      letterSpacing: '0.08em',
                      mb: 0.5,
                    }}
                  >
                    ENTITIES ({totalEntities})
                  </Typography>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {notification.entities.length > 0
                      ? notification.entities.slice(0, limit).map((entity) => (
                          <Chip
                            key={entity.id}
                            data-testid={`notif-entity-chip-${entity.id}`}
                            icon={
                              entity.layerType ? (
                                <CanvasLayerIcon layerType={entity.layerType} size={14} />
                              ) : undefined
                            }
                            label={entity.name || entity.externalId || entity.id.slice(0, 8)}
                            size="small"
                            onClick={(e) =>
                              handleEntityChipClick(
                                e,
                                entity.layerType && entity.externalId
                                  ? `${entity.layerType}:${entity.externalId}`
                                  : entity.id,
                                entity.layerType,
                              )
                            }
                            sx={{
                              height: chipHeight,
                              fontSize: '0.7rem',
                              fontFamily: 'monospace',
                              bgcolor: semanticColors.primary.alpha10,
                              color: 'primary.main',
                              border: `1px solid ${semanticColors.primary.alpha20}`,
                              cursor: 'pointer',
                              '&:hover': { bgcolor: semanticColors.primary.alpha20 },
                              '& > img': { ml: '5px' },
                            }}
                          />
                        ))
                      : notification.entityIds.slice(0, limit).map((eid) => (
                          <Chip
                            key={eid}
                            data-testid={`notif-entity-chip-${eid}`}
                            label={eid.slice(0, 8)}
                            size="small"
                            onClick={(e) => handleEntityChipClick(e, eid)}
                            sx={{
                              height: chipHeight,
                              fontSize: '0.7rem',
                              fontFamily: 'monospace',
                              bgcolor: semanticColors.primary.alpha10,
                              color: 'primary.main',
                              border: `1px solid ${semanticColors.primary.alpha20}`,
                              cursor: 'pointer',
                              '&:hover': { bgcolor: semanticColors.primary.alpha20 },
                            }}
                          />
                        ))}
                  </Box>
                  {remaining > 0 && (
                    <Typography
                      data-testid="notif-show-more-entities"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowAllEntities((prev) => !prev);
                      }}
                      sx={{
                        fontSize: '0.6rem',
                        fontFamily: 'monospace',
                        color: 'primary.main',
                        cursor: 'pointer',
                        mt: 0.5,
                        '&:hover': { textDecoration: 'underline' },
                      }}
                    >
                      {showAllEntities ? 'show less' : `+${remaining} more`}
                    </Typography>
                  )}
                </Box>
              );
            })()}

          {/* Expanded: observation chips */}
          {expanded && notification.observationIds.length > 0 && (
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, mt: 0.5 }}>
              {notification.observationIds.map((obsId) => (
                <Chip
                  key={obsId}
                  data-testid={`notif-obs-chip-${obsId}`}
                  label={`obs: ${obsId.slice(0, 8)}...`}
                  size="small"
                  onClick={(e) => handleObservationChipClick(e, obsId)}
                  sx={{
                    height: chipHeight,
                    fontSize: '0.7rem',
                    fontFamily: 'monospace',
                    bgcolor: 'rgba(255, 255, 255, 0.05)',
                    color: 'text.secondary',
                    border: '1px solid rgba(255, 255, 255, 0.1)',
                    cursor: 'pointer',
                    '&:hover': { bgcolor: 'rgba(255, 255, 255, 0.1)' },
                  }}
                />
              ))}
            </Box>
          )}
        </Box>
      </Box>
    );
  },
);

NotificationItem.displayName = 'NotificationItem';

export default NotificationItem;
