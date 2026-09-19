/** Human-readable relative timestamp. */
export function timeAgo(timestampMs: number): string {
  const diff = Date.now() - timestampMs;
  if (diff < 60_000) return 'just now';
  const mins = Math.floor(diff / 60_000);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  return `${hours}h ago`;
}
