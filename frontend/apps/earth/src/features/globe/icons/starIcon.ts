const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawStarIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const drawStar = (outerR: number, innerR: number) => {
    ctx.beginPath();
    for (let i = 0; i < 10; i++) {
      const angle = -Math.PI / 2 + (i * Math.PI) / 5;
      const r = i % 2 === 0 ? outerR : innerR;
      const x = CX + Math.cos(angle) * r;
      const y = CY + Math.sin(angle) * r;
      if (i === 0) {
        ctx.moveTo(x, y);
      } else {
        ctx.lineTo(x, y);
      }
    }
    ctx.closePath();
    ctx.fill();
  };

  ctx.fillStyle = color;
  drawStar(13, 5.5);

  ctx.fillStyle = '#000000';
  drawStar(8, 3.5);

  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2, 0, Math.PI * 2);
  ctx.fill();
}
