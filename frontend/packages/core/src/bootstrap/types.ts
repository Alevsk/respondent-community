// Bootstrap types — shared across apps

export interface BootstrapStep {
  key: string;
  label: string;
  status: 'pending' | 'loading' | 'success' | 'error';
}

export interface BootstrapState {
  steps: BootstrapStep[];
  /** All steps succeeded. */
  ready: boolean;
  /** Any step exhausted retries or timed out. */
  error: boolean;
  /** Label of the first step still loading, or the first error step. */
  currentLabel: string;
  /** Refetch failed queries + reconnect WS if timed out. */
  retry: () => void;
}

export interface AIInsightsResponse {
  insights: Array<Record<string, unknown>>;
  totalCount: number;
}
