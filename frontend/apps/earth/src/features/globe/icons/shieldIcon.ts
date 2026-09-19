const SIZE = 32;
const CX = SIZE / 2;

export function drawShieldIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer shield — flat top, straight sides, tapers to point at bottom
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(4, 4);
  ctx.lineTo(28, 4);
  ctx.lineTo(28, 18);
  ctx.lineTo(CX, 30);
  ctx.lineTo(4, 18);
  ctx.closePath();
  ctx.fill();

  // Inner cutout — inset ~3.5px to create outlined ring feel
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(7.5, 7.5);
  ctx.lineTo(24.5, 7.5);
  ctx.lineTo(24.5, 17);
  ctx.lineTo(CX, 26.5);
  ctx.lineTo(7.5, 17);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, 15, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
