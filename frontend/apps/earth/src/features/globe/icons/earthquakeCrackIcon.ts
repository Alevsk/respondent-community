// Earthquake crack icon: solid filled circle with black crack zigzag cut through it.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawEarthquakeCrackIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Solid filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  // Black crack zigzag running top-to-bottom through center
  ctx.strokeStyle = '#000000';
  ctx.lineWidth = 2;
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';

  // Main crack: top to bottom zigzag
  ctx.beginPath();
  ctx.moveTo(CX + 1, CY - 12);
  ctx.lineTo(CX - 3, CY - 6);
  ctx.lineTo(CX + 2, CY - 2);
  ctx.lineTo(CX - 2, CY + 3);
  ctx.lineTo(CX + 3, CY + 7);
  ctx.lineTo(CX, CY + 12);
  ctx.stroke();

  // Small branch from the second joint going right
  ctx.beginPath();
  ctx.moveTo(CX + 2, CY - 2);
  ctx.lineTo(CX + 6, CY - 4);
  ctx.stroke();

  // Small branch from the third joint going left
  ctx.beginPath();
  ctx.moveTo(CX - 2, CY + 3);
  ctx.lineTo(CX - 6, CY + 2);
  ctx.stroke();
}
