import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider } from '@mui/material/styles';
import { theme } from '@respondent/core';

// Mock the bootstrap hook
const mockBootstrap = {
  steps: [],
  ready: false,
  error: false,
  currentLabel: 'LOADING LAYERS...',
  retry: vi.fn(),
};

vi.mock('./useAppBootstrap', () => ({
  useAppBootstrap: () => mockBootstrap,
}));

import BootstrapGate from './BootstrapGate';

function renderGate() {
  return render(
    <ThemeProvider theme={theme}>
      <BootstrapGate>
        <div data-testid="app-content">App loaded</div>
      </BootstrapGate>
    </ThemeProvider>,
  );
}

describe('BootstrapGate', () => {
  it('shows loading overlay when not ready', () => {
    mockBootstrap.ready = false;
    mockBootstrap.error = false;
    renderGate();
    expect(screen.getByTestId('app-loading-overlay')).toBeInTheDocument();
    expect(screen.queryByTestId('app-content')).not.toBeInTheDocument();
  });

  it('renders children when ready', () => {
    mockBootstrap.ready = true;
    mockBootstrap.error = false;
    renderGate();
    expect(screen.getByTestId('app-content')).toBeInTheDocument();
    expect(screen.queryByTestId('app-loading-overlay')).not.toBeInTheDocument();
  });

  it('shows error overlay with retry when error', () => {
    mockBootstrap.ready = false;
    mockBootstrap.error = true;
    renderGate();
    expect(screen.getByTestId('app-loading-overlay')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    expect(screen.queryByTestId('app-content')).not.toBeInTheDocument();
  });

  it('passes currentLabel to overlay', () => {
    mockBootstrap.ready = false;
    mockBootstrap.error = false;
    mockBootstrap.currentLabel = 'CONNECTING STREAM...';
    renderGate();
    expect(screen.getByText('CONNECTING STREAM...')).toBeInTheDocument();
  });
});
