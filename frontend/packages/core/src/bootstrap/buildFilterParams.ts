// Notification backfill query string builder
import { ATTENTION_ENUM } from './constants';

export function buildFilterParams(
  filter: { minAttention: string; insightTypes: string[] },
  limit: number,
  offset: number,
): string {
  const params = new URLSearchParams();
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  if (filter.minAttention && ATTENTION_ENUM[filter.minAttention]) {
    params.set('min_attention', String(ATTENTION_ENUM[filter.minAttention]));
  }
  if (filter.insightTypes.length === 1) {
    params.set('insight_type', filter.insightTypes[0]);
  }
  return params.toString();
}
