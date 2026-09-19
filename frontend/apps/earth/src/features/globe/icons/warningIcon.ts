// Warning icon: filled triangle with exclamation mark cutout
// Same filled-shape approach as flightIcon/diamondIcon for crisp rendering

const SIZE = 32;
const CX = SIZE / 2;

export function drawWarningIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Outer triangle — filled solid
  ctx.beginPath();
  ctx.moveTo(CX, 2);
  ctx.lineTo(CX + 14, 28);
  ctx.lineTo(CX - 14, 28);
  ctx.closePath();
  ctx.fill();

  // Inner triangle cutout — creates the outlined look
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX, 8);
  ctx.lineTo(CX + 9.5, 25);
  ctx.lineTo(CX - 9.5, 25);
  ctx.closePath();
  ctx.fill();

  // Exclamation stem — thick filled rectangle
  ctx.fillStyle = color;
  const stemW = 2.5;
  ctx.beginPath();
  ctx.moveTo(CX - stemW / 2, 12);
  ctx.lineTo(CX + stemW / 2, 12);
  ctx.lineTo(CX + stemW / 2 - 0.3, 20);
  ctx.lineTo(CX - stemW / 2 + 0.3, 20);
  ctx.closePath();
  ctx.fill();

  // Exclamation dot
  ctx.beginPath();
  ctx.arc(CX, 23, 1.8, 0, Math.PI * 2);
  ctx.fill();
}
