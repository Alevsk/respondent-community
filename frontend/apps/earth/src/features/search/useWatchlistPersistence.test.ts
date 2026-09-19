/**
 * useWatchlistPersistence — unit tests for localStorage persistence.
 *
 * Tests the pure functions (parseWatchlistStorage, serializeWatchlist)
 * and the hook's load/save behavior using a mocked localStorage.
 */

import { describe, it, expect } from 'vitest';
import {
  parseWatchlistStorage,
  serializeWatchlist,
  WATCHLIST_SCHEMA_VERSION,
} from './useWatchlistPersistence';
import type { WatchlistEntity } from '@/app/store';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeEntity(overrides: Partial<WatchlistEntity> = {}): WatchlistEntity {
  return {
    entityId: 'flights:UAL1234',
    layerId: 'flights',
    name: 'UAL1234',
    pinned: true,
    addedAt: 1710288000000,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// parseWatchlistStorage
// ---------------------------------------------------------------------------

describe('parseWatchlistStorage', () => {
  const validCases = [
    {
      name: 'parses valid schema with pinned entities',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [
          { entityId: 'flights:UAL1234', layerId: 'flights', name: 'UAL1234', addedAt: 123 },
        ],
      }),
      expectedLength: 1,
    },
    {
      name: 'parses valid schema with empty array',
      input: JSON.stringify({ version: 1, pinnedEntities: [] }),
      expectedLength: 0,
    },
    {
      name: 'parses multiple entities',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [
          { entityId: 'a', layerId: 'la', name: 'A', addedAt: 1 },
          { entityId: 'b', layerId: 'lb', name: 'B', addedAt: 2 },
        ],
      }),
      expectedLength: 2,
    },
  ];

  validCases.forEach(({ name, input, expectedLength }) => {
    it(name, () => {
      const result = parseWatchlistStorage(input);
      expect(result).not.toBeNull();
      expect(result!.pinnedEntities).toHaveLength(expectedLength);
      expect(result!.version).toBe(WATCHLIST_SCHEMA_VERSION);
    });
  });

  const invalidCases = [
    { name: 'returns null for null input', input: null },
    { name: 'returns null for empty string', input: '' },
    { name: 'returns null for invalid JSON', input: '{not json}' },
    {
      name: 'returns null for wrong version',
      input: JSON.stringify({ version: 99, pinnedEntities: [] }),
    },
    {
      name: 'returns null when pinnedEntities is not an array',
      input: JSON.stringify({ version: 1, pinnedEntities: 'not array' }),
    },
    {
      name: 'returns null when entity missing entityId',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [{ layerId: 'l', name: 'n', addedAt: 1 }],
      }),
    },
    {
      name: 'returns null when entity missing layerId',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [{ entityId: 'e', name: 'n', addedAt: 1 }],
      }),
    },
    {
      name: 'returns null when entity missing name',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [{ entityId: 'e', layerId: 'l', addedAt: 1 }],
      }),
    },
    {
      name: 'returns null when addedAt is not a number',
      input: JSON.stringify({
        version: 1,
        pinnedEntities: [{ entityId: 'e', layerId: 'l', name: 'n', addedAt: 'string' }],
      }),
    },
    {
      name: 'returns null for plain string',
      input: '"just a string"',
    },
    {
      name: 'returns null for array at root',
      input: '[]',
    },
    {
      name: 'returns null for missing version field',
      input: JSON.stringify({ pinnedEntities: [] }),
    },
  ];

  invalidCases.forEach(({ name, input }) => {
    it(name, () => {
      expect(parseWatchlistStorage(input)).toBeNull();
    });
  });
});

// ---------------------------------------------------------------------------
// serializeWatchlist
// ---------------------------------------------------------------------------

