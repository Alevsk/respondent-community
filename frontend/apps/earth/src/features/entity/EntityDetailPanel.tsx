/**
 * EntityDetailPanel — displays detailed information about a selected entity.
 *
 * Wraps ConfigPanel for consistent styling and uses the tab registry
 * for extensible content tabs (Overview, History, Metadata, etc).
 *
 * Shows a tabbed interface with content filtered by entity layer type.
 * Includes an "Exit Entity View" button when in entity tracking mode.
 */

import React, { useState, useMemo, useCallback } from 'react';
import { Box, Button } from '@mui/material';
import { TrackChangesIcon, iconSizes } from '../../shared/icons';
import ConfigPanel, { type ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import { usePanelPosition } from '../../shared/layout';
import { useUIStore } from '@/app/store';
import { semanticColors, alpha, theme, DASHBOARD_TYPOGRAPHY } from '@respondent/core';
import { useEntityDetailWithFallback } from './useEntityDetailWithFallback';
import { getTabsForLayerType } from './tabs/tabRegistry';
// Import tab modules to trigger self-registration (must come after tabRegistry)
import './tabs/OverviewTab';
import './tabs/HistoryTab'; // registers as 'timeline' tab
import './tabs/MetadataTab';

interface EntityDetailPanelProps {
  entityId: string;
  layerId: string;
}

const EntityDetailPanel: React.FC<EntityDetailPanelProps> = ({ entityId, layerId }) => {
  const selectedEntityId = useUIStore((s) => s.selectedEntityId);
  const viewMode = useUIStore((s) => s.viewMode);
  const clearSelection = useUIStore((s) => s.clearSelection);
  const removeSelectedEntity = useUIStore((s) => s.removeSelectedEntity);

  const activeTabId = useUIStore((s) => s.entityViewState[entityId]?.activeTab ?? 'overview');
  const updateEntityViewState = useUIStore((s) => s.updateEntityViewState);
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');

  const { detail, isLoading } = useEntityDetailWithFallback(entityId, layerId);

  const layerType = detail?.entity?.layerType ?? layerId ?? 'unknown';

  const tabs = useMemo(() => getTabsForLayerType(layerType), [layerType]);

  const activeTab = useMemo(() => {
    const found = tabs.find((t) => t.id === activeTabId);
    return found ?? tabs[0];
  }, [tabs, activeTabId]);

  const handleClose = useCallback(() => {
    removeSelectedEntity(entityId);
  }, [removeSelectedEntity, entityId]);

  const handleExitEntityView = useCallback(() => {
    clearSelection();
  }, [clearSelection]);

  const handleTabClick = useCallback(
    (tabId: string) => {
      updateEntityViewState(entityId, { activeTab: tabId });
    },
    [entityId, updateEntityViewState],
  );

  const isPrimary = selectedEntityId === entityId;
  const pos = usePanelPosition({
    id: `entity-detail-${entityId}`,
    zone: 'top-right',
    width: 320,
    visible: true,
  });

  const entityName = detail?.entity?.name ?? entityId ?? '';

  const tabBar =
    tabs.length > 0 ? (
      <Box
        sx={{
          display: 'flex',
          borderBottom: '1px solid rgba(255, 255, 255, 0.08)',
        }}
      >
        {tabs.map((tab) => (
          <Box
            key={tab.id}
            data-testid={`entity-tab-${tab.id}`}
            data-active={activeTab?.id === tab.id}
            onClick={() => handleTabClick(tab.id)}
            sx={{
              px: 1.5,
              py: 0.75,
              cursor: 'pointer',
              fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
              fontWeight: 600,
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              color: activeTab?.id === tab.id ? 'primary.main' : 'text.secondary',
              borderBottom: activeTab?.id === tab.id ? '2px solid' : '2px solid transparent',
              borderColor: activeTab?.id === tab.id ? 'primary.main' : 'transparent',
              transition: 'color 0.15s, border-color 0.15s',
              '&:hover': {
                color: 'primary.main',
              },
            }}
          >
            {tab.label}
          </Box>
        ))}
      </Box>
    ) : null;

  const tabContent = activeTab ? (
    <activeTab.component
      entityId={entityId}
      layerType={layerType}
      detail={detail}
      isLoading={isLoading}
      // A minimized panel keeps its state but must not keep a camera refreshing.
      mediaActive={displayMode !== 'minimized'}
    />
  ) : null;

  const exitButton =
    isPrimary && viewMode === 'entity' ? (
      <Box sx={{ pt: 1 }}>
        <Button
          fullWidth
          size="small"
          variant="outlined"
          data-testid="entity-exit-view"
          onClick={handleExitEntityView}
          sx={{
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
            fontWeight: 600,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'primary.main',
            borderColor: semanticColors.primary.alpha30,
            '&:hover': {
              borderColor: 'primary.main',
              bgcolor: alpha(theme.palette.primary.main, 0.05),
            },
          }}
        >
          Exit Entity View
        </Button>
      </Box>
    ) : null;

  return (
    <ConfigPanel
      open={true}
      onClose={handleClose}
      title="Entity Detail"
      icon={<TrackChangesIcon size={iconSizes.sm} />}
      panelId={`entity-detail-${entityId}`}
      statusLabel={isPrimary && viewMode === 'entity' ? 'TRACKING' : 'SELECTED'}
      statusValue={entityName}
      top={80}
      right={pos.right}
      width={320}
      maxHeight="calc(100vh - 120px)"
      data-testid="panel-entity-detail"
      minimizable
      maximizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={entityName}
      stickyFooter={exitButton ?? undefined}
      stickyContent={tabBar ?? undefined}
    >
      {tabContent}
    </ConfigPanel>
  );
};

export default EntityDetailPanel;
