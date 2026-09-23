/**
 * MediaProvider — app-level ownership for declarative entity media.
 *
 * Two things must be singular no matter how many entity panels are open: the
 * camera that is currently refreshing, and the audio element. Both live here.
 * Panels ask for the snapshot slot and read the shared audio session; nobody
 * else creates timers or media elements.
 */

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { AudioSession } from './audioSession';

interface MediaContextValue {
  /** The one audio element in the application. */
  audio: AudioSession;
  /** Key of the snapshot slot currently allowed to refresh, if any. */
  snapshotOwner: string | null;
  /** Take the snapshot slot. Without `force`, an existing owner keeps it. */
  claimSnapshot: (key: string, force?: boolean) => void;
  /** Give up the snapshot slot, if this key holds it. */
  releaseSnapshot: (key: string) => void;
  /** False while the browser document is hidden. */
  documentVisible: boolean;
}

const MediaContext = createContext<MediaContextValue | null>(null);

export const MediaProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const audioRef = useRef<AudioSession | null>(null);
  if (audioRef.current === null) audioRef.current = new AudioSession();
  const audio = audioRef.current;

  const [snapshotOwner, setSnapshotOwner] = useState<string | null>(null);
  const [documentVisible, setDocumentVisible] = useState(
    () => typeof document === 'undefined' || !document.hidden,
  );

  useEffect(() => {
    const onVisibility = () => setDocumentVisible(!document.hidden);
    document.addEventListener('visibilitychange', onVisibility);
    return () => document.removeEventListener('visibilitychange', onVisibility);
  }, []);

  // The audio element outlives every panel, so only the provider disposes it.
  useEffect(() => () => audio.dispose(), [audio]);

  // Identity-stable: consumers hold these in effect dependency lists, and a
  // claim/release that churned on every provider render would hand the slot
  // back and forth between panels.
  const claimSnapshot = useCallback(
    (key: string, force?: boolean) =>
      setSnapshotOwner((current) => (force || current === null ? key : current)),
    [],
  );
  const releaseSnapshot = useCallback(
    (key: string) => setSnapshotOwner((current) => (current === key ? null : current)),
    [],
  );

  const value = useMemo<MediaContextValue>(
    () => ({ audio, snapshotOwner, documentVisible, claimSnapshot, releaseSnapshot }),
    [audio, snapshotOwner, documentVisible, claimSnapshot, releaseSnapshot],
  );

  return <MediaContext.Provider value={value}>{children}</MediaContext.Provider>;
};

/** Returns the media context, or null when no provider is mounted. */
export function useMediaContext(): MediaContextValue | null {
  return useContext(MediaContext);
}
