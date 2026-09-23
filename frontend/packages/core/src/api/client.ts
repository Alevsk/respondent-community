// API Client Configuration

// Default to same-origin RELATIVE URLs (`/v1/layers`). The community binary serves
// the UI and the API on one origin (so the embedded/prod frontend must call the
// origin it was served from, NOT a hardcoded host), and the Vite dev server proxies
// /v1 + /ws to the backend (see vite.config.ts). Set VITE_API_URL to a full origin
// only for split deployments where the API lives on a different host.
const API_BASE_URL = import.meta.env.VITE_API_URL ?? '';

// For WebSocket: if VITE_WS_URL is empty/relative, derive from current page origin
// This allows remote access via ngrok/tunnels where localhost isn't reachable
// `||` so that an empty string falls through to the origin-derived default
// (an empty WS URL is meaningless — it must come from somewhere).
const WS_BASE_URL =
  import.meta.env.VITE_WS_URL ||
  (() => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${window.location.host}`;
  })();

// API endpoints
export const endpoints = {
  layers: `${API_BASE_URL}/v1/layers`,
  entities: `${API_BASE_URL}/v1/entities`,
  scenes: `${API_BASE_URL}/v1/scenes`,
  filters: `${API_BASE_URL}/v1/filters`,
  cctv: `${API_BASE_URL}/v1/cctv`,
  mediaPlayback: `${API_BASE_URL}/v1/media/playback`,
  aiInsights: `${API_BASE_URL}/v1/ai/insights`,
  aiNotificationFilters: `${API_BASE_URL}/v1/ai/notifications/filters`,
};

// WebSocket connection
export const WS_URL = `${WS_BASE_URL}/ws`;

// Helper for making API calls
async function apiCall<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(endpoint, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.statusText}`);
  }

  return response.json();
}

export const api = {
  get: <T>(endpoint: string) => apiCall<T>(endpoint),
  post: <T>(endpoint: string, data: unknown) =>
    apiCall<T>(endpoint, { method: 'POST', body: JSON.stringify(data) }),
  put: <T>(endpoint: string, data: unknown) =>
    apiCall<T>(endpoint, { method: 'PUT', body: JSON.stringify(data) }),
  delete: <T>(endpoint: string) => apiCall<T>(endpoint, { method: 'DELETE' }),
};
