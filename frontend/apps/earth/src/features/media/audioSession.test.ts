import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest';
import { AudioSession, type AudioStation } from './audioSession';

const station: AudioStation = {
  entityId: 'station:1',
  mediaId: 'radio',
  layerId: 'radio',
  name: 'Station One',
  attribution: 'Owner',
  url: 'https://radio.example/live',
  playbackAction: 'count',
};

describe('app audio session', () => {
  let audio: HTMLAudioElement;
  let session: AudioSession;
  let factory: Mock<() => HTMLAudioElement>;
  let report: Mock<(body: { entityId: string; mediaId: string }) => Promise<unknown>>;
  beforeEach(() => {
    audio = document.createElement('audio');
    audio.play = vi.fn().mockResolvedValue(undefined);
    audio.pause = vi.fn();
    audio.load = vi.fn();
    audio.canPlayType = vi.fn().mockReturnValue('probably');
    factory = vi.fn(() => audio);
    report = vi.fn(async () => ({ reported: true }));
    session = new AudioSession(factory, report);
  });

  it('creates no audio or URL traffic before user play and reuses one element', async () => {
    expect(factory).not.toHaveBeenCalled();
    session.play(station);
    expect(audio.play).toHaveBeenCalledTimes(1); // synchronous user gesture
    expect(audio.src).toBe(station.url);
    expect(session.store.getState().status).toBe('loading');
    await Promise.resolve();
    expect(session.store.getState().status).toBe('loading');
    audio.dispatchEvent(new Event('playing'));
    expect(session.store.getState().status).toBe('playing');
    session.play({ ...station, entityId: 'station:2', url: 'https://second.example/live' });
    expect(factory).toHaveBeenCalledTimes(1);
    expect(audio.pause).toHaveBeenCalled();
    expect(audio.src).toBe('https://second.example/live');
  });

  it('reports identifiers once per successful user start, never waiting/reconnect or selection', async () => {
    session.play(station);
    await Promise.resolve();
    expect(report).not.toHaveBeenCalled();
    audio.dispatchEvent(new Event('playing'));
    audio.dispatchEvent(new Event('waiting'));
    audio.dispatchEvent(new Event('playing'));
    expect(report).toHaveBeenCalledExactlyOnceWith({
      entityId: station.entityId,
      mediaId: station.mediaId,
    });
    session.pause();
    session.resume();
    audio.dispatchEvent(new Event('playing'));
    expect(report).toHaveBeenCalledTimes(2);
  });

  it('notification failure cannot stop successful audio', async () => {
    report.mockRejectedValue(new Error('offline'));
    session.play(station);
    audio.dispatchEvent(new Event('playing'));
    await Promise.resolve();
    expect(session.store.getState().status).toBe('playing');
  });

  it('handles rejected play and ignores stale promises after switching station', async () => {
    let rejectFirst!: (reason: Error) => void;
    vi.mocked(audio.play).mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectFirst = reject;
        }),
    );
    session.play(station);
    session.play({ ...station, entityId: 'station:2', url: 'https://second.example/live' });
    rejectFirst(new Error('late rejection'));
    await Promise.resolve();
    expect(session.store.getState()).toMatchObject({
      station: { entityId: 'station:2' },
      status: 'loading',
    });
    vi.mocked(audio.play).mockRejectedValueOnce(new Error('NotAllowedError'));
    session.resume();
    await Promise.resolve();
    expect(session.store.getState().status).toBe('error');
  });

  it('releases src on pause, stop and teardown and never stores the audio element', () => {
    session.play(station);
    session.pause();
    expect(audio.getAttribute('src')).toBeNull();
    session.resume();
    session.setVolume(0.4);
    expect(audio.volume).toBe(0.4);
    session.stop();
    expect(audio.getAttribute('src')).toBeNull();
    expect(session.store.getState().station).toBeNull();
    expect(JSON.stringify(session.store.getState())).not.toContain('https://');
    session.dispose();
    audio.dispatchEvent(new Event('playing'));
    expect(session.store.getState().status).toBe('idle');
  });

  it('fails without a request for unsafe URLs or unavailable native codecs', () => {
    session.play({ ...station, url: 'http://radio.example/live' });
    expect(factory).not.toHaveBeenCalled();
    expect(session.store.getState().status).toBe('error');
    vi.mocked(audio.canPlayType).mockReturnValue('');
    session.play(station);
    expect(audio.getAttribute('src')).toBeNull();
    expect(session.store.getState().error).toMatch(/supported/i);
    expect(report).not.toHaveBeenCalled();
  });
});
