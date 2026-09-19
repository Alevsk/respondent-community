const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawPentagonIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const drawPent = (r: number) => {
    ctx.beginPath();
    for (let i = 0; i < 5; i++) {
      const angle = -Math.PI / 2 + (i * Math.PI * 2) / 5;
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
  drawPent(14);

  ctx.fillStyle = '#000000';
  drawPent(9);

  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
