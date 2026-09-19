/**
 * MobileClusterPanel — the clicked cluster's member list on mobile, rendered via
 * the SHARED MobileDrawer (identical slide-up/down animation and styling to the
 * layers / settings / notifications drawers).
 *
 * Driven DIRECTLY by `activeCluster` store state (`open={!!activeCluster}`)
 * rather than the activeMobileDrawer / openMobileDrawer mechanism, so a Cesium
 * canvas tap opens it reliably without a useEffect bridge (the unreliable
 * Cesium-callback→effect path MobileEntityPanel's comment warns about).
 *
 * `disableBackdropClose` is essential here: the drawer is opened by a canvas
 * gesture, and the browser's touch→mouse compatibility "ghost click" ~300ms
 * after the tap lands on the Modal backdrop and would otherwise close the
 * just-opened drawer. It still closes via the close button, swipe-down, Escape,
 * or selecting a member row (which clears activeCluster).
 */
import React from 'react';
import MobileDrawer from '../../shared/ui/MobileDrawer';
import { useUIStore } from '@/app/store';
import { ClusterMemberList } from './ClusterMemberList';

const noop = () => {};

const MobileClusterPanel: React.FC = () => {
  const activeCluster = useUIStore((s) => s.activeCluster);
  const clearActiveCluster = useUIStore((s) => s.clearActiveCluster);

  return (
    <MobileDrawer
      open={!!activeCluster}
      onClose={clearActiveCluster}
      onOpen={noop}
      title="Cluster"
      heightPercent={55}
      disableBackdropClose
    >
      {activeCluster && (
        <ClusterMemberList
          layerId={activeCluster.layerId}
          entityIds={activeCluster.entityIds}
          count={activeCluster.count}
          truncated={activeCluster.truncated}
        />
      )}
    </MobileDrawer>
  );
};

export default MobileClusterPanel;
