import React, { useEffect, useRef } from 'react';
import {
  type Viewer,
  type Billboard,
  BillboardCollection,
  Cartesian3,
  Color,
  Ellipsoid,
  Math as CesiumMath,
} from 'cesium';
// EllipsoidalOccluder exists at runtime but is missing from Cesium's bundled .d.ts
import * as Cesium from 'cesium';
const EllipsoidalOccluder = (Cesium as Record<string, unknown>).EllipsoidalOccluder as new (
  ellipsoid: typeof Ellipsoid.WGS84,
  cameraPosition: Cartesian3,
) => {
  cameraPosition: Cartesian3;
  isPointVisible(position: Cartesian3): boolean;
};
import { useUIStore, type DataSource } from '@/app/store';
import { selectIsolatedEntityId } from '@/app/selectors';
import { useLayerEntities } from '../../shared/api/useLayerStream';
import { getIconCanvas, supportsRotation, getIconScale } from './icons/iconRegistry';
import {
  extractHeadingRadians,
  projectPosition,
  isBackwardMotion,
  KNOTS_TO_MPS,
  MAX_EXTRAPOLATE_SEC,
} from './entityUtils';
import { interpolatedPositions } from './interpolatedPositions';
import {
  type AnimationState,
  INTERP_DURATION_MS,
  CATCHUP_PAUSE_MS,
  LIVE_IDLE_ALPHA,
  LIVE_PULSE_ALPHA,
  POS_EPSILON,
  getBillboardEntityId,
  getSourceAlpha,
  resolveColorByHex,
  desaturateColor,
  smoothstep,
  normalizeObservation,
  supportsInterpolation,
} from './billboardRendering';

export interface BillboardLayerRendererProps {
  layerId: string;
  layerType: string;
  viewerRef: React.MutableRefObject<Viewer | null>;
  color?: string;
  pointSize?: number;
}

// Re-export resolveColorByHex so the test file can continue to import it from
// this module without modification.
export { resolveColorByHex } from './billboardRendering';

