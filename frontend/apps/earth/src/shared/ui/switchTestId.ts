import type React from 'react';

/**
 * Places a test hook on a MUI Switch's inner input.
 *
 * MUI spreads unknown props onto the switch's wrapper span, but `disabled`,
 * `checked` and the checkbox role all live on the input it wraps. A testid on
 * the span therefore addresses an element that reports neither state: a test
 * asking whether a switch is disabled gets `false` for a disabled switch, and
 * one asking whether it is checked has to reach through to `.locator('input')`
 * itself. Putting the hook on the input makes the testid mean what it says.
 *
 * The return type is an intersection so the value is describable without a
 * cast: `data-*` attributes are valid DOM attributes that React's
 * InputHTMLAttributes — whose members are all optional — does not declare.
 */
export const switchTestId = (
  id: string,
): React.InputHTMLAttributes<HTMLInputElement> & { 'data-testid': string } => ({
  'data-testid': id,
});
