// Skull — videogame-style skull with round dome cranium.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawSkullIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // --- Cranium — large rounded dome (semicircle top) ---
  ctx.beginPath();
  ctx.arc(CX, CY - 3, 12, Math.PI, 0); // wide dome
  ctx.lineTo(CX + 12, CY + 1);
  ctx.lineTo(CX - 12, CY + 1);
  ctx.closePath();
  ctx.fill();

  // --- Jaw (narrower, tapering down with rounded bottom) ---
  ctx.beginPath();
  ctx.moveTo(CX - 12, CY + 1);
  ctx.lineTo(CX - 9, CY + 5);
  ctx.quadraticCurveTo(CX - 8, CY + 10, CX - 4, CY + 10);
  ctx.lineTo(CX + 4, CY + 10);
  ctx.quadraticCurveTo(CX + 8, CY + 10, CX + 9, CY + 5);
  ctx.lineTo(CX + 12, CY + 1);
  ctx.closePath();
  ctx.fill();

  // --- Eye sockets (large round cutouts) ---
  ctx.fillStyle = '#000000';

  // Left eye
  ctx.beginPath();
  ctx.arc(CX - 5, CY - 4, 4, 0, Math.PI * 2);
  ctx.fill();

  // Right eye
  ctx.beginPath();
  ctx.arc(CX + 5, CY - 4, 4, 0, Math.PI * 2);
  ctx.fill();

  // --- Nose cavity (small inverted triangle) ---
  ctx.beginPath();
  ctx.moveTo(CX - 2, CY + 1);
  ctx.lineTo(CX + 2, CY + 1);
  ctx.lineTo(CX, CY + 4);
  ctx.closePath();
  ctx.fill();

  // --- Teeth (vertical black lines across the jaw) ---
  const teethY = CY + 6;
  const teethH = 4;
  const teethW = 1.5;
  const teethSpacing = 3.5;
  const teethCount = 5;
  const teethStartX = CX - ((teethCount - 1) * teethSpacing) / 2;

  for (let i = 0; i < teethCount; i++) {
    const x = teethStartX + i * teethSpacing;
    ctx.fillRect(x - teethW / 2, teethY, teethW, teethH);
  }
}