export const BillboardLayerRenderer: React.FC<BillboardLayerRendererProps> = ({
  layerId,
  layerType,
  viewerRef,
  color,
  pointSize,
}) => {
  const { entityMap, obsMap, dirtyEntityIds, hasData, version, maxEntities } =
    useLayerEntities(layerId);
  const showOccluded = useUIStore((s) => s.showOccluded);
  const smoothMotion = useUIStore((s) => s.smoothMotion);
  const smoothMotionRef = useRef(smoothMotion);
  smoothMotionRef.current = smoothMotion;

  // Derive isolation state from shared selector
  const isolatedEntityId = useUIStore(selectIsolatedEntityId);

  // Set of this layer's entities absorbed into cluster markers (published by
  // ClusterLayerRenderer). Reactive copy drives Effect 4 (show); the ref keeps
  // the per-frame interpolation loop current without re-subscribing.
  const clusteredMembers = useUIStore((s) => s.clusteredMemberIds[layerId]);
  const clusteredMembersRef = useRef<Set<string> | undefined>(clusteredMembers);
  clusteredMembersRef.current = clusteredMembers;

  const collectionRef = useRef<BillboardCollection | null>(null);
  const baseColorRef = useRef<Color>(Color.WHITE);
  const occlusionDirtyRef = useRef(true);

  // Cache pinned entity IDs + findMode for O(1) lookups in the occlusion hot path.
  // Uses subscribe pattern (same as FindModeFilter) to stay current without
  // per-frame getState() calls.
  const pinnedSetRef = useRef<Set<string>>(new Set());
  const findModeRef = useRef(false);

  useEffect(() => {
    const sync = () => {
      const { watchlistEntities, findMode } = useUIStore.getState();
      findModeRef.current = findMode;
      pinnedSetRef.current = new Set(
        watchlistEntities.filter((e) => e.pinned).map((e) => e.entityId),
      );
    };
    sync();
    const unsub = useUIStore.subscribe((state, prev) => {
      if (state.watchlistEntities !== prev.watchlistEntities || state.findMode !== prev.findMode) {
        sync();
      }
    });
    return unsub;
  }, []);

  // Keyed billboard tracking for animated flight layers
  const billboardMapRef = useRef<Map<string, Billboard>>(new Map());
  const animStateRef = useRef<Map<string, AnimationState>>(new Map());
  const entitySourceRef = useRef<Map<string, DataSource>>(new Map());
  // Per-entity idle alpha — used by the occlusion system to restore correct
  // brightness when an entity rotates back into view on the visible side.
  const entityIdleAlphaRef = useRef<Map<string, number>>(new Map());
  const scratchCartesian = useRef(new Cartesian3());

  // Track whether the next Effect 2 run needs a full rebuild
  const needsFullRebuildRef = useRef(true);

  const isFlightLayer = supportsInterpolation(layerType);

  // Alpha ref — keeps Cesium listeners current without re-subscribing
  const occludedAlphaRef = useRef(showOccluded ? 0.12 : 0.0);
  occludedAlphaRef.current = showOccluded ? 0.12 : 0.0;
  const VISIBLE_ALPHA = 1.0;

  // Effect 1: Create/destroy BillboardCollection — stable lifecycle tied to viewer
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const collection = new BillboardCollection();
    viewer.scene.primitives.add(collection);
    collectionRef.current = collection;

    const billboardMap = billboardMapRef.current;
    const animState = animStateRef.current;
    const entitySource = entitySourceRef.current;
    const entityIdleAlpha = entityIdleAlphaRef.current;

    return () => {
      collectionRef.current = null;
      billboardMap.clear();
      animState.clear();
      entitySource.clear();
      entityIdleAlpha.clear();
      needsFullRebuildRef.current = true;
      if (!viewer.isDestroyed()) {
        viewer.scene.primitives.remove(collection);
      }
      if (!collection.isDestroyed()) {
        collection.destroy();
      }
    };
  }, [viewerRef]);

  // Effect 2: Sync billboards with store data.
  // Flight layers use O(delta) incremental updates via dirtyEntityIds;
  // non-flight layers use full rebuild.
  useEffect(() => {
    const collection = collectionRef.current;
    if (!collection || collection.isDestroyed()) return;

    if (!hasData || !entityMap || !obsMap || !dirtyEntityIds) {
      // Data was cleared — remove all existing billboards and flag for a full
      // rebuild on next data arrival. Without this, stale billboards remain
      // visible on the globe until new data triggers a rebuild.
      needsFullRebuildRef.current = true;
      const bbMap = billboardMapRef.current;
      for (const [, bb] of bbMap) {
        collection.remove(bb);
      }
      bbMap.clear();
      animStateRef.current.clear();
      entitySourceRef.current.clear();
      entityIdleAlphaRef.current.clear();
      return;
    }

    const currentLayerData = useUIStore.getState().layerEntities.get(layerId);
    if (currentLayerData?.needsRebuild) {
      needsFullRebuildRef.current = true;
      currentLayerData.needsRebuild = false;
    }

    // Nothing changed since last render
    if (dirtyEntityIds.size === 0 && !needsFullRebuildRef.current) return;

    // Color is baked into the icon canvas by the draw function.
    // Billboard uses Color.WHITE so the canvas colors come through unchanged.
    // Alpha and visibility are still controlled via billboard.color.alpha.
    baseColorRef.current = Color.WHITE;

    // color_by layers carry the per-entity color via billboard.color, which Cesium
    // multiplies with the canvas — so draw the canvas white to render it unmultiplied.
    const colorBy = useUIStore.getState().layers[layerType]?.displayConfig?.colorBy;
    const iconCanvas = getIconCanvas(layerType, colorBy ? '#ffffff' : color);
    const scale = getIconScale(layerType, pointSize);
    const rotatable = supportsRotation(layerType);

    if (isFlightLayer) {
      const bbMap = billboardMapRef.current;
      const animMap = animStateRef.current;

      if (needsFullRebuildRef.current) {
        // --- Full rebuild path (first load or after clearLayerEntities) ---
        needsFullRebuildRef.current = false;
        const currentEntityIds = new Set<string>();
        let count = 0;

        for (const [entityId, rawEntity] of entityMap) {
          if (count >= maxEntities) break;

          const rawObs = obsMap.get(entityId);
          if (!rawObs) continue;
          const obs = normalizeObservation(rawObs);
          if (!obs) continue;

          currentEntityIds.add(entityId);
          count++;

          const newPos = Cartesian3.fromDegrees(obs.lon, obs.lat, obs.alt);
          const newRotation =
            rotatable && obs.heading != null ? extractHeadingRadians(obs.heading) : 0;

          const source = (rawEntity.source ?? 'live') as DataSource;
          const alpha = getSourceAlpha(source);
          // Live entities on flight layers rest at a dimmed alpha between updates
          const idleAlpha = source === 'live' ? LIVE_IDLE_ALPHA : alpha;
          let entityColor = Color.clone(baseColorRef.current);
          if (source === 'historical') {
            entityColor = desaturateColor(entityColor, 0.5);
          }
          entityColor.alpha = idleAlpha;

          const existingBb = bbMap.get(entityId);
          if (existingBb) {
            existingBb.position = newPos;
            existingBb.color = entityColor;
            existingBb.scale = scale;
            existingBb.rotation = newRotation;
            // No animation on full rebuild — snap to position
            animMap.delete(entityId);
          } else {
            const bb = collection.add({
              id: { entityId, layerId },
              position: newPos,
              image: iconCanvas,
              color: entityColor,
              scale,
              rotation: newRotation,
              alignedAxis: Cartesian3.UNIT_Z,
              disableDepthTestDistance: Number.POSITIVE_INFINITY,
            } as Billboard.ConstructorOptions);
            bbMap.set(entityId, bb);
          }
          entitySourceRef.current.set(entityId, source);
          entityIdleAlphaRef.current.set(entityId, idleAlpha);
        }

        // Remove departed entities (only needed on full rebuild)
        for (const [entityId, bb] of bbMap) {
          if (!currentEntityIds.has(entityId)) {
            collection.remove(bb);
            bbMap.delete(entityId);
            animMap.delete(entityId);
            entitySourceRef.current.delete(entityId);
            entityIdleAlphaRef.current.delete(entityId);
            interpolatedPositions.delete(entityId);
          }
        }
      } else {
        // --- O(delta) incremental path: only process dirty entities ---
        for (const entityId of dirtyEntityIds) {
          const rawEntity = entityMap.get(entityId);
          if (!rawEntity) continue;

          const rawObs = obsMap.get(entityId);
          if (!rawObs) continue;
          const obs = normalizeObservation(rawObs);
          if (!obs) continue;

          const newPos = Cartesian3.fromDegrees(obs.lon, obs.lat, obs.alt);
          const newRotation =
            rotatable && obs.heading != null ? extractHeadingRadians(obs.heading) : 0;

          const source = (rawEntity.source ?? 'live') as DataSource;
          const alpha = getSourceAlpha(source);
          // Live entities on flight layers rest at a dimmed alpha between updates
          const idleAlpha = source === 'live' ? LIVE_IDLE_ALPHA : alpha;
          let entityColor = Color.clone(baseColorRef.current);
          if (source === 'historical') {
            entityColor = desaturateColor(entityColor, 0.5);
          }
          entityColor.alpha = idleAlpha;

          const prevSource = entitySourceRef.current.get(entityId);
          const existingBb = bbMap.get(entityId);
          if (existingBb) {
            // Only animate if position actually changed
            const curPos = existingBb.position;
            const dx = Math.abs(curPos.x - newPos.x);
            const dy = Math.abs(curPos.y - newPos.y);
            const dz = Math.abs(curPos.z - newPos.z);
            if (dx > POS_EPSILON || dy > POS_EPSILON || dz > POS_EPSILON) {
              const existingAnim = animMap.get(entityId);
              const currentRotation = existingAnim ? existingAnim.toRotation : existingBb.rotation;
              // Pulse: live entities flash bright on position update, then fade to idle
              const fromAlpha =
                source === 'live'
                  ? LIVE_PULSE_ALPHA
                  : prevSource !== undefined
                    ? getSourceAlpha(prevSource)
                    : alpha;

              // Dead-reckoning fields for post-lerp extrapolation
              const headingRad =
                obs.heading != null ? CesiumMath.toRadians(obs.heading) : undefined;
              const groundSpeedMps =
                obs.groundSpeedKnots != null ? obs.groundSpeedKnots * KNOTS_TO_MPS : undefined;

              // Detect backward motion: server position is behind the extrapolated billboard.
              // When detected, snap to server position and pause dead-reckoning to let backend catch up.
              const wasExtrapolating =
                existingAnim &&
                existingAnim.headingRad != null &&
                Date.now() > existingAnim.startTime + existingAnim.duration;
              const backward =
                wasExtrapolating &&
                headingRad != null &&
                existingAnim!.headingRad != null &&
                isBackwardMotion(curPos, newPos, existingAnim!.headingRad!, headingRad);

              if (backward) {
                // Snap to server position — no lerp, no backward animation
                existingBb.position = newPos;
                existingBb.rotation = newRotation;
                interpolatedPositions.set(entityId, newPos);
                animMap.set(entityId, {
                  fromPos: newPos,
                  toPos: newPos,
                  fromRotation: newRotation,
                  toRotation: newRotation,
                  fromAlpha: source === 'live' ? LIVE_PULSE_ALPHA : idleAlpha,
                  toAlpha: idleAlpha,
                  startTime: Date.now(),
                  // Keep duration for alpha pulse fade even though position is snapped
                  duration: source === 'live' ? INTERP_DURATION_MS : 0,
                  groundSpeedMps,
                  headingRad,
                  pauseUntil: Date.now() + CATCHUP_PAUSE_MS,
                });
              } else {
                // Normal path: smooth lerp from current to new position
                const fromPos = Cartesian3.clone(existingBb.position);
                animMap.set(entityId, {
                  fromPos,
                  toPos: newPos,
                  fromRotation: currentRotation,
                  toRotation: newRotation,
                  fromAlpha,
                  toAlpha: idleAlpha,
                  startTime: Date.now(),
                  duration: INTERP_DURATION_MS,
                  groundSpeedMps,
                  headingRad,
                });
              }

              // Set billboard to pulse brightness so the first rendered frame is bright
              if (source === 'live') {
                entityColor.alpha = LIVE_PULSE_ALPHA;
              }
            }

            existingBb.color = entityColor;
            existingBb.scale = scale;
          } else {
            // New entity — add billboard (respect maxEntities)
            if (bbMap.size >= maxEntities) continue;

            const bb = collection.add({
              id: { entityId, layerId },
              position: newPos,
              image: iconCanvas,
              color: entityColor,
              scale,
              rotation: newRotation,
              alignedAxis: Cartesian3.UNIT_Z,
              disableDepthTestDistance: Number.POSITIVE_INFINITY,
            } as Billboard.ConstructorOptions);
            bbMap.set(entityId, bb);
          }
          entitySourceRef.current.set(entityId, source);
          entityIdleAlphaRef.current.set(entityId, idleAlpha);
        }

        // Clear dirty set after processing — direct mutation, no state update triggered
        dirtyEntityIds.clear();
      }
    } else {
      // Non-flight layers: O(delta) incremental updates via dirtyEntityIds.
      // Full rebuild only on first load or after clearLayerEntities. Non-flight
      // layers reuse billboards by mutating position/color/scale/rotation in
      // place (no dead-reckoning/lerp animation), add new entities, and — on
      // full rebuild — remove departed ones.
      const bbMap = billboardMapRef.current;

      // Computes the display color for a non-flight entity. If the layer declares
      // a metadata→color rule (displayConfig.colorBy, hoisted above), the color is
      // derived from the entity's metadata; otherwise the layer's base color is used.
      // Parse each distinct color_by hex once (≤ a handful of values per layer) and
      // reuse it — the per-entity loop then only allocates the alpha variant.
      const colorByCache = new Map<string, Color>();
      const parseColorByHex = (hex: string): Color => {
        let c = colorByCache.get(hex);
        if (!c) {
          c = Color.fromCssColorString(hex);
          colorByCache.set(hex, c);
        }
        return c;
      };
      const computeColor = (
        rawObs: typeof obsMap extends Map<string, infer V> ? V : never,
        source: DataSource,
        alpha: number,
      ): Color => {
        const hex = resolveColorByHex(colorBy, rawObs.metadata);
        if (hex) {
          // withAlpha returns a NEW Color, so the cached base is never mutated.
          let c = parseColorByHex(hex).withAlpha(alpha);
          if (source === 'historical') {
            c = desaturateColor(c, 0.5);
          }
          return c;
        }
        let entityColor = Color.clone(baseColorRef.current);
        if (source === 'historical') {
          entityColor = desaturateColor(entityColor, 0.5);
        }
        entityColor.alpha = alpha;
        return entityColor;
      };

      if (needsFullRebuildRef.current) {
        // --- Full rebuild path (first load or after clearLayerEntities) ---
        needsFullRebuildRef.current = false;
        const currentEntityIds = new Set<string>();
        let count = 0;

        for (const [entityId, rawEntity] of entityMap) {
          if (count >= maxEntities) break;

          const rawObs = obsMap.get(entityId);
          if (!rawObs) continue;
          const obs = normalizeObservation(rawObs);
          if (!obs) continue;

          currentEntityIds.add(entityId);
          count++;

          const position = Cartesian3.fromDegrees(obs.lon, obs.lat, obs.alt);
          const rotation =
            rotatable && obs.heading != null ? extractHeadingRadians(obs.heading) : 0;
          const source = (rawEntity.source ?? 'live') as DataSource;
          const alpha = getSourceAlpha(source);
          const entityColor = computeColor(rawObs, source, alpha);

          const existingBb = bbMap.get(entityId);
          if (existingBb) {
            // Reuse existing billboard — avoids GPU buffer destroy+recreate
            existingBb.position = position;
            existingBb.color = entityColor;
            existingBb.scale = scale;
            existingBb.rotation = rotation;
          } else {
            const bb = collection.add({
              id: { entityId, layerId },
              position,
              image: iconCanvas,
              color: entityColor,
              scale,
              rotation,
              alignedAxis: Cartesian3.UNIT_Z,
              disableDepthTestDistance: Number.POSITIVE_INFINITY,
            } as Billboard.ConstructorOptions);
            bbMap.set(entityId, bb);
          }
        }

        // Remove departed entities (only needed on full rebuild)
        for (const [entityId, bb] of bbMap) {
          if (!currentEntityIds.has(entityId)) {
            collection.remove(bb);
            bbMap.delete(entityId);
          }
        }
      } else {
        // --- O(delta) incremental path: only process dirty entities ---
        for (const entityId of dirtyEntityIds) {
          const rawEntity = entityMap.get(entityId);
          if (!rawEntity) continue;

          const rawObs = obsMap.get(entityId);
          if (!rawObs) continue;
          const obs = normalizeObservation(rawObs);
          if (!obs) continue;

          const position = Cartesian3.fromDegrees(obs.lon, obs.lat, obs.alt);
          const rotation =
            rotatable && obs.heading != null ? extractHeadingRadians(obs.heading) : 0;
          const source = (rawEntity.source ?? 'live') as DataSource;
          const alpha = getSourceAlpha(source);
          const entityColor = computeColor(rawObs, source, alpha);

          const existingBb = bbMap.get(entityId);
          if (existingBb) {
            existingBb.position = position;
            existingBb.color = entityColor;
            existingBb.scale = scale;
            existingBb.rotation = rotation;
          } else {
            // New entity — add billboard (respect maxEntities)
            if (bbMap.size >= maxEntities) continue;

            const bb = collection.add({
              id: { entityId, layerId },
              position,
              image: iconCanvas,
              color: entityColor,
              scale,
              rotation,
              alignedAxis: Cartesian3.UNIT_Z,
              disableDepthTestDistance: Number.POSITIVE_INFINITY,
            } as Billboard.ConstructorOptions);
            bbMap.set(entityId, bb);
          }
        }
      }

      // Clear dirty set after processing — direct mutation, no state update triggered
      dirtyEntityIds.clear();
    }

    // Mark occlusion as needing recalculation after data rebuild
    occlusionDirtyRef.current = true;
    // Immediate render request for this frame
    viewerRef.current?.scene.requestRender();
    // Belt-and-suspenders: schedule a second render request on the next frame.
    // In React StrictMode the effect cleanup/re-run cycle can consume the first
    // requestRender() before the billboard GPU upload completes.
    const rafId = requestAnimationFrame(() => {
      viewerRef.current?.scene.requestRender();
    });
    return () => cancelAnimationFrame(rafId);
  }, [
    version,
    hasData,
    layerId,
    layerType,
    color,
    pointSize,
    isFlightLayer,
    viewerRef,
    maxEntities,
    dirtyEntityIds,
    entityMap,
    obsMap,
  ]);

  // Effect 2b: Interpolation loop via scene.preUpdate — only for flight layers
  useEffect(() => {
    if (!isFlightLayer) return;

    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const onPreUpdate = () => {
      const animMap = animStateRef.current;
      const bbMap = billboardMapRef.current;
      if (animMap.size === 0) return;

      const now = Date.now();
      const scratch = scratchCartesian.current;
      let animated = false;
      const useSmoothMotion = smoothMotionRef.current;
      const clustered = clusteredMembersRef.current;

      for (const [entityId, state] of animMap) {
        // Clustered members are hidden inside a cluster marker — skip their
        // per-frame interpolation so they neither drift from the idle-computed
        // centroid nor force renders (which would defeat requestRenderMode).
        if (clustered && clustered.has(entityId)) continue;

        const bb = bbMap.get(entityId);
        if (!bb) {
          animMap.delete(entityId);
          interpolatedPositions.delete(entityId);
          continue;
        }

        const lerpEnd = state.startTime + state.duration;
        if (now < lerpEnd && state.duration > 0) {
          // Phase 1: Lerp interpolation (0→2s)
          const rawT = (now - state.startTime) / state.duration;
          const t = smoothstep(rawT);

          Cartesian3.lerp(state.fromPos, state.toPos, t, scratch);
          bb.position = Cartesian3.clone(scratch);
          bb.rotation = CesiumMath.lerp(state.fromRotation, state.toRotation, t);

          // Lerp alpha when source changed (stale→live transition)
          if (state.fromAlpha !== state.toAlpha && bb.color) {
            const lerpedAlpha = state.fromAlpha + (state.toAlpha - state.fromAlpha) * t;
            const c = bb.color;
            bb.color = new Color(c.red, c.green, c.blue, lerpedAlpha);
          }

          interpolatedPositions.set(entityId, bb.position);
          animated = true;
        } else if (useSmoothMotion && state.groundSpeedMps && state.headingRad != null) {
          // Check catchup pause — skip dead-reckoning while backend catches up
          if (state.pauseUntil && now < state.pauseUntil) {
            // Paused: hold at server position, keep entry alive for resume
            interpolatedPositions.set(entityId, bb.position);
            animated = true;
            continue;
          }

          // Phase 2: Dead-reckoning extrapolation (post-lerp or post-pause)
          // Use pauseUntil as extrapolation origin if a pause occurred, otherwise lerpEnd
          const extrapolateStart = state.pauseUntil ?? lerpEnd;
          const elapsed = (now - extrapolateStart) / 1000;
          if (elapsed > 0 && elapsed < MAX_EXTRAPOLATE_SEC) {
            const distM = state.groundSpeedMps * elapsed;
            bb.position = projectPosition(state.toPos, state.headingRad, distM);
            interpolatedPositions.set(entityId, bb.position);
            animated = true;
          } else {
            // Exceeded max extrapolation — freeze and clean up
            interpolatedPositions.set(entityId, bb.position);
            animMap.delete(entityId);
          }
        } else {
          // No extrapolation — lerp completed, clean up
          interpolatedPositions.set(entityId, bb.position);
          animMap.delete(entityId);
        }
      }

      if (animated) {
        viewerRef.current?.scene.requestRender();
      }
    };

    viewer.scene.preUpdate.addEventListener(onPreUpdate);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.scene.preUpdate.removeEventListener(onPreUpdate);
      }
    };
  }, [viewerRef, isFlightLayer]);

  // Effect 2c: When showOccluded changes, mark dirty so the stable Effect 3
  // listeners pick up the new alpha ref on the next postRender frame.
  useEffect(() => {
    occlusionDirtyRef.current = true;
  }, [showOccluded]);

  // Effect 3: Fade occluded billboards (behind the globe) on camera change.
  // Throttled: only runs every 3rd postRender frame to reduce O(N) iteration cost.
  useEffect(() => {
    const viewer = viewerRef.current;
    if (!viewer || viewer.isDestroyed()) return;

    const occluder = new EllipsoidalOccluder(Ellipsoid.WGS84, Cartesian3.ZERO);
    const scratchColor = new Color();
    let frameCounter = 0;

    const updateOcclusion = () => {
      const collection = collectionRef.current;
      if (!collection || collection.isDestroyed() || collection.length === 0) return;

      occluder.cameraPosition = viewer.camera.positionWC;

      const occludedAlpha = occludedAlphaRef.current;
      const count = collection.length;
      let changed = false;

      // Use cached refs (updated via subscribe) — no per-frame getState() needed
      const fm = findModeRef.current;
      const pinnedIds = fm ? pinnedSetRef.current : null;
      const idleAlphas = entityIdleAlphaRef.current;

      for (let i = 0; i < count; i++) {
        const billboard = collection.get(i);
        if (!billboard?.color) continue;
        const visible = occluder.isPointVisible(billboard.position);

        const entityId = getBillboardEntityId(billboard);

        // Pinned entities in find mode: never fully hidden by occlusion
        if (pinnedIds && entityId && pinnedIds.has(entityId)) {
          const pinnedAlpha = visible ? VISIBLE_ALPHA : Math.max(occludedAlpha, 0.12);
          if (Math.abs(billboard.color.alpha - pinnedAlpha) > 0.01) {
            Color.clone(billboard.color, scratchColor);
            scratchColor.alpha = pinnedAlpha;
            billboard.color = scratchColor;
            changed = true;
          }
          continue;
        }

        // Use per-entity idle alpha so pulsing flight entities restore to their
        // dimmed resting state (not full brightness) when rotating back into view
        const restAlpha = entityId ? (idleAlphas.get(entityId) ?? VISIBLE_ALPHA) : VISIBLE_ALPHA;
        const targetAlpha = visible
          ? billboard.color.alpha > occludedAlpha + 0.01
            ? billboard.color.alpha
            : restAlpha
          : occludedAlpha;
        // Only update if alpha actually changed to avoid unnecessary GPU uploads
        if (Math.abs(billboard.color.alpha - targetAlpha) > 0.01) {
          Color.clone(billboard.color, scratchColor);
          scratchColor.alpha = targetAlpha;
          billboard.color = scratchColor;
          changed = true;
        }
      }
      if (changed) {
        viewer.scene.requestRender();
      }
    };

    const onCameraChanged = () => {
      occlusionDirtyRef.current = true;
    };

    // Throttled postRender: only check occlusion every 3rd frame to amortize O(N) cost
    const onPostRender = () => {
      if (occlusionDirtyRef.current) {
        occlusionDirtyRef.current = false;
        updateOcclusion();
        return;
      }
      frameCounter++;
      if (frameCounter >= 3) {
        frameCounter = 0;
        // Lightweight check — only needed if animations are running
        if (animStateRef.current.size > 0) {
          updateOcclusion();
        }
      }
    };

    viewer.camera.changed.addEventListener(onCameraChanged);
    viewer.scene.postRender.addEventListener(onPostRender);

    return () => {
      if (!viewer.isDestroyed()) {
        viewer.camera.changed.removeEventListener(onCameraChanged);
        viewer.scene.postRender.removeEventListener(onPostRender);
      }
    };
  }, [viewerRef]);

  // Effect 4: Sole owner of billboard `show` — entity isolation AND cluster
  // member suppression. Precedence: isolation/find-mode wins (an isolated entity
  // stays visible even if it is also a cluster member); otherwise members
  // absorbed into a cluster marker are hidden.
  useEffect(() => {
    const collection = collectionRef.current;
    if (!collection || collection.isDestroyed()) return;

    const count = collection.length;
    const clustered = clusteredMembers;
    if (isolatedEntityId !== null) {
      // Isolation active — only show the isolated entity's billboard
      for (let i = 0; i < count; i++) {
        const bb = collection.get(i);
        bb.show = getBillboardEntityId(bb) === isolatedEntityId;
      }
    } else if (clustered && clustered.size > 0) {
      // Clustering active — hide entities absorbed into a cluster marker
      for (let i = 0; i < count; i++) {
        const bb = collection.get(i);
        const entityId = getBillboardEntityId(bb);
        bb.show = entityId ? !clustered.has(entityId) : true;
      }
    } else {
      // No isolation, no clustering — show all
      for (let i = 0; i < count; i++) {
        collection.get(i).show = true;
      }
    }
    viewerRef.current?.scene.requestRender();
  }, [isolatedEntityId, version, viewerRef, clusteredMembers]);

  return null;
};
