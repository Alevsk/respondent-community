import type { Page } from '@playwright/test';

/**
 * Selection helpers driving the e2e store hooks exposed in `main.tsx`:
 *   - window.__store_setSelected(entityId, layerId)  → setSelectedEntity(...)
 *   - window.__store()                               → read-only store snapshot
 *
 * These let a spec drive entity selection deterministically without a canvas
 * click (Cesium picking is not testid-addressable). The hooks are defined in
 * `frontend/apps/earth/src/main.tsx` and typed in `src/vite-env.d.ts`; the e2e
 * project does not see that global augmentation, so we narrow `window` locally.
 */

/** A cluster the e2e proxy can open (mirrors ActiveCluster minus runtime types). */
export interface E2ECluster {
  clusterId: string;
  layerId: string;
  entityIds: string[];
  count: number;
  truncated: boolean;
  screenX: number;
  screenY: number;
}

/** Minimal shape of the e2e window hooks, scoped to this helper. */
interface E2EWindow {
  __store_setSelected?: (entityId: string | null, layerId: string | null) => void;
  __store_enterEntityView?: (entityId: string, layerId: string) => void;
  __store_setActiveCluster?: (cluster: E2ECluster) => void;
  __store?: () => {
    selectedEntityId: string | null;
    selectedLayerId: string | null;
    viewMode: 'globe' | 'entity';
    activeCluster: E2ECluster | null;
  };
}

/**
 * Open a cluster's member list via the store hook — the DOM-driven proxy for a
 * Cesium cluster-marker click (canvas picks are not testid-addressable, same
 * constraint as entity selection).
 */
export async function openCluster(page: Page, cluster: E2ECluster): Promise<void> {
  await page.evaluate((c) => {
    const w = window as unknown as E2EWindow;
    w.__store_setActiveCluster?.(c);
  }, cluster);
}

/** Read the active cluster id from the store snapshot hook (null if none). */
export async function getActiveClusterId(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    const w = window as unknown as E2EWindow;
    return w.__store?.().activeCluster?.clusterId ?? null;
  });
}

/**
 * Select an entity via the store hook (no globe interaction). Mounts the
 * EntityDetailPanel for the entity, which renders from `selectedEntities`
 * regardless of whether live data is present.
 */
export async function selectEntity(page: Page, entityId: string, layerId: string): Promise<void> {
  await page.evaluate(
    ([e, l]) => {
      const w = window as unknown as E2EWindow;
      w.__store_setSelected?.(e, l);
    },
    [entityId, layerId] as const,
  );
}

/** Read the currently selected entity id from the store snapshot hook. */
export async function getSelectedEntityId(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    const w = window as unknown as E2EWindow;
    return w.__store?.().selectedEntityId ?? null;
  });
}

/** Read the currently selected layer id from the store snapshot hook. */
export async function getSelectedLayerId(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    const w = window as unknown as E2EWindow;
    return w.__store?.().selectedLayerId ?? null;
  });
}

/** Clear the current selection (empty-click proxy: no canvas pick). */
export async function clearSelection(page: Page): Promise<void> {
  await page.evaluate(() => {
    const w = window as unknown as E2EWindow;
    w.__store_setSelected?.(null, null);
  });
}

/**
 * Enter entity-view (viewMode === 'entity') via the store hook — the DOM-driven
 * proxy for the Cesium canvas double-click that frames/tracks an entity. Selects
 * the entity and flips viewMode so the "Exit Entity View" affordance mounts.
 */
export async function enterEntityView(
  page: Page,
  entityId: string,
  layerId: string,
): Promise<void> {
  await page.evaluate(
    ([e, l]) => {
      const w = window as unknown as E2EWindow;
      w.__store_enterEntityView?.(e, l);
    },
    [entityId, layerId] as const,
  );
}

/** Read the current viewMode ('globe' | 'entity') from the store snapshot hook. */
export async function getViewMode(page: Page): Promise<'globe' | 'entity' | null> {
  return page.evaluate(() => {
    const w = window as unknown as E2EWindow;
    return w.__store?.().viewMode ?? null;
  });
}
