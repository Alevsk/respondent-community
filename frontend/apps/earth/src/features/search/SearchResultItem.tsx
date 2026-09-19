/**
 * SearchResultItem - A single row in the entity search results list.
 *
 * Displays the layer icon, entity name, and layer type.
 * Clicking the row adds the entity to the watchlist. Already-added items
 * show a persistent highlight and a check icon instead of the layer icon.
 */

import React, { useCallback } from 'react';
import { Box, Typography } from '@mui/material';
import { CheckCircleIcon } from '../../shared/icons';
import { CanvasLayerIcon } from './layerIcons';
import { useResponsive, alpha, theme } from '@respondent/core';

/** Shape of a search result from the API (gRPC-gateway returns camelCase JSON). */
export interface EntitySearchResult {
  entityId: string;
  externalId: string;
  layerType: string;
  name: string;
  latestObservation?: {
    entityId: string;
    ts: string;
    position: { lat: number; lon: number; altM: number };
    altitudeM: number;
    velocity?: Record<string, number>;
  };
  metadata?: Record<string, string>;
}

export interface SearchResultItemProps {
  result: EntitySearchResult;
  isInWatchlist: boolean;
  onAdd: (result: EntitySearchResult) => void;
}

const SearchResultItem: React.FC<SearchResultItemProps> = React.memo(
  ({ result, isInWatchlist, onAdd }) => {
    const { isMobile } = useResponsive();

    const handleClick = useCallback(() => {
      if (!isInWatchlist) {
        onAdd(result);
      }
    }, [onAdd, result, isInWatchlist]);

    return (
      <Box
        data-testid="search-result-item"
        onClick={handleClick}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          height: isMobile ? 44 : 36,
          px: 1.5,
          cursor: isInWatchlist ? 'default' : 'pointer',
          transition: 'background-color 0.15s ease',
          bgcolor: isInWatchlist ? alpha(theme.palette.primary.main, 0.08) : 'transparent',
          '&:hover': {
            bgcolor: isInWatchlist
              ? alpha(theme.palette.primary.main, 0.08)
              : 'rgba(255, 255, 255, 0.06)',
          },
        }}
      >
        {/* Layer icon or check icon if added */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            fontSize: 18,
            color: isInWatchlist ? 'primary.main' : 'text.secondary',
            flexShrink: 0,
            '& .MuiSvgIcon-root': { fontSize: 18 },
          }}
        >
          {isInWatchlist ? (
            <CheckCircleIcon size={18} />
          ) : (
            <CanvasLayerIcon layerType={result.layerType} />
          )}
        </Box>

        {/* Entity name */}
        <Typography
          data-testid="search-result-name"
          noWrap
          sx={{
            fontWeight: 600,
            fontSize: '0.8125rem',
            color: isInWatchlist ? 'primary.main' : '#e0e0e0',
            flex: 1,
            minWidth: 0,
          }}
        >
          {result.name || result.externalId}
        </Typography>

        {/* Layer type label */}
        <Typography
          data-testid="search-result-layer"
          noWrap
          sx={{
            fontSize: '0.625rem',
            color: isInWatchlist ? alpha(theme.palette.primary.main, 0.5) : '#555',
            flexShrink: 0,
            maxWidth: 100,
            fontFamily: 'monospace',
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
          }}
        >
          {result.layerType}
        </Typography>
      </Box>
    );
  },
);

SearchResultItem.displayName = 'SearchResultItem';

export default SearchResultItem;
