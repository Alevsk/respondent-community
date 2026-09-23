import type { Page, Route } from '@playwright/test';

/**
 * Deterministic REST stub feed for e2e specs.
 *
 * Registers `page.route` handlers for the read endpoints the earth app fetches
 * on boot and during search/selection, so a spec can assert UI behavior WITHOUT
 * depending on the live feeder. MUST be called BEFORE `page.goto('/')` — routes
 * registered after navigation do not intercept the initial document's requests.
 *
 * Endpoint shapes were confirmed against the running community server
 * (`make build && ./bin/community serve`, port 8091):
 *
 *   GET /v1/layers
 *     → { layers: [{ id, name, type, enabled, mode, density, source,
 *                     lastUpdate, count, color, pointSize, displayConfig,
 *                     historyConfig, renderingMode, filteringMode }] }
 *     (count is a STRING; filteringMode is a proto enum string)
 *
 *   GET /v1/layers/:id/snapshot
 *     → { entities: [{ id, externalId, layerType, name, metadata,
 *                       aiMetadataJson }],
 *         observations: [{ id, entityId, ts, position:{lat,lon,altM},
 *                          altitudeM, velocity, metadata, eventTimeMs,
 *                          eventEndMs }],
 *         totalCount, hasMore }
 *
 *   GET /v1/entities/search?query=&limit=
 *     → { results: [{ entityId, externalId, layerType, name,
 *                      latestObservation?, metadata }],
 *         totalCount }   (totalCount is a NUMBER here)
 *
 *   GET /v1/entities/detail?entity_id=
 *     → { entity: { id, externalId, layerType, name, metadata,
 *                   aiMetadataJson }, latestObservation? }
 *
 * The frontend builds these as same-origin relative URLs (VITE_API_URL=""),
 * so the glob patterns below match regardless of host/port.
 */

/** A stub entity, shaped to satisfy both the snapshot and search responses. */
export interface StubEntity {
  /** Canonical id: `${layerType}:${externalId}` (matches the real server). */
  id: string;
  externalId: string;
  name: string;
  /** The layer this entity belongs to (== layerType in the real API). */
  layerType: string;
  metadata?: Record<string, string>;
  /** Parsed AI enrichment, serialized to aiMetadataJson in responses. */
  aiMetadata?: Record<string, unknown>;
  lat?: number;
  lon?: number;
}

export const DEFAULT_LAYER_ID = 'flights_commercial';

export const DEFAULT_ENTITIES: StubEntity[] = [
  {
    id: 'flights_commercial:E2E001',
    externalId: 'E2E001',
    name: 'E2E Flight Alpha',
    layerType: DEFAULT_LAYER_ID,
    metadata: { callsign: 'E2EAAA', origin: 'TEST' },
    lat: 40.0,
    lon: -74.0,
  },
  {
    id: 'flights_commercial:E2E002',
    externalId: 'E2E002',
    name: 'E2E Flight Bravo',
    layerType: DEFAULT_LAYER_ID,
    metadata: { callsign: 'E2EBBB', origin: 'TEST' },
    lat: 41.0,
    lon: -73.0,
  },
];

export interface MockRouteOptions {
  /** Layer id advertised by GET /v1/layers (defaults to DEFAULT_LAYER_ID). */
  layerId?: string;
  /** Stub entities served by snapshot + search (defaults to DEFAULT_ENTITIES). */
  entities?: StubEntity[];
  /**
   * Filtering mode for the advertised layer. Defaults to UNSPECIFIED (non-spatial)
   * so the client does not gate snapshots on a viewport. Use a viewport value only
   * if a spec specifically exercises spatial behavior.
   */
  filteringMode?: string;
  /**
   * When true, the advertised layer's displayConfig sets icon.interpolation = true,
   * marking the entity type as "moving" (HistoryTab renders the Trajectory view and
   * the Trails/Isolate switches). Defaults to false (stationary) to match how the
   * existing specs see the layer. The entity-history spec opts in.
   */
  movingEntities?: boolean;
  /**
   * Per-entity observation history served by GET /v1/entities/observations.
   * Keyed by entity id. When provided for the requested entity_id, the stub
   * paginates by the `before_ms` cursor (newest-first, like the real server),
   * exposing `load-more-observations` when more pages remain. Observations
   * should be supplied newest-first. When omitted, the endpoint returns an
   * empty history (`{ observations: [], hasMore: false }`).
   */
  observations?: Record<string, StubObservation[]>;
  /** Page size used to paginate the stubbed observation history. Defaults to 2. */
  observationsPageSize?: number;
  /**
   * Declarative media entries advertised in the layer's displayConfig, in the
   * camelCase shape GetLayers returns. Omitted by default so existing specs see
   * a layer with no media at all.
   */
  media?: StubMediaConfig[];
}

