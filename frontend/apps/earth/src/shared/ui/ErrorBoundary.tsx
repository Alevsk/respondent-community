import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Box, Typography, Button } from '@mui/material';

/** Props accepted by the ErrorBoundary component. */
export interface ErrorBoundaryProps {
  children: ReactNode;
  /** Optional custom fallback UI. When omitted, the default error screen is shown. */
  fallback?: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

/**
 * ErrorBoundary -- catches runtime exceptions thrown by descendant components
 * and renders a recoverable fallback UI instead of an empty white screen.
 *
 * Usage:
 *   <ErrorBoundary>
 *     <AppShell />
 *   </ErrorBoundary>
 *
 *   <ErrorBoundary fallback={<span>Globe unavailable</span>}>
 *     <GlobeScene />
 *   </ErrorBoundary>
 */
class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error(
      JSON.stringify({
        level: 'error',
        component: 'ErrorBoundary',
        message: error.message,
        stack: error.stack,
        componentStack: info.componentStack,
      }),
    );
  }

  private handleReload = (): void => {
    window.location.reload();
  };

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback !== undefined) {
        return this.props.fallback;
      }

      return (
        <Box
          data-testid="error-boundary-fallback"
          role="alert"
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            width: '100%',
            height: '100%',
            bgcolor: 'background.default',
            color: 'text.primary',
            p: 4,
            textAlign: 'center',
          }}
        >
          <Typography
            variant="h6"
            sx={{
              color: 'secondary.main',
              mb: 1,
              letterSpacing: '0.05em',
            }}
          >
            SYSTEM ERROR
          </Typography>
          <Typography
            variant="body2"
            sx={{
              color: 'text.secondary',
              mb: 3,
              maxWidth: 480,
            }}
          >
            An unexpected error occurred. The interface could not render this section. Reloading the
            page may resolve the issue.
          </Typography>
          <Button
            variant="outlined"
            color="primary"
            onClick={this.handleReload}
            data-testid="error-boundary-reload"
          >
            Reload
          </Button>
        </Box>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
