import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { theme } from '@respondent/core';
import { useUIStore } from '@/app/store';
import NotificationPanel from './NotificationPanel';
import type { AIInsightNotification } from '@respondent/core';

function makeNotification(id: string): AIInsightNotification {
  return {
    id,
    insightType: 'anomaly',
    sourceName: 'test',
    operationName: 'test_op',
    result: { title: `Notification ${id}` },
    entityIds: [],
    entities: [],
    observationIds: [],
    createdAt: '2026-03-31T12:00:00Z',
  };
}

/** Notification with multiple rich entity refs for multi-select testing. */
function makeMultiEntityNotification(): AIInsightNotification {
  return {
    id: 'multi-1',
    insightType: 'environmental_cascade',
    sourceName: 'env_detector',
    operationName: 'env_impact',
    layerType: 'fires_active',
    result: { title: 'Environmental Impact Assessment' },
    entityIds: [],
    entities: [
      { id: 'e1', externalId: 'FIRE_001', name: 'Fire Alpha', layerType: 'fires_active' },
      { id: 'e2', externalId: 'AQ_001', name: 'PM2.5 Sensor', layerType: 'air_quality' },
    ],
    observationIds: [],
    createdAt: '2026-03-31T12:00:00Z',
  };
}

function renderPanel() {
  const loadMore = vi.fn();
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    loadMore,
    ...render(
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <NotificationPanel loadMore={loadMore} />
        </ThemeProvider>
      </QueryClientProvider>,
    ),
  };
}

beforeEach(() => {
  useUIStore.setState(
    {
      notifications: [],
      notificationIds: new Set(),
      readInsightIds: new Set(),
      unreadCount: 0,
      notificationPanelOpen: true,
      notificationsTotalCount: 0,
      notificationsLoading: false,
    },
    false,
  );
});

