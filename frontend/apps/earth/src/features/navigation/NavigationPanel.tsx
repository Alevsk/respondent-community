import React, { useCallback, useRef, useState } from 'react';
import { Box, Button, Divider, Typography } from '@mui/material';
import {
  HomeIcon,
  NavigationIcon,
  ZoomInIcon,
  ZoomOutIcon,
  ExploreIcon,
  iconSizes,
} from '../../shared/icons';
import ConfigPanel, { type ConfigPanelDisplayMode } from '../../shared/ui/ConfigPanel';
import { usePanelPosition } from '../../shared/layout';
import { useViewerStore, DEFAULT_CAMERA } from '../globe/store';
import type { CameraState } from '../globe/store';
import { Cartesian3, Math as CesiumMath } from 'cesium';
import { alpha, theme } from '@respondent/core';

/** Format altitude for display: m / km / Mm */
function formatAltitude(meters: number): string {
  if (meters >= 1_000_000) return `${(meters / 1_000_000).toFixed(1)} Mm`;
  if (meters >= 1_000) return `${(meters / 1_000).toFixed(1)} km`;
  return `${Math.round(meters)} m`;
}

interface NavButtonProps {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  testId?: string;
}

const NavButton: React.FC<NavButtonProps> = ({ icon, label, onClick, testId }) => (
  <Button
    variant="outlined"
    size="small"
    startIcon={icon}
    onClick={onClick}
    data-testid={testId}
    sx={{
      color: 'rgba(255, 255, 255, 0.7)',
      borderColor: 'rgba(255, 255, 255, 0.15)',
      fontSize: '0.65rem',
      fontWeight: 600,
      textTransform: 'uppercase',
      letterSpacing: '0.06em',
      py: 0.8,
      '&:hover': {
        borderColor: 'primary.main',
        color: 'primary.main',
        bgcolor: alpha(theme.palette.primary.main, 0.08),
      },
    }}
  >
    {label}
  </Button>
);

interface CameraReadoutRowProps {
  label: string;
  value: string;
}

const CameraReadoutRow: React.FC<CameraReadoutRowProps> = ({ label, value }) => (
  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', py: 0.3 }}>
    <Typography
      variant="caption"
      sx={{ color: 'text.secondary', fontSize: '0.6rem', fontWeight: 600, letterSpacing: '0.08em' }}
    >
      {label}
    </Typography>
    <Typography
      variant="caption"
      sx={{ color: 'primary.main', fontFamily: 'monospace', fontWeight: 700, fontSize: '0.7rem' }}
    >
      {value}
    </Typography>
  </Box>
);

export interface NavigationPanelProps {
  open: boolean;
  onClose: () => void;
  mobile?: boolean;
}

const NavigationPanel: React.FC<NavigationPanelProps> = ({ open, onClose, mobile }) => {
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');
  const pos = usePanelPosition({
    id: 'nav',
    zone: 'bottom-right',
    width: 300,
    visible: open && !mobile,
  });
  const camera = useViewerStore((s) => s.camera);
  const viewer = useViewerStore((s) => s.viewer);

  // Keep a ref to avoid recreating callbacks on every camera change.
  const cameraRef = useRef<CameraState | null>(camera);
  cameraRef.current = camera;

  const altitudeDisplay = camera ? formatAltitude(camera.altitude) : '---';

  const handleResetView = useCallback(() => {
    if (!viewer) return;
    viewer.camera.flyTo({
      destination: Cartesian3.fromDegrees(
        DEFAULT_CAMERA.lon,
        DEFAULT_CAMERA.lat,
        DEFAULT_CAMERA.altitude,
      ),
      orientation: {
        heading: CesiumMath.toRadians(DEFAULT_CAMERA.heading),
        pitch: CesiumMath.toRadians(DEFAULT_CAMERA.pitch),
        roll: CesiumMath.toRadians(DEFAULT_CAMERA.roll),
      },
    });
  }, [viewer]);

  const handleZoomIn = useCallback(() => {
    if (!viewer || !cameraRef.current) return;
    viewer.camera.zoomIn(cameraRef.current.altitude * 0.3);
  }, [viewer]);

  const handleZoomOut = useCallback(() => {
    if (!viewer || !cameraRef.current) return;
    viewer.camera.zoomOut(cameraRef.current.altitude * 0.5);
  }, [viewer]);

  const handleNorthUp = useCallback(() => {
    const cam = cameraRef.current;
    if (!viewer || !cam) return;
    viewer.camera.flyTo({
      destination: Cartesian3.fromDegrees(cam.lon, cam.lat, cam.altitude),
      orientation: {
        heading: CesiumMath.toRadians(0),
        pitch: CesiumMath.toRadians(cam.pitch),
        roll: 0,
      },
    });
  }, [viewer]);

  const content = (
    <>
      {/* Camera Controls */}
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
        Camera Controls
      </Typography>

      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1, px: 0.5 }}>
        <NavButton
          icon={<HomeIcon size={18} />}
          label="Reset View"
          onClick={handleResetView}
          testId="nav-reset-view"
        />
        <NavButton
          icon={<ZoomInIcon size={18} />}
          label="Zoom In"
          onClick={handleZoomIn}
          testId="nav-zoom-in"
        />
        <NavButton
          icon={<ZoomOutIcon size={18} />}
          label="Zoom Out"
          onClick={handleZoomOut}
          testId="nav-zoom-out"
        />
        <NavButton
          icon={<NavigationIcon size={18} />}
          label="North Up"
          onClick={handleNorthUp}
          testId="nav-north-up"
        />
      </Box>

      <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)', my: 2 }} />

      {/* Camera Position Readout */}
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
          Camera Position
        </Typography>
        <CameraReadoutRow label="LAT" value={camera ? camera.lat.toFixed(4) + '\u00B0' : '---'} />
        <CameraReadoutRow label="LON" value={camera ? camera.lon.toFixed(4) + '\u00B0' : '---'} />
        <CameraReadoutRow
          label="HDG"
          value={camera ? camera.heading.toFixed(1) + '\u00B0' : '---'}
        />
      </Box>
    </>
  );

  if (mobile) return content;

  return (
    <ConfigPanel
      open={open}
      onClose={onClose}
      title="Navigation"
      icon={<ExploreIcon size={iconSizes.sm} />}
      panelId="navigation"
      data-testid="panel-navigation"
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={`Nav · ${altitudeDisplay}`}
      statusLabel="ALTITUDE"
      statusValue={altitudeDisplay}
      bottom={80}
      right={pos.right}
    >
      {content}
    </ConfigPanel>
  );
};

export default NavigationPanel;
