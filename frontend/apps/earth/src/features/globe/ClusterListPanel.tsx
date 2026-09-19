/**
 * ClusterListPanel — desktop floating panel that lists a clicked cluster's
 * members, anchored near the click. Built on the shared ConfigPanel so it looks,
 * feels, and drags exactly like the Settings/Layers panels (drag-by-header +
 * click-to-focus come from ConfigPanel's `panelId` integration). Closes via the
 * header close button or Escape; clicking empty globe space also clears it (see
 * useEntityInteraction). Mobile uses a bottom drawer instead (see EarthShell),
 * so this is mounted desktop-only.
 */
import React, { useEffect } from 'react';
import ConfigPanel from '../../shared/ui/ConfigPanel';
import { GridOnIcon, iconSizes } from '../../shared/icons';
import { useUIStore } from '@/app/store';
import { ClusterMemberList } from './ClusterMemberList';

const PANEL_WIDTH = 280;
const MARGIN = 12;

export const ClusterListPanel: React.FC = () => {
  const activeCluster = useUIStore((s) => s.activeCluster);
  const clearActiveCluster = useUIStore((s) => s.clearActiveCluster);

  useEffect(() => {
    if (!activeCluster) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') clearActiveCluster();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [activeCluster, clearActiveCluster]);

  if (!activeCluster) return null;

  // Anchor near the click, clamped so the panel opens fully on-screen. The user
  // can then drag it anywhere via the header (ConfigPanel panelId integration).
  const left = Math.max(
    MARGIN,
    Math.min(activeCluster.screenX + 12, window.innerWidth - PANEL_WIDTH - MARGIN),
  );
  const top = Math.max(MARGIN, Math.min(activeCluster.screenY + 12, window.innerHeight - 160));

  return (
    <ConfigPanel
      open
      onClose={clearActiveCluster}
      title="Cluster"
      icon={<GridOnIcon size={iconSizes.sm} />}
      panelId="cluster-list"
      width={PANEL_WIDTH}
      left={left}
      top={top}
      data-testid="cluster-list-panel"
    >
      <ClusterMemberList
        layerId={activeCluster.layerId}
        entityIds={activeCluster.entityIds}
        count={activeCluster.count}
        truncated={activeCluster.truncated}
      />
    </ConfigPanel>
  );
};
