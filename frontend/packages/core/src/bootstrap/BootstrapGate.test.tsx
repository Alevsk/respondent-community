import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider } from '@mui/material/styles';
import { theme } from '../theme';
import BootstrapGate from './BootstrapGate';
import type { BootstrapState } from './types';

function renderGate(overrides: Partial<BootstrapState> = {}) {
  const defaultState: BootstrapState = {
    steps: [],
    ready: false,
    error: false,
    currentLabel: 'LOADING LAYERS...',
    retry: vi.fn(),
    ...overrides,
  };
  return render(
    <ThemeProvider theme={theme}>
      <BootstrapGate {...defaultState}>
        <div data-testid="app-content">App loaded</div>
      </BootstrapGate>
    </ThemeProvider>,
  );
}

describe('BootstrapGate', () => {
  it('shows loading overlay when not ready', () => {
    renderGate({ ready: false, error: false });
    expect(screen.getByTestId('app-loading-overlay')).toBeInTheDocument();
    expect(screen.queryByTestId('app-content')).not.toBeInTheDocument();
  });

  it('renders children when ready', () => {
    renderGate({ ready: true, error: false });
    expect(screen.getByTestId('app-content')).toBeInTheDocument();
    expect(screen.queryByTestId('app-loading-overlay')).not.toBeInTheDocument();
  });

  it('shows error overlay with retry when error', () => {
    renderGate({ ready: false, error: true });
    expect(screen.getByTestId('app-loading-overlay')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    expect(screen.queryByTestId('app-content')).not.toBeInTheDocument();
  });

  it('passes currentLabel to overlay', () => {
    renderGate({ ready: false, error: false, currentLabel: 'CONNECTING STREAM...' });
    expect(screen.getByText('CONNECTING STREAM...')).toBeInTheDocument();
  });
});
