import React, { useState, useCallback } from 'react';
import { Box, Typography, Button, CircularProgress, Divider } from '@mui/material';
import { NotificationsIcon, iconSizes } from '../icons';
import ConfigPanel, { type ConfigPanelDisplayMode } from '../ui/ConfigPanel';
import MobileDrawer from '../ui/MobileDrawer';
import NotificationItem from './NotificationItem';
import { NotificationFilterToggle, NotificationFilterPanel } from './NotificationFilterBar';
import { useNotificationFilterOptions } from './useNotificationFilterOptions';
import { useResponsive, api, endpoints, semanticColors } from '@respondent/core';
import type { AIInsightNotification } from '@respondent/core';
import { useUIStore } from '@/app/store';

/** Camera altitude offset above entity — matches WatchlistBar.FLY_TO_OFFSET_M. */
const FLY_TO_OFFSET_M = 25_000;

interface NotificationPanelProps {
  loadMore: () => void;
}

const NotificationPanelContent: React.FC<{
  loadMore: () => void;
  isMobile: boolean;
  onEntityClick: (
    entityId: string,
    layerType: string | undefined,
    notification: AIInsightNotification,
  ) => void;
  onObservationClick: (observationId: string, layerType?: string) => void;
}> = ({ loadMore, isMobile, onEntityClick, onObservationClick }) => {
  const notifications = useUIStore((s) => s.notifications);
  const readInsightIds = useUIStore((s) => s.readInsightIds);
  const unreadCount = useUIStore((s) => s.unreadCount);
  const notificationsTotalCount = useUIStore((s) => s.notificationsTotalCount);
  const notificationsLoading = useUIStore((s) => s.notificationsLoading);
  const markAllRead = useUIStore((s) => s.markAllRead);
  const { data: filterOptions } = useNotificationFilterOptions();

  const [filterOpen, setFilterOpen] = useState(false);
  const handleFilterToggle = useCallback(() => setFilterOpen((prev) => !prev), []);

  const hasMore = notifications.length < notificationsTotalCount;

  return (
    <>
      {/* Sticky header: unread count + mark all read + filter toggle + filter panel */}
      <Box
        sx={{
          position: 'sticky',
          top: 0,
          zIndex: 2,
          bgcolor: 'rgba(10, 10, 10, 0.95)',
          backdropFilter: 'blur(8px)',
          mx: -1.5,
        }}
      >
        <Box
          sx={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            px: 1.5,
            py: 1,
          }}
        >
          <Typography
            sx={{
              fontSize: '0.7rem',
              fontFamily: 'monospace',
              color: 'primary.main',
              letterSpacing: '0.08em',
            }}
          >
            {notifications.length === 0
              ? 'NO NOTIFICATIONS'
              : unreadCount > 0
                ? `${unreadCount} UNREAD`
                : 'ALL READ'}
          </Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            {unreadCount > 0 && (
              <Button
                data-testid="notif-mark-all-read"
                size="small"
                onClick={markAllRead}
                sx={{
                  fontSize: '0.65rem',
                  fontFamily: 'monospace',
                  letterSpacing: '0.08em',
                  color: 'primary.main',
                  textTransform: 'none',
                  minWidth: 0,
                  py: 0,
                }}
              >
                MARK ALL READ
              </Button>
            )}
            {filterOptions && (
              <NotificationFilterToggle open={filterOpen} onToggle={handleFilterToggle} />
            )}
          </Box>
        </Box>
        {/* Filter panel — inline, pushes notification list down when open */}
        {filterOpen && filterOptions && <NotificationFilterPanel options={filterOptions} />}
        <Divider sx={{ borderColor: semanticColors.primary.alpha10 }} />
      </Box>

      {/* Empty state */}
      {notifications.length === 0 && (
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', py: 4 }}>
          <Typography
            sx={{
              fontSize: '0.8rem',
              fontFamily: 'monospace',
              color: 'text.secondary',
              letterSpacing: '0.05em',
            }}
          >
            No notifications match current filters
          </Typography>
        </Box>
      )}

      {/* Notification list */}
      {notifications.map((notification) => (
        <React.Fragment key={notification.id}>
          <NotificationItem
            notification={notification}
            isRead={readInsightIds.has(notification.id)}
            isMobile={isMobile}
            onEntityClick={onEntityClick}
            onObservationClick={onObservationClick}
          />
          <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.05)' }} />
        </React.Fragment>
      ))}

      {/* Load more */}
      {hasMore && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 1.5 }}>
          {notificationsLoading ? (
            <CircularProgress size={20} sx={{ color: 'primary.main' }} />
          ) : (
            <Button
              data-testid="notif-load-more"
              size="small"
              onClick={loadMore}
              sx={{
                fontSize: '0.7rem',
                fontFamily: 'monospace',
                letterSpacing: '0.1em',
                color: 'primary.main',
                textTransform: 'none',
              }}
            >
              LOAD MORE
            </Button>
          )}
        </Box>
      )}
    </>
  );
};

