// Meteor — bold fireball with wide trailing streaks, falling diagonally.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawMeteorIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Fireball head position — lower-right quadrant
  const hx = CX + 4;
  const hy = CY + 4;

  // Wide trailing streak — single bold wedge from fireball to upper-left corner
  ctx.beginPath();
  ctx.moveTo(hx - 5, hy - 9); // top edge of wedge at fireball
  ctx.lineTo(1, 1); // converge at upper-left corner
  ctx.lineTo(hx - 9, hy - 5); // bottom edge of wedge at fireball
  ctx.closePath();
  ctx.fill();

  // Secondary trail — thinner, offset upward
  ctx.beginPath();
  ctx.moveTo(hx - 9, hy - 2);
  ctx.lineTo(1, 9);
  ctx.lineTo(3, 12);
  ctx.lineTo(hx - 7, hy + 1);
  ctx.closePath();
  ctx.fill();

  // Secondary trail — thinner, offset rightward
  ctx.beginPath();
  ctx.moveTo(hx - 2, hy - 9);
  ctx.lineTo(9, 1);
  ctx.lineTo(12, 3);
  ctx.lineTo(hx + 1, hy - 7);
  ctx.closePath();
  ctx.fill();

  // Fireball head — large filled circle (on top of trails)
  ctx.beginPath();
  ctx.arc(hx, hy, 10, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout ring
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(hx, hy, 7, 0, Math.PI * 2);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(hx, hy, 3, 0, Math.PI * 2);
  ctx.fill();
}
