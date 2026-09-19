// Default icon: simple filled circle
// Fallback for unregistered layer types

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawDefaultIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 6, 0, Math.PI * 2);
  ctx.fill();
}
