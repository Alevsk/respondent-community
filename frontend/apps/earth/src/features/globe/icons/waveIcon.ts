// Wave — filled circle with three bold wavy bars inside.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawWaveIcon(ctx: CanvasRenderingContext2D, color: string): void {
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

  // Three small wavy bars centered inside the inner circle
  ctx.fillStyle = color;
  const offsets = [-3.5, 0, 3.5];
  for (const yOff of offsets) {
    const y = CY + yOff;
    ctx.beginPath();
    ctx.moveTo(CX - 5, y);
    ctx.quadraticCurveTo(CX - 2.5, y - 2, CX, y);
    ctx.quadraticCurveTo(CX + 2.5, y + 2, CX + 5, y);
    ctx.lineTo(CX + 5, y + 1.5);
    ctx.quadraticCurveTo(CX + 2.5, y + 3.5, CX, y + 1.5);
    ctx.quadraticCurveTo(CX - 2.5, y - 0.5, CX - 5, y + 1.5);
    ctx.closePath();
    ctx.fill();
  }
}
