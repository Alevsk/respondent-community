// Crane — bold filled inverted-L: thick vertical mast + thick horizontal jib.
// Single filled polygon for the L-shape. Black cutout inside. Center dot. No strokes.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawCraneIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer L-shape — vertical mast + horizontal jib as one polygon
  ctx.beginPath();
  ctx.moveTo(CX - 9, 5); // top-left of jib
  ctx.lineTo(CX + 11, 5); // top-right of jib
  ctx.lineTo(CX + 11, 11); // bottom-right of jib
  ctx.lineTo(CX - 3, 11); // inner corner where jib meets mast
  ctx.lineTo(CX - 3, 28); // bottom of mast right
  ctx.lineTo(CX - 9, 28); // bottom of mast left
  ctx.closePath();
  ctx.fill();

  // Black cutout — inset L to create thick outline effect
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX - 7, 7);
  ctx.lineTo(CX + 9, 7);
  ctx.lineTo(CX + 9, 9);
  ctx.lineTo(CX - 5, 9);
  ctx.lineTo(CX - 5, 26);
  ctx.lineTo(CX - 7, 26);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX - 6, CY, 2, 0, Math.PI * 2);
  ctx.fill();
}
