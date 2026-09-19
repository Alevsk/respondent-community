import type { AIInsightNotification } from '../models';

/* eslint-disable @typescript-eslint/no-explicit-any */

/** Strips the `ATTENTION_LEVEL_` prefix from proto enum values (e.g. "ATTENTION_LEVEL_HIGH" → "high"). */
function normalizeAttention(value: unknown): string | undefined {
  if (typeof value !== 'string' || !value) return undefined;
  return value.replace(/^ATTENTION_LEVEL_/i, '').toLowerCase();
}

/**
 * Normalizes a raw ai_insight payload (snake_case from WebSocket or REST)
 * into the camelCase AIInsightNotification interface.
 */
export function normalizeInsight(raw: any): AIInsightNotification {
  let result: Record<string, unknown>;
  if (typeof raw.result === 'string') {
    try {
      result = JSON.parse(raw.result);
    } catch {
      result = {};
    }
  } else {
    result = raw.result ?? {};
  }

  return {
    id: raw.id,
    insightType: raw.insight_type ?? raw.insightType ?? '',
    sourceName: raw.source_name ?? raw.sourceName ?? '',
    operationName: raw.operation_name ?? raw.operationName ?? '',
    layerType: raw.layer_type ?? raw.layerType ?? undefined,
    attention:
      normalizeAttention(raw.attention) ??
      normalizeAttention(result.attention as string) ??
      undefined,
    result,
    entityIds: raw.entity_ids ?? raw.entityIds ?? [],
    entities: (raw.entities ?? []).map((e: any) => ({
      id: e.id ?? '',
      externalId: e.external_id ?? e.externalId ?? '',
      name: e.name ?? '',
      layerType: e.layer_type ?? e.layerType ?? '',
    })),
    observationIds: raw.observation_ids ?? raw.observationIds ?? [],
    createdAt: raw.created_at ?? raw.createdAt ?? '',
  };
}
/* eslint-enable @typescript-eslint/no-explicit-any */
