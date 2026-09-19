/**
 * MetadataTab — Raw key-value table of all entity metadata fields.
 *
 * Displays all metadata as-is without any formatting or filtering.
 * Useful for debugging and inspecting raw entity data.
 */

import React from 'react';
import { Box, Typography, Skeleton } from '@mui/material';
import { Code, Database, Activity } from 'lucide-react';
import { Surface, DASHBOARD_TYPOGRAPHY } from '@respondent/core';
import { registerTab, type EntityTabProps } from './tabRegistry';
import FieldRow from './shared/FieldRow';
import SectionHeader from './shared/SectionHeader';
import { useDetailVariant } from './shared/DetailVariantContext';

const MetadataSection: React.FC<{
  title: string;
  icon: React.ReactNode;
  entries: [string, string][];
}> = ({ title, icon, entries }) => {
  const variant = useDetailVariant();
  const isClean = variant === 'clean';

  const content = (
    <>
      <SectionHeader icon={icon} title={title} />
      {entries.map(([key, value]) => (
        <FieldRow key={key} label={key} value={value || '—'} />
      ))}
    </>
  );

  if (isClean) {
    return <Box sx={{ mb: 0.5 }}>{content}</Box>;
  }

  return (
    <Surface level="base" sx={{ p: 1, my: 0.5 }}>
      {content}
    </Surface>
  );
};

const MetadataTab: React.FC<EntityTabProps> = ({ detail, isLoading }) => {
  if (isLoading) {
    return (
      <Box sx={{ p: 1 }}>
        {[...Array(4)].map((_, i) => (
          <Skeleton key={i} variant="text" sx={{ mb: 0.5 }} />
        ))}
      </Box>
    );
  }

  const entityMeta = detail?.entity?.metadata;
  const obsMeta = detail?.latestObservation?.metadata;
  const hasEntityMeta = entityMeta && Object.keys(entityMeta).length > 0;
  const hasObsMeta = obsMeta && Object.keys(obsMeta).length > 0;

  if (!hasEntityMeta && !hasObsMeta) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{ color: 'text.secondary', fontSize: DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize }}
        >
          No metadata available
        </Typography>
      </Box>
    );
  }

  const entityEntries = hasEntityMeta
    ? Object.entries(entityMeta).sort(([a], [b]) => a.localeCompare(b))
    : [];
  const obsEntries = hasObsMeta
    ? Object.entries(obsMeta).sort(([a], [b]) => a.localeCompare(b))
    : [];

  return (
    <Box sx={{ p: 0.5 }}>
      {entityEntries.length > 0 && (
        <MetadataSection
          title="Entity"
          icon={<Database size={14} />}
          entries={entityEntries as [string, string][]}
        />
      )}
      {obsEntries.length > 0 && (
        <MetadataSection
          title="Observation"
          icon={<Activity size={14} />}
          entries={obsEntries as [string, string][]}
        />
      )}
    </Box>
  );
};

registerTab({
  id: 'metadata',
  label: 'Metadata',
  icon: React.createElement(Code, { size: 14 }),
  priority: 20,
  component: MetadataTab,
});

export default MetadataTab;
