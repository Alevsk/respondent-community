import { describe, expect, it } from 'vitest';
import { resolveMedia, snapshotUrl, validateMediaUrl } from './mediaUrls';
import type { MediaConfig } from '@respondent/core';

describe('media URL policy', () => {
  it.each([
    'http://media.example/a',
    '//media.example/a',
    '/image',
    'javascript:alert(1)',
    'data:image/png;base64,AA',
    'https://user:secret@media.example/a',
    'https://localhost/a',
    'https://printer/a',
    'https://a.local/a',
    'https://a.internal/a',
    'https://a.localhost/a',
    'https://127.0.0.1/a',
    'https://0x7f000001/a',
    'https://10.1.2.3/a',
    'https://172.31.1.2/a',
    'https://192.168.1.1/a',
    'https://169.254.169.254/a',
    'https://100.64.1.1/a',
    'https://0.0.0.0/a',
    'https://224.0.0.1/a',
    'https://[::1]/a',
    'https://[::]/a',
    'https://[fc00::1]/a',
    'https://[fe80::1]/a',
    'https://[::ffff:127.0.0.1]/a',
    'https://[::ffff:c0a8:101]/a',
  ])('rejects unsafe URL %s', (url) => expect(validateMediaUrl(url)).toBeNull());

  it('accepts public HTTPS and exact allowed origins, independent of extension', () => {
    expect(validateMediaUrl('https://media.example/live?id=42')).toBe(
      'https://media.example/live?id=42',
    );
    expect(validateMediaUrl('https://8.8.8.8/feed')).toBe('https://8.8.8.8/feed');
    expect(validateMediaUrl('https://media.example/a', ['https://media.example'])).toBeTruthy();
    expect(
      validateMediaUrl('https://media.example.evil.test/a', ['https://media.example']),
    ).toBeNull();
    expect(validateMediaUrl('https://media.example:8443/a', ['https://media.example'])).toBeNull();
    expect(validateMediaUrl('https://media.example/a', [])).toBeNull();
  });

  it('adds only an explicitly configured cache parameter and preserves queries', () => {
    expect(snapshotUrl('https://media.example/a?token=a%2Bb', undefined, 123)).toBe(
      'https://media.example/a?token=a%2Bb',
    );
    const url = new URL(snapshotUrl('https://media.example/a?token=a%2Bb&tick=old', 'tick', 123));
    expect(url.searchParams.get('token')).toBe('a+b');
    expect(url.searchParams.getAll('tick')).toEqual(['123']);
  });

  it('resolves only declared media, with observation metadata taking precedence', () => {
    const config: MediaConfig[] = [
      {
        id: 'view',
        kind: 'snapshot',
        label: 'Camera',
        urlKey: 'frame',
        attributionKey: 'credit',
        snapshot: { refreshIntervalSeconds: 30 },
      },
    ];
    const media = resolveMedia(
      config,
      { frame: 'https://old.example/a', credit: 'Owner' },
      { frame: 'https://new.example/b' },
    );
    expect(media[0]).toMatchObject({
      url: 'https://new.example/b',
      attribution: 'Owner',
      config: config[0],
    });
    expect(resolveMedia(undefined, { frame: 'https://old.example/a' })).toEqual([]);
    expect(
      resolveMedia(config, { frame: 'https://old.example/a' }, { frame: '' })[0].url,
    ).toBeNull();
  });
});
