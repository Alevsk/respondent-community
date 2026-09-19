// Location marker — inverted pin: arc top tapering to a point at the bottom

const SIZE = 32;
const CX = SIZE / 2;

export function drawMarkerIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer pin shape — semicircle head + taper to point
  const headY = 13;
  const headR = 10;
  ctx.beginPath();
  ctx.arc(CX, headY, headR, Math.PI, 0);
  ctx.lineTo(CX, 30);
  ctx.closePath();
  ctx.fill();

  // Inner black cutout — hollow ring on the head
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, headY, 6, 0, Math.PI * 2);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, headY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
