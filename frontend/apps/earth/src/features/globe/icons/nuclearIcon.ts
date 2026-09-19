// Nuclear — filled mushroom cloud silhouette: billowing cap, tapered stem, ground spread.

const SIZE = 32;
const CX = SIZE / 2;

export function drawNuclearIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // --- Cloud cap (billowing top) using overlapping filled circles ---
  // Main central dome
  ctx.beginPath();
  ctx.arc(CX, 9, 7, 0, Math.PI * 2);
  ctx.fill();

  // Left billow
  ctx.beginPath();
  ctx.arc(CX - 6, 11, 5.5, 0, Math.PI * 2);
  ctx.fill();

  // Right billow
  ctx.beginPath();
  ctx.arc(CX + 6, 11, 5.5, 0, Math.PI * 2);
  ctx.fill();

  // Upper-left accent
  ctx.beginPath();
  ctx.arc(CX - 3, 7, 4.5, 0, Math.PI * 2);
  ctx.fill();

  // Upper-right accent
  ctx.beginPath();
  ctx.arc(CX + 3, 7, 4.5, 0, Math.PI * 2);
  ctx.fill();

  // --- Stem (narrower column below the cap) ---
  ctx.beginPath();
  ctx.moveTo(CX - 5, 14);
  ctx.quadraticCurveTo(CX - 3, 20, CX - 2, 28);
  ctx.lineTo(CX + 2, 28);
  ctx.quadraticCurveTo(CX + 3, 20, CX + 5, 14);
  ctx.closePath();
  ctx.fill();

  // --- Ground spread (base ring at bottom of stem) ---
  ctx.beginPath();
  ctx.ellipse(CX, 28, 5, 2, 0, 0, Math.PI * 2);
  ctx.fill();
}
