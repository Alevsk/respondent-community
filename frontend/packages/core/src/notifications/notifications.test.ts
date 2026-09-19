import { describe, it, expect } from 'vitest';
import {
  ATTENTION_COLORS,
  attentionColor,
  humanizeOperationName,
  relativeTime,
  extractTitle,
  extractDescription,
  DESCRIPTION_KEYS,
} from './utils';
import { DEFAULT_NOTIFICATION_FILTER } from '../stores';
import { normalizeInsight } from './normalizeInsight';
import type { AIInsightNotification } from '../models';

describe('ATTENTION_COLORS', () => {
  it('has entries for all standard levels', () => {
    expect(ATTENTION_COLORS.critical).toBe('#ff006e');
    expect(ATTENTION_COLORS.high).toBe('#ff6b00');
    expect(ATTENTION_COLORS.medium).toBe('#ffaa00');
    expect(ATTENTION_COLORS.low).toBe('rgba(0, 255, 157, 0.5)');
    expect(ATTENTION_COLORS.info).toBe('rgba(0, 255, 157, 0.2)');
  });
});

describe('attentionColor', () => {
  it('returns the color for a known level', () => {
    expect(attentionColor('critical')).toBe('#ff006e');
    expect(attentionColor('high')).toBe('#ff6b00');
  });

  it('defaults to info color when level is undefined', () => {
    expect(attentionColor(undefined)).toBe(ATTENTION_COLORS.info);
  });

  it('defaults to info color for unknown level', () => {
    expect(attentionColor('unknown')).toBe(ATTENTION_COLORS.info);
  });
});

describe('humanizeOperationName', () => {
  it('converts snake_case to Title Case', () => {
    expect(humanizeOperationName('flight_anomaly_scan')).toBe('Flight Anomaly Scan');
  });

  it('handles single word', () => {
    expect(humanizeOperationName('scan')).toBe('Scan');
  });

  it('handles already-titlecased word', () => {
    expect(humanizeOperationName('Flight')).toBe('Flight');
  });
});

describe('relativeTime', () => {
  it('returns seconds for recent timestamps', () => {
    const now = new Date(Date.now() - 30 * 1000).toISOString();
    expect(relativeTime(now)).toBe('30s ago');
  });

  it('returns minutes for timestamps 1–59 minutes ago', () => {
    const now = new Date(Date.now() - 5 * 60 * 1000).toISOString();
    expect(relativeTime(now)).toBe('5m ago');
  });

  it('returns hours for timestamps 1–23 hours ago', () => {
    const now = new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString();
    expect(relativeTime(now)).toBe('3h ago');
  });

  it('returns days for timestamps 24+ hours ago', () => {
    const now = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString();
    expect(relativeTime(now)).toBe('2d ago');
  });
});

describe('extractTitle', () => {
  const base: AIInsightNotification = {
    id: '1',
    insightType: 'flight_anomaly',
    sourceName: 'adsb',
    operationName: 'flight_anomaly_scan',
    result: {},
    entityIds: [],
    entities: [],
    observationIds: [],
    createdAt: '',
  };

  it('returns result.title when present', () => {
    const n = { ...base, result: { title: 'Custom Title' } };
    expect(extractTitle(n)).toBe('Custom Title');
  });

  it('falls back to humanizeOperationName when title is absent', () => {
    expect(extractTitle(base)).toBe('Flight Anomaly Scan');
  });

  it('falls back when title is an empty string', () => {
    const n = { ...base, result: { title: '' } };
    expect(extractTitle(n)).toBe('Flight Anomaly Scan');
  });
});

describe('extractDescription', () => {
  it('returns description field when present', () => {
    expect(extractDescription({ description: 'A description' })).toBe('A description');
  });

  it('returns summary when description is absent', () => {
    expect(extractDescription({ summary: 'A summary' })).toBe('A summary');
  });

  it('returns assessment when earlier fields are absent', () => {
    expect(extractDescription({ assessment: 'An assessment' })).toBe('An assessment');
  });

  it('returns empty string when no known keys are present', () => {
    expect(extractDescription({ other_field: 'ignored' })).toBe('');
  });

  it('skips empty string values', () => {
    expect(extractDescription({ description: '', summary: 'Found' })).toBe('Found');
  });
});

