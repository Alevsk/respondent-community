// Radar — sonar/radar sweep display: circle with crosshairs and a sweep wedge.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawRadarIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const R = 13;

  // Outer filled circle
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, R, 0, Math.PI * 2);
  ctx.fill();

  // Black cutout circle — creates the ring
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, CY, R - 2.5, 0, Math.PI * 2);
  ctx.fill();

  // Crosshair lines — thin colored cross inside the ring
  ctx.fillStyle = color;
  ctx.fillRect(CX - 0.5, CY - R + 2.5, 1, (R - 2.5) * 2); // vertical
  ctx.fillRect(CX - R + 2.5, CY - 0.5, (R - 2.5) * 2, 1); // horizontal

  // Sweep wedge — filled pie slice pointing upper-right (~25° wide)
  ctx.beginPath();
  ctx.moveTo(CX, CY);
  ctx.arc(CX, CY, R - 3, -Math.PI * 0.42, -Math.PI * 0.28);
  ctx.closePath();
  ctx.fill();

  // Center dot
  ctx.beginPath();
  ctx.arc(CX, CY, 2, 0, Math.PI * 2);
  ctx.fill();
}
