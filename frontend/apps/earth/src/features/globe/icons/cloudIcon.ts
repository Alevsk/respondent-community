// Cloud — bold filled cloud with three bumps, thick outline via cutout.

const SIZE = 32;
const CX = SIZE / 2;

export function drawCloudIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Solid filled cloud — three bumps + flat base
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(3, 24);
  ctx.lineTo(29, 24);
  ctx.arc(23, 18, 6, 0, -Math.PI, true);
  ctx.arc(CX, 12, 8, -0.1, -Math.PI + 0.1, true);
  ctx.arc(9, 18, 6, 0, -Math.PI, true);
  ctx.closePath();
  ctx.fill();
}
