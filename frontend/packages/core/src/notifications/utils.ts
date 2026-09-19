import type { AIInsightNotification } from '../models';
import { theme, alpha, semanticColors } from '../theme';

/** Maps attention level to a left-border color for NotificationItem. */
export const ATTENTION_COLORS: Record<string, string> = {
  critical: theme.palette.secondary.main,
  high: '#ff6b00',
  medium: '#ffaa00',
  low: alpha(theme.palette.primary.main, 0.5),
  info: semanticColors.primary.alpha20,
};

/** Returns attention border color, defaulting to info. */
export function attentionColor(attention?: string): string {
  return ATTENTION_COLORS[attention ?? 'info'] ?? ATTENTION_COLORS.info;
}

/**
 * Converts an operation_name like "flight_anomaly_scan" to title case:
 * "Flight Anomaly Scan". Used as fallback title when result.title is missing.
 */
export function humanizeOperationName(name: string): string {
  return name
    .split('_')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

/** Returns a relative time string like "2m ago", "1h ago", "3d ago". */
export function relativeTime(isoString: string): string {
  const diffMs = Date.now() - new Date(isoString).getTime();
  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/**
 * Extracts a human-readable title from the notification result.
 * The backend is the single source of truth — it sets result.title.
 * Falls back to humanizeOperationName when title is missing.
 */
export function extractTitle(notification: AIInsightNotification): string {
  const title = notification.result.title;
  if (typeof title === 'string' && title.length > 0) return title;
  return humanizeOperationName(notification.operationName);
}

/**
 * Extracts a description from the result object.
 * Checks common result field names — no insightType switch needed.
 * The backend may use different field names across analysis types;
 * this list covers the known conventions.
 */
export const DESCRIPTION_KEYS = [
  'description',
  'summary',
  'assessment',
  'disruption_assessment',
  'health_impact',
  'damage_probability',
] as const;

export function extractDescription(result: Record<string, unknown>): string {
  for (const key of DESCRIPTION_KEYS) {
    const v = result[key];
    if (typeof v === 'string' && v.length > 0) return v;
  }
  return '';
}

/**
 * Fields excluded from result chip display in notification UI
 * (long text, shown elsewhere, or internal).
 */
export const NOTIFICATION_SKIP_KEYS = new Set([
  'attention',
  'entity_external_id',
  'title',
  'description',
  'summary',
  'assessment',
  'disruption_assessment',
  'health_impact',
  'damage_probability',
  'redundancy_notes',
  'public_health_advisory',
]);

/** Single insight type option from the discovery endpoint. */
export interface InsightTypeOption {
  value: string;
  displayName: string;
  sourceName: string;
}

/** Response from GET /v1/ai/notifications/filters. */
export interface NotificationFilterOptions {
  insightTypes: InsightTypeOption[];
  attentionLevels: string[];
  layerTypes: string[];
}
