const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawWarehouseIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer wide low rectangle — landscape orientation
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.rect(3, CY - 7, 26, 14);
  ctx.fill();

  // Black cutout — simple inset rectangle, no dividers
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.rect(8, CY - 3, 16, 6);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
