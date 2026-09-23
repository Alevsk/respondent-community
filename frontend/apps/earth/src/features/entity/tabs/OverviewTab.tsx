/**
 * OverviewTab — Entity summary view with grouped sections.
 *
 * Three visual sections, each with an icon header:
 *   1. Location  — position, timestamp, velocity from latestObservation
 *   2. Details   — metadata fields from the field renderer registry
 *   3. AI Analysis — parsed aiMetadataJson with severity badge and summary
 */

import React, { useMemo } from 'react';
import { Box, Divider, Typography, Skeleton, Tooltip } from '@mui/material';
import { MapPin, FileText, Bot } from 'lucide-react';
import { registerTab, type EntityTabProps } from './tabRegistry';
import { resolveFields } from './overview/fieldRenderers';
import {
  alpha,
  theme,
  formatRelativeTime,
  formatAbsoluteTime,
  DASHBOARD_TYPOGRAPHY,
} from '@respondent/core';
import FieldRow from './shared/FieldRow';
import SmartValue from './shared/SmartValue';
import SeverityBadge from './shared/SeverityBadge';
import SectionHeader from './shared/SectionHeader';
import { parseNumericString } from './shared/numericUtils';
import { useDetailVariant } from './shared/DetailVariantContext';
import MediaSection, { mediaMetadataKeys } from '@/features/media/MediaSection';
import { useUIStore } from '@/app/store';

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Format a snake_case or camelCase key into "Sentence case". */
function formatAiKey(key: string): string {
  const spaced = key.replace(/([a-z])([A-Z])/g, '$1 $2').replace(/[_-]/g, ' ');
  return spaced.charAt(0).toUpperCase() + spaced.slice(1).toLowerCase();
}

/** Render an unknown AI field value as a string, returning null if empty/null. */
function renderAiValue(value: unknown): string | null {
  if (value === null || value === undefined) return null;
  if (typeof value === 'boolean') return value ? 'YES' : 'NO';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return value.trim() || null;
  if (Array.isArray(value)) {
    const joined = value.join(', ').trim();
    return joined || null;
  }
  return JSON.stringify(value);
}

/** Keys from AI metadata that duplicate position data already shown in Location section. */
const AI_SKIP_KEYS = new Set(['lat', 'lon', 'location_confidence', 'country_iso3']);

/** A nested operation result — the shape enrichment uses when a source has no output_mapping. */
function isOperationResult(v: unknown): v is Record<string, unknown> {
  return v !== null && typeof v === 'object' && !Array.isArray(v);
}

/**
 * Hoist enrichment fields into a single flat map. Operations without an
 * output_mapping store their result nested under the operation name
 * (e.g. { news_geo_enrichment: { severity, summary, … } }); mapped operations
 * write fields at the top level. The Overview collapses every operation into one
 * "AI Analysis" section, so fields are merged with a stable precedence: explicit
 * top-level (mapped) fields win, then nested operation fields fill in without
 * clobbering. On a key collision the first value wins, keeping the result
 * deterministic rather than dependent on object key order.
 */
function flattenAiFields(metadata: Record<string, unknown>): Record<string, unknown> {
  const fields: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(metadata)) {
    if (!isOperationResult(value)) fields[key] = value;
  }
  for (const value of Object.values(metadata)) {
    if (!isOperationResult(value)) continue;
    for (const [key, nested] of Object.entries(value)) {
      if (!(key in fields)) fields[key] = nested;
    }
  }
  return fields;
}

// ─── Section Divider ──────────────────────────────────────────────────────────

const SectionDivider: React.FC = () => (
  <Divider sx={{ my: 1.5, borderColor: alpha(theme.palette.primary.main, 0.08) }} />
);

// ─── Number Formatter ─────────────────────────────────────────────────────────

/** Format raw metadata string values for display. Handles locale-formatted
 *  numbers (e.g. "8,534") and trims float precision to 4 dp. */
