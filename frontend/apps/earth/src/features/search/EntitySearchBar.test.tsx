/**
 * EntitySearchBar Component Tests
 *
 * Tests input rendering, debounced query propagation, result display,
 * ESC key closing, click-outside closing, and keyboard navigation.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { useUIStore } from '@/app/store';
import EntitySearchBar from './EntitySearchBar';
import type { EntitySearchResult } from './SearchResultItem';

// ─── Module mock for useResponsive ───────────────────────────────────────────

// Default to desktop behaviour so existing tests are unaffected.
const mockUseResponsive = vi.fn(() => ({ isMobile: false, isTablet: false, isDesktop: true }));
vi.mock('@respondent/core', async () => {
  const actual = await vi.importActual<Record<string, unknown>>('@respondent/core');
  return { ...actual, useResponsive: () => mockUseResponsive() };
});

const theme = createTheme({ palette: { mode: 'dark' } });

const wrap = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

const mockResults: EntitySearchResult[] = [
  {
    entityId: 'flights_commercial:UAL1234',
    externalId: 'UAL1234',
    layerType: 'flights_commercial',
    name: 'UAL1234',
  },
  {
    entityId: 'satellites:ISS',
    externalId: 'ISS',
    layerType: 'satellites',
    name: 'ISS (ZARYA)',
  },
];

describe('EntitySearchBar', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    useUIStore.setState({
      searchOpen: true,
      searchQuery: '',
      watchlistEntities: [],
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders when searchOpen is true', () => {
    wrap(<EntitySearchBar />);
    expect(screen.getByTestId('entity-search-bar')).toBeInTheDocument();
  });

  it('renders the search input with correct placeholder', () => {
    wrap(<EntitySearchBar />);
    expect(screen.getByPlaceholderText('Search by ID, name, or callsign...')).toBeInTheDocument();
  });

  it('debounces input before updating store searchQuery', () => {
    wrap(<EntitySearchBar />);
    const input = screen.getByTestId('search-input-field');

    fireEvent.change(input, { target: { value: 'UAL' } });
    // Before debounce fires, store should not be updated
    expect(useUIStore.getState().searchQuery).toBe('');

    // Advance past debounce
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(useUIStore.getState().searchQuery).toBe('UAL');
  });

  it('displays search results when provided', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 2 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    // Type something to show results area
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'UAL' } });

    const resultItems = screen.getAllByTestId('search-result-item');
    expect(resultItems).toHaveLength(2);
  });

  it('displays loading skeletons when isLoading is true', () => {
    const searchResults = {
      data: undefined,
      isLoading: true,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'test' } });

    // MUI Skeleton elements should be present
    const skeletons = screen.getByTestId('search-results').querySelectorAll('.MuiSkeleton-root');
    expect(skeletons).toHaveLength(3);
  });

  it('displays "No entities found" when results are empty', () => {
    const searchResults = {
      data: { results: [], totalCount: 0 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'xyz' } });

    expect(screen.getByTestId('search-no-results')).toHaveTextContent('No entities found');
  });

  it('closes search bar when ESC is pressed', () => {
    useUIStore.setState({ searchOpen: true });
    wrap(<EntitySearchBar />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(useUIStore.getState().searchOpen).toBe(false);
  });

  it('adds first result to watchlist when Enter is pressed with no focused item', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 2 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'UAL' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    const watchlist = useUIStore.getState().watchlistEntities;
    expect(watchlist).toHaveLength(1);
    expect(watchlist[0].entityId).toBe('flights_commercial:UAL1234');
    expect(watchlist[0].pinned).toBe(true);
  });

  it('navigates results with Arrow keys and adds focused result on Enter', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 2 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'test' } });

    // Arrow down twice to focus ISS
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'Enter' });

    const watchlist = useUIStore.getState().watchlistEntities;
    expect(watchlist).toHaveLength(1);
    expect(watchlist[0].entityId).toBe('satellites:ISS');
  });

  it('does not show results area when input is empty', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 2 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    expect(screen.queryByTestId('search-results')).not.toBeInTheDocument();
  });

  it('shows "N more" footer when total exceeds displayed count', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 50 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'test' } });

    expect(screen.getByText(/48 more/)).toBeInTheDocument();
  });

  it('adds entity to watchlist when clicking a result row', () => {
    const searchResults = {
      data: { results: mockResults, totalCount: 2 },
      isLoading: false,
    };
    wrap(<EntitySearchBar searchResults={searchResults} />);
    const input = screen.getByTestId('search-input-field');
    fireEvent.change(input, { target: { value: 'test' } });

    const resultItems = screen.getAllByTestId('search-result-item');
    fireEvent.click(resultItems[0]);

    const watchlist = useUIStore.getState().watchlistEntities;
    expect(watchlist).toHaveLength(1);
    expect(watchlist[0].entityId).toBe('flights_commercial:UAL1234');
  });

  // ─── Mobile layout ────────────────────────────────────────────────────────

  describe('Mobile layout', () => {
    /**
     * EntitySearchBar reads `isMobile` from useResponsive and applies
     * different alignment and sizing to the outer overlay Box on mobile.
     *
     * MUI sx props are processed by Emotion, which injects CSS class rules
     * into jsdom's CSSOM rather than setting inline `style` attributes.
     * `window.getComputedStyle(el)` resolves those injected rules, so all
     * assertions use it instead of `element.style.*`.
     *
     * DOM structure (from inside out):
     *   <div> (Fade transparent wrapper — no className)
     *     <div class="MuiBox-root …"> ← fixed overlay (position:fixed)
     *       <ClickAwayListener sentinel>
     *         <div data-testid="entity-search-bar"> ← search widget
     *
     * `searchBar.parentElement` is the fixed overlay Box.
     *
     * Constants used in the component:
     *   MOBILE_HEADER_HEIGHT = 40
     *   top padding on mobile = `calc(var(--sat, 0px) + 48px)` (40 + 8)
     *   maxHeight on mobile   = `calc(50dvh - var(--sat, 0px))`
     *   alignItems on mobile  = `flex-start`
     *   alignItems on desktop = `center`
     */

    beforeEach(() => {
      // Each mobile test starts with the mock returning mobile dimensions.
      mockUseResponsive.mockReturnValue({ isMobile: true, isTablet: false, isDesktop: false });
    });

    afterEach(() => {
      // Restore desktop default so other test suites are unaffected.
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });
    });

    it('uses flex-start alignment on mobile viewport', () => {
      wrap(<EntitySearchBar />);

      // The fixed overlay is the direct parent of the entity-search-bar box.
      const searchBar = screen.getByTestId('entity-search-bar');
      const overlay = searchBar.parentElement as HTMLElement;

      expect(window.getComputedStyle(overlay).alignItems).toBe('flex-start');
    });

    it('applies top padding equal to MOBILE_HEADER_HEIGHT + 8px above safe area on mobile', () => {
      wrap(<EntitySearchBar />);

      const searchBar = screen.getByTestId('entity-search-bar');
      const overlay = searchBar.parentElement as HTMLElement;

      // MOBILE_HEADER_HEIGHT = 40, gap = 8 → 48px
      expect(window.getComputedStyle(overlay).paddingTop).toBe('calc(var(--sat, 0px) + 48px)');
    });

    it('constrains search box max-height to 50dvh minus safe area inset on mobile', () => {
      wrap(<EntitySearchBar />);

      const searchBar = screen.getByTestId('entity-search-bar');
      expect(window.getComputedStyle(searchBar).maxHeight).toBe('calc(50dvh - var(--sat, 0px))');
    });

    it('uses center alignment on desktop viewport', () => {
      // Override back to desktop for this specific test.
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });

      wrap(<EntitySearchBar />);

      const searchBar = screen.getByTestId('entity-search-bar');
      const overlay = searchBar.parentElement as HTMLElement;

      expect(window.getComputedStyle(overlay).alignItems).toBe('center');
    });

    it('applies no top padding on desktop viewport', () => {
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });

      wrap(<EntitySearchBar />);

      const searchBar = screen.getByTestId('entity-search-bar');
      const overlay = searchBar.parentElement as HTMLElement;

      // pt: 0 on desktop — jsdom resolves this as '' or '0px'
      expect(window.getComputedStyle(overlay).paddingTop).toMatch(/^(0px)?$/);
    });

    it('limits search box max-height to 60vh on desktop viewport', () => {
      mockUseResponsive.mockReturnValue({ isMobile: false, isTablet: false, isDesktop: true });

      wrap(<EntitySearchBar />);

      const searchBar = screen.getByTestId('entity-search-bar');
      expect(window.getComputedStyle(searchBar).maxHeight).toBe('60vh');
    });
  });
});
