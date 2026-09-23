import type { SnapshotOptions } from '@respondent/core';
import { snapshotUrl } from './mediaUrls';

interface SnapshotState {
  image: HTMLImageElement | null;
  lastLoaded: number | null;
  stale: boolean;
  loading: boolean;
}

/** One completion-driven refresh loop. Ownership/visibility are supplied by the UI. */
export class SnapshotSession {
  private state: SnapshotState = { image: null, lastLoaded: null, stale: false, loading: false };
  private listeners = new Set<() => void>();
  private running = false;
  private failures = 0;
  private timer?: ReturnType<typeof setTimeout>;
  private cancelLoad?: () => void;

  constructor(
    private url: string,
    private options: SnapshotOptions,
    private createImage: () => HTMLImageElement = () => new Image(),
  ) {}
  getState = () => this.state;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  private update(patch: Partial<SnapshotState>) {
    this.state = { ...this.state, ...patch };
    this.listeners.forEach((listener) => listener());
  }
  start() {
    if (this.running) return;
    this.running = true;
    this.refresh();
  }
  stop() {
    this.running = false;
    clearTimeout(this.timer);
    this.cancelLoad?.();
    this.cancelLoad = undefined;
    if (this.state.loading) this.update({ loading: false });
  }
  refresh() {
    if (!this.running || this.cancelLoad) return;
    clearTimeout(this.timer);
    const image = this.createImage();
    let completed = false;
    // Held in an object so detach() can clear a timer armed after it is defined.
    const load: { timeout?: ReturnType<typeof setTimeout> } = {};
    const detach = () => {
      clearTimeout(load.timeout);
      image.removeEventListener('load', success);
      image.removeEventListener('error', failure);
    };
    const finish = (ok: boolean) => {
      if (completed) return;
      completed = true;
      detach();
      this.cancelLoad = undefined;
      if (ok) {
        this.failures = 0;
        this.update({ image, lastLoaded: Date.now(), stale: false, loading: false });
      } else {
        this.failures = Math.min(this.failures + 1, 6);
        image.removeAttribute('src');
        this.update({ stale: true, loading: false });
      }
      const interval = Math.max(5, Math.min(3600, this.options.refreshIntervalSeconds)) * 1000;
      const delay = this.failures
        ? Math.min(interval * 2 ** this.failures, Math.max(interval, 300_000))
        : interval;
      if (this.running) this.timer = setTimeout(() => this.refresh(), delay);
    };
    const success = () => finish(true);
    const failure = () => finish(false);
    this.cancelLoad = () => {
      completed = true;
      detach();
      image.removeAttribute('src');
    };
    image.addEventListener('load', success);
    image.addEventListener('error', failure);
    image.referrerPolicy = 'no-referrer';
    load.timeout = setTimeout(failure, 15_000);
    this.update({ loading: true });
    image.src = snapshotUrl(this.url, this.options.cacheBustParam);
  }
  dispose() {
    this.stop();
    this.listeners.clear();
    this.state.image?.removeAttribute('src');
    this.state = { image: null, lastLoaded: null, stale: false, loading: false };
  }
}
