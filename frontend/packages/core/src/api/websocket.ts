// WebSocket Client for Real-time Entity Updates

import { useSyncExternalStore } from 'react';
import ReconnectingWebSocket from 'partysocket/ws';
import { WS_URL } from './client';

export type WSMessageType =
  | 'subscribe'
  | 'unsubscribe'
  | 'viewport_update'
  | 'time_range'
  | 'layer.update'
  | 'layer.batch_update'
  | 'layer.snapshot'
  | 'layer_update'
  | 'snapshot'
  | 'indicator.update'
  | 'cctv.frame'
  | 'ai_insight'
  | 'notification_filter'
  | 'page'
  | 'ping'
  | 'pong';

export interface ViewportBBox {
  west: number;
  south: number;
  east: number;
  north: number;
}

export interface WSMessage<T = unknown> {
  type: WSMessageType;
  layer_id?: string;
  data: T;
}

export interface EntityUpdate {
  type: 'create' | 'update' | 'delete';
  entity: {
    id: string;
    external_id: string;
    layer_type: string;
    name: string;
    metadata: Record<string, string>;
  };
  observation?: {
    id: string;
    entity_id: string;
    ts: number;
    position: { lat: number; lon: number; alt: number };
    altitude_m: number;
    velocity?: Record<string, number>;
    metadata?: Record<string, string>;
  };
}

export interface LayerSnapshot {
  entities: EntityUpdate['entity'][];
  // `EntityUpdate['observation']` is optional on EntityUpdate, which makes it
  // `T | undefined`. A snapshot's observation list never contains undefined
  // entries — strip the `undefined` so consumers don't have to re-narrow.
  observations: NonNullable<EntityUpdate['observation']>[];
}

export interface CCTVFrame {
  camera_feed_id: string;
  frame: string; // base64 encoded
}

type MessageHandler = (message: WSMessage) => void;

export type WSConnectionStatus = 'connected' | 'disconnected' | 'reconnecting' | 'failed';

/** Per-layer subscription state preserved across reconnections. */
interface LayerSubscriptionState {
  viewport?: ViewportBBox;
  /** If set, this layer was subscribed via time_range (range or live mode). */
  timeRange?: { from: string; to: string };
  /** If true, this layer uses global mode (dashboard analytics — no viewport filtering). */
  global?: boolean;
}

/** Retries partysocket attempts (with backoff) before entering 'failed'. */
const MAX_RETRIES = 5;
/** App-level heartbeat ping cadence (ms). */
const HEARTBEAT_INTERVAL_MS = 30_000;
/** Watchdog window (ms): missing a pong this long after a ping = dead socket. */
const HEARTBEAT_TIMEOUT_MS = 10_000;
/**
 * Cooldown (ms) before auto-recovering from 'failed'. Matches the worst-case
 * backoff so a sustained outage retries forever without a page reload.
 */
const FAILED_RECOVERY_COOLDOWN_MS = 30_000;
/**
 * Max messages held while the WS is connecting. Beyond this, the oldest
 * entry is evicted (FIFO) to bound memory. Normal mount bursts are well
 * under 20 entries; 256 guards against a stuck-in-CONNECTING runaway.
 */
const PENDING_SEND_CAP = 256;

export class WebSocketClient {
  private ws: ReconnectingWebSocket | null = null;
  private handlers: Map<WSMessageType, Set<MessageHandler>> = new Map();
  private subscribedLayers: Map<string, LayerSubscriptionState> = new Map();
  private statusListeners: Set<() => void> = new Set();
  private _status: WSConnectionStatus = 'disconnected';
  /** Set to true when disconnect() is called deliberately; prevents onclose from setting reconnecting. */
  private intentionalDisconnect = false;
  private pendingNotificationFilter: { min_attention: string; insight_types: string[] } | null =
    null;
  /** Heartbeat ping interval timer. */
  private heartbeatIntervalId: ReturnType<typeof setInterval> | null = null;
  /** Pong watchdog timer; fires if no pong arrives within the timeout. */
  private heartbeatTimeoutId: ReturnType<typeof setTimeout> | null = null;
  /** Pending auto-recovery from 'failed'; cancelled on disconnect/reconnect. */
  private failedRecoveryId: ReturnType<typeof setTimeout> | null = null;
  /**
   * FIFO buffer of messages enqueued while the socket is not yet OPEN.
   * Flushed after the subscribedLayers replay in onopen.
   *
   * subscribe / unsubscribe / time_range intentionally bypass this buffer —
   * they are reconstructed canonically from subscribedLayers on open and
   * must not be double-sent.
   */
  private pendingSends: WSMessage[] = [];

