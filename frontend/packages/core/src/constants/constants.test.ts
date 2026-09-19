import { describe, it, expect } from 'vitest';

describe('core constants', () => {
  it('exports layout constants', async () => {
    const layout = await import('./layout');
    expect(layout.MOBILE_NAV_HEIGHT).toBe(56);
    expect(layout.MOBILE_NAV_GAP).toBe(16);
    expect(layout.MOBILE_HEADER_HEIGHT).toBe(40);
  });

  it('exports ASPECT_RATIOS', async () => {
    const { ASPECT_RATIOS } = await import('./aspectRatios');
    expect(ASPECT_RATIOS['9:16'].width).toBe(9);
    expect(ASPECT_RATIOS['16:9'].width).toBe(16);
  });
});
