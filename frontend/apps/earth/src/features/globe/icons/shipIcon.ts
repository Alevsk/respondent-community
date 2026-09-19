// Ship — top-down vessel: pointed bow, wide hull, flat stern.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawShipIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer hull — pointed bow, wide body, flat stern
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX, 3); // bow tip
  ctx.lineTo(CX + 8, 12); // starboard bow shoulder
  ctx.lineTo(CX + 8, 28); // starboard stern
  ctx.lineTo(CX - 8, 28); // port stern
  ctx.lineTo(CX - 8, 12); // port bow shoulder
  ctx.closePath();
  ctx.fill();

  // Black cutout — inset hull
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.moveTo(CX, 8);
  ctx.lineTo(CX + 5, 14);
  ctx.lineTo(CX + 5, 26);
  ctx.lineTo(CX - 5, 26);
  ctx.lineTo(CX - 5, 14);
  ctx.closePath();
  ctx.fill();

  // Superstructure — small filled rectangle mid-ship
  ctx.fillStyle = color;
  ctx.fillRect(CX - 3, 17, 6, 6);

  // Center dot
  ctx.beginPath();
  ctx.arc(CX, CY + 4, 1.5, 0, Math.PI * 2);
  ctx.fill();
}
