// Rocket — sharp pointed nose cone with cylindrical body, fins, and exhaust.

const SIZE = 32;
const CX = SIZE / 2;

export function drawRocketIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Solid rocket silhouette as a single filled path
  ctx.beginPath();

  // Start at the pointy nose tip
  ctx.moveTo(CX, 2);

  // Right side of nose cone tapering out to body
  ctx.lineTo(CX + 5, 12);

  // Right body edge down to fin start
  ctx.lineTo(CX + 5, 22);

  // Right fin — juts out and comes back
  ctx.lineTo(CX + 11, 28);
  ctx.lineTo(CX + 5, 24);

  // Right side of exhaust nozzle
  ctx.lineTo(CX + 3, 24);
  ctx.lineTo(CX + 4, 30);

  // Exhaust nozzle bottom center
  ctx.lineTo(CX, 27);

  // Left side of exhaust nozzle
  ctx.lineTo(CX - 4, 30);
  ctx.lineTo(CX - 3, 24);

  // Left fin — juts out and comes back
  ctx.lineTo(CX - 5, 24);
  ctx.lineTo(CX - 11, 28);
  ctx.lineTo(CX - 5, 22);

  // Left body edge up to nose
  ctx.lineTo(CX - 5, 12);

  // Back to pointy nose tip
  ctx.closePath();
  ctx.fill();

  // Small window cutout
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, 15, 1.8, 0, Math.PI * 2);
  ctx.fill();
}
