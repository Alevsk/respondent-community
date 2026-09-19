/**
 * MobileEntityPanel Component Tests
 *
 * Tests for the floating entity detail overlay rendered on mobile viewports.
 * The panel is purely reactive to Zustand store state — no drawer mechanism.
 * It starts minimized and expands when the header is tapped.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import { useUIStore } from '@/app/store';

// ─── Module mocks ────────────────────────────────────────────────────────────

// Mock the shared detail hook so tests don't need a running API or React Query provider.
const mockUseEntityDetailWithFallback = vi.fn();
vi.mock('./useEntityDetailWithFallback', () => ({
  useEntityDetailWithFallback: (...args: unknown[]) => mockUseEntityDetailWithFallback(...args),
}));

// Stub out the side-effect tab registrations (may pull in Cesium or heavy deps).
vi.mock('./tabs/OverviewTab', () => ({}));
vi.mock('./tabs/AIAnalysisTab', () => ({}));
vi.mock('./tabs/HistoryTab', () => ({}));
vi.mock('./tabs/MetadataTab', () => ({}));

// Mock the tab registry with a controllable implementation.
const mockGetTabsForLayerType = vi.fn();
vi.mock('./tabs/tabRegistry', () => ({
  registerTab: vi.fn(),
  getTabsForLayerType: (...args: unknown[]) => mockGetTabsForLayerType(...args),
}));

// Import the component AFTER mocks are established.
import MobileEntityPanel from './MobileEntityPanel';

// ─── Helpers ─────────────────────────────────────────────────────────────────

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>
    <CssBaseline />
    {children}
  </ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

/** Minimal tab stub returned by the mocked tab registry. */
const StubTabComponent: React.FC = () => <div data-testid="stub-tab-content">Overview content</div>;

const STUB_TABS = [{ id: 'overview', label: 'Overview', priority: 0, component: StubTabComponent }];

/** Helper: make the mock hook return entity data for a given set of entities. */
function configureMockDetail(entities: Record<string, { name: string; layerType: string }>) {
  mockUseEntityDetailWithFallback.mockImplementation((eid: string) => {
    const e = entities[eid];
    if (!e) return { detail: undefined, isLoading: false };
    return {
      detail: {
        entity: {
          id: eid,
          externalId: `ext-${eid}`,
          layerType: e.layerType,
          name: e.name,
          metadata: {},
        },
      },
      isLoading: false,
    };
  });
}

/**
 * Populate the Zustand store with a single selected entity.
 */
function setupSingleEntity({
  entityId = 'entity-1',
  layerId = 'layer-flights',
  name = 'Alpha 7',
  viewMode = 'globe' as 'globe' | 'entity',
} = {}) {
  useUIStore.setState({
    selectedEntities: [{ entityId, layerId }],
    selectedEntityId: entityId,
    viewMode,
    entityViewState: {},
  });
  configureMockDetail({ [entityId]: { name, layerType: layerId } });
}

// ─── Tests ────────────────────────────────────────────────────────────────────

