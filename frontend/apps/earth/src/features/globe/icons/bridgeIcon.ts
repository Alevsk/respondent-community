// Bridge — filled arch/gate silhouette: rectangle with a semicircular arch cut out of
// the bottom half. Pure fill-based. Black cutout creates outline. Center dot at top.

const SIZE = 32;
const CX = SIZE / 2;

export function drawBridgeIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer arch block — full rectangle representing the arch structure
  const left = 5;
  const right = 27;
  const top = 8;
  const bottom = 27;
  const archR = 8.5;

  ctx.beginPath();
  ctx.rect(left, top, right - left, bottom - top);
  ctx.fill();

  // Carve out the semicircular archway from the bottom
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, bottom, archR, Math.PI, Math.PI * 2);
  ctx.rect(CX - archR, bottom - archR, archR * 2, archR);
  ctx.fill();

  // Inner black cutout — inset rectangle to create thick border effect
  ctx.beginPath();
  ctx.rect(left + 2.5, top + 2.5, right - left - 5, bottom - top - 5);
  ctx.fill();

  // Restore the top portion of the arch (outer border only visible at top)
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.rect(left + 2.5, top + 2.5, right - left - 5, 5);
  ctx.fill();

  // Center dot at visual top-center
  ctx.beginPath();
  ctx.arc(CX, top + 5, 2, 0, Math.PI * 2);
  ctx.fill();
}
