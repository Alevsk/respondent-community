const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawCrosshairIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer ring
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 11, 0, Math.PI * 2);
  ctx.fill();

  // Cardinal lines — 4 segments, each starting ~3px from center, ending at ring inner edge
  ctx.fillStyle = color;

  // North
  ctx.beginPath();
  ctx.moveTo(CX - 1, 3);
  ctx.lineTo(CX + 1, 3);
  ctx.lineTo(CX + 1, CY - 3);
  ctx.lineTo(CX - 1, CY - 3);
  ctx.closePath();
  ctx.fill();

  // South
  ctx.beginPath();
  ctx.moveTo(CX - 1, CY + 3);
  ctx.lineTo(CX + 1, CY + 3);
  ctx.lineTo(CX + 1, 29);
  ctx.lineTo(CX - 1, 29);
  ctx.closePath();
  ctx.fill();

  // East
  ctx.beginPath();
  ctx.moveTo(CX + 3, CY - 1);
  ctx.lineTo(29, CY - 1);
  ctx.lineTo(29, CY + 1);
  ctx.lineTo(CX + 3, CY + 1);
  ctx.closePath();
  ctx.fill();

  // West
  ctx.beginPath();
  ctx.moveTo(3, CY - 1);
  ctx.lineTo(CX - 3, CY - 1);
  ctx.lineTo(CX - 3, CY + 1);
  ctx.lineTo(3, CY + 1);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
