// Explosion — jagged starburst inside a filled circle ring.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawExplosionIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 13, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout circle
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, 10, 0, Math.PI * 2);
  ctx.fill();

  // Jagged starburst inside the ring
  ctx.fillStyle = color;
  const SPIKES = 10;
  const outerR = 9;
  const innerR = 4.5;
  const angleStep = (Math.PI * 2) / SPIKES;
  const offset = -Math.PI / 2;

  ctx.beginPath();
  for (let i = 0; i < SPIKES; i++) {
    const outerAngle = offset + i * angleStep;
    const innerAngle = offset + (i + 0.5) * angleStep;
    ctx.lineTo(CX + Math.cos(outerAngle) * outerR, CY + Math.sin(outerAngle) * outerR);
    ctx.lineTo(CX + Math.cos(innerAngle) * innerR, CY + Math.sin(innerAngle) * innerR);
  }
  ctx.closePath();
  ctx.fill();
}
