// Hurricane — classic weather-map spiral: two thick curved arms
// spiraling out from a small filled eye in the center.

const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawHurricaneIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;
  ctx.strokeStyle = color;
  ctx.lineCap = 'round';

  // --- Spiral arms ---
  // Each arm is drawn as a thick stroked path tracing an Archimedean spiral.
  // Two arms offset by PI give the classic pinwheel/cyclone look.
  const startR = 3.5; // spiral begins just outside the eye
  const endR = 13; // spiral ends near the canvas edge
  const totalAngle = Math.PI * 1.4; // how far each arm winds (~250°)
  const steps = 60;

  ctx.lineWidth = 3.8;

  for (let arm = 0; arm < 2; arm++) {
    const offsetAngle = arm * Math.PI; // second arm is opposite
    ctx.beginPath();
    for (let i = 0; i <= steps; i++) {
      const t = i / steps;
      const angle = offsetAngle + t * totalAngle;
      const r = startR + (endR - startR) * t;
      const x = CX + Math.cos(angle) * r;
      const y = CY + Math.sin(angle) * r;
      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.stroke();
  }

  // --- Central eye (filled circle) ---
  ctx.beginPath();
  ctx.arc(CX, CY, 3, 0, Math.PI * 2);
  ctx.fill();
}
