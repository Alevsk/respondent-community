const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawDroneIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // 4 diagonal arms — thin strokes from center to rotor endpoints
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.globalAlpha = 1.0;
  const armLen = 9;
  const diag = armLen / Math.SQRT2;
  const endpoints = [
    [CX - diag, CY - diag],
    [CX + diag, CY - diag],
    [CX + diag, CY + diag],
    [CX - diag, CY + diag],
  ] as const;

  for (const [ex, ey] of endpoints) {
    ctx.beginPath();
    ctx.moveTo(CX, CY);
    ctx.lineTo(ex, ey);
    ctx.stroke();
  }

  // Rotor dots — filled circle at each endpoint
  ctx.fillStyle = color;
  for (const [ex, ey] of endpoints) {
    ctx.beginPath();
    ctx.arc(ex, ey, 3.5, 0, Math.PI * 2);
    ctx.fill();
  }

  // Rotor cutouts
  ctx.fillStyle = '#000000';
  for (const [ex, ey] of endpoints) {
    ctx.beginPath();
    ctx.arc(ex, ey, 1.5, 0, Math.PI * 2);
    ctx.fill();
  }

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
