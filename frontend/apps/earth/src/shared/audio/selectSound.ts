/**
 * Selection audio cue — short metallic "lock-on" click.
 *
 * Uses Web Audio API to synthesize the sound programmatically,
 * avoiding the need for an external .wav file. The sound is a
 * brief (120ms) high-frequency ping with rapid decay.
 *
 * AudioContext is lazily initialized on first user gesture
 * to comply with browser autoplay policies.
 */

let audioCtx: AudioContext | null = null;

function getAudioContext(): AudioContext {
  if (!audioCtx) {
    audioCtx = new AudioContext();
  }
  return audioCtx;
}

export function playSelectSound(): void {
  try {
    const ctx = getAudioContext();

    // Resume if suspended (browser autoplay policy)
    if (ctx.state === 'suspended') {
      ctx.resume();
    }

    const now = ctx.currentTime;

    // Oscillator: short, high-frequency ping
    const osc = ctx.createOscillator();
    osc.type = 'sine';
    osc.frequency.setValueAtTime(1800, now);
    osc.frequency.exponentialRampToValueAtTime(1200, now + 0.08);

    // Gain envelope: sharp attack, fast decay
    const gain = ctx.createGain();
    gain.gain.setValueAtTime(0.15, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.12);

    osc.connect(gain);
    gain.connect(ctx.destination);

    osc.start(now);
    osc.stop(now + 0.12);
  } catch {
    // Audio is non-critical — fail silently
  }
}
