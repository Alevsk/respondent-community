import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider } from '@mui/material/styles';
import { theme } from '../theme';
import AppLoadingOverlay from './AppLoadingOverlay';

function renderOverlay(props: Partial<React.ComponentProps<typeof AppLoadingOverlay>> = {}) {
  return render(
    <ThemeProvider theme={theme}>
      <AppLoadingOverlay
        label={props.label ?? 'LOADING LAYERS...'}
        error={props.error ?? false}
        onRetry={props.onRetry ?? vi.fn()}
      />
    </ThemeProvider>,
  );
}

describe('AppLoadingOverlay', () => {
  it('renders the status label', () => {
    renderOverlay({ label: 'CONNECTING STREAM...' });
    expect(screen.getByText('CONNECTING STREAM...')).toBeInTheDocument();
  });

  it('renders INITIALIZING prefix', () => {
    renderOverlay();
    expect(screen.getByText('INITIALIZING')).toBeInTheDocument();
  });

  it('does not show retry button in loading state', () => {
    renderOverlay();
    expect(screen.queryByRole('button', { name: /retry/i })).not.toBeInTheDocument();
  });

  it('shows retry button in error state', () => {
    renderOverlay({ error: true });
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('calls onRetry when retry button is clicked', () => {
    const onRetry = vi.fn();
    renderOverlay({ error: true, onRetry });
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('shows CONNECTION FAILED text in error state', () => {
    renderOverlay({ error: true, label: 'LOADING LAYERS...' });
    expect(screen.getByText('CONNECTION FAILED')).toBeInTheDocument();
  });
});
