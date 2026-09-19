/**
 * Layout Constants Tests
 *
 * Snapshot-style assertions for every value exported from layout.ts.
 * These tests exist to make accidental changes to magic numbers immediately
 * visible during CI — a failing test here is a deliberate signal to review
 * the layout impact of any constant change before merging.
 */

import { describe, it, expect } from 'vitest';
import {
  MOBILE_NAV_HEIGHT,
  MOBILE_NAV_GAP,
  MOBILE_HEADER_HEIGHT,
  MOBILE_STACK_GAP,
  MOBILE_PANEL_MARGIN,
} from '@respondent/core';

describe('Layout constants', () => {
  describe('MOBILE_NAV_HEIGHT', () => {
    it('equals 56px (MobileBottomNav icon 44px + padding)', () => {
      expect(MOBILE_NAV_HEIGHT).toBe(56);
    });

    it('is a positive integer', () => {
      expect(Number.isInteger(MOBILE_NAV_HEIGHT)).toBe(true);
      expect(MOBILE_NAV_HEIGHT).toBeGreaterThan(0);
    });
  });

  describe('MOBILE_NAV_GAP', () => {
    it('equals 16px (gap between bottom nav and watchlist bar)', () => {
      expect(MOBILE_NAV_GAP).toBe(16);
    });

    it('is a positive integer', () => {
      expect(Number.isInteger(MOBILE_NAV_GAP)).toBe(true);
      expect(MOBILE_NAV_GAP).toBeGreaterThan(0);
    });
  });

  describe('MOBILE_HEADER_HEIGHT', () => {
    it('equals 40px (RESPONDENT label + connection status header)', () => {
      expect(MOBILE_HEADER_HEIGHT).toBe(40);
    });

    it('is a positive integer', () => {
      expect(Number.isInteger(MOBILE_HEADER_HEIGHT)).toBe(true);
      expect(MOBILE_HEADER_HEIGHT).toBeGreaterThan(0);
    });
  });

  describe('MOBILE_STACK_GAP', () => {
    it('equals 8px (gap between entity panel and watchlist bar)', () => {
      expect(MOBILE_STACK_GAP).toBe(8);
    });

    it('is a positive integer', () => {
      expect(Number.isInteger(MOBILE_STACK_GAP)).toBe(true);
      expect(MOBILE_STACK_GAP).toBeGreaterThan(0);
    });
  });

  describe('MOBILE_PANEL_MARGIN', () => {
    it('equals 8px (horizontal margin for floating mobile panels)', () => {
      expect(MOBILE_PANEL_MARGIN).toBe(8);
    });

    it('is a positive integer', () => {
      expect(Number.isInteger(MOBILE_PANEL_MARGIN)).toBe(true);
      expect(MOBILE_PANEL_MARGIN).toBeGreaterThan(0);
    });
  });

  describe('Derived layout relationships', () => {
    it('base entity panel offset (MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP) equals 72px', () => {
      expect(MOBILE_NAV_HEIGHT + MOBILE_NAV_GAP).toBe(72);
    });

    it('search bar top padding (MOBILE_HEADER_HEIGHT + 8) equals 48px', () => {
      // Used in EntitySearchBar: `calc(var(--sat, 0px) + ${MOBILE_HEADER_HEIGHT + 8}px)`
      expect(MOBILE_HEADER_HEIGHT + 8).toBe(48);
    });

    it('MOBILE_STACK_GAP is smaller than MOBILE_NAV_GAP (tighter gap between stacked elements)', () => {
      expect(MOBILE_STACK_GAP).toBeLessThan(MOBILE_NAV_GAP);
    });
  });
});
