const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawCrossIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const arm = 5; // half-width of each arm
  const ext = 14; // reach from center to tip

  ctx.fillStyle = color;

  // Vertical bar
  ctx.beginPath();
  ctx.moveTo(CX - arm, CY - ext);
  ctx.lineTo(CX + arm, CY - ext);
  ctx.lineTo(CX + arm, CY + ext);
  ctx.lineTo(CX - arm, CY + ext);
  ctx.closePath();
  ctx.fill();

  // Horizontal bar
  ctx.beginPath();
  ctx.moveTo(CX - ext, CY - arm);
  ctx.lineTo(CX + ext, CY - arm);
  ctx.lineTo(CX + ext, CY + arm);
  ctx.lineTo(CX - ext, CY + arm);
  ctx.closePath();
  ctx.fill();
}
