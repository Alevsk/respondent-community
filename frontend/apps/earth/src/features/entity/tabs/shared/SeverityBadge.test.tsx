/**
 * SeverityBadge Component Tests
 *
 * Covers:
 * - Renders severity text
 * - Low severity: green-tinted background
 * - Moderate severity: amber-tinted background
 * - High severity: red-tinted background
 * - Critical severity: dark red background
 * - Unknown severity: falls back to neutral gray
 * - Case-insensitive severity matching
 * - text is capitalized via textTransform
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import SeverityBadge from './SeverityBadge';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const renderWithTheme = (ui: React.ReactElement) =>
  render(ui, {
    wrapper: ({ children }) => <ThemeProvider theme={theme}>{children}</ThemeProvider>,
  });

describe('SeverityBadge', () => {
  describe('Text Rendering', () => {
    it('renders "low" severity text', () => {
      renderWithTheme(<SeverityBadge severity="low" />);
      expect(screen.getByText('low')).toBeInTheDocument();
    });

    it('renders "moderate" severity text', () => {
      renderWithTheme(<SeverityBadge severity="moderate" />);
      expect(screen.getByText('moderate')).toBeInTheDocument();
    });

    it('renders "high" severity text', () => {
      renderWithTheme(<SeverityBadge severity="high" />);
      expect(screen.getByText('high')).toBeInTheDocument();
    });

    it('renders "critical" severity text', () => {
      renderWithTheme(<SeverityBadge severity="critical" />);
      expect(screen.getByText('critical')).toBeInTheDocument();
    });

    it('renders unknown severity text verbatim', () => {
      renderWithTheme(<SeverityBadge severity="unknown" />);
      expect(screen.getByText('unknown')).toBeInTheDocument();
    });

    it('applies textTransform capitalize to severity text', () => {
      renderWithTheme(<SeverityBadge severity="high" />);
      const text = screen.getByText('high');
      expect(text).toHaveStyle({ textTransform: 'capitalize' });
    });

    it('applies fontWeight 600 to severity text', () => {
      renderWithTheme(<SeverityBadge severity="low" />);
      const text = screen.getByText('low');
      expect(text).toHaveStyle({ fontWeight: 600 });
    });
  });

  describe('Color Mapping', () => {
    it('applies green text color for low severity', () => {
      renderWithTheme(<SeverityBadge severity="low" />);
      const text = screen.getByText('low');
      expect(text).toHaveStyle({ color: '#4caf50' });
    });

    it('applies amber text color for moderate severity', () => {
      renderWithTheme(<SeverityBadge severity="moderate" />);
      const text = screen.getByText('moderate');
      expect(text).toHaveStyle({ color: '#ff9800' });
    });

    it('applies red text color for high severity', () => {
      renderWithTheme(<SeverityBadge severity="high" />);
      const text = screen.getByText('high');
      expect(text).toHaveStyle({ color: '#f44336' });
    });

    it('applies dark red text color for critical severity', () => {
      renderWithTheme(<SeverityBadge severity="critical" />);
      const text = screen.getByText('critical');
      expect(text).toHaveStyle({ color: '#d32f2f' });
    });

    it('applies neutral gray background for unknown severity', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="unknown" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ backgroundColor: 'rgba(255,255,255,0.08)' });
    });

    it('applies green-tinted background for low severity', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="low" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ backgroundColor: 'rgba(76, 175, 80, 0.15)' });
    });

    it('applies amber-tinted background for moderate severity', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="moderate" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ backgroundColor: 'rgba(255, 152, 0, 0.15)' });
    });

    it('applies red-tinted background for high severity', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="high" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ backgroundColor: 'rgba(244, 67, 54, 0.15)' });
    });

    it('applies dark red background for critical severity', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="critical" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ backgroundColor: 'rgba(211, 47, 47, 0.25)' });
    });
  });

  describe('Case Insensitivity', () => {
    it('matches "LOW" (uppercase) to green color', () => {
      renderWithTheme(<SeverityBadge severity="LOW" />);
      const text = screen.getByText('LOW');
      expect(text).toHaveStyle({ color: '#4caf50' });
    });

    it('matches "High" (mixed case) to red color', () => {
      renderWithTheme(<SeverityBadge severity="High" />);
      const text = screen.getByText('High');
      expect(text).toHaveStyle({ color: '#f44336' });
    });

    it('matches "CRITICAL" (uppercase) to dark red color', () => {
      renderWithTheme(<SeverityBadge severity="CRITICAL" />);
      const text = screen.getByText('CRITICAL');
      expect(text).toHaveStyle({ color: '#d32f2f' });
    });
  });

  describe('Badge Layout', () => {
    it('renders as an inline-flex container', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="low" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ display: 'inline-flex' });
    });

    it('renders with a border radius', () => {
      const { container } = renderWithTheme(<SeverityBadge severity="low" />);
      const badge = container.firstChild as HTMLElement;
      expect(badge).toHaveStyle({ borderRadius: '4px' });
    });
  });
});
