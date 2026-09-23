/**
 * MediaSection — renders the media a layer declares for one entity.
 *
 * The component chooses what to mount from the declared `kind` alone. There is
 * no provider name, layer-name prefix or URL suffix anywhere in here: a new
 * camera or radio source is onboarded in YAML, not in this file.
 */

import React, { useEffect, useRef, useState } from 'react';
import { Box, Button, IconButton, Tooltip, Typography } from '@mui/material';
import { Camera, Pause, Play, RefreshCw } from 'lucide-react';
import { alpha, theme, DASHBOARD_TYPOGRAPHY, formatRelativeTime } from '@respondent/core';
import type { MediaConfig, SnapshotOptions } from '@respondent/core';
import { useUIStore } from '@/app/store';
import SectionHeader from '../entity/tabs/shared/SectionHeader';
import { useMediaContext } from './MediaProvider';
import { resolveMedia, type ResolvedMedia } from './mediaUrls';
import { SnapshotSession } from './snapshotSession';

export interface MediaSectionProps {
  entityId: string;
  layerType: string;
  entityName: string;
  /** Entity metadata plus latest-observation metadata, already merged. */
  metadata: Record<string, string>;
  /** False while the panel is minimized or the overview is not on screen. */
  active: boolean;
}

/** The metadata keys a layer's media declarations consume. */
export function mediaMetadataKeys(media: MediaConfig[] | undefined): Set<string> {
  const keys = new Set<string>();
  for (const config of media ?? []) {
    keys.add(config.urlKey);
    if (config.attributionKey) keys.add(config.attributionKey);
  }
  return keys;
}

const captionSx = {
  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
  color: 'text.secondary',
} as const;

const MediaSection: React.FC<MediaSectionProps> = ({
  entityId,
  layerType,
  entityName,
  metadata,
  active,
}) => {
  const media = useUIStore((s) => s.layers[layerType]?.displayConfig?.media);
  const ctx = useMediaContext();

  if (!ctx || !media || media.length === 0) return null;
  const resolved = resolveMedia(media, metadata);
  if (resolved.length === 0) return null;

  return (
    <Box data-testid="media-section">
      <SectionHeader icon={<Camera size={14} />} title="Media" />
      {resolved.map((item) =>
        item.config.kind === 'snapshot' ? (
          <SnapshotCard
            key={item.config.id}
            entityId={entityId}
            layerType={layerType}
            item={item}
            active={active}
          />
        ) : (
          <AudioLauncher
            key={item.config.id}
            entityId={entityId}
            layerType={layerType}
            entityName={entityName}
            item={item}
          />
        ),
      )}
    </Box>
  );
};

// ─── Snapshot ────────────────────────────────────────────────────────────────

interface SnapshotCardProps {
  entityId: string;
  layerType: string;
  item: ResolvedMedia;
  active: boolean;
}

