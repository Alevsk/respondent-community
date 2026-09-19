// Radiation icon: filled circle with trefoil hazard symbol inside.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawRadiationIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 14, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout circle
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 11, 0, Math.PI * 2);
  ctx.fill();

  // Three filled wedge/pie sections arranged 120° apart
  ctx.fillStyle = color;
  const innerR = 3.5;
  const outerR = 10;
  const bladeArc = Math.PI / 3; // 60° wide blades

  for (let i = 0; i < 3; i++) {
    const angle = (i * 2 * Math.PI) / 3 - Math.PI / 2;
    const startAngle = angle - bladeArc / 2;
    const endAngle = angle + bladeArc / 2;

    ctx.beginPath();
    ctx.arc(CX, CY, outerR, startAngle, endAngle);
    ctx.arc(CX, CY, innerR, endAngle, startAngle, true);
    ctx.closePath();
    ctx.fill();
  }

  // Filled center circle
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