/** A `display.media` entry as the layers API serves it. */
export interface StubMediaConfig {
  id: string;
  kind: 'snapshot' | 'audio';
  label: string;
  urlKey: string;
  attributionKey?: string;
  allowedOrigins?: string[];
  playbackAction?: string;
  snapshot?: { refreshIntervalSeconds: number; cacheBustParam?: string };
  audio?: Record<string, never>;
}

/** A stub observation for the GET /v1/entities/observations history endpoint. */
export interface StubObservation {
  /** Epoch-ms timestamp (drives the `before_ms` pagination cursor). */
  ts: number;
  lat: number;
  lon: number;
  altitudeM?: number;
  velocity?: Record<string, number>;
  metadata?: Record<string, string>;
}

/** Map a StubEntity to the REST snapshot/detail entity shape (aiMetadataJson string). */
function toApiEntity(e: StubEntity) {
  return {
    id: e.id,
    externalId: e.externalId,
    layerType: e.layerType,
    name: e.name,
    metadata: e.metadata ?? {},
    aiMetadataJson:
      e.aiMetadata && Object.keys(e.aiMetadata).length > 0 ? JSON.stringify(e.aiMetadata) : '',
  };
}

/** Map a StubEntity to a snapshot observation (best-effort position). */
function toApiObservation(e: StubEntity) {
  const ts = String(Date.now());
  return {
    id: '',
    entityId: e.id,
    ts,
    position: { lat: e.lat ?? 0, lon: e.lon ?? 0, altM: 0 },
    altitudeM: 0,
    velocity: {},
    metadata: e.metadata ?? {},
    eventTimeMs: ts,
    eventEndMs: '0',
  };
}

/** Map a StubEntity to the search-result shape (entityId, latestObservation). */
function toSearchResult(e: StubEntity) {
  const ts = String(Date.now());
  return {
    entityId: e.id,
    externalId: e.externalId,
    layerType: e.layerType,
    name: e.name,
    latestObservation: {
      id: '',
      entityId: e.id,
      ts,
      position: { lat: e.lat ?? 0, lon: e.lon ?? 0, altM: 0 },
      altitudeM: 0,
      velocity: {},
      metadata: e.metadata ?? {},
      eventTimeMs: ts,
      eventEndMs: '0',
    },
    metadata: e.metadata ?? {},
  };
}

/**
 * Register deterministic REST stubs. Call BEFORE `page.goto('/')`.
 *
 * Only the read endpoints the app touches are stubbed; everything else falls
 * through to the live server (harmless — the deterministic specs never assert
 * on it). The mock WS (helpers/mock-ws.ts) is separate and optional.
 */
