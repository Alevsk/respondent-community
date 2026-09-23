import { createStore } from 'zustand/vanilla';
import { api, endpoints } from '@respondent/core';
import type { ReportMediaPlaybackRequest, ReportMediaPlaybackResponse } from '@respondent/core';
import { validateMediaUrl } from './mediaUrls';

export interface AudioStation {
  entityId: string;
  mediaId: string;
  layerId: string;
  name: string;
  attribution: string;
  url: string;
  playbackAction?: string;
}
interface AudioState {
  station: Omit<AudioStation, 'url'> | null;
  status: 'idle' | 'loading' | 'playing' | 'paused' | 'error';
  error: string;
  volume: number;
}

/** App-owned native element; the observable store contains only serializable UI state. */
export class AudioSession {
  readonly store = createStore<AudioState>(() => ({
    station: null,
    status: 'idle',
    error: '',
    volume: 0.8,
  }));
  private element: HTMLAudioElement | null = null;
  private station: AudioStation | null = null;
  private generation = 0;
  private detach?: () => void;
  private timeout?: ReturnType<typeof setTimeout>;

  constructor(
    private createAudio: () => HTMLAudioElement = () => new Audio(),
    private report: (body: ReportMediaPlaybackRequest) => Promise<ReportMediaPlaybackResponse> = (
      body,
    ) => api.post<ReportMediaPlaybackResponse>(endpoints.mediaPlayback, body),
  ) {}

  play(station: AudioStation) {
    this.release();
    this.station = station;
    const { url, ...summary } = station;
    this.store.setState({ station: summary, status: 'loading', error: '' });
    if (!validateMediaUrl(url)) {
      this.fail('Media URL is unavailable or not allowed.');
      return;
    }
    const audio = (this.element ??= this.createAudio());
    audio.preload = 'none';
    if (
      !audio.canPlayType('audio/mpeg') &&
      !audio.canPlayType('audio/aac') &&
      !audio.canPlayType('audio/mp4; codecs="mp4a.40.2"')
    ) {
      this.fail('Native MP3/AAC playback is not supported by this browser.');
      return;
    }
    audio.volume = this.store.getState().volume;
    const generation = this.generation;
    let reported = false;
    const current = () =>
      generation === this.generation && (!audio.currentSrc || audio.currentSrc === url);
    const playing = () => {
      if (!current()) return;
      clearTimeout(this.timeout);
      // Clear the handle too, not just the timer: armTimeout is a no-op while
      // one is assigned, so leaving it set would stop a later stall from ever
      // re-arming the watchdog and park the player in `loading` for good.
      this.timeout = undefined;
      this.store.setState({ status: 'playing', error: '' });
      if (!reported) {
        reported = true;
        if (station.playbackAction) {
          // Reporting is best-effort and must never gate or interrupt playback.
          try {
            void this.report({ entityId: station.entityId, mediaId: station.mediaId }).catch(
              () => {},
            );
          } catch {
            /* synchronous adapters are isolated too */
          }
        }
      }
    };
    const waiting = () => {
      if (!current()) return;
      this.store.setState({ status: 'loading' });
      this.armTimeout(generation);
    };
    const error = () => {
      if (current())
        this.fail(
          audio.error?.code === 4
            ? 'This stream or codec is not supported.'
            : 'The live audio stream is unavailable. Try Play again.',
        );
    };
    const ended = () => {
      if (current()) this.pause();
    };
    audio.addEventListener('playing', playing);
    audio.addEventListener('waiting', waiting);
    audio.addEventListener('stalled', waiting);
    audio.addEventListener('error', error);
    audio.addEventListener('ended', ended);
    this.detach = () => {
      audio.removeEventListener('playing', playing);
      audio.removeEventListener('waiting', waiting);
      audio.removeEventListener('stalled', waiting);
      audio.removeEventListener('error', error);
      audio.removeEventListener('ended', ended);
    };
    audio.src = url;
    this.armTimeout(generation);
    try {
      // Called synchronously from the Play click, before any await.
      void audio.play().catch(() => {
        if (generation === this.generation) this.fail('Playback could not start. Try Play again.');
      });
    } catch {
      this.fail('Playback could not start. Try Play again.');
    }
  }
  private armTimeout(generation: number) {
    if (this.timeout) return;
    this.timeout = setTimeout(() => {
      if (generation === this.generation)
        this.fail('The live audio stream timed out. Try Play again.');
    }, 20_000);
  }
  private release() {
    this.generation++;
    clearTimeout(this.timeout);
    this.timeout = undefined;
    this.detach?.();
    this.detach = undefined;
    if (this.element) {
      this.element.pause();
      this.element.removeAttribute('src');
      this.element.load();
    }
  }
  private fail(error: string) {
    this.release();
    this.store.setState({ status: 'error', error });
  }
  pause() {
    this.release();
    if (this.station) this.store.setState({ status: 'paused' });
  }
  resume() {
    if (this.station) this.play(this.station);
  }
  setVolume(volume: number) {
    const value = Math.max(0, Math.min(1, volume));
    if (this.element) this.element.volume = value;
    this.store.setState({ volume: value });
  }
  stop() {
    this.release();
    this.station = null;
    this.store.setState({ station: null, status: 'idle', error: '' });
  }
  dispose() {
    this.stop();
    this.element = null;
  }
}
