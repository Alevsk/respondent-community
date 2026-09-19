import type { AIInsightNotification, NotificationFilterState } from '../models/notification';
import type { SliceCreator } from './types';

// --- localStorage persistence helpers ---

const STORAGE_KEY_READ_INSIGHTS = 'respondent:readInsightIds';
const STORAGE_KEY_NOTIFICATION_FILTER = 'respondent:notificationFilter';

/** Default notification filter — medium attention and above, all insight types. */
export const DEFAULT_NOTIFICATION_FILTER: NotificationFilterState = {
  minAttention: 'medium',
  insightTypes: [],
};

/** Hydrate read insight IDs from localStorage. */
function loadReadInsightIds(): Set<string> {
  try {
    const stored =
      typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY_READ_INSIGHTS) : null;
    return new Set(stored ? JSON.parse(stored) : []);
  } catch {
    return new Set();
  }
}

function persistReadInsightIds(ids: Set<string>): void {
  try {
    localStorage.setItem(STORAGE_KEY_READ_INSIGHTS, JSON.stringify([...ids]));
  } catch {
    /* noop */
  }
}

/** Hydrate notification filter from localStorage. */
function loadNotificationFilter(): NotificationFilterState {
  try {
    const raw =
      typeof localStorage !== 'undefined'
        ? localStorage.getItem(STORAGE_KEY_NOTIFICATION_FILTER)
        : null;
    if (raw) return { ...DEFAULT_NOTIFICATION_FILTER, ...JSON.parse(raw) };
  } catch {
    /* noop */
  }
  return { ...DEFAULT_NOTIFICATION_FILTER };
}

function persistNotificationFilter(filter: NotificationFilterState): void {
  try {
    localStorage.setItem(STORAGE_KEY_NOTIFICATION_FILTER, JSON.stringify(filter));
  } catch {
    /* noop */
  }
}

// --- Slice definition ---

/** State + actions for AI insight notifications. */
export interface NotificationSlice {
  notifications: AIInsightNotification[];
  notificationIds: Set<string>;
  readInsightIds: Set<string>;
  unreadCount: number;
  notificationPanelOpen: boolean;
  notificationsTotalCount: number;
  notificationsLoading: boolean;
  notificationFilter: NotificationFilterState;
  addNotification: (notification: AIInsightNotification) => void;
  mergeNotifications: (items: AIInsightNotification[], total: number) => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
  toggleNotificationPanel: () => void;
  setNotificationPanelOpen: (open: boolean) => void;
  setNotificationsLoading: (loading: boolean) => void;
  setNotificationFilter: (filter: Partial<NotificationFilterState>) => void;
  clearNotifications: () => void;
}

export const createNotificationSlice: SliceCreator<NotificationSlice> = (set) => ({
  notifications: [],
  notificationIds: new Set<string>(),
  readInsightIds: loadReadInsightIds(),
  unreadCount: 0,
  notificationPanelOpen: false,
  notificationsTotalCount: 0,
  notificationsLoading: false,
  notificationFilter: loadNotificationFilter(),
  addNotification: (notification) =>
    set((state) => {
      if (state.notificationIds.has(notification.id)) return state;
      const nextIds = new Set(state.notificationIds);
      nextIds.add(notification.id);
      const isUnread = !state.readInsightIds.has(notification.id);
      return {
        notifications: [notification, ...state.notifications],
        notificationIds: nextIds,
        unreadCount: state.unreadCount + (isUnread ? 1 : 0),
      };
    }),
  mergeNotifications: (items, total) =>
    set((state) => {
      const nextIds = new Set(state.notificationIds);
      const newItems: AIInsightNotification[] = [];
      let addedUnread = 0;
      for (const item of items) {
        if (!nextIds.has(item.id)) {
          nextIds.add(item.id);
          newItems.push(item);
          if (!state.readInsightIds.has(item.id)) addedUnread++;
        }
      }
      if (newItems.length === 0) return { notificationsTotalCount: total };
      return {
        notifications: [...state.notifications, ...newItems],
        notificationIds: nextIds,
        notificationsTotalCount: total,
        unreadCount: state.unreadCount + addedUnread,
      };
    }),
  markRead: (id) =>
    set((state) => {
      if (state.readInsightIds.has(id)) return state;
      const next = new Set(state.readInsightIds);
      next.add(id);
      persistReadInsightIds(next);
      const wasUnread = state.notificationIds.has(id);
      return { readInsightIds: next, unreadCount: state.unreadCount - (wasUnread ? 1 : 0) };
    }),
  markAllRead: () =>
    set((state) => {
      if (state.unreadCount === 0) return state;
      const next = new Set(state.readInsightIds);
      for (const n of state.notifications) next.add(n.id);
      persistReadInsightIds(next);
      return { readInsightIds: next, unreadCount: 0 };
    }),
  toggleNotificationPanel: () =>
    set((state) => ({ notificationPanelOpen: !state.notificationPanelOpen })),
  setNotificationPanelOpen: (open) => set({ notificationPanelOpen: open }),
  setNotificationsLoading: (loading) => set({ notificationsLoading: loading }),
  setNotificationFilter: (partial) =>
    set((state) => {
      const next = { ...state.notificationFilter, ...partial };
      persistNotificationFilter(next);
      return { notificationFilter: next };
    }),
  clearNotifications: () =>
    set({
      notifications: [],
      notificationIds: new Set<string>(),
      notificationsTotalCount: 0,
      unreadCount: 0,
    }),
});
