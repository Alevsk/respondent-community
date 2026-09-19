/**
 * AIAnalysisTab — Displays AI enrichment data for entities.
 *
 * Parses the ai_metadata_json field from the entity detail response
 * and renders each top-level key as a labeled section with its sub-fields.
 * Only visible when AI metadata is available.
 */

import React, { useMemo } from 'react';
import { Box, Typography, Skeleton } from '@mui/material';
import type { EntityTabProps } from './tabRegistry';
import FieldRow from './shared/FieldRow';
import SmartValue from './shared/SmartValue';
import SectionHeader from './shared/SectionHeader';
import { theme, alpha, DASHBOARD_TYPOGRAPHY } from '@respondent/core';
import { useDetailVariant } from './shared/DetailVariantContext';

/** Format a snake_case or camelCase key into a human-readable label. */
function formatKey(key: string, clean: boolean = false): string {
  const spaced = key.replace(/([a-z])([A-Z])/g, '$1 $2').replace(/[_-]/g, ' ');
  if (clean) {
    return spaced.charAt(0).toUpperCase() + spaced.slice(1).toLowerCase();
  }
  return spaced.toUpperCase();
}

/** Map known severity/risk values to theme-consistent colors. */
function getValueColor(value: string): string | undefined {
  const v = value.toLowerCase();
  if (['critical', 'extreme', 'high', 'emergency', 'hijack'].includes(v))
    return theme.palette.secondary.main;
  if (['elevated', 'moderate', 'medium'].includes(v)) return '#ffbe0b';
  if (['low', 'minor', 'none', 'minimal'].includes(v)) return theme.palette.primary.main;
  return undefined;
}

const SummaryBlock: React.FC<{ text: string; isClean?: boolean }> = ({ text, isClean = false }) => (
  <Box
    sx={{
      py: 0.75,
      px: 1,
      mb: 0.5,
      borderLeft: isClean ? '1px solid' : '2px solid',
      borderColor: isClean ? alpha(theme.palette.primary.main, 0.3) : 'primary.main',
      bgcolor: alpha(theme.palette.primary.main, isClean ? 0.02 : 0.03),
      borderRadius: '0 2px 2px 0',
    }}
  >
    <SmartValue
      value={text}
      fontSize={DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize}
      mono={false}
      truncateAt={200}
    />
  </Box>
);

function renderValue(value: unknown): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'boolean') return value ? 'YES' : 'NO';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return value || '—';
  if (Array.isArray(value)) return value.join(', ') || '—';
  return JSON.stringify(value);
}

/** Check if a value is a plain object (not array, not null). */
function isPlainObject(v: unknown): v is Record<string, unknown> {
  return v !== null && typeof v === 'object' && !Array.isArray(v);
}

/** Render a section (one top-level AI operation result). */
const AISection: React.FC<{ name: string; data: Record<string, unknown> }> = ({ name, data }) => {
  const isClean = useDetailVariant() === 'clean';
  const summaryValue = data.summary as string | undefined;
  const entries = Object.entries(data).filter(([k]) => k !== 'summary');

  return (
    <Box sx={{ mb: 1.5 }}>
      <SectionHeader title={formatKey(name, isClean)} />
      {summaryValue && <SummaryBlock text={summaryValue} isClean={isClean} />}
      {entries.map(([key, value]) => {
        const rendered = renderValue(value);
        const color = getValueColor(rendered);
        return (
          <FieldRow
            key={key}
            label={formatKey(key, isClean)}
            value={rendered}
            color={color}
            fontWeight={color ? 600 : undefined}
          />
        );
      })}
    </Box>
  );
};

/** Render flat fields (top-level primitives/arrays — no section header). */
const AIFlatFields: React.FC<{ data: Record<string, unknown> }> = ({ data }) => {
  const isClean = useDetailVariant() === 'clean';
  const summaryValue = data.summary as string | undefined;
  const entries = Object.entries(data).filter(([k, v]) => k !== 'summary' && !isPlainObject(v));

  return (
    <Box sx={{ mb: 1.5 }}>
      {summaryValue && <SummaryBlock text={summaryValue} isClean={isClean} />}
      {entries.map(([key, value]) => {
        const rendered = renderValue(value);
        const color = getValueColor(rendered);
        return (
          <FieldRow
            key={key}
            label={formatKey(key, isClean)}
            value={rendered}
            color={color}
            fontWeight={color ? 600 : undefined}
          />
        );
      })}
    </Box>
  );
};

const AIAnalysisTab: React.FC<EntityTabProps> = ({ detail, isLoading }) => {
  const parsed = useMemo(() => {
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

  const { flatFields, sections } = useMemo(() => {
    if (!parsed) return { flatFields: null, sections: null };
    const flat: Record<string, unknown> = {};
    const nested: Record<string, Record<string, unknown>> = {};
    for (const [key, value] of Object.entries(parsed)) {
      if (isPlainObject(value)) {
        nested[key] = value;
      } else {
        flat[key] = value;
      }
    }
    return {
      flatFields: Object.keys(flat).length > 0 ? flat : null,
      sections: Object.keys(nested).length > 0 ? nested : null,
    };
  }, [parsed]);

  if (isLoading) {
    return (
      <Box sx={{ p: 1 }}>
        {[...Array(4)].map((_, i) => (
          <Skeleton key={i} variant="text" sx={{ mb: 0.5 }} />
        ))}
      </Box>
    );
  }

  if (!flatFields && !sections) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{ color: 'text.secondary', fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize }}
        >
          No AI analysis available
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ p: 0.5 }}>
      {flatFields && <AIFlatFields data={flatFields} />}
      {sections &&
        Object.entries(sections).map(([name, data]) => (
          <AISection key={name} name={name} data={data} />
        ))}
    </Box>
  );
};

// Tab registration removed — AI analysis merged into OverviewTab
// registerTab({
//   id: 'ai',
//   label: 'AI',
//   priority: 5, // between Overview (0) and History (10)
//   component: AIAnalysisTab,
// });

export default AIAnalysisTab;
