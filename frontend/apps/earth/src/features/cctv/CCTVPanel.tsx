import React from 'react';
import { Box, Typography, Divider, Switch, Button } from '@mui/material';
import { VideocamIcon, iconSizes } from '../../shared/icons';
import PanelShell from '../../shared/ui/PanelShell';
import EffectSlider from '../../shared/ui/EffectSlider';

const CCTVPanel: React.FC = () => {
  return (
    <PanelShell
      title="CCTV Mesh"
      icon={<VideocamIcon size={iconSizes.md} />}
      sx={{
        bottom: 16,
        left: 80,
        width: 320,
      }}
    >
      <Divider sx={{ mb: 2, borderColor: 'rgba(255, 255, 255, 0.1)' }} />

      {/* Controls */}
      <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
        <Button
          variant="contained"
          size="small"
          sx={{
            flex: 1,
            textTransform: 'uppercase',
            fontSize: '0.65rem',
          }}
        >
          Nearest
        </Button>
        <Button
          variant="outlined"
          size="small"
          sx={{
            flex: 1,
            textTransform: 'uppercase',
            fontSize: '0.65rem',
          }}
        >
          Prev
        </Button>
        <Button
          variant="outlined"
          size="small"
          sx={{
            flex: 1,
            textTransform: 'uppercase',
            fontSize: '0.65rem',
          }}
        >
          Next
        </Button>
      </Box>

      {/* City selector */}
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
          City
        </Typography>
        <Button
          fullWidth
          variant="outlined"
          size="small"
          sx={{
            textTransform: 'uppercase',
            fontSize: '0.7rem',
            justifyContent: 'space-between',
          }}
          endIcon="▼"
        >
          San Francisco
        </Button>
      </Box>

      {/* Toggles */}
      <Box sx={{ mb: 2 }}>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mb: 1,
          }}
        >
          <Typography variant="caption">CCTV ON</Typography>
          <Switch size="small" />
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mb: 1,
          }}
        >
          <Typography variant="caption">Coverage</Typography>
          <Switch size="small" />
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography variant="caption">Auto Hop</Typography>
          <Switch size="small" />
        </Box>
      </Box>

      {/* Projection controls */}
      <Box sx={{ mb: 2 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            mb: 1,
            display: 'block',
          }}
        >
          Projection
        </Typography>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            mb: 1,
          }}
        >
          <Typography variant="caption">Projection On</Typography>
          <Switch size="small" />
        </Box>
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography variant="caption">Auto Cal</Typography>
          <Switch size="small" />
        </Box>
      </Box>

      {/* Calibration sliders */}
      <Box>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            mb: 1,
            display: 'block',
          }}
        >
          Calibration
        </Typography>

        <EffectSlider
          label="Heading"
          value={0}
          onChange={() => {}}
          min={-180}
          max={180}
          suffix="°"
        />
        <EffectSlider label="Pitch" value={0} onChange={() => {}} min={-90} max={90} suffix="°" />
        <EffectSlider label="FOV" value={90} onChange={() => {}} min={10} max={180} suffix="°" />
        <EffectSlider
          label="Range"
          value={1000}
          onChange={() => {}}
          min={100}
          max={10000}
          suffix="m"
        />

        <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
          <Button
            variant="contained"
            size="small"
            sx={{
              flex: 1,
              textTransform: 'uppercase',
              fontSize: '0.65rem',
            }}
          >
            Save Cal
          </Button>
          <Button
            variant="outlined"
            size="small"
            sx={{
              flex: 1,
              textTransform: 'uppercase',
              fontSize: '0.65rem',
            }}
          >
            Reset Cal
          </Button>
        </Box>
      </Box>
    </PanelShell>
  );
};

export default CCTVPanel;
