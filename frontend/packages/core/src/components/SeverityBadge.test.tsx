import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { theme } from '../theme';
import { SeverityBadge } from './SeverityBadge';

describe('SeverityBadge', () => {
  const renderBadge = (attention: string) =>
    render(
      <ThemeProvider theme={theme}>
        <SeverityBadge attention={attention} />
      </ThemeProvider>,
    );

  it('renders CRITICAL badge', () => {
    renderBadge('critical');
    expect(screen.getByText('CRITICAL')).toBeInTheDocument();
  });

  it('renders HIGH badge', () => {
    renderBadge('high');
    expect(screen.getByText('HIGH')).toBeInTheDocument();
  });

  it('renders MEDIUM badge', () => {
    renderBadge('medium');
    expect(screen.getByText('MEDIUM')).toBeInTheDocument();
  });

  it('falls back to info colors for unknown levels', () => {
    renderBadge('unknown');
    expect(screen.getByText('UNKNOWN')).toBeInTheDocument();
  });
});
