import { describe, it, expect, beforeEach } from 'vitest';
import { create } from 'zustand';
import { createImmersiveUISlice, type ImmersiveUISlice } from './createImmersiveUISlice';

function makeStore() {
  return create<ImmersiveUISlice>()((...args) => createImmersiveUISlice(...args));
}

describe('createImmersiveUISlice', () => {
  let store: ReturnType<typeof makeStore>;

  beforeEach(() => {
    store = makeStore();
  });

  it('has correct initial state', () => {
    const s = store.getState();
    expect(s.cleanUI).toBe(false);
    expect(s.activePreset).toBe('NORMAL');
    expect(s.detectMode).toBe('sparse');
    expect(s.density).toBe(50);
    expect(s.panelCollapsed).toEqual({ layers: false, effects: false, cctv: false });
    expect(s.recordingMode).toBe(false);
    expect(s.recordingAspectRatio).toBe('9:16');
    expect(s.recordingShowGrid).toBe(false);
    expect(s.activeMobileDrawer).toBeNull();
    expect(s.mobileHudExpanded).toBe(false);
  });

  it('setCleanUI updates cleanUI', () => {
    store.getState().setCleanUI(true);
    expect(store.getState().cleanUI).toBe(true);
  });

  it('setActivePreset updates activePreset', () => {
    store.getState().setActivePreset('CRT');
    expect(store.getState().activePreset).toBe('CRT');
  });

  it('setDetectMode updates detectMode', () => {
    store.getState().setDetectMode('panoptic');
    expect(store.getState().detectMode).toBe('panoptic');
  });

  it('setDensity updates density', () => {
    store.getState().setDensity(75);
    expect(store.getState().density).toBe(75);
  });

  it('togglePanel toggles individual panel', () => {
    expect(store.getState().panelCollapsed.layers).toBe(false);
    store.getState().togglePanel('layers');
    expect(store.getState().panelCollapsed.layers).toBe(true);
    store.getState().togglePanel('layers');
    expect(store.getState().panelCollapsed.layers).toBe(false);
  });

  it('togglePanel does not affect other panels', () => {
    store.getState().togglePanel('effects');
    expect(store.getState().panelCollapsed.layers).toBe(false);
    expect(store.getState().panelCollapsed.effects).toBe(true);
    expect(store.getState().panelCollapsed.cctv).toBe(false);
  });

  it('toggleRecordingMode toggles recordingMode', () => {
    store.getState().toggleRecordingMode();
    expect(store.getState().recordingMode).toBe(true);
    store.getState().toggleRecordingMode();
    expect(store.getState().recordingMode).toBe(false);
  });

  it('toggleRecordingMode clears activeMobileDrawer on enter', () => {
    store.getState().openMobileDrawer('layers');
    store.getState().toggleRecordingMode();
    expect(store.getState().activeMobileDrawer).toBeNull();
    expect(store.getState().recordingMode).toBe(true);
  });

  it('toggleRecordingMode resets recordingShowGrid on exit', () => {
    store.getState().toggleRecordingMode(); // enter
    store.getState().setRecordingShowGrid(true);
    store.getState().toggleRecordingMode(); // exit
    expect(store.getState().recordingShowGrid).toBe(false);
  });

  it('setRecordingAspectRatio updates ratio', () => {
    store.getState().setRecordingAspectRatio('16:9');
    expect(store.getState().recordingAspectRatio).toBe('16:9');
  });

  it('setRecordingShowGrid updates grid visibility', () => {
    store.getState().setRecordingShowGrid(true);
    expect(store.getState().recordingShowGrid).toBe(true);
  });

  it('openMobileDrawer sets activeMobileDrawer', () => {
    store.getState().openMobileDrawer('settings');
    expect(store.getState().activeMobileDrawer).toBe('settings');
  });

  it('closeMobileDrawer clears activeMobileDrawer', () => {
    store.getState().openMobileDrawer('nav');
    store.getState().closeMobileDrawer();
    expect(store.getState().activeMobileDrawer).toBeNull();
  });

  it('toggleMobileHud toggles mobileHudExpanded', () => {
    store.getState().toggleMobileHud();
    expect(store.getState().mobileHudExpanded).toBe(true);
    store.getState().toggleMobileHud();
    expect(store.getState().mobileHudExpanded).toBe(false);
  });

  it('desktopPanelOpen initializes all panels to false', () => {
    const s = store.getState();
    expect(s.desktopPanelOpen).toEqual({ layers: false, settings: false, nav: false });
  });

  it('toggleDesktopPanel toggles individual desktop panel', () => {
    expect(store.getState().desktopPanelOpen.layers).toBe(false);
    store.getState().toggleDesktopPanel('layers');
    expect(store.getState().desktopPanelOpen.layers).toBe(true);
    store.getState().toggleDesktopPanel('layers');
    expect(store.getState().desktopPanelOpen.layers).toBe(false);
  });

  it('toggleDesktopPanel does not affect other desktop panels', () => {
    store.getState().toggleDesktopPanel('settings');
    expect(store.getState().desktopPanelOpen.layers).toBe(false);
    expect(store.getState().desktopPanelOpen.settings).toBe(true);
    expect(store.getState().desktopPanelOpen.nav).toBe(false);
  });

  it('closeDesktopPanel sets the panel to false', () => {
    store.getState().toggleDesktopPanel('nav');
    expect(store.getState().desktopPanelOpen.nav).toBe(true);
    store.getState().closeDesktopPanel('nav');
    expect(store.getState().desktopPanelOpen.nav).toBe(false);
  });

  it('closeDesktopPanel does not affect other desktop panels', () => {
    store.getState().toggleDesktopPanel('layers');
    store.getState().toggleDesktopPanel('settings');
    store.getState().closeDesktopPanel('layers');
    expect(store.getState().desktopPanelOpen.layers).toBe(false);
    expect(store.getState().desktopPanelOpen.settings).toBe(true);
    expect(store.getState().desktopPanelOpen.nav).toBe(false);
  });
});