describe('MobileEntityPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetTabsForLayerType.mockReturnValue(STUB_TABS);
    mockUseEntityDetailWithFallback.mockReturnValue({ detail: undefined, isLoading: false });
    useUIStore.setState({
      selectedEntities: [],
      selectedEntityId: null,
      viewMode: 'globe',
      entityViewState: {},
    });
  });

  // ─── Visibility ────────────────────────────────────────────────────────────

  describe('Visibility', () => {
    it('does not render anything when selectedEntities is empty', () => {
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.queryByTestId('mobile-entity-panel-container')).not.toBeInTheDocument();
      expect(screen.queryByTestId('mobile-entity-panel')).not.toBeInTheDocument();
    });

    it('renders the panel container when at least one entity is selected', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByTestId('mobile-entity-panel-container')).toBeInTheDocument();
    });

    it('renders one panel item per selected entity', () => {
      useUIStore.setState({
        selectedEntities: [
          { entityId: 'entity-1', layerId: 'layer-a' },
          { entityId: 'entity-2', layerId: 'layer-b' },
        ],
        selectedEntityId: 'entity-1',
        viewMode: 'globe',
        entityViewState: {},
      });
      configureMockDetail({
        'entity-1': { name: 'Alpha', layerType: 'layer-a' },
        'entity-2': { name: 'Bravo', layerType: 'layer-b' },
      });

      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getAllByTestId('mobile-entity-panel')).toHaveLength(2);
    });
  });

  // ─── Entity name ───────────────────────────────────────────────────────────

  describe('Entity name', () => {
    it('shows the entity name from the detail hook', () => {
      setupSingleEntity({ name: 'Alpha 7' });
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByText('Alpha 7')).toBeInTheDocument();
    });

    it('falls back to entityId when no detail data is available', () => {
      useUIStore.setState({
        selectedEntities: [{ entityId: 'mystery-id', layerId: 'layer-x' }],
        selectedEntityId: 'mystery-id',
        viewMode: 'globe',
        entityViewState: {},
      });
      // Default mock returns undefined detail
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByText('mystery-id')).toBeInTheDocument();
    });
  });

  // ─── Status badge ──────────────────────────────────────────────────────────

  describe('Status badge', () => {
    it('shows "SELECTED" status for a selected entity in globe view mode', () => {
      setupSingleEntity({ viewMode: 'globe' });
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByText('SELECTED')).toBeInTheDocument();
    });

    it('shows "TRACKING" status for the primary entity when viewMode is "entity"', () => {
      setupSingleEntity({ viewMode: 'entity' });
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByText('TRACKING')).toBeInTheDocument();
    });

    it('shows "SELECTED" for a non-primary entity even when viewMode is "entity"', () => {
      useUIStore.setState({
        selectedEntities: [
          { entityId: 'entity-1', layerId: 'layer-a' },
          { entityId: 'entity-2', layerId: 'layer-a' },
        ],
        selectedEntityId: 'entity-1',
        viewMode: 'entity',
        entityViewState: {},
      });
      configureMockDetail({
        'entity-1': { name: 'Primary', layerType: 'layer-a' },
        'entity-2': { name: 'Secondary', layerType: 'layer-a' },
      });

      renderWithTheme(<MobileEntityPanel />);
      expect(screen.getByText('TRACKING')).toBeInTheDocument();
      expect(screen.getByText('SELECTED')).toBeInTheDocument();
    });
  });

  // ─── Minimized / maximized state ──────────────────────────────────────────

  describe('Minimized state', () => {
    it('starts in minimized state — tab content is NOT visible on initial render', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.queryByTestId('stub-tab-content')).not.toBeInTheDocument();
    });

    it('tapping the header maximizes the panel and shows tab content', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);
      fireEvent.click(screen.getByText('Alpha 7'));
      expect(screen.getByTestId('stub-tab-content')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Restore panel' })).toBeInTheDocument();
    });

    it('tapping the header a second time re-minimizes and hides tab content', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);

      const entityNameEl = screen.getByText('Alpha 7');
      fireEvent.click(entityNameEl); // maximize
      expect(screen.getByTestId('stub-tab-content')).toBeInTheDocument();

      fireEvent.click(entityNameEl); // minimize
      expect(screen.queryByTestId('stub-tab-content')).not.toBeInTheDocument();
    });
  });

  // ─── Tab bar ───────────────────────────────────────────────────────────────

  describe('Tab bar', () => {
    it('renders tab labels after the panel is maximized via header tap', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);
      fireEvent.click(screen.getByText('Alpha 7'));
      expect(screen.getByText('Overview')).toBeInTheDocument();
    });

    it('does not render tabs when panel is minimized', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);
      expect(screen.queryByText('Overview')).not.toBeInTheDocument();
    });
  });

  // ─── Close button ──────────────────────────────────────────────────────────

  describe('Close button', () => {
    it('calls removeSelectedEntity with the correct entityId when close is clicked', () => {
      setupSingleEntity({ entityId: 'entity-close-test' });
      renderWithTheme(<MobileEntityPanel />);

      const closeButton = screen.getByRole('button', { name: 'Close entity panel' });
      fireEvent.click(closeButton);

      expect(screen.queryByTestId('mobile-entity-panel-container')).not.toBeInTheDocument();
    });

    it('removes only the closed entity when multiple entities are selected', () => {
      useUIStore.setState({
        selectedEntities: [
          { entityId: 'entity-1', layerId: 'layer-a' },
          { entityId: 'entity-2', layerId: 'layer-a' },
        ],
        selectedEntityId: 'entity-1',
        viewMode: 'globe',
        entityViewState: {},
      });
      configureMockDetail({
        'entity-1': { name: 'Alpha', layerType: 'layer-a' },
        'entity-2': { name: 'Bravo', layerType: 'layer-a' },
      });

      renderWithTheme(<MobileEntityPanel />);
      const closeButtons = screen.getAllByRole('button', { name: 'Close entity panel' });
      expect(closeButtons).toHaveLength(2);

      fireEvent.click(closeButtons[0]); // close Alpha

      expect(screen.getByTestId('mobile-entity-panel-container')).toBeInTheDocument();
      expect(screen.getAllByTestId('mobile-entity-panel')).toHaveLength(1);
      expect(screen.getByText('Bravo')).toBeInTheDocument();
    });
  });

  // ─── Maximize ─────────────────────────────────────────────────────────────

  describe('Maximize button', () => {
    it('expands to near-full-screen when the maximize button is clicked', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);

      fireEvent.click(screen.getByRole('button', { name: 'Maximize panel' }));
      expect(screen.getByRole('button', { name: 'Restore panel' })).toBeInTheDocument();
    });

    it('restores to minimized state when maximize is clicked a second time', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);

      fireEvent.click(screen.getByRole('button', { name: 'Maximize panel' }));
      fireEvent.click(screen.getByRole('button', { name: 'Restore panel' }));

      expect(screen.getByRole('button', { name: 'Maximize panel' })).toBeInTheDocument();
      expect(screen.queryByTestId('stub-tab-content')).not.toBeInTheDocument();
    });

    it('shows tab content in maximized mode without needing to tap the header', () => {
      setupSingleEntity();
      renderWithTheme(<MobileEntityPanel />);

      fireEvent.click(screen.getByRole('button', { name: 'Maximize panel' }));
      expect(screen.getByTestId('stub-tab-content')).toBeInTheDocument();
    });
  });

  // ─── Multi-select ─────────────────────────────────────────────────────────

  describe('Multi-select rendering', () => {
    it('renders a separate panel for each selected entity', () => {
      useUIStore.setState({
        selectedEntities: [
          { entityId: 'e-1', layerId: 'layer-a' },
          { entityId: 'e-2', layerId: 'layer-a' },
          { entityId: 'e-3', layerId: 'layer-a' },
        ],
        selectedEntityId: 'e-1',
        viewMode: 'globe',
        entityViewState: {},
      });
      configureMockDetail({
        'e-1': { name: 'Alpha', layerType: 'layer-a' },
        'e-2': { name: 'Bravo', layerType: 'layer-a' },
        'e-3': { name: 'Charlie', layerType: 'layer-a' },
      });

      renderWithTheme(<MobileEntityPanel />);

      expect(screen.getAllByTestId('mobile-entity-panel')).toHaveLength(3);
      expect(screen.getByText('Alpha')).toBeInTheDocument();
      expect(screen.getByText('Bravo')).toBeInTheDocument();
      expect(screen.getByText('Charlie')).toBeInTheDocument();
    });
  });

  // ─── Watchlist-aware positioning ──────────────────────────────────────────

  describe('Watchlist-aware positioning', () => {
    /**
     * MUI sx props are processed by Emotion which injects CSS rules into
     * jsdom's CSSOM rather than writing inline `style` attributes. Calling
     * `window.getComputedStyle(el)` resolves those injected rules correctly,
     * so we use that instead of `element.style.*` or `toHaveStyle()`.
     *
     * Emotion generates a new class hash whenever the resolved style string
     * changes (e.g. when bottomOffset changes). After a rerender we must
     * re-query the container element because the new class might be a fresh
     * DOM node, but in practice React reuses the node and updates its class.
     * We therefore re-query by test-id after each store mutation.
     */

    it('uses base offset (MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP = 72px) when watchlistBarHeight is 0', () => {
      setupSingleEntity();
      useUIStore.setState({ watchlistBarHeight: 0 });

      renderWithTheme(<MobileEntityPanel />);

      const container = screen.getByTestId('mobile-entity-panel-container');
      // bottom = calc(var(--sab) + 72px)  (56 + 16)
      expect(window.getComputedStyle(container).bottom).toBe('calc(var(--sab) + 72px)');
    });

    it('increases bottom offset by watchlistBarHeight + MOBILE_STACK_GAP (8px) when watchlist is visible', () => {
      setupSingleEntity();
      useUIStore.setState({ watchlistBarHeight: 200 });

      renderWithTheme(<MobileEntityPanel />);

      const container = screen.getByTestId('mobile-entity-panel-container');
      // bottomOffset = 72 + 200 + 8 = 280
      expect(window.getComputedStyle(container).bottom).toBe('calc(var(--sab) + 280px)');
    });

    it('always includes var(--sab) in the bottom CSS value', () => {
      setupSingleEntity();
      useUIStore.setState({ watchlistBarHeight: 0 });

      renderWithTheme(<MobileEntityPanel />);

      const container = screen.getByTestId('mobile-entity-panel-container');
      expect(window.getComputedStyle(container).bottom).toContain('var(--sab)');
    });

    it('reflects updated bottom offset when watchlistBarHeight changes in the store', () => {
      setupSingleEntity();
      useUIStore.setState({ watchlistBarHeight: 0 });

      const { rerender } = renderWithTheme(<MobileEntityPanel />);

      // Confirm base offset before watchlist appears.
      expect(
        window.getComputedStyle(screen.getByTestId('mobile-entity-panel-container')).bottom,
      ).toBe('calc(var(--sab) + 72px)');

      // Simulate the watchlist bar being mounted and measured.
      useUIStore.setState({ watchlistBarHeight: 120 });
      rerender(
        <TestWrapper>
          <MobileEntityPanel />
        </TestWrapper>,
      );

      // bottomOffset = 72 + 120 + 8 = 200
      expect(
        window.getComputedStyle(screen.getByTestId('mobile-entity-panel-container')).bottom,
      ).toBe('calc(var(--sab) + 200px)');
    });
  });
});
