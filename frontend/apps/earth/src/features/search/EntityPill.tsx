/**
 * EntityPill - A compact chip displaying a watchlist entity.
 *
 * Shows layer icon, entity name (truncated), pin toggle, and remove button.
 * Single-click selects the entity. Double-click flies the camera to it.
 * Pinned entities get a primary-colored border; unpinned use a subtle border.
 */

import React, { useCallback } from 'react';
import { Box, IconButton, Typography } from '@mui/material';
import { PushPinIcon, PushPinOutlinedIcon, CloseIcon } from '../../shared/icons';
import type { WatchlistEntity } from '@/app/store';
import { CanvasLayerIcon } from './layerIcons';
import { alpha, theme } from '@respondent/core';

export interface EntityPillProps {
  entity: WatchlistEntity;
  onSelect: (entityId: string) => void;
  onTogglePin: (entityId: string) => void;
  onRemove: (entityId: string) => void;
  onFlyTo?: (entityId: string) => void;
}

const EntityPill: React.FC<EntityPillProps> = React.memo(
  ({ entity, onSelect, onTogglePin, onRemove, onFlyTo }) => {
    const handleClick = useCallback(() => {
      onSelect(entity.entityId);
    }, [onSelect, entity.entityId]);

    const handleDoubleClick = useCallback(() => {
      onFlyTo?.(entity.entityId);
    }, [onFlyTo, entity.entityId]);

    const handleTogglePin = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onTogglePin(entity.entityId);
      },
      [onTogglePin, entity.entityId],
    );

    const handleRemove = useCallback(
      (e: React.MouseEvent) => {
        e.stopPropagation();
        onRemove(entity.entityId);
      },
      [onRemove, entity.entityId],
    );

    return (
      <Box
        data-testid="entity-pill"
        onClick={handleClick}
        onDoubleClick={handleDoubleClick}
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0.5,
          height: 32,
          px: 0.75,
          borderRadius: '16px',
          bgcolor: 'rgba(255, 255, 255, 0.06)',
          border: entity.pinned
            ? `1px solid ${alpha(theme.palette.primary.main, 0.4)}`
            : '1px solid rgba(255, 255, 255, 0.12)',
          cursor: 'pointer',
          flexShrink: 0,
          transition: 'background-color 0.15s ease, border-color 0.15s ease',
          '&:hover': {
            bgcolor: 'rgba(255, 255, 255, 0.1)',
          },
          '&:active': {
            bgcolor: alpha(theme.palette.primary.main, 0.15),
          },
        }}
      >
        {/* Layer icon */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            fontSize: 14,
            color: 'text.secondary',
            '& .MuiSvgIcon-root': { fontSize: 14 },
          }}
        >
          <CanvasLayerIcon layerType={entity.layerId} />
        </Box>

        {/* Entity name */}
        <Typography
          data-testid="entity-pill-name"
          noWrap
          sx={{
            fontFamily: 'monospace',
            fontSize: '0.6875rem',
            color: '#e0e0e0',
            maxWidth: 100,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            lineHeight: 1,
          }}
        >
          {entity.name}
        </Typography>

        {/* Pin toggle */}
        <IconButton
          size="small"
          onClick={handleTogglePin}
          aria-label={entity.pinned ? 'Unpin entity' : 'Pin entity'}
          data-testid="entity-pill-pin"
          sx={{
            p: 0.125,
            color: entity.pinned ? 'primary.main' : '#666',
            '&:hover': { color: 'primary.main' },
          }}
        >
          {entity.pinned ? <PushPinIcon size={14} /> : <PushPinOutlinedIcon size={14} />}
        </IconButton>

        {/* Remove button */}
        <IconButton
          size="small"
          onClick={handleRemove}
          aria-label="Remove from watchlist"
          data-testid="entity-pill-remove"
          sx={{
            p: 0.125,
            color: '#666',
            '&:hover': { color: 'secondary.main' },
          }}
        >
          <CloseIcon size={12} />
        </IconButton>
      </Box>
    );
  },
);

EntityPill.displayName = 'EntityPill';

export default EntityPill;
