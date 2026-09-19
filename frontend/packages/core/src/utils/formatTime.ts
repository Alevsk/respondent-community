/**
 * Shared time formatting utilities.
 *
 * Pure functions — no React or DOM dependencies.
 */

const AVG_DAYS_PER_MONTH = 30.44;
const AVG_DAYS_PER_YEAR = 365.25;

/**
 * Format a Unix-epoch millisecond timestamp as a human-friendly relative
 * string ("just now", "5m ago", "yesterday", "3 weeks ago", etc.).
 */
export function formatRelativeTime(ts: number): string {
  const deltaMs = Date.now() - ts;
  if (deltaMs < 0) return 'just now';
  const seconds = Math.floor(deltaMs / 1000);
  if (seconds < 5) return 'just now';
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'yesterday';
  if (days < 7) return `${days} days ago`;
  if (days < 30) {
    const weeks = Math.floor(days / 7);
    return weeks === 1 ? '1 week ago' : `${weeks} weeks ago`;
  }
  const months = Math.floor(days / AVG_DAYS_PER_MONTH);
  if (months < 12) return months <= 1 ? '1 month ago' : `${months} months ago`;
  const years = Math.floor(days / AVG_DAYS_PER_YEAR);
  return years <= 1 ? '1 year ago' : `${years} years ago`;
}

/**
 * Format a Unix-epoch millisecond timestamp as a locale-aware absolute
 * date+time string, suitable for tooltip display.
 *
 * Example output: "Mar 12, 2026, 02:15:30 PM EST"
 */
export function formatAbsoluteTime(ts: number): string {
  return new Date(ts).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short',
  });
}

/**
 * Format a Unix-epoch millisecond timestamp as a UTC time string (HH:mm:ssZ).
 * Returns '—' for invalid (zero/negative) timestamps.
 */
export function formatUtcTime(ts: number): string {
  if (ts <= 0) return '—';
  return `${new Date(ts).toISOString().slice(11, 19)}Z`;
}

/**
 * Format a Unix-epoch millisecond timestamp as an ISO date string (YYYY-MM-DD).
 * Returns '—' for invalid (zero/negative) timestamps.
 */
export function formatUtcDate(ts: number): string {
  if (ts <= 0) return '—';
  return new Date(ts).toISOString().slice(0, 10);
}
