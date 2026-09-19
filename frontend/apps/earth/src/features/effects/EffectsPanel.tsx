import React from 'react';
import { Box, Typography, Divider, Switch, Select, MenuItem, Button } from '@mui/material';
import { CleaningServicesIcon as CleanUpIcon, TuneIcon, iconSizes } from '../../shared/icons';
import ConfigPanel from '../../shared/ui/ConfigPanel';
import EffectSlider from '../../shared/ui/EffectSlider';
import { useUIStore } from '@/app/store';
import { usePanelPosition } from '../../shared/layout';

const EffectsPanel: React.FC = () => {
  const pos = usePanelPosition({ id: 'effects', zone: 'top-right', width: 280, visible: true });
  const panelCollapsed = useUIStore((s) => s.panelCollapsed);
  const detectMode = useUIStore((s) => s.detectMode);
  const setDetectMode = useUIStore((s) => s.setDetectMode);
  const density = useUIStore((s) => s.density);
  const setDensity = useUIStore((s) => s.setDensity);
  const cleanUI = useUIStore((s) => s.cleanUI);
  const setCleanUI = useUIStore((s) => s.setCleanUI);

  const togglePanel = useUIStore((s) => s.togglePanel);

  return (
    <ConfigPanel
      panelId="effects"
      title="Effects & HUD"
      icon={<TuneIcon size={iconSizes.md} />}
      open={true}
      onClose={() => {}}
      minimizable={true}
      displayMode={panelCollapsed.effects ? 'minimized' : 'normal'}
      onDisplayModeChange={() => togglePanel('effects')}
      width={280}
      top={80}
      right={pos.right}
      maxHeight="calc(100vh - 100px)"
      hideCloseButton={true}
    >
      <Divider sx={{ mb: 2, borderColor: 'rgba(255, 255, 255, 0.1)' }} />

      {/* Bloom toggle */}
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          mb: 2,
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
          }}
        >
          Bloom
        </Typography>
        <Switch
          checked={detectMode === 'panoptic'}
          onChange={(_, checked) => setDetectMode(checked ? 'panoptic' : 'sparse')}
          size="small"
        />
      </Box>

      {/* Layout selector */}
      <Box sx={{ mb: 2 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            display: 'block',
            mb: 1,
          }}
        >
          Layout
        </Typography>
        <Select
          fullWidth
          size="small"
          defaultValue="tactical"
          sx={{
            fontSize: '0.75rem',
            '& .MuiSelect-select': {
              py: 0.75,
            },
          }}
        >
          <MenuItem value="tactical">Tactical</MenuItem>
          <MenuItem value="minimal">Minimal</MenuItem>
          <MenuItem value="detailed">Detailed</MenuItem>
        </Select>
      </Box>

      {/* Detect mode */}
      <Box sx={{ mb: 2 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            display: 'block',
            mb: 1,
          }}
        >
          Detect Mode
        </Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button
            size="small"
            variant={detectMode === 'panoptic' ? 'contained' : 'outlined'}
            onClick={() => setDetectMode('panoptic')}
            sx={{
              flex: 1,
              fontSize: '0.65rem',
              py: 0.5,
            }}
          >
            Panoptic
          </Button>
          <Button
            size="small"
            variant={detectMode === 'sparse' ? 'contained' : 'outlined'}
            onClick={() => setDetectMode('sparse')}
            sx={{
              flex: 1,
              fontSize: '0.65rem',
              py: 0.5,
            }}
          >
            Sparse
          </Button>
        </Box>
      </Box>

      {/* Density slider */}
      <EffectSlider
        label="Density"
        value={density}
        onChange={setDensity}
        min={0}
        max={100}
        step={1}
        defaultValue={50}
      />

      <Divider sx={{ my: 2, borderColor: 'rgba(255, 255, 255, 0.1)' }} />

      {/* Clean UI button */}
      <Button
        fullWidth
        startIcon={<CleanUpIcon size={18} />}
        variant={cleanUI ? 'contained' : 'outlined'}
        onClick={() => setCleanUI(!cleanUI)}
        sx={{
          textTransform: 'uppercase',
          fontSize: '0.7rem',
          fontWeight: 600,
          letterSpacing: '0.05em',
          py: 1,
        }}
      >
        {cleanUI ? 'Show UI' : 'Clean UI'}
      </Button>
    </ConfigPanel>
  );
};

export default EffectsPanel;
