/**
 * WatchlistBar - A ConfigPanel-based watchlist panel displaying entity pills.
 *
 * Positioned above the BottomToolbar / MobileBottomNav. Supports minimize,
 * expand, and close via ConfigPanel. Double-click a pill to fly the camera
 * to that entity.
 *
 * Hidden when cleanUI or recordingMode is true, or when watchlist is empty.
 */

import React, { useCallback } from 'react';
import { Box, IconButton, Tooltip } from '@mui/material';
import {
  VisibilityIcon,
  VisibilityOffIcon,
  BlurOnIcon,
  DeleteSweepIcon,
  BookmarksIcon,
  iconSizes,
} from '../../shared/icons';
import ConfigPanel from '../../shared/ui/ConfigPanel';
import type { ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import { useUIStore } from '@/app/store';
import {
  useResponsive,
  DESKTOP_BOTTOM_STACK_BASE,
  MOBILE_NAV_HEIGHT,
  MOBILE_NAV_GAP,
  MOBILE_PANEL_MARGIN,
} from '@respondent/core';
import { useViewerStore } from '../globe/store';
import { Cartesian3 } from 'cesium';
import EntityPill from './EntityPill';
import { useWatchlistSync } from './useWatchlistSync';
import { useWatchlistBarHeight } from './useWatchlistBarHeight';

/** Minimum camera altitude above the entity for fly-to (meters). */
const FLY_TO_OFFSET_M = 25_000;

/** Look up an entity's observation data from layerEntities. */
function getEntityObservation(entityId: string) {
  const { layerEntities } = useUIStore.getState();
  for (const [, layerData] of layerEntities) {
    const obs = layerData.obsMap.get(entityId);
    if (obs) return obs;
  }
  return null;
}

const WatchlistBar: React.FC = React.memo(() => {
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);
  const findMode = useUIStore((s) => s.findMode);
  const findModeDisplay = useUIStore((s) => s.findModeDisplay);
  const cleanUI = useUIStore((s) => s.cleanUI);
  const recordingMode = useUIStore((s) => s.recordingMode);
  const panelOpen = useUIStore((s) => s.watchlistPanelOpen);
  const setSelectedEntity = useUIStore((s) => s.setSelectedEntity);
  const togglePin = useUIStore((s) => s.togglePin);
  const removeFromWatchlist = useUIStore((s) => s.removeFromWatchlist);
  const { isMobile } = useResponsive();

  // Ensure pinned entities have observation data for brackets and fly-to
  useWatchlistSync();

  // Measure and report the watchlist bar height to the store so sibling
  // components (MobileEntityPanel) can position themselves above it.
  // Must be called before any early returns to satisfy Rules of Hooks.
  const isVisible = !cleanUI && !recordingMode && watchlistEntities.length > 0;
  useWatchlistBarHeight(isVisible);

  const [displayMode, setDisplayMode] = React.useState<ConfigPanelDisplayMode>('normal');

  const handleClose = useCallback(() => {
    useUIStore.getState().clearUnpinned();
    useUIStore.getState().setWatchlistPanelOpen(false);
  }, []);

  // Auto-open panel and de-minimize when new entities are added.
  // prevCountRef initialized to current length so mount doesn't trigger.
  const prevCountRef = React.useRef(watchlistEntities.length);
  React.useEffect(() => {
    const count = watchlistEntities.length;
    const grew = count > prevCountRef.current;
    prevCountRef.current = count;

    if (grew) {
      if (!panelOpen) useUIStore.getState().setWatchlistPanelOpen(true);
      if (displayMode === 'minimized') setDisplayMode('normal');
    }
  }, [watchlistEntities.length, panelOpen, displayMode]);

  const handleSelect = useCallback(
    (entityId: string) => {
      const entity = useUIStore.getState().watchlistEntities.find((e) => e.entityId === entityId);
      if (entity) {
        setSelectedEntity(entityId, entity.layerId);
      }
      // Also fly the camera to the entity
      const viewer = useViewerStore.getState().viewer;
      if (!viewer || viewer.isDestroyed()) return;
      const obs = getEntityObservation(entityId);
      if (!obs) return;
      // Fly to a point above the entity so the billboard is visible below
      viewer.camera.flyTo({
        destination: Cartesian3.fromDegrees(
          obs.position.lon,
          obs.position.lat,
          Math.max((obs.altitudeM ?? 0) + FLY_TO_OFFSET_M, FLY_TO_OFFSET_M),
        ),
        duration: 1.5,
      });
    },
    [setSelectedEntity],
  );

  // Cycle: show all → dim non-pinned → hide non-pinned → show all
  // Read all state imperatively via getState() so callback is stable.
  const handleCycleVisibility = useCallback(() => {
    const state = useUIStore.getState();
    const hasPinned = state.watchlistEntities.some((e) => e.pinned);
    if (!state.findMode) {
      if (!hasPinned) return;
      state.toggleFindMode();
      state.setFindModeDisplay('dimmed');
    } else if (state.findModeDisplay === 'dimmed') {
      state.setFindModeDisplay('hidden');
    } else {
      state.toggleFindMode();
    }
  }, []);

  const handleClearWatchlist = useCallback(() => {
    useUIStore.getState().clearWatchlist();
  }, []);

  // Don't render when hidden or empty
  if (cleanUI || recordingMode || watchlistEntities.length === 0) return null;

  const pinnedCount = watchlistEntities.filter((e) => e.pinned).length;
  const statusValue =
    pinnedCount > 0
      ? `${pinnedCount} pinned / ${watchlistEntities.length} total`
      : `${watchlistEntities.length} tracked`;

  const visibilityTooltip = !findMode
    ? 'Dim non-pinned entities'
    : findModeDisplay === 'dimmed'
      ? 'Hide non-pinned entities'
      : 'Show all entities';

  const findModeToolbar = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
      {/* Clear all */}
      <Tooltip title="Clear watchlist">
        <IconButton
          size="small"
          onClick={handleClearWatchlist}
          aria-label="Clear watchlist"
          data-testid="watchlist-clear-all"
          sx={{
            color: 'text.secondary',
            p: 0.25,
            minWidth: isMobile ? 40 : 'auto',
            minHeight: isMobile ? 40 : 'auto',
            '&:hover': { color: 'error.main' },
          }}
        >
          <DeleteSweepIcon size={iconSizes.sm} />
        </IconButton>
      </Tooltip>

      {/* Visibility cycle: show all → dim non-pinned → hide non-pinned */}
      <Tooltip title={visibilityTooltip}>
        <IconButton
          size="small"
          onClick={handleCycleVisibility}
          aria-label={visibilityTooltip}
          data-testid="find-mode-toggle"
          sx={{
            color: findMode ? 'primary.main' : 'text.secondary',
            p: 0.25,
            minWidth: isMobile ? 40 : 'auto',
            minHeight: isMobile ? 40 : 'auto',
            '&:hover': { color: 'primary.main' },
            transition: 'color 0.2s ease, transform 0.2s ease',
          }}
        >
          {!findMode ? (
            <VisibilityIcon size={iconSizes.sm} />
          ) : findModeDisplay === 'dimmed' ? (
            <BlurOnIcon size={iconSizes.sm} />
          ) : (
            <VisibilityOffIcon size={iconSizes.sm} />
          )}
        </IconButton>
      </Tooltip>
    </Box>
  );

  const mobileBottom = `calc(${MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP}px + var(--sab, 0px))`;

  return (
    <ConfigPanel
      open={panelOpen}
      onClose={handleClose}
      title="Watchlist"
      icon={<BookmarksIcon size={iconSizes.sm} />}
      panelId="watchlist"
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={`Watchlist (${watchlistEntities.length})`}
      statusLabel="TRACKING"
      statusValue={statusValue}
      statusAction={findModeToolbar}
      data-testid="watchlist-bar"
      width={isMobile ? `calc(100vw - ${MOBILE_PANEL_MARGIN * 2}px)` : 480}
      bottom={isMobile ? mobileBottom : DESKTOP_BOTTOM_STACK_BASE}
      left={0}
      right={0}
      sx={{
        mx: 'auto',
        maxWidth: isMobile ? undefined : 560,
      }}
      maxHeight={isMobile ? '40dvh' : 300}
    >
      {/* Pills — flex-wrap layout */}
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 0.75,
        }}
      >
        {watchlistEntities.map((entity) => (
          <EntityPill
            key={entity.entityId}
            entity={entity}
            onSelect={handleSelect}
            onTogglePin={togglePin}
            onRemove={removeFromWatchlist}
          />
        ))}
      </Box>
    </ConfigPanel>
  );
});

WatchlistBar.displayName = 'WatchlistBar';

export default WatchlistBar;
