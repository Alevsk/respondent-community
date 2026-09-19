import React, { useState, useCallback } from 'react';
import { Box, Typography } from '@mui/material';
import { useUIStore } from '@/app/store';
import type { ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import type { IndicatorSnapshot } from '@/app/store';
import ConfigPanel from '../../shared/ui/ConfigPanel';
import MobileDrawer from '../../shared/ui/MobileDrawer';
import { usePanelPosition } from '../../shared/layout/usePanelPosition';
import { useResponsive } from '@respondent/core';
import IndicatorGauge from './IndicatorGauge';
import { levelColor } from './indicatorColors';
import { timeAgo } from './indicatorUtils';

const PANEL_WIDTH = 320;

/** Shared content for an indicator layer (used by both desktop panel and mobile drawer). */
const IndicatorLayerContent: React.FC<{ snap: IndicatorSnapshot }> = ({ snap }) => (
  <Box data-testid={`indicator-content-${snap.layerId}`}>
    {/* Status row */}
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        mb: 0.75,
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
        <Box
          sx={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            backgroundColor: levelColor(snap.overallLevel),
          }}
        />
        <Typography
          sx={{
            fontSize: '0.6rem',
            fontWeight: 600,
            color: levelColor(snap.overallLevel),
            fontFamily: 'monospace',
          }}
        >
          {snap.summary || 'Quiet'}
        </Typography>
      </Box>
    </Box>

    {/* Value gauges */}
    <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
      {snap.values.map((v) => (
        <IndicatorGauge key={v.key} value={v} />
      ))}
    </Box>

    {/* Timestamp */}
    {snap.timestampMs > 0 && (
      <Typography
        sx={{
          fontSize: '0.5rem',
          color: 'text.secondary',
          mt: 0.5,
          fontFamily: 'monospace',
        }}
      >
        Updated {timeAgo(snap.timestampMs)}
      </Typography>
    )}
  </Box>
);

/** Desktop: renders a single indicator layer as its own floating ConfigPanel. */
const IndicatorLayerPanel: React.FC<{ snap: IndicatorSnapshot }> = ({ snap }) => {
  const toggleLayer = useUIStore((s) => s.toggleLayer);
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');

  const panelId = `indicator-${snap.layerId}`;
  const title = snap.layerName.toUpperCase();

  const pos = usePanelPosition({
    id: panelId,
    zone: 'top-right',
    width: PANEL_WIDTH,
    visible: true,
  });

  return (
    <ConfigPanel
      data-testid={`indicator-panel-${snap.layerId}`}
      open
      onClose={() => toggleLayer(snap.layerId)}
      title={title}
      panelId={panelId}
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={`${title} · ${snap.summary || 'Quiet'}`}
      top={48}
      right={pos.right}
      width={PANEL_WIDTH}
      hideHeaderDivider
    >
      <IndicatorLayerContent snap={snap} />
    </ConfigPanel>
  );
};

/**
 * Mobile: compact chips that open a bottom drawer with full gauge content.
 *
 * Unlike Layers/Settings/Nav which use the centralized activeMobileDrawer store
 * (routed through AppShell's MobileDrawerPanels), indicator chips manage their
 * own local drawer state. This is intentional: indicator chips are ephemeral
 * detail-on-tap overlays — not full-featured panels reachable from bottom nav.
 */
const MobileIndicatorChips: React.FC<{ indicators: IndicatorSnapshot[] }> = ({ indicators }) => {
  const toggleLayer = useUIStore((s) => s.toggleLayer);
  const [expandedLayerId, setExpandedLayerId] = useState<string | null>(null);

  const expandedSnap = expandedLayerId
    ? (indicators.find((s) => s.layerId === expandedLayerId) ?? null)
    : null;

  const handleClose = useCallback(() => setExpandedLayerId(null), []);
  const handleOpen = useCallback(() => {}, []);

  return (
    <>
      {/* Chip strip below mobile top bar */}
      <Box
        data-testid="mobile-indicator-chips"
        sx={{
          position: 'absolute',
          top: 'calc(var(--sat) + 48px)',
          left: 8,
          right: 8,
          zIndex: 100,
          display: 'flex',
          gap: 0.75,
          flexWrap: 'wrap',
          pointerEvents: 'none',
        }}
      >
        {indicators.map((snap) => (
          <Box
            key={snap.layerId}
            data-testid={`indicator-chip-${snap.layerId}`}
            onClick={() => setExpandedLayerId(snap.layerId)}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 0.5,
              px: 1,
              py: 0.5,
              borderRadius: 1,
              bgcolor: 'rgba(5, 5, 5, 0.85)',
              border: '1px solid rgba(255, 255, 255, 0.08)',
              backdropFilter: 'blur(8px)',
              cursor: 'pointer',
              pointerEvents: 'auto',
              '&:active': { bgcolor: 'rgba(20, 20, 20, 0.95)' },
            }}
          >
            <Box
              sx={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                backgroundColor: levelColor(snap.overallLevel),
                flexShrink: 0,
              }}
            />
            <Typography
              sx={{
                fontSize: '0.6rem',
                fontWeight: 700,
                letterSpacing: '0.08em',
                color: 'text.primary',
                fontFamily: 'monospace',
                textTransform: 'uppercase',
                whiteSpace: 'nowrap',
              }}
            >
              {snap.layerName}
            </Typography>
            <Typography
              sx={{
                fontSize: '0.55rem',
                fontWeight: 600,
                color: levelColor(snap.overallLevel),
                fontFamily: 'monospace',
                whiteSpace: 'nowrap',
              }}
            >
              {snap.summary || 'Quiet'}
            </Typography>
          </Box>
        ))}
      </Box>

      {/* Expanded drawer for selected indicator */}
      {expandedSnap && (
        <MobileDrawer
          open={!!expandedSnap}
          onClose={handleClose}
          onOpen={handleOpen}
          title={expandedSnap.layerName}
          heightPercent={45}
        >
          <IndicatorLayerContent snap={expandedSnap} />
          <Box sx={{ mt: 2, display: 'flex', justifyContent: 'center' }}>
            <Typography
              onClick={() => {
                toggleLayer(expandedSnap.layerId);
                handleClose();
              }}
              sx={{
                fontSize: '0.65rem',
                fontWeight: 600,
                color: 'error.main',
                cursor: 'pointer',
                letterSpacing: '0.08em',
                textTransform: 'uppercase',
                py: 1,
                px: 2,
              }}
            >
              Disable Layer
            </Typography>
          </Box>
        </MobileDrawer>
      )}
    </>
  );
};

const IndicatorHUD: React.FC = () => {
  const { isMobile } = useResponsive();
  const indicators = useUIStore((s) => s.indicators);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const indicatorLayerIds = useUIStore((s) => s.indicatorLayerIds);

  const visibleIndicators = Object.values(indicators).filter(
    (snap) => enabledLayers.includes(snap.layerId) && indicatorLayerIds.has(snap.layerId),
  );

  if (visibleIndicators.length === 0) return null;

  if (isMobile) {
    return <MobileIndicatorChips indicators={visibleIndicators} />;
  }

  return (
    <>
      {visibleIndicators.map((snap) => (
        <IndicatorLayerPanel key={snap.layerId} snap={snap} />
      ))}
    </>
  );
};

export default IndicatorHUD;
