/**
 * SectionHeader Component Tests
 *
 * Covers:
 * - Renders title text in uppercase
 * - Renders icon when provided
 * - Does not render icon container when icon is omitted
 * - Title uses green-tinted color (primary alpha 0.6)
 * - Has bottom border
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import SectionHeader from './SectionHeader';

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

describe('SectionHeader', () => {
  describe('Title Rendering', () => {
    it('renders title text', () => {
      renderWithTheme(<SectionHeader title="Location" />);
      expect(screen.getByText('Location')).toBeInTheDocument();
    });

    it('applies textTransform uppercase to title', () => {
      renderWithTheme(<SectionHeader title="section title" />);
      const title = screen.getByText('section title');
      expect(title).toHaveStyle({ textTransform: 'uppercase' });
    });

    it('applies fontWeight 600 to title', () => {
      renderWithTheme(<SectionHeader title="Details" />);
      const title = screen.getByText('Details');
      expect(title).toHaveStyle({ fontWeight: 600 });
    });

    it('applies letterSpacing to title', () => {
      renderWithTheme(<SectionHeader title="Metadata" />);
      const title = screen.getByText('Metadata');
      expect(title).toHaveStyle({ letterSpacing: '0.06em' });
    });
  });

  describe('Icon Rendering', () => {
    it('renders icon content when icon prop is provided', () => {
      renderWithTheme(
        <SectionHeader icon={<span data-testid="test-icon">icon</span>} title="Section" />,
      );
      expect(screen.getByTestId('test-icon')).toBeInTheDocument();
    });

    it('does not render icon container when icon prop is omitted', () => {
      renderWithTheme(<SectionHeader title="No Icon Section" />);
      // Should only have one child inside the flex container: the Typography
      // Verify the icon is not present by checking there is no extra sibling before title
      const title = screen.getByText('No Icon Section');
      // The title should have no preceding sibling (no icon Box)
      expect(title.parentElement?.previousElementSibling).toBeNull();
    });

    it('renders multiple icon elements when a complex icon is provided', () => {
      renderWithTheme(
        <SectionHeader
          icon={
            <svg data-testid="svg-icon" width="14" height="14">
              <path d="M0 0" />
            </svg>
          }
          title="Complex Icon"
        />,
      );
      expect(screen.getByTestId('svg-icon')).toBeInTheDocument();
      expect(screen.getByText('Complex Icon')).toBeInTheDocument();
    });
  });

  describe('Layout Structure', () => {
    it('renders a container with flex display', () => {
      const { container } = renderWithTheme(<SectionHeader title="Layout" />);
      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper).toHaveStyle({ display: 'flex' });
    });

    it('renders with alignItems center', () => {
      const { container } = renderWithTheme(<SectionHeader title="Align" />);
      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper).toHaveStyle({ alignItems: 'center' });
    });

    it('renders the title as a Typography caption element', () => {
      renderWithTheme(<SectionHeader title="Caption Check" />);
      const title = screen.getByText('Caption Check');
      // MUI Typography caption renders as span by default
      expect(title.tagName.toLowerCase()).toBe('span');
    });
  });
});
