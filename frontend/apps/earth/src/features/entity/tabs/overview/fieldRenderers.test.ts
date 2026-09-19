/**
 * fieldRenderers unit tests
 *
 * Covers:
 * - applyFormat() for all format types, transforms, prefix/suffix, and NaN handling
 * - registerDynamicRenderers() + resolveFields() dynamic path
 * - resolveFields() hardcoded fallback path
 */

import { describe, it, expect, beforeEach } from 'vitest';
import {
  applyFormat,
  registerDynamicRenderers,
  resolveFields,
  _resetDynamicRenderers,
} from './fieldRenderers';
import type { FieldFormat } from '@/app/store';

// ---------------------------------------------------------------------------
// applyFormat
// ---------------------------------------------------------------------------
describe('applyFormat', () => {
  describe('float', () => {
    const fmt = (overrides?: Partial<FieldFormat>): FieldFormat => ({
      type: 'float',
      precision: 2,
      ...overrides,
    });

    it('formats a numeric string to fixed precision', () => {
      expect(applyFormat('3.14159', fmt())).toBe('3.14');
    });

    it('uses default precision of 1 when not specified', () => {
      expect(applyFormat('3.14159', fmt({ precision: undefined }))).toBe('3.1');
    });

    it('applies prefix and suffix', () => {
      expect(applyFormat('100.5', fmt({ prefix: '$', suffix: ' USD', precision: 2 }))).toBe(
        '$100.50 USD',
      );
    });

    it('returns raw value when NaN (non-numeric string)', () => {
      expect(applyFormat('not-a-number', fmt())).toBe('not-a-number');
    });

    it('handles zero correctly', () => {
      expect(applyFormat('0', fmt({ precision: 1 }))).toBe('0.0');
    });

    it('handles negative numbers', () => {
      expect(applyFormat('-5.678', fmt({ precision: 2 }))).toBe('-5.68');
    });
  });

  describe('integer', () => {
    const fmt = (overrides?: Partial<FieldFormat>): FieldFormat => ({
      type: 'integer',
      ...overrides,
    });

    it('parses and returns an integer string', () => {
      expect(applyFormat('42', fmt())).toBe('42');
    });

    it('truncates decimal portion', () => {
      expect(applyFormat('9.99', fmt())).toBe('9');
    });

    it('applies prefix and suffix', () => {
      expect(applyFormat('7', fmt({ prefix: 'Ch.', suffix: ' km' }))).toBe('Ch.7 km');
    });

    it('returns raw value when NaN', () => {
      expect(applyFormat('abc', fmt())).toBe('abc');
    });

    it('handles negative integers', () => {
      expect(applyFormat('-12', fmt())).toBe('-12');
    });
  });

  describe('string', () => {
    const fmt = (overrides?: Partial<FieldFormat>): FieldFormat => ({
      type: 'string',
      ...overrides,
    });

    it('returns value as-is with no transform', () => {
      expect(applyFormat('Hello World', fmt())).toBe('Hello World');
    });

    it('applies upper transform', () => {
      expect(applyFormat('hello', fmt({ transform: 'upper' }))).toBe('HELLO');
    });

    it('applies lower transform', () => {
      expect(applyFormat('HELLO', fmt({ transform: 'lower' }))).toBe('hello');
    });

    it('applies prefix and suffix with transform', () => {
      expect(applyFormat('abc', fmt({ transform: 'upper', prefix: '[', suffix: ']' }))).toBe(
        '[ABC]',
      );
    });

    it('handles empty string with transform', () => {
      expect(applyFormat('', fmt({ transform: 'upper' }))).toBe('');
    });
  });

  describe('raw / default', () => {
    it('returns the value unchanged for type raw', () => {
      expect(applyFormat('raw-value', { type: 'raw' })).toBe('raw-value');
    });

    it('returns value unchanged for unknown type', () => {
      // TypeScript prevents this at compile time, but runtime should be safe
      expect(
        applyFormat('x', { type: 'unknown' } as unknown as Parameters<typeof applyFormat>[1]),
      ).toBe('x');
    });
  });
});

