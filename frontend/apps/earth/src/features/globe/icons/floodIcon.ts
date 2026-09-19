const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawFloodIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Filled circle base — water body
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  // Three horizontal bar cutouts — stylized water layers
  ctx.fillStyle = '#000000';

  // Top bar cutout
  ctx.beginPath();
  ctx.rect(CX - 9, CY - 9, 18, 3.5);
  ctx.fill();

  // Middle bar cutout
  ctx.beginPath();
  ctx.rect(CX - 9, CY - 1.75, 18, 3.5);
  ctx.fill();

  // Bottom bar cutout
  ctx.beginPath();
  ctx.rect(CX - 9, CY + 5.5, 18, 3.5);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2, 0, Math.PI * 2);
  ctx.fill();
}
