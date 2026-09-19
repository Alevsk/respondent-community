import React, { useEffect } from 'react';
import { useUIStore } from '@/app/store';
import AspectRatioOverlay from './AspectRatioOverlay';
import AspectRatioPicker from './AspectRatioPicker';

// Typed extension for the Screen Orientation lock API (not in all TS DOM libs)
interface OrientationLockable {
  lock?: (orientation: string) => Promise<void>;
  unlock?: () => void;
}

const RecordingMode: React.FC = () => {
  const recordingMode = useUIStore((s) => s.recordingMode);
  const recordingAspectRatio = useUIStore((s) => s.recordingAspectRatio);
  const recordingShowGrid = useUIStore((s) => s.recordingShowGrid);

  // Attempt screen orientation lock
  useEffect(() => {
    if (!recordingMode) return;

    const lockOrientation = async () => {
      try {
        const orientation = screen.orientation as ScreenOrientation & OrientationLockable;
        if (typeof orientation?.lock === 'function') {
          if (recordingAspectRatio === '9:16' || recordingAspectRatio === '4:5') {
            await orientation.lock('portrait');
          } else if (recordingAspectRatio === '16:9') {
            await orientation.lock('landscape');
          }
        }
      } catch {
        // Orientation lock not supported
      }
    };

    lockOrientation();

    return () => {
      try {
        const orientation = screen.orientation as ScreenOrientation & OrientationLockable;
        if (typeof orientation?.unlock === 'function') {
          orientation.unlock();
        }
      } catch {
        // Ignore
      }
    };
  }, [recordingMode, recordingAspectRatio]);

  // Deselect entities when entering recording mode
  useEffect(() => {
    if (recordingMode) {
      useUIStore.getState().clearSelection();
    }
  }, [recordingMode]);

  if (!recordingMode) return null;

  return (
    <>
      <AspectRatioOverlay ratio={recordingAspectRatio} showGrid={recordingShowGrid} />
      <AspectRatioPicker />
    </>
  );
};

export default RecordingMode;
