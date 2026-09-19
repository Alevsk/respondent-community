const SIZE = 32;
const CX = SIZE / 2;

export function drawTankIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Hull — top-down view, wide rectangle with chamfered front corners
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX - 10, 28); // rear left
  ctx.lineTo(CX - 10, 10); // side left
  ctx.lineTo(CX - 6, 6); // front-left chamfer
  ctx.lineTo(CX + 6, 6); // front-right chamfer
  ctx.lineTo(CX + 10, 10); // side right
  ctx.lineTo(CX + 10, 28); // rear right
  ctx.closePath();
  ctx.fill();

  // Hull cutout
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX - 7, 26);
  ctx.lineTo(CX - 7, 13);
  ctx.lineTo(CX - 4, 10);
  ctx.lineTo(CX + 4, 10);
  ctx.lineTo(CX + 7, 13);
  ctx.lineTo(CX + 7, 26);
  ctx.closePath();
  ctx.fill();

  // Gun barrel — thick bar pointing up from turret
  ctx.fillStyle = color;
  ctx.fillRect(CX - 1.5, 2, 3, 14);

  // Turret — filled circle centered on hull
  ctx.beginPath();
  ctx.arc(CX, 18, 5.5, 0, Math.PI * 2);
  ctx.fill();

  // Turret cutout
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.arc(CX, 18, 3, 0, Math.PI * 2);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, 18, 1.5, 0, Math.PI * 2);
  ctx.fill();
}