describe('DESCRIPTION_KEYS', () => {
  it('contains the expected keys', () => {
    expect(DESCRIPTION_KEYS).toContain('description');
    expect(DESCRIPTION_KEYS).toContain('summary');
    expect(DESCRIPTION_KEYS).toContain('assessment');
    expect(DESCRIPTION_KEYS).toContain('disruption_assessment');
    expect(DESCRIPTION_KEYS).toContain('health_impact');
    expect(DESCRIPTION_KEYS).toContain('damage_probability');
  });
});

describe('DEFAULT_NOTIFICATION_FILTER', () => {
  it('has minAttention set to medium', () => {
    expect(DEFAULT_NOTIFICATION_FILTER.minAttention).toBe('medium');
  });

  it('has empty insightTypes array', () => {
    expect(DEFAULT_NOTIFICATION_FILTER.insightTypes).toEqual([]);
  });
});

describe('normalizeInsight', () => {
  it('normalizes snake_case fields to camelCase', () => {
    const raw = {
      id: 'abc',
      insight_type: 'flight_anomaly',
      source_name: 'adsb',
      operation_name: 'scan',
      layer_type: 'flight',
      attention: 'ATTENTION_LEVEL_HIGH',
      result: { title: 'Test' },
      entity_ids: ['e1'],
      entities: [{ id: 'e1', external_id: 'ext1', name: 'Entity 1', layer_type: 'flight' }],
      observation_ids: ['o1'],
      created_at: '2024-01-01T00:00:00Z',
    };

    const result = normalizeInsight(raw);

    expect(result.id).toBe('abc');
    expect(result.insightType).toBe('flight_anomaly');
    expect(result.sourceName).toBe('adsb');
    expect(result.operationName).toBe('scan');
    expect(result.layerType).toBe('flight');
    expect(result.attention).toBe('high');
    expect(result.entityIds).toEqual(['e1']);
    expect(result.entities[0].externalId).toBe('ext1');
    expect(result.observationIds).toEqual(['o1']);
    expect(result.createdAt).toBe('2024-01-01T00:00:00Z');
  });

  it('falls back to camelCase fields when snake_case absent', () => {
    const raw = {
      id: 'xyz',
      insightType: 'test',
      sourceName: 'src',
      operationName: 'op',
      result: {},
      entityIds: [],
      entities: [],
      observationIds: [],
      createdAt: '2024-06-01T00:00:00Z',
    };

    const result = normalizeInsight(raw);
    expect(result.insightType).toBe('test');
    expect(result.sourceName).toBe('src');
    expect(result.createdAt).toBe('2024-06-01T00:00:00Z');
  });

  it('strips ATTENTION_LEVEL_ prefix and lowercases', () => {
    const raw = { id: '1', attention: 'ATTENTION_LEVEL_CRITICAL', result: {}, entities: [] };
    expect(normalizeInsight(raw).attention).toBe('critical');
  });

  it('parses JSON string result', () => {
    const raw = {
      id: '1',
      result: JSON.stringify({ title: 'Parsed', description: 'From JSON' }),
      entities: [],
    };
    const result = normalizeInsight(raw);
    expect(result.result.title).toBe('Parsed');
    expect(result.result.description).toBe('From JSON');
  });

  it('falls back to empty object when result JSON is invalid', () => {
    const raw = { id: '1', result: '{bad json', entities: [] };
    const result = normalizeInsight(raw);
    expect(result.result).toEqual({});
  });

  it('reads attention from result object when top-level attention is absent', () => {
    const raw = {
      id: '1',
      result: { attention: 'ATTENTION_LEVEL_MEDIUM' },
      entities: [],
    };
    expect(normalizeInsight(raw).attention).toBe('medium');
  });

  it('normalizes entity fields', () => {
    const raw = {
      id: '1',
      result: {},
      entities: [{ id: 'e1', external_id: 'ext1', name: 'Name', layer_type: 'flight' }],
    };
    const entity = normalizeInsight(raw).entities[0];
    expect(entity.id).toBe('e1');
    expect(entity.externalId).toBe('ext1');
    expect(entity.layerType).toBe('flight');
  });
});
