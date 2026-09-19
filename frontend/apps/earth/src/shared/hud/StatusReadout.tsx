import React, { useMemo } from 'react';
import { Box, Typography } from '@mui/material';
import { useViewerStore } from '../../features/globe/store';
import { useUIStore } from '@/app/store';

function getViewLevel(altitudeM: number): string {
  if (altitudeM > 10_000_000) return 'ORBITAL';
  if (altitudeM > 5_000_000) return 'GLOBAL';
  if (altitudeM > 1_000_000) return 'CONTINENTAL';
  if (altitudeM > 200_000) return 'REGIONAL';
  if (altitudeM > 50_000) return 'AREA';
  if (altitudeM > 10_000) return 'CITY';
  return 'STREET';
}

function formatAltitude(altitudeM: number): string {
  if (altitudeM >= 1_000_000) return `${(altitudeM / 1_000_000).toFixed(1)}M m`;
  if (altitudeM >= 1_000) return `${(altitudeM / 1_000).toFixed(0)} km`;
  return `${Math.round(altitudeM)} m`;
}

function formatEntityCount(count: number): string {
  if (count >= 1_000_000) return `${(count / 1_000_000).toFixed(1)}M`;
  if (count >= 1_000) return `${(count / 1_000).toFixed(1)}k`;
  return count.toString();
}

const StatusReadout: React.FC = () => {
  const camera = useViewerStore((s) => s.camera);
  const enabledLayers = useUIStore((s) => s.enabledLayers);
  const layerVersions = useUIStore((s) => s.layerVersions);

  const totalEntities = useMemo(() => {
    const layerEntities = useUIStore.getState().layerEntities;
    let count = 0;
    for (const [layerId, data] of layerEntities) {
      if (enabledLayers.includes(layerId)) {
        count += data.entityMap.size;
      }
    }
    return count;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layerVersions, enabledLayers]);

  const viewLevel = camera ? getViewLevel(camera.altitude) : 'INITIALIZING';
  const altitude = camera ? formatAltitude(camera.altitude) : '---';

  return (
    <Box
      sx={{
        position: 'absolute',
        bottom: 16,
        left: 80,
        zIndex: 100,
        display: 'flex',
        gap: 2,
      }}
    >
      <Box>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: '0.6rem',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
          }}
        >
          VIEW
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'primary.main',
            fontWeight: 600,
            fontSize: '0.7rem',
            display: 'block',
          }}
        >
          {viewLevel}
        </Typography>
      </Box>

      <Box>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: '0.6rem',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
          }}
        >
          ALTITUDE
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'primary.main',
            fontWeight: 600,
            fontSize: '0.7rem',
            display: 'block',
            fontFamily: 'monospace',
          }}
        >
          {altitude}
        </Typography>
      </Box>

      <Box>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: '0.6rem',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
          }}
        >
          LAYERS
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'primary.main',
            fontWeight: 600,
            fontSize: '0.7rem',
            display: 'block',
          }}
        >
          {enabledLayers.length} ACTIVE
        </Typography>
      </Box>

      <Box>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: '0.6rem',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
          }}
        >
          ENTITIES
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'primary.main',
            fontWeight: 600,
            fontSize: '0.7rem',
            display: 'block',
            fontFamily: 'monospace',
          }}
        >
          {totalEntities > 0 ? formatEntityCount(totalEntities) : 'NONE'}
        </Typography>
      </Box>
    </Box>
  );
};

export default StatusReadout;