describe('NotificationPanel', () => {
  it('shows empty state when no notifications', () => {
    renderPanel();
    expect(screen.getByText('No notifications match current filters')).toBeInTheDocument();
  });

  it('renders notification items', () => {
    useUIStore.setState(
      {
        notifications: [makeNotification('n1'), makeNotification('n2')],
        notificationIds: new Set(['n1', 'n2']),
        unreadCount: 2,
      },
      false,
    );

    renderPanel();
    expect(screen.getByText('Notification n1')).toBeInTheDocument();
    expect(screen.getByText('Notification n2')).toBeInTheDocument();
  });

  it('shows load more button when more data exists', () => {
    useUIStore.setState(
      {
        notifications: [makeNotification('n1')],
        notificationIds: new Set(['n1']),
        notificationsTotalCount: 50,
        unreadCount: 1,
      },
      false,
    );

    renderPanel();
    expect(screen.getByText('LOAD MORE')).toBeInTheDocument();
  });

  it('hides load more when all loaded', () => {
    useUIStore.setState(
      {
        notifications: [makeNotification('n1')],
        notificationIds: new Set(['n1']),
        notificationsTotalCount: 1,
        unreadCount: 1,
      },
      false,
    );

    renderPanel();
    expect(screen.queryByText('LOAD MORE')).not.toBeInTheDocument();
  });

  it('calls loadMore on button click', () => {
    useUIStore.setState(
      {
        notifications: [makeNotification('n1')],
        notificationIds: new Set(['n1']),
        notificationsTotalCount: 50,
        unreadCount: 1,
      },
      false,
    );

    const { loadMore } = renderPanel();
    fireEvent.click(screen.getByText('LOAD MORE'));
    expect(loadMore).toHaveBeenCalledTimes(1);
  });

  it('mark all read button marks all as read', () => {
    useUIStore.setState(
      {
        notifications: [makeNotification('n1'), makeNotification('n2')],
        notificationIds: new Set(['n1', 'n2']),
        unreadCount: 2,
      },
      false,
    );

    renderPanel();
    fireEvent.click(screen.getByText('MARK ALL READ'));

    const { readInsightIds, unreadCount } = useUIStore.getState();
    expect(readInsightIds.has('n1')).toBe(true);
    expect(readInsightIds.has('n2')).toBe(true);
    expect(unreadCount).toBe(0);
  });

  it('does not render when panel is closed', () => {
    useUIStore.setState({ notificationPanelOpen: false }, false);
    renderPanel();
    expect(screen.queryByTestId('notification-panel')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Multi-entity selection from notification click
// ---------------------------------------------------------------------------

describe('NotificationPanel multi-entity selection', () => {
  let flyToEvents: Array<Record<string, unknown>>;

  beforeEach(() => {
    flyToEvents = [];
    const listener = (e: Event) => {
      flyToEvents.push((e as CustomEvent).detail);
    };
    window.addEventListener('respondent:flyto', listener);
    // Store listener ref for cleanup
    (window as unknown as Record<string, unknown>).__flyToListener = listener;
  });

  afterEach(() => {
    const listener = (window as unknown as Record<string, unknown>)
      .__flyToListener as EventListener;
    if (listener) window.removeEventListener('respondent:flyto', listener);
  });

  it('selects all notification entities when clicking an entity chip', async () => {
    const notification = makeMultiEntityNotification();
    useUIStore.setState(
      {
        notifications: [notification],
        notificationIds: new Set([notification.id]),
        unreadCount: 1,
        notificationPanelOpen: true,
        notificationsTotalCount: 1,
      },
      false,
    );

    renderPanel();
    // Expand the notification
    fireEvent.click(screen.getByText('Environmental Impact Assessment'));
    // Click the first entity chip
    await act(async () => {
      fireEvent.click(screen.getByText('Fire Alpha'));
    });

    const state = useUIStore.getState();
    // Both entities should be selected
    expect(state.selectedEntities).toHaveLength(2);
    expect(state.selectedEntities[0].entityId).toBe('fires_active:FIRE_001');
    expect(state.selectedEntities[1].entityId).toBe('air_quality:AQ_001');
  });

  it('adds all notification entities to watchlist', async () => {
    const notification = makeMultiEntityNotification();
    useUIStore.setState(
      {
        notifications: [notification],
        notificationIds: new Set([notification.id]),
        unreadCount: 1,
        notificationPanelOpen: true,
        notificationsTotalCount: 1,
      },
      false,
    );

    renderPanel();
    fireEvent.click(screen.getByText('Environmental Impact Assessment'));
    await act(async () => {
      fireEvent.click(screen.getByText('PM2.5 Sensor'));
    });

    const entities = useUIStore.getState().watchlistEntities;
    expect(entities).toHaveLength(2);
    expect(entities.map((e) => e.entityId)).toEqual([
      'fires_active:FIRE_001',
      'air_quality:AQ_001',
    ]);
  });

  it('dispatches bounds fly-to when multiple entity positions exist', async () => {
    const notification = makeMultiEntityNotification();

    // Seed entity observations in the store so positions are known.
    useUIStore.getState().setLayerEntities('fires_active', {
      entities: [
        {
          id: 'fires_active:FIRE_001',
          externalId: 'FIRE_001',
          layerType: 'fires_active',
          name: 'Fire Alpha',
          metadata: {},
        },
      ],
      observations: [
        {
          entityId: 'fires_active:FIRE_001',
          position: { lat: 31.0, lon: 48.1 },
          altitudeM: 0,
          timestamp: '2026-03-31T12:00:00Z',
        },
      ],
    });
    useUIStore.getState().setLayerEntities('air_quality', {
      entities: [
        {
          id: 'air_quality:AQ_001',
          externalId: 'AQ_001',
          layerType: 'air_quality',
          name: 'PM2.5 Sensor',
          metadata: {},
        },
      ],
      observations: [
        {
          entityId: 'air_quality:AQ_001',
          position: { lat: 29.3, lon: 48.0 },
          altitudeM: 0,
          timestamp: '2026-03-31T12:00:00Z',
        },
      ],
    });

    useUIStore.setState(
      {
        notifications: [notification],
        notificationIds: new Set([notification.id]),
        unreadCount: 1,
        notificationPanelOpen: true,
        notificationsTotalCount: 1,
      },
      false,
    );

    renderPanel();
    fireEvent.click(screen.getByText('Environmental Impact Assessment'));
    await act(async () => {
      fireEvent.click(screen.getByText('Fire Alpha'));
    });

    // Should dispatch bounds-based fly-to (not a single point)
    expect(flyToEvents.length).toBe(1);
    const detail = flyToEvents[0];
    expect(detail).toHaveProperty('bounds');
    const bounds = detail.bounds as { west: number; south: number; east: number; north: number };
    // Bounds should encompass both positions (lat 29.3–31.0, lon 48.0–48.1)
    expect(bounds.south).toBeLessThan(29.3);
    expect(bounds.north).toBeGreaterThan(31.0);
    expect(bounds.west).toBeLessThan(48.0);
    expect(bounds.east).toBeGreaterThan(48.1);
  });

  it('dispatches single-point fly-to when only one entity position exists', async () => {
    const singleEntityNotification: AIInsightNotification = {
      id: 'single-1',
      insightType: 'anomaly',
      sourceName: 'test',
      operationName: 'test_op',
      layerType: 'flights',
      result: { title: 'Single entity test' },
      entityIds: [],
      entities: [{ id: 'e1', externalId: 'FLT001', name: 'Flight 001', layerType: 'flights' }],
      observationIds: [],
      createdAt: '2026-03-31T12:00:00Z',
    };

    useUIStore.getState().setLayerEntities('flights', {
      entities: [
        {
          id: 'flights:FLT001',
          externalId: 'FLT001',
          layerType: 'flights',
          name: 'Flight 001',
          metadata: {},
        },
      ],
      observations: [
        {
          entityId: 'flights:FLT001',
          position: { lat: 40.0, lon: -74.0 },
          altitudeM: 10000,
          timestamp: '2026-03-31T12:00:00Z',
        },
      ],
    });

    useUIStore.setState(
      {
        notifications: [singleEntityNotification],
        notificationIds: new Set([singleEntityNotification.id]),
        unreadCount: 1,
        notificationPanelOpen: true,
        notificationsTotalCount: 1,
      },
      false,
    );

    renderPanel();
    fireEvent.click(screen.getByText('Single entity test'));
    await act(async () => {
      fireEvent.click(screen.getByText('Flight 001'));
    });

    expect(flyToEvents.length).toBe(1);
    const detail = flyToEvents[0];
    // Single position — should dispatch lat/lon/alt, not bounds
    expect(detail).toHaveProperty('lat', 40.0);
    expect(detail).toHaveProperty('lon', -74.0);
    expect(detail).toHaveProperty('alt');
    expect(detail).not.toHaveProperty('bounds');
  });

  it('preserves pinned watchlist entries when selecting notification entities', async () => {
    const notification = makeMultiEntityNotification();
    useUIStore.getState().addToWatchlist({
      entityId: 'pinned:X',
      layerId: 'threats',
      name: 'Pinned Threat',
      pinned: true,
      addedAt: Date.now(),
    });

    useUIStore.setState(
      {
        notifications: [notification],
        notificationIds: new Set([notification.id]),
        unreadCount: 1,
        notificationPanelOpen: true,
        notificationsTotalCount: 1,
      },
      false,
    );

    renderPanel();
    fireEvent.click(screen.getByText('Environmental Impact Assessment'));
    await act(async () => {
      fireEvent.click(screen.getByText('Fire Alpha'));
    });

    const entities = useUIStore.getState().watchlistEntities;
    // Pinned entity + 2 notification entities
    expect(entities).toHaveLength(3);
    expect(entities[0].entityId).toBe('pinned:X');
    expect(entities[0].pinned).toBe(true);
  });
});