function formatFieldValue(value: string): string {
  if (value.trim() === '') return value;
  const num = parseNumericString(value);
  if (num === null) return value;
  if (Number.isInteger(num)) return num.toLocaleString();
  // Float: max 4 decimal places, trim trailing zeros
  return parseFloat(num.toFixed(4)).toLocaleString(undefined, { maximumFractionDigits: 4 });
}

// ─── OverviewTab ──────────────────────────────────────────────────────────────

/**
 * `mediaActive` is false while the panel is minimized. A minimized panel keeps
 * its state but must not keep a camera refreshing.
 */
export interface OverviewTabProps extends EntityTabProps {
  mediaActive?: boolean;
}

const OverviewTab: React.FC<OverviewTabProps> = ({
  detail,
  isLoading,
  layerType,
  entityId,
  mediaActive = true,
}) => {
  const declaredMedia = useUIStore((s) => s.layers[layerType]?.displayConfig?.media);
  useDetailVariant(); // consumed by child components via context

  // Parse AI metadata JSON once
  const aiMetadata = useMemo(() => {
    const jsonStr = detail?.entity?.aiMetadataJson;
    if (!jsonStr) return null;
    try {
      const obj = JSON.parse(jsonStr);
      if (!obj || typeof obj !== 'object' || Array.isArray(obj) || Object.keys(obj).length === 0)
        return null;
      return obj as Record<string, unknown>;
    } catch {
      return null;
    }
  }, [detail?.entity?.aiMetadataJson]);

  // Hoist nested operation results so severity/summary/fields resolve uniformly
  // regardless of whether the source defines an output_mapping (see flattenAiFields).
  const aiFields = useMemo(() => (aiMetadata ? flattenAiFields(aiMetadata) : null), [aiMetadata]);

  if (isLoading) {
    return (
      <Box sx={{ p: 1 }}>
        {[...Array(5)].map((_, i) => (
          <Skeleton key={i} variant="text" sx={{ mb: 0.5 }} />
        ))}
      </Box>
    );
  }

  if (!detail?.entity) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{ color: 'error.main', fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize }}
        >
          Entity data unavailable
        </Typography>
      </Box>
    );
  }

  const { entity, latestObservation } = detail;

  // Merge entity metadata + observation metadata for field resolution.
  // Observation metadata takes precedence as it represents the latest state.
  const mergedMetadata: Record<string, string> = {
    ...(entity.metadata || {}),
    ...(latestObservation?.metadata || {}),
  };
  // Media URLs are rendered by the media section, not repeated as scalar rows.
  // The raw values stay untouched in mergedMetadata for the Metadata tab.
  const mediaKeys = mediaMetadataKeys(declaredMedia);
  const fieldMetadata = mediaKeys.size
    ? Object.fromEntries(Object.entries(mergedMetadata).filter(([key]) => !mediaKeys.has(key)))
    : mergedMetadata;
  const fields = resolveFields(fieldMetadata, layerType);

  const hasLocation = !!latestObservation;
  const hasDetails = fields.length > 0;

  // Collect severity, summary, and remaining scalar fields from the flattened set.
  const aiSeverity = aiFields ? ((aiFields['severity'] as string | undefined) ?? null) : null;
  const aiSummary = aiFields ? ((aiFields['summary'] as string | undefined) ?? null) : null;
  const aiRemainingEntries = aiFields
    ? Object.entries(aiFields).filter(([key, value]) => {
        if (key === 'severity' || key === 'summary') return false;
        if (AI_SKIP_KEYS.has(key)) return false;
        if (typeof value === 'object' && value !== null && !Array.isArray(value)) return false;
        return renderAiValue(value) !== null;
      })
    : [];

  // Only show the AI section when there is something to render — never an empty header.
  const hasAi = !!(aiSeverity || aiSummary || aiRemainingEntries.length > 0);

  return (
    <Box sx={{ p: 0.5 }}>
      {/* Layer type badge — entity name shown in the panel header statusValue */}
      <Box sx={{ mb: 1 }}>
        <Typography
          variant="caption"
          sx={{
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            color: 'text.secondary',
            textTransform: 'uppercase',
            letterSpacing: '0.1em',
          }}
        >
          {layerType.replace(/_/g, ' ')}
        </Typography>
      </Box>

      <MediaSection
        entityId={entityId}
        layerType={layerType}
        entityName={entity.name}
        metadata={mergedMetadata}
        active={mediaActive}
      />

      {/* ── Section 1: Location ── */}
      {hasLocation && (
        <>
          <SectionHeader icon={<MapPin size={14} />} title="Location" />

          {latestObservation!.ts != null && (
            <Tooltip title={formatAbsoluteTime(latestObservation!.ts)} placement="top" arrow>
              <Typography
                variant="caption"
                tabIndex={0}
                sx={{
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
                  color: 'text.secondary',
                  display: 'inline-block',
                  mb: 0.5,
                  cursor: 'default',
                }}
              >
                Last seen: {formatRelativeTime(latestObservation!.ts)}
              </Typography>
            </Tooltip>
          )}

          <FieldRow
            variant="overview"
            label="Latitude"
            value={latestObservation!.position.lat.toFixed(4)}
          />
          <FieldRow
            variant="overview"
            label="Longitude"
            value={latestObservation!.position.lon.toFixed(4)}
          />
          <FieldRow
            variant="overview"
            label="Altitude"
            value={`${Math.round(latestObservation!.altitudeM).toLocaleString()} m`}
          />
          {latestObservation!.velocity?.speed != null && (
            <FieldRow
              variant="overview"
              label="Speed"
              value={`${Math.round(latestObservation!.velocity.speed)} kts`}
            />
          )}
          {latestObservation!.velocity?.heading != null && (
            <FieldRow
              variant="overview"
              label="Heading"
              value={`${Math.round(latestObservation!.velocity.heading)}°`}
            />
          )}
        </>
      )}

      {/* ── Section 2: Details ── */}
      {hasDetails && (
        <>
          {hasLocation && <SectionDivider />}
          <SectionHeader icon={<FileText size={14} />} title="Details" />
          {fields.map((field) => (
            <FieldRow
              key={field.label}
              variant="overview"
              label={field.label}
              value={formatFieldValue(field.value)}
            />
          ))}
        </>
      )}

      {/* ── Section 3: AI Analysis ── */}
      {hasAi && (
        <>
          {(hasLocation || hasDetails) && <SectionDivider />}
          <SectionHeader icon={<Bot size={14} />} title="AI Analysis" />

          {/* Severity — side-by-side row with colored badge */}
          {aiSeverity && (
            <Box
              sx={{
                display: 'flex',
                flexDirection: 'row',
                justifyContent: 'space-between',
                alignItems: 'center',
                py: 0.5,
                px: 0.5,
              }}
            >
              <Typography
                variant="caption"
                sx={{
                  color: 'text.secondary',
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize,
                  fontWeight: 500,
                  letterSpacing: '0.04em',
                  flexShrink: 0,
                }}
              >
                Severity
              </Typography>
              <SeverityBadge severity={aiSeverity} />
            </Box>
          )}

          {/* Summary — stacked, full paragraph, no truncation */}
          {aiSummary && (
            <Box sx={{ py: 0.5, px: 0.5 }}>
              <Typography
                variant="caption"
                sx={{
                  color: 'text.secondary',
                  fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize,
                  fontWeight: 500,
                  letterSpacing: '0.04em',
                  display: 'block',
                  mb: 0.5,
                }}
              >
                Summary
              </Typography>
              <SmartValue
                value={aiSummary}
                fontSize={DASHBOARD_TYPOGRAPHY.dashboardBase.fontSize as string}
                mono={false}
                truncateAt={0}
              />
            </Box>
          )}

          {/* Remaining AI fields */}
          {aiRemainingEntries.map(([key, value]) => {
            const rendered = renderAiValue(value)!;
            return (
              <FieldRow
                key={key}
                variant="overview"
                label={formatAiKey(key)}
                value={formatFieldValue(rendered)}
              />
            );
          })}
        </>
      )}
    </Box>
  );
};

registerTab({
  id: 'overview',
  label: 'Overview',
  priority: 0,
  component: OverviewTab,
});

export default OverviewTab;
