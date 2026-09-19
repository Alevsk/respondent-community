import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { theme, DEFAULT_NOTIFICATION_FILTER } from '@respondent/core';
import type { NotificationFilterOptions } from '@respondent/core';
import { useUIStore } from '@/app/store';
import { NotificationFilterToggle, NotificationFilterPanel } from './NotificationFilterBar';
import { useState } from 'react';

const mockOptions: NotificationFilterOptions = {
  insightTypes: [
    {
      value: 'geopolitical_intel',
      displayName: 'Geopolitical Intel',
      sourceName: 'military_conflict',
    },
    {
      value: 'radiation_anomaly',
      displayName: 'Radiation Anomaly',
      sourceName: 'radiation_monitor',
    },
    {
      value: 'maritime_cable_threat',
      displayName: 'Maritime Cable Threat',
      sourceName: 'cable_monitor',
    },
  ],
  attentionLevels: ['info', 'low', 'medium', 'high', 'critical'],
  layerTypes: ['flights_military', 'ships', 'radiation'],
};

/** Wrapper that pairs the toggle and panel like NotificationPanelContent does. */
function FilterBarHarness({ options }: { options: NotificationFilterOptions }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <NotificationFilterToggle open={open} onToggle={() => setOpen((p) => !p)} />
      {open && <NotificationFilterPanel options={options} />}
    </>
  );
}

function renderFilterBar(options = mockOptions) {
  return render(
    <ThemeProvider theme={theme}>
      <FilterBarHarness options={options} />
    </ThemeProvider>,
  );
}

beforeEach(() => {
  useUIStore.setState({ notificationFilter: { ...DEFAULT_NOTIFICATION_FILTER } }, false);
});

describe('NotificationFilterBar', () => {
  it('renders filter toggle button', () => {
    renderFilterBar();
    expect(screen.getByTestId('notification-filter-toggle')).toBeInTheDocument();
  });

  it('shows filter controls when toggle is clicked', () => {
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));
    expect(screen.getByText('MIN ATTENTION')).toBeInTheDocument();
    expect(screen.getByText(/INSIGHT TYPES/)).toBeInTheDocument();
  });

  it('renders attention level pills', () => {
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));
    expect(screen.getByText('INFO')).toBeInTheDocument();
    expect(screen.getByText('LOW')).toBeInTheDocument();
    expect(screen.getByText('MEDIUM')).toBeInTheDocument();
    expect(screen.getByText('HIGH')).toBeInTheDocument();
    expect(screen.getByText('CRITICAL')).toBeInTheDocument();
  });

  it('toggles attention level pill on click', () => {
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));

    // Click HIGH to select it
    fireEvent.click(screen.getByText('HIGH'));
    let { notificationFilter } = useUIStore.getState();
    expect(notificationFilter.minAttention).toBe('high');

    // Click HIGH again — reverts to default (medium), not empty string
    fireEvent.click(screen.getByText('HIGH'));
    ({ notificationFilter } = useUIStore.getState());
    expect(notificationFilter.minAttention).toBe('medium');
  });

  it('renders all insight type chips from options', () => {
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));
    expect(screen.getByText('Geopolitical Intel')).toBeInTheDocument();
    expect(screen.getByText('Radiation Anomaly')).toBeInTheDocument();
    expect(screen.getByText('Maritime Cable Threat')).toBeInTheDocument();
  });

  it('toggles insight type chip on click', () => {
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));

    // Click a chip to select it
    fireEvent.click(screen.getByText('Radiation Anomaly'));
    let { notificationFilter } = useUIStore.getState();
    expect(notificationFilter.insightTypes).toEqual(['radiation_anomaly']);

    // Click again to deselect it
    fireEvent.click(screen.getByText('Radiation Anomaly'));
    ({ notificationFilter } = useUIStore.getState());
    expect(notificationFilter.insightTypes).toEqual([]);
  });

  it('shows active indicator dot when filter is non-default and panel is closed', () => {
    useUIStore.setState(
      {
        notificationFilter: { minAttention: 'high', insightTypes: [] },
      },
      false,
    );
    renderFilterBar();

    // Panel is closed — badge dot should be visible
    const toggle = screen.getByTestId('notification-filter-toggle');
    expect(toggle.querySelector('[class*="MuiBox"]')).toBeInTheDocument();

    // Open the panel — badge dot hides
    fireEvent.click(toggle);
    expect(screen.getByText('MIN ATTENTION')).toBeInTheDocument();
  });

  it('resets filter to defaults', () => {
    useUIStore.setState(
      {
        notificationFilter: { minAttention: 'critical', insightTypes: ['radiation_anomaly'] },
      },
      false,
    );
    renderFilterBar();
    fireEvent.click(screen.getByTestId('notification-filter-toggle'));
    fireEvent.click(screen.getByText('RESET'));

    const { notificationFilter } = useUIStore.getState();
    expect(notificationFilter.minAttention).toBe('medium');
    expect(notificationFilter.insightTypes).toEqual([]);
  });
});
