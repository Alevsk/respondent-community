import { drawFlightIcon } from './flightIcon';
import { drawDiamondIcon } from './diamondIcon';
import { drawEarthquakeIcon } from './earthquakeIcon';
import { drawDefaultIcon } from './defaultIcon';
import { drawRadioIcon } from './radioIcon';
import { drawWarningIcon } from './warningIcon';
import { drawRadiationIcon } from './radiationIcon';
import { drawFireIcon } from './fireIcon';
import { drawMissileIcon } from './missileIcon';
import { drawDroneIcon } from './droneIcon';
import { drawHelicopterIcon } from './helicopterIcon';
import { drawShieldIcon } from './shieldIcon';
import { drawCrosshairIcon } from './crosshairIcon';
import { drawRadarIcon } from './radarIcon';
import { drawExplosionIcon } from './explosionIcon';
import { drawTankIcon } from './tankIcon';
import { drawShipIcon } from './shipIcon';
import { drawAnchorIcon } from './anchorIcon';
import { drawWaveIcon } from './waveIcon';
import { drawLightningIcon } from './lightningIcon';
import { drawCloudIcon } from './cloudIcon';
import { drawWindIcon } from './windIcon';
import { drawTornadoIcon } from './tornadoIcon';
import { drawSnowflakeIcon } from './snowflakeIcon';
import { drawTowerIcon } from './towerIcon';
import { drawMarkerIcon } from './markerIcon';
import { drawBuildingIcon } from './buildingIcon';
import { drawBridgeIcon } from './bridgeIcon';
import { drawFactoryIcon } from './factoryIcon';
import { drawPowerplantIcon } from './powerplantIcon';
import { drawCraneIcon } from './craneIcon';
import { drawWarehouseIcon } from './warehouseIcon';
import { drawBiohazardIcon } from './biohazardIcon';
import { drawSkullIcon } from './skullIcon';
import { drawFloodIcon } from './floodIcon';
import { drawHurricaneIcon } from './hurricaneIcon';
import { drawMeteorIcon } from './meteorIcon';
import { drawTreeIcon } from './treeIcon';
import { drawEarthquakeCrackIcon } from './earthquakeCrackIcon';
import { drawStarIcon } from './starIcon';
import { drawHexagonIcon } from './hexagonIcon';
import { drawCircleRingIcon } from './circleRingIcon';
import { drawChevronIcon } from './chevronIcon';
import { drawTriangleIcon } from './triangleIcon';
import { drawPentagonIcon } from './pentagonIcon';
import { drawCrossIcon } from './crossIcon';
import { drawRocketIcon } from './rocketIcon';
import { drawTelescopeIcon } from './telescopeIcon';
import { drawBullseyeIcon } from './bullseyeIcon';
import { drawNuclearIcon } from './nuclearIcon';
import { drawVolcanoIcon } from './volcanoIcon';
import { drawIssIcon } from './issIcon';
import { drawFlightAltAIcon } from './flightAltAIcon';
import { drawFlightAltBIcon } from './flightAltBIcon';
import { drawFlightAltCIcon } from './flightAltCIcon';

export type IconDrawFn = (ctx: CanvasRenderingContext2D, color: string) => void;

const ICON_SIZE = 32;
const CANVAS_SCALE = 2; // Render at 2x for crisp icons when Cesium scales billboards

// Static draw function lookup keyed by named shape from declarative source YAML.
const SHAPE_TO_DRAW_FN: Record<string, IconDrawFn> = {
  flight: drawFlightIcon,
  diamond: drawDiamondIcon,
  satellite: drawDiamondIcon, // alias for backwards compat
  ripple: drawEarthquakeIcon,
  radio: drawRadioIcon,
  warning: drawWarningIcon,
  radiation: drawRadiationIcon,
  fire: drawFireIcon,
  missile: drawMissileIcon,
  drone: drawDroneIcon,
  helicopter: drawHelicopterIcon,
  shield: drawShieldIcon,
  crosshair: drawCrosshairIcon,
  radar: drawRadarIcon,
  explosion: drawExplosionIcon,
  tank: drawTankIcon,
  ship: drawShipIcon,
  anchor: drawAnchorIcon,
  wave: drawWaveIcon,
  lightning: drawLightningIcon,
  cloud: drawCloudIcon,
  wind: drawWindIcon,
  tornado: drawTornadoIcon,
  snowflake: drawSnowflakeIcon,
  tower: drawTowerIcon,
  marker: drawMarkerIcon,
  building: drawBuildingIcon,
  bridge: drawBridgeIcon,
  factory: drawFactoryIcon,
  powerplant: drawPowerplantIcon,
  crane: drawCraneIcon,
  warehouse: drawWarehouseIcon,
  biohazard: drawBiohazardIcon,
  skull: drawSkullIcon,
  flood: drawFloodIcon,
  hurricane: drawHurricaneIcon,
  meteor: drawMeteorIcon,
  tree: drawTreeIcon,
  crack: drawEarthquakeCrackIcon,
  star: drawStarIcon,
  hexagon: drawHexagonIcon,
  'circle-ring': drawCircleRingIcon,
  chevron: drawChevronIcon,
  triangle: drawTriangleIcon,
  pentagon: drawPentagonIcon,
  cross: drawCrossIcon,
  rocket: drawRocketIcon,
  telescope: drawTelescopeIcon,
  bullseye: drawBullseyeIcon,
  nuclear: drawNuclearIcon,
  volcano: drawVolcanoIcon,
  iss: drawIssIcon,
  'flight-alt-a': drawFlightAltAIcon,
  'flight-alt-b': drawFlightAltBIcon,
  'flight-alt-c': drawFlightAltCIcon,
  dot: drawDefaultIcon,
};

