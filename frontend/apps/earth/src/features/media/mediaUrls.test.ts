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
    // An omitted `allowed_origins` and an explicit empty list are the same
    // thing by the time they cross the wire: protojson emits an unset repeated
    // field as []. Treating [] as "nothing is allowed" would silently disable
    // every station of a source that deliberately declares no fixed origins.
    expect(validateMediaUrl('https://media.example/a', [])).toBe('https://media.example/a');
  });

  it('adds only an explicitly configured cache parameter and preserves queries', () => {
    expect(snapshotUrl('https://media.example/a?token=a%2Bb', undefined, 123)).toBe(
      'https://media.example/a?token=a%2Bb',
    );
    const url = new URL(snapshotUrl('https://media.example/a?token=a%2Bb&tick=old', 'tick', 123));
    expect(url.searchParams.get('token')).toBe('a+b');
    expect(url.searchParams.getAll('tick')).toEqual(['123']);
  });

  // The loader rejects these spellings when a source declares them as an
  // allowed origin; the browser must refuse them in a catalog value too. It
  // gets there differently — the URL parser normalizes a numeric host to its
  // dotted form first — so the agreement is asserted rather than assumed.
  it.each([
    'https://127.1/frame.jpg',
    'https://0177.0.0.1/frame.jpg',
    'https://2130706433/frame.jpg',
    'https://0x7f000001/frame.jpg',
    'https://3232235777/frame.jpg',
  ])('rejects shorthand address literal %s', (url) => {
    expect(validateMediaUrl(url)).toBeNull();
  });

  it('leaves every other query parameter byte-identical', () => {
    // A signed URL's signature covers the exact query bytes. Re-serializing the
    // query would turn %20 into +, give a bare flag an =, and reorder nothing
    // visibly — all of which break a signature while looking harmless.
    expect(snapshotUrl('https://media.example/a?q=a%20b&flag&sig=AbC%2F123', 'tick', 9)).toBe(
      'https://media.example/a?q=a%20b&flag&sig=AbC%2F123&tick=9',
    );
  });

  it('keeps the fragment after the appended parameter', () => {
    expect(snapshotUrl('https://media.example/a?x=1#frag', 'tick', 9)).toBe(
      'https://media.example/a?x=1&tick=9#frag',
    );
  });

  it('adds the parameter to a URL that has no query', () => {
    expect(snapshotUrl('https://media.example/a', 'tick', 9)).toBe(
      'https://media.example/a?tick=9',
    );
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
