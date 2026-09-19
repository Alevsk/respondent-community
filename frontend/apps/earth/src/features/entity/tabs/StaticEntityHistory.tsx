/**
 * StaticEntityHistory — Summary card + observation changelog for stationary entities.
 *
 * Renders a compact position line and resolved metadata fields at the top,
 * followed by a chronological changelog where changed field values are
 * highlighted in primary color.
 */

import React, { useMemo, useState } from 'react';
import { Box, Typography, Button, Skeleton } from '@mui/material';
import type { TrailPoint } from '../hooks/useEntityTrail';
import type { EntityDetailResponse } from '../../../shared/api/queries';
import { resolveFields, type ResolvedField } from './overview/fieldRenderers';
import FieldRow from './shared/FieldRow';
import { formatUtcTime, formatUtcDate, alpha, theme, semanticColors } from '@respondent/core';

export interface StaticEntityHistoryProps {
  entityId: string;
  layerType: string;
  detail: EntityDetailResponse | undefined;
  points: TrailPoint[];
  hasMore: boolean;
  isLoading: boolean;
  onLoadMore: () => void;
}

/** Format position as a compact single line: "16.345°S, 70.897°W · 5,608m" */
function formatPosition(lat: number, lon: number, altitudeM: number): string {
  const latDir = lat >= 0 ? 'N' : 'S';
  const lonDir = lon >= 0 ? 'E' : 'W';
  const latStr = `${Math.abs(lat).toFixed(3)}°${latDir}`;
  const lonStr = `${Math.abs(lon).toFixed(3)}°${lonDir}`;
  const altStr = `${Math.round(altitudeM).toLocaleString()}m`;
  return `${latStr}, ${lonStr} · ${altStr}`;
}

interface ChangelogEntry {
  point: TrailPoint;
  fields: ResolvedField[];
  /** Set of field labels whose value changed from the previous (older) observation. */
  changedLabels: Set<string>;
}