export async function setupMockRoutes(page: Page, opts?: MockRouteOptions): Promise<void> {
  const layerId = opts?.layerId ?? DEFAULT_LAYER_ID;
  const entities = opts?.entities ?? DEFAULT_ENTITIES;
  const filteringMode = opts?.filteringMode ?? 'FILTERING_MODE_UNSPECIFIED';
  const moving = opts?.movingEntities ?? false;
  const observations = opts?.observations ?? {};
  const observationsPageSize = opts?.observationsPageSize ?? 2;

  // displayConfig mirrors the live GetLayers shape (icon/trail/style/fieldRenderers).
  // icon.interpolation drives HistoryTab's moving-vs-stationary branch.
  const displayConfig = {
    icon: { shape: 'diamond', rotatable: false, interpolation: moving, scale: 1 },
    trail: { color: '#00e5ff', width: 2, opacity: 0.8 },
    style: { color: '#00e5ff', pointSize: 6 },
    fieldRenderers: [],
    ...(opts?.media ? { media: opts.media } : {}),
  };

  // GET /v1/layers — one advertised layer with a count matching the stub feed.
  await page.route('**/v1/layers', (route: Route) =>
    route.fulfill({
      json: {
        layers: [
          {
            id: layerId,
            name: layerId,
            type: layerId,
            enabled: true,
            mode: 'LAYER_MODE_FULL',
            density: 100,
            source: 'e2e-stub',
            lastUpdate: String(Date.now()),
            count: String(entities.length),
            color: '#00e5ff',
            pointSize: 6,
            displayConfig,
            renderingMode: 'RENDERING_MODE_MAP',
            filteringMode,
          },
        ],
      },
    }),
  );

  // GET /v1/layers/:id/snapshot — the stub entities + observations.
  await page.route('**/v1/layers/*/snapshot*', (route: Route) =>
    route.fulfill({
      json: {
        entities: entities.map(toApiEntity),
        observations: entities.map(toApiObservation),
        totalCount: String(entities.length),
        hasMore: false,
      },
    }),
  );

  // GET /v1/entities/search?query=&limit= — case-insensitive prefix/substring
  // match against the stub entities' name / externalId / id, mirroring how the
  // live server returns the entities matching the typed query.
  await page.route('**/v1/entities/search*', (route: Route) => {
    const url = new URL(route.request().url());
    const query = (url.searchParams.get('query') ?? '').trim().toLowerCase();
    const matched = query
      ? entities.filter((e) =>
          [e.name, e.externalId, e.id].some((f) => f.toLowerCase().includes(query)),
        )
      : [];
    route.fulfill({
      json: { results: matched.map(toSearchResult), totalCount: matched.length },
    });
  });

  // GET /v1/entities/detail?entity_id= — return the matching stub entity, or a
  // 404-style gRPC error envelope (matches the real server) when unknown.
  await page.route('**/v1/entities/detail*', (route: Route) => {
    const url = new URL(route.request().url());
    const entityId = url.searchParams.get('entity_id') ?? '';
    const found = entities.find((e) => e.id === entityId);
    if (!found) {
      route.fulfill({ status: 404, json: { code: 5, message: 'entity not found', details: [] } });
      return;
    }
    route.fulfill({
      json: { entity: toApiEntity(found), latestObservation: toApiObservation(found) },
    });
  });

  // GET /v1/entities/observations?entity_id=&limit=&before_ms= — observation
  // history. Paginates the provided (newest-first) observations by the before_ms
  // cursor, mirroring the real server, so `load-more-observations` is reachable
  // deterministically. Unknown/empty histories return an empty page.
  await page.route('**/v1/entities/observations*', (route: Route) => {
    const url = new URL(route.request().url());
    const entityId = url.searchParams.get('entity_id') ?? '';
    const reqLimit = Number(url.searchParams.get('limit') ?? observationsPageSize);
    const beforeMs = url.searchParams.get('before_ms');
    // Cap the page at the stub page size so a small fixture can still force
    // pagination (the client requests limit=100 by default).
    const limit = Math.min(reqLimit, observationsPageSize);

    const all = observations[entityId] ?? [];
    // Newest-first; when a cursor is present, return only strictly-older rows.
    const filtered = beforeMs ? all.filter((o) => o.ts < Number(beforeMs)) : all;
    const page = filtered.slice(0, limit);
    const hasMore = filtered.length > limit;

    route.fulfill({
      json: {
        observations: page.map((o) => ({
          id: '',
          entityId,
          ts: String(o.ts),
          position: { lat: o.lat, lon: o.lon, altM: o.altitudeM ?? 0 },
          altitudeM: o.altitudeM ?? 0,
          velocity: o.velocity ?? {},
          metadata: o.metadata ?? {},
          eventTimeMs: String(o.ts),
          eventEndMs: '0',
        })),
        hasMore,
      },
    });
  });
}

// ---------------------------------------------------------------------------
// Notification (AI insight) stubs — reusable for notifications & filters specs.
// ---------------------------------------------------------------------------

/**
 * A stub AI insight, shaped to satisfy the GET /v1/ai/insights response after
 * `normalizeInsight` (frontend/packages/core/src/notifications/normalizeInsight.ts).
 *
 * The client reads (snake_case, like the live server):
 *   id, insight_type, source_name, operation_name, layer_type, attention
 *   (or result.attention), result (object or JSON string), entity_ids,
 *   entities[{ id, external_id, name, layer_type }], observation_ids, created_at.
 *
 * `result.title` becomes the item title; `result.description`/`summary` becomes
 * the body. `attention` is normalized by stripping `ATTENTION_LEVEL_` and
 * lower-casing — pass a plain level here ("high", "critical", …).
 */
export interface StubInsight {
  /** Unique id → renders as `notification-item-${id}`. */
  id: string;
  insightType?: string;
  attention?: string;
  /** Result object — `title` drives the heading, `description` the body. */
  result?: Record<string, unknown>;
  /** Rich entity refs → render as `notif-entity-chip-${id}` when expanded. */
  entities?: Array<{ id: string; externalId?: string; name?: string; layerType?: string }>;
  entityIds?: string[];
  layerType?: string;
  createdAt?: string;
}