const NotificationPanel: React.FC<NotificationPanelProps> = React.memo(({ loadMore }) => {
  const { isMobile } = useResponsive();
  const open = useUIStore((s) => s.notificationPanelOpen);
  const unreadCount = useUIStore((s) => s.unreadCount);
  const [displayMode, setDisplayMode] = useState<ConfigPanelDisplayMode>('normal');
  const setOpen = useUIStore((s) => s.setNotificationPanelOpen);
  const setSelectedEntity = useUIStore((s) => s.setSelectedEntity);
  const selectMultipleEntities = useUIStore((s) => s.selectMultipleEntities);

  const handleClose = useCallback(() => setOpen(false), [setOpen]);
  const handleOpen = useCallback(() => setOpen(true), [setOpen]);

  const dispatchFlyTo = useCallback(
    (
      detail:
        | { lat: number; lon: number; alt?: number }
        | { bounds: { west: number; south: number; east: number; north: number } },
    ) => {
      window.dispatchEvent(new CustomEvent('respondent:flyto', { detail }));
    },
    [],
  );

  /** Look up an observation from the store by entity ID (scans all layers). */
  const findObsInStore = useCallback((eid: string) => {
    const state = useUIStore.getState();
    for (const [, layerData] of state.layerEntities) {
      const obs = layerData.obsMap.get(eid);
      if (obs?.position) return obs;
    }
    return undefined;
  }, []);

  const handleEntityClick = useCallback(
    async (
      _entityId: string,
      layerType: string | undefined,
      notification: AIInsightNotification,
    ) => {
      // Build composite keys for ALL entities in the notification.
      const allRefs =
        notification.entities.length > 0
          ? notification.entities.map((e) => ({
              entityId: e.layerType && e.externalId ? `${e.layerType}:${e.externalId}` : e.id,
              layerId: e.layerType || layerType || '',
            }))
          : notification.entityIds.map((eid) => ({
              entityId: notification.layerType ? `${notification.layerType}:${eid}` : eid,
              layerId: notification.layerType || layerType || '',
            }));

      // Fallback to single-entity if no refs available.
      if (allRefs.length === 0) {
        const foundLayerId = layerType ?? '';
        setSelectedEntity(_entityId, foundLayerId);
        const obs = findObsInStore(_entityId);
        if (obs) {
          const flyAlt = Math.max((obs.altitudeM ?? 0) + FLY_TO_OFFSET_M, FLY_TO_OFFSET_M);
          dispatchFlyTo({ lat: obs.position.lat, lon: obs.position.lon, alt: flyAlt });
        }
        if (isMobile) handleClose();
        return;
      }

      // Select all related entities at once (brackets + watchlist).
      selectMultipleEntities(allRefs);

      // Gather positions for all entities — from store first, then REST fallback.
      // Single-pass: collect known positions and missing refs in one loop.
      const positions: Array<{ lat: number; lon: number; alt: number }> = [];
      const missingRefs: typeof allRefs = [];

      for (const ref of allRefs) {
        const obs = findObsInStore(ref.entityId);
        if (obs?.position) {
          positions.push({ lat: obs.position.lat, lon: obs.position.lon, alt: obs.altitudeM ?? 0 });
        } else {
          missingRefs.push(ref);
        }
      }
      if (missingRefs.length > 0) {
        const fetches = missingRefs.map(async (ref) => {
          try {
            const data = await api.get<{
              entity: { layerType?: string };
              latestObservation?: { position?: { lat: number; lon: number }; altitudeM?: number };
            }>(`${endpoints.entities}/detail?entity_id=${encodeURIComponent(ref.entityId)}`);
            const pos = data.latestObservation?.position;
            if (pos) {
              positions.push({
                lat: pos.lat,
                lon: pos.lon,
                alt: data.latestObservation?.altitudeM ?? 0,
              });
            }
          } catch {
            // Skip entities that can't be fetched
          }
        });
        await Promise.all(fetches);
      }

      // Fly camera to encompass all entity positions.
      if (positions.length === 1) {
        const p = positions[0];
        const flyAlt = Math.max(p.alt + FLY_TO_OFFSET_M, FLY_TO_OFFSET_M);
        dispatchFlyTo({ lat: p.lat, lon: p.lon, alt: flyAlt });
      } else if (positions.length > 1) {
        // Compute bounding box with padding.
        let west = Infinity,
          south = Infinity,
          east = -Infinity,
          north = -Infinity;
        for (const p of positions) {
          if (p.lon < west) west = p.lon;
          if (p.lon > east) east = p.lon;
          if (p.lat < south) south = p.lat;
          if (p.lat > north) north = p.lat;
        }
        // Add ~10% padding so entities aren't right at the edges.
        const lonPad = Math.max((east - west) * 0.15, 0.5);
        const latPad = Math.max((north - south) * 0.15, 0.5);
        dispatchFlyTo({
          bounds: {
            west: west - lonPad,
            south: south - latPad,
            east: east + lonPad,
            north: north + latPad,
          },
        });
      }

      if (isMobile) handleClose();
    },
    [
      selectMultipleEntities,
      setSelectedEntity,
      findObsInStore,
      isMobile,
      handleClose,
      dispatchFlyTo,
    ],
  );

  const handleObservationClick = useCallback(
    async (observationId: string) => {
      // Look up observation position from layerEntities
      const state = useUIStore.getState();
      for (const [, layerData] of state.layerEntities) {
        for (const [, obs] of layerData.obsMap) {
          if (obs.entityId === observationId && obs.position) {
            dispatchFlyTo({ lat: obs.position.lat, lon: obs.position.lon, alt: obs.altitudeM });
            if (isMobile) handleClose();
            return;
          }
        }
      }

      // Fallback: fetch observation coordinates from REST.
      try {
        const data = await api.get<{
          observation: { position?: { lat: number; lon: number }; altitudeM?: number };
        }>(`${endpoints.entities}/observations/${observationId}`);
        const pos = data.observation?.position;
        if (pos) {
          dispatchFlyTo({ lat: pos.lat, lon: pos.lon, alt: data.observation?.altitudeM });
        }
      } catch {
        // Observation not fetchable — no fly-to
      }

      if (isMobile) handleClose();
    },
    [isMobile, handleClose, dispatchFlyTo],
  );

  if (isMobile) {
    return (
      <MobileDrawer
        open={open}
        onClose={handleClose}
        onOpen={handleOpen}
        title="Notifications"
        heightPercent={70}
      >
        <NotificationPanelContent
          loadMore={loadMore}
          isMobile={true}
          onEntityClick={handleEntityClick}
          onObservationClick={handleObservationClick}
        />
      </MobileDrawer>
    );
  }

  if (!open) return null;

  return (
    <ConfigPanel
      data-testid="notification-panel"
      open={open}
      onClose={handleClose}
      title="NOTIFICATIONS"
      icon={<NotificationsIcon size={iconSizes.sm} />}
      panelId="notifications"
      minimizable
      displayMode={displayMode}
      onDisplayModeChange={setDisplayMode}
      minimizedTitle={unreadCount > 0 ? `Notifications (${unreadCount})` : 'Notifications'}
      width={360}
      top={16}
      right={200}
      sx={{ '& [data-testid="config-panel-content"]': { pt: 0 } }}
    >
      <NotificationPanelContent
        loadMore={loadMore}
        isMobile={false}
        onEntityClick={handleEntityClick}
        onObservationClick={handleObservationClick}
      />
    </ConfigPanel>
  );
});

NotificationPanel.displayName = 'NotificationPanel';

export default NotificationPanel;
