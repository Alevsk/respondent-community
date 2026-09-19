const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawHelicopterIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer ring — filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  // Inner cutout — creates a ring
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 9, 0, Math.PI * 2);
  ctx.fill();

  // Cross / plus inscribed through ring — horizontal bar
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX - 15, CY - 1);
  ctx.lineTo(CX + 15, CY - 1);
  ctx.lineTo(CX + 15, CY + 1);
  ctx.lineTo(CX - 15, CY + 1);
  ctx.closePath();
  ctx.fill();

  // Cross — vertical bar
  ctx.beginPath();
  ctx.moveTo(CX - 1, CY - 15);
  ctx.lineTo(CX + 1, CY - 15);
  ctx.lineTo(CX + 1, CY + 15);
  ctx.lineTo(CX - 1, CY + 15);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
