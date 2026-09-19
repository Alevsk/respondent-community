/**
 * numericUtils — Unit tests for smart numeric parsing.
 *
 * Covers:
 * - Plain integers and floats
 * - Thousands separators (commas)
 * - Mixed format: commas + decimal point
 * - Negative numbers with separators
 * - Scientific notation
 * - Non-numeric strings (text, symbols, mixed)
 * - Edge cases: empty, whitespace, null, undefined
 * - isNumericString convenience wrapper
 */

import { describe, it, expect } from 'vitest';
import { parseNumericString, isNumericString } from './numericUtils';

describe('numericUtils', () => {
  describe('parseNumericString', () => {
    // ── Plain numbers ───────────────────────────────────────────────
    it('parses plain integer', () => {
      expect(parseNumericString('42')).toBe(42);
    });

    it('parses plain float', () => {
      expect(parseNumericString('3.14')).toBe(3.14);
    });

    it('parses zero', () => {
      expect(parseNumericString('0')).toBe(0);
    });

    it('parses negative integer', () => {
      expect(parseNumericString('-100')).toBe(-100);
    });

    it('parses negative float', () => {
      expect(parseNumericString('-3.14')).toBe(-3.14);
    });

    it('parses positive with explicit plus sign', () => {
      expect(parseNumericString('+42')).toBe(42);
    });

    // ── Thousands separators (commas) ───────────────────────────────
    it('parses integer with thousands comma: "8,534"', () => {
      expect(parseNumericString('8,534')).toBe(8534);
    });

    it('parses large integer with multiple commas: "1,234,567"', () => {
      expect(parseNumericString('1,234,567')).toBe(1234567);
    });

    it('parses millions: "10,000,000"', () => {
      expect(parseNumericString('10,000,000')).toBe(10000000);
    });

    it('parses US format with commas and decimal: "1,234.56"', () => {
      expect(parseNumericString('1,234.56')).toBe(1234.56);
    });

    it('parses large float: "2,461,162.3199"', () => {
      expect(parseNumericString('2,461,162.3199')).toBe(2461162.3199);
    });

    it('parses negative with thousands comma: "-8,534"', () => {
      expect(parseNumericString('-8,534')).toBe(-8534);
    });

    it('parses negative with commas and decimal: "-1,234.56"', () => {
      expect(parseNumericString('-1,234.56')).toBe(-1234.56);
    });

    // ── Scientific notation ─────────────────────────────────────────
    it('parses scientific notation: "1.5e3"', () => {
      expect(parseNumericString('1.5e3')).toBe(1500);
    });

    it('parses scientific notation uppercase: "2.5E-4"', () => {
      expect(parseNumericString('2.5E-4')).toBe(0.00025);
    });

    // ── Non-numeric strings ─────────────────────────────────────────
    it('returns null for plain text: "hello"', () => {
      expect(parseNumericString('hello')).toBeNull();
    });

    it('returns null for "N/A"', () => {
      expect(parseNumericString('N/A')).toBeNull();
    });

    it('returns null for "daylight"', () => {
      expect(parseNumericString('daylight')).toBeNull();
    });

    it('returns null for "Good"', () => {
      expect(parseNumericString('Good')).toBeNull();
    });

    it('returns null for mixed text and numbers: "PM2.5"', () => {
      expect(parseNumericString('PM2.5')).toBeNull();
    });

    it('returns null for units: "100km"', () => {
      expect(parseNumericString('100km')).toBeNull();
    });

    it('returns null for percentage: "85%"', () => {
      expect(parseNumericString('85%')).toBeNull();
    });

    it('returns null for URL-like: "http://example.com"', () => {
      expect(parseNumericString('http://example.com')).toBeNull();
    });

    it('returns null for boolean-like: "true"', () => {
      expect(parseNumericString('true')).toBeNull();
    });

    it('returns null for date-like: "2026-05-01"', () => {
      expect(parseNumericString('2026-05-01')).toBeNull();
    });

    // ── Edge cases ──────────────────────────────────────────────────
    it('returns null for empty string', () => {
      expect(parseNumericString('')).toBeNull();
    });

    it('returns null for whitespace-only string', () => {
      expect(parseNumericString('   ')).toBeNull();
    });

    it('returns null for null', () => {
      expect(parseNumericString(null)).toBeNull();
    });

    it('returns null for undefined', () => {
      expect(parseNumericString(undefined)).toBeNull();
    });

    it('trims whitespace around number', () => {
      expect(parseNumericString('  42  ')).toBe(42);
    });

    it('trims whitespace around formatted number', () => {
      expect(parseNumericString(' 8,534 ')).toBe(8534);
    });

    it('returns null for Infinity', () => {
      expect(parseNumericString('Infinity')).toBeNull();
    });

    it('returns null for NaN string', () => {
      expect(parseNumericString('NaN')).toBeNull();
    });

    // ── Real-world values from observation data ─────────────────────
    it('parses ISS altitude: "437,188"', () => {
      expect(parseNumericString('437,188')).toBe(437188);
    });

    it('parses ISS daynum: "2,461,162"', () => {
      expect(parseNumericString('2,461,162')).toBe(2461162);
    });

    it('parses flight altitude: "8,534"', () => {
      expect(parseNumericString('8,534')).toBe(8534);
    });

    it('parses ocean buoy pressure: "1020.4"', () => {
      expect(parseNumericString('1020.4')).toBe(1020.4);
    });

    it('parses earthquake magnitude: "2.9"', () => {
      expect(parseNumericString('2.9')).toBe(2.9);
    });

    it('parses radiation value: "0.0867"', () => {
      expect(parseNumericString('0.0867')).toBe(0.0867);
    });

    it('rejects air quality label: "Good"', () => {
      expect(parseNumericString('Good')).toBeNull();
    });

    it('rejects visibility: "daylight"', () => {
      expect(parseNumericString('daylight')).toBeNull();
    });

    it('rejects visibility: "eclipsed"', () => {
      expect(parseNumericString('eclipsed')).toBeNull();
    });
  });

  describe('isNumericString', () => {
    it('returns true for plain number', () => {
      expect(isNumericString('42')).toBe(true);
    });

    it('returns true for comma-formatted number', () => {
      expect(isNumericString('8,534')).toBe(true);
    });

    it('returns false for text', () => {
      expect(isNumericString('hello')).toBe(false);
    });

    it('returns false for null', () => {
      expect(isNumericString(null)).toBe(false);
    });

    it('returns false for empty string', () => {
      expect(isNumericString('')).toBe(false);
    });
  });
});
