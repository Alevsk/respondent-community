import type { SliceCreator } from '@respondent/core';
import type { FilterPreset, DetectMode, MobileDrawerType, AspectRatioKey } from '@respondent/core';

export interface ImmersiveUISlice {
  cleanUI: boolean;
  activePreset: FilterPreset;
  detectMode: DetectMode;
  density: number;
  panelCollapsed: {
    layers: boolean;
    effects: boolean;
    cctv: boolean;
  };
  recordingMode: boolean;
  recordingAspectRatio: AspectRatioKey;
  recordingShowGrid: boolean;
  activeMobileDrawer: MobileDrawerType;
  mobileHudExpanded: boolean;
  desktopPanelOpen: Record<'layers' | 'settings' | 'nav', boolean>;
  setCleanUI: (clean: boolean) => void;
  setActivePreset: (preset: FilterPreset) => void;
  setDetectMode: (mode: DetectMode) => void;
  setDensity: (density: number) => void;
  togglePanel: (panel: 'layers' | 'effects' | 'cctv') => void;
  toggleRecordingMode: () => void;
  setRecordingAspectRatio: (ratio: AspectRatioKey) => void;
  setRecordingShowGrid: (show: boolean) => void;
  openMobileDrawer: (drawer: MobileDrawerType) => void;
  closeMobileDrawer: () => void;
  toggleMobileHud: () => void;
  toggleDesktopPanel: (panel: 'layers' | 'settings' | 'nav') => void;
  closeDesktopPanel: (panel: 'layers' | 'settings' | 'nav') => void;
}

export const createImmersiveUISlice: SliceCreator<ImmersiveUISlice> = (set) => ({
  cleanUI: false,
  activePreset: 'NORMAL' as FilterPreset,
  detectMode: 'sparse' as DetectMode,
  density: 50,
  panelCollapsed: { layers: false, effects: false, cctv: false },
  recordingMode: false,
  recordingAspectRatio: '9:16' as AspectRatioKey,
  recordingShowGrid: false,
  activeMobileDrawer: null as MobileDrawerType,
  mobileHudExpanded: false,
  desktopPanelOpen: { layers: false, settings: false, nav: false },
  setCleanUI: (clean) => set({ cleanUI: clean }),
  setActivePreset: (preset) => set({ activePreset: preset }),
  setDetectMode: (mode) => set({ detectMode: mode }),
  setDensity: (density) => set({ density }),
  togglePanel: (panel) =>
    set((state) => ({
      panelCollapsed: { ...state.panelCollapsed, [panel]: !state.panelCollapsed[panel] },
    })),
  toggleRecordingMode: () =>
    set((state) => ({
      recordingMode: !state.recordingMode,
      activeMobileDrawer: !state.recordingMode ? null : state.activeMobileDrawer,
      recordingShowGrid: !state.recordingMode ? state.recordingShowGrid : false,
    })),
  setRecordingAspectRatio: (ratio) => set({ recordingAspectRatio: ratio }),
  setRecordingShowGrid: (show) => set({ recordingShowGrid: show }),
  openMobileDrawer: (drawer) => set({ activeMobileDrawer: drawer }),
  closeMobileDrawer: () => set({ activeMobileDrawer: null }),
  toggleMobileHud: () => set((state) => ({ mobileHudExpanded: !state.mobileHudExpanded })),
  toggleDesktopPanel: (panel) =>
    set((state) => ({
      desktopPanelOpen: { ...state.desktopPanelOpen, [panel]: !state.desktopPanelOpen[panel] },
    })),
  closeDesktopPanel: (panel) =>
    set((state) => ({
      desktopPanelOpen: { ...state.desktopPanelOpen, [panel]: false },
    })),
});
