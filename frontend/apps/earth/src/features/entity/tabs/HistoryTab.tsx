/**
 * TimelineTab — Timeline view of entity observation history.
 *
 * Owns all view mode state and the unified toolbar for ALL entity types:
 *   [Map*] [Table] [Chart]    [Copy]    [Trails switch*] [Isolate switch*]
 * (* = moving entities only)
 *
 * ObservationDataView is a pure renderer — it receives data and viewMode,
 * renders output, and owns no state of its own.
 *
 * Branches rendering based on icon.interpolation from the layer display config:
 * - Moving entities (interpolation: true): trajectory / table / chart
 * - Stationary entities (interpolation: false): table / chart
 *
 * Consumes useEntityTrail for progressive data loading. Trail highlight state
 * is shared via the Zustand store so the globe trail renderer can show the
 * selected point.
 */

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Box, IconButton, Menu, MenuItem, Switch, Tooltip, Typography } from '@mui/material';
import { Copy, LineChart as LineChartIcon, Map, Table2 } from 'lucide-react';
import { BORDER, DASHBOARD_TYPOGRAPHY, HOVER } from '@respondent/core';
import { RouteIcon, VisibilityOffIcon } from '../../../shared/icons';
import { Cartesian3 } from 'cesium';
import { registerTab, type EntityTabProps } from './tabRegistry';
import { useEntityTrail, type TrailPoint } from '../hooks/useEntityTrail';
import { useUIStore } from '@/app/store';
import { useViewerStore } from '../../globe/store';
import { switchTestId } from '@/shared/ui/switchTestId';
import TimelineEventList from './TimelineEventList';
import ObservationDataView, {
  buildCsvString,
  buildJsonString,
  buildMarkdownString,
  collectMetadataKeys,
} from './ObservationDataView';

/** Altitude offset above the point when flying to a position. */
const FLY_TO_ALTITUDE_OFFSET = 50_000;

/**
 * Check layer display config to determine if this entity type moves through space.
 * Uses imperative getState() because layer metadata is loaded once at bootstrap
 * and is stable — no need for a reactive selector. Same pattern as
 * BillboardLayerRenderer's supportsInterpolation check.
 */
function isMovingEntity(layerType: string): boolean {
  const layer = useUIStore.getState().layers[layerType];
  return layer?.displayConfig?.icon?.interpolation ?? false;
}

type ViewMode = 'trajectory' | 'table' | 'chart';