  get status(): WSConnectionStatus {
    return this._status;
  }

  private setStatus(status: WSConnectionStatus): void {
    if (this._status !== status) {
      this._status = status;
      this.statusListeners.forEach((listener) => listener());
    }
  }

  subscribeStatus(listener: () => void): () => void {
    this.statusListeners.add(listener);
    return () => this.statusListeners.delete(listener);
  }

  connect(): void {
    // Guard: don't create a new connection if one is already open or connecting.
    // This prevents duplicate connections from React StrictMode double-mount.
    if (
      this.ws &&
      (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }
    // Clear intentional disconnect flag so onclose will track reconnecting.
    this.intentionalDisconnect = false;

    try {
      // partysocket owns the reconnect loop (exponential backoff + jitter +
      // visibility-aware retry). startClosed=false connects immediately;
      // maxEnqueuedMessages=0 because send() gates on readyState itself.
      this.ws = new ReconnectingWebSocket(WS_URL, undefined, {
        maxRetries: MAX_RETRIES,
        connectionTimeout: 5000,
        startClosed: false,
        maxEnqueuedMessages: 0,
      });

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.setStatus('connected');
        // A live socket invalidates any pending 'failed' recovery.
        this.clearFailedRecovery();
        this.startHeartbeat();
        // Re-subscribe to all previously subscribed layers, replaying the
        // correct message type so range-mode layers get time_range (not subscribe).
        for (const [layerId, state] of this.subscribedLayers) {
          if (state.global && state.timeRange) {
            this.send({
              type: 'time_range',
              layer_id: layerId,
              data: { from: state.timeRange.from, to: state.timeRange.to, mode: 'global' },
            });
          } else if (state.global) {
            this.send({
              type: 'subscribe',
              layer_id: layerId,
              data: { mode: 'global' },
            });
          } else if (state.timeRange) {
            this.send({
              type: 'time_range',
              layer_id: layerId,
              data: {
                from: state.timeRange.from,
                to: state.timeRange.to,
                viewport: state.viewport,
              },
            });
          } else {
            this.send({
              type: 'subscribe',
              layer_id: layerId,
              data: state.viewport ? { viewport: state.viewport } : {},
            });
          }
        }
        // Re-send notification filter on reconnect
        if (this.pendingNotificationFilter) {
          this.send({
            type: 'notification_filter',
            data: this.pendingNotificationFilter,
          });
        }
        // Drain non-subscribe messages (viewport_update, page, …) that callers
        // issued before the socket opened. Flushed after the replay above so
        // subscribe/time_range always precede view-state updates.
        this.flushPendingSends();
      };

      this.ws.onmessage = (event: MessageEvent) => {
        try {
          const message: WSMessage = JSON.parse(event.data);
          // pong is internal transport — clear the watchdog and do NOT
          // dispatch it to layer/handler subscribers.
          if (message.type === 'pong') {
            this.clearHeartbeatTimeout();
            return;
          }
          this.handleMessage(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.stopHeartbeat();
        if (this.intentionalDisconnect) {
          this.setStatus('disconnected');
          return;
        }
        // partysocket reconnects automatically until retries are exhausted.
        // Once retryCount hits the configured max it stops creating sockets;
        // surface that as the recoverable 'failed' state, otherwise keep
        // reporting 'reconnecting' while it retries with backoff.
        if (this.ws && this.ws.retryCount >= MAX_RETRIES) {
          this.setStatus('failed');
          this.scheduleFailedRecovery();
        } else {
          this.setStatus('reconnecting');
        }
      };

      this.ws.onerror = (error: Event) => {
        console.error('WebSocket error:', error);
      };
    } catch (error) {
      console.error('Failed to connect to WebSocket:', error);
      this.setStatus('disconnected');
    }
  }

  disconnect(): void {
    this.intentionalDisconnect = true;
    this.stopHeartbeat();
    // Cancel any pending 'failed' auto-recovery so a deliberate close is final.
    this.clearFailedRecovery();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.subscribedLayers.clear();
    this.pendingSends = [];
    this.setStatus('disconnected');
  }

  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  /** Reset from the 'failed' state and force a fresh reconnection attempt. */
  resetAndReconnect(): void {
    if (this.ws) {
      this.ws.reconnect();
      this.setStatus('reconnecting');
    } else {
      // ws is null (e.g. called before first connect or after a full teardown) —
      // fall back to a clean connect so recovery always does something.
      this.connect();
    }
  }

  subscribe(type: WSMessageType, handler: MessageHandler): () => void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set());
    }
    this.handlers.get(type)!.add(handler);

