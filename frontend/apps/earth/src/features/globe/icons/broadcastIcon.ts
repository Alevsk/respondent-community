// Broadcast station: a transmitter mast with waves radiating from its tip.
//
// Distinct from the `radio` shape on purpose. Four layers were drawing radio
// arcs — radiosondes, APRS, meshtastic and internet radio — which made a
// station you can listen to look identical to a weather balloon. A mast reads
// as a fixed transmitter; the arcs read as a signal in the air.

const SIZE = 32;
const CX = SIZE / 2;

export function drawBroadcastIcon(ctx: CanvasRenderingContext2D, color: string): void {
  ctx.fillStyle = color;

  // Mast: a narrow tapering tower from the base up to the emitter.
  ctx.beginPath();
  ctx.moveTo(CX - 1.6, 12);
  ctx.lineTo(CX + 1.6, 12);
  ctx.lineTo(CX + 4.5, 29);
  ctx.lineTo(CX - 4.5, 29);
  ctx.closePath();
  ctx.fill();

  // Base feet, so the mast reads as planted rather than floating.
  ctx.beginPath();
  ctx.rect(CX - 7.5, 28.5, 15, 2.5);
  ctx.fill();

  // Cross brace.
  ctx.beginPath();
  ctx.rect(CX - 3.2, 21, 6.4, 1.8);
  ctx.fill();

  // Emitter at the tip.
  ctx.beginPath();
  ctx.arc(CX, 9, 2.6, 0, Math.PI * 2);
  ctx.fill();

  // Two waves each side, radiating from the emitter.
  for (const r of [6.5, 10.5]) {
    ctx.lineWidth = 2;
    ctx.strokeStyle = color;
    ctx.beginPath();
    ctx.arc(CX, 9, r, Math.PI * 1.15, Math.PI * 1.45);
    ctx.stroke();
    ctx.beginPath();
    ctx.arc(CX, 9, r, Math.PI * 1.55, Math.PI * 1.85);
    ctx.stroke();
  }
}