const StaticEntityHistory: React.FC<StaticEntityHistoryProps> = ({
  layerType,
  detail,
  points,
  hasMore,
  isLoading,
  onLoadMore,
}) => {
  const entityMetadata = detail?.entity?.metadata ?? {};

  // Build summary card fields from latest observation merged with entity metadata
  const summaryFields = useMemo(() => {
    if (points.length === 0) return [];
    const latest = points[points.length - 1]; // oldest-first, so last = newest
    const merged = { ...entityMetadata, ...(latest.metadata ?? {}) };
    return resolveFields(merged, layerType);
  }, [points, entityMetadata, layerType]);

  // Build changelog entries with change detection
  const changelog = useMemo(() => {
    const entries: ChangelogEntry[] = [];

    for (let i = 0; i < points.length; i++) {
      const point = points[i];
      const merged = { ...entityMetadata, ...(point.metadata ?? {}) };
      const fields = resolveFields(merged, layerType);

      const changedLabels = new Set<string>();
      if (i > 0) {
        const prevPoint = points[i - 1];
        const prevMerged = { ...entityMetadata, ...(prevPoint.metadata ?? {}) };
        const prevFields = resolveFields(prevMerged, layerType);

        // Build a map of previous field values by label
        const prevValueMap = new Map<string, string>();
        for (const f of prevFields) {
          prevValueMap.set(f.label, f.value);
        }

        // Compare current fields to previous
        for (const f of fields) {
          const prevValue = prevValueMap.get(f.label);
          if (prevValue !== undefined && prevValue !== f.value) {
            changedLabels.add(f.label);
          } else if (prevValue === undefined) {
            // New field that didn't exist in previous observation
            changedLabels.add(f.label);
          }
        }
      }

      entries.push({ point, fields, changedLabels });
    }

    // Reverse for newest-first display
    return entries.slice().reverse();
  }, [points, entityMetadata, layerType]);

  // Latest point for position display
  const latestPoint = points.length > 0 ? points[points.length - 1] : null;

  const [selectedTs, setSelectedTs] = useState<number | null>(null);

  if (isLoading && points.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        {[...Array(5)].map((_, i) => (
          <Skeleton key={i} variant="rectangular" height={40} sx={{ mb: 0.5, borderRadius: 1 }} />
        ))}
      </Box>
    );
  }

  if (points.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography variant="caption" sx={{ color: 'text.secondary', fontSize: '0.6rem' }}>
          No observation history available
        </Typography>
      </Box>
    );
  }

  return (
    <Box data-testid="static-entity-history">
      {/* Summary Card */}
      <Box sx={{ mb: 1.5 }}>
        {/* Location section */}
        {latestPoint && (
          <>
            <Typography
              variant="caption"
              sx={{
                fontSize: '0.55rem',
                color: 'text.secondary',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                mb: 0.5,
                display: 'block',
              }}
            >
              LOCATION
            </Typography>
            <Typography
              data-testid="static-history-position"
              variant="caption"
              sx={{
                fontSize: '0.55rem',
                fontFamily: 'monospace',
                color: 'text.primary',
                display: 'block',
                mb: 1,
              }}
            >
              {formatPosition(latestPoint.lat, latestPoint.lon, latestPoint.altitudeM)}
            </Typography>
          </>
        )}

        {/* Event summary fields */}
        {summaryFields.length > 0 && (
          <Box>
            <Typography
              variant="caption"
              sx={{
                fontSize: '0.55rem',
                color: 'text.secondary',
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
                mb: 0.5,
                display: 'block',
              }}
            >
              EVENT SUMMARY
            </Typography>
            {summaryFields.map((field) => (
              <FieldRow
                key={field.label}
                variant="overview"
                label={field.label}
                value={field.value}
              />
            ))}
          </Box>
        )}
      </Box>

      {/* Divider */}
      <Box sx={{ borderBottom: '1px solid rgba(255, 255, 255, 0.06)', mb: 1 }} />

      {/* Observation Log Header */}
      <Typography
        variant="caption"
        sx={{
          fontSize: '0.55rem',
          color: 'text.secondary',
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          mb: 1,
          display: 'block',
        }}
      >
        OBSERVATION LOG
      </Typography>

      {/* Changelog */}
      <Box sx={{ position: 'relative' }}>
        {/* Timeline spine */}
        <Box
          sx={{
            position: 'absolute',
            left: 10,
            top: 0,
            bottom: 0,
            width: '1px',
            bgcolor: 'text.secondary',
            opacity: 0.3,
          }}
        />

        {changelog.map((entry, idx) => (
          <Box
            key={`${entry.point.ts}-${idx}`}
            data-testid={
              selectedTs === entry.point.ts ? 'changelog-entry-selected' : 'changelog-entry'
            }
            onClick={() =>
              setSelectedTs((prev) => (prev === entry.point.ts ? null : entry.point.ts))
            }
            sx={{
              position: 'relative',
              pl: 3,
              pr: 0.5,
              py: 0.75,
              cursor: 'pointer',
              borderRadius: 1,
              ...(selectedTs === entry.point.ts && {
                borderLeft: '2px solid',
                borderLeftColor: 'primary.main',
                bgcolor: alpha(theme.palette.primary.main, 0.04),
              }),
            }}
          >
            {/* Spine dot */}
            <Box
              sx={{
                position: 'absolute',
                left: 7,
                top: 12,
                width: 5,
                height: 5,
                borderRadius: '50%',
                bgcolor: 'text.secondary',
              }}
            />

            {/* Time + date row */}
            <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
              <Typography
                variant="caption"
                sx={{
                  fontSize: '0.55rem',
                  fontFamily: 'monospace',
                  color: 'primary.main',
                  fontWeight: 600,
                }}
              >
                {formatUtcDate(entry.point.ts)}
              </Typography>
              <Typography variant="caption" sx={{ fontSize: '0.5rem', color: 'text.secondary' }}>
                {formatUtcTime(entry.point.ts)}
              </Typography>
            </Box>

            {/* Field values with change highlighting */}
            {entry.fields.map((field) => {
              const isChanged = entry.changedLabels.has(field.label);
              return (
                <Box
                  key={field.label}
                  data-testid={isChanged ? 'field-value-changed' : 'field-value-unchanged'}
                  sx={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    py: 0.25,
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{
                      fontSize: '0.5rem',
                      color: 'text.secondary',
                      letterSpacing: '0.05em',
                      textTransform: 'uppercase',
                    }}
                  >
                    {field.label}
                  </Typography>
                  <Typography
                    variant="caption"
                    sx={{
                      fontSize: '0.55rem',
                      fontFamily: 'monospace',
                      color: isChanged ? 'primary.main' : 'text.secondary',
                      fontWeight: isChanged ? 600 : 400,
                    }}
                  >
                    {field.value}
                  </Typography>
                </Box>
              );
            })}
          </Box>
        ))}

        {/* Load More */}
        {hasMore && (
          <Button
            size="small"
            onClick={onLoadMore}
            disabled={isLoading}
            sx={{
              mt: 1,
              ml: 3,
              width: 'calc(100% - 24px)',
              fontSize: '0.6rem',
              color: 'primary.main',
              borderColor: semanticColors.primary.alpha20,
              textTransform: 'uppercase',
              letterSpacing: '0.1em',
            }}
            variant="outlined"
          >
            {isLoading ? 'Loading...' : 'Load More'}
          </Button>
        )}
      </Box>
    </Box>
  );
};

export default StaticEntityHistory;
