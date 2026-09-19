const SIZE = 32;
const CX = SIZE / 2;
const CY = SIZE / 2;

export function drawHexagonIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const drawHex = (r: number) => {
    ctx.beginPath();
    for (let i = 0; i < 6; i++) {
      const angle = -Math.PI / 2 + (i * Math.PI * 2) / 6;
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
  drawHex(14);

  ctx.fillStyle = '#000000';
  drawHex(9);

  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, CY, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