const SnapshotCard: React.FC<SnapshotCardProps> = ({ entityId, layerType, item, active }) => {
  const ctx = useMediaContext()!;
  const slotKey = `${entityId}:${item.config.id}`;
  const options = item.config.kind === 'snapshot' ? item.config.snapshot : undefined;

  const containerRef = useRef<HTMLDivElement | null>(null);
  const [onScreen, setOnScreen] = useState(false);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState<{
    src: string;
    at: number | null;
    stale: boolean;
    loading: boolean;
  }>({
    src: '',
    at: null,
    stale: false,
    loading: false,
  });

  const timeMode = useUIStore((s) => s.timeMode);
  const layerEnabled = useUIStore((s) => s.enabledLayers.includes(layerType));

  // Always observe, even while inactive: the panel may be expanded later and
  // the section must already know whether it is on screen.
  useEffect(() => {
    const node = containerRef.current;
    if (!node || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver((entries) => {
      setOnScreen(entries.some((entry) => entry.isIntersecting));
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  // Historical exploration describes catalog metadata, not archived frames, so
  // a camera never fetches while the user is looking at the past.
  const showable =
    active &&
    onScreen &&
    layerEnabled &&
    ctx.documentVisible &&
    timeMode === 'live' &&
    item.url !== null &&
    options !== undefined;
  // A paused camera keeps the slot: it is still the one the user chose, so it
  // offers Resume rather than reverting to the hand-off affordance.
  const eligible = showable && !paused;

  const { claimSnapshot, releaseSnapshot, snapshotOwner } = ctx;
  useEffect(() => {
    if (showable) claimSnapshot(slotKey);
    else releaseSnapshot(slotKey);
  }, [showable, slotKey, claimSnapshot, releaseSnapshot]);

  useEffect(() => () => releaseSnapshot(slotKey), [slotKey, releaseSnapshot]);

  const owns = snapshotOwner === slotKey;
  const url = item.url;
  useEffect(() => {
    if (!eligible || !owns || !url || !options) return;
    const session = new SnapshotSession(url, options as SnapshotOptions);
    const sync = () => {
      const state = session.getState();
      setFrame({
        src: state.image?.getAttribute('src') ?? '',
        at: state.lastLoaded,
        stale: state.stale,
        loading: state.loading,
      });
    };
    const unsubscribe = session.subscribe(sync);
    session.start();
    return () => {
      unsubscribe();
      session.dispose();
      setFrame({ src: '', at: null, stale: false, loading: false });
    };
    // Keyed on the option values rather than the object: the layer config is
    // refetched periodically and a new identity must not restart the camera.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [eligible, owns, url, options?.refreshIntervalSeconds, options?.cacheBustParam]);

  const unavailable = item.url === null;
  const label = item.config.label;

  return (
    <Box ref={containerRef} sx={{ mb: 1 }} data-testid={`media-snapshot-${item.config.id}`}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
        <Typography variant="caption" sx={{ ...captionSx, color: 'text.primary', fontWeight: 600 }}>
          {label}
        </Typography>
        {!unavailable &&
          (owns ? (
            <Tooltip title={paused ? 'Resume refresh' : 'Pause refresh'} placement="top" arrow>
              <IconButton
                size="small"
                aria-label={paused ? `Resume ${label}` : `Pause ${label}`}
                onClick={() => setPaused((p) => !p)}
              >
                {paused ? <Play size={12} /> : <Pause size={12} />}
              </IconButton>
            </Tooltip>
          ) : (
            <Button
              size="small"
              variant="text"
              startIcon={<RefreshCw size={12} />}
              onClick={() => {
                setPaused(false);
                claimSnapshot(slotKey, true);
              }}
            >
              View camera
            </Button>
          ))}
      </Box>

      <Box
        sx={{
          mt: 0.5,
          position: 'relative',
          borderRadius: 1,
          overflow: 'hidden',
          border: `1px solid ${alpha(theme.palette.primary.main, 0.15)}`,
          backgroundColor: alpha(theme.palette.common.black, 0.35),
          aspectRatio: '16 / 9',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {frame.src ? (
          <Box
            component="img"
            src={frame.src}
            alt={label}
            referrerPolicy="no-referrer"
            sx={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }}
          />
        ) : (
          <Typography variant="caption" sx={captionSx}>
            {idleReason({
              unavailable,
              paused,
              owns,
              heldByAnother: snapshotOwner !== null && snapshotOwner !== slotKey,
              layerEnabled,
              documentVisible: ctx.documentVisible,
              historical: timeMode !== 'live',
              loading: frame.loading,
            })}
          </Typography>
        )}
        {frame.stale && frame.src && (
          <Box
            sx={{
              position: 'absolute',
              top: 4,
              right: 4,
              px: 0.5,
              borderRadius: 0.5,
              backgroundColor: alpha(theme.palette.warning.main, 0.85),
            }}
          >
            <Typography
              variant="caption"
              sx={{ fontSize: 9, color: 'common.black', fontWeight: 700 }}
            >
              STALE
            </Typography>
          </Box>
        )}
      </Box>

      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 1, mt: 0.25 }}>
        <Typography variant="caption" sx={captionSx}>
          {item.attribution}
        </Typography>
        {frame.at !== null && (
          // The receipt time, not the camera's capture time — we only know when
          // the frame arrived here.
          <Typography variant="caption" sx={captionSx}>
            Last loaded {formatRelativeTime(frame.at)}
          </Typography>
        )}
      </Box>
    </Box>
  );
};

/**
 * Says why no frame is on screen. The reasons are distinct on purpose: "another
 * camera is live" is only true when some other panel actually holds the slot,
 * and telling a user that while their tab is simply in the background would be
 * a lie they cannot act on.
 */
function idleReason(s: {
  unavailable: boolean;
  paused: boolean;
  owns: boolean;
  heldByAnother: boolean;
  layerEnabled: boolean;
  documentVisible: boolean;
  historical: boolean;
  loading: boolean;
}): string {
  if (s.unavailable) return 'Camera unavailable';
  if (s.paused) return 'Paused';
  if (s.heldByAnother) return 'Paused — another camera is live';
  if (!s.layerEnabled) return 'Paused — this layer is switched off';
  if (s.historical) return 'Paused — live frames resume in live mode';
  if (!s.documentVisible) return 'Paused while this tab is in the background';
  if (!s.owns) return 'Paused';
  return s.loading ? 'Loading…' : 'Waiting for the first frame';
}

// ─── Audio ───────────────────────────────────────────────────────────────────

interface AudioLauncherProps {
  entityId: string;
  layerType: string;
  entityName: string;
  item: ResolvedMedia;
}

const AudioLauncher: React.FC<AudioLauncherProps> = ({ entityId, layerType, entityName, item }) => {
  const { audio } = useMediaContext()!;
  const label = item.config.label;

  return (
    <Box
      sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}
      data-testid={`media-audio-${item.config.id}`}
    >
      <Button
        size="small"
        variant="outlined"
        startIcon={<Play size={12} />}
        aria-label={`Play ${label}`}
        disabled={item.url === null}
        // Native playback only ever starts from this gesture.
        onClick={() =>
          item.url &&
          audio.play({
            entityId,
            mediaId: item.config.id,
            layerId: layerType,
            name: entityName,
            attribution: item.attribution,
            url: item.url,
            playbackAction: item.config.playbackAction,
          })
        }
      >
        {label}
      </Button>
      {item.url === null && (
        <Typography variant="caption" sx={captionSx}>
          Stream unavailable
        </Typography>
      )}
    </Box>
  );
};

export default MediaSection;
