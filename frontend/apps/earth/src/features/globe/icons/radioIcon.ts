// Radio / WiFi signal icon: three concentric arc bands radiating from a center dot.
// Pure fill-based — each band is an outer arc + inner arc forming a thick crescent.

const SIZE = 32;
const CX = SIZE / 2;

export function drawRadioIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const originY = 26; // signal radiates upward from bottom-center

  // Three filled arc bands — outer arc + inner arc closed into crescent shapes
  // Band 3 (outermost)
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, originY, 22, Math.PI * 1.25, Math.PI * 1.75, false);
  ctx.arc(CX, originY, 18, Math.PI * 1.75, Math.PI * 1.25, true);
  ctx.closePath();
  ctx.fill();

  // Band 2 (middle)
  ctx.beginPath();
  ctx.arc(CX, originY, 15, Math.PI * 1.25, Math.PI * 1.75, false);
  ctx.arc(CX, originY, 11, Math.PI * 1.75, Math.PI * 1.25, true);
  ctx.closePath();
  ctx.fill();

  // Band 1 (innermost)
  ctx.beginPath();
  ctx.arc(CX, originY, 8, Math.PI * 1.25, Math.PI * 1.75, false);
  ctx.arc(CX, originY, 4.5, Math.PI * 1.75, Math.PI * 1.25, true);
  ctx.closePath();
  ctx.fill();

  // Center dot at the signal origin
  ctx.beginPath();
  ctx.arc(CX, originY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
