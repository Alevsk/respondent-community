/**
 * time-range.spec.ts — Time-range presets + custom range picker
 *
 * Covers:
 *  1. time-range-trigger opens the preset menu
 *  2. time-preset-1h / 8h / 24h → store.timePreset + store.timeMode === 'range' + data-active
 *  3. time-preset-live → store.timeMode === 'live'
 *  4. time-preset-custom → opens the CustomRangePicker (keeps menu open)
 *  5. CustomRangePicker: tab switching, month navigation, day/hour/minute/period picks,
 *     crp-apply → store.timeFrom/timeTo non-null ISO + store.timePreset === 'custom'
 *
 * Implementation notes confirmed from source:
 *  - RecBlock: clicking live/1h/8h/24h closes the Popover; custom does NOT close it.
 *  - setTimePreset sets timeMode='range', timeTo=null (sliding window).
 *  - setCustomTimeRange sets timeMode='range', timeTo=<ISO>, timePreset='custom'.
 *  - CustomRangePicker defaults: fromVal = 1h ago, toVal = now; both valid on init.
 *  - HOURS = [12,1..11]; MINUTES = [0,15,30,45]; testid crp-hour-{h}/crp-minute-{m}.
 *  - Apply validation: from must be < to, span ≤ 24h, from ≥ now-48h.
 *  - The default init values already pass validation (from=1h ago, to=now).
 */

import { test, expect } from '@playwright/test';
import { setupMockRoutes } from '../helpers/routes';

/** Read the time-related fields from window.__store(). */
async function getTimeState(page: import('@playwright/test').Page) {
  return page.evaluate(() => {
    const s = window.__store();
    return {
      timeMode: s.timeMode,
      timePreset: s.timePreset,
      timeFrom: s.timeFrom,
      timeTo: s.timeTo,
    };
  });
}

