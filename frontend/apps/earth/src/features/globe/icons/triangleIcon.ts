const SIZE = 32;
const CX = SIZE / 2;

export function drawTriangleIcon(ctx: CanvasRenderingContext2D, color: string): void {
  const drawTri = (apex: number, base: number, halfBase: number) => {
    ctx.beginPath();
    ctx.moveTo(CX, apex);
    ctx.lineTo(CX + halfBase, base);
    ctx.lineTo(CX - halfBase, base);
    ctx.closePath();
    ctx.fill();
  };

  ctx.fillStyle = color;
  drawTri(2, 29, 14);

  ctx.fillStyle = '#000000';
  drawTri(9, 26, 9);

  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(CX, 20, 2.5, 0, Math.PI * 2);
  ctx.fill();
}
