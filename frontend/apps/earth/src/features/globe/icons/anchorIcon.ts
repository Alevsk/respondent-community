// Anchor — bold anchor silhouette inside a filled circle ring.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawAnchorIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 14, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout circle
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 11, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = color;

  // Ring at top
  ctx.beginPath();
  ctx.arc(CX, CY - 6, 3, 0, Math.PI * 2);
  ctx.fill();
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY - 6, 1.5, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = color;

  // Shank — thick vertical bar
  ctx.fillRect(CX - 1.5, CY - 3, 3, 12);

  // Crossbar
  ctx.fillRect(CX - 6, CY - 2, 12, 3);

  // Left arm — curved hook
  ctx.beginPath();
  ctx.arc(CX, CY + 9, 7, Math.PI * 0.7, Math.PI * 1.0, false);
  ctx.arc(CX, CY + 9, 4, Math.PI * 1.0, Math.PI * 0.7, true);
  ctx.closePath();
  ctx.fill();

  // Right arm — curved hook
  ctx.beginPath();
  ctx.arc(CX, CY + 9, 7, Math.PI * 0.0, Math.PI * 0.3, false);
  ctx.arc(CX, CY + 9, 4, Math.PI * 0.3, Math.PI * 0.0, true);
  ctx.closePath();
  ctx.fill();
}
