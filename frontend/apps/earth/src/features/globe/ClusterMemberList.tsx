/**
 * ClusterMemberList — the list of entities inside a clicked cluster. Shared by
 * the desktop floating panel and the mobile drawer. Resolves names + ages from
 * the in-memory store (no round trip). Selecting a row opens that entity's detail
 * and closes the cluster list.
 */
import React, { useMemo } from 'react';
import { Box, Typography } from '@mui/material';
import { useUIStore } from '@/app/store';
import { formatRelativeAge } from './clusterRendererUtils';

export interface ClusterMemberListProps {
  layerId: string;
  /** Member ids to display (already capped at MAX_CLUSTER_IDS upstream). */
  entityIds: string[];
  /** True bin size; may exceed entityIds.length. */
  count: number;
  /** Layer working set was at capacity → list reflects a loaded subset. */
  truncated: boolean;
  /** Called after a row is selected (e.g. to close a mobile drawer). */
  onSelect?: () => void;
}

interface MemberRow {
  id: string;
  name: string;
  age: string;
}

export const ClusterMemberList: React.FC<ClusterMemberListProps> = ({
  layerId,
  entityIds,
  count,
  truncated,
  onSelect,
}) => {
  const setSelectedEntity = useUIStore((s) => s.setSelectedEntity);
  const clearActiveCluster = useUIStore((s) => s.clearActiveCluster);
  // Reactive change signal — the store mutates layerEntities Maps in place.
  const version = useUIStore((s) => s.layerVersions[layerId] ?? 0);

  const rows = useMemo<MemberRow[]>(() => {
    const data = useUIStore.getState().layerEntities.get(layerId);
    const now = Date.now();
    return entityIds.map((id) => {
      const entity = data?.entityMap.get(id);
      const obs = data?.obsMap.get(id);
      return { id, name: entity?.name || id, age: formatRelativeAge(obs?.timestamp, now) };
    });
    // version drives recomputation as observations update in place.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [layerId, version, entityIds]);

  const handleSelect = (id: string) => {
    setSelectedEntity(id, layerId);
    clearActiveCluster();
    onSelect?.();
  };

  const hiddenCount = Math.max(0, count - entityIds.length);

  return (
    <Box data-testid="cluster-member-list">
      <Box
        sx={{
          display: 'flex',
          alignItems: 'baseline',
          justifyContent: 'space-between',
          mb: 0.5,
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 700,
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            color: 'text.secondary',
            fontSize: '0.6rem',
          }}
        >
          {count} {count === 1 ? 'entity' : 'entities'}
        </Typography>
        {truncated && (
          <Typography
            data-testid="cluster-truncated-indicator"
            variant="caption"
            sx={{ color: 'warning.main', fontSize: '0.6rem' }}
          >
            loaded subset
          </Typography>
        )}
      </Box>

      <Box sx={{ maxHeight: 320, overflowY: 'auto' }}>
        {rows.map((row) => (
          <Box
            key={row.id}
            component="button"
            type="button"
            data-testid="cluster-member-row"
            onClick={() => handleSelect(row.id)}
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 1,
              width: '100%',
              minHeight: 44,
              px: 1,
              py: 0.5,
              border: 'none',
              borderRadius: 1,
              background: 'transparent',
              color: 'text.primary',
              cursor: 'pointer',
              textAlign: 'left',
              font: 'inherit',
              '&:hover': { background: 'rgba(255, 255, 255, 0.06)' },
            }}
          >
            <Typography variant="body2" noWrap sx={{ color: 'text.primary', flex: 1, minWidth: 0 }}>
              {row.name}
            </Typography>
            {row.age && (
              <Typography variant="caption" sx={{ color: 'text.secondary', flexShrink: 0 }}>
                {row.age}
              </Typography>
            )}
          </Box>
        ))}

        {hiddenCount > 0 && (
          <Typography
            data-testid="cluster-more-indicator"
            variant="caption"
            sx={{ display: 'block', color: 'text.secondary', px: 1, py: 0.5, fontStyle: 'italic' }}
          >
            +{hiddenCount} more (zoom in to separate)
          </Typography>
        )}
      </Box>
    </Box>
  );
};
