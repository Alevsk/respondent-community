import React, { useState } from 'react';
import { Box, CircularProgress, IconButton, Tooltip } from '@mui/material';
import {
  LayersIcon,
  SpeedIcon,
  LayersClearIcon,
  VisibilityIcon,
  VisibilityOffIcon,
  iconSizes,
} from '../../shared/icons';
import ConfigPanel, { type ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import FilterOption from '../../shared/ui/FilterOption';
import { useUIStore, isIndicatorLayer } from '@/app/store';
import { usePanelPosition } from '../../shared/layout';
import { useLayers, useToggleLayer } from '../../shared/api/queries';
import { CanvasLayerIcon } from '../search/layerIcons';

// Get icon for layer type — uses canvas icons from the icon registry
// so icons are fully driven by declarative source YAML definitions.
function getLayerIcon(type: string, renderingMode?: string, color?: string): React.ReactNode {
  if (renderingMode === 'indicator') {
    return <SpeedIcon size={iconSizes.md} color="#ffcc00" />;
  }
  return <CanvasLayerIcon layerType={type} color={color} />;
}

// Format entity count with k suffix for thousands
function formatCount(count: number): string {
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}k`;
  }
  return count.toString();
}

interface DataLayersPanelProps {
  open: boolean;
  onClose: () => void;
  mobile?: boolean;
}

const DataLayersPanel: React.FC<DataLayersPanelProps> = ({ open, onClose, mobile }) => {
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');
  const pos = usePanelPosition({
    id: 'data-layers',
    zone: 'bottom-left',
    width: 300,
    visible: open && !mobile,
    baseMargin: 80,
  });
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const toggleLayer = useUIStore((s) => s.toggleLayer);
  const clearAllLayers = useUIStore((s) => s.clearAllLayers);
  const toggleLayersVisibility = useUIStore((s) => s.toggleLayersVisibility);
  const stashedLayers = useUIStore((s) => s.stashedLayers);
  const { data: layers, isLoading } = useLayers();
  const toggleLayerMutation = useToggleLayer();

  const handleToggleLayer = (layerId: string, enabled: boolean) => {
    toggleLayer(layerId);
    toggleLayerMutation.mutate({
      layer_id: layerId,
      enabled,
    });
  };

  const isHidden = stashedLayers !== null;

  const handleToggleVisibility = () => {
    if (isHidden) {
      // Restore: enable the stashed layers on the backend
      stashedLayers.forEach((layerId) => {
        toggleLayerMutation.mutate({ layer_id: layerId, enabled: true });
      });
    } else {
      // Hide: disable all enabled layers on the backend
      enabledLayers.forEach((layerId) => {
        toggleLayerMutation.mutate({ layer_id: layerId, enabled: false });
      });
    }
    toggleLayersVisibility();
  };

  const handleClearAll = () => {
    enabledLayers.forEach((layerId) => {
      toggleLayerMutation.mutate({ layer_id: layerId, enabled: false });
    });
    clearAllLayers();
  };

  const enabledCount = enabledLayers.length;
  const totalCount = layers?.length || 0;

  const content = isLoading ? (
    <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
      <CircularProgress size={24} />
    </Box>
  ) : (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      {layers?.map((layer) => {
        const isEnabled = enabledLayers.includes(layer.id);
        const indicator = isIndicatorLayer(layer.id);
        return (
          <FilterOption
            key={layer.id}
            id={layer.id}
            icon={getLayerIcon(layer.type, indicator ? 'indicator' : undefined, layer.color)}
            label={indicator ? `${layer.name} (Global)` : layer.name}
            description={
              indicator
                ? `Global indicator • ${layer.source}`
                : `${formatCount(layer.count)} entities • ${layer.source}`
            }
            active={isEnabled}
            onClick={() => handleToggleLayer(layer.id, !isEnabled)}
          />
        );
      })}

      {!layers || layers.length === 0 ? (
        <Box sx={{ py: 2, textAlign: 'center', color: 'text.secondary' }}>No layers available</Box>
      ) : null}
    </Box>
  );

  if (mobile) return content;

  return (
    <ConfigPanel
      open={open}
      onClose={onClose}
      title="Data Layers"
      icon={<LayersIcon size={iconSizes.md} />}
      panelId="data-layers"
      data-testid="panel-layers"
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={`Layers (${enabledCount}/${totalCount})`}
      statusLabel="ACTIVE LAYERS"
      statusValue={`${enabledCount} / ${totalCount}`}
      statusAction={
        <>
          <Tooltip title={isHidden ? 'Show layers' : 'Hide layers'}>
            <span>
              <IconButton
                size="small"
                onClick={handleToggleVisibility}
                disabled={enabledCount === 0 && !isHidden}
                aria-label={isHidden ? 'Show layers' : 'Hide layers'}
                data-testid="toggle-layers-visibility"
                sx={{
                  color: isHidden ? 'primary.main' : 'text.secondary',
                  p: 0.25,
                  '&:hover': { color: 'primary.main' },
                }}
              >
                {isHidden ? <VisibilityOffIcon size={14} /> : <VisibilityIcon size={14} />}
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="Clear all layers">
            <span>
              <IconButton
                size="small"
                onClick={handleClearAll}
                disabled={enabledCount === 0}
                aria-label="Clear all layers"
                data-testid="clear-all-layers"
                sx={{
                  color: 'text.secondary',
                  p: 0.25,
                  '&:hover': { color: 'primary.main' },
                }}
              >
                <LayersClearIcon size={14} />
              </IconButton>
            </span>
          </Tooltip>
        </>
      }
      footer="Toggle layers to visualize real-time data on the globe."
      bottom={80}
      left={pos.left}
      width={300}
    >
      {content}
    </ConfigPanel>
  );
};

export default DataLayersPanel;
