// Volcano — bold mountain peak silhouette with crater and eruption detail.

const SIZE = 32;
const CX = SIZE / 2;

export function drawVolcanoIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Mountain silhouette — wide triangle
  ctx.beginPath();
  ctx.moveTo(CX, 4); // peak
  ctx.lineTo(CX + 14, 28); // right base
  ctx.lineTo(CX - 14, 28); // left base
  ctx.closePath();
  ctx.fill();

  // Black cutout — inset triangle
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX, 10);
  ctx.lineTo(CX + 9, 26);
  ctx.lineTo(CX - 9, 26);
  ctx.closePath();
  ctx.fill();

  // Crater notch at the top
  ctx.beginPath();
  ctx.moveTo(CX - 3, 8);
  ctx.lineTo(CX + 3, 8);
  ctx.lineTo(CX + 2, 5);
  ctx.lineTo(CX - 2, 5);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, 18, 2, 0, Math.PI * 2);
  ctx.fill();
}
