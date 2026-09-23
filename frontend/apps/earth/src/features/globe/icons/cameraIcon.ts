// CCTV camera icon: a wedge-shaped housing on a wall mount, lens facing right.
// Pure fill-based so it stays legible at billboard sizes.

const SIZE = 32;

export function drawCameraIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Housing — a tapered body angled slightly downward, lens end on the right.
  ctx.beginPath();
  ctx.moveTo(7, 11);
  ctx.lineTo(24, 14);
  ctx.lineTo(24, 21);
  ctx.lineTo(7, 19);
  ctx.closePath();
  ctx.fill();

  // Lens barrel at the wide end.
  ctx.beginPath();
  ctx.moveTo(24, 15);
  ctx.lineTo(28, 16.5);
  ctx.lineTo(28, 19);
  ctx.lineTo(24, 20);
  ctx.closePath();
  ctx.fill();

  // Mount arm and wall plate on the left.
  ctx.beginPath();
  ctx.moveTo(9, 11);
  ctx.lineTo(11, 11);
  ctx.lineTo(11, 6);
  ctx.lineTo(9, 6);
  ctx.closePath();
  ctx.fill();

  ctx.beginPath();
  ctx.moveTo(4, 4);
  ctx.lineTo(16, 4);
  ctx.lineTo(16, 7);
  ctx.lineTo(4, 7);
  ctx.closePath();
  ctx.fill();

  // Sight line from the lens, hinting at the field of view.
  ctx.beginPath();
  ctx.arc(SIZE - 3, 26, 2, 0, Math.PI * 2);
  ctx.fill();
}
