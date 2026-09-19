// Top-down airplane silhouette: fuselage + swept wings + tail
// Drawn white, nose pointing up (north = 0 heading)

const SIZE = 32;
const CX = SIZE / 2;

export function drawFlightIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;
  ctx.beginPath();

  // Fuselage — elongated vertical shape
  ctx.moveTo(CX, 3); // nose
  ctx.lineTo(CX + 2, 8);
  ctx.lineTo(CX + 2, 22);
  ctx.lineTo(CX + 3, 27); // tail base right
  ctx.lineTo(CX - 3, 27); // tail base left
  ctx.lineTo(CX - 2, 22);
  ctx.lineTo(CX - 2, 8);
  ctx.closePath();
  ctx.fill();

  // Main wings — swept back
  ctx.beginPath();
  ctx.moveTo(CX, 12); // wing root top
  ctx.lineTo(CX + 13, 18); // right wingtip
  ctx.lineTo(CX + 12, 20);
  ctx.lineTo(CX, 15); // wing root bottom
  ctx.lineTo(CX - 12, 20); // left wingtip
  ctx.lineTo(CX - 13, 18);
  ctx.closePath();
  ctx.fill();

  // Tail wings — smaller swept
  ctx.beginPath();
  ctx.moveTo(CX, 24);
  ctx.lineTo(CX + 6, 28);
  ctx.lineTo(CX + 5, 29);
  ctx.lineTo(CX, 26);
  ctx.lineTo(CX - 5, 29);
  ctx.lineTo(CX - 6, 28);
  ctx.closePath();
  ctx.fill();
}