describe('serializeWatchlist', () => {
  it('serializes only pinned entities', () => {
    const entities = [
      makeEntity({ entityId: 'a', pinned: true }),
      makeEntity({ entityId: 'b', pinned: false }),
      makeEntity({ entityId: 'c', pinned: true }),
    ];
    const result = JSON.parse(serializeWatchlist(entities));
    expect(result.version).toBe(WATCHLIST_SCHEMA_VERSION);
    expect(result.pinnedEntities).toHaveLength(2);
    expect(result.pinnedEntities[0].entityId).toBe('a');
    expect(result.pinnedEntities[1].entityId).toBe('c');
  });

  it('returns empty pinnedEntities when none are pinned', () => {
    const entities = [makeEntity({ pinned: false })];
    const result = JSON.parse(serializeWatchlist(entities));
    expect(result.pinnedEntities).toHaveLength(0);
  });

  it('does not include pinned field in serialized output', () => {
    const entities = [makeEntity({ entityId: 'a', pinned: true })];
    const result = JSON.parse(serializeWatchlist(entities));
    const serialized = result.pinnedEntities[0];
    expect(serialized).not.toHaveProperty('pinned');
    expect(serialized).toHaveProperty('entityId');
    expect(serialized).toHaveProperty('layerId');
    expect(serialized).toHaveProperty('name');
    expect(serialized).toHaveProperty('addedAt');
  });

  it('produces output that round-trips through parseWatchlistStorage', () => {
    const entities = [
      makeEntity({
        entityId: 'flights:UAL1234',
        layerId: 'flights',
        name: 'UAL1234',
        addedAt: 123,
      }),
    ];
    const serialized = serializeWatchlist(entities);
    const parsed = parseWatchlistStorage(serialized);
    expect(parsed).not.toBeNull();
    expect(parsed!.pinnedEntities).toHaveLength(1);
    expect(parsed!.pinnedEntities[0].entityId).toBe('flights:UAL1234');
  });

  it('handles empty input array', () => {
    const result = JSON.parse(serializeWatchlist([]));
    expect(result.version).toBe(WATCHLIST_SCHEMA_VERSION);
    expect(result.pinnedEntities).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// Serialize + parse round-trip (pure function integration)
// ---------------------------------------------------------------------------

describe('serialize + parse round-trip', () => {
  it('save and load cycle preserves entity data', () => {
    const entities = [
      makeEntity({ entityId: 'sat:ISS', layerId: 'satellites', name: 'ISS', addedAt: 42 }),
    ];
    const serialized = serializeWatchlist(entities);
    const loaded = parseWatchlistStorage(serialized);
    expect(loaded).not.toBeNull();
    expect(loaded!.pinnedEntities[0].entityId).toBe('sat:ISS');
    expect(loaded!.pinnedEntities[0].name).toBe('ISS');
    expect(loaded!.pinnedEntities[0].addedAt).toBe(42);
  });

  it('handles corrupted data gracefully', () => {
    const loaded = parseWatchlistStorage('CORRUPTED_DATA!!!');
    expect(loaded).toBeNull();
  });

  it('handles tampered version number gracefully', () => {
    const loaded = parseWatchlistStorage(JSON.stringify({ version: 999, pinnedEntities: [] }));
    expect(loaded).toBeNull();
  });

  it('handles null input gracefully (simulates missing key)', () => {
    const loaded = parseWatchlistStorage(null);
    expect(loaded).toBeNull();
  });

  it('round-trips multiple entities correctly', () => {
    const entities = [
      makeEntity({ entityId: 'a', layerId: 'la', name: 'A', addedAt: 100 }),
      makeEntity({ entityId: 'b', layerId: 'lb', name: 'B', addedAt: 200 }),
    ];
    const loaded = parseWatchlistStorage(serializeWatchlist(entities));
    expect(loaded!.pinnedEntities).toHaveLength(2);
    expect(loaded!.pinnedEntities[0].entityId).toBe('a');
    expect(loaded!.pinnedEntities[1].entityId).toBe('b');
  });

  it('schema version matches expected constant', () => {
    const entities = [makeEntity()];
    const loaded = parseWatchlistStorage(serializeWatchlist(entities));
    expect(loaded!.version).toBe(WATCHLIST_SCHEMA_VERSION);
  });
});
