/**
 * ObservationChart — line chart for numeric observation fields.
 *
 * Scans all trail points for numeric metadata fields (at least 2 observations
 * with at least 2 distinct values), plus altitudeM and speed if they vary.
 * Renders a recharts line chart with pill buttons to toggle visible fields.
 * Multiple fields may be selected simultaneously, each rendered as a separate
 * colored line.
 */

import React, { useMemo, useState } from 'react';
import { Box, Typography } from '@mui/material';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import type { TrailPoint } from '@respondent/core';
import { parseNumericString } from './shared/numericUtils';
import { DASHBOARD_TYPOGRAPHY, FONT_FAMILY, alpha, formatUtcTime, theme } from '@respondent/core';

interface ObservationChartProps {
  points: TrailPoint[];
}

interface ChartableField {
  key: string;
  label: string;
}

/** Color palette for multi-line charts. Index 0 = primary green. */
const LINE_COLORS = ['#00ff9d', '#ff006e', '#00e5ff', '#ffbe0b', '#8338ec'];

/** Collect all numeric fields present in any observation.
 *  With 1 observation the chart shows a single data point.
 *  With 2+ observations it draws a proper line. */
function detectChartableFields(points: TrailPoint[]): ChartableField[] {
  if (points.length === 0) return [];

  const fields: ChartableField[] = [];

  // Check altitudeM — include if any observation has a non-zero value
  if (points.some((p) => p.altitudeM !== 0)) {
    fields.push({ key: '__altitudeM', label: 'Altitude (m)' });
  }

  // Check speed — include if any observation has speed
  if (points.some((p) => p.speed !== undefined && p.speed !== null)) {
    fields.push({ key: '__speed', label: 'Speed (m/s)' });
  }

  // Gather all metadata keys across all observations
  const allKeys = new Set<string>();
  for (const point of points) {
    if (point.metadata) {
      for (const key of Object.keys(point.metadata)) {
        allKeys.add(key);
      }
    }
  }

  // Include any metadata field that has at least 1 numeric value
  for (const key of allKeys) {
    let hasNumeric = false;
    for (const point of points) {
      const raw = point.metadata?.[key];
      if (parseNumericString(raw) !== null) {
        hasNumeric = true;
        break;
      }
    }
    if (hasNumeric) {
      const label = key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
      fields.push({ key, label });
    }
  }

  return fields;
}

/** Extract a numeric value for a given field key from a TrailPoint. */
function extractValue(point: TrailPoint, key: string): number | null {
  if (key === '__altitudeM') return point.altitudeM;
  if (key === '__speed') return point.speed ?? null;
  const raw = point.metadata?.[key];
  return parseNumericString(raw);
}

const ObservationChart: React.FC<ObservationChartProps> = ({ points }) => {
  const chartableFields = useMemo(() => detectChartableFields(points), [points]);

  // Track selected field keys as a Set — multiple can be active simultaneously
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(
    () => new Set(chartableFields[0] ? [chartableFields[0].key] : []),
  );

  // Ensure at least one field is always selected; sync when fields change
  const activeKeys = useMemo(() => {
    const valid = new Set(
      [...selectedKeys].filter((k) => chartableFields.some((f) => f.key === k)),
    );
    if (valid.size === 0 && chartableFields[0]) valid.add(chartableFields[0].key);
    return valid;
  }, [selectedKeys, chartableFields]);

  const toggleField = (key: string) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        // Don't allow deselecting the last active field
        if (next.size === 1) return prev;
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  // Build chart data: one row per point, one numeric key per active field
  const chartData = useMemo(() => {
    const activeFieldList = chartableFields.filter((f) => activeKeys.has(f.key));
    // Points are oldest-first; chart oldest on the left
    return points.map((p) => {
      const row: Record<string, number | null | string> = { ts: p.ts };
      for (const field of activeFieldList) {
        row[field.key] = extractValue(p, field.key);
      }
      return row;
    });
  }, [points, chartableFields, activeKeys]);

  if (chartableFields.length === 0) {
    return (
      <Box sx={{ p: 1 }}>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
          }}
        >
          No numeric fields available for charting
        </Typography>
      </Box>
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      {/* Field selector — pill buttons */}
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
        {chartableFields.map((field, idx) => {
          const isActive = activeKeys.has(field.key);
          const color = LINE_COLORS[idx % LINE_COLORS.length];
          return (
            <Box
              key={field.key}
              component="button"
              onClick={() => toggleField(field.key)}
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                cursor: 'pointer',
                border: `1px solid ${isActive ? color : 'rgba(255,255,255,0.12)'}`,
                borderRadius: '12px',
                backgroundColor: isActive ? alpha(color, 0.12) : 'transparent',
                color: isActive ? color : 'text.secondary',
                px: 1,
                py: 0.25,
                fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs?.fontSize ?? '9px',
                fontFamily: FONT_FAMILY,
                fontWeight: isActive ? 600 : 400,
                letterSpacing: '0.04em',
                textTransform: 'uppercase',
                transition: 'all 0.15s ease',
                outline: 'none',
                '&:hover': {
                  borderColor: color,
                  backgroundColor: alpha(color, 0.08),
                  color: color,
                },
              }}
            >
              {field.label}
            </Box>
          );
        })}
      </Box>

      {/* Chart */}
      <Box sx={{ height: 180 }}>
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 4, left: 0 }}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke="rgba(255, 255, 255, 0.06)"
              vertical={false}
            />
            <XAxis
              dataKey="ts"
              tickFormatter={(ts: number) => formatUtcTime(ts)}
              tick={{
                fill: theme.palette.text.secondary,
                fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize as string,
              }}
              axisLine={{ stroke: 'rgba(255, 255, 255, 0.1)' }}
              tickLine={false}
              minTickGap={40}
            />
            <YAxis
              tick={{
                fill: theme.palette.text.secondary,
                fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize as string,
              }}
              axisLine={{ stroke: 'rgba(255, 255, 255, 0.1)' }}
              tickLine={false}
              width={40}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'rgba(10, 10, 10, 0.95)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                borderRadius: 4,
                fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize as string,
                color: theme.palette.text.primary,
              }}
              labelFormatter={(ts) => formatUtcTime(ts as number)}
              formatter={(value, dataKey) => {
                const field = chartableFields.find((f) => f.key === (dataKey as string));
                return [value as number, field?.label ?? (dataKey as string)];
              }}
            />
            {chartableFields
              .filter((f) => activeKeys.has(f.key))
              .map((field) => {
                const originalIdx = chartableFields.findIndex((f) => f.key === field.key);
                const color = LINE_COLORS[originalIdx % LINE_COLORS.length];
                return (
                  <Line
                    key={field.key}
                    type="monotone"
                    dataKey={field.key}
                    stroke={color}
                    strokeWidth={1.5}
                    dot={false}
                    activeDot={{ r: 3, fill: color }}
                    connectNulls={false}
                  />
                );
              })}
          </LineChart>
        </ResponsiveContainer>
      </Box>
    </Box>
  );
};

export default ObservationChart;
