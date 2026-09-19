import { describe, it, expect } from 'vitest';
import { normalizeInsight } from '@respondent/core';
import type { AIInsightNotification } from '@respondent/core';

describe('normalizeInsight', () => {
  it('converts snake_case payload to camelCase', () => {
    const raw = {
      id: 'abc-123',
      insight_type: 'anomaly',
      source_name: 'flight_anomaly_detection',
      operation_name: 'flight_anomaly_scan',
      layer_type: 'flights_commercial',
      attention: 'high',
      result: { title: 'Test', description: 'A test insight' },
      entity_ids: ['entity-1'],
      observation_ids: ['obs-1'],
      created_at: '2026-03-31T12:00:00Z',
    };

    const result: AIInsightNotification = normalizeInsight(raw);

    expect(result).toEqual({
      id: 'abc-123',
      insightType: 'anomaly',
      sourceName: 'flight_anomaly_detection',
      operationName: 'flight_anomaly_scan',
      layerType: 'flights_commercial',
      attention: 'high',
      result: { title: 'Test', description: 'A test insight' },
      entityIds: ['entity-1'],
      entities: [],
      observationIds: ['obs-1'],
      createdAt: '2026-03-31T12:00:00Z',
    });
  });

  it('handles missing optional fields', () => {
    const raw = {
      id: 'abc-123',
      insight_type: 'summary',
      source_name: 'source',
      operation_name: 'op',
      result: { summary: 'hello' },
      created_at: '2026-03-31T12:00:00Z',
    };

    const result = normalizeInsight(raw);

    expect(result.layerType).toBeUndefined();
    expect(result.attention).toBeUndefined();
    expect(result.entityIds).toEqual([]);
    expect(result.entities).toEqual([]);
    expect(result.observationIds).toEqual([]);
  });

  it('parses result from JSON string', () => {
    const raw = {
      id: 'abc-123',
      insight_type: 'anomaly',
      source_name: 's',
      operation_name: 'o',
      result: '{"title":"Parsed"}',
      entity_ids: [],
      observation_ids: [],
      created_at: '2026-03-31T12:00:00Z',
    };

    const result = normalizeInsight(raw);

    expect(result.result).toEqual({ title: 'Parsed' });
  });

  it('returns empty object for malformed result JSON string', () => {
    const raw = {
      id: 'abc-123',
      insight_type: 'anomaly',
      source_name: 's',
      operation_name: 'o',
      result: 'not-json',
      entity_ids: [],
      observation_ids: [],
      created_at: '2026-03-31T12:00:00Z',
    };

    const result = normalizeInsight(raw);

    expect(result.result).toEqual({});
  });
});
