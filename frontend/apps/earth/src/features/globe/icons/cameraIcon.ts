// CCTV camera: a bullet-body camera on a wall mount, lens to the right, with a
// view cone spreading from it.
//
// Drawn as a few large filled shapes rather than fine detail — these render as
// ~16px billboards among hundreds of siblings, where thin strokes turn to mush.
// The view cone is what makes it read as a camera rather than a box at a glance.

const SIZE = 32;

export function drawCameraIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // View cone, spreading down-right from the lens. Drawn first so the body
  // sits on top of it.
  ctx.globalAlpha = 0.35;
  ctx.beginPath();
  ctx.moveTo(25, 15);
  ctx.lineTo(SIZE, 27);
  ctx.lineTo(14, 30);
  ctx.closePath();
  ctx.fill();
  ctx.globalAlpha = 1;

  // Wall plate and mount arm.
  ctx.beginPath();
  ctx.rect(2, 4, 4, 11);
  ctx.fill();
  ctx.beginPath();
  ctx.rect(6, 8, 4, 3);
  ctx.fill();

  // Camera body: a chunky tapered bullet.
  ctx.beginPath();
  ctx.moveTo(9, 5);
  ctx.lineTo(23, 8);
  ctx.lineTo(23, 16);
  ctx.lineTo(9, 15);
  ctx.closePath();
  ctx.fill();

  // Lens hood at the wide end.
  ctx.beginPath();
  ctx.moveTo(23, 8.5);
  ctx.lineTo(28, 10.5);
  ctx.lineTo(28, 15);
  ctx.lineTo(23, 16);
  ctx.closePath();
  ctx.fill();
}
