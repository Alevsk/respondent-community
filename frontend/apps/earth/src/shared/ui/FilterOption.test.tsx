/**
 * FilterOption Component Tests
 *
 * Tests for the styled option component used within filter/configuration panels.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import FilterOption, { FilterOptionProps } from './FilterOption';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const TestWrapper: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <ThemeProvider theme={theme}>{children}</ThemeProvider>
);

const renderWithTheme = (ui: React.ReactElement) => render(ui, { wrapper: TestWrapper });

describe('FilterOption', () => {
  const defaultProps: FilterOptionProps = {
    icon: '◉',
    label: 'Normal',
    description: 'Standard view',
    onClick: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render with icon, label, and description', () => {
      renderWithTheme(<FilterOption {...defaultProps} />);
      expect(screen.getByText('◉')).toBeInTheDocument();
      // Label is rendered as-is, CSS handles uppercase transformation
      expect(screen.getByText('Normal')).toBeInTheDocument();
      expect(screen.getByText('Standard view')).toBeInTheDocument();
    });

    it('should render React node as icon', () => {
      renderWithTheme(
        <FilterOption {...defaultProps} icon={<span data-testid="custom-icon">🔧</span>} />,
      );
      expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
    });

    it('should generate testid from label when id is not provided', () => {
      renderWithTheme(<FilterOption {...defaultProps} label="Night Vision" />);
      expect(screen.getByTestId('filter-option-night-vision')).toBeInTheDocument();
    });

    it('should use id for testid when provided', () => {
      renderWithTheme(<FilterOption {...defaultProps} id="nvg-mode" />);
      expect(screen.getByTestId('filter-option-nvg-mode')).toBeInTheDocument();
    });
  });

  describe('Active State', () => {
    it('should not show active indicator when active is false', () => {
      renderWithTheme(<FilterOption {...defaultProps} active={false} />);
      expect(screen.queryByTestId('filter-option-normal-indicator')).not.toBeInTheDocument();
    });

    it('should show active indicator when active is true', () => {
      renderWithTheme(<FilterOption {...defaultProps} active={true} />);
      expect(screen.getByTestId('filter-option-normal-indicator')).toBeInTheDocument();
    });

    it('should have data-active attribute reflecting active state', () => {
      const { rerender } = renderWithTheme(<FilterOption {...defaultProps} active={false} />);
      expect(screen.getByTestId('filter-option-normal')).toHaveAttribute('data-active', 'false');

      rerender(
        <TestWrapper>
          <FilterOption {...defaultProps} active={true} />
        </TestWrapper>,
      );
      expect(screen.getByTestId('filter-option-normal')).toHaveAttribute('data-active', 'true');
    });
  });

  describe('Interactions', () => {
    it('should call onClick when clicked', async () => {
      const onClick = vi.fn();
      renderWithTheme(<FilterOption {...defaultProps} onClick={onClick} />);

      await userEvent.click(screen.getByTestId('filter-option-normal'));
      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('should be keyboard accessible with Enter key', async () => {
      const onClick = vi.fn();
      renderWithTheme(<FilterOption {...defaultProps} onClick={onClick} />);

      const option = screen.getByTestId('filter-option-normal');
      option.focus();
      fireEvent.keyDown(option, { key: 'Enter' });

      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('should be keyboard accessible with Space key', async () => {
      const onClick = vi.fn();
      renderWithTheme(<FilterOption {...defaultProps} onClick={onClick} />);

      const option = screen.getByTestId('filter-option-normal');
      option.focus();
      fireEvent.keyDown(option, { key: ' ' });

      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('should not trigger onClick on other keys', async () => {
      const onClick = vi.fn();
      renderWithTheme(<FilterOption {...defaultProps} onClick={onClick} />);

      const option = screen.getByTestId('filter-option-normal');
      option.focus();
      fireEvent.keyDown(option, { key: 'Tab' });

      expect(onClick).not.toHaveBeenCalled();
    });
  });

  describe('Accessibility', () => {
    it('should have role="option"', () => {
      renderWithTheme(<FilterOption {...defaultProps} />);
      expect(screen.getByRole('option')).toBeInTheDocument();
    });

    it('should have aria-selected reflecting active state', () => {
      const { rerender } = renderWithTheme(<FilterOption {...defaultProps} active={false} />);
      expect(screen.getByRole('option')).toHaveAttribute('aria-selected', 'false');

      rerender(
        <TestWrapper>
          <FilterOption {...defaultProps} active={true} />
        </TestWrapper>,
      );
      expect(screen.getByRole('option')).toHaveAttribute('aria-selected', 'true');
    });

    it('should be focusable (tabIndex={0})', () => {
      renderWithTheme(<FilterOption {...defaultProps} />);
      expect(screen.getByRole('option')).toHaveAttribute('tabindex', '0');
    });
  });

  describe('Styling', () => {
    it('should apply custom sx props', () => {
      renderWithTheme(<FilterOption {...defaultProps} sx={{ marginTop: '10px' }} />);
      const option = screen.getByTestId('filter-option-normal');
      expect(option).toHaveStyle({ marginTop: '10px' });
    });
  });
});
