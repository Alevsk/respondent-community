import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { theme } from '../theme';
import { Surface } from './Surface';

describe('Surface', () => {
  const renderSurface = (level: 'base' | 'raised' | 'overlay') =>
    render(
      <ThemeProvider theme={theme}>
        <Surface level={level}>
          <span>content</span>
        </Surface>
      </ThemeProvider>,
    );

  it('renders children inside a base surface', () => {
    renderSurface('base');
    expect(screen.getByText('content')).toBeInTheDocument();
  });

  it('renders children inside a raised surface', () => {
    renderSurface('raised');
    expect(screen.getByText('content')).toBeInTheDocument();
  });

  it('renders children inside an overlay surface', () => {
    renderSurface('overlay');
    expect(screen.getByText('content')).toBeInTheDocument();
  });
});
