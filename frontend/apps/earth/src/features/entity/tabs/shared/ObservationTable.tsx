/**
 * ObservationTable — Reusable, interactive table for observation data.
 *
 * Features:
 *   - Left-aligned headers with sticky top, uppercase, muted color
 *   - Proper cell padding, subtle borders between rows and columns
 *   - Row hover highlight
 *   - Alt+click to copy cell value with green flash animation
 *   - Right-click row context menu: "Copy Row as JSON" / "Copy Row as CSV"
 *   - Smart number formatting (integer vs float, toLocaleString for large numbers)
 *   - snake_case headers → Title Case
 *   - Column order: Time, Lat, Lon, Alt, ...meta keys alphabetically
 *   - Time column wider (min 140px), all others min 60px
 */

import React, { useCallback, useRef, useState } from 'react';
import { Box, Menu, MenuItem } from '@mui/material';
import { keyframes } from '@emotion/react';
import type { TrailPoint } from '@respondent/core';
import { parseNumericString } from './numericUtils';
import {
  DASHBOARD_TYPOGRAPHY,
  BORDER,
  HOVER,
  FONT_FAMILY,
  theme,
  formatUtcTime,
} from '@respondent/core';

// ─── Types ────────────────────────────────────────────────────────────────────

export interface ObservationTableProps {
  /** Pre-sorted points (newest first). */
  points: TrailPoint[];
  /** Metadata keys to render as extra columns (alphabetical). */
  metaKeys: string[];
}

interface ContextMenuState {
  mouseX: number;
  mouseY: number;
  rowIndex: number;
}

// ─── Animations ───────────────────────────────────────────────────────────────

