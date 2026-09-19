import { describe, it, expect } from 'vitest';
import { collectClusterInputs, formatRelativeAge } from './clusterRendererUtils';
import type { Entity, Observation } from '@respondent/core';

function entity(id: string): Entity {
  return { id, name: id, layerType: 'flights' } as Entity;
}
function obs(entityId: string, lat: number, lon: number): Observation {
  return { entityId, position: { lat, lon } } as unknown as Observation;
}

describe('collectClusterInputs', () => {
  it('maps entities with a valid observation to clusterable points', () => {
    const entityMap = new Map<string, Entity>([
      ['a', entity('a')],
      ['b', entity('b')],
    ]);
    const obsMap = new Map<string, Observation>([
      ['a', obs('a', 10, 20)],
      ['b', obs('b', 30, 40)],
    ]);
    const points = collectClusterInputs(entityMap, obsMap, 2000);
    expect(points).toEqual([
      { id: 'a', lat: 10, lon: 20 },
      { id: 'b', lat: 30, lon: 40 },
    ]);
  });

  it('skips entities without an observation', () => {
    const entityMap = new Map<string, Entity>([
      ['a', entity('a')],
      ['b', entity('b')],
    ]);
    const obsMap = new Map<string, Observation>([['a', obs('a', 1, 2)]]);
    const points = collectClusterInputs(entityMap, obsMap, 2000);
    expect(points.map((p) => p.id)).toEqual(['a']);
  });

  it('skips observations with non-numeric coordinates', () => {
    const entityMap = new Map<string, Entity>([['a', entity('a')]]);
    const obsMap = new Map<string, Observation>([
      ['a', { entityId: 'a', position: { lat: 'nope', lon: 'x' } } as unknown as Observation],
    ]);
    expect(collectClusterInputs(entityMap, obsMap, 2000)).toEqual([]);
  });

  it('caps the number of points at maxEntities', () => {
    const entityMap = new Map<string, Entity>();
    const obsMap = new Map<string, Observation>();
    for (let i = 0; i < 10; i++) {
      entityMap.set(`e${i}`, entity(`e${i}`));
      obsMap.set(`e${i}`, obs(`e${i}`, i, i));
    }
    expect(collectClusterInputs(entityMap, obsMap, 3)).toHaveLength(3);
  });
});

describe('formatRelativeAge', () => {
  const now = Date.parse('2026-06-29T12:00:00Z');
  it('returns empty string for missing or invalid input', () => {
    expect(formatRelativeAge(undefined, now)).toBe('');
    expect(formatRelativeAge('not-a-date', now)).toBe('');
  });
  it('formats seconds, minutes, hours, and days', () => {
    expect(formatRelativeAge('2026-06-29T11:59:45Z', now)).toBe('15s ago');
    expect(formatRelativeAge('2026-06-29T11:55:00Z', now)).toBe('5m ago');
    expect(formatRelativeAge('2026-06-29T09:00:00Z', now)).toBe('3h ago');
    expect(formatRelativeAge('2026-06-27T12:00:00Z', now)).toBe('2d ago');
  });
  it('clamps future timestamps to 0s', () => {
    expect(formatRelativeAge('2026-06-29T12:00:30Z', now)).toBe('0s ago');
  });
});
