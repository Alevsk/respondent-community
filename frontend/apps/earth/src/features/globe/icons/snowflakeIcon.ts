// Snowflake — six-armed asterisk inside a filled circle ring.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawSnowflakeIcon(ctx: CanvasRenderingContext2D, color: string): void {
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

  // Six bold arms radiating from center
  ctx.fillStyle = color;
  const armLength = 9;
  const armWidth = 2.5;

  for (let i = 0; i < 6; i++) {
    const angle = (Math.PI / 3) * i;
    ctx.save();
    ctx.translate(CX, CY);
    ctx.rotate(angle);
    ctx.fillRect(-armWidth / 2, -armLength, armWidth, armLength * 2);
    // Small crossbars near the tips
    ctx.fillRect(-4, -armLength + 2, 8, 2);
    ctx.fillRect(-4, armLength - 4, 8, 2);
    ctx.restore();
  }

  // Center dot
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
