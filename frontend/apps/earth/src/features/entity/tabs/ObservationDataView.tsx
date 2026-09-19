/**
 * ObservationDataView — pure data renderer for observation history.
 *
 * Receives viewMode from the parent (TimelineTab) and renders accordingly.
 * Does NOT own view mode state, copy state, or a toolbar — those live in TimelineTab.
 *
 * Single observation: section header + timestamp + position + observation
 * metadata fields as FieldRow entries.
 *
 * Multiple observations: renders table or chart based on the viewMode prop,
 * with a "Load more" button when hasMore is true.
 *
 * Does NOT render entity metadata (source, country, url, author) — that
 * lives in OverviewTab.
 */

import React, { useMemo } from 'react';
import { Box, Typography, Button } from '@mui/material';
import type { TrailPoint } from '@respondent/core';
import { DASHBOARD_TYPOGRAPHY, formatUtcTime } from '@respondent/core';
import ObservationChart from './ObservationChart';
import ObservationTable from './shared/ObservationTable';

export interface ObservationDataViewProps {
  points: TrailPoint[];
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore: () => void;
  viewMode: 'table' | 'chart';
}

// ─── Helpers (exported for use in TimelineTab) ────────────────────────────────

/** Collect every metadata key that appears across all observations. Sorted alphabetically. */
export function collectMetadataKeys(points: TrailPoint[]): string[] {
  const keys = new Set<string>();
  for (const p of points) {
    if (p.metadata) {
      for (const k of Object.keys(p.metadata)) keys.add(k);
    }
  }
  return Array.from(keys).sort();
}

export function buildCsvString(points: TrailPoint[], metaKeys: string[]): string {
  const headers = ['time', 'lat', 'lon', 'alt_m', ...metaKeys];
  const rows = [...points].reverse().map((p) => {
    const cells = [
      formatUtcTime(p.ts),
      p.lat.toFixed(3),
      p.lon.toFixed(3),
      String(Math.round(p.altitudeM)),
      ...metaKeys.map((k) => {
        const v = String(p.metadata?.[k] ?? '');
        return v.includes(',') ? `"${v}"` : v;
      }),
    ];
    return cells.join(',');
  });
  return [headers.join(','), ...rows].join('\n');
}

export function buildJsonString(points: TrailPoint[], metaKeys: string[]): string {
  const rows = [...points].reverse().map((p) => {
    const obj: Record<string, unknown> = {
      time: formatUtcTime(p.ts),
      lat: p.lat,
      lon: p.lon,
      alt_m: Math.round(p.altitudeM),
    };
    for (const k of metaKeys) {
      obj[k] = p.metadata?.[k] ?? null;
    }
    return obj;
  });
  return JSON.stringify(rows, null, 2);
}

export function buildMarkdownString(points: TrailPoint[], metaKeys: string[]): string {
  const headers = [
    'Time',
    'Lat',
    'Lon',
    'Alt (m)',
    ...metaKeys.map((k) => k.replace(/_/g, ' ').toUpperCase()),
  ];
  const separator = headers.map(() => '---');
  const rows = [...points]
    .reverse()
    .map((p) => [
      formatUtcTime(p.ts),
      p.lat.toFixed(3),
      p.lon.toFixed(3),
      String(Math.round(p.altitudeM)),
      ...metaKeys.map((k) => String(p.metadata?.[k] ?? '')),
    ]);

  const lines = [
    `| ${headers.join(' | ')} |`,
    `| ${separator.join(' | ')} |`,
    ...rows.map((r) => `| ${r.join(' | ')} |`),
  ];
  return lines.join('\n');
}

// ─── Main Component ───────────────────────────────────────────────────────────

const ObservationDataView: React.FC<ObservationDataViewProps> = ({
  points,
  hasMore,
  isLoading,
  onLoadMore,
  viewMode,
}) => {
  const metaKeys = useMemo(() => collectMetadataKeys(points), [points]);
  // ObservationTable expects newest-first order
  const newestFirst = useMemo(() => [...points].reverse(), [points]);

  if (points.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
          }}
        >
          No observation data available
        </Typography>
      </Box>
    );
  }

  // Render based on viewMode — same behavior for 1 or N observations
  return (
    <Box
      data-testid={points.length === 1 ? 'observation-single' : 'observation-multi'}
      sx={{ display: 'flex', flexDirection: 'column', height: '100%', gap: 1 }}
    >
      {/* Content */}
      {viewMode === 'table' ? (
        <ObservationTable points={newestFirst} metaKeys={metaKeys} />
      ) : (
        <ObservationChart points={points} />
      )}

      {/* Load more */}
      {hasMore && (
        <Button
          data-testid="load-more-observations"
          size="small"
          onClick={onLoadMore}
          disabled={isLoading}
          variant="outlined"
          sx={{
            flexShrink: 0,
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            color: 'primary.main',
            borderColor: 'rgba(0, 255, 157, 0.2)',
            textTransform: 'uppercase',
            letterSpacing: '0.1em',
          }}
        >
          {isLoading ? 'Loading...' : 'Load More'}
        </Button>
      )}
    </Box>
  );
};

export default ObservationDataView;
