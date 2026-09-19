import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ClusterListPanel } from './ClusterListPanel';
import { useUIStore } from '@/app/store';
import type { Entity, Observation } from '@respondent/core';

const LAYER = 'flights';

function activate() {
  const entities: Entity[] = [{ id: 'a', name: 'Alpha', layerType: LAYER } as Entity];
  const observations = [
    { entityId: 'a', position: { lat: 1, lon: 2 }, timestamp: '2026-06-29T12:00:00Z' },
  ] as unknown as Observation[];
  useUIStore.getState().setLayerEntities(LAYER, { entities, observations });
  useUIStore.getState().setActiveCluster({
    clusterId: 'c1',
    layerId: LAYER,
    entityIds: ['a'],
    count: 1,
    truncated: false,
    screenX: 100,
    screenY: 100,
  });
}

describe('ClusterListPanel', () => {
  beforeEach(() => {
    useUIStore.getState().clearActiveCluster();
    useUIStore.getState().clearLayerEntities(LAYER);
  });

  it('renders nothing when there is no active cluster', () => {
    render(<ClusterListPanel />);
    expect(screen.queryByTestId('cluster-list-panel')).not.toBeInTheDocument();
  });

  it('renders the member list when a cluster is active', () => {
    activate();
    render(<ClusterListPanel />);
    expect(screen.getByTestId('cluster-list-panel')).toBeInTheDocument();
    expect(screen.getByText('Alpha')).toBeInTheDocument();
  });

  it('closes on the ConfigPanel header close button', () => {
    activate();
    render(<ClusterListPanel />);
    fireEvent.click(screen.getByTestId('config-panel-close'));
    expect(useUIStore.getState().activeCluster).toBeNull();
  });

  it('renders inside the shared ConfigPanel with a draggable header', () => {
    activate();
    render(<ClusterListPanel />);
    // ConfigPanel header is what enables drag-by-header + consistent styling.
    expect(screen.getByTestId('config-panel-header')).toBeInTheDocument();
    expect(screen.getByTestId('config-panel-title')).toHaveTextContent('Cluster');
  });

  it('closes on Escape', () => {
    activate();
    render(<ClusterListPanel />);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(useUIStore.getState().activeCluster).toBeNull();
  });
});
