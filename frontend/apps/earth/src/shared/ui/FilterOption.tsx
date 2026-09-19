/**
 * FilterOption - A styled option component for use within filter/configuration panels.
 *
 * Displays an icon, label, description, and optional active indicator.
 * Designed for selecting from a list of mutually exclusive options.
 *
 * @example
 * ```tsx
 * <FilterOption
 *   icon="◉"
 *   label="Normal"
 *   description="Standard view with subtle vignette"
 *   active={currentFilter === 'NORMAL'}
 *   onClick={() => setFilter('NORMAL')}
 * />
 * ```
 */

import React from 'react';
import { Box, Typography, SxProps, Theme } from '@mui/material';
import { alpha, theme, semanticColors } from '@respondent/core';

export interface FilterOptionProps {
  /**
   * Icon or symbol displayed in the option box.
   * Can be a string (emoji/symbol) or React node.
   */
  icon: React.ReactNode;

  /**
   * Primary label text for the option.
   * Displayed in uppercase with bold styling.
   */
  label: string;

  /**
   * Secondary description text explaining the option.
   * Displayed in a smaller, muted font below the label.
   */
  description: string;

  /**
   * Whether this option is currently selected/active.
   * When true, applies highlight styling and shows indicator.
   */
  active?: boolean;

  /**
   * Callback fired when the option is clicked.
   */
  onClick: () => void;

  /**
   * Optional unique identifier for the option.
   * Used for data-testid attribute.
   */
  id?: string;

  /**
   * Optional additional styling.
   */
  sx?: SxProps<Theme>;
}

/**
 * FilterOption Component
 *
 * A clickable option card with icon, label, description, and active state.
 * Uses consistent styling with the Respondent UI theme.
 */
const FilterOption: React.FC<FilterOptionProps> = ({
  icon,
  label,
  description,
  active = false,
  onClick,
  id,
  sx,
}) => {
  const testId = id
    ? `filter-option-${id}`
    : `filter-option-${label.toLowerCase().replace(/\s+/g, '-')}`;

  return (
    <Box
      data-testid={testId}
      data-active={active}
      onClick={onClick}
      role="option"
      aria-selected={active}
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onClick();
        }
      }}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        p: 1.5,
        borderRadius: 2,
        cursor: 'pointer',
        transition: 'all 0.2s ease',
        border: '1px solid transparent',
        bgcolor: active ? alpha(theme.palette.primary.main, 0.08) : 'transparent',
        '&:hover': {
          bgcolor: active ? alpha(theme.palette.primary.main, 0.12) : 'rgba(255, 255, 255, 0.03)',
          borderColor: semanticColors.primary.alpha20,
        },
        '&:focus-visible': {
          outline: '2px solid',
          outlineColor: 'primary.main',
          outlineOffset: 2,
        },
        ...(active && {
          borderColor: 'primary.main',
          boxShadow: `0 0 15px ${alpha(theme.palette.primary.main, 0.15)}`,
        }),
        ...sx,
      }}
    >
      {/* Icon Box */}
      <Box
        sx={{
          width: 40,
          height: 40,
          borderRadius: 1.5,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: '1.2rem',
          bgcolor: active ? alpha(theme.palette.primary.main, 0.15) : 'rgba(255, 255, 255, 0.05)',
          color: active ? 'primary.main' : 'text.secondary',
          border: active
            ? `1px solid ${semanticColors.primary.alpha30}`
            : '1px solid rgba(255, 255, 255, 0.1)',
          flexShrink: 0,
        }}
      >
        {icon}
      </Box>

      {/* Label and Description */}
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            color: active ? 'primary.main' : 'text.primary',
            display: 'block',
          }}
        >
          {label}
        </Typography>
        <Typography
          variant="caption"
          sx={{
            color: 'text.secondary',
            fontSize: '0.65rem',
            display: 'block',
            mt: 0.25,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {description}
        </Typography>
      </Box>

      {/* Active Indicator */}
      {active && (
        <Box
          data-testid={`${testId}-indicator`}
          sx={{
            width: 8,
            height: 8,
            borderRadius: '50%',
            bgcolor: 'primary.main',
            boxShadow: `0 0 8px ${alpha(theme.palette.primary.main, 0.6)}`,
            flexShrink: 0,
          }}
        />
      )}
    </Box>
  );
};

export default FilterOption;
