import type { SliceCreator } from './types';

/** State + actions for globe display toggles. */
export interface GlobeDisplaySlice {
  showOccluded: boolean;
  showGeoLabels: boolean;
  show3DBuildings: boolean;
  smoothMotion: boolean;
  cinematicDrift: boolean;
  /** Groups nearby entities into cluster markers (client-side declutter). Off by default. */
  spatialAggregation: boolean;
  setShowOccluded: (show: boolean) => void;
  setShowGeoLabels: (show: boolean) => void;
  setShow3DBuildings: (show: boolean) => void;
  setSmoothMotion: (enabled: boolean) => void;
  setCinematicDrift: (enabled: boolean) => void;
  setSpatialAggregation: (enabled: boolean) => void;
}

export const createGlobeDisplaySlice: SliceCreator<GlobeDisplaySlice> = (set) => ({
  showOccluded: true,
  showGeoLabels: true,
  show3DBuildings: false,
  smoothMotion: true,
  cinematicDrift: false,
  spatialAggregation: false,
  setShowOccluded: (show) => set({ showOccluded: show }),
  setShowGeoLabels: (show) => set({ showGeoLabels: show }),
  setShow3DBuildings: (show) => set({ show3DBuildings: show }),
  setSmoothMotion: (enabled) => set({ smoothMotion: enabled }),
  setCinematicDrift: (enabled) => set({ cinematicDrift: enabled }),
  setSpatialAggregation: (enabled) => set({ spatialAggregation: enabled }),
});
