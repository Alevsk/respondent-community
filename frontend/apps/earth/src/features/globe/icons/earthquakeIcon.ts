// Earthquake: solid center dot + 2 concentric rings with decreasing opacity
// Drawn white on 32x32 canvas

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawEarthquakeIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.lineWidth = 1.5;

  // Outer ring — lowest opacity
  ctx.globalAlpha = 0.3;
  ctx.strokeStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.stroke();

  // Middle ring
  ctx.globalAlpha = 0.6;
  ctx.strokeStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 8, 0, Math.PI * 2);
  ctx.stroke();

  // Center dot — full opacity
  ctx.globalAlpha = 1.0;
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 3.5, 0, Math.PI * 2);
  ctx.fill();
}
