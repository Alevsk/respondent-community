/**
 * WatchlistBar Component Tests
 *
 * Tests rendering of entity pills, Find Mode indicators,
 * and visibility conditions (empty, cleanUI, recording).
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { useUIStore } from '@/app/store';
import type { WatchlistEntity } from '@/app/store';
import WatchlistBar from './WatchlistBar';

const theme = createTheme({ palette: { mode: 'dark' } });

const wrap = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

const testEntity: WatchlistEntity = {
  entityId: 'flights_commercial:UAL1234',
  layerId: 'flights_commercial',
  name: 'UAL1234',
  pinned: true,
  addedAt: 1710288000000,
};

const testEntity2: WatchlistEntity = {
  entityId: 'satellites:ISS',
  layerId: 'satellites',
  name: 'ISS',
  pinned: false,
  addedAt: 1710288001000,
};

describe('WatchlistBar', () => {
  beforeEach(() => {
    // Reset store to defaults
    useUIStore.setState({
      watchlistEntities: [],
      findMode: false,
      findModeDisplay: 'dimmed',
      cleanUI: false,
      recordingMode: false,
      searchOpen: false,
    });
  });

  it('renders nothing when watchlist is empty', () => {
    wrap(<WatchlistBar />);
    expect(screen.queryByTestId('watchlist-bar')).not.toBeInTheDocument();
  });

  it('renders nothing when cleanUI is true', () => {
    useUIStore.setState({ watchlistEntities: [testEntity], cleanUI: true });
    wrap(<WatchlistBar />);
    expect(screen.queryByTestId('watchlist-bar')).not.toBeInTheDocument();
  });

  it('renders nothing when recordingMode is true', () => {
    useUIStore.setState({ watchlistEntities: [testEntity], recordingMode: true });
    wrap(<WatchlistBar />);
    expect(screen.queryByTestId('watchlist-bar')).not.toBeInTheDocument();
  });

  it('renders entity pills when watchlist has entities', () => {
    useUIStore.setState({ watchlistEntities: [testEntity, testEntity2] });
    wrap(<WatchlistBar />);
    expect(screen.getByTestId('watchlist-bar')).toBeInTheDocument();
    const pills = screen.getAllByTestId('entity-pill');
    expect(pills).toHaveLength(2);
  });

  it('renders Find Mode toggle button', () => {
    useUIStore.setState({ watchlistEntities: [testEntity] });
    wrap(<WatchlistBar />);
    expect(screen.getByTestId('find-mode-toggle')).toBeInTheDocument();
  });

  it('cycles visibility: off → dimmed → hidden → off', () => {
    useUIStore.setState({ watchlistEntities: [testEntity], findMode: false });
    wrap(<WatchlistBar />);
    const btn = screen.getByTestId('find-mode-toggle');

    // Off → dimmed
    fireEvent.click(btn);
    expect(useUIStore.getState().findMode).toBe(true);
    expect(useUIStore.getState().findModeDisplay).toBe('dimmed');

    // Dimmed → hidden
    fireEvent.click(btn);
    expect(useUIStore.getState().findMode).toBe(true);
    expect(useUIStore.getState().findModeDisplay).toBe('hidden');

    // Hidden → off
    fireEvent.click(btn);
    expect(useUIStore.getState().findMode).toBe(false);
  });

  it('removes entity from watchlist when remove is clicked', () => {
    useUIStore.setState({ watchlistEntities: [testEntity, testEntity2] });
    wrap(<WatchlistBar />);
    const removeButtons = screen.getAllByTestId('entity-pill-remove');
    fireEvent.click(removeButtons[0]);
    expect(useUIStore.getState().watchlistEntities).toHaveLength(1);
    expect(useUIStore.getState().watchlistEntities[0].entityId).toBe('satellites:ISS');
  });
});
