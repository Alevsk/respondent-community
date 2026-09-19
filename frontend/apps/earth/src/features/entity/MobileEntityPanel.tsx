/**
 * MobileEntityPanel — floating entity detail overlay for mobile viewports.
 *
 * Renders directly when entities are selected (driven by store state, not
 * drawer open/close). Starts minimized as a compact header bar showing the
 * entity name; tapping the header or the maximize button toggles to a
 * near-full-screen view with tabs and scrollable content.
 *
 * Two-state model: minimized ↔ maximized. No intermediate "normal" state.
 *
 * This replaces the MobileDrawer approach for entities, eliminating the
 * timing issue where Cesium's ScreenSpaceEventHandler callbacks couldn't
 * reliably trigger React useEffect-based drawer opens.
 */

import React, { useState, useMemo, useCallback } from 'react';
import { Box, Typography, IconButton, Button } from '@mui/material';
import {
  TrackChangesIcon,
  CloseIcon,
  OpenInFullIcon,
  CloseFullscreenIcon,
} from '../../shared/icons';
import {
  MOBILE_NAV_HEIGHT,
  MOBILE_NAV_GAP,
  MOBILE_STACK_GAP,
  MOBILE_PANEL_MARGIN,
  DASHBOARD_TYPOGRAPHY,
} from '@respondent/core';
import { useUIStore } from '@/app/store';
import { useEntityDetailWithFallback } from './useEntityDetailWithFallback';
import { getTabsForLayerType } from './tabs/tabRegistry';
import './tabs/OverviewTab';
import './tabs/AIAnalysisTab';
import './tabs/HistoryTab';
import './tabs/MetadataTab';

interface MobileEntityPanelItemProps {
  entityId: string;
  layerId: string;
}

const MobileEntityPanelItem: React.FC<MobileEntityPanelItemProps> = ({ entityId, layerId }) => {
  const isPrimary = useUIStore((s) => s.selectedEntityId === entityId);
  const viewMode = useUIStore((s) => s.viewMode);
  const clearSelection = useUIStore((s) => s.clearSelection);
  const removeSelectedEntity = useUIStore((s) => s.removeSelectedEntity);
  const activeTabId = useUIStore((s) => s.entityViewState[entityId]?.activeTab ?? 'overview');
  const updateEntityViewState = useUIStore((s) => s.updateEntityViewState);
  const [isMaximized, setIsMaximized] = useState(false);

  const { detail, isLoading } = useEntityDetailWithFallback(entityId, layerId);

  const layerType = detail?.entity?.layerType ?? layerId ?? 'unknown';
  const tabs = useMemo(() => getTabsForLayerType(layerType), [layerType]);
  const activeTab = useMemo(() => {
    return tabs.find((t) => t.id === activeTabId) ?? tabs[0];
  }, [tabs, activeTabId]);

  const entityName = detail?.entity?.name ?? entityId ?? '';

  const handleClose = useCallback(() => {
    removeSelectedEntity(entityId);
  }, [removeSelectedEntity, entityId]);

  const handleTabClick = useCallback(
    (tabId: string) => {
      updateEntityViewState(entityId, { activeTab: tabId });
    },
    [entityId, updateEntityViewState],
  );

  const toggleDisplayMode = useCallback(() => {
    setIsMaximized((prev) => !prev);
  }, []);

  const handleExitEntityView = useCallback(() => {
    clearSelection();
  }, [clearSelection]);

  const statusText = isPrimary && viewMode === 'entity' ? 'TRACKING' : 'SELECTED';

  if (isMaximized) {
    return (
      <Box
        data-testid="mobile-entity-panel"
        sx={(theme) => ({
          position: 'fixed',
          top: `calc(var(--sat) + ${MOBILE_PANEL_MARGIN}px)`,
          left: MOBILE_PANEL_MARGIN,
          right: MOBILE_PANEL_MARGIN,
          bottom: 'calc(var(--sab) + 64px)',
          zIndex: theme.zIndex.modal,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          bgcolor: 'background.paper',
          borderRadius: 2,
          border: `1px solid ${theme.palette.primary.main}33`,
          backdropFilter: 'blur(16px)',
          boxShadow: `0 8px 32px rgba(0,0,0,0.5), 0 0 40px ${theme.palette.primary.main}0d`,
        })}
      >
        <PanelHeader
          entityName={entityName}
          statusText={statusText}
          isMaximized={isMaximized}
          onToggle={toggleDisplayMode}
          onClose={handleClose}
        />
        <TabBar tabs={tabs} activeTabId={activeTab?.id} onTabClick={handleTabClick} />
        <Box sx={{ flex: 1, overflowY: 'auto', p: 1.5 }}>
          {activeTab && (
            <activeTab.component
              entityId={entityId}
              layerType={layerType}
              detail={detail}
              isLoading={isLoading}
            />
          )}
        </Box>
        {isPrimary && viewMode === 'entity' && <ExitButton onClick={handleExitEntityView} />}
      </Box>
    );
  }

  return (
    <Box
      data-testid="mobile-entity-panel"
      sx={(theme) => ({
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: `1px solid ${theme.palette.primary.main}33`,
        backdropFilter: 'blur(16px)',
        boxShadow: `0 4px 16px rgba(0,0,0,0.4), 0 0 20px ${theme.palette.primary.main}08`,
      })}
    >
      <PanelHeader
        entityName={entityName}
        statusText={statusText}
        isMaximized={isMaximized}
        onToggle={toggleDisplayMode}
        onClose={handleClose}
      />
    </Box>
  );
};