    // Return unsubscribe function
    return () => {
      this.handlers.get(type)?.delete(handler);
    };
  }

  send(message: WSMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
      return;
    }
    // subscribe / unsubscribe / time_range / notification_filter are replay-owned
    // (re-sent canonically in onopen) — buffering them would cause double-sends.
    if (
      message.type === 'subscribe' ||
      message.type === 'unsubscribe' ||
      message.type === 'time_range' ||
      message.type === 'notification_filter'
    ) {
      return;
    }
    if (this.pendingSends.length >= PENDING_SEND_CAP) {
      this.pendingSends.shift(); // drop oldest
    }
    this.pendingSends.push(message);
  }

  /** Drain buffered pre-OPEN messages in FIFO order. Called from onopen after replay. */
  private flushPendingSends(): void {
    if (this.pendingSends.length === 0 || this.ws?.readyState !== WebSocket.OPEN) {
      return;
    }
    const drained = this.pendingSends;
    this.pendingSends = [];
    for (const m of drained) {
      this.ws.send(JSON.stringify(m));
    }
  }

  subscribeLayer(layerId: string, viewport?: ViewportBBox): void {
    // Explicitly clear timeRange — a plain subscribe replaces any prior
    // time_range subscription so reconnection replays the correct message type.
    this.subscribedLayers.set(layerId, { viewport });
    this.send({
      type: 'subscribe',
      layer_id: layerId,
      data: viewport ? { viewport } : {},
    });
  }

  unsubscribeLayer(layerId: string): void {
    this.subscribedLayers.delete(layerId);
    this.send({
      type: 'unsubscribe',
      layer_id: layerId,
      data: {},
    });
  }

  sendViewportUpdate(layerId: string, bbox: ViewportBBox): void {
    // Update stored viewport so reconnection replays the latest camera position,
    // not the stale viewport from the initial subscription.
    const existing = this.subscribedLayers.get(layerId);
    if (existing) {
      existing.viewport = bbox;
    }
    this.send({
      type: 'viewport_update',
      layer_id: layerId,
      data: bbox,
    });
  }

  sendTimeRange(layerId: string, from: string, to: string, viewport?: ViewportBBox): void {
    // Track the subscription with time range state so reconnection replays
    // the correct message type (time_range instead of subscribe).
    this.subscribedLayers.set(layerId, { viewport, timeRange: { from, to } });
    this.send({
      type: 'time_range',
      layer_id: layerId,
      data: { from, to, viewport },
    });
  }

  /**
   * Subscribe in global mode (dashboard analytics).
   * Skips viewport filtering, returns paginated results with total + cursor.
   */
  subscribeLayerGlobal(layerId: string, limit?: number): void {
    this.subscribedLayers.set(layerId, { global: true });
    this.send({
      type: 'subscribe',
      layer_id: layerId,
      data: { mode: 'global', ...(limit ? { limit } : {}) },
    });
  }

  /** Request the next page of entities for a global-mode subscription. */
  requestPage(layerId: string, cursor: string, limit?: number): void {
    this.send({
      type: 'page',
      layer_id: layerId,
      data: { cursor, ...(limit ? { limit } : {}) },
    });
  }

  /** Time range query in global mode (no viewport filtering). */
  sendTimeRangeGlobal(layerId: string, from: string, to: string): void {
    this.subscribedLayers.set(layerId, { global: true, timeRange: { from, to } });
    this.send({
      type: 'time_range',
      layer_id: layerId,
      data: { from, to, mode: 'global' },
    });
  }

  /** Sends notification filter preferences to the server. */
  sendNotificationFilter(filter: { min_attention: string; insight_types: string[] }): void {
    this.pendingNotificationFilter = filter;
    this.send({
      type: 'notification_filter',
      data: filter,
    });
  }

  private handleMessage(message: WSMessage): void {
    const handlers = this.handlers.get(message.type);
    if (handlers) {
      handlers.forEach((handler) => handler(message));
    }
  }

  // --- Heartbeat: detect half-open sockets (sleep/wake, NAT idle) ---

  /** Start the ping interval (idempotent — clears any prior heartbeat first). */
  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatIntervalId = setInterval(() => this.sendPing(), HEARTBEAT_INTERVAL_MS);
  }

  /** Stop the ping interval and clear the pong watchdog. */
  private stopHeartbeat(): void {
    if (this.heartbeatIntervalId !== null) {
      clearInterval(this.heartbeatIntervalId);
      this.heartbeatIntervalId = null;
    }
    this.clearHeartbeatTimeout();
  }

  /** Send a ping and arm the pong watchdog; a missed pong forces a reconnect. */
  private sendPing(): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      return;
    }
    // Defensively clear any prior watchdog before arming a new one — enforces
    // the single-heartbeatTimeoutId invariant and future-proofs against races.
    this.clearHeartbeatTimeout();
    this.ws.send(JSON.stringify({ type: 'ping' }));
    this.heartbeatTimeoutId = setTimeout(() => {
      console.warn('Heartbeat timeout — no pong, forcing reconnect');
      // Correctness relies on partysocket's reconnect() resetting retryCount,
      // so the synthetic onclose surfaces 'reconnecting', not 'failed'.
      this.ws?.reconnect();
    }, HEARTBEAT_TIMEOUT_MS);
  }

  /** Clear the pong watchdog (called on inbound pong or teardown). */
  private clearHeartbeatTimeout(): void {
    if (this.heartbeatTimeoutId !== null) {
      clearTimeout(this.heartbeatTimeoutId);
      this.heartbeatTimeoutId = null;
    }
  }

  // --- Failed-state auto-recovery: never stuck-until-reload ---

  /** Schedule a delayed resetAndReconnect() so 'failed' self-heals after a cooldown. */
  private scheduleFailedRecovery(): void {
    this.clearFailedRecovery();
    this.failedRecoveryId = setTimeout(() => {
      this.failedRecoveryId = null;
      this.resetAndReconnect();
    }, FAILED_RECOVERY_COOLDOWN_MS);
  }

  /** Cancel a pending 'failed' recovery (on deliberate disconnect or reconnect). */
  private clearFailedRecovery(): void {
    if (this.failedRecoveryId !== null) {
      clearTimeout(this.failedRecoveryId);
      this.failedRecoveryId = null;
    }
  }
}

// Singleton instance
export const wsClient = new WebSocketClient();

// React hook for reactive WebSocket connection status
export function useWebSocketStatus(): WSConnectionStatus {
  return useSyncExternalStore(
    (cb) => wsClient.subscribeStatus(cb),
    () => wsClient.status,
  );
}