const TimelineTab: React.FC<EntityTabProps> = ({ entityId, layerType }) => {
  const trail = useEntityTrail(entityId);
  const highlightedTs = useUIStore((s) => s.entityViewState[entityId]?.trailHighlight ?? null);
  const updateEntityViewState = useUIStore((s) => s.updateEntityViewState);

  const viewMode = useUIStore((s) => s.viewMode);
  const showTrails = useUIStore((s) => s.entityViewState[entityId]?.showTrails ?? true);
  const isolateEntity = useUIStore((s) => s.entityViewState[entityId]?.isolateEntity ?? true);

  const moving = isMovingEntity(layerType);

  const [localViewMode, setLocalViewMode] = useState<ViewMode>(moving ? 'trajectory' : 'table');

  const [copyMenuAnchor, setCopyMenuAnchor] = useState<HTMLElement | null>(null);

  const metaKeys = useMemo(() => collectMetadataKeys(trail.points), [trail.points]);

  useEffect(() => {
    const vs = useUIStore.getState().entityViewState[entityId];
    if (vs?.showTrails === undefined || vs?.isolateEntity === undefined) {
      updateEntityViewState(entityId, {
        showTrails: vs?.showTrails ?? true,
        isolateEntity: vs?.isolateEntity ?? true,
      });
    }
  }, [entityId, updateEntityViewState]);

  const handleToggleTrails = useCallback(
    (_: React.ChangeEvent<HTMLInputElement>, checked: boolean) => {
      updateEntityViewState(entityId, { showTrails: checked });
    },
    [entityId, updateEntityViewState],
  );

  const handleToggleIsolate = useCallback(
    (_: React.ChangeEvent<HTMLInputElement>, checked: boolean) => {
      updateEntityViewState(entityId, { isolateEntity: checked });
    },
    [entityId, updateEntityViewState],
  );

  const handleHighlight = useCallback(
    (ts: number | null) => {
      updateEntityViewState(entityId, { trailHighlight: ts });
      if (ts === null && viewMode === 'entity') {
        useViewerStore.getState().setTrackingOverridePosition(null);
      }
    },
    [entityId, updateEntityViewState, viewMode],
  );

  const handleFlyTo = useCallback(
    (point: TrailPoint) => {
      const viewer = useViewerStore.getState().viewer;
      if (!viewer || viewer.isDestroyed()) return;

      const destination = Cartesian3.fromDegrees(point.lon, point.lat, point.altitudeM);

      if (viewMode === 'entity') {
        useViewerStore.getState().setTrackingOverridePosition(destination);
      } else {
        viewer.camera.flyTo({
          destination: Cartesian3.fromDegrees(
            point.lon,
            point.lat,
            point.altitudeM + FLY_TO_ALTITUDE_OFFSET,
          ),
          duration: 1.5,
        });
      }
    },
    [viewMode],
  );

  const handleCopyMenuOpen = useCallback((e: React.MouseEvent<HTMLElement>) => {
    setCopyMenuAnchor(e.currentTarget);
  }, []);

  const handleCopyMenuClose = useCallback(() => {
    setCopyMenuAnchor(null);
  }, []);

  const handleCopy = useCallback(
    (format: 'csv' | 'json' | 'markdown') => {
      handleCopyMenuClose();
      const text =
        format === 'csv'
          ? buildCsvString(trail.points, metaKeys)
          : format === 'json'
            ? buildJsonString(trail.points, metaKeys)
            : buildMarkdownString(trail.points, metaKeys);
      navigator.clipboard.writeText(text).catch(() => {
        console.error('Failed to copy to clipboard');
      });
    },
    [trail.points, metaKeys, handleCopyMenuClose],
  );

  const showCopy = localViewMode !== 'trajectory' && trail.points.length > 0;

  const iconButtonSx = (active: boolean) => ({
    p: 0.5,
    color: active ? 'primary.main' : 'text.secondary',
    borderRadius: 1,
    '&:hover': { bgcolor: HOVER.button, color: 'primary.main' },
  });

  const switchSx = {
    '& .MuiSwitch-switchBase.Mui-checked': {
      color: 'primary.main',
    },
    '& .MuiSwitch-switchBase.Mui-checked + .MuiSwitch-track': {
      backgroundColor: 'primary.main',
    },
  };

  const toggleLabelSx = {
    fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
    fontWeight: 600,
    letterSpacing: '0.05em',
    textTransform: 'uppercase',
    color: 'text.secondary',
    userSelect: 'none',
  } as const;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* Sticky controls area */}
      <Box sx={{ flexShrink: 0 }}>
        {/* Row 1: Trails + Isolate switches (moving entities only) */}
        {moving && (
          <Box
            sx={{
              display: 'flex',
              gap: 1.5,
              pb: 0.75,
              mb: 0.75,
              borderBottom: `1px solid ${BORDER.subtle}`,
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box component="span" sx={{ display: 'inline-flex', color: 'text.secondary' }}>
                <RouteIcon size={12} />
              </Box>
              <Typography variant="caption" sx={toggleLabelSx}>
                Trails
              </Typography>
              <Switch
                inputProps={switchTestId('history-switch-trails')}
                size="small"
                checked={showTrails}
                onChange={handleToggleTrails}
                sx={switchSx}
              />
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box component="span" sx={{ display: 'inline-flex', color: 'text.secondary' }}>
                <VisibilityOffIcon size={12} />
              </Box>
              <Typography variant="caption" sx={toggleLabelSx}>
                Isolate
              </Typography>
              <Switch
                inputProps={switchTestId('history-switch-isolate')}
                size="small"
                checked={isolateEntity}
                onChange={handleToggleIsolate}
                sx={switchSx}
              />
            </Box>
          </Box>
        )}

        {/* Row 2: View mode icons (left) + contextual actions (right) */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            pb: 0.75,
            mb: 0.75,
            borderBottom: `1px solid ${BORDER.subtle}`,
          }}
        >
          {/* Left: view mode toggle */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
            {moving && (
              <Tooltip title="Trajectory">
                <IconButton
                  size="small"
                  onClick={() => setLocalViewMode('trajectory')}
                  aria-label="Trajectory view"
                  aria-pressed={localViewMode === 'trajectory'}
                  sx={iconButtonSx(localViewMode === 'trajectory')}
                >
                  <Map size={14} />
                </IconButton>
              </Tooltip>
            )}
            <Tooltip title="Table view">
              <IconButton
                size="small"
                onClick={() => setLocalViewMode('table')}
                aria-label="Table view"
                aria-pressed={localViewMode === 'table'}
                sx={iconButtonSx(localViewMode === 'table')}
              >
                <Table2 size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title="Chart view">
              <IconButton
                size="small"
                onClick={() => setLocalViewMode('chart')}
                aria-label="Chart view"
                aria-pressed={localViewMode === 'chart'}
                sx={iconButtonSx(localViewMode === 'chart')}
              >
                <LineChartIcon size={14} />
              </IconButton>
            </Tooltip>
          </Box>

          {/* Right: contextual actions */}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
            {showCopy && (
              <Tooltip title="Copy data">
                <IconButton
                  size="small"
                  onClick={handleCopyMenuOpen}
                  aria-label="Copy data"
                  sx={iconButtonSx(Boolean(copyMenuAnchor))}
                >
                  <Copy size={14} />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        </Box>
      </Box>

      {/* Copy context menu */}
      <Menu
        anchorEl={copyMenuAnchor}
        open={Boolean(copyMenuAnchor)}
        onClose={handleCopyMenuClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{
          paper: {
            sx: {
              backgroundColor: 'rgba(10, 10, 10, 0.97)',
              border: `1px solid ${BORDER.subtle}`,
              borderRadius: 1,
              minWidth: 148,
            },
          },
        }}
      >
        <MenuItem
          data-testid="history-copy-csv"
          onClick={() => handleCopy('csv')}
          sx={{ fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize, py: 0.75 }}
        >
          Copy as CSV
        </MenuItem>
        <MenuItem
          data-testid="history-copy-json"
          onClick={() => handleCopy('json')}
          sx={{ fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize, py: 0.75 }}
        >
          Copy as JSON
        </MenuItem>
        <MenuItem
          data-testid="history-copy-md"
          onClick={() => handleCopy('markdown')}
          sx={{ fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize, py: 0.75 }}
        >
          Copy as Markdown
        </MenuItem>
      </Menu>

      {/* Content */}
      <Box sx={{ flex: 1, minHeight: 0 }}>
        {localViewMode === 'trajectory' ? (
          <TimelineEventList
            points={trail.points}
            highlightedTs={highlightedTs}
            onHighlight={handleHighlight}
            onFlyTo={handleFlyTo}
            trackingMode={viewMode === 'entity'}
            hasMore={trail.hasMore}
            isLoading={trail.isLoading}
            onLoadMore={trail.fetchMore}
          />
        ) : (
          <ObservationDataView
            points={trail.points}
            hasMore={trail.hasMore}
            isLoading={trail.isLoading}
            onLoadMore={trail.fetchMore}
            viewMode={localViewMode === 'chart' ? 'chart' : 'table'}
          />
        )}
      </Box>
    </Box>
  );
};

registerTab({
  id: 'timeline',
  label: 'Timeline',
  priority: 10,
  component: TimelineTab,
});

export default TimelineTab;
