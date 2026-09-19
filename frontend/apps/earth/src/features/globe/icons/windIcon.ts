// Wind — filled square with black wind swooshes cut out.

const SIZE = 32;

export function drawWindIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Filled square background
  ctx.fillStyle = color;
  ctx.fillRect(2, 2, SIZE - 4, SIZE - 4);

  // Black wind swooshes cut into the square
  ctx.strokeStyle = '#000000';
  ctx.lineCap = 'round';
  ctx.lineWidth = 3;

  // Top swoosh — curves right and curls upward
  ctx.beginPath();
  ctx.moveTo(6, 10);
  ctx.quadraticCurveTo(16, 10, 22, 8);
  ctx.quadraticCurveTo(27, 6, 27, 4);
  ctx.stroke();

  // Middle swoosh — longest, curves right and curls upward
  ctx.beginPath();
  ctx.moveTo(5, 17);
  ctx.quadraticCurveTo(15, 17, 22, 16);
  ctx.quadraticCurveTo(29, 14, 29, 11);
  ctx.stroke();

  // Bottom swoosh — curves right and curls downward
  ctx.beginPath();
  ctx.moveTo(6, 23);
  ctx.quadraticCurveTo(16, 23, 22, 25);
  ctx.quadraticCurveTo(27, 27, 27, 29);
  ctx.stroke();
}
