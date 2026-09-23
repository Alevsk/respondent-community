import { act, fireEvent, render, screen, cleanup, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { EntityDetailResponse, Layer, MediaConfig } from '@respondent/core';
import { useUIStore } from '@/app/store';
import OverviewTab from '../entity/tabs/OverviewTab';
import MetadataTab from '../entity/tabs/MetadataTab';
import { MediaProvider } from './MediaProvider';
import { AudioPlayer } from './AudioPlayer';

const config: MediaConfig[] = [
  {
    id: 'camera',
    label: 'Harbour view',
    kind: 'snapshot',
    urlKey: 'frame',
    attributionKey: 'credit',
    allowedOrigins: ['https://camera.example'],
    snapshot: { refreshIntervalSeconds: 5 },
  },
  {
    id: 'radio',
    label: 'Local radio',
    kind: 'audio',
    urlKey: 'stream',
    attributionKey: 'credit',
    playbackAction: 'count',
  },
];
const detail: EntityDetailResponse = {
  entity: {
    id: 'media:one',
    externalId: 'one',
    name: 'Harbour',
    layerType: 'custom',
    metadata: {
      frame: 'https://camera.example/old',
      stream: 'https://radio.example/live',
      credit: 'Public station',
      note: 'Local service',
    },
  },
  latestObservation: {
    entityId: 'media:one',
    ts: 123,
    position: { lat: 1, lon: 2, altM: 0 },
    altitudeM: 0,
    metadata: { frame: 'https://camera.example/latest' },
  },
};

describe('declarative media panel integration', () => {
  let images: HTMLImageElement[];
  let observers: { callback: IntersectionObserverCallback; observer: IntersectionObserver }[];
  let audio: HTMLAudioElement;
  const visible = (value = true) =>
    act(() =>
      observers.forEach(({ callback, observer }) =>
        callback([{ isIntersecting: value } as IntersectionObserverEntry], observer),
      ),
    );
  const overview = (id = 'media:one', active = true) => (
    <OverviewTab
      entityId={id}
      layerType="custom"
      detail={{ ...detail, entity: { ...detail.entity!, id } }}
      isLoading={false}
      mediaActive={active}
    />
  );

  beforeEach(() => {
    vi.useFakeTimers();
    images = [];
    observers = [];
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(callback: IntersectionObserverCallback) {
          observers.push({ callback, observer: this as unknown as IntersectionObserver });
        }
        observe() {}
        disconnect() {}
      },
    );
    vi.stubGlobal('Image', function () {
      const img = document.createElement('img');
      images.push(img);
      return img;
    });
    audio = document.createElement('audio');
    audio.play = vi.fn().mockResolvedValue(undefined);
    audio.pause = vi.fn();
    audio.load = vi.fn();
    audio.canPlayType = vi.fn().mockReturnValue('probably');
    vi.stubGlobal('Audio', function () {
      return audio;
    });
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ reported: true }) }),
    );
    const layer = { id: 'custom', type: 'custom', displayConfig: { media: config } } as Layer;
    useUIStore.setState({
      layers: { custom: layer },
      enabledLayers: ['custom'],
      selectedEntityId: 'media:one',
      timeMode: 'live',
    });
  });
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
    useUIStore.setState({
      layers: {},
      enabledLayers: [],
      selectedEntityId: null,
      timeMode: 'live',
    });
  });

  it('renders media before location without duplicate scalar URLs, retaining raw metadata', () => {
    const { container, unmount } = render(<MediaProvider>{overview()}</MediaProvider>);
    expect(screen.getByText('Harbour view')).toBeInTheDocument();
    expect(screen.getByText('Local service')).toBeInTheDocument();
    expect(screen.queryByText('https://camera.example/latest')).not.toBeInTheDocument();
    expect(container.textContent!.indexOf('Harbour view')).toBeLessThan(
      container.textContent!.indexOf('Location'),
    );
    expect(images).toHaveLength(0);
    expect(audio.play).not.toHaveBeenCalled();
    visible();
    expect(images).toHaveLength(1);
    expect(images[0].src).toBe('https://camera.example/latest');
    unmount();
    render(
      <MetadataTab entityId="media:one" layerType="custom" detail={detail} isLoading={false} />,
    );
    expect(screen.getByText('https://radio.example/live')).toBeInTheDocument();
  });

  it('keeps one refresh owner; pinned cards only load after View camera', () => {
    render(
      <MediaProvider>
        {overview()}
        {overview('media:two')}
      </MediaProvider>,
    );
    visible();
    expect(images).toHaveLength(1);
    fireEvent.click(screen.getByRole('button', { name: 'View camera' }));
    expect(images).toHaveLength(2);
    expect(images[0].getAttribute('src')).toBeNull();
    act(() => images[1].dispatchEvent(new Event('load')));
    act(() => vi.advanceTimersByTime(5_000));
    expect(images).toHaveLength(3);
  });

  it('suspends snapshots when collapsed, offscreen, historical, hidden or layer disabled', () => {
    const { rerender } = render(<MediaProvider>{overview('media:one', false)}</MediaProvider>);
    visible();
    expect(images).toHaveLength(0);
    rerender(<MediaProvider>{overview()}</MediaProvider>);
    expect(images).toHaveLength(1);
    visible(false);
    expect(images[0].getAttribute('src')).toBeNull();
    visible();
    expect(images).toHaveLength(2);
    act(() => useUIStore.setState({ timeMode: 'range' }));
    act(() => vi.advanceTimersByTime(60_000));
    expect(images).toHaveLength(2);
    act(() => useUIStore.setState({ timeMode: 'live' }));
    expect(images).toHaveLength(3);
    Object.defineProperty(document, 'hidden', { configurable: true, value: true });
    act(() => document.dispatchEvent(new Event('visibilitychange')));
    act(() => vi.advanceTimersByTime(60_000));
    expect(images).toHaveLength(3);
    Object.defineProperty(document, 'hidden', { configurable: true, value: false });
    act(() => document.dispatchEvent(new Event('visibilitychange')));
    expect(images).toHaveLength(4);
    act(() => useUIStore.setState({ enabledLayers: [] }));
    act(() => vi.advanceTimersByTime(60_000));
    expect(images).toHaveLength(4);
  });

  it('opens the frame full size on click, keeps it refreshing, and closes again', () => {
    render(<MediaProvider>{overview()}</MediaProvider>);
    visible();
    expect(images).toHaveLength(1);
    act(() => images[0].dispatchEvent(new Event('load')));

    // No dialog until the user asks for one.
    expect(screen.queryByTestId('media-snapshot-expanded')).not.toBeInTheDocument();

    // Clicking the picture is the pointer affordance the Expand button names.
    fireEvent.click(screen.getByTestId('media-frame-camera'));
    const dialog = screen.getByTestId('media-snapshot-expanded');
    expect(dialog).toBeInTheDocument();
    // The expanded view shows the frame the running session already loaded —
    // it must not start a second one.
    expect(images).toHaveLength(1);
    expect(within(dialog).getByRole('img')).toHaveAttribute(
      'src',
      'https://camera.example.example/latest'.replace('.example.example', '.example'),
    );
    expect(within(dialog).getByText('Public station')).toBeInTheDocument();

    // The camera keeps refreshing behind the dialog on its own cadence.
    act(() => vi.advanceTimersByTime(5_000));
    expect(images).toHaveLength(2);

    fireEvent.click(screen.getByRole('button', { name: 'Close expanded camera' }));
    act(() => vi.advanceTimersByTime(500));
    expect(screen.queryByTestId('media-snapshot-expanded')).not.toBeInTheDocument();
  });

  it('closes the expanded frame on Escape and when its layer is disabled', () => {
    render(<MediaProvider>{overview()}</MediaProvider>);
    visible();
    act(() => images[0].dispatchEvent(new Event('load')));

    fireEvent.click(screen.getByRole('button', { name: 'Expand Harbour view' }));
    expect(screen.getByTestId('media-snapshot-expanded')).toBeInTheDocument();
    fireEvent.keyDown(screen.getByTestId('media-snapshot-expanded'), { key: 'Escape' });
    act(() => vi.advanceTimersByTime(500));
    expect(screen.queryByTestId('media-snapshot-expanded')).not.toBeInTheDocument();

    // A camera that stops being viewable must not leave a dialog of a frozen
    // frame on screen.
    fireEvent.click(screen.getByRole('button', { name: 'Expand Harbour view' }));
    expect(screen.getByTestId('media-snapshot-expanded')).toBeInTheDocument();
    act(() => useUIStore.setState({ enabledLayers: [] }));
    act(() => vi.advanceTimersByTime(500));
    expect(screen.queryByTestId('media-snapshot-expanded')).not.toBeInTheDocument();
  });

  it('keeps audio when entity panel closes, labels it LIVE in history, and releases on layer disable', () => {
    const { rerender } = render(
      <MediaProvider>
        {overview()}
        <AudioPlayer />
      </MediaProvider>,
    );
    expect(audio.getAttribute('src')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Play Local radio' }));
    expect(audio.src).toBe('https://radio.example/live');
    act(() => audio.dispatchEvent(new Event('playing')));
    expect(fetch).toHaveBeenCalledWith(
      '/v1/media/playback',
      expect.objectContaining({
        body: JSON.stringify({ entityId: 'media:one', mediaId: 'radio' }),
      }),
    );
    rerender(
      <MediaProvider>
        <AudioPlayer />
      </MediaProvider>,
    );
    act(() => useUIStore.setState({ timeMode: 'range' }));
    expect(screen.getByText('LIVE')).toBeInTheDocument();
    expect(screen.getByTestId('media-audio-player')).toHaveTextContent('Harbour');
    expect(audio.src).toBe('https://radio.example/live');
    act(() => useUIStore.setState({ enabledLayers: [] }));
    expect(audio.getAttribute('src')).toBeNull();
    expect(screen.queryByTestId('media-audio-player')).not.toBeInTheDocument();
  });

  it('preserves non-media Overview behavior and rejects declared unsafe URLs without traffic', () => {
    act(() => useUIStore.setState({ layers: {} }));
    const { rerender } = render(<MediaProvider>{overview()}</MediaProvider>);
    expect(screen.queryByText('Harbour view')).not.toBeInTheDocument();
    expect(screen.getByText('https://camera.example/latest')).toBeInTheDocument();
    act(() =>
      useUIStore.setState({
        layers: { custom: { id: 'custom', displayConfig: { media: config } } as Layer },
      }),
    );
    rerender(
      <MediaProvider>
        <OverviewTab
          entityId="media:one"
          layerType="custom"
          detail={{
            entity: {
              ...detail.entity!,
              metadata: { frame: 'https://127.0.0.1/frame', stream: 'javascript:alert(1)' },
            },
          }}
          isLoading={false}
        />
      </MediaProvider>,
    );
    visible();
    expect(images).toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Play Local radio' })).toBeDisabled();
  });
});
