// Cooling tower — hourglass silhouette, wide at top and base, pinched at waist.
// No steam strokes; the distinctive hourglass shape is sufficient.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawPowerplantIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer hourglass — wide top, pinched mid, wide base
  ctx.beginPath();
  ctx.moveTo(CX - 12, 4);
  ctx.lineTo(CX + 12, 4);
  ctx.arcTo(CX + 5, CY, CX + 10, 30, 14);
  ctx.lineTo(CX + 10, 30);
  ctx.lineTo(CX - 10, 30);
  ctx.arcTo(CX - 5, CY, CX - 12, 4, 14);
  ctx.closePath();
  ctx.fill();

  // Inner black cutout — mirrors the hourglass inside
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX - 9, 6);
  ctx.lineTo(CX + 9, 6);
  ctx.arcTo(CX + 3.5, CY, CX + 7, 28, 10);
  ctx.lineTo(CX + 7, 28);
  ctx.lineTo(CX - 7, 28);
  ctx.arcTo(CX - 3.5, CY, CX - 9, 6, 10);
  ctx.closePath();
  ctx.fill();

  // Center dot at waist
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
