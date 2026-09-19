import { describe, it, expect } from 'vitest';
import { buildFilterParams } from './buildFilterParams';

describe('buildFilterParams', () => {
  it('sets limit and offset', () => {
    const qs = buildFilterParams({ minAttention: '', insightTypes: [] }, 20, 0);
    const p = new URLSearchParams(qs);
    expect(p.get('limit')).toBe('20');
    expect(p.get('offset')).toBe('0');
  });

  it('sets min_attention for valid attention level', () => {
    const qs = buildFilterParams({ minAttention: 'medium', insightTypes: [] }, 20, 0);
    const p = new URLSearchParams(qs);
    expect(p.get('min_attention')).toBe('3');
  });

  it('omits min_attention for empty string', () => {
    const qs = buildFilterParams({ minAttention: '', insightTypes: [] }, 10, 5);
    const p = new URLSearchParams(qs);
    expect(p.has('min_attention')).toBe(false);
  });

  it('omits min_attention for unknown level', () => {
    const qs = buildFilterParams({ minAttention: 'unknown', insightTypes: [] }, 10, 0);
    const p = new URLSearchParams(qs);
    expect(p.has('min_attention')).toBe(false);
  });

  it('sets insight_type when exactly one type', () => {
    const qs = buildFilterParams({ minAttention: '', insightTypes: ['anomaly'] }, 20, 0);
    const p = new URLSearchParams(qs);
    expect(p.get('insight_type')).toBe('anomaly');
  });

  it('omits insight_type when multiple types', () => {
    const qs = buildFilterParams(
      { minAttention: '', insightTypes: ['anomaly', 'correlation'] },
      20,
      0,
    );
    const p = new URLSearchParams(qs);
    expect(p.has('insight_type')).toBe(false);
  });

  it('omits insight_type when no types', () => {
    const qs = buildFilterParams({ minAttention: '', insightTypes: [] }, 20, 0);
    const p = new URLSearchParams(qs);
    expect(p.has('insight_type')).toBe(false);
  });

  it('handles all attention levels correctly', () => {
    const levels = { info: 1, low: 2, medium: 3, high: 4, critical: 5 };
    for (const [name, value] of Object.entries(levels)) {
      const qs = buildFilterParams({ minAttention: name, insightTypes: [] }, 20, 0);
      const p = new URLSearchParams(qs);
      expect(p.get('min_attention')).toBe(String(value));
    }
  });
});
