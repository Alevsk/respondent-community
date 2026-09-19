import { useQuery } from '@tanstack/react-query';
import { api, endpoints } from '@respondent/core';
import type { NotificationFilterOptions } from '@respondent/core';

interface RawInsightType {
  value: string;
  display_name?: string;
  displayName?: string;
  source_name?: string;
  sourceName?: string;
}

interface RawFilterOptionsResponse {
  insightTypes?: RawInsightType[];
  insight_types?: RawInsightType[];
  attentionLevels?: string[];
  attention_levels?: string[];
  layerTypes?: string[];
  layer_types?: string[];
}

function normalize(raw: RawFilterOptionsResponse): NotificationFilterOptions {
  const types = raw.insightTypes ?? raw.insight_types ?? [];
  return {
    insightTypes: types.map((t) => ({
      value: t.value,
      displayName: t.displayName ?? t.display_name ?? t.value,
      sourceName: t.sourceName ?? t.source_name ?? '',
    })),
    attentionLevels: raw.attentionLevels ?? raw.attention_levels ?? [],
    layerTypes: raw.layerTypes ?? raw.layer_types ?? [],
  };
}

/**
 * Fetches available notification filter options from the discovery endpoint.
 * Cached for 5 minutes — options change rarely (only when analysis definitions reload).
 */
export function useNotificationFilterOptions() {
  return useQuery({
    queryKey: ['notification-filter-options'],
    queryFn: async () => {
      const raw = await api.get<RawFilterOptionsResponse>(endpoints.aiNotificationFilters);
      return normalize(raw);
    },
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
    retry: 1,
  });
}
