import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import { theme } from '@respondent/core';
import EarthShell from './EarthShell';
import BootstrapGate from '../shared/bootstrap/BootstrapGate';
import ErrorBoundary from '../shared/ui/ErrorBoundary';
import { MediaProvider } from '../features/media/MediaProvider';
import { AudioPlayer } from '../features/media/AudioPlayer';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: 30000,
      retry: 1,
    },
  },
});

function EarthApp() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <ErrorBoundary>
          <BootstrapGate>
            {/* Media ownership lives above the shell so an audio session
                survives closing the entity panel that started it. */}
            <MediaProvider>
              <EarthShell />
              <AudioPlayer />
            </MediaProvider>
          </BootstrapGate>
        </ErrorBoundary>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default EarthApp;
