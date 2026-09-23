// CCTV camera: a bullet-body camera on a wall bracket, lens to the right, with
// a view cone spreading from it.
//
// Drawn as a few large filled shapes rather than fine detail — these render as
// ~16px billboards among hundreds of siblings, where thin strokes and small
// separate parts turn to mush. The bracket is one solid form for that reason,
// and the view cone is what makes the silhouette read as a camera rather than
// a box at a glance.

const SIZE = 32;

export function drawCameraIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // View cone, spreading down-right from the lens. Drawn first so the body
  // sits on top of it. Kept fairly opaque so it survives at billboard size.
  ctx.globalAlpha = 0.45;
  ctx.beginPath();
  ctx.moveTo(26, 19);
  ctx.lineTo(SIZE, 29);
  ctx.lineTo(17, 31);
  ctx.closePath();
  ctx.fill();
  ctx.globalAlpha = 1;

  // Wall bracket: one solid L, so it cannot break into specks when scaled down.
  ctx.beginPath();
  ctx.moveTo(2, 3);
  ctx.lineTo(7, 3);
  ctx.lineTo(7, 12);
  ctx.lineTo(11, 12);
  ctx.lineTo(11, 16);
  ctx.lineTo(2, 16);
  ctx.closePath();
  ctx.fill();

  // Camera body: a chunky tapered bullet filling most of the width.
  ctx.beginPath();
  ctx.moveTo(9, 8);
  ctx.lineTo(25, 11);
  ctx.lineTo(25, 20);
  ctx.lineTo(9, 18);
  ctx.closePath();
  ctx.fill();

  // Lens hood at the wide end.
  ctx.beginPath();
  ctx.moveTo(25, 11.5);
  ctx.lineTo(30, 13.5);
  ctx.lineTo(30, 19);
  ctx.lineTo(25, 20);
  ctx.closePath();
  ctx.fill();
}