/** Two deterministic insights, both >= medium attention so the default filter shows them. */
export const DEFAULT_INSIGHTS: StubInsight[] = [
  {
    id: 'insight-e2e-001',
    insightType: 'flight_anomaly',
    attention: 'high',
    layerType: DEFAULT_LAYER_ID,
    result: {
      title: 'E2E Flight Anomaly Alpha',
      description: 'Unusual altitude change detected for E2E Flight Alpha.',
      callsign: 'E2EAAA',
    },
    entities: [
      {
        id: 'flights_commercial:E2E001',
        externalId: 'E2E001',
        name: 'E2E Flight Alpha',
        layerType: DEFAULT_LAYER_ID,
      },
    ],
    createdAt: new Date().toISOString(),
  },
  {
    id: 'insight-e2e-002',
    insightType: 'proximity_alert',
    attention: 'critical',
    layerType: DEFAULT_LAYER_ID,
    result: {
      title: 'E2E Proximity Alert Bravo',
      description: 'Two tracked entities converged within the alert radius.',
    },
    entities: [
      {
        id: 'flights_commercial:E2E002',
        externalId: 'E2E002',
        name: 'E2E Flight Bravo',
        layerType: DEFAULT_LAYER_ID,
      },
    ],
    createdAt: new Date().toISOString(),
  },
];

export interface NotificationRouteOptions {
  /** Insights served by GET /v1/ai/insights (defaults to DEFAULT_INSIGHTS). */
  insights?: StubInsight[];
  /** Attention levels offered by the filter discovery endpoint. */
  attentionLevels?: string[];
  /** Insight types offered by the filter discovery endpoint. */
  insightTypes?: Array<{ value: string; displayName?: string; sourceName?: string }>;
}

/** Map a StubInsight to the GET /v1/ai/insights item shape (snake_case wire format). */
export function toApiInsight(i: StubInsight) {
  return {
    id: i.id,
    insight_type: i.insightType ?? 'analysis',
    source_name: 'e2e-stub',
    operation_name: i.insightType ?? 'analysis',
    layer_type: i.layerType ?? '',
    attention: i.attention ?? 'medium',
    result: i.result ?? {},
    entity_ids: i.entityIds ?? [],
    entities: (i.entities ?? []).map((e) => ({
      id: e.id,
      external_id: e.externalId ?? '',
      name: e.name ?? '',
      layer_type: e.layerType ?? '',
    })),
    observation_ids: [],
    created_at: i.createdAt ?? new Date().toISOString(),
  };
}

/**
 * Register deterministic stubs for the notification (AI insight) endpoints,
 * which `setupMockRoutes` does NOT cover. Call BEFORE `page.goto('/')`.
 *
 *   GET /v1/ai/insights?...            → { insights: [...], totalCount }
 *   GET /v1/ai/notifications/filters   → { insightTypes, attentionLevels, layerTypes }
 *
 * The insights endpoint ignores the filter query params and always returns the
 * provided set — the deterministic tier asserts UI behavior, not server-side
 * filtering. Filter-driven refetch behavior (re-query on filter change) is left
 * to specs that explicitly exercise it.
 *
 * Returns the stub insights so a spec can reference their ids without re-deriving.
 */
export async function setupNotificationRoutes(
  page: Page,
  opts?: NotificationRouteOptions,
): Promise<StubInsight[]> {
  const insights = opts?.insights ?? DEFAULT_INSIGHTS;
  const attentionLevels = opts?.attentionLevels ?? ['info', 'low', 'medium', 'high', 'critical'];
  const insightTypes =
    opts?.insightTypes ??
    Array.from(new Set(insights.map((i) => i.insightType ?? 'analysis'))).map((v) => ({
      value: v,
      displayName: v.replace(/_/g, ' '),
      sourceName: 'e2e-stub',
    }));

  // GET /v1/ai/insights — the stub insights + total. Filter params are ignored.
  await page.route('**/v1/ai/insights*', (route: Route) =>
    route.fulfill({
      json: { insights: insights.map(toApiInsight), totalCount: insights.length },
    }),
  );

  // GET /v1/ai/notifications/filters — discovery options so the filter toggle renders.
  await page.route('**/v1/ai/notifications/filters*', (route: Route) =>
    route.fulfill({
      json: {
        insightTypes: insightTypes.map((t) => ({
          value: t.value,
          displayName: t.displayName ?? t.value,
          sourceName: t.sourceName ?? 'e2e-stub',
        })),
        attentionLevels,
        layerTypes: Array.from(new Set(insights.map((i) => i.layerType ?? '').filter(Boolean))),
      },
    }),
  );

  return insights;
}
