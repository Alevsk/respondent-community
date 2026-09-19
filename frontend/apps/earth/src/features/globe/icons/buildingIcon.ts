const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawBuildingIcon(ctx: CanvasRenderingContext2D, color: string): void {
  // Outer shape — tall rectangle with a triangle roof peak fused on top
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(CX, 3); // roof apex
  ctx.lineTo(CX + 9, 10); // roof right shoulder
  ctx.lineTo(CX + 9, 29); // base right
  ctx.lineTo(CX - 9, 29); // base left
  ctx.lineTo(CX - 9, 10); // roof left shoulder
  ctx.closePath();
  ctx.fill();

  // Black cutout — inset rectangle (no roof)
  ctx.fillStyle = '#000000';
  ctx.beginPath();
  ctx.rect(CX - 5, 14, 10, 13);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY + 2, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
