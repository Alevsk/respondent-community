import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import IndicatorGauge, { formatIndicatorValue, changeColor } from './IndicatorGauge';
import { levelColor, LEVEL_COLORS } from './indicatorColors';
import type { IndicatorValue } from '@/app/store';

describe('levelColor', () => {
  it('returns correct color for each level', () => {
    expect(levelColor(0)).toBe(LEVEL_COLORS[0]);
    expect(levelColor(1)).toBe(LEVEL_COLORS[1]);
    expect(levelColor(2)).toBe(LEVEL_COLORS[2]);
    expect(levelColor(3)).toBe(LEVEL_COLORS[3]);
    expect(levelColor(4)).toBe(LEVEL_COLORS[4]);
    expect(levelColor(5)).toBe(LEVEL_COLORS[5]);
  });

  it('clamps out-of-range levels', () => {
    expect(levelColor(-1)).toBe(LEVEL_COLORS[0]);
    expect(levelColor(10)).toBe(LEVEL_COLORS[5]);
  });
});

describe('formatIndicatorValue', () => {
  it('formats a number with precision', () => {
    const v: IndicatorValue = {
      key: 'vix',
      value: '24.31',
      label: 'VIX',
      unit: '',
      level: 2,
      format: 'number',
      precision: 2,
      prefix: '',
    };
    expect(formatIndicatorValue(v)).toBe('24.31');
  });

  it('formats currency with prefix and precision', () => {
    const v: IndicatorValue = {
      key: 'wti',
      value: '68.42',
      label: 'Crude Oil',
      unit: '',
      level: 1,
      format: 'currency',
      precision: 2,
      prefix: '$',
    };
    expect(formatIndicatorValue(v)).toBe('$68.42');
  });

  it('formats large currency with comma grouping', () => {
    const v: IndicatorValue = {
      key: 'gold',
      value: '2412',
      label: 'Gold',
      unit: '',
      level: 0,
      format: 'currency',
      precision: 0,
      prefix: '$',
    };
    const result = formatIndicatorValue(v);
    expect(result).toMatch(/^\$2,?412$/);
  });

  it('formats percent with precision', () => {
    const v: IndicatorValue = {
      key: 'us10y',
      value: '4.312',
      label: '10Y Yield',
      unit: '%',
      level: 0,
      format: 'percent',
      precision: 3,
      prefix: '',
    };
    expect(formatIndicatorValue(v)).toBe('4.312');
  });

  it('returns raw value for NaN', () => {
    const v: IndicatorValue = {
      key: 'x',
      value: 'N/A',
      label: 'X',
      unit: '',
      level: 0,
      format: 'number',
      precision: 2,
      prefix: '',
    };
    expect(formatIndicatorValue(v)).toBe('N/A');
  });

  it('returns value+unit for scale format', () => {
    const v: IndicatorValue = { key: 'r', value: '2', label: 'R', unit: 'nT', level: 2 };
    expect(formatIndicatorValue(v)).toBe('2 nT');
  });

  it('returns value without trailing space for scale format with no unit', () => {
    const v: IndicatorValue = { key: 'r', value: '2', label: 'R', unit: '', level: 2 };
    expect(formatIndicatorValue(v)).toBe('2');
  });
});

describe('changeColor', () => {
  it('returns neutral color for zero', () => {
    expect(changeColor(0)).toBe('rgba(255,255,255,0.4)');
  });

  it('returns green for positive change', () => {
    const color = changeColor(2.5);
    expect(color).toMatch(/^rgba\(0, 255, 157,/);
  });

  it('returns red for negative change', () => {
    const color = changeColor(-3.0);
    expect(color).toMatch(/^rgba\(255, 0, 110,/);
  });

  it('increases intensity with magnitude', () => {
    const low = changeColor(0.5);
    const high = changeColor(4.0);
    const lowOpacity = parseFloat(low.match(/[\d.]+\)$/)?.[0] ?? '0');
    const highOpacity = parseFloat(high.match(/[\d.]+\)$/)?.[0] ?? '0');
    expect(highOpacity).toBeGreaterThan(lowOpacity);
  });
});

describe('IndicatorGauge', () => {
  const baseValue: IndicatorValue = {
    key: 'r_scale',
    value: '2',
    label: 'Radio Blackout',
    unit: '',
    level: 2,
  };

  it('renders value and label', () => {
    render(<IndicatorGauge value={baseValue} />);
    expect(screen.getByText('2')).toBeTruthy();
    expect(screen.getByText('Radio Blackout')).toBeTruthy();
  });

  it('renders with correct test id', () => {
    render(<IndicatorGauge value={baseValue} />);
    expect(screen.getByTestId('indicator-gauge-r_scale')).toBeTruthy();
  });

  it('renders zero level', () => {
    const zeroVal: IndicatorValue = { ...baseValue, value: '0', level: 0 };
    render(<IndicatorGauge value={zeroVal} />);
    expect(screen.getByText('0')).toBeTruthy();
  });

  it('renders unit suffix when present', () => {
    const withUnit: IndicatorValue = { ...baseValue, value: '42', unit: 'nT' };
    render(<IndicatorGauge value={withUnit} />);
    expect(screen.getByText('42 nT')).toBeTruthy();
  });

  it('omits unit suffix when empty', () => {
    render(<IndicatorGauge value={baseValue} />);
    expect(screen.getByText('2')).toBeTruthy();
  });

  it('renders number format with value and change %', () => {
    const v: IndicatorValue = {
      key: 'vix',
      value: '24.31',
      label: 'VIX',
      unit: '',
      level: 2,
      format: 'number',
      precision: 2,
      changePct: 8.2,
    };
    render(<IndicatorGauge value={v} />);
    expect(screen.getByText('24.31')).toBeTruthy();
    expect(screen.getByText('VIX')).toBeTruthy();
    expect(screen.getByText('+8.20%')).toBeTruthy();
  });

  it('renders currency format with prefix', () => {
    const v: IndicatorValue = {
      key: 'wti',
      value: '68.42',
      label: 'Crude Oil',
      unit: '',
      level: 1,
      format: 'currency',
      precision: 2,
      prefix: '$',
      changePct: -1.5,
    };
    render(<IndicatorGauge value={v} />);
    expect(screen.getByText('$68.42')).toBeTruthy();
    expect(screen.getByText('-1.50%')).toBeTruthy();
  });

  it('renders percent format', () => {
    const v: IndicatorValue = {
      key: 'us10y',
      value: '4.312',
      label: '10Y Yield',
      unit: '%',
      level: 0,
      format: 'percent',
      precision: 3,
      changePct: 0.3,
    };
    render(<IndicatorGauge value={v} />);
    expect(screen.getByText('4.312%')).toBeTruthy();
  });
});