test.describe('Time Range — presets and custom picker', () => {
  test.beforeEach(async ({ page }) => {
    await setupMockRoutes(page);
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  // ── 1. trigger opens the menu ────────────────────────────────────────────────

  test('time-range-trigger opens the preset menu', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-live')).toBeVisible();
    await expect(page.getByTestId('time-preset-1h')).toBeVisible();
    await expect(page.getByTestId('time-preset-8h')).toBeVisible();
    await expect(page.getByTestId('time-preset-24h')).toBeVisible();
    await expect(page.getByTestId('time-preset-custom')).toBeVisible();
  });

  // ── 2a. preset-1h ────────────────────────────────────────────────────────────

  test('time-preset-1h sets timePreset=1h and timeMode=range in store', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-1h')).toBeVisible();
    await page.getByTestId('time-preset-1h').click();

    // Menu closes after a preset selection
    await expect(page.getByTestId('time-preset-1h')).not.toBeVisible();

    const state = await getTimeState(page);
    expect(state.timeMode).toBe('range');
    expect(state.timePreset).toBe('1h');

    // Re-open to verify data-active on the clicked preset
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-1h')).toHaveAttribute('data-active', 'true');
  });

  // ── 2b. preset-8h ────────────────────────────────────────────────────────────

  test('time-preset-8h sets timePreset=8h and timeMode=range in store', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-8h')).toBeVisible();
    await page.getByTestId('time-preset-8h').click();

    const state = await getTimeState(page);
    expect(state.timeMode).toBe('range');
    expect(state.timePreset).toBe('8h');

    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-8h')).toHaveAttribute('data-active', 'true');
  });

  // ── 2c. preset-24h ───────────────────────────────────────────────────────────

  test('time-preset-24h sets timePreset=24h and timeMode=range in store', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-24h')).toBeVisible();
    await page.getByTestId('time-preset-24h').click();

    const state = await getTimeState(page);
    expect(state.timeMode).toBe('range');
    expect(state.timePreset).toBe('24h');

    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-24h')).toHaveAttribute('data-active', 'true');
  });

  // ── 3. preset-live returns to live mode ──────────────────────────────────────

  test('time-preset-live returns timeMode to live', async ({ page }) => {
    // First activate a range preset
    await page.getByTestId('time-range-trigger').click();
    await page.getByTestId('time-preset-1h').click();
    const rangeState = await getTimeState(page);
    expect(rangeState.timeMode).toBe('range');

    // Now switch back to live
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-live')).toBeVisible();
    await page.getByTestId('time-preset-live').click();

    const liveState = await getTimeState(page);
    expect(liveState.timeMode).toBe('live');
    expect(liveState.timeFrom).toBeNull();
    expect(liveState.timeTo).toBeNull();
  });

  // ── 4. preset-custom opens the CustomRangePicker (menu stays open) ───────────

  test('time-preset-custom opens CustomRangePicker without closing the menu', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await expect(page.getByTestId('time-preset-custom')).toBeVisible();
    await page.getByTestId('time-preset-custom').click();

    // Menu stays open; the picker tabs should appear
    await expect(page.getByTestId('crp-tab-from')).toBeVisible();
    await expect(page.getByTestId('crp-tab-to')).toBeVisible();
    await expect(page.getByTestId('crp-apply')).toBeVisible();
  });

  // ── 5a. CustomRangePicker: tab switching ─────────────────────────────────────

  test('CustomRangePicker: crp-tab-from and crp-tab-to switch the active tab', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await page.getByTestId('time-preset-custom').click();
    await expect(page.getByTestId('crp-tab-from')).toBeVisible();

    // FROM tab is active by default
    await expect(page.getByTestId('crp-tab-from')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('crp-tab-to')).toHaveAttribute('data-active', 'false');

    // Switch to TO tab
    await page.getByTestId('crp-tab-to').click();
    await expect(page.getByTestId('crp-tab-to')).toHaveAttribute('data-active', 'true');
    await expect(page.getByTestId('crp-tab-from')).toHaveAttribute('data-active', 'false');

    // Switch back to FROM
    await page.getByTestId('crp-tab-from').click();
    await expect(page.getByTestId('crp-tab-from')).toHaveAttribute('data-active', 'true');
  });

  // ── 5b. CustomRangePicker: month navigation ───────────────────────────────────

  test('CustomRangePicker: crp-prev-month and crp-next-month navigate months', async ({ page }) => {
    await page.getByTestId('time-range-trigger').click();
    await page.getByTestId('time-preset-custom').click();
    await expect(page.getByTestId('crp-prev-month')).toBeVisible();
    await expect(page.getByTestId('crp-next-month')).toBeVisible();

    // Navigate back one month — crp-day-1 is always present in any month
    await page.getByTestId('crp-prev-month').click();
    await expect(page.getByTestId('crp-day-1')).toBeVisible();

    // Navigate forward one month (back to original month)
    await page.getByTestId('crp-next-month').click();
    await expect(page.getByTestId('crp-day-1')).toBeVisible();
  });

  // ── 5c. CustomRangePicker: full custom-range apply flow ──────────────────────
  //
  // Strategy: mutate ONLY the TO minute, then assert the committed timeTo ISO
  // reflects the exact minute we clicked. This is a VALUE-TIED assertion.
  //
  // Why this is deterministic across any run-date and any lookback config:
  //
  //  • We never touch the calendar (no day change). FROM stays at its default
  //    (1h ago, today-UTC), which is ALWAYS within any lookback window ≥ 1h.
  //    APPLY only checks `from >= now - lookback`; keeping FROM at default means
  //    it passes regardless of maxLookbackHours.
  //
  //  • We keep the TO date at today (the default). isDisabled() in MiniCalendar
  //    gates day cells; minute cells have NO isDisabled logic and are always
  //    clickable.
  //
  //  • APPLY does NOT require `to <= now`. It only requires `from < to`,
  //    `span <= maxSpanMs`, and `from >= now - lookback`. Our FROM (1h ago) and
  //    TO (today, same UTC hour as default but with a chosen minute) always
  //    satisfy these constraints as long as the chosen minute keeps TO > FROM.
  //    We pick a TO hour (12 PM noon) that is always > FROM (1h ago at ~current
  //    time), guaranteeing span ≤ 24h and from < to regardless of time of day.
  //
  //  • The target minute we click MUST differ from the current TO default minute
  //    (initValue(0) snaps to Math.floor(UTCMinutes/15)*15, one of [0,15,30,45]).
  //    We compute the default minute at runtime and choose a different MINUTES
  //    cell. The assertion `new Date(timeTo).getUTCMinutes() === targetMinute`
  //    can ONLY pass if the minute picker actually committed that value — it is
  //    impossible to satisfy accidentally because the default had a different
  //    minute. This is the mutation proof.
  //
  //  • toUTCDate() builds Date.UTC(year, month, day, h24, minute, 0, 0), so
  //    getUTCMinutes() on the ISO string directly mirrors the clicked minute cell.

  test('CustomRangePicker: full from/to/apply flow sets custom range in store', async ({
    page,
  }) => {
    // Step 1: compute the TO default minute before opening the picker.
    // initValue(0) uses UTC: minute = Math.floor(getUTCMinutes() / 15) * 15.
    // We find a MINUTES cell ([0, 15, 30, 45]) that differs from this default.
    const { defaultToMinute, targetMinute } = await page.evaluate(() => {
      const MINUTES = [0, 15, 30, 45];
      const now = new Date();
      const defaultMin = Math.floor(now.getUTCMinutes() / 15) * 15;
      // Pick the first MINUTES value that is not the default.
      const target = MINUTES.find((m) => m !== defaultMin) ?? MINUTES[1];
      return { defaultToMinute: defaultMin, targetMinute: target };
    });

    await page.getByTestId('time-range-trigger').click();
    await page.getByTestId('time-preset-custom').click();

    // FROM tab is active by default — leave it untouched (1h ago, today-UTC).
    await expect(page.getByTestId('crp-tab-from')).toHaveAttribute('data-active', 'true');

    // Switch to TO tab.
    await page.getByTestId('crp-tab-to').click();
    await expect(page.getByTestId('crp-tab-to')).toHaveAttribute('data-active', 'true');

    // Do NOT change day, hour, or period on the TO tab. The default TO is "now"
    // (initValue(0)), which is always ~1h after the default FROM (initValue(-3600_000)).
    // Changing only the minute cannot flip from > to: FROM is ≥1h behind TO in
    // hour terms (same day, 1h earlier), so a ±45-minute minute adjustment on
    // TO cannot cross FROM. span stays well within 24h.

    // Click the target minute — a value that DIFFERS from the default TO minute.
    // This is the mutation: clicking a different cell changes toVal.minute.
    await page.getByTestId(`crp-minute-${targetMinute}`).click();
    // Confirm the target cell is now marked active.
    await expect(page.getByTestId(`crp-minute-${targetMinute}`)).toHaveAttribute(
      'data-active',
      'true',
    );
    // Confirm the old default minute cell is NOT active (it differs from targetMinute).
    await expect(page.getByTestId(`crp-minute-${defaultToMinute}`)).toHaveAttribute(
      'data-active',
      'false',
    );

    // Apply the range.
    await page.getByTestId('crp-apply').click();

    // Menu should close after apply.
    await expect(page.getByTestId('crp-apply')).not.toBeVisible();

    // Store must reflect the custom range.
    const state = await getTimeState(page);
    expect(state.timePreset).toBe('custom');
    expect(state.timeMode).toBe('range');
    expect(state.timeFrom).not.toBeNull();
    expect(state.timeTo).not.toBeNull();

    // Both must be valid ISO 8601 strings (round-trips through Date).
    expect(new Date(state.timeFrom!).toISOString()).toBe(state.timeFrom);
    expect(new Date(state.timeTo!).toISOString()).toBe(state.timeTo);

    // FROM must be before TO.
    expect(new Date(state.timeFrom!).getTime()).toBeLessThan(new Date(state.timeTo!).getTime());

    // VALUE-TIED ASSERTION: timeTo's UTC minute must equal the minute cell we
    // clicked. toUTCDate() stores minute directly into Date.UTC(..., minute, ...),
    // so getUTCMinutes() on the ISO string directly reflects the picker selection.
    // This assertion CANNOT pass accidentally: the default TO had a different
    // minute (defaultToMinute !== targetMinute), so only a real picker mutation
    // produces the correct value.
    expect(new Date(state.timeTo!).getUTCMinutes()).toBe(targetMinute);
  });
});
