// Diamond / rhombus — clean radar-style tracked asset marker

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawDiamondIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Diamond shape — 4-point rhombus
  const R = 11; // half-diagonal
  ctx.beginPath();
  ctx.moveTo(CX, CY - R); // top
  ctx.lineTo(CX + R, CY); // right
  ctx.lineTo(CX, CY + R); // bottom
  ctx.lineTo(CX - R, CY); // left
  ctx.closePath();
  ctx.fill();

  // Inner cutout — smaller diamond to create an outlined feel
  ctx.fillStyle = '#000000';
  const IR = 6; // inner half-diagonal
  ctx.beginPath();
  ctx.moveTo(CX, CY - IR);
  ctx.lineTo(CX + IR, CY);
  ctx.lineTo(CX, CY + IR);
  ctx.lineTo(CX - IR, CY);
  ctx.closePath();
  ctx.fill();

  // Center dot — solid core
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
