/**
 * clusterRendererUtils.ts — pure helpers for ClusterLayerRenderer, isolated from
 * Cesium so they can be unit-tested without a viewer.
 */

import type { Entity, Observation } from '@respondent/core';
import { normalizeObservation } from './billboardRendering';
import type { ClusterInput } from './clustering';

/**
 * Extracts clusterable points (id + current lat/lon) from a layer's working set.
 * Mirrors BillboardLayerRenderer's iteration: skip entities without a valid
 * observation, honor `maxEntities`. Reuses `normalizeObservation` so coordinate
 * parsing stays identical to the billboard path.
 */
export function collectClusterInputs(
  entityMap: Map<string, Entity>,
  obsMap: Map<string, Observation>,
  maxEntities: number,
): ClusterInput[] {
  const out: ClusterInput[] = [];
  for (const [entityId] of entityMap) {
    if (out.length >= maxEntities) break;
    const rawObs = obsMap.get(entityId);
    if (!rawObs) continue;
    const obs = normalizeObservation(rawObs);
    if (!obs) continue;
    out.push({ id: entityId, lat: obs.lat, lon: obs.lon });
  }
  return out;
}

/** Fallback marker color when a layer declares none. */
export const DEFAULT_CLUSTER_COLOR = '#33ccff';

/**
 * Renders a cluster's member count into a solid disc canvas tinted with the
 * layer's configured color, so a cluster reads as "many of this entity type".
 * The real colors are baked into the canvas and the billboard is tinted WHITE
 * (Cesium multiplies `billboard.color × texture`) — the same convention as the
 * billboard icon canvases. The count is white with a dark halo so it stays
 * legible on any layer color. Returns an undrawn canvas when a 2D context is
 * unavailable (e.g. jsdom).
 */
export function createClusterCountCanvas(count: number, color: string): HTMLCanvasElement {
  const size = 48;
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) return canvas;

  const label = count > 999 ? '999+' : String(count);
  const r = size / 2;
  const fill = color || DEFAULT_CLUSTER_COLOR;

  // Solid colored disc (slightly translucent so dense overlaps still read).
  ctx.beginPath();
  ctx.arc(r, r, r - 4, 0, Math.PI * 2);
  ctx.globalAlpha = 0.9;
  ctx.fillStyle = fill;
  ctx.fill();
  ctx.globalAlpha = 1;

  // White ring for definition against the globe.
  ctx.lineWidth = 2.5;
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.9)';
  ctx.stroke();

  // Count — white glyph with a dark halo for contrast on light layer colors.
  ctx.font = 'bold 18px monospace';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.lineWidth = 3;
  ctx.strokeStyle = 'rgba(0, 0, 0, 0.55)';
  ctx.strokeText(label, r, r + 1);
  ctx.fillStyle = '#ffffff';
  ctx.fillText(label, r, r + 1);

  return canvas;
}

/**
 * Formats an ISO observation timestamp as a compact relative age (e.g. "12s ago",
 * "4m ago"). Returns '' for missing or unparseable input. `nowMs` is injected for
 * deterministic testing.
 */
export function formatRelativeAge(iso: string | undefined, nowMs: number): string {
  if (!iso) return '';
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return '';
  const diffSec = Math.max(0, Math.floor((nowMs - t) / 1000));
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return `${Math.floor(diffHr / 24)}d ago`;
}
