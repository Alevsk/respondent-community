import { ThemeProvider, CssBaseline, Box, Typography } from '@mui/material';
import { theme } from '@respondent/core';

export function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box
        sx={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
      >
        <Typography variant="h4" sx={{ color: 'primary.main', fontFamily: 'monospace' }}>
          RESPONDENT — Immersive Mode
        </Typography>
      </Box>
    </ThemeProvider>
  );
}
