/**
 * Icon preview script — renders all registered shapes into the icon-preview.html page.
 * Run with: npm run icons
 */

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

type DrawFn = (ctx: CanvasRenderingContext2D, color: string) => void;

interface IconEntry {
  name: string;
  drawFn: DrawFn;
  status: 'yes' | 'maybe' | 'no' | 'needs-improvement';
}

const ICONS: IconEntry[] = [
  { name: 'flight', drawFn: drawFlightIcon, status: 'yes' },
  { name: 'diamond', drawFn: drawDiamondIcon, status: 'yes' },
  { name: 'ripple', drawFn: drawEarthquakeIcon, status: 'yes' },
  { name: 'radio', drawFn: drawRadioIcon, status: 'needs-improvement' },
  { name: 'warning', drawFn: drawWarningIcon, status: 'yes' },
  { name: 'radiation', drawFn: drawRadiationIcon, status: 'yes' },
  { name: 'fire', drawFn: drawFireIcon, status: 'yes' },
  { name: 'volcano', drawFn: drawVolcanoIcon, status: 'no' },
  { name: 'missile', drawFn: drawMissileIcon, status: 'no' },
  { name: 'drone', drawFn: drawDroneIcon, status: 'yes' },
  { name: 'helicopter', drawFn: drawHelicopterIcon, status: 'yes' },
  { name: 'shield', drawFn: drawShieldIcon, status: 'yes' },
  { name: 'crosshair', drawFn: drawCrosshairIcon, status: 'yes' },
  { name: 'radar', drawFn: drawRadarIcon, status: 'no' },
  { name: 'explosion', drawFn: drawExplosionIcon, status: 'yes' },
  { name: 'tank', drawFn: drawTankIcon, status: 'no' },
  { name: 'ship', drawFn: drawShipIcon, status: 'yes' },
  { name: 'anchor', drawFn: drawAnchorIcon, status: 'no' },
  { name: 'wave', drawFn: drawWaveIcon, status: 'no' },
  { name: 'lightning', drawFn: drawLightningIcon, status: 'no' },
  { name: 'cloud', drawFn: drawCloudIcon, status: 'no' },
  { name: 'wind', drawFn: drawWindIcon, status: 'no' },
  { name: 'tornado', drawFn: drawTornadoIcon, status: 'yes' },
  { name: 'snowflake', drawFn: drawSnowflakeIcon, status: 'yes' },
  { name: 'tower', drawFn: drawTowerIcon, status: 'maybe' },
  { name: 'marker', drawFn: drawMarkerIcon, status: 'yes' },
  { name: 'building', drawFn: drawBuildingIcon, status: 'no' },
  { name: 'bridge', drawFn: drawBridgeIcon, status: 'no' },
  { name: 'factory', drawFn: drawFactoryIcon, status: 'maybe' },
  { name: 'powerplant', drawFn: drawPowerplantIcon, status: 'maybe' },
  { name: 'crane', drawFn: drawCraneIcon, status: 'no' },
  { name: 'warehouse', drawFn: drawWarehouseIcon, status: 'no' },
  { name: 'biohazard', drawFn: drawBiohazardIcon, status: 'yes' },
  { name: 'skull', drawFn: drawSkullIcon, status: 'no' },
  { name: 'flood', drawFn: drawFloodIcon, status: 'no' },
  { name: 'hurricane', drawFn: drawHurricaneIcon, status: 'no' },
  { name: 'meteor', drawFn: drawMeteorIcon, status: 'no' },
  { name: 'tree', drawFn: drawTreeIcon, status: 'maybe' },
  { name: 'crack', drawFn: drawEarthquakeCrackIcon, status: 'maybe' },
  { name: 'star', drawFn: drawStarIcon, status: 'yes' },
  { name: 'hexagon', drawFn: drawHexagonIcon, status: 'yes' },
  { name: 'circle-ring', drawFn: drawCircleRingIcon, status: 'yes' },
  { name: 'chevron', drawFn: drawChevronIcon, status: 'no' },
  { name: 'triangle', drawFn: drawTriangleIcon, status: 'yes' },
  { name: 'pentagon', drawFn: drawPentagonIcon, status: 'yes' },
  { name: 'cross', drawFn: drawCrossIcon, status: 'yes' },
  { name: 'rocket', drawFn: drawRocketIcon, status: 'yes' },
  { name: 'telescope', drawFn: drawTelescopeIcon, status: 'no' },
  { name: 'bullseye', drawFn: drawBullseyeIcon, status: 'yes' },
  { name: 'nuclear', drawFn: drawNuclearIcon, status: 'yes' },
  { name: 'dot', drawFn: drawDefaultIcon, status: 'yes' },
  { name: 'iss', drawFn: drawIssIcon, status: 'maybe' },
  { name: 'flight-alt-a', drawFn: drawFlightAltAIcon, status: 'maybe' },
  { name: 'flight-alt-b', drawFn: drawFlightAltBIcon, status: 'maybe' },
  { name: 'flight-alt-c', drawFn: drawFlightAltCIcon, status: 'maybe' },
];

