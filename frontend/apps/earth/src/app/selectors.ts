import type { EarthState } from './store';

export function selectIsolatedEntityId(state: EarthState): string | null {
  for (const [eid, vs] of Object.entries(state.entityViewState)) {
    if (vs.isolateEntity && state.selectedEntities.some((e) => e.entityId === eid)) {
      return eid;
    }
  }
  return null;
}
