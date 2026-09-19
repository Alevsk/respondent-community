// Flight Alt A — rounder fuselage with pointed nose, curved wings.

const SIZE = 32;
const CX = SIZE / 2;

export function drawFlightAltAIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Fuselage — pointed nose tapering into a rounded body
  ctx.beginPath();
  ctx.moveTo(CX, 1); // sharp nose tip
  ctx.quadraticCurveTo(CX + 4, 8, CX + 3, 14); // right curve of nose into body
  ctx.lineTo(CX + 3, 23);
  ctx.quadraticCurveTo(CX + 3, 27, CX, 28); // rounded tail
  ctx.quadraticCurveTo(CX - 3, 27, CX - 3, 23);
  ctx.lineTo(CX - 3, 14);
  ctx.quadraticCurveTo(CX - 4, 8, CX, 1); // left curve of nose
  ctx.closePath();
  ctx.fill();

  // Main wings — swept back with curved leading edge
  ctx.beginPath();
  ctx.moveTo(CX, 11);
  ctx.quadraticCurveTo(CX + 8, 14, CX + 14, 17); // right wing leading edge
  ctx.lineTo(CX + 13, 19);
  ctx.quadraticCurveTo(CX + 6, 17, CX, 15); // right wing trailing edge
  ctx.quadraticCurveTo(CX - 6, 17, CX - 13, 19); // left wing trailing edge
  ctx.lineTo(CX - 14, 17);
  ctx.quadraticCurveTo(CX - 8, 14, CX, 11); // left wing leading edge
  ctx.closePath();
  ctx.fill();

  // Tail fins — small swept
  ctx.beginPath();
  ctx.moveTo(CX, 24);
  ctx.lineTo(CX + 7, 28);
  ctx.lineTo(CX + 6, 30);
  ctx.lineTo(CX, 26);
  ctx.lineTo(CX - 6, 30);
  ctx.lineTo(CX - 7, 28);
  ctx.closePath();
  ctx.fill();
}
