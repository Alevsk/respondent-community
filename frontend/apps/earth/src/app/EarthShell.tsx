import React, { useCallback } from 'react';
import { Box } from '@mui/material';
import GlobeScene from '../features/globe/GlobeScene';
import DataLayersPanel from '../features/layers/DataLayersPanel';
import RecBlock from '../shared/hud/RecBlock';
import StatusReadout from '../shared/hud/StatusReadout';
import BottomToolbar from '../shared/hud/BottomToolbar';
import SettingsPanel from '../features/settings/SettingsPanel';
import NavigationPanel from '../features/navigation/NavigationPanel';
import EntityDetailPanel from '../features/entity/EntityDetailPanel';
import MobileEntityPanel from '../features/entity/MobileEntityPanel';
import MobileHudLayout from '../shared/hud/MobileHudLayout';
import MobileBottomNav from '../shared/hud/MobileBottomNav';
import MobileDrawer from '../shared/ui/MobileDrawer';
import { ClusterListPanel } from '../features/globe/ClusterListPanel';
import MobileClusterPanel from '../features/globe/MobileClusterPanel';
import RecordingMode from '../features/recording/RecordingMode';
import EntitySearchBar from '../features/search/EntitySearchBar';
import WatchlistBar from '../features/search/WatchlistBar';
import IndicatorHUD from '../features/indicators/IndicatorHUD';
import { useWatchlistPersistence } from '../features/search/useWatchlistPersistence';
import { useSearchEntities } from '../features/search/useSearchEntities';
import { useResponsive } from '@respondent/core';
import ErrorBoundary from '../shared/ui/ErrorBoundary';
import NotificationBell from '../shared/notifications/NotificationBell';
import NotificationPanel from '../shared/notifications/NotificationPanel';
import { useNotificationBackfill } from '../shared/notifications/useNotificationBackfill';
import { useUIStore } from '@/app/store';

/** Renders one EntityDetailPanel per selected entity (desktop only). */
const EntityDetailPanels: React.FC = () => {
  const selectedEntities = useUIStore((s) => s.selectedEntities);
  return (
    <>
      {selectedEntities.map((sel) => (
        <EntityDetailPanel key={sel.entityId} entityId={sel.entityId} layerId={sel.layerId} />
      ))}
    </>
  );
};

/** Mobile drawer panels for Layers, Settings, Nav (entity detail uses floating panel instead). */
const MobileDrawerPanels: React.FC = () => {
  const activeMobileDrawer = useUIStore((s) => s.activeMobileDrawer);
  const closeMobileDrawer = useUIStore((s) => s.closeMobileDrawer);
  const openMobileDrawer = useUIStore((s) => s.openMobileDrawer);
  const handleOpenLayers = useCallback(() => openMobileDrawer('layers'), [openMobileDrawer]);
  const handleOpenSettings = useCallback(() => openMobileDrawer('settings'), [openMobileDrawer]);
  const handleOpenNav = useCallback(() => openMobileDrawer('nav'), [openMobileDrawer]);

  return (
    <>
      <MobileDrawer
        open={activeMobileDrawer === 'layers'}
        onClose={closeMobileDrawer}
        onOpen={handleOpenLayers}
        title="Data Layers"
        heightPercent={65}
      >
        <DataLayersPanel open mobile onClose={closeMobileDrawer} />
      </MobileDrawer>

      <MobileDrawer
        open={activeMobileDrawer === 'settings'}
        onClose={closeMobileDrawer}
        onOpen={handleOpenSettings}
        title="Settings"
        heightPercent={60}
      >
        <SettingsPanel open mobile onClose={closeMobileDrawer} />
      </MobileDrawer>

      <MobileDrawer
        open={activeMobileDrawer === 'nav'}
        onClose={closeMobileDrawer}
        onOpen={handleOpenNav}
        title="Navigation"
        heightPercent={50}
      >
        <NavigationPanel open mobile onClose={closeMobileDrawer} />
      </MobileDrawer>
    </>
  );
};

/**
 * EarthShell - The main application layout component.
 *
 * Renders the 3D globe with HUD elements, panels, and navigation toolbar.
 * On mobile viewports, switches to a condensed mobile HUD with bottom
 * navigation and drawer-based panels.
 */