const cellCopyFlash = keyframes`
  0%   { background-color: rgba(0, 255, 157, 0.3); }
  100% { background-color: transparent; }
`;

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** snake_case → Title Case: "altitude_km" → "Altitude Km" */
function toTitleCase(key: string): string {
  return key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

/**
 * Format a raw metadata value for display.
 *   - Integers → no decimal places, toLocaleString for thousands separators
 *   - Floats → max 4 decimal places with trailing zeros trimmed
 *   - Non-numeric → render as-is
 */
function formatCellValue(value: unknown): string {
  if (value === null || value === undefined) return '—';
  const str = String(value);
  if (str.trim() === '') return '—';
  const num = parseNumericString(str);
  if (num === null) return str;
  if (Number.isInteger(num)) return num.toLocaleString();
  // Float — max 4 decimal places, trim trailing zeros
  const fixed = parseFloat(num.toFixed(4));
  return fixed.toLocaleString(undefined, { maximumFractionDigits: 4 });
}

// ─── Style constants ─────────────────────────────────────────────────────────

const HEADER_BG = '#0a0a0a';
const ROW_HOVER_BG = 'rgba(255, 255, 255, 0.03)';
const COL_BORDER = 'rgba(255, 255, 255, 0.06)';

// ─── Component ────────────────────────────────────────────────────────────────

const ObservationTable: React.FC<ObservationTableProps> = ({ points, metaKeys }) => {
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const [flashCell, setFlashCell] = useState<{ row: number; col: number } | null>(null);
  const flashTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const totalColumns = 4 + metaKeys.length;

  // ── Row data builders ──────────────────────────────────────────────────────

  const buildRowJson = useCallback(
    (point: TrailPoint): string => {
      const obj: Record<string, unknown> = {
        time: formatUtcTime(point.ts),
        lat: parseFloat(point.lat.toFixed(4)),
        lon: parseFloat(point.lon.toFixed(4)),
        alt_m: Math.round(point.altitudeM),
      };
      for (const k of metaKeys) {
        obj[k] = point.metadata?.[k] ?? null;
      }
      return JSON.stringify(obj);
    },
    [metaKeys],
  );

  const buildRowCsv = useCallback(
    (point: TrailPoint): string => {
      const cells = [
        formatUtcTime(point.ts),
        point.lat.toFixed(4),
        point.lon.toFixed(4),
        String(Math.round(point.altitudeM)),
        ...metaKeys.map((k) => {
          const v = String(point.metadata?.[k] ?? '');
          return v.includes(',') ? `"${v}"` : v;
        }),
      ];
      return cells.join(',');
    },
    [metaKeys],
  );

  // ── Alt+click to copy cell ─────────────────────────────────────────────────

  const handleCellClick = useCallback(
    (e: React.MouseEvent<HTMLTableCellElement>, value: string, rowIdx: number, colIdx: number) => {
      if (!e.altKey) return;
      e.preventDefault();
      navigator.clipboard.writeText(value).catch(() => {});

      // Clear any pending flash timer
      if (flashTimerRef.current) clearTimeout(flashTimerRef.current);
      setFlashCell({ row: rowIdx, col: colIdx });
      flashTimerRef.current = setTimeout(() => setFlashCell(null), 350);
    },
    [],
  );

  // ── Right-click context menu ───────────────────────────────────────────────

  const handleRowContextMenu = useCallback(
    (e: React.MouseEvent<HTMLTableRowElement>, rowIdx: number) => {
      e.preventDefault();
      setContextMenu({ mouseX: e.clientX, mouseY: e.clientY, rowIndex: rowIdx });
    },
    [],
  );

  const handleMenuClose = useCallback(() => setContextMenu(null), []);

  const handleCopyRowJson = useCallback(() => {
    if (contextMenu === null) return;
    const point = points[contextMenu.rowIndex];
    if (point) navigator.clipboard.writeText(buildRowJson(point)).catch(() => {});
    handleMenuClose();
  }, [contextMenu, points, buildRowJson, handleMenuClose]);

  const handleCopyRowCsv = useCallback(() => {
    if (contextMenu === null) return;
    const point = points[contextMenu.rowIndex];
    if (point) navigator.clipboard.writeText(buildRowCsv(point)).catch(() => {});
    handleMenuClose();
  }, [contextMenu, points, buildRowCsv, handleMenuClose]);

  // ── Shared sx builders ─────────────────────────────────────────────────────

  const headerCellSx = (colIdx: number): React.CSSProperties => ({
    fontFamily: FONT_FAMILY,
    fontSize: DASHBOARD_TYPOGRAPHY.dashboardXxs.fontSize,
    fontWeight: 600,
    color: theme.palette.text.secondary,
    textAlign: 'left',
    textTransform: 'uppercase',
    letterSpacing: '0.06em',
    whiteSpace: 'nowrap',
    padding: '6px 12px',
    position: 'sticky',
    top: 0,
    backgroundColor: HEADER_BG,
    zIndex: 1,
    borderBottom: `1px solid ${BORDER.default}`,
    borderRight: colIdx < totalColumns - 1 ? `1px solid ${COL_BORDER}` : undefined,
    minWidth: colIdx === 0 ? 140 : 60,
  });

  const bodyCellSx = (colIdx: number, rowIdx: number): React.CSSProperties => {
    const isFlashing = flashCell?.row === rowIdx && flashCell?.col === colIdx;
    return {
      fontFamily: FONT_FAMILY,
      fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
      color: theme.palette.text.primary,
      textAlign: 'left',
      whiteSpace: 'nowrap',
      padding: '6px 12px',
      borderBottom: `1px solid ${BORDER.subtle}`,
      borderRight: colIdx < totalColumns - 1 ? `1px solid ${COL_BORDER}` : undefined,
      cursor: 'pointer',
      animation: isFlashing ? `${cellCopyFlash} 300ms ease-out forwards` : undefined,
    };
  };

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <>
      <Box
        data-testid="observation-table"
        sx={{
          flex: 1,
          overflowX: 'auto',
          overflowY: 'auto',
          borderRadius: 1,
          border: `1px solid ${BORDER.subtle}`,
          '&:hover': { borderColor: HOVER.fieldRow },
        }}
      >
        <table
          style={{
            borderCollapse: 'collapse',
            width: '100%',
            tableLayout: 'auto',
          }}
        >
          <thead>
            <tr>
              <th style={headerCellSx(0)}>Time</th>
              <th style={headerCellSx(1)}>Lat</th>
              <th style={headerCellSx(2)}>Lon</th>
              <th style={headerCellSx(3)}>Alt (m)</th>
              {metaKeys.map((key, i) => (
                <th key={key} style={headerCellSx(4 + i)}>
                  {toTitleCase(key)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {points.map((point, rowIdx) => {
              const timeVal = formatUtcTime(point.ts);
              const latVal = point.lat.toFixed(4);
              const lonVal = point.lon.toFixed(4);
              const altVal = Math.round(point.altitudeM).toLocaleString();

              return (
                <tr
                  key={`${point.ts}-${rowIdx}`}
                  onContextMenu={(e) => handleRowContextMenu(e, rowIdx)}
                  style={{ transition: 'background-color 0.1s', cursor: 'pointer' }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLTableRowElement).style.backgroundColor = ROW_HOVER_BG;
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLTableRowElement).style.backgroundColor = 'transparent';
                  }}
                >
                  <td
                    style={bodyCellSx(0, rowIdx)}
                    onClick={(e) => handleCellClick(e, timeVal, rowIdx, 0)}
                  >
                    {timeVal}
                  </td>
                  <td
                    style={bodyCellSx(1, rowIdx)}
                    onClick={(e) => handleCellClick(e, latVal, rowIdx, 1)}
                  >
                    {latVal}
                  </td>
                  <td
                    style={bodyCellSx(2, rowIdx)}
                    onClick={(e) => handleCellClick(e, lonVal, rowIdx, 2)}
                  >
                    {lonVal}
                  </td>
                  <td
                    style={bodyCellSx(3, rowIdx)}
                    onClick={(e) => handleCellClick(e, altVal, rowIdx, 3)}
                  >
                    {altVal}
                  </td>
                  {metaKeys.map((key, i) => {
                    const raw = point.metadata?.[key] ?? null;
                    const display = formatCellValue(raw);
                    return (
                      <td
                        key={key}
                        style={bodyCellSx(4 + i, rowIdx)}
                        onClick={(e) => handleCellClick(e, display, rowIdx, 4 + i)}
                      >
                        {display}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </Box>

      {/* Right-click context menu */}
      <Menu
        open={contextMenu !== null}
        onClose={handleMenuClose}
        anchorReference="anchorPosition"
        anchorPosition={
          contextMenu !== null ? { top: contextMenu.mouseY, left: contextMenu.mouseX } : undefined
        }
        slotProps={{
          paper: {
            sx: {
              bgcolor: '#0a0a0a',
              border: `1px solid ${BORDER.default}`,
              borderRadius: 1,
              minWidth: 180,
            },
          },
        }}
      >
        <MenuItem
          onClick={handleCopyRowJson}
          sx={{
            fontFamily: FONT_FAMILY,
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            color: 'text.primary',
            py: 0.75,
            px: 1.5,
            '&:hover': { bgcolor: HOVER.row },
          }}
        >
          Copy Row as JSON
        </MenuItem>
        <MenuItem
          onClick={handleCopyRowCsv}
          sx={{
            fontFamily: FONT_FAMILY,
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            color: 'text.primary',
            py: 0.75,
            px: 1.5,
            '&:hover': { bgcolor: HOVER.row },
          }}
        >
          Copy Row as CSV
        </MenuItem>
      </Menu>
    </>
  );
};

export default ObservationTable;
