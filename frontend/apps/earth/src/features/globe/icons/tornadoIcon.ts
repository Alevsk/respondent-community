// Tornado — classic funnel silhouette, wide cloud top narrowing to a ground point,
// with subtle curved bands suggesting rotation.

export function drawTornadoIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;
  ctx.strokeStyle = color;

  // --- Solid funnel body (bezier curves giving organic funnel shape) ---
  ctx.beginPath();
  // Start at top-left of the wide cloud cap
  ctx.moveTo(3, 6);
  // Flat-ish cloud top with a slight upward bulge
  ctx.quadraticCurveTo(16, 2, 29, 6);
  // Right side curves inward as funnel narrows
  ctx.bezierCurveTo(27, 12, 23, 18, 20, 22);
  // Continue narrowing toward ground contact point
  ctx.bezierCurveTo(19, 24, 17.5, 27, 17, 30);
  // Narrow bottom tip
  ctx.lineTo(15, 30);
  // Left side mirrors — back up the funnel
  ctx.bezierCurveTo(14.5, 27, 13, 24, 12, 22);
  ctx.bezierCurveTo(9, 18, 5, 12, 3, 6);
  ctx.closePath();
  ctx.fill();

  // --- Rotation bands (subtle curved lines across the funnel) ---
  ctx.strokeStyle = '#000000';
  ctx.lineCap = 'round';

  // Band 1 — near top, wide
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.moveTo(6, 10);
  ctx.quadraticCurveTo(16, 8, 26, 10);
  ctx.stroke();

  // Band 2 — mid-upper
  ctx.lineWidth = 1.3;
  ctx.beginPath();
  ctx.moveTo(9, 15);
  ctx.quadraticCurveTo(16, 13, 23, 15);
  ctx.stroke();

  // Band 3 — mid-lower, narrower
  ctx.lineWidth = 1.2;
  ctx.beginPath();
  ctx.moveTo(12, 20);
  ctx.quadraticCurveTo(16, 18.5, 20, 20);
  ctx.stroke();

  // Band 4 — near tip
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(14, 25);
  ctx.quadraticCurveTo(16, 24, 18, 25);
  ctx.stroke();
}