// ---------------------------------------------------------------------------
// registerDynamicRenderers + resolveFields (dynamic path)
// ---------------------------------------------------------------------------
describe('registerDynamicRenderers', () => {
  beforeEach(() => {
    _resetDynamicRenderers();
  });

  it('resolves fields using dynamic renderers for a registered layer type', () => {
    registerDynamicRenderers('custom_ships', [
      {
        keys: ['vessel_name'],
        label: 'VESSEL',
        format: { type: 'string', transform: 'upper' },
        priority: 0,
      },
      {
        keys: ['speed_knots'],
        label: 'SPEED',
        format: { type: 'float', precision: 1, suffix: ' kts' },
        priority: 1,
      },
    ]);

    const fields = resolveFields(
      { vessel_name: 'ever given', speed_knots: '12.345' },
      'custom_ships',
    );

    expect(fields).toHaveLength(2);
    expect(fields[0]).toEqual({ label: 'VESSEL', value: 'EVER GIVEN', priority: 0 });
    expect(fields[1]).toEqual({ label: 'SPEED', value: '12.3 kts', priority: 1 });
  });

  it('returns unmatched keys as generic fields after dynamic renderers run', () => {
    registerDynamicRenderers('custom_ships', [
      {
        keys: ['vessel_name'],
        label: 'VESSEL',
        format: { type: 'string' },
        priority: 0,
      },
    ]);

    const fields = resolveFields({ vessel_name: 'Titanic', flag_state: 'GB' }, 'custom_ships');

    const labels = fields.map((f) => f.label);
    expect(labels).toContain('VESSEL');
    expect(labels).toContain('FLAG STATE');
  });

  it('falls back to hardcoded renderers for unregistered layer type', () => {
    // flights_commercial uses hardcoded renderers, not dynamic ones
    const fields = resolveFields(
      { callsign: 'AA123', registration: 'N12345' },
      'flights_commercial',
    );

    const callsignField = fields.find((f) => f.label === 'CALLSIGN');
    expect(callsignField).toBeDefined();
    expect(callsignField!.value).toBe('AA123');
  });

  it('replaces previous registration when called again for same layer type', () => {
    registerDynamicRenderers('my_layer', [
      { keys: ['old_key'], label: 'OLD', format: { type: 'string' }, priority: 0 },
    ]);
    registerDynamicRenderers('my_layer', [
      { keys: ['new_key'], label: 'NEW', format: { type: 'string' }, priority: 0 },
    ]);

    const fields = resolveFields({ old_key: 'x', new_key: 'y' }, 'my_layer');
    const labels = fields.map((f) => f.label);
    expect(labels).toContain('NEW');
    // OLD renderer should not be present (was replaced)
    const oldRendered = fields.find((f) => f.label === 'OLD' && f.value === 'x');
    expect(oldRendered).toBeUndefined();
  });

  it('sorts resolved fields by priority', () => {
    registerDynamicRenderers('priority_test', [
      { keys: ['z_field'], label: 'Z', format: { type: 'string' }, priority: 10 },
      { keys: ['a_field'], label: 'A', format: { type: 'string' }, priority: 0 },
      { keys: ['m_field'], label: 'M', format: { type: 'string' }, priority: 5 },
    ]);

    const fields = resolveFields({ z_field: '1', a_field: '2', m_field: '3' }, 'priority_test');

    expect(fields.map((f) => f.label)).toEqual(['A', 'M', 'Z']);
  });

  it('uses first matching key when renderer lists multiple keys', () => {
    registerDynamicRenderers('multi_key', [
      {
        keys: ['magnitude', 'mag', 'M'],
        label: 'MAGNITUDE',
        format: { type: 'float', precision: 1 },
        priority: 0,
      },
    ]);

    // Only 'mag' is present in metadata
    const fields = resolveFields({ mag: '5.3' }, 'multi_key');
    expect(fields).toHaveLength(1);
    expect(fields[0].value).toBe('5.3');
  });
});

// ---------------------------------------------------------------------------
// resolveFields — hardcoded path (no dynamic registration)
// ---------------------------------------------------------------------------
describe('resolveFields (hardcoded path)', () => {
  beforeEach(() => {
    _resetDynamicRenderers();
  });

  it('returns all metadata as generic fields for completely unknown layer type', () => {
    const fields = resolveFields({ foo: 'bar', baz: 'qux' }, 'unknown_layer');
    const labels = fields.map((f) => f.label);
    expect(labels).toContain('FOO');
    expect(labels).toContain('BAZ');
  });

  it('formats earthquake magnitude via hardcoded renderer', () => {
    const fields = resolveFields({ magnitude: '5.7' }, 'earthquakes');
    const magField = fields.find((f) => f.label === 'MAGNITUDE');
    expect(magField?.value).toBe('M5.7');
  });

  it('all fields sorted by priority ascending', () => {
    const fields = resolveFields({ norad_id: '12345', inclination: '51.6' }, 'satellites');
    expect(fields[0].priority).toBeLessThanOrEqual(fields[fields.length - 1].priority);
  });
});
