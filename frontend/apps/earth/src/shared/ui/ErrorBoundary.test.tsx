/**
 * ErrorBoundary Component Tests
 *
 * Verifies that the ErrorBoundary:
 *  - renders children when no error occurs
 *  - catches thrown errors and renders the default fallback UI
 *  - renders a custom fallback when the `fallback` prop is provided
 *  - logs the error via console.error with structured JSON
 *  - provides a Reload button that calls window.location.reload
 */

import { describe, it, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import ErrorBoundary from './ErrorBoundary';

// ─── Theme matching the project ──────────────────────────────────────────────
const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    secondary: { main: '#ff006e' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
  typography: { fontFamily: 'monospace' },
});

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>
    <CssBaseline />
    {children}
  </ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

// ─── Helper: a component that always throws ──────────────────────────────────
const ThrowingComponent: React.FC<{ message?: string }> = ({ message = 'test explosion' }) => {
  throw new Error(message);
};

describe('ErrorBoundary', () => {
  let consoleSpy: MockInstance<typeof console.error>;

  beforeEach(() => {
    // Suppress noisy React error logging and spy on console.error
    consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    consoleSpy.mockRestore();
  });

  // ── Happy path ──────────────────────────────────────────────────────────────
  describe('when children render successfully', () => {
    it('should render children without any fallback', () => {
      renderWithTheme(
        <ErrorBoundary>
          <div data-testid="child">OK</div>
        </ErrorBoundary>,
      );
      expect(screen.getByTestId('child')).toBeInTheDocument();
      expect(screen.queryByTestId('error-boundary-fallback')).not.toBeInTheDocument();
    });
  });

  // ── Default fallback ────────────────────────────────────────────────────────
  describe('when a child throws an error', () => {
    it('should render the default fallback UI', () => {
      renderWithTheme(
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>,
      );
      expect(screen.getByTestId('error-boundary-fallback')).toBeInTheDocument();
      expect(screen.getByText('SYSTEM ERROR')).toBeInTheDocument();
      expect(screen.getByText(/An unexpected error occurred/)).toBeInTheDocument();
    });

    it('should render a Reload button in the default fallback', () => {
      renderWithTheme(
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>,
      );
      expect(screen.getByTestId('error-boundary-reload')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /reload/i })).toBeInTheDocument();
    });

    it('should have role="alert" on the fallback container', () => {
      renderWithTheme(
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>,
      );
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('should log the error as structured JSON via console.error', () => {
      renderWithTheme(
        <ErrorBoundary>
          <ThrowingComponent message="kaboom" />
        </ErrorBoundary>,
      );
      const structuredCall = consoleSpy.mock.calls.find(
        (args) => typeof args[0] === 'string' && args[0].includes('"component":"ErrorBoundary"'),
      );
      expect(structuredCall).toBeDefined();
      const parsed = JSON.parse(structuredCall![0] as string);
      expect(parsed).toMatchObject({
        level: 'error',
        component: 'ErrorBoundary',
        message: 'kaboom',
      });
    });
  });

  // ── Custom fallback ─────────────────────────────────────────────────────────
  describe('when a custom fallback prop is provided', () => {
    it('should render the custom fallback instead of the default', () => {
      renderWithTheme(
        <ErrorBoundary fallback={<div data-testid="custom-fallback">Custom Error View</div>}>
          <ThrowingComponent />
        </ErrorBoundary>,
      );
      expect(screen.getByTestId('custom-fallback')).toBeInTheDocument();
      expect(screen.queryByTestId('error-boundary-fallback')).not.toBeInTheDocument();
    });
  });

  // ── Reload button ───────────────────────────────────────────────────────────
  describe('Reload button', () => {
    it('should call window.location.reload when clicked', async () => {
      const reloadMock = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { ...window.location, reload: reloadMock },
        writable: true,
      });

      renderWithTheme(
        <ErrorBoundary>
          <ThrowingComponent />
        </ErrorBoundary>,
      );

      await userEvent.click(screen.getByTestId('error-boundary-reload'));
      expect(reloadMock).toHaveBeenCalledTimes(1);
    });
  });
});
