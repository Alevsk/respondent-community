import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ClusterMemberList } from './ClusterMemberList';
import { useUIStore } from '@/app/store';
import type { Entity, Observation } from '@respondent/core';

const LAYER = 'flights';

function seed() {
  const entities: Entity[] = [
    { id: 'a', name: 'Alpha', layerType: LAYER } as Entity,
    { id: 'b', name: 'Bravo', layerType: LAYER } as Entity,
  ];
  const observations = [
    { entityId: 'a', position: { lat: 1, lon: 2 }, timestamp: '2026-06-29T12:00:00Z' },
    { entityId: 'b', position: { lat: 3, lon: 4 }, timestamp: '2026-06-29T12:00:00Z' },
  ] as unknown as Observation[];
  useUIStore.getState().setLayerEntities(LAYER, { entities, observations });
}

describe('ClusterMemberList', () => {
  beforeEach(() => {
    useUIStore.getState().clearLayerEntities(LAYER);
    useUIStore.getState().clearActiveCluster();
    useUIStore.getState().clearSelection();
    seed();
  });

  it('renders a row per member resolved from the store', () => {
    render(
      <ClusterMemberList layerId={LAYER} entityIds={['a', 'b']} count={2} truncated={false} />,
    );
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Bravo')).toBeInTheDocument();
    expect(screen.getAllByTestId('cluster-member-row')).toHaveLength(2);
  });

  it('selecting a row opens entity detail and clears the active cluster', () => {
    const onSelect = vi.fn();
    useUIStore.getState().setActiveCluster({
      clusterId: 'c1',
      layerId: LAYER,
      entityIds: ['a', 'b'],
      count: 2,
      truncated: false,
      screenX: 0,
      screenY: 0,
    });
    render(
      <ClusterMemberList
        layerId={LAYER}
        entityIds={['a', 'b']}
        count={2}
        truncated={false}
        onSelect={onSelect}
      />,
    );
    fireEvent.click(screen.getByText('Alpha'));
    expect(useUIStore.getState().selectedEntityId).toBe('a');
    expect(useUIStore.getState().activeCluster).toBeNull();
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('shows a "+N more" affordance when count exceeds the shown ids', () => {
    render(
      <ClusterMemberList layerId={LAYER} entityIds={['a', 'b']} count={150} truncated={false} />,
    );
    expect(screen.getByTestId('cluster-more-indicator')).toHaveTextContent('+148 more');
  });

  it('shows a loaded-subset indicator when truncated', () => {
    render(<ClusterMemberList layerId={LAYER} entityIds={['a', 'b']} count={2} truncated />);
    expect(screen.getByTestId('cluster-truncated-indicator')).toBeInTheDocument();
  });

  it('falls back to the entity id when no name is present', () => {
    render(<ClusterMemberList layerId={LAYER} entityIds={['ghost']} count={1} truncated={false} />);
    expect(screen.getByText('ghost')).toBeInTheDocument();
  });
});