const ICON_SIZE = 32;
const CANVAS_SCALE = 2;

function renderIcon(drawFn: DrawFn, color: string): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = ICON_SIZE * CANVAS_SCALE;
  canvas.height = ICON_SIZE * CANVAS_SCALE;
  const ctx = canvas.getContext('2d')!;
  ctx.scale(CANVAS_SCALE, CANVAS_SCALE);
  drawFn(ctx, color);
  return canvas;
}

function buildGrid(color: string, scale: string, bg: string, hideYes: boolean) {
  const grid = document.getElementById('grid')!;
  grid.innerHTML = '';
  if (bg === 'earth') {
    document.body.style.background =
      'url("https://upload.wikimedia.org/wikipedia/commons/thumb/2/23/Blue_Marble_2002.png/1280px-Blue_Marble_2002.png") center/cover no-repeat #000';
  } else {
    document.body.style.background = bg;
  }

  for (const icon of ICONS) {
    if (hideYes && icon.status === 'yes') continue;

    const cell = document.createElement('div');
    cell.className = `icon-cell status-${icon.status}`;

    const canvases = document.createElement('div');
    canvases.className = 'canvases';

    const canvas = renderIcon(icon.drawFn, color);
    canvas.className = scale === '1x' ? 'native' : scale === '4x' ? 'x4' : 'x2';
    canvases.appendChild(canvas);
    cell.appendChild(canvases);

    const label = document.createElement('div');
    label.className = 'icon-label';
    label.textContent = icon.name;
    cell.appendChild(label);

    const status = document.createElement('div');
    status.className = `icon-status ${icon.status}`;
    status.textContent = icon.status;
    cell.appendChild(status);

    grid.appendChild(cell);
  }
}

// Initial render
let currentColor = '#00ff9d';
let currentScale = '2x';
let currentBg = '#000000';
let currentHideYes = false;

buildGrid(currentColor, currentScale, currentBg, currentHideYes);

// Controls
document.getElementById('colorPicker')!.addEventListener('input', (e) => {
  currentColor = (e.target as HTMLInputElement).value;
  buildGrid(currentColor, currentScale, currentBg, currentHideYes);
});

document.getElementById('scalePicker')!.addEventListener('change', (e) => {
  currentScale = (e.target as HTMLSelectElement).value;
  buildGrid(currentColor, currentScale, currentBg, currentHideYes);
});

document.getElementById('bgPicker')!.addEventListener('change', (e) => {
  currentBg = (e.target as HTMLSelectElement).value;
  buildGrid(currentColor, currentScale, currentBg, currentHideYes);
});

document.getElementById('filterNo')!.addEventListener('change', (e) => {
  currentHideYes = (e.target as HTMLInputElement).checked;
  buildGrid(currentColor, currentScale, currentBg, currentHideYes);
});
