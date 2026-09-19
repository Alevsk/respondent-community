import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import MobileClusterPanel from './MobileClusterPanel';
import { useUIStore } from '@/app/store';
import type { Entity, Observation } from '@respondent/core';

const LAYER = 'flights';

function activate() {
  const entities: Entity[] = [
    { id: 'a', name: 'Alpha', layerType: LAYER } as Entity,
    { id: 'b', name: 'Bravo', layerType: LAYER } as Entity,
  ];
  const observations = [
    { entityId: 'a', position: { lat: 1, lon: 2 }, timestamp: '2026-06-29T12:00:00Z' },
    { entityId: 'b', position: { lat: 3, lon: 4 }, timestamp: '2026-06-29T12:00:00Z' },
  ] as unknown as Observation[];
  useUIStore.getState().setLayerEntities(LAYER, { entities, observations });
  useUIStore.getState().setActiveCluster({
    clusterId: 'c1',
    layerId: LAYER,
    entityIds: ['a', 'b'],
    count: 2,
    truncated: false,
    screenX: 0,
    screenY: 0,
  });
}

describe('MobileClusterPanel (shared MobileDrawer)', () => {
  beforeEach(() => {
    useUIStore.getState().clearActiveCluster();
    useUIStore.getState().clearLayerEntities(LAYER);
    useUIStore.getState().clearSelection();
  });

  it('renders no member content when there is no active cluster', () => {
    render(<MobileClusterPanel />);
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
  });

  it('renders the member list via the shared drawer when a cluster is active', () => {
    activate();
    render(<MobileClusterPanel />);
    // Uses the shared MobileDrawer (testid derived from the "Cluster" title).
    expect(screen.getByTestId('mobile-drawer-cluster')).toBeInTheDocument();
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Bravo')).toBeInTheDocument();
  });

  it('closes via the shared drawer close button (clears active cluster)', () => {
    activate();
    render(<MobileClusterPanel />);
    fireEvent.click(screen.getByTestId('mobile-drawer-close-cluster'));
    expect(useUIStore.getState().activeCluster).toBeNull();
  });

  it('selecting a member selects the entity and dismisses the drawer', () => {
    activate();
    render(<MobileClusterPanel />);
    fireEvent.click(screen.getByText('Alpha'));
    expect(useUIStore.getState().selectedEntityId).toBe('a');
    expect(useUIStore.getState().activeCluster).toBeNull();
  });
});
