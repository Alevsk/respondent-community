// Observatory dome — semicircular dome with a vertical slit/opening,
// sitting on a rectangular base. Filled geometric silhouette, recognizable
// at small sizes. Uses `color` for fills, `#000000` for cutout details.

const SIZE = 32;
const CX = SIZE / 2;

export function drawTelescopeIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.save();
  ctx.fillStyle = color;

  // --- Base platform (wide rectangle at the bottom) ---
  const baseTop = 20;
  const baseBottom = 27;
  const baseLeft = 5;
  const baseRight = 27;
  ctx.fillRect(baseLeft, baseTop, baseRight - baseLeft, baseBottom - baseTop);

  // --- Dome (semicircle on top of the base) ---
  const domeRadius = 11;
  const domeCY = baseTop; // dome sits right on top of the base
  ctx.beginPath();
  ctx.arc(CX, domeCY, domeRadius, Math.PI, 0); // upper semicircle
  ctx.closePath();
  ctx.fill();

  // --- Dome slit (vertical opening cut into the dome) ---
  ctx.fillStyle = '#000000';
  const slitWidth = 3;
  const slitLeft = CX - slitWidth / 2;
  // The slit runs from top of dome down to the base top
  const slitTop = domeCY - domeRadius + 1.5;
  const slitBottom = domeCY;
  ctx.fillRect(slitLeft, slitTop, slitWidth, slitBottom - slitTop);

  // --- Telescope tube poking out of the slit (angled upward-right) ---
  ctx.fillStyle = color;
  ctx.save();
  ctx.translate(CX, domeCY - domeRadius * 0.45);
  ctx.rotate(-Math.PI / 5); // angled upward to the right

  const tubeLength = 9;
  const tubeThick = 2.2;
  ctx.fillRect(0, -tubeThick / 2, tubeLength, tubeThick);

  // Small finder scope on top of tube
  ctx.fillRect(2, -tubeThick / 2 - 1.8, 4, 1.4);

  ctx.restore();

  // --- Base detail line (horizontal groove in the base) ---
  ctx.fillStyle = '#000000';
  ctx.fillRect(baseLeft + 1.5, baseTop + 3.5, baseRight - baseLeft - 3, 1.2);

  // --- Small feet / ground line under the base ---
  ctx.fillStyle = color;
  ctx.fillRect(baseLeft + 1, baseBottom, 4, 1.5);
  ctx.fillRect(baseRight - 5, baseBottom, 4, 1.5);

  ctx.restore();
}
