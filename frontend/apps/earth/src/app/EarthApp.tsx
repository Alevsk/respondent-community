import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import { theme } from '@respondent/core';
import EarthShell from './EarthShell';
import BootstrapGate from '../shared/bootstrap/BootstrapGate';
import ErrorBoundary from '../shared/ui/ErrorBoundary';

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
            <EarthShell />
          </BootstrapGate>
        </ErrorBoundary>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default EarthApp;
