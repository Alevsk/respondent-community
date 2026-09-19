const SIZE = 32;
const CX = SIZE / 2;

export function drawMissileIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Solid filled missile — pointed nose, wide body, tail fins
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX, 2); // nose tip
  ctx.lineTo(CX + 5, 9); // nose right shoulder
  ctx.lineTo(CX + 5, 22); // body right edge
  ctx.lineTo(CX + 10, 29); // right fin tip
  ctx.lineTo(CX + 5, 25); // right fin inner
  ctx.lineTo(CX - 5, 25); // left fin inner
  ctx.lineTo(CX - 10, 29); // left fin tip
  ctx.lineTo(CX - 5, 22); // body left edge
  ctx.lineTo(CX - 5, 9); // nose left shoulder
  ctx.closePath();
  ctx.fill();
}
