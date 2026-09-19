// Flight Alt B — sleek jet silhouette, very pointy nose, delta wings.

const SIZE = 32;
const CX = SIZE / 2;

export function drawFlightAltBIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Entire jet as a single filled path — pointy nose, delta wings, tail
  ctx.beginPath();

  // Pointy nose
  ctx.moveTo(CX, 1);

  // Right side of fuselage curving out
  ctx.quadraticCurveTo(CX + 2, 6, CX + 3, 12);

  // Right delta wing — sweeps out wide
  ctx.lineTo(CX + 14, 20);
  ctx.lineTo(CX + 12, 22);

  // Back to fuselage right side
  ctx.lineTo(CX + 3, 18);
  ctx.lineTo(CX + 3, 23);

  // Right tail fin
  ctx.lineTo(CX + 6, 28);
  ctx.lineTo(CX + 5, 30);

  // Tail bottom
  ctx.lineTo(CX, 27);

  // Left tail fin
  ctx.lineTo(CX - 5, 30);
  ctx.lineTo(CX - 6, 28);

  // Left fuselage
  ctx.lineTo(CX - 3, 23);
  ctx.lineTo(CX - 3, 18);

  // Left delta wing
  ctx.lineTo(CX - 12, 22);
  ctx.lineTo(CX - 14, 20);

  // Left side of fuselage curving back to nose
  ctx.lineTo(CX - 3, 12);
  ctx.quadraticCurveTo(CX - 2, 6, CX, 1);

  ctx.closePath();
  ctx.fill();
}
