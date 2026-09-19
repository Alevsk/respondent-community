// Fire icon: stylized flame silhouette
// For active fire detection, thermal hotspots
// Uses filled shapes only for crisp rendering at all scales

const SIZE = 32;
const CX = SIZE / 2;

export function drawFireIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer flame uses the layer color
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX, 2);
  ctx.bezierCurveTo(CX + 3, 8, CX + 12, 13, CX + 11, 21);
  ctx.bezierCurveTo(CX + 10, 27, CX + 5, 30, CX, 30);
  ctx.bezierCurveTo(CX - 5, 30, CX - 10, 27, CX - 11, 21);
  ctx.bezierCurveTo(CX - 12, 13, CX - 3, 8, CX, 2);
  ctx.closePath();
  ctx.fill();

  // Inner cutout for flame detail
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX, 13);
  ctx.bezierCurveTo(CX + 2, 16, CX + 6, 19, CX + 5, 24);
  ctx.bezierCurveTo(CX + 4, 27, CX + 2, 29, CX, 29);
  ctx.bezierCurveTo(CX - 2, 29, CX - 4, 27, CX - 5, 24);
  ctx.bezierCurveTo(CX - 6, 19, CX - 2, 16, CX, 13);
  ctx.closePath();
  ctx.fill();
}
