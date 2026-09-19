// Bootstrap constants — shared across apps

export const WS_TIMEOUT_MS = 10_000;

export const STEP_DEFS = [
  { key: 'layers', label: 'LOADING LAYERS...' },
  { key: 'notifications', label: 'LOADING NOTIFICATIONS...' },
  { key: 'websocket', label: 'CONNECTING STREAM...' },
] as const;

export const ATTENTION_ENUM: Record<string, number> = {
  info: 1,
  low: 2,
  medium: 3,
  high: 4,
  critical: 5,
};
