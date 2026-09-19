// Lightning — bold zigzag bolt inside a filled circle.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawLightningIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout circle
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 10, 0, Math.PI * 2);
  ctx.fill();

  // Bold lightning bolt inside
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX + 4, CY - 10);
  ctx.lineTo(CX - 5, CY + 1);
  ctx.lineTo(CX, CY + 1);
  ctx.lineTo(CX - 4, CY + 10);
  ctx.lineTo(CX + 5, CY - 1);
  ctx.lineTo(CX, CY - 1);
  ctx.closePath();
  ctx.fill();
}
