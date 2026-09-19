/**
 * SelectionIndicator — renders tactical corner brackets around selected
 * AND pinned watchlist entities on the globe, with real-time coordinate readout.
 *
 * Uses a dedicated BillboardCollection with a canvas-drawn bracket icon.
 * The bracket billboard is repositioned every frame via scene.preUpdate
 * to follow the entity's actual interpolated billboard position (not the
 * raw store position), ensuring brackets track smooth-motion extrapolation.
 *
 * Pinned entities that are NOT also selected render slightly dimmer brackets
 * (alpha 0.6 vs 0.9) to distinguish them visually.
 *
 * Visual style:
 *  ┌─        ─┐
 *  │  ENTITY  │
 *  └─        ─┘
 *  32.1234° N  117.5678° W
 */

import { useEffect, useRef, useMemo, useCallback } from 'react';
import {
  type Viewer,
  type Billboard,
  type Label,
  BillboardCollection,
  Cartesian2,
  Cartesian3,
  Cartographic,
  Color,
  LabelCollection,
  LabelStyle,
  VerticalOrigin,
  HorizontalOrigin,
  Math as CesiumMath,
} from 'cesium';
import { useUIStore } from '@/app/store';
import type { Observation } from '@/app/store';
import { selectIsolatedEntityId } from '@/app/selectors';
import { interpolatedPositions } from './interpolatedPositions';
import { theme } from '@respondent/core';

const BRACKET_SIZE = 64;
const BRACKET_COLOR = theme.palette.primary.main;
const BRACKET_LINE_WIDTH = 2;
const CORNER_LENGTH = 14;

/** Alpha for brackets on selected entities. */
const SELECTED_BRACKET_ALPHA = 0.9;
/** Alpha for brackets on pinned-but-not-selected entities (dimmer). */
const PINNED_BRACKET_ALPHA = 0.6;

/**
 * Draws tactical corner brackets on a canvas.
 * Returns the canvas to be used as a billboard image.
 */
function createBracketCanvas(): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = BRACKET_SIZE;
  canvas.height = BRACKET_SIZE;
  const ctx = canvas.getContext('2d')!;

  ctx.strokeStyle = BRACKET_COLOR;
  ctx.lineWidth = BRACKET_LINE_WIDTH;
  ctx.shadowColor = BRACKET_COLOR;
  ctx.shadowBlur = 4;

  const m = 4; // margin
  const s = BRACKET_SIZE - m;
  const c = CORNER_LENGTH;

  // Top-left corner
  ctx.beginPath();
  ctx.moveTo(m, m + c);
  ctx.lineTo(m, m);
  ctx.lineTo(m + c, m);
  ctx.stroke();

  // Top-right corner
  ctx.beginPath();
  ctx.moveTo(s - c, m);
  ctx.lineTo(s, m);
  ctx.lineTo(s, m + c);
  ctx.stroke();

  // Bottom-right corner
  ctx.beginPath();
  ctx.moveTo(s, s - c);
  ctx.lineTo(s, s);
  ctx.lineTo(s - c, s);
  ctx.stroke();

  // Bottom-left corner
  ctx.beginPath();
  ctx.moveTo(m + c, s);
  ctx.lineTo(m, s);
  ctx.lineTo(m, s - c);
  ctx.stroke();

  return canvas;
}

/**
 * Format a coordinate value as degrees with N/S/E/W suffix.
 */
function formatCoord(degrees: number, isLat: boolean): string {
  const abs = Math.abs(degrees);
  const suffix = isLat ? (degrees >= 0 ? 'N' : 'S') : degrees >= 0 ? 'E' : 'W';
  return `${abs.toFixed(4)}° ${suffix}`;
}

export interface SelectionIndicatorProps {
  viewerRef: React.MutableRefObject<Viewer | null>;
  viewerReady?: boolean;
}

/** Stable empty map to avoid re-renders when no entities are selected. */
const EMPTY_OBS_MAP = new Map<string, Observation>();

