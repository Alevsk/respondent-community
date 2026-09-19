/**
 * interpolatedPositions module tests
 *
 * Verifies the shared mutable Map<string, Cartesian3> exported as a constant.
 * This module is a frame-synchronous data channel — no reactivity, no state.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import type { Cartesian3 } from 'cesium';

// ---------------------------------------------------------------------------
// Import subject under test
// ---------------------------------------------------------------------------

import { interpolatedPositions } from './interpolatedPositions';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Build a minimal Cartesian3-shaped object for position assertions.
 * The module types the map values as Cartesian3 but accepts any conforming
 * object — we avoid a full Cesium import in unit tests for simplicity.
 */
function makeCartesian3(x: number, y: number, z: number): Cartesian3 {
  return { x, y, z } as unknown as Cartesian3;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('interpolatedPositions', () => {
  beforeEach(() => {
    // Reset to a known empty state before each test.
    // The module is a singleton so mutations persist across imports.
    interpolatedPositions.clear();
  });

  it('is initially empty after clear', () => {
    expect(interpolatedPositions.size).toBe(0);
  });

  it('is a Map instance', () => {
    expect(interpolatedPositions).toBeInstanceOf(Map);
  });

  it('stores a position that can be retrieved by entity ID', () => {
    const pos = makeCartesian3(10, 20, 500);
    interpolatedPositions.set('entity-1', pos);

    expect(interpolatedPositions.get('entity-1')).toBe(pos);
  });

  it('returns undefined for an unknown entity ID', () => {
    expect(interpolatedPositions.get('nonexistent')).toBeUndefined();
  });

  it('overwrites a previously stored position when set again', () => {
    const first = makeCartesian3(1, 2, 3);
    const second = makeCartesian3(4, 5, 6);

    interpolatedPositions.set('entity-1', first);
    interpolatedPositions.set('entity-1', second);

    expect(interpolatedPositions.get('entity-1')).toBe(second);
    expect(interpolatedPositions.size).toBe(1);
  });

  it('removes an entry when deleted by entity ID', () => {
    interpolatedPositions.set('entity-1', makeCartesian3(1, 2, 3));
    interpolatedPositions.delete('entity-1');

    expect(interpolatedPositions.has('entity-1')).toBe(false);
    expect(interpolatedPositions.size).toBe(0);
  });

  it('delete on a missing key is a no-op and returns false', () => {
    const result = interpolatedPositions.delete('does-not-exist');
    expect(result).toBe(false);
    expect(interpolatedPositions.size).toBe(0);
  });

  it('tracks multiple entities simultaneously', () => {
    const posA = makeCartesian3(10, 20, 100);
    const posB = makeCartesian3(30, 40, 200);
    const posC = makeCartesian3(50, 60, 300);

    interpolatedPositions.set('entity-a', posA);
    interpolatedPositions.set('entity-b', posB);
    interpolatedPositions.set('entity-c', posC);

    expect(interpolatedPositions.size).toBe(3);
    expect(interpolatedPositions.get('entity-a')).toBe(posA);
    expect(interpolatedPositions.get('entity-b')).toBe(posB);
    expect(interpolatedPositions.get('entity-c')).toBe(posC);
  });

  it('removing one entity does not affect others', () => {
    const posA = makeCartesian3(1, 2, 3);
    const posB = makeCartesian3(4, 5, 6);

    interpolatedPositions.set('entity-a', posA);
    interpolatedPositions.set('entity-b', posB);
    interpolatedPositions.delete('entity-a');

    expect(interpolatedPositions.has('entity-a')).toBe(false);
    expect(interpolatedPositions.get('entity-b')).toBe(posB);
  });

  it('is a singleton — the same Map reference is returned on every import', async () => {
    // Re-import the module; ES module caching guarantees the same instance.
    const { interpolatedPositions: imported } = await import('./interpolatedPositions');
    expect(imported).toBe(interpolatedPositions);
  });

  it('mutations made through one reference are visible through the same reference', () => {
    const pos = makeCartesian3(7, 8, 9);
    interpolatedPositions.set('shared-entity', pos);

    // Because it is the same Map object, the size and value are immediately
    // visible without any re-import or notification mechanism.
    expect(interpolatedPositions.size).toBe(1);
    expect(interpolatedPositions.get('shared-entity')).toBe(pos);
  });
});
