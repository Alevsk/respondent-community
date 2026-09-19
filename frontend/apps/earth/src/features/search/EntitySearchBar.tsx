/**
 * EntitySearchBar - Spotlight-style search overlay.
 *
 * Toggled by the Search button. Centers vertically on screen with a fade
 * animation matching ConfigPanel. Shows search input and results.
 * Closes on click-outside or ESC.
 */

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Box, ClickAwayListener, Fade, InputBase, Skeleton, Typography } from '@mui/material';
import { SearchIcon } from '../../shared/icons';
import { useUIStore } from '@/app/store';
import type { WatchlistEntity } from '@/app/store';
import {
  useResponsive,
  MOBILE_HEADER_HEIGHT,
  semanticColors,
  alpha,
  theme,
} from '@respondent/core';
import SearchResultItem from './SearchResultItem';
import type { EntitySearchResult } from './SearchResultItem';

export interface UseSearchEntitiesResult {
  data: { results: EntitySearchResult[]; totalCount: number } | undefined;
  isLoading: boolean;
}

export interface EntitySearchBarProps {
  searchResults?: UseSearchEntitiesResult;
}

const DEBOUNCE_MS = 300;

const EntitySearchBar: React.FC<EntitySearchBarProps> = React.memo(({ searchResults }) => {
  const searchOpen = useUIStore((s) => s.searchOpen);
  const toggleSearch = useUIStore((s) => s.toggleSearch);
  const searchQuery = useUIStore((s) => s.searchQuery);
  const setSearchQuery = useUIStore((s) => s.setSearchQuery);
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const addToWatchlist = useUIStore((s) => s.addToWatchlist);
  const { isMobile } = useResponsive();

  const inputRef = useRef<HTMLInputElement>(null);
  const [localQuery, setLocalQuery] = useState(searchQuery);
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Watchlist entity IDs for O(1) lookup
  const watchlistIds = useMemo(
    () => new Set(watchlistEntities.map((e) => e.entityId)),
    [watchlistEntities],
  );

  // Debounce local query → store query
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setSearchQuery(localQuery);
    }, DEBOUNCE_MS);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [localQuery, setSearchQuery]);

  // Auto-focus input when opened
  useEffect(() => {
    if (searchOpen) {
      const timer = setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    }
    setLocalQuery('');
    setFocusedIndex(-1);
  }, [searchOpen]);

  const results = useMemo(() => searchResults?.data?.results ?? [], [searchResults?.data?.results]);
  const isLoading = searchResults?.isLoading ?? false;
  const totalCount = searchResults?.data?.totalCount ?? 0;

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    setLocalQuery(e.target.value);
    setFocusedIndex(-1);
  }, []);

  const handleAdd = useCallback(
    (result: EntitySearchResult) => {
      const entity: WatchlistEntity = {
        entityId: result.entityId,
        layerId: result.layerType,
        name: result.name || result.externalId,
        pinned: true,
        addedAt: Date.now(),
      };
      addToWatchlist(entity);
    },
    [addToWatchlist],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        toggleSearch();
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setFocusedIndex((prev) => Math.min(prev + 1, results.length - 1));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setFocusedIndex((prev) => Math.max(prev - 1, -1));
        return;
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        const target = focusedIndex >= 0 ? results[focusedIndex] : results[0];
        if (target && !watchlistIds.has(target.entityId)) {
          handleAdd(target);
        }
        return;
      }
    },
    [toggleSearch, results, focusedIndex, watchlistIds, handleAdd],
  );

  const handleClickAway = useCallback(() => {
    if (searchOpen) {
      toggleSearch();
    }
  }, [searchOpen, toggleSearch]);

  if (!searchOpen) return null;

  return (
    <Fade in={searchOpen} timeout={200}>
      <Box
        sx={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 1200,
          display: 'flex',
          justifyContent: 'center',
          alignItems: isMobile ? 'flex-start' : 'center',
          pt: isMobile ? `calc(var(--sat, 0px) + ${MOBILE_HEADER_HEIGHT + 8}px)` : 0,
          pointerEvents: 'none',
        }}
      >
        <ClickAwayListener onClickAway={handleClickAway}>
          <Box
            data-testid="entity-search-bar"
            sx={{
              width: isMobile ? 'calc(100% - 32px)' : '100%',
              maxWidth: 560,
              pointerEvents: 'auto',
              bgcolor: 'rgba(5, 5, 5, 0.95)',
              backdropFilter: 'blur(16px)',
              borderRadius: 2,
              border: `1px solid ${semanticColors.primary.alpha20}`,
              boxShadow: `
                0 8px 32px rgba(0, 0, 0, 0.4),
                0 0 40px ${alpha(theme.palette.primary.main, 0.05)}
              `,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              maxHeight: isMobile ? 'calc(50dvh - var(--sat, 0px))' : '60vh',
            }}
          >
            {/* Search input */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                px: 1.5,
                py: 0.75,
                borderBottom:
                  localQuery.length > 0 ? '1px solid rgba(255, 255, 255, 0.06)' : 'none',
              }}
            >
              <SearchIcon size={16} color="#555" style={{ flexShrink: 0 }} />
              <InputBase
                inputRef={inputRef}
                value={localQuery}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder="Search by ID, name, or callsign..."
                fullWidth
                data-testid="search-input"
                inputProps={{
                  'aria-label': 'Search entities',
                  'data-testid': 'search-input-field',
                }}
                sx={{
                  fontFamily: 'monospace',
                  fontSize: '0.8125rem',
                  color: '#e0e0e0',
                  '& input::placeholder': {
                    color: '#555',
                    opacity: 1,
                  },
                }}
              />
            </Box>

            {/* Results area */}
            {localQuery.length > 0 && (
              <Box
                data-testid="search-results"
                sx={{
                  overflowY: 'auto',
                  scrollbarWidth: 'thin',
                  flex: 1,
                  minHeight: 0,
                }}
              >
                {isLoading ? (
                  <Box sx={{ p: 1 }}>
                    {[0, 1, 2].map((i) => (
                      <Skeleton
                        key={i}
                        variant="rectangular"
                        height={isMobile ? 44 : 36}
                        sx={{
                          mb: 0.5,
                          borderRadius: 1,
                          bgcolor: 'rgba(255, 255, 255, 0.04)',
                        }}
                      />
                    ))}
                  </Box>
                ) : results.length === 0 ? (
                  <Box data-testid="search-no-results" sx={{ py: 2.5, textAlign: 'center' }}>
                    <Typography
                      sx={{ color: '#555', fontSize: '0.75rem', fontFamily: 'monospace' }}
                    >
                      No entities found
                    </Typography>
                  </Box>
                ) : (
                  <>
                    {results.map((result, index) => (
                      <Box
                        key={result.entityId}
                        sx={{
                          bgcolor:
                            index === focusedIndex ? 'rgba(255, 255, 255, 0.06)' : 'transparent',
                        }}
                      >
                        <SearchResultItem
                          result={result}
                          isInWatchlist={watchlistIds.has(result.entityId)}
                          onAdd={handleAdd}
                        />
                      </Box>
                    ))}
                    {totalCount > results.length && (
                      <Box sx={{ py: 0.75, textAlign: 'center' }}>
                        <Typography
                          sx={{ color: '#444', fontSize: '0.625rem', fontFamily: 'monospace' }}
                        >
                          {totalCount - results.length} more — refine your search
                        </Typography>
                      </Box>
                    )}
                  </>
                )}
              </Box>
            )}
          </Box>
        </ClickAwayListener>
      </Box>
    </Fade>
  );
});

EntitySearchBar.displayName = 'EntitySearchBar';

export default EntitySearchBar;
