// ISS — simplified top-down: central truss with four bold solar panels.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawIssIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Main truss — horizontal backbone
  ctx.fillRect(2, CY - 1.5, 28, 3);

  // Hab module — rounded vertical block at center
  ctx.beginPath();
  ctx.arc(CX, CY, 4, 0, Math.PI * 2);
  ctx.fill();

  // Four solar panels — bold rectangles, symmetric
  // Top-left
  ctx.fillRect(2, CY - 13, 10, 10);
  // Top-right
  ctx.fillRect(20, CY - 13, 10, 10);
  // Bottom-left
  ctx.fillRect(2, CY + 3, 10, 10);
  // Bottom-right
  ctx.fillRect(20, CY + 3, 10, 10);

  // Black cutouts — single cross line per panel
  ctx.fillStyle = '#000000';

  // Top-left
  ctx.fillRect(7, CY - 12, 1, 8);
  ctx.fillRect(3, CY - 8, 8, 1);

  // Top-right
  ctx.fillRect(25, CY - 12, 1, 8);
  ctx.fillRect(21, CY - 8, 8, 1);

  // Bottom-left
  ctx.fillRect(7, CY + 4, 1, 8);
  ctx.fillRect(3, CY + 8, 8, 1);

  // Bottom-right
  ctx.fillRect(25, CY + 4, 1, 8);
  ctx.fillRect(21, CY + 8, 8, 1);

  // Hab module cutout
  ctx.beginPath();
  ctx.arc(CX, CY, 2, 0, Math.PI * 2);
  ctx.fill();

  // Center dot
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 1, 0, Math.PI * 2);
  ctx.fill();
}
