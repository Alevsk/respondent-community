// Flight Alt C — bulbous rounded body with sharp nose, stubby wings.

const SIZE = 32;
const CX = SIZE / 2;

export function drawFlightAltCIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Fuselage — sharp nose into a wide rounded body
  ctx.beginPath();
  ctx.moveTo(CX, 1); // sharp nose tip
  ctx.quadraticCurveTo(CX + 5, 10, CX + 4, 16); // right side widens
  ctx.lineTo(CX + 4, 22);
  ctx.quadraticCurveTo(CX + 4, 27, CX, 28); // rounded tail
  ctx.quadraticCurveTo(CX - 4, 27, CX - 4, 22);
  ctx.lineTo(CX - 4, 16);
  ctx.quadraticCurveTo(CX - 5, 10, CX, 1); // left side widens
  ctx.closePath();
  ctx.fill();

  // Main wings — wide and rounded
  ctx.beginPath();
  ctx.moveTo(CX + 2, 13);
  ctx.quadraticCurveTo(CX + 8, 15, CX + 13, 18);
  ctx.quadraticCurveTo(CX + 14, 19, CX + 12, 20);
  ctx.quadraticCurveTo(CX + 7, 18, CX + 2, 17);
  ctx.lineTo(CX - 2, 17);
  ctx.quadraticCurveTo(CX - 7, 18, CX - 12, 20);
  ctx.quadraticCurveTo(CX - 14, 19, CX - 13, 18);
  ctx.quadraticCurveTo(CX - 8, 15, CX - 2, 13);
  ctx.closePath();
  ctx.fill();

  // Tail fins — small rounded
  ctx.beginPath();
  ctx.moveTo(CX + 2, 24);
  ctx.quadraticCurveTo(CX + 5, 26, CX + 7, 29);
  ctx.quadraticCurveTo(CX + 6, 30, CX + 5, 29);
  ctx.lineTo(CX, 27);
  ctx.lineTo(CX - 5, 29);
  ctx.quadraticCurveTo(CX - 6, 30, CX - 7, 29);
  ctx.quadraticCurveTo(CX - 5, 26, CX - 2, 24);
  ctx.closePath();
  ctx.fill();
}