export const SelectionIndicator: React.FC<SelectionIndicatorProps> = ({
  viewerRef,
  viewerReady = false,
}) => {
  const selectedEntities = useUIStore((s) => s.selectedEntities);
  const watchlistEntities = useUIStore((s) => s.watchlistEntities);

  // Merge selected + pinned entities (deduplicated by entityId)
  const mergedEntities = useMemo(() => {
    const seen = new Set<string>();
    const result: Array<{ entityId: string; layerId: string; isPinnedOnly: boolean }> = [];

    // Selected entities first (full brightness)
    for (const sel of selectedEntities) {
      if (!seen.has(sel.entityId)) {
        seen.add(sel.entityId);
        result.push({ entityId: sel.entityId, layerId: sel.layerId, isPinnedOnly: false });
      }
    }

    // Watchlist entities (pinned + unpinned) that aren't already selected (dimmer brackets)
    for (const we of watchlistEntities) {
      if (!seen.has(we.entityId)) {
        seen.add(we.entityId);
        result.push({ entityId: we.entityId, layerId: we.layerId, isPinnedOnly: true });
      }
    }

    return result;
  }, [selectedEntities, watchlistEntities]);

  // Track which entities are pinned-only (not selected) for bracket alpha
  const pinnedOnlySet = useMemo(
    () => new Set(mergedEntities.filter((e) => e.isPinnedOnly).map((e) => e.entityId)),
    [mergedEntities],
  );

  // Only subscribe to version changes for layers that have bracket entities
  const selectedLayerIds = useMemo(
    () => [...new Set(mergedEntities.map((s) => s.layerId))],
    [mergedEntities],
  );
  // Build a version key from only the relevant layers — changes in other layers are ignored
  const relevantVersionKey = useUIStore(
    useCallback(
      (s) => {
        if (selectedLayerIds.length === 0) return '';
        return selectedLayerIds.map((id) => s.layerVersions[id] ?? 0).join(',');
      },
      [selectedLayerIds],
    ),
  );
  // Read observation data for all bracket entities — stable empty ref when nothing to show
  const selectedObservations = useMemo(() => {
    if (mergedEntities.length === 0) return EMPTY_OBS_MAP;
    const result = new Map<string, Observation>();
    const layerEntities = useUIStore.getState().layerEntities;
    for (const sel of mergedEntities) {
      for (const [, layerData] of layerEntities) {
        const obs = layerData.obsMap.get(sel.entityId);
        if (obs) {
          result.set(sel.entityId, obs);
          break;
        }
      }
    }
    return result;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mergedEntities, relevantVersionKey]);

  // Derive isolation state from shared selector
  const isolatedEntityId = useUIStore(selectIsolatedEntityId);
  const collectionRef = useRef<BillboardCollection | null>(null);
  const labelCollectionRef = useRef<LabelCollection | null>(null);
  // Keyed refs for incremental bracket/label management — avoids removeAll flicker
  const bracketMapRef = useRef<Map<string, Billboard>>(new Map());
  const labelMapRef = useRef<Map<string, Label>>(new Map());
  const bracketCanvas = useMemo(() => createBracketCanvas(), []);

  // Create/destroy the bracket BillboardCollection + LabelCollection
  // Depends on viewerReady so the effect re-runs after viewer initialization
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const collection = new BillboardCollection();
    viewer.scene.primitives.add(collection);
    collectionRef.current = collection;

    const labels = new LabelCollection();
    viewer.scene.primitives.add(labels);
    labelCollectionRef.current = labels;

    const bracketMap = bracketMapRef.current;
    const labelMap = labelMapRef.current;

    return () => {
      collectionRef.current = null;
      labelCollectionRef.current = null;
      bracketMap.clear();
      labelMap.clear();
      if (!viewer.isDestroyed()) {
        viewer.scene.primitives.remove(collection);
        viewer.scene.primitives.remove(labels);
      }
      if (!collection.isDestroyed()) {
        collection.destroy();
      }
      if (!labels.isDestroyed()) {
        labels.destroy();
      }
    };
  }, [viewerRef, viewerReady]);

  // Incrementally sync bracket billboards + labels when selection or pinned entities change.
  // Uses keyed maps to add/remove only the entities that changed — avoids the
  // removeAll() → re-add cycle that causes a single-frame flicker.
  useEffect(() => {
    const collection = collectionRef.current;
    const labels = labelCollectionRef.current;
    if (!collection || collection.isDestroyed()) return;

    const bbMap = bracketMapRef.current;
    const lbMap = labelMapRef.current;

    const entitiesToShow = isolatedEntityId
      ? mergedEntities.filter((s) => s.entityId === isolatedEntityId)
      : mergedEntities;

    const targetIds = new Set(entitiesToShow.map((s) => s.entityId));

    // Remove brackets/labels for entities no longer in the target set
    for (const [entityId, bb] of bbMap) {
      if (!targetIds.has(entityId)) {
        collection.remove(bb);
        bbMap.delete(entityId);
      }
    }
    if (labels && !labels.isDestroyed()) {
      for (const [entityId, lb] of lbMap) {
        if (!targetIds.has(entityId)) {
          labels.remove(lb);
          lbMap.delete(entityId);
        }
      }
    }

    // Add or update brackets/labels for current target set
    for (const sel of entitiesToShow) {
      const obs = selectedObservations.get(sel.entityId);
      if (!obs) continue;

      const lat = obs.position.lat;
      const lon = obs.position.lon;
      const alt = obs.altitudeM ?? 0;
      const position = Cartesian3.fromDegrees(lon, lat, alt);
      const bracketAlpha = pinnedOnlySet.has(sel.entityId)
        ? PINNED_BRACKET_ALPHA
        : SELECTED_BRACKET_ALPHA;

      const existingBb = bbMap.get(sel.entityId);
      if (existingBb) {
        // Update in place — no remove/add, no flicker
        existingBb.position = position;
        existingBb.color = Color.fromCssColorString(BRACKET_COLOR).withAlpha(bracketAlpha);
      } else {
        const bb = collection.add({
          position,
          image: bracketCanvas,
          color: Color.fromCssColorString(BRACKET_COLOR).withAlpha(bracketAlpha),
          scale: 1.0,
          disableDepthTestDistance: Number.POSITIVE_INFINITY,
          id: sel.entityId,
        });
        bbMap.set(sel.entityId, bb);
      }

      // Labels
      if (labels && !labels.isDestroyed()) {
        const labelAlpha = pinnedOnlySet.has(sel.entityId) ? 0.55 : 0.85;
        const existingLb = lbMap.get(sel.entityId);
        if (existingLb) {
          existingLb.position = position;
          existingLb.fillColor = Color.fromCssColorString(BRACKET_COLOR).withAlpha(labelAlpha);
          existingLb.text = `${formatCoord(lat, true)}  ${formatCoord(lon, false)}`;
        } else {
          const lb = labels.add({
            position,
            text: `${formatCoord(lat, true)}  ${formatCoord(lon, false)}`,
            font: '11px monospace',
            fillColor: Color.fromCssColorString(BRACKET_COLOR).withAlpha(labelAlpha),
            outlineColor: Color.BLACK.withAlpha(0.7),
            outlineWidth: 2,
            style: LabelStyle.FILL_AND_OUTLINE,
            verticalOrigin: VerticalOrigin.TOP,
            horizontalOrigin: HorizontalOrigin.CENTER,
            pixelOffset: new Cartesian2(0, 34),
            disableDepthTestDistance: Number.POSITIVE_INFINITY,
            id: sel.entityId,
          });
          lbMap.set(sel.entityId, lb);
        }
      }
    }
    viewerRef.current?.scene.requestRender();
  }, [
    mergedEntities,
    selectedObservations,
    bracketCanvas,
    isolatedEntityId,
    pinnedOnlySet,
    viewerRef,
  ]);

  // Track all bracket entities' positions every frame using actual billboard positions
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed() || mergedEntities.length === 0) return;

    const onPreUpdate = () => {
      const collection = collectionRef.current;
      if (!collection || collection.isDestroyed() || collection.length === 0) return;

      let moved = false;

      for (let i = 0; i < collection.length; i++) {
        const bracket = collection.get(i);
        if (!bracket || !bracket.id) continue;

        const bracketEntityId = bracket.id as string;

        // O(1) lookup: read interpolated position from shared map (written by BillboardLayerRenderer)
        let pos = interpolatedPositions.get(bracketEntityId);

        // Fallback: use store position for non-flight layers or before first animation frame
        if (!pos) {
          const entities = useUIStore.getState().layerEntities;
          for (const [, layerData] of entities) {
            const obs = layerData.obsMap.get(bracketEntityId);
            if (obs) {
              pos = Cartesian3.fromDegrees(obs.position.lon, obs.position.lat, obs.altitudeM ?? 0);
              break;
            }
          }
        }

        if (!pos) continue;

        bracket.position = pos;
        moved = true;

        // Update coordinate label to match — O(1) via keyed ref map
        const label = labelMapRef.current.get(bracketEntityId);
        if (label) {
          label.position = pos;
          const carto = Cartographic.fromCartesian(pos);
          const latDeg = CesiumMath.toDegrees(carto.latitude);
          const lonDeg = CesiumMath.toDegrees(carto.longitude);
          label.text = `${formatCoord(latDeg, true)}  ${formatCoord(lonDeg, false)}`;
        }
      }

      if (moved) {
        viewerRef.current?.scene.requestRender();
      }
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
      }
    };
  }, [viewerRef, mergedEntities]);

  return null;
};
