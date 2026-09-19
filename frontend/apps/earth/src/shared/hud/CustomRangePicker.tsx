import React, { useState, useMemo, useCallback } from 'react';
import { Box, Typography, IconButton } from '@mui/material';
import { ChevronLeftIcon, ChevronRightIcon } from '../icons';
import { semanticColors, alpha, theme } from '@respondent/core';

// ---- Types ----

interface DateTimeValue {
  year: number;
  month: number; // 0-indexed
  day: number;
  hour: number; // 1-12
  minute: number; // 0, 15, 30, 45
  period: 'AM' | 'PM';
}

interface CustomRangePickerProps {
  onApply: (from: string, to: string) => void;
  onError: (error: string) => void;
  /** Max lookback in hours. 0 or undefined = 48h default. */
  maxLookbackHours?: number;
  /** Max range span in hours. 0 or undefined = 24h default. */
  maxRangeSpanHours?: number;
}

// ---- Constants ----

const HOURS = [12, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11];
const MINUTES = [0, 15, 30, 45];
const DAY_NAMES = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'];
const MONTH_NAMES = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
];

// ---- Helpers ----

function toUTCDate(v: DateTimeValue): Date {
  let h24 = v.hour % 12;
  if (v.period === 'PM') h24 += 12;
  return new Date(Date.UTC(v.year, v.month, v.day, h24, v.minute, 0, 0));
}

function initValue(offset: number): DateTimeValue {
  const d = new Date(Date.now() + offset);
  const rawH = d.getUTCHours();
  return {
    year: d.getUTCFullYear(),
    month: d.getUTCMonth(),
    day: d.getUTCDate(),
    hour: rawH === 0 ? 12 : rawH > 12 ? rawH - 12 : rawH,
    minute: Math.floor(d.getUTCMinutes() / 15) * 15,
    period: rawH < 12 ? 'AM' : 'PM',
  };
}

function getDaysInMonth(year: number, month: number): number {
  return new Date(year, month + 1, 0).getDate();
}

function getFirstDayOfWeek(year: number, month: number): number {
  return new Date(year, month, 1).getDay();
}

// ---- Shared Styles ----

const cellSx = {
  width: 28,
  height: 28,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: '4px',
  cursor: 'pointer',
  fontSize: '0.65rem',
  fontFamily: 'monospace',
  transition: 'background 0.1s',
  '&:hover': { bgcolor: 'rgba(255,255,255,0.08)' },
} as const;

const selectedCellSx = {
  ...cellSx,
  bgcolor: alpha(theme.palette.primary.main, 0.15),
  color: 'primary.main',
  fontWeight: 700,
} as const;

const timeOptionSx = {
  px: 1,
  py: 0.5,
  borderRadius: '4px',
  cursor: 'pointer',
  fontSize: '0.65rem',
  fontFamily: 'monospace',
  textAlign: 'center' as const,
  transition: 'background 0.1s',
  '&:hover': { bgcolor: 'rgba(255,255,255,0.08)' },
};

const timeOptionSelectedSx = {
  ...timeOptionSx,
  bgcolor: alpha(theme.palette.primary.main, 0.15),
  color: 'primary.main',
  fontWeight: 700,
};

// ---- Mini Calendar ----

