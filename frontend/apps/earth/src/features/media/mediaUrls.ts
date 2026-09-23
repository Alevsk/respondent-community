import type { MediaConfig } from '@respondent/core';

/** Backslashes, whitespace and control characters never belong in a media URL. */
function hasUnsafeCharacter(value: string): boolean {
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    if (code <= 0x20 || code === 0x7f || value[i] === '\\') return true;
  }
  return false;
}

/** Browser-side literal-host screening; browsers cannot DNS-pin or inspect redirects. */
export function validateMediaUrl(value: string, origins?: string[]): string | null {
  if (!/^https:\/\//i.test(value) || hasUnsafeCharacter(value)) return null;
  try {
    const url = new URL(value);
    if (url.protocol !== 'https:' || url.username || url.password) return null;
    const host = url.hostname.toLowerCase().replace(/\.$/, '');
    if (host.startsWith('[')) {
      // Only global unicast IPv6. Also excludes IPv4-mapped, local, multicast,
      // unspecified and loopback literals without ambiguous text-prefix checks.
      const first = parseInt(host.slice(1).split(':')[0], 16);
      if (!(first >= 0x2000 && first <= 0x3fff)) return null;
    } else if (/^\d+\.\d+\.\d+\.\d+$/.test(host)) {
      const [a, b] = host.split('.').map(Number);
      if (
        a === 0 ||
        a === 10 ||
        a === 127 ||
        a >= 224 ||
        (a === 100 && b >= 64 && b <= 127) ||
        (a === 169 && b === 254) ||
        (a === 172 && b >= 16 && b <= 31) ||
        (a === 192 && (b === 168 || b === 0)) ||
        (a === 198 && (b === 18 || b === 19))
      )
        return null;
    } else if (
      !host.includes('.') ||
      /(^|\.)(localhost|local|internal|lan|home|test\.invalid)$/.test(host) ||
      host.endsWith('.home.arpa')
    ) {
      return null;
    }
    // An empty list means "no fixed origins declared", not "deny everything":
    // protojson emits an unset repeated field as [], so the browser cannot tell
    // the two apart and a source that legitimately omits allowed_origins (a
    // community radio directory, say) must still play.
    if (origins?.length && !origins.includes(url.origin)) return null;
    return url.href;
  } catch {
    return null;
  }
}

/**
 * Adds a cache-busting parameter without disturbing the rest of the URL.
 *
 * The query is rewritten segment by segment so every parameter we are not
 * replacing keeps its original bytes. Re-serializing through URLSearchParams
 * would normalize the whole query — %20 becomes +, a bare flag gains an = —
 * which silently invalidates a signed URL while looking harmless.
 */
export function snapshotUrl(value: string, param?: string, now = Date.now()): string {
  if (!param) return value;

  const hashAt = value.indexOf('#');
  const fragment = hashAt === -1 ? '' : value.slice(hashAt);
  const addressed = hashAt === -1 ? value : value.slice(0, hashAt);
  const queryAt = addressed.indexOf('?');
  const path = queryAt === -1 ? addressed : addressed.slice(0, queryAt);
  const query = queryAt === -1 ? '' : addressed.slice(queryAt + 1);

  const kept = query.split('&').filter((segment) => segment !== '' && queryKey(segment) !== param);
  kept.push(`${encodeURIComponent(param)}=${now}`);
  return `${path}?${kept.join('&')}${fragment}`;
}

/** The decoded name of one `k=v` query segment. */
function queryKey(segment: string): string {
  const raw = segment.split('=', 1)[0];
  try {
    return decodeURIComponent(raw.replace(/\+/g, ' '));
  } catch {
    return raw;
  }
}

export interface ResolvedMedia {
  config: MediaConfig;
  url: string | null;
  attribution: string;
}

export function resolveMedia(
  configs: MediaConfig[] | undefined,
  entity: Record<string, string>,
  observation?: Record<string, string>,
): ResolvedMedia[] {
  const metadata = { ...entity, ...observation };
  return (configs ?? []).map((config) => ({
    config,
    url: validateMediaUrl(metadata[config.urlKey] ?? '', config.allowedOrigins),
    attribution: config.attributionKey ? (metadata[config.attributionKey] ?? '') : '',
  }));
}
