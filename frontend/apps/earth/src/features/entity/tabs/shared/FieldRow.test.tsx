/**
 * FieldRow Component Tests
 *
 * Covers:
 * - Label and value rendering
 * - Variant-specific label styles (overview vs detail)
 * - Adaptive layout: side-by-side for short values, stacked for long values
 * - Custom stackThreshold
 * - Color and fontWeight pass-through to SmartValue
 * - URL detection and HTML stripping delegated via SmartValue
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import FieldRow from './FieldRow';
import { FONT_FAMILY } from '@respondent/core';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const renderWithTheme = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

describe('FieldRow', () => {
  describe('Basic Rendering', () => {
    it('renders label and value', () => {
      renderWithTheme(<FieldRow label="SOURCE" value="darkreading" />);
      expect(screen.getByText('SOURCE')).toBeInTheDocument();
      expect(screen.getByText('darkreading')).toBeInTheDocument();
    });

    it('renders empty value as em dash', () => {
      renderWithTheme(<FieldRow label="AUTHOR" value="—" />);
      expect(screen.getByText('AUTHOR')).toBeInTheDocument();
      expect(screen.getByText('—')).toBeInTheDocument();
    });
  });

  describe('Layout Adaptation', () => {
    it('uses row layout for short values (default threshold 50)', () => {
      const { container } = renderWithTheme(<FieldRow label="LAT" value="40.3737" />);
      const row = container.firstChild as HTMLElement;
      expect(row).toHaveStyle({ flexDirection: 'row' });
    });

    it('uses column layout for long values', () => {
      const longValue = 'A'.repeat(61); // must exceed DEFAULT_STACK_THRESHOLD (60) to trigger column
      const { container } = renderWithTheme(<FieldRow label="DESC" value={longValue} />);
      const row = container.firstChild as HTMLElement;
      expect(row).toHaveStyle({ flexDirection: 'column' });
    });

    it('uses row layout at exactly the threshold', () => {
      const exactValue = 'A'.repeat(50);
      const { container } = renderWithTheme(<FieldRow label="VAL" value={exactValue} />);
      const row = container.firstChild as HTMLElement;
      expect(row).toHaveStyle({ flexDirection: 'row' });
    });

    it('respects custom stackThreshold', () => {
      const { container } = renderWithTheme(
        <FieldRow label="VAL" value="medium text" stackThreshold={5} />,
      );
      const row = container.firstChild as HTMLElement;
      expect(row).toHaveStyle({ flexDirection: 'column' });
    });
  });

  describe('Variant Styles', () => {
    it('defaults to detail variant', () => {
      renderWithTheme(<FieldRow label="key" value="val" />);
      const label = screen.getByText('key');
      expect(label).toHaveStyle({ fontFamily: FONT_FAMILY });
    });

    it('applies overview variant label styles', () => {
      renderWithTheme(<FieldRow variant="overview" label="LAT" value="40.37" />);
      const label = screen.getByText('LAT');
      // Overview labels do not use monospace
      expect(label).not.toHaveStyle({ fontFamily: 'monospace' });
    });

    it('uses larger font size for overview values', () => {
      renderWithTheme(<FieldRow variant="overview" label="LAT" value="40.37" />);
      const value = screen.getByText('40.37');
      // overview uses DASHBOARD_TYPOGRAPHY.dashboardBase.fontSize = '0.8rem'
      expect(value).toHaveStyle({ fontSize: '0.8rem' });
    });

    it('uses smaller font size for detail values', () => {
      renderWithTheme(<FieldRow label="key" value="val" />);
      const value = screen.getByText('val');
      // detail uses DASHBOARD_TYPOGRAPHY.dashboardSm.fontSize = '0.75rem'
      expect(value).toHaveStyle({ fontSize: '0.75rem' });
    });

    it('uses medium weight for overview variant values', () => {
      renderWithTheme(<FieldRow variant="overview" label="LAT" value="40.37" />);
      const value = screen.getByText('40.37');
      // both variants default to fontWeight 500 unless explicitly overridden
      expect(value).toHaveStyle({ fontWeight: 500 });
    });

    it('uses medium weight for detail variant values', () => {
      renderWithTheme(<FieldRow label="key" value="val" />);
      const value = screen.getByText('val');
      // both variants default to fontWeight 500 unless explicitly overridden
      expect(value).toHaveStyle({ fontWeight: 500 });
    });
  });

  describe('Color and Weight Overrides', () => {
    it('passes color to SmartValue', () => {
      renderWithTheme(<FieldRow label="RISK" value="HIGH" color="#ff006e" />);
      const value = screen.getByText('HIGH');
      expect(value).toHaveStyle({ color: '#ff006e' });
    });

    it('passes fontWeight override to SmartValue', () => {
      renderWithTheme(<FieldRow label="RISK" value="HIGH" fontWeight={700} />);
      const value = screen.getByText('HIGH');
      expect(value).toHaveStyle({ fontWeight: 700 });
    });

    it('passes fontSize override to SmartValue', () => {
      renderWithTheme(<FieldRow label="KEY" value="val" fontSize="0.8rem" />);
      const value = screen.getByText('val');
      expect(value).toHaveStyle({ fontSize: '0.8rem' });
    });
  });

  describe('SmartValue Integration', () => {
    it('renders URLs as clickable links for short values', () => {
      renderWithTheme(<FieldRow label="URL" value="https://example.com" />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', 'https://example.com');
    });

    it('renders URLs as clickable links for long values', () => {
      const longUrl = 'https://example.com/' + 'a'.repeat(60);
      renderWithTheme(<FieldRow label="URL" value={longUrl} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', longUrl);
    });

    it('strips HTML tags from values', () => {
      renderWithTheme(<FieldRow label="DESC" value="<p>Hello <b>world</b></p>" />);
      expect(screen.getByText('Hello world')).toBeInTheDocument();
      expect(screen.queryByText(/<p>/)).not.toBeInTheDocument();
    });

    it('truncates long values with see more', () => {
      const longValue = 'A'.repeat(200);
      renderWithTheme(<FieldRow label="DESC" value={longValue} />);
      expect(screen.getByText('see more')).toBeInTheDocument();
    });

    it('does not truncate short values', () => {
      renderWithTheme(<FieldRow label="KEY" value="short" />);
      expect(screen.queryByText('see more')).not.toBeInTheDocument();
    });
  });
});
