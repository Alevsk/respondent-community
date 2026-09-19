import React from 'react';
import ReactDOM from 'react-dom/client';
import { wsClient, loadRuntimeConfig } from '@respondent/core';
import { useUIStore } from './app/store';
import './index.css';

// E2E automation hook — read-only access to the WS client singleton so
// Playwright tests can call resetAndReconnect() to simulate a network drop
// without triggering the intentional-disconnect path. Exposed in all builds (the
// e2e bundle is a production build); no application code calls it.
window.__wsClient = wsClient;

// E2E automation hook — read-only snapshot of UI store state for assertions, and
// a selection seeder so tests can drive entity selection without a canvas click.
// Exposed in all builds (like __wsClient): __store() is read-only, and the
// setters below wrap store actions a user can already trigger, so they grant no
// new capability. No application code calls them.
window.__store = () => {
  const s = useUIStore.getState();
  return {
    selectedEntityId: s.selectedEntityId,
    selectedLayerId: s.selectedLayerId,
    selectedEntities: s.selectedEntities,
    viewMode: s.viewMode,
    watchlistEntities: s.watchlistEntities,
    findMode: s.findMode,
    timeMode: s.timeMode,
    timeFrom: s.timeFrom,
    timeTo: s.timeTo,
    timePreset: s.timePreset,
    showOccluded: s.showOccluded,
    showGeoLabels: s.showGeoLabels,
    show3DBuildings: s.show3DBuildings,
    smoothMotion: s.smoothMotion,
    cinematicDrift: s.cinematicDrift,
    spatialAggregation: s.spatialAggregation,
    activeCluster: s.activeCluster,
    maxEntities: s.maxEntities,
  };
};
window.__store_setSelected = (entityId: string | null, layerId: string | null) =>
  useUIStore.getState().setSelectedEntity(entityId, layerId);

// E2E automation hook — opens a cluster's member list deterministically,
// mirroring the Cesium cluster-marker pick (useEntityInteraction.ts) which is not
// DOM/testid-addressable. Drives the same setActiveCluster store action the pick
// handler calls; adds no capability. No application code calls it.
window.__store_setActiveCluster = (cluster) => useUIStore.getState().setActiveCluster(cluster);

// E2E automation hook — enters entity-view (viewMode === 'entity') deterministically,
// mirroring the Cesium canvas LEFT_DOUBLE_CLICK path (useEntityInteraction.ts) which
// is not DOM/testid-addressable. Selects the entity, then flips viewMode to 'entity'
// via the real store actions so the "Exit Entity View" affordance mounts. Exposed in
// all builds; the double-click handler drives the same actions, so this adds no
// capability. No application code calls it.
window.__store_enterEntityView = (entityId: string, layerId: string) => {
  useUIStore.getState().setSelectedEntity(entityId, layerId);
  useUIStore.getState().setViewMode('entity');
};

// Load runtime config (GET /config.json) BEFORE importing the app, so the app's
// module graph (e.g. GlobeScene's Cesium ion token) reads the loaded values at
// import time. loadRuntimeConfig resolves even on failure, so startup never blocks;
// the dynamic import then evaluates the app modules with config already in place.
void loadRuntimeConfig().then(async () => {
  const { default: EarthApp } = await import('./app/EarthApp');
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <EarthApp />
    </React.StrictMode>,
  );
});
