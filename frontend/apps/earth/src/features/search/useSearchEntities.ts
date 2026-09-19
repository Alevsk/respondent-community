/**
 * useSearchEntities — React Query hook for the entity search API.
 *
 * Calls GET /v1/entities/search?query=...&layer_type=...&limit=20
 * Disabled when the query is empty or shorter than 2 characters.
 * The component is responsible for debouncing the query value before
 * passing it to this hook.
 */

import { useQuery } from '@tanstack/react-query';
import { api, endpoints } from '@respondent/core';
import { queryKeys } from '../../shared/api/queries';

// Matches the proto EntitySearchResult message (gRPC-gateway returns camelCase JSON)
export interface EntitySearchResult {
  entityId: string;
  externalId: string;
  layerType: string;
  name: string;
  latestObservation?: {
    entityId: string;
    ts: string;
    position: { lat: number; lon: number; altM: number };
    altitudeM: number;
    velocity?: Record<string, number>;
  };
  metadata: Record<string, string>;
}

export interface SearchEntitiesResponse {
  results: EntitySearchResult[];
  totalCount: number;
}

export function useSearchEntities(query: string, layerType?: string, limit = 20) {
  const trimmed = query.trim();

  return useQuery({
    queryKey: queryKeys.entitySearch(trimmed, layerType, limit),
    queryFn: async () => {
      const params = new URLSearchParams({ query: trimmed, limit: String(limit) });
      if (layerType) params.set('layer_type', layerType);
      const raw = await api.get<SearchEntitiesResponse>(
        `${endpoints.entities}/search?${params.toString()}`,
      );
      return {
        results: raw.results ?? [],
        totalCount: raw.totalCount ?? 0,
      };
    },
    enabled: trimmed.length >= 2,
    staleTime: 30_000, // Cache search results for 30s to avoid redundant fetches during typeahead
  });
}
