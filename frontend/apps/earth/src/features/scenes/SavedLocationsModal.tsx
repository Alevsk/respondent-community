import React from 'react';
import { Box, Typography, Chip } from '@mui/material';
import { LocationOnIcon, iconSizes } from '../../shared/icons';
import PanelShell from '../../shared/ui/PanelShell';
import { semanticColors } from '@respondent/core';

const cities = [
  { id: 'sf', name: 'San Francisco', region: 'US' },
  { id: 'ny', name: 'New York', region: 'US' },
  { id: 'ldn', name: 'London', region: 'Europe' },
  { id: 'tky', name: 'Tokyo', region: 'Asia' },
];

const landmarks = [
  { id: 'ggb', name: 'Golden Gate Bridge', cityId: 'sf', lat: 37.8199, lon: -122.4783 },
  { id: 'sf-dt', name: 'Downtown SF', cityId: 'sf', lat: 37.7749, lon: -122.4194 },
  { id: 'sf-air', name: 'SFO Airport', cityId: 'sf', lat: 37.6213, lon: -122.379 },
];

const SavedLocationsModal: React.FC = () => {
  const selectedCity = 'sf';
  const filteredLandmarks = landmarks.filter((l) => l.cityId === selectedCity);

  const handleCitySelect = (cityId: string) => {
    // TODO: Implement city selection
    console.log('Select city:', cityId);
  };

  const handleLandmarkSelect = (landmark: (typeof landmarks)[0]) => {
    // TODO: Fly to landmark
    console.log('Fly to:', landmark);
  };

  return (
    <PanelShell
      title="Locations"
      icon={<LocationOnIcon size={iconSizes.md} />}
      sx={{
        bottom: 140,
        left: '50%',
        transform: 'translateX(-50%)',
        width: 500,
      }}
    >
      {/* City chips */}
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
          Cities
        </Typography>
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {cities.map((city) => (
            <Chip
              key={city.id}
              label={city.name}
              size="small"
              onClick={() => handleCitySelect(city.id)}
              sx={{
                fontSize: '0.65rem',
                bgcolor: city.id === selectedCity ? 'primary.main' : 'rgba(255, 255, 255, 0.1)',
                color: city.id === selectedCity ? '#000' : 'text.primary',
                cursor: 'pointer',
                '&:hover': {
                  bgcolor: semanticColors.primary.alpha20,
                },
              }}
            />
          ))}
        </Box>
      </Box>

      {/* Landmark chips */}
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
          Landmarks
        </Typography>
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {filteredLandmarks.map((landmark) => (
            <Chip
              key={landmark.id}
              label={landmark.name}
              size="small"
              onClick={() => handleLandmarkSelect(landmark)}
              sx={{
                fontSize: '0.65rem',
                bgcolor: 'rgba(255, 255, 255, 0.1)',
                color: 'text.primary',
                cursor: 'pointer',
                '&:hover': {
                  bgcolor: semanticColors.primary.alpha20,
                },
              }}
            />
          ))}
        </Box>
      </Box>
    </PanelShell>
  );
};

export default SavedLocationsModal;
