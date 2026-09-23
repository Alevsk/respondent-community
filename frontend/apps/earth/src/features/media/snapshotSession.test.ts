import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SnapshotSession } from './snapshotSession';

describe('snapshot refresh lifecycle', () => {
  let images: HTMLImageElement[];
  let session: SnapshotSession;
  beforeEach(() => {
    vi.useFakeTimers();
    images = [];
    session = new SnapshotSession(
      'https://camera.example/frame?key=abc',
      { refreshIntervalSeconds: 5, cacheBustParam: 'tick' },
      () => {
        const image = document.createElement('img');
        images.push(image);
        return image;
      },
    );
  });
  afterEach(() => {
    session.dispose();
    vi.useRealTimers();
  });

  it('does not request until started, refreshes only after completion, and never overlaps', () => {
    vi.advanceTimersByTime(60_000);
    expect(images).toHaveLength(0);
    session.start();
    expect(images).toHaveLength(1);
    session.refresh();
    vi.advanceTimersByTime(6_000);
    expect(images).toHaveLength(1);
    images[0].dispatchEvent(new Event('load'));
    expect(session.getState().lastLoaded).toBe(Date.now());
    vi.advanceTimersByTime(4_999);
    expect(images).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(images).toHaveLength(2);
  });

  it('preserves the last good frame, marks it stale and backs off with a bound', () => {
    session.start();
    images[0].dispatchEvent(new Event('load'));
    const good = session.getState().image;
    vi.advanceTimersByTime(5_000);
    images[1].dispatchEvent(new Event('error'));
    expect(session.getState()).toMatchObject({ image: good, stale: true, loading: false });
    vi.advanceTimersByTime(9_999);
    expect(images).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(images).toHaveLength(3);
    for (let i = 0; i < 8; i++) {
      images[images.length - 1].dispatchEvent(new Event('error'));
      vi.advanceTimersByTime(300_000);
    }
    expect(images.length).toBeGreaterThan(9);
  });

  it('times out an unfinished image and cancels pending loads/timers when suspended', () => {
    session.start();
    vi.advanceTimersByTime(15_000);
    expect(session.getState().stale).toBe(true);
    expect(images[0].getAttribute('src')).toBeNull();
    session.stop();
    vi.advanceTimersByTime(600_000);
    expect(images).toHaveLength(1);
  });

  it('ignores late completion after stop, restart and disposal', () => {
    session.start();
    const old = images[0];
    session.stop();
    session.start();
    old.dispatchEvent(new Event('load'));
    expect(session.getState().image).toBeNull();
    images[1].dispatchEvent(new Event('load'));
    expect(session.getState().image).toBe(images[1]);
    session.dispose();
    vi.advanceTimersByTime(60_000);
    expect(images).toHaveLength(2);
  });
});
