import React, { useState } from 'react';
import { Box, Divider, Slider, Switch, Tooltip, Typography } from '@mui/material';
import {
  SettingsIcon,
  EffectNormalIcon,
  EffectCrtIcon,
  EffectNvgIcon,
  EffectFlirIcon,
  iconSizes,
} from '../../shared/icons';
import ConfigPanel, { type ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import FilterOption from '../../shared/ui/FilterOption';
import { useUIStore, FilterPreset } from '@/app/store';
import { usePanelPosition } from '../../shared/layout';
import { switchTestId } from '@/shared/ui/switchTestId';

const FILTER_OPTIONS: {
  id: FilterPreset;
  label: string;
  icon: React.ReactNode;
  description: string;
}[] = [
  {
    id: 'NORMAL',
    label: 'Normal',
    icon: <EffectNormalIcon size={iconSizes.md} />,
    description: 'Standard view with subtle vignette',
  },
  {
    id: 'CRT',
    label: 'CRT Monitor',
    icon: <EffectCrtIcon size={iconSizes.md} />,
    description: 'Retro CRT with scanlines & glow',
  },
  {
    id: 'NVG',
    label: 'Night Vision',
    icon: <EffectNvgIcon size={iconSizes.md} />,
    description: 'Green monochrome NVG simulation',
  },
  {
    id: 'FLIR',
    label: 'Thermal / FLIR',
    icon: <EffectFlirIcon size={iconSizes.md} />,
    description: 'Infrared thermal imaging view',
  },
];

const HAS_ION_TOKEN = !!import.meta.env.VITE_CESIUM_ION_TOKEN;

const SLIDER_MIN = 100;
const SLIDER_MAX = 10000;
const SLIDER_STEP = 100;

export interface SettingsPanelProps {
  open: boolean;
  onClose: () => void;
  mobile?: boolean;
}

const SettingsPanel: React.FC<SettingsPanelProps> = ({ open, onClose, mobile }) => {
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');
  const pos = usePanelPosition({
    id: 'settings',
    zone: 'bottom-right',
    width: 300,
    visible: open && !mobile,
  });
  const activePreset = useUIStore((s) => s.activePreset);
  const setActivePreset = useUIStore((s) => s.setActivePreset);
  const maxEntities = useUIStore((s) => s.maxEntities);
  const setMaxEntities = useUIStore((s) => s.setMaxEntities);
  const showOccluded = useUIStore((s) => s.showOccluded);
  const setShowOccluded = useUIStore((s) => s.setShowOccluded);
  const showGeoLabels = useUIStore((s) => s.showGeoLabels);
  const setShowGeoLabels = useUIStore((s) => s.setShowGeoLabels);
  const show3DBuildings = useUIStore((s) => s.show3DBuildings);
  const setShow3DBuildings = useUIStore((s) => s.setShow3DBuildings);
  const smoothMotion = useUIStore((s) => s.smoothMotion);
  const setSmoothMotion = useUIStore((s) => s.setSmoothMotion);
  const cinematicDrift = useUIStore((s) => s.cinematicDrift);
  const setCinematicDrift = useUIStore((s) => s.setCinematicDrift);
  const spatialAggregation = useUIStore((s) => s.spatialAggregation);
  const setSpatialAggregation = useUIStore((s) => s.setSpatialAggregation);

  const activeFilterLabel =
    FILTER_OPTIONS.find((opt) => opt.id === activePreset)?.label || activePreset;

  const handleFilterSelect = (preset: FilterPreset) => {
    setActivePreset(preset);
  };

  const displayValue = maxEntities >= SLIDER_MAX ? 'MAX' : String(maxEntities);

  const content = (
    <>
      {/* Display Filter Section */}
      <Typography
        variant="caption"
        sx={{
          fontWeight: 600,
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          color: 'text.secondary',
          fontSize: '0.6rem',
          display: 'block',
          mb: 1,
          px: 0.5,
        }}
      >
        Display Filter
      </Typography>

      {FILTER_OPTIONS.map((option) => (
        <FilterOption
          key={option.id}
          id={option.id}
          icon={option.icon}
          label={option.label}
          description={option.description}
          active={activePreset === option.id}
          onClick={() => handleFilterSelect(option.id)}
        />
      ))}

      <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)', my: 2 }} />

      {/* Render Limit Section */}
      <Box sx={{ px: 0.5 }}>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mb: 1,
          }}
        >
          <Typography
            variant="caption"
            sx={{
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              color: 'text.secondary',
              fontSize: '0.6rem',
            }}
          >
            Render Limit
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: 'primary.main',
              fontWeight: 700,
              fontFamily: 'monospace',
            }}
          >
            {displayValue}
          </Typography>
        </Box>
        <Box sx={{ px: 1 }}>
          <Slider
            data-testid="settings-render-limit-slider"
            value={maxEntities}
            onChange={(_, newValue) => setMaxEntities(newValue as number)}
            min={SLIDER_MIN}
            max={SLIDER_MAX}
            step={SLIDER_STEP}
            sx={{
              height: 4,
              '& .MuiSlider-thumb': {
                width: 16,
                height: 16,
              },
              '& .MuiSlider-track': {
                border: 'none',
              },
              '& .MuiSlider-rail': {
                opacity: 0.3,
              },
            }}
          />
        </Box>
      </Box>

      <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)', my: 2 }} />

      {/* Visibility Section */}
      <Box sx={{ px: 0.5 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            color: 'text.secondary',
            fontSize: '0.6rem',
            display: 'block',
            mb: 1,
          }}
        >
          Visibility
        </Typography>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography variant="body2" sx={{ color: 'text.primary' }}>
            Show Occluded
          </Typography>
          <Switch
            inputProps={switchTestId('settings-switch-occluded')}
            size="small"
            checked={showOccluded}
            onChange={(_, checked) => setShowOccluded(checked)}
          />
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mt: 1,
          }}
        >
          <Typography variant="body2" sx={{ color: 'text.primary' }}>
            Geo Labels
          </Typography>
          <Switch
            inputProps={switchTestId('settings-switch-geo-labels')}
            size="small"
            checked={showGeoLabels}
            onChange={(_, checked) => setShowGeoLabels(checked)}
          />
        </Box>
        <Tooltip
          title={HAS_ION_TOKEN ? '' : 'Requires VITE_CESIUM_ION_TOKEN'}
          placement="left"
          arrow
        >
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              mt: 1,
              opacity: HAS_ION_TOKEN ? 1 : 0.4,
            }}
          >
            <Typography variant="body2" sx={{ color: 'text.primary' }}>
              3D Buildings
            </Typography>
            <Switch
              inputProps={switchTestId('settings-switch-3d-buildings')}
              size="small"
              checked={show3DBuildings}
              disabled={!HAS_ION_TOKEN}
              onChange={(_, checked) => setShow3DBuildings(checked)}
            />
          </Box>
        </Tooltip>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mt: 1,
          }}
        >
          <Typography variant="body2" sx={{ color: 'text.primary' }}>
            Smooth Motion
          </Typography>
          <Switch
            inputProps={switchTestId('settings-switch-smooth-motion')}
            size="small"
            checked={smoothMotion}
            onChange={(_, checked) => setSmoothMotion(checked)}
          />
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mt: 1,
          }}
        >
          <Typography variant="body2" sx={{ color: 'text.primary' }}>
            Cinematic Drift
          </Typography>
          <Switch
            inputProps={switchTestId('settings-switch-cinematic-drift')}
            size="small"
            checked={cinematicDrift}
            onChange={(_, checked) => setCinematicDrift(checked)}
          />
        </Box>
        <Tooltip
          title="Group nearby entities into clusters; click a cluster to list its members"
          placement="left"
          arrow
        >
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              mt: 1,
            }}
          >
            <Typography variant="body2" sx={{ color: 'text.primary' }}>
              Spatial Aggregation
            </Typography>
            <Switch
              inputProps={switchTestId('settings-switch-spatial-aggregation')}
              size="small"
              checked={spatialAggregation}
              onChange={(_, checked) => setSpatialAggregation(checked)}
            />
          </Box>
        </Tooltip>
      </Box>
    </>
  );

  if (mobile) return content;

  return (
    <ConfigPanel
      open={open}
      onClose={onClose}
      title="Settings"
      icon={<SettingsIcon size={iconSizes.sm} />}
      panelId="settings"
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={activeFilterLabel}
      statusLabel="ACTIVE FILTER"
      statusValue={activeFilterLabel.toUpperCase()}
      bottom={80}
      right={pos.right}
      data-testid="panel-settings"
    >
      {content}
    </ConfigPanel>
  );
};

export default SettingsPanel;
