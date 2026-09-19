// Industrial facility — single unified filled polygon (building + chimney), black cutout, center dot.

const SIZE = 32;
const CX = SIZE / 2;

export function drawFactoryIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer shape: building body with a single wide chimney as one polygon
  ctx.beginPath();
  ctx.moveTo(CX - 11, 30); // bottom-left of building
  ctx.lineTo(CX - 11, 15); // top-left of building
  ctx.lineTo(CX - 6, 15); // left shoulder of chimney
  ctx.lineTo(CX - 6, 5); // top-left of chimney
  ctx.lineTo(CX + 6, 5); // top-right of chimney
  ctx.lineTo(CX + 6, 15); // right shoulder of chimney
  ctx.lineTo(CX + 11, 15); // top-right of building
  ctx.lineTo(CX + 11, 30); // bottom-right of building
  ctx.closePath();
  ctx.fill();

  // Inner black cutout
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX - 8, 28);
  ctx.lineTo(CX - 8, 18);
  ctx.lineTo(CX - 3, 18);
  ctx.lineTo(CX - 3, 8);
  ctx.lineTo(CX + 3, 8);
  ctx.lineTo(CX + 3, 18);
  ctx.lineTo(CX + 8, 18);
  ctx.lineTo(CX + 8, 28);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, 23, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
