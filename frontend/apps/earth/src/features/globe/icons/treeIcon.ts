const SIZE = 32;
const CX = SIZE / 2;

export function drawTreeIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Canopy: solid filled upward-pointing triangle
  ctx.beginPath();
  ctx.moveTo(CX, 4);
  ctx.lineTo(CX - 11, 22);
  ctx.lineTo(CX + 11, 22);
  ctx.closePath();
  ctx.fill();

  // Trunk: thin vertical rectangle
  ctx.beginPath();
  ctx.rect(CX - 1.5, 22, 3, 7);
  ctx.fill();
}
