import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { Viewer, Cartesian3 } from 'cesium';

export interface ViewportBBox {
  west: number;
  south: number;
  east: number;
  north: number;
}

export interface CameraState {
  lat: number;
  lon: number;
  altitude: number;
  heading: number;
  pitch: number;
  roll: number;
}

const CAMERA_STORAGE_KEY = 'respondent:camera';
const PERSIST_DEBOUNCE_MS = 1000;

/** Default camera: full globe view, top-down, centered on 0° lon / 20° N lat. */
export const DEFAULT_CAMERA: CameraState = {
  lon: 0,
  lat: 20,
  altitude: 18_000_000,
  heading: 0,
  pitch: -90,
  roll: 0,
};

function isValidCameraState(v: unknown): v is CameraState {
  if (typeof v !== 'object' || v === null) return false;
  const o = v as Record<string, unknown>;
  return (
    typeof o.lat === 'number' &&
    typeof o.lon === 'number' &&
    typeof o.altitude === 'number' &&
    typeof o.heading === 'number' &&
    typeof o.pitch === 'number' &&
    typeof o.roll === 'number'
  );
}

/** Load persisted camera from localStorage, falling back to DEFAULT_CAMERA. */
export function loadPersistedCamera(): CameraState {
  try {
    const raw = localStorage.getItem(CAMERA_STORAGE_KEY);
    if (raw) {
      const parsed: unknown = JSON.parse(raw);
      if (isValidCameraState(parsed)) return parsed;
    }
  } catch {
    // Corrupted data — fall through to default.
  }
  return DEFAULT_CAMERA;
}

/** Debounced persist: coalesces rapid camera updates into a single write. */
let persistTimer: ReturnType<typeof setTimeout> | null = null;

export function persistCamera(camera: CameraState): void {
  if (persistTimer) clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    try {
      localStorage.setItem(CAMERA_STORAGE_KEY, JSON.stringify(camera));
    } catch {
      // Storage full or unavailable — silently ignore.
    }
  }, PERSIST_DEBOUNCE_MS);
}

interface ViewerState {
  /** Cesium Viewer instance — set once on init, cleared on destroy. */
  viewer: Viewer | null;
  setViewer: (viewer: Viewer | null) => void;
  camera: CameraState | null;
  setCamera: (camera: CameraState) => void;
  trackingEntity: string | null;
  setTrackingEntity: (entityId: string | null) => void;
  viewport: ViewportBBox | null;
  setViewport: (viewport: ViewportBBox) => void;
  savedCamera: CameraState | null;
  setSavedCamera: (camera: CameraState | null) => void;
  /** When set, the tracking loop targets this position instead of the live entity. */
  trackingOverridePosition: Cartesian3 | null;
  setTrackingOverridePosition: (pos: Cartesian3 | null) => void;
}

export const useViewerStore = create<ViewerState>()(
  subscribeWithSelector((set) => ({
    viewer: null,
    setViewer: (viewer) => set({ viewer }),
    camera: null,
    setCamera: (camera) => set({ camera }),
    trackingEntity: null,
    setTrackingEntity: (entityId) => set({ trackingEntity: entityId }),
    viewport: null,
    setViewport: (viewport) => set({ viewport }),
    savedCamera: null,
    setSavedCamera: (camera) => set({ savedCamera: camera }),
    trackingOverridePosition: null,
    setTrackingOverridePosition: (pos) => set({ trackingOverridePosition: pos }),
  })),
);