// ------------------------------------------------------------------
// Dynamic registration registry
// Declarative source definitions (from YAML → GetLayers API) register
// their icon config here so the renderer picks it up without a code change.
// ------------------------------------------------------------------
interface DynamicIconRegistration {
  drawFn: IconDrawFn;
  rotatable: boolean;
  scale: number;
}

const dynamicIconRegistry = new Map<string, DynamicIconRegistration>();

const canvasCache = new Map<string, HTMLCanvasElement>();
// Reverse index: layerType → set of cache keys for O(1) invalidation.
const cacheKeysByType = new Map<string, Set<string>>();

function createIconCanvas(drawFn: IconDrawFn, color: string): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = ICON_SIZE * CANVAS_SCALE;
  canvas.height = ICON_SIZE * CANVAS_SCALE;
  const ctx = canvas.getContext('2d');
  if (ctx) {
    ctx.scale(CANVAS_SCALE, CANVAS_SCALE);
    drawFn(ctx, color);
  }
  return canvas;
}

/**
 * Register a dynamic icon for a layer type from a declarative source definition.
 * The `shape` string maps to one of the built-in draw functions.
 *
 * Idempotent: if an identical registration already exists (same shape, rotatable,
 * scale), this is a no-op — the canvas cache is not invalidated. This prevents
 * cache thrashing when React Query refetches layers.
 */
export function registerDynamicIcon(
  layerType: string,
  config: { shape: string; rotatable: boolean; scale: number },
): void {
  const drawFn = SHAPE_TO_DRAW_FN[config.shape] ?? drawDefaultIcon;

  // Skip re-registration if the existing entry is identical — avoids unnecessary
  // canvas cache invalidation on every React Query refetch.
  const existing = dynamicIconRegistry.get(layerType);
  if (
    existing &&
    existing.drawFn === drawFn &&
    existing.rotatable === config.rotatable &&
    existing.scale === config.scale
  ) {
    return;
  }

  dynamicIconRegistry.set(layerType, {
    drawFn,
    rotatable: config.rotatable,
    scale: config.scale,
  });
  // O(1) invalidation: delete only the cache keys belonging to this layer type.
  const keys = cacheKeysByType.get(layerType);
  if (keys) {
    for (const key of keys) canvasCache.delete(key);
    cacheKeysByType.delete(layerType);
  }
}

/**
 * Get the cached icon canvas for a layer type.
 * The color is baked into the canvas by the draw function — the billboard
 * should use Color.WHITE so the drawn colors come through unchanged.
 */
export function getIconCanvas(layerType: string, color?: string): HTMLCanvasElement {
  const resolvedColor = color ?? '#ffffff';
  const cacheKey = `${layerType}:${resolvedColor}`;

  const cached = canvasCache.get(cacheKey);
  if (cached) return cached;

  // Dynamic registrations checked first; absent config falls back to generic default.
  const dynamic = dynamicIconRegistry.get(layerType);
  const drawFn = dynamic?.drawFn ?? drawDefaultIcon;

  const canvas = createIconCanvas(drawFn, resolvedColor);
  canvasCache.set(cacheKey, canvas);

  // Track cache key in the reverse index for O(1) invalidation.
  let keys = cacheKeysByType.get(layerType);
  if (!keys) {
    keys = new Set();
    cacheKeysByType.set(layerType, keys);
  }
  keys.add(cacheKey);

  return canvas;
}

export function supportsRotation(layerType: string): boolean {
  // Dynamic registration drives rotation; unregistered layers are non-rotatable.
  const dynamic = dynamicIconRegistry.get(layerType);
  return dynamic?.rotatable ?? false;
}

export function getIconScale(layerType: string, pointSize?: number): number {
  const baseScale = (pointSize ?? 8) / 8;
  // Dynamic scale is an absolute multiplier set by the declarative config.
  // Unregistered layers use a multiplier of 1.0 (no scaling adjustment).
  // Divide by CANVAS_SCALE because the canvas is rendered at 2x resolution.
  const dynamic = dynamicIconRegistry.get(layerType);
  const typeMultiplier = dynamic?.scale ?? 1.0;
  return (baseScale * typeMultiplier) / CANVAS_SCALE;
}

/** Exposed for testing only — clears all dynamic registrations and the canvas cache. */
export function _resetDynamicRegistry(): void {
  dynamicIconRegistry.clear();
  canvasCache.clear();
  cacheKeysByType.clear();
}
