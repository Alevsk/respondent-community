/**
 * SearchResultItem Component Tests
 *
 * Tests rendering of result data, click-to-add behavior, and added state highlight.
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import SearchResultItem from './SearchResultItem';
import type { EntitySearchResult } from './SearchResultItem';

const theme = createTheme({ palette: { mode: 'dark' } });

const wrap = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

const mockResult: EntitySearchResult = {
  entityId: 'flights_commercial:UAL1234',
  externalId: 'UAL1234',
  layerType: 'flights_commercial',
  name: 'UAL1234',
};

describe('SearchResultItem', () => {
  it('renders entity name', () => {
    wrap(<SearchResultItem result={mockResult} isInWatchlist={false} onAdd={vi.fn()} />);
    expect(screen.getByTestId('search-result-name')).toHaveTextContent('UAL1234');
  });

  it('renders layer type label', () => {
    wrap(<SearchResultItem result={mockResult} isInWatchlist={false} onAdd={vi.fn()} />);
    expect(screen.getByText('flights_commercial')).toBeInTheDocument();
  });

  it('calls onAdd when the row is clicked', () => {
    const onAdd = vi.fn();
    wrap(<SearchResultItem result={mockResult} isInWatchlist={false} onAdd={onAdd} />);
    fireEvent.click(screen.getByTestId('search-result-item'));
    expect(onAdd).toHaveBeenCalledWith(mockResult);
  });

  it('does not call onAdd when already in watchlist', () => {
    const onAdd = vi.fn();
    wrap(<SearchResultItem result={mockResult} isInWatchlist={true} onAdd={onAdd} />);
    fireEvent.click(screen.getByTestId('search-result-item'));
    expect(onAdd).not.toHaveBeenCalled();
  });

  it('shows check icon when in watchlist', () => {
    const { container } = wrap(
      <SearchResultItem result={mockResult} isInWatchlist={true} onAdd={vi.fn()} />,
    );
    expect(container.querySelector('.lucide-circle-check')).toBeInTheDocument();
  });

  it('falls back to externalId when name is empty', () => {
    const noNameResult = { ...mockResult, name: '' };
    wrap(<SearchResultItem result={noNameResult} isInWatchlist={false} onAdd={vi.fn()} />);
    expect(screen.getByTestId('search-result-name')).toHaveTextContent('UAL1234');
  });
});
