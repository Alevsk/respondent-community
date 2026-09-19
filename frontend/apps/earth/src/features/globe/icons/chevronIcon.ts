// Chevron — upward-pointing directional wedge with stern notch.

const SIZE = 32;
const CX = SIZE / 2;

export function drawChevronIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Solid filled chevron — wide base tapering to upward point
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX, 4); // top tip
  ctx.lineTo(CX + 11, 26); // right outer
  ctx.lineTo(CX + 5, 26); // right inner notch
  ctx.lineTo(CX, 21); // center indent
  ctx.lineTo(CX - 5, 26); // left inner notch
  ctx.lineTo(CX - 11, 26); // left outer
  ctx.closePath();
  ctx.fill();
}