const EarthShell: React.FC = () => {
  const { isMobile } = useResponsive();
  const cleanUI = useUIStore((s) => s.cleanUI);
  const recordingMode = useUIStore((s) => s.recordingMode);
  const searchQuery = useUIStore((s) => s.searchQuery);
  const desktopPanelOpen = useUIStore((s) => s.desktopPanelOpen);
  const closeDesktopPanel = useUIStore((s) => s.closeDesktopPanel);

  // Persist pinned watchlist entities to localStorage
  useWatchlistPersistence();

  // Search API integration
  const searchResult = useSearchEntities(searchQuery);

  // Notification hooks (stream subscription moved to useAppBootstrap)
  const { loadMore: loadMoreNotifications } = useNotificationBackfill();
  const searchResults = React.useMemo(
    () => ({
      data: searchResult.data
        ? { results: searchResult.data.results, totalCount: searchResult.data.totalCount }
        : undefined,
      isLoading: searchResult.isLoading,
    }),
    [searchResult.data, searchResult.isLoading],
  );

  return (
    <Box
      data-testid="earth-shell"
      sx={{
        width: '100%',
        height: '100%',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* 3D Globe with post-processing effects (effects applied only to map) */}
      <ErrorBoundary
        fallback={
          <Box
            data-testid="globe-error-fallback"
            sx={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              bgcolor: 'background.default',
              color: 'text.secondary',
              fontFamily: 'monospace',
              fontSize: '0.75rem',
              letterSpacing: '0.05em',
            }}
          >
            GLOBE UNAVAILABLE
          </Box>
        }
      >
        <GlobeScene />
      </ErrorBoundary>

      {/* Recording mode overlay (both mobile and desktop) */}
      {recordingMode && <RecordingMode />}

      {isMobile ? (
        <>
          {/* Mobile HUD */}
          <MobileHudLayout />

          {/* Global indicator overlays */}
          {!recordingMode && <IndicatorHUD />}

          {/* Mobile drawer-based panels (layers, settings, nav) */}
          <MobileDrawerPanels />

          {/* Notification panel (mobile uses MobileDrawer) */}
          <NotificationPanel loadMore={loadMoreNotifications} />

          {/* Floating entity detail panel — reactive to store selection state */}
          <MobileEntityPanel />

          {/* Floating cluster member list — reactive to activeCluster (no Drawer:
              a canvas-gesture-opened Modal is closed by the touch ghost-click) */}
          <MobileClusterPanel />

          {/* Spotlight search overlay */}
          <EntitySearchBar searchResults={searchResults} />

          {/* Pinned entities bar (above bottom nav) */}
          <WatchlistBar />

          {/* Mobile bottom navigation */}
          <MobileBottomNav />
        </>
      ) : (
        <>
          {/* Desktop HUD elements (unchanged) */}
          {!recordingMode && (
            <>
              <RecBlock />
              <StatusReadout />
              <IndicatorHUD />
              <NotificationBell />
            </>
          )}

          {/* Floating panels (hidden in clean UI mode or recording mode) */}
          {!cleanUI && !recordingMode && (
            <>
              <DataLayersPanel
                open={desktopPanelOpen.layers}
                onClose={() => closeDesktopPanel('layers')}
              />
              <SettingsPanel
                open={desktopPanelOpen.settings}
                onClose={() => closeDesktopPanel('settings')}
              />
              <NavigationPanel
                open={desktopPanelOpen.nav}
                onClose={() => closeDesktopPanel('nav')}
              />
              <EntityDetailPanels />
              <ClusterListPanel />
              <NotificationPanel loadMore={loadMoreNotifications} />
            </>
          )}

          {/* Spotlight search overlay */}
          {!recordingMode && <EntitySearchBar searchResults={searchResults} />}

          {/* Pinned entities bar (above bottom toolbar) */}
          {!recordingMode && <WatchlistBar />}

          {/* Bottom Toolbar */}
          {!recordingMode && <BottomToolbar />}
        </>
      )}
    </Box>
  );
};

export default EarthShell;
