// Cell/communication tower — triangular lattice wider at base, narrow at top,
// antenna mast on top with signal arcs. Filled with `color`, black for structural cutouts.

const SIZE = 32;
const CX = SIZE / 2;

export function drawTowerIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;
  ctx.strokeStyle = color;

  // --- Tower body: solid trapezoid/triangle from y=12 down to y=30 ---
  ctx.beginPath();
  ctx.moveTo(CX - 2, 12); // narrow top of tower body
  ctx.lineTo(CX + 2, 12);
  ctx.lineTo(CX + 9, 30); // wide base right
  ctx.lineTo(CX - 9, 30); // wide base left
  ctx.closePath();
  ctx.fill();

  // --- Black lattice cutouts: two triangular windows stacked vertically ---
  ctx.fillStyle = '#000000';

  // Upper cutout (smaller)
  ctx.beginPath();
  ctx.moveTo(CX, 14.5);
  ctx.lineTo(CX + 3.5, 20);
  ctx.lineTo(CX - 3.5, 20);
  ctx.closePath();
  ctx.fill();

  // Lower cutout (larger)
  ctx.beginPath();
  ctx.moveTo(CX, 21.5);
  ctx.lineTo(CX + 6, 28.5);
  ctx.lineTo(CX - 6, 28.5);
  ctx.closePath();
  ctx.fill();

  // --- Antenna mast: thin vertical line extending from tower top ---
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.rect(CX - 1, 3, 2, 10);
  ctx.fill();

  // Small antenna tip / bulb
  ctx.beginPath();
  ctx.arc(CX, 3, 1.5, 0, Math.PI * 2);
  ctx.fill();

  // --- Signal arcs emanating from the antenna tip ---
  ctx.lineWidth = 1.4;
  ctx.lineCap = 'round';

  // Inner arc (left)
  ctx.beginPath();
  ctx.arc(CX, 4, 4, -Math.PI * 0.85, -Math.PI * 0.15);
  ctx.stroke();

  // Outer arc (left-right, wider)
  ctx.beginPath();
  ctx.arc(CX, 4, 7, -Math.PI * 0.8, -Math.PI * 0.2);
  ctx.stroke();

  // --- Horizontal cross-beam at the tower-antenna junction ---
  ctx.beginPath();
  ctx.rect(CX - 4, 11.5, 8, 1.5);
  ctx.fill();

  // --- Base feet: two small rectangles anchoring the tower ---
  ctx.beginPath();
  ctx.rect(CX - 10, 29, 4, 2);
  ctx.fill();
  ctx.beginPath();
  ctx.rect(CX + 6, 29, 4, 2);
  ctx.fill();
}
