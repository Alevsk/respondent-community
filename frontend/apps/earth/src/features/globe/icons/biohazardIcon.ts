const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawBiohazardIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Three annular blade sectors at 120° — biohazard trefoil geometry
  ctx.fillStyle = color;
  const outerR = 13;
  const innerR = 5.5;
  const bladeArc = (Math.PI / 3) * 0.85;

  for (let i = 0; i < 3; i++) {
    const angle = (i * 2 * Math.PI) / 3 - Math.PI / 2;
    ctx.beginPath();
    ctx.arc(CX, CY, outerR, angle - bladeArc / 2, angle + bladeArc / 2);
    ctx.arc(CX, CY, innerR, angle + bladeArc / 2, angle - bladeArc / 2, true);
    ctx.closePath();
    ctx.fill();
  }

  // Center ring
  ctx.beginPath();
  ctx.arc(CX, CY, 3.8, 0, Math.PI * 2);
  ctx.fill();

  // Center cutout
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 2.0, 0, Math.PI * 2);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 1.0, 0, Math.PI * 2);
  ctx.fill();
}
