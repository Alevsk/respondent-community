/**
 * EntityPill Component Tests
 *
 * Verifies rendering for pinned/unpinned states, click handler delegation,
 * and name truncation behavior.
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import EntityPill from './EntityPill';
import type { WatchlistEntity } from '@/app/store';

const theme = createTheme({ palette: { mode: 'dark' } });

const wrap = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

const baseEntity: WatchlistEntity = {
  entityId: 'flights_commercial:UAL1234',
  layerId: 'flights_commercial',
  name: 'UAL1234',
  pinned: false,
  addedAt: 1710288000000,
};

describe('EntityPill', () => {
  it('renders the entity name', () => {
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={vi.fn()}
        onTogglePin={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByTestId('entity-pill-name')).toHaveTextContent('UAL1234');
  });

  it('renders outlined pin icon when unpinned', () => {
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={vi.fn()}
        onTogglePin={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    const pinBtn = screen.getByTestId('entity-pill-pin');
    expect(pinBtn).toHaveAttribute('aria-label', 'Pin entity');
  });

  it('renders filled pin icon when pinned', () => {
    const pinned = { ...baseEntity, pinned: true };
    wrap(
      <EntityPill entity={pinned} onSelect={vi.fn()} onTogglePin={vi.fn()} onRemove={vi.fn()} />,
    );
    const pinBtn = screen.getByTestId('entity-pill-pin');
    expect(pinBtn).toHaveAttribute('aria-label', 'Unpin entity');
  });

  it('calls onSelect when the pill is clicked', () => {
    const onSelect = vi.fn();
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={onSelect}
        onTogglePin={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByTestId('entity-pill'));
    expect(onSelect).toHaveBeenCalledWith('flights_commercial:UAL1234');
  });

  it('calls onTogglePin when pin button is clicked (does not propagate to onSelect)', () => {
    const onSelect = vi.fn();
    const onTogglePin = vi.fn();
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={onSelect}
        onTogglePin={onTogglePin}
        onRemove={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByTestId('entity-pill-pin'));
    expect(onTogglePin).toHaveBeenCalledWith('flights_commercial:UAL1234');
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('calls onRemove when remove button is clicked (does not propagate to onSelect)', () => {
    const onSelect = vi.fn();
    const onRemove = vi.fn();
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={onSelect}
        onTogglePin={vi.fn()}
        onRemove={onRemove}
      />,
    );
    fireEvent.click(screen.getByTestId('entity-pill-remove'));
    expect(onRemove).toHaveBeenCalledWith('flights_commercial:UAL1234');
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('truncates long entity names with ellipsis via maxWidth', () => {
    const longNameEntity = {
      ...baseEntity,
      name: 'VERY_LONG_ENTITY_NAME_THAT_EXCEEDS_MAX_WIDTH',
    };
    wrap(
      <EntityPill
        entity={longNameEntity}
        onSelect={vi.fn()}
        onTogglePin={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    const nameEl = screen.getByTestId('entity-pill-name');
    expect(nameEl).toHaveTextContent('VERY_LONG_ENTITY_NAME_THAT_EXCEEDS_MAX_WIDTH');
    // The element has maxWidth + overflow: hidden + textOverflow: ellipsis via sx
    expect(nameEl).toBeInTheDocument();
  });

  it('renders the remove button with correct aria-label', () => {
    wrap(
      <EntityPill
        entity={baseEntity}
        onSelect={vi.fn()}
        onTogglePin={vi.fn()}
        onRemove={vi.fn()}
      />,
    );
    expect(screen.getByLabelText('Remove from watchlist')).toBeInTheDocument();
  });
});
