export interface InsightEntityRef {
  id: string;
  externalId: string;
  name: string;
  layerType: string;
}

export interface AIInsightNotification {
  id: string;
  insightType: string;
  sourceName: string;
  operationName: string;
  layerType?: string;
  attention?: string;
  result: Record<string, unknown>;
  entityIds: string[];
  entities: InsightEntityRef[];
  observationIds: string[];
  createdAt: string;
}

/** Persisted notification filter preferences. */
export interface NotificationFilterState {
  minAttention: string; // Default: 'medium'
  insightTypes: string[]; // Empty = all types
}
