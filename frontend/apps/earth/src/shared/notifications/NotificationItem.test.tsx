import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { theme } from '@respondent/core';
import { useUIStore } from '@/app/store';
import NotificationItem from './NotificationItem';
import type { AIInsightNotification } from '@respondent/core';

const mockNotification: AIInsightNotification = {
  id: 'n1',
  insightType: 'anomaly',
  sourceName: 'flight_anomaly_detection',
  operationName: 'flight_anomaly_scan',
  layerType: 'flights_commercial',
  attention: 'high',
  result: {
    title: 'Rapid altitude loss',
    description: 'Flight ABC123 dropped 5000ft in 30 seconds',
    entity_external_id: 'ABC123',
  },
  entityIds: ['entity-1'],
  entities: [
    {
      id: 'entity-1',
      externalId: 'ABC123',
      name: 'Flight ABC123',
      layerType: 'flights_commercial',
    },
  ],
  observationIds: ['obs-1'],
  createdAt: new Date().toISOString(),
};

function renderItem(
  props: Partial<{ notification: AIInsightNotification; isRead: boolean; isMobile: boolean }> = {},
) {
  const onEntityClick = vi.fn();
  const onObservationClick = vi.fn();
  return {
    onEntityClick,
    onObservationClick,
    ...render(
      <ThemeProvider theme={theme}>
        <NotificationItem
          notification={props.notification ?? mockNotification}
          isRead={props.isRead ?? false}
          isMobile={props.isMobile ?? false}
          onEntityClick={onEntityClick}
          onObservationClick={onObservationClick}
        />
      </ThemeProvider>,
    ),
  };
}

beforeEach(() => {
  useUIStore.setState({ readInsightIds: new Set() }, false);
});

describe('NotificationItem', () => {
  it('renders title from result.title', () => {
    renderItem();
    expect(screen.getByText('Rapid altitude loss')).toBeInTheDocument();
  });

  it('renders insight type badge', () => {
    renderItem();
    // textTransform: uppercase is CSS-only, DOM text remains lowercase
    expect(screen.getByText('anomaly')).toBeInTheDocument();
  });

  it('falls back to humanized operation name when no title', () => {
    const noTitle = { ...mockNotification, result: { description: 'Something' } };
    renderItem({ notification: noTitle });
    expect(screen.getByText('Flight Anomaly Scan')).toBeInTheDocument();
  });

  it('shows truncated description in collapsed state', () => {
    renderItem();
    expect(screen.getByText(/Flight ABC123 dropped/)).toBeInTheDocument();
  });

  it('expands on click and shows entity chips with name from entities array', () => {
    renderItem();
    fireEvent.click(screen.getByTestId('notification-item-n1'));

    // Standalone ID chip is hidden when rich entities exist (avoids redundancy)
    expect(screen.queryByText(/ID: ABC123/)).not.toBeInTheDocument();
    // Entity chip should show the name from the rich entities array
    expect(screen.getByText('Flight ABC123')).toBeInTheDocument();
  });

  it('shows standalone ID chip when entities array is empty', () => {
    const noEntities = { ...mockNotification, entities: [], entityIds: [] };
    renderItem({ notification: noEntities });
    fireEvent.click(screen.getByTestId('notification-item-n1'));
    expect(screen.getByText(/ID: ABC123/)).toBeInTheDocument();
  });

  it('formats scientific notation in external ID', () => {
    const sciNotation = {
      ...mockNotification,
      entities: [],
      entityIds: [],
      result: { ...mockNotification.result, entity_external_id: '1.66217243e+08' },
    };
    renderItem({ notification: sciNotation });
    fireEvent.click(screen.getByTestId('notification-item-n1'));
    expect(screen.getByText(/ID: 166217243/)).toBeInTheDocument();
    expect(screen.queryByText(/e\+08/)).not.toBeInTheDocument();
  });

  it('calls onEntityClick with entity id and layerType from entities array', () => {
    const { onEntityClick } = renderItem();
    fireEvent.click(screen.getByTestId('notification-item-n1')); // expand
    const entityChip = screen.getByText('Flight ABC123');
    fireEvent.click(entityChip);
    expect(onEntityClick).toHaveBeenCalledWith(
      'flights_commercial:ABC123',
      'flights_commercial',
      mockNotification,
    );
  });

  it('falls back to truncated entityIds when entities array is empty', () => {
    const noEntities = { ...mockNotification, entities: [] };
    renderItem({ notification: noEntities });
    fireEvent.click(screen.getByTestId('notification-item-n1'));
    // Falls back to entityIds — shows first 8 chars of 'entity-1'
    expect(screen.getByText('entity-1')).toBeInTheDocument();
  });

  it('shows different background for unread vs read', () => {
    const { container: unreadContainer } = renderItem({ isRead: false });
    const unreadEl = unreadContainer.querySelector('[data-testid="notification-item-n1"]');
    expect(unreadEl).toBeTruthy();

    const { container: readContainer } = renderItem({ isRead: true });
    const readEl = readContainer.querySelector('[data-testid="notification-item-n1"]');
    expect(readEl).toBeTruthy();
  });

  it('expands on click and shows observation chips', () => {
    renderItem();
    fireEvent.click(screen.getByTestId('notification-item-n1'));

    // After expanding, observation chips should appear
    expect(screen.getByText(/obs:/)).toBeInTheDocument();
  });

  it('calls onObservationClick when observation chip is clicked', () => {
    const { onObservationClick } = renderItem();
    fireEvent.click(screen.getByTestId('notification-item-n1')); // expand
    const obsChip = screen.getByText(/obs:/);
    fireEvent.click(obsChip);
    expect(onObservationClick).toHaveBeenCalledWith('obs-1', 'flights_commercial');
  });

  it('applies mobile min-height and larger chips when isMobile', () => {
    const { container } = renderItem({ isMobile: true });
    fireEvent.click(screen.getByTestId('notification-item-n1')); // expand to show chips
    const itemEl = container.querySelector('[data-testid="notification-item-n1"]');
    expect(itemEl).toBeTruthy();
  });
});