const MiniCalendar: React.FC<{
  value: DateTimeValue;
  onChange: (patch: Partial<DateTimeValue>) => void;
  minDate?: Date;
  maxDate?: Date;
}> = ({ value, onChange, minDate, maxDate }) => {
  const [viewYear, setViewYear] = useState(value.year);
  const [viewMonth, setViewMonth] = useState(value.month);

  const days = useMemo(() => {
    const count = getDaysInMonth(viewYear, viewMonth);
    const offset = getFirstDayOfWeek(viewYear, viewMonth);
    return { count, offset };
  }, [viewYear, viewMonth]);

  const prevMonth = useCallback(() => {
    if (viewMonth === 0) {
      setViewMonth(11);
      setViewYear((y) => y - 1);
    } else {
      setViewMonth((m) => m - 1);
    }
  }, [viewMonth]);

  const nextMonth = useCallback(() => {
    if (viewMonth === 11) {
      setViewMonth(0);
      setViewYear((y) => y + 1);
    } else {
      setViewMonth((m) => m + 1);
    }
  }, [viewMonth]);

  const isDisabled = (day: number) => {
    const d = new Date(viewYear, viewMonth, day);
    if (minDate && d < new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate()))
      return true;
    if (maxDate && d > new Date(maxDate.getFullYear(), maxDate.getMonth(), maxDate.getDate()))
      return true;
    return false;
  };

  return (
    <Box>
      {/* Month/Year header */}
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <IconButton
          data-testid="crp-prev-month"
          onClick={prevMonth}
          size="small"
          sx={{ color: 'text.secondary', p: 0.25 }}
        >
          <ChevronLeftIcon size={16} />
        </IconButton>
        <Typography
          variant="caption"
          sx={{ fontWeight: 700, fontSize: '0.65rem', fontFamily: 'monospace' }}
        >
          {MONTH_NAMES[viewMonth].slice(0, 3).toUpperCase()} {viewYear}
        </Typography>
        <IconButton
          data-testid="crp-next-month"
          onClick={nextMonth}
          size="small"
          sx={{ color: 'text.secondary', p: 0.25 }}
        >
          <ChevronRightIcon size={16} />
        </IconButton>
      </Box>

      {/* Day-of-week headers */}
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(7, 28px)', gap: '1px' }}>
        {DAY_NAMES.map((d) => (
          <Box key={d} sx={{ ...cellSx, cursor: 'default', '&:hover': {} }}>
            <Typography
              variant="caption"
              sx={{ fontSize: '0.55rem', color: 'text.disabled', fontFamily: 'monospace' }}
            >
              {d}
            </Typography>
          </Box>
        ))}

        {/* Blank cells for offset */}
        {Array.from({ length: days.offset }).map((_, i) => (
          <Box key={`blank-${i}`} sx={{ width: 28, height: 28 }} />
        ))}

        {/* Day cells */}
        {Array.from({ length: days.count }).map((_, i) => {
          const day = i + 1;
          const isSelected =
            day === value.day && viewMonth === value.month && viewYear === value.year;
          const disabled = isDisabled(day);
          return (
            <Box
              key={day}
              data-testid={`crp-day-${day}`}
              data-active={isSelected}
              onClick={() => {
                if (!disabled) onChange({ year: viewYear, month: viewMonth, day });
              }}
              sx={{
                ...(isSelected ? selectedCellSx : cellSx),
                ...(disabled ? { opacity: 0.25, cursor: 'default', '&:hover': {} } : {}),
                color: isSelected ? 'primary.main' : 'text.primary',
              }}
            >
              {day}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
};

// ---- Time Picker (Google Calendar style) ----

const TimePicker: React.FC<{
  value: DateTimeValue;
  onChange: (patch: Partial<DateTimeValue>) => void;
}> = ({ value, onChange }) => {
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      {/* Hour column */}
      <Box>
        <Typography
          variant="caption"
          sx={{
            fontSize: '0.55rem',
            color: 'text.disabled',
            fontFamily: 'monospace',
            mb: 0.25,
            display: 'block',
            textAlign: 'center',
          }}
        >
          HR
        </Typography>
        <Box
          sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '2px', maxWidth: 90 }}
        >
          {HOURS.map((h) => (
            <Box
              key={h}
              data-testid={`crp-hour-${h}`}
              data-active={h === value.hour}
              onClick={() => onChange({ hour: h })}
              sx={h === value.hour ? timeOptionSelectedSx : timeOptionSx}
            >
              {h}
            </Box>
          ))}
        </Box>
      </Box>

      {/* Minute column */}
      <Box>
        <Typography
          variant="caption"
          sx={{
            fontSize: '0.55rem',
            color: 'text.disabled',
            fontFamily: 'monospace',
            mb: 0.25,
            display: 'block',
            textAlign: 'center',
          }}
        >
          MIN
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {MINUTES.map((m) => (
            <Box
              key={m}
              data-testid={`crp-minute-${m}`}
              data-active={m === value.minute}
              onClick={() => onChange({ minute: m })}
              sx={m === value.minute ? timeOptionSelectedSx : timeOptionSx}
            >
              :{String(m).padStart(2, '0')}
            </Box>
          ))}
        </Box>
      </Box>

      {/* AM/PM toggle */}
      <Box>
        <Typography
          variant="caption"
          sx={{
            fontSize: '0.55rem',
            color: 'text.disabled',
            fontFamily: 'monospace',
            mb: 0.25,
            display: 'block',
            textAlign: 'center',
          }}
        >
          &nbsp;
        </Typography>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {(['AM', 'PM'] as const).map((p) => (
            <Box
              key={p}
              data-testid={`crp-period-${p.toLowerCase()}`}
              data-active={p === value.period}
              onClick={() => onChange({ period: p })}
              sx={p === value.period ? timeOptionSelectedSx : timeOptionSx}
            >
              {p}
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  );
};

// ---- Composed DateTime Picker ----

const DateTimePicker: React.FC<{
  label: string;
  value: DateTimeValue;
  onChange: (patch: Partial<DateTimeValue>) => void;
  minDate?: Date;
  maxDate?: Date;
}> = ({ label, value, onChange, minDate, maxDate }) => {
  const displayTime = `${value.hour}:${String(value.minute).padStart(2, '0')} ${value.period}`;

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 700,
            fontSize: '0.65rem',
            color: 'warning.main',
            fontFamily: 'monospace',
          }}
        >
          {label}
        </Typography>
        <Typography
          variant="caption"
          sx={{ fontSize: '0.6rem', color: 'primary.main', fontFamily: 'monospace' }}
        >
          {value.year}-{String(value.month + 1).padStart(2, '0')}-
          {String(value.day).padStart(2, '0')} {displayTime} UTC
        </Typography>
      </Box>
      <MiniCalendar value={value} onChange={onChange} minDate={minDate} maxDate={maxDate} />
      <Box sx={{ mt: 1 }}>
        <TimePicker value={value} onChange={onChange} />
      </Box>
    </Box>
  );
};

// ---- Main Component ----

const CustomRangePicker: React.FC<CustomRangePickerProps> = ({
  onApply,
  onError,
  maxLookbackHours,
  maxRangeSpanHours,
}) => {
  const lookbackMs = (maxLookbackHours && maxLookbackHours > 0 ? maxLookbackHours : 48) * 3600_000;
  const maxSpanMs =
    (maxRangeSpanHours && maxRangeSpanHours > 0 ? maxRangeSpanHours : 24) * 3600_000;

  const [fromVal, setFromVal] = useState<DateTimeValue>(() => initValue(-3600_000)); // default: 1h ago
  const [toVal, setToVal] = useState<DateTimeValue>(() => initValue(0)); // default: now
  const [activeTab, setActiveTab] = useState<'from' | 'to'>('from');

  const maxDate = new Date();
  const minDate = new Date(Date.now() - lookbackMs);

  const handleFromChange = useCallback((patch: Partial<DateTimeValue>) => {
    setFromVal((prev) => ({ ...prev, ...patch }));
  }, []);

  const handleToChange = useCallback((patch: Partial<DateTimeValue>) => {
    setToVal((prev) => ({ ...prev, ...patch }));
  }, []);

  const handleApply = useCallback(() => {
    const fromDate = toUTCDate(fromVal);
    const toDate = toUTCDate(toVal);
    const fromMs = fromDate.getTime();
    const toMs = toDate.getTime();
    const nowMs = Date.now();

    if (fromMs >= toMs) {
      onError('Start must be before end');
      return;
    }
    if (toMs - fromMs > maxSpanMs) {
      const spanHours = Math.round(maxSpanMs / 3600_000);
      onError(
        `Range cannot exceed ${spanHours >= 24 ? `${Math.round(spanHours / 24)}d` : `${spanHours}h`}`,
      );
      return;
    }
    if (fromMs < nowMs - lookbackMs) {
      const lookbackHours = Math.round(lookbackMs / 3600_000);
      onError(
        `Start cannot be older than ${lookbackHours >= 24 ? `${Math.round(lookbackHours / 24)}d` : `${lookbackHours}h`}`,
      );
      return;
    }

    onApply(fromDate.toISOString(), toDate.toISOString());
  }, [fromVal, toVal, onApply, onError, maxSpanMs, lookbackMs]);

  return (
    <Box sx={{ px: 1.5, py: 1, minWidth: 240 }}>
      {/* From / To tab switcher */}
      <Box sx={{ display: 'flex', gap: 0.5, mb: 1 }}>
        {(['from', 'to'] as const).map((tab) => (
          <Box
            key={tab}
            data-testid={`crp-tab-${tab}`}
            data-active={activeTab === tab}
            onClick={() => setActiveTab(tab)}
            sx={{
              flex: 1,
              textAlign: 'center',
              py: 0.5,
              borderRadius: '4px',
              cursor: 'pointer',
              bgcolor: activeTab === tab ? 'rgba(255,255,255,0.08)' : 'transparent',
              borderBottom: activeTab === tab ? '2px solid' : '2px solid transparent',
              borderColor:
                activeTab === tab
                  ? tab === 'from'
                    ? 'warning.main'
                    : 'primary.main'
                  : 'transparent',
              transition: 'all 0.15s',
            }}
          >
            <Typography
              variant="caption"
              sx={{
                fontWeight: 700,
                fontSize: '0.65rem',
                fontFamily: 'monospace',
                color:
                  activeTab === tab
                    ? tab === 'from'
                      ? 'warning.main'
                      : 'primary.main'
                    : 'text.secondary',
              }}
            >
              {tab === 'from' ? 'FROM' : 'TO'}
            </Typography>
          </Box>
        ))}
      </Box>

      {/* Active picker */}
      {activeTab === 'from' ? (
        <DateTimePicker
          label="START"
          value={fromVal}
          onChange={handleFromChange}
          minDate={minDate}
          maxDate={maxDate}
        />
      ) : (
        <DateTimePicker
          label="END"
          value={toVal}
          onChange={handleToChange}
          minDate={minDate}
          maxDate={maxDate}
        />
      )}

      {/* Apply button */}
      <Box
        data-testid="crp-apply"
        onClick={handleApply}
        sx={{
          mt: 1.5,
          textAlign: 'center',
          py: 0.75,
          bgcolor: semanticColors.primary.alpha10,
          border: `1px solid ${alpha(theme.palette.primary.main, 0.25)}`,
          borderRadius: '4px',
          cursor: 'pointer',
          transition: 'all 0.15s',
          '&:hover': {
            bgcolor: alpha(theme.palette.primary.main, 0.18),
            borderColor: alpha(theme.palette.primary.main, 0.4),
          },
        }}
      >
        <Typography
          variant="caption"
          sx={{
            fontWeight: 700,
            fontSize: '0.7rem',
            color: 'primary.main',
            fontFamily: 'monospace',
          }}
        >
          APPLY RANGE
        </Typography>
      </Box>
    </Box>
  );
};

export default CustomRangePicker;
