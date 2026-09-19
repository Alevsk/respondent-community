import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { theme } from '@respondent/core';
import { useUIStore } from '@/app/store';
import NotificationBell from './NotificationBell';

function renderBell() {
  return render(
    <ThemeProvider theme={theme}>
      <NotificationBell />
    </ThemeProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState(
    {
      notifications: [],
      notificationIds: new Set(),
      readInsightIds: new Set(),
      unreadCount: 0,
      notificationPanelOpen: false,
      cleanUI: false,
      recordingMode: false,
    },
    false,
  );
});

describe('NotificationBell', () => {
  it('renders the bell icon', () => {
    renderBell();
    expect(screen.getByTestId('notification-bell')).toBeInTheDocument();
  });

  it('shows no badge when unreadCount is 0', () => {
    useUIStore.setState({ unreadCount: 0 }, false);
    renderBell();
    const badge = screen.queryByText('1');
    expect(badge).not.toBeInTheDocument();
  });

  it('shows badge count for unread notifications', () => {
    useUIStore.setState({ unreadCount: 2 }, false);
    renderBell();
    expect(screen.getByText('2')).toBeInTheDocument();
  });

  it('toggles notification panel on click', () => {
    renderBell();
    fireEvent.click(screen.getByTestId('notification-bell'));
    expect(useUIStore.getState().notificationPanelOpen).toBe(true);
  });

  it('is hidden when cleanUI is true', () => {
    useUIStore.setState({ cleanUI: true }, false);
    renderBell();
    expect(screen.queryByTestId('notification-bell')).not.toBeInTheDocument();
  });

  it('is hidden when recordingMode is true', () => {
    useUIStore.setState({ recordingMode: true }, false);
    renderBell();
    expect(screen.queryByTestId('notification-bell')).not.toBeInTheDocument();
  });
});
