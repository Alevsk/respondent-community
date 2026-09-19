import { describe, it, expect, beforeEach } from 'vitest';
import { createStore } from 'zustand/vanilla';
import { createNotificationSlice, type NotificationSlice } from './createNotificationSlice';
import type { AIInsightNotification } from '../models/notification';

const makeStore = () => createStore<NotificationSlice>()(createNotificationSlice);

const mockNotification = (id: string): AIInsightNotification => ({
  id,
  insightType: 'anomaly',
  sourceName: 'flight_monitor',
  operationName: 'flight_anomaly_scan',
  attention: 'medium',
  result: { title: 'Test' },
  entityIds: ['e1'],
  entities: [{ id: 'e1', externalId: 'ext1', name: 'Entity 1', layerType: 'flights' }],
  observationIds: [],
  createdAt: new Date().toISOString(),
});

beforeEach(() => {
  // Clear localStorage between tests
  try {
    localStorage.clear();
  } catch {
    /* noop in non-browser */
  }
});

describe('createNotificationSlice', () => {
  it('defaults to empty notifications', () => {
    const store = makeStore();
    expect(store.getState().notifications).toEqual([]);
    expect(store.getState().unreadCount).toBe(0);
  });

  it('addNotification adds and increments unread count', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    const s = store.getState();
    expect(s.notifications).toHaveLength(1);
    expect(s.unreadCount).toBe(1);
    expect(s.notificationIds.has('n1')).toBe(true);
  });

  it('addNotification deduplicates by id', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().addNotification(mockNotification('n1'));
    expect(store.getState().notifications).toHaveLength(1);
    expect(store.getState().unreadCount).toBe(1);
  });

  it('mergeNotifications appends new items', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().mergeNotifications([mockNotification('n1'), mockNotification('n2')], 10);
    const s = store.getState();
    expect(s.notifications).toHaveLength(2);
    expect(s.notificationsTotalCount).toBe(10);
    expect(s.unreadCount).toBe(2);
  });

  it('mergeNotifications with all duplicates only updates total', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().mergeNotifications([mockNotification('n1')], 5);
    expect(store.getState().notificationsTotalCount).toBe(5);
    expect(store.getState().notifications).toHaveLength(1);
  });

  it('markRead decrements unread and persists', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().markRead('n1');
    expect(store.getState().unreadCount).toBe(0);
    expect(store.getState().readInsightIds.has('n1')).toBe(true);
  });

  it('markRead is idempotent', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().markRead('n1');
    store.getState().markRead('n1');
    expect(store.getState().unreadCount).toBe(0);
  });

  it('markAllRead marks everything as read', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().addNotification(mockNotification('n2'));
    store.getState().markAllRead();
    expect(store.getState().unreadCount).toBe(0);
    expect(store.getState().readInsightIds.has('n1')).toBe(true);
    expect(store.getState().readInsightIds.has('n2')).toBe(true);
  });

  it('toggleNotificationPanel toggles open state', () => {
    const store = makeStore();
    expect(store.getState().notificationPanelOpen).toBe(false);
    store.getState().toggleNotificationPanel();
    expect(store.getState().notificationPanelOpen).toBe(true);
    store.getState().toggleNotificationPanel();
    expect(store.getState().notificationPanelOpen).toBe(false);
  });

  it('setNotificationFilter merges and persists filter', () => {
    const store = makeStore();
    store.getState().setNotificationFilter({ minAttention: 'high' });
    expect(store.getState().notificationFilter.minAttention).toBe('high');
    expect(store.getState().notificationFilter.insightTypes).toEqual([]);
  });

  it('clearNotifications resets notification state', () => {
    const store = makeStore();
    store.getState().addNotification(mockNotification('n1'));
    store.getState().clearNotifications();
    const s = store.getState();
    expect(s.notifications).toEqual([]);
    expect(s.notificationIds.size).toBe(0);
    expect(s.unreadCount).toBe(0);
    expect(s.notificationsTotalCount).toBe(0);
  });
});