/** Compact header with entity name, status badge, and window controls. */
const PanelHeader: React.FC<{
  entityName: string;
  statusText: string;
  isMaximized: boolean;
  onToggle: () => void;
  onClose: () => void;
}> = React.memo(({ entityName, statusText, isMaximized, onToggle, onClose }) => (
  <Box
    sx={(theme) => ({
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      px: 1.5,
      py: 1,
      flexShrink: 0,
      borderBottom: isMaximized ? `1px solid ${theme.palette.divider}` : 'none',
      cursor: 'pointer',
      WebkitTapHighlightColor: 'transparent',
    })}
    onClick={onToggle}
  >
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0, flex: 1 }}>
      <Box component="span" sx={{ display: 'inline-flex', color: 'primary.main', flexShrink: 0 }}>
        <TrackChangesIcon size={14} />
      </Box>
      <Typography
        noWrap
        sx={{
          fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize,
          fontWeight: 700,
          letterSpacing: '0.08em',
          textTransform: 'uppercase',
          color: 'primary.main',
        }}
      >
        {entityName}
      </Typography>
      <Typography
        sx={{
          fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
          fontWeight: 600,
          letterSpacing: '0.06em',
          color: statusText === 'TRACKING' ? 'warning.main' : 'text.secondary',
          textTransform: 'uppercase',
          flexShrink: 0,
        }}
      >
        {statusText}
      </Typography>
    </Box>
    <Box
      sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}
      onClick={(e) => e.stopPropagation()}
    >
      <IconButton
        size="small"
        onClick={onToggle}
        aria-label={isMaximized ? 'Restore panel' : 'Maximize panel'}
        sx={{
          color: isMaximized ? 'primary.main' : 'text.secondary',
          '&:hover': { color: 'primary.main' },
          p: 0.5,
        }}
      >
        {isMaximized ? <CloseFullscreenIcon size={14} /> : <OpenInFullIcon size={14} />}
      </IconButton>
      <IconButton
        size="small"
        onClick={onClose}
        aria-label="Close entity panel"
        sx={{
          color: 'text.secondary',
          '&:hover': { color: 'primary.main' },
          p: 0.5,
        }}
      >
        <CloseIcon size={14} />
      </IconButton>
    </Box>
  </Box>
));

PanelHeader.displayName = 'PanelHeader';

/** Tab bar for switching between Overview / History / Metadata. */
const TabBar: React.FC<{
  tabs: Array<{ id: string; label: string }>;
  activeTabId: string | undefined;
  onTabClick: (tabId: string) => void;
}> = React.memo(({ tabs, activeTabId, onTabClick }) => {
  if (tabs.length === 0) return null;
  return (
    <Box
      sx={(theme) => ({
        display: 'flex',
        flexShrink: 0,
        borderBottom: `1px solid ${theme.palette.divider}`,
      })}
    >
      {tabs.map((tab) => (
        <Box
          key={tab.id}
          data-testid={`mobile-entity-tab-${tab.id}`}
          data-active={activeTabId === tab.id}
          onClick={() => onTabClick(tab.id)}
          sx={{
            px: 1.5,
            py: 0.75,
            cursor: 'pointer',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
            fontWeight: 600,
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
            color: activeTabId === tab.id ? 'primary.main' : 'text.secondary',
            borderBottom: activeTabId === tab.id ? '2px solid' : '2px solid transparent',
            borderColor: activeTabId === tab.id ? 'primary.main' : 'transparent',
            transition: 'color 0.15s, border-color 0.15s',
            WebkitTapHighlightColor: 'transparent',
          }}
        >
          {tab.label}
        </Box>
      ))}
    </Box>
  );
});

TabBar.displayName = 'TabBar';

/** Exit entity view button. */
const ExitButton: React.FC<{ onClick: () => void }> = ({ onClick }) => (
  <Box
    sx={(theme) => ({
      flexShrink: 0,
      px: 1.5,
      pb: 1,
      pt: 0.5,
      borderTop: `1px solid ${theme.palette.divider}`,
    })}
  >
    <Button
      fullWidth
      size="small"
      variant="outlined"
      data-testid="entity-exit-view"
      onClick={onClick}
      sx={{
        fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
        fontWeight: 600,
        letterSpacing: '0.1em',
        textTransform: 'uppercase',
        color: 'primary.main',
        borderColor: 'primary.dark',
        '&:hover': {
          borderColor: 'primary.main',
          bgcolor: 'action.hover',
        },
      }}
    >
      Exit Entity View
    </Button>
  </Box>
);

/**
 * MobileEntityPanel — container that renders a floating panel per selected entity.
 *
 * Positioned above the bottom nav bar. Purely reactive to store state —
 * no drawer open/close mechanism needed.
 */
const MobileEntityPanel: React.FC = () => {
  const selectedEntities = useUIStore((s) => s.selectedEntities);
  const watchlistBarHeight = useUIStore((s) => s.watchlistBarHeight);

  if (selectedEntities.length === 0) return null;

  const baseOffset = MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP;
  const bottomOffset =
    watchlistBarHeight > 0 ? baseOffset + watchlistBarHeight + MOBILE_STACK_GAP : baseOffset;

  return (
    <Box
      data-testid="mobile-entity-panel-container"
      sx={(theme) => ({
        position: 'absolute',
        bottom: `calc(var(--sab) + ${bottomOffset}px)`,
        left: MOBILE_PANEL_MARGIN,
        right: MOBILE_PANEL_MARGIN,
        zIndex: theme.zIndex.drawer,
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
        pointerEvents: 'auto',
      })}
    >
      {selectedEntities.map((sel) => (
        <MobileEntityPanelItem key={sel.entityId} entityId={sel.entityId} layerId={sel.layerId} />
      ))}
    </Box>
  );
};

export default MobileEntityPanel;
