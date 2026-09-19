import { describe, it, expect } from 'vitest';

describe('core models', () => {
  it('exports Entity and Observation types', async () => {
    const models = await import('./entity');
    expect(models).toBeDefined();
    expect(models.DEFAULT_ENTITY_VIEW_STATE).toBeDefined();
    expect(typeof models.entityKey).toBe('function');
    expect(typeof models.obsKey).toBe('function');
  });

  it('exports Layer and LayerConfig types', async () => {
    const models = await import('./layer');
    expect(models).toBeDefined();
  });

  it('exports MAX_PINNED_ENTITIES constant', async () => {
    const { MAX_PINNED_ENTITIES } = await import('./watchlist');
    expect(MAX_PINNED_ENTITIES).toBe(20);
  });

  it('exports FilterPreset values', async () => {
    const { FILTER_PRESETS } = await import('./ui');
    expect(FILTER_PRESETS).toContain('NORMAL');
    expect(FILTER_PRESETS).toContain('CRT');
    expect(FILTER_PRESETS).toContain('NVG');
    expect(FILTER_PRESETS).toContain('FLIR');
  });

  it('exports TimePreset values', async () => {
    const { TIME_PRESETS } = await import('./time');
    expect(TIME_PRESETS).toContain('1h');
    expect(TIME_PRESETS).toContain('8h');
    expect(TIME_PRESETS).toContain('24h');
    expect(TIME_PRESETS).toContain('custom');
  });
});
