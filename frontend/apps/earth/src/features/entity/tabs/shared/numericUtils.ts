/**
 * numericUtils — Smart numeric parsing for string values from data sources.
 *
 * All observation metadata is stored as strings in the database. Values like
 * "8,534" (thousands separator), "1,234.56" (US formatted), or "-3,200.5"
 * are common but fail JavaScript's native Number() parser which doesn't
 * understand locale formatting.
 *
 * This module provides a single source of truth for numeric detection and
 * parsing across the entire frontend — ObservationChart (field detection),
 * ObservationTable (cell formatting), and any future consumers.
 */

/**
 * Attempt to parse a string as a number, handling common locale formats:
 * - "8,534"     → 8534       (thousands commas)
 * - "1,234.56"  → 1234.56    (US format)
 * - "-8,534"    → -8534      (negative with commas)
 * - "3.14"      → 3.14       (plain float)
 * - "8534"      → 8534       (plain integer)
 * - ""          → null       (empty)
 * - "hello"     → null       (not numeric)
 * - "N/A"       → null       (not numeric)
 *
 * Returns the parsed number or null if the string is not numeric.
 */
export function parseNumericString(raw: string | undefined | null): number | null {
  if (raw === undefined || raw === null) return null;

  const trimmed = raw.trim();
  if (trimmed === '') return null;

  // Quick check: if the string contains only digits, dots, commas, minus,
  // plus, and optional whitespace, it might be numeric.
  // Reject strings that contain letters (except 'e'/'E' for scientific notation).
  if (/[a-df-zA-DF-Z]/.test(trimmed)) return null;

  // Strip thousands separators (commas followed by 3 digits).
  // This handles: "8,534" → "8534", "1,234,567" → "1234567", "1,234.56" → "1234.56"
  const stripped = trimmed.replace(/,(\d{3})/g, '$1');

  const num = Number(stripped);
  if (isNaN(num) || !isFinite(num)) return null;

  return num;
}

/**
 * Check if a string value represents a number.
 * Equivalent to `parseNumericString(raw) !== null`.
 */
export function isNumericString(raw: string | undefined | null): boolean {
  return parseNumericString(raw) !== null;
}
