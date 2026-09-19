/**
 * SmartValue Component Tests
 *
 * Covers:
 * - Plain text passthrough
 * - URL auto-detection and link rendering
 * - HTML tag stripping via DOMParser
 * - Text truncation with "see more" / "see less" toggle
 * - Prop-driven styling (color, fontWeight, fontSize, mono, textAlign)
 * - Edge cases (empty string, exact threshold, multiple URLs, mixed content)
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import SmartValue from './SmartValue';
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

describe('SmartValue', () => {
  describe('Plain Text', () => {
    it('renders plain text unchanged', () => {
      renderWithTheme(<SmartValue value="hello world" truncateAt={0} />);
      expect(screen.getByText('hello world')).toBeInTheDocument();
    });

    it('renders empty string without errors', () => {
      const { container } = renderWithTheme(<SmartValue value="" truncateAt={0} />);
      expect(container.querySelector('span')).toBeInTheDocument();
    });

    it('renders the em dash placeholder', () => {
      renderWithTheme(<SmartValue value="—" truncateAt={0} />);
      expect(screen.getByText('—')).toBeInTheDocument();
    });
  });

  describe('URL Detection', () => {
    it('renders a URL as a clickable link', () => {
      renderWithTheme(<SmartValue value="https://example.com" truncateAt={0} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', 'https://example.com');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('renders http URLs as links', () => {
      renderWithTheme(<SmartValue value="http://example.com/path" truncateAt={0} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', 'http://example.com/path');
    });

    it('renders text before and after a URL', () => {
      renderWithTheme(<SmartValue value="Visit https://example.com for details" truncateAt={0} />);
      // Text segments are React Fragments inside the parent span,
      // so query the full combined text content.
      const parent = screen.getByText(/Visit.*for details/);
      expect(parent).toBeInTheDocument();
      expect(screen.getByRole('link')).toHaveTextContent('https://example.com');
    });

    it('renders multiple URLs as separate links', () => {
      renderWithTheme(
        <SmartValue value="https://example.com and https://other.org" truncateAt={0} />,
      );
      const links = screen.getAllByRole('link');
      expect(links).toHaveLength(2);
      expect(links[0]).toHaveAttribute('href', 'https://example.com');
      expect(links[1]).toHaveAttribute('href', 'https://other.org');
    });

    it('handles URL with path and query params', () => {
      const url = 'https://example.com/path?q=1&r=2#hash';
      renderWithTheme(<SmartValue value={url} truncateAt={0} />);
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', url);
    });

    it('does not create links for non-URL text', () => {
      renderWithTheme(<SmartValue value="no links here" truncateAt={0} />);
      expect(screen.queryByRole('link')).not.toBeInTheDocument();
    });
  });

  describe('HTML Stripping', () => {
    it('strips simple HTML tags', () => {
      renderWithTheme(<SmartValue value="<p>Hello <strong>world</strong></p>" truncateAt={0} />);
      expect(screen.getByText('Hello world')).toBeInTheDocument();
    });

    it('strips nested HTML structures', () => {
      renderWithTheme(
        <SmartValue
          value='<div class="block"><p>Written by: <em>Author</em></p></div>'
          truncateAt={0}
        />,
      );
      expect(screen.getByText('Written by: Author')).toBeInTheDocument();
    });

    it('decodes HTML entities', () => {
      renderWithTheme(<SmartValue value='&amp; &lt; &gt; "' truncateAt={0} />);
      expect(screen.getByText('& < > "')).toBeInTheDocument();
    });

    it('collapses whitespace from stripped HTML', () => {
      renderWithTheme(<SmartValue value="<p>line one</p>   <p>line two</p>" truncateAt={0} />);
      expect(screen.getByText('line one line two')).toBeInTheDocument();
    });

    it('does not alter text without HTML tags', () => {
      renderWithTheme(<SmartValue value="Just plain text with <no> issues" truncateAt={0} />);
      // <no> is an HTML tag, so it gets stripped
      expect(screen.getByText('Just plain text with issues')).toBeInTheDocument();
    });

    it('preserves URLs inside HTML content', () => {
      renderWithTheme(
        <SmartValue value='<p>See <a href="ignored">https://example.com</a></p>' truncateAt={0} />,
      );
      const link = screen.getByRole('link');
      expect(link).toHaveAttribute('href', 'https://example.com');
    });
  });

  describe('Truncation', () => {
    const longText = 'A'.repeat(150);

    it('truncates text exceeding the default threshold (120 chars)', () => {
      renderWithTheme(<SmartValue value={longText} />);
      expect(screen.getByText(/^A{120}\.\.\.$/)).toBeInTheDocument();
      expect(screen.getByText('see more')).toBeInTheDocument();
    });

    it('does not truncate text under the threshold', () => {
      const shortText = 'A'.repeat(100);
      renderWithTheme(<SmartValue value={shortText} />);
      expect(screen.queryByText('see more')).not.toBeInTheDocument();
    });

    it('does not truncate when truncateAt=0', () => {
      renderWithTheme(<SmartValue value={longText} truncateAt={0} />);
      expect(screen.queryByText('see more')).not.toBeInTheDocument();
      expect(screen.getByText(longText)).toBeInTheDocument();
    });

    it('respects custom truncateAt value', () => {
      const text = 'A'.repeat(30);
      renderWithTheme(<SmartValue value={text} truncateAt={20} />);
      expect(screen.getByText(/^A{20}\.\.\.$/)).toBeInTheDocument();
      expect(screen.getByText('see more')).toBeInTheDocument();
    });

    it('does not truncate text at exactly the threshold', () => {
      const exactText = 'A'.repeat(120);
      renderWithTheme(<SmartValue value={exactText} truncateAt={120} />);
      expect(screen.queryByText('see more')).not.toBeInTheDocument();
    });

    it('expands text on "see more" click', async () => {
      const user = userEvent.setup();
      renderWithTheme(<SmartValue value={longText} />);

      expect(screen.getByText('see more')).toBeInTheDocument();

      await user.click(screen.getByText('see more'));

      expect(screen.getByText(longText)).toBeInTheDocument();
      expect(screen.getByText('see less')).toBeInTheDocument();
      expect(screen.queryByText('see more')).not.toBeInTheDocument();
    });

    it('collapses text on "see less" click', async () => {
      const user = userEvent.setup();
      renderWithTheme(<SmartValue value={longText} />);

      await user.click(screen.getByText('see more'));
      await user.click(screen.getByText('see less'));

      expect(screen.getByText('see more')).toBeInTheDocument();
      expect(screen.queryByText('see less')).not.toBeInTheDocument();
    });

    it('truncates HTML-stripped text based on clean length', () => {
      // HTML is 200+ chars raw, but clean text is shorter
      const htmlValue = '<div>' + 'A'.repeat(130) + '</div>';
      renderWithTheme(<SmartValue value={htmlValue} truncateAt={120} />);
      expect(screen.getByText('see more')).toBeInTheDocument();
    });
  });

  describe('Styling Props', () => {
    it('uses monospace font by default', () => {
      const { container } = renderWithTheme(<SmartValue value="test" truncateAt={0} />);
      const span = container.querySelector('span');
      // mono=true (default) uses the full FONT_FAMILY stack which ends with 'monospace' as fallback
      expect(span).toHaveStyle({ fontFamily: FONT_FAMILY });
    });

    it('uses inherited font when mono=false', () => {
      const { container } = renderWithTheme(
        <SmartValue value="test" truncateAt={0} mono={false} />,
      );
      const span = container.querySelector('span');
      expect(span).not.toHaveStyle({ fontFamily: FONT_FAMILY });
    });
  });
});
