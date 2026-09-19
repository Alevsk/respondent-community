import type { Page } from '@playwright/test';
import type { StubEntity } from './routes';

/**
 * Mock the app's WebSocket so deterministic specs can push layer frames without
 * a live feeder. Uses Playwright's `routeWebSocket` (available in 1.61).
 *
 * The app's render path does NOT gate on the WS reaching a "LIVE" state — the
 * earth-shell, globe, panels, and search all render purely from REST + store
 * state. Billboards (the only WS-data-driven surface) are exercised separately
 * by the live-data-gated specs (progressive-loading). So for the deterministic
 * tier the mock simply stands in for the real socket; pushing frames is optional.
 *
 * WS envelope confirmed against internal/realtime/websocket.go (handleSubscribe
 * / page reply, line ~1538):
 *   { "type": "layer.snapshot", "layer_id": "<id>",
 *     "data": { "entities": [...], "observations": [...], "total": <n> } }
 *
 * The client (useLayerStream.handleMessage, case 'layer.snapshot') reads
 * message.type, message.layer_id, message.data.entities/observations. Entities
 * use the same snake/camel-tolerant shape as the REST snapshot.
 *
 * By default the mock does NOT connect upstream — it fully replaces the socket,
 * so no real feeder data leaks in (determinism). Pass `{ connectUpstream: true }`
 * to additionally connect to the real server (e.g. when a spec wants live
 * billboards plus the ability to inject extra frames).
 */
export interface MockWebSocketOptions {
  /** If true, also connect to the real server WS (default false — fully stubbed). */
  connectUpstream?: boolean;
}

/** Handle returned by setupMockWebSocket for pushing frames from a spec. */
export interface MockWebSocketHandle {
  /**
   * Push a `layer.snapshot` frame to every connected client socket.
   * @param layerId  the layer the snapshot belongs to.
   * @param entities stub entities to materialize into the frame.
   */
  pushSnapshot: (layerId: string, entities: StubEntity[]) => Promise<void>;
}

function toFrameEntity(e: StubEntity) {
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

function toFrameObservation(e: StubEntity) {
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

export async function setupMockWebSocket(
  page: Page,
  opts?: MockWebSocketOptions,
): Promise<MockWebSocketHandle> {
  const connectUpstream = opts?.connectUpstream ?? false;

  // Track the live route handles so the spec can push frames to the browser.
  // routeWebSocket's handler runs in Node; we capture each WebSocketRoute.
  const sockets: Array<{ send: (data: string) => void }> = [];

  await page.routeWebSocket('**/ws', (ws) => {
    sockets.push(ws);
    if (connectUpstream) {
      // Bridge to the real server so live frames still flow; we can also inject.
      const server = ws.connectToServer();
      ws.onMessage((msg) => server.send(msg));
      server.onMessage((msg) => ws.send(msg));
    } else {
      // Fully stubbed: swallow client → server frames (subscribe, viewport, ping).
      // The app does not require any reply to render the deterministic surfaces.
      ws.onMessage(() => {
        /* ignore */
      });
    }
  });

  return {
    pushSnapshot: async (layerId: string, entities: StubEntity[]) => {
      const frame = JSON.stringify({
        type: 'layer.snapshot',
        layer_id: layerId,
        data: {
          entities: entities.map(toFrameEntity),
          observations: entities.map(toFrameObservation),
          total: entities.length,
        },
      });
      for (const s of sockets) s.send(frame);
    },
  };
}
