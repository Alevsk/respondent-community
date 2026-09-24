/**
 * ConfigPanel - A foundational floating panel component for the Respondent UI.
 *
 * This component provides a consistent, reusable panel design that can be used
 * across the application for configuration, filtering, and settings interfaces.
 *
 * @example
 * const panel = (
 *   ConfigPanel({
 *     open: isOpen,
 *     onClose: () => setIsOpen(false),
 *     title: "Display Filters",
 *     icon: TuneIcon,
 *     statusLabel: "ACTIVE FILTER",
 *     statusValue: "CRT",
 *     footer: "Display filters apply visual effects to the entire map.",
 *     children: YourContent
 *   })
 * )
 */

import React from 'react';
import { Box, Typography, Divider, IconButton, Fade, SxProps, Theme } from '@mui/material';
import { CloseIcon, MinimizeIcon, OpenInFullIcon, CloseFullscreenIcon, iconSizes } from '../icons';
import { useResponsive, useDraggable, alpha, theme, semanticColors } from '@respondent/core';
import { usePanelLayoutStore, selectPanelZIndex } from '../layout/panelLayoutStore';

export type ConfigPanelDisplayMode = 'normal' | 'minimized' | 'maximized';

const DEFAULT_TRANSITION_PROPS: Omit<React.ComponentProps<typeof Fade>, 'children' | 'in'> = {
  timeout: 200,
};

export interface ConfigPanelProps {
  /**
   * Controls the visibility of the panel.
   * When false, the panel is hidden with a fade-out animation.
   */
  open: boolean;

  /**
   * Callback fired when the panel should close.
   * Triggered by clicking the close button or pressing Escape.
   */
  onClose: () => void;

  /**
   * The title displayed in the panel header.
   * Rendered in uppercase with primary color styling.
   */
  title: string;

  /**
   * Optional icon displayed next to the title in the header.
   * Should be a React element (e.g., MUI icon component).
   */
  icon?: React.ReactNode;

  /**
   * Optional status label displayed in a sub-header section.
   * Typically used to show current state (e.g., "ACTIVE FILTER").
   */
  statusLabel?: string;

  /**
   * Optional status value displayed alongside the status label.
   * Highlighted with primary color styling.
   */
  statusValue?: string;

  /**
   * Optional action element rendered next to the status value.
   * Typically a small icon button (e.g., clear/reset).
   */
  statusAction?: React.ReactNode;

  /**
   * Optional footer text displayed at the bottom of the panel.
   * Used for help text or descriptions.
   */
  footer?: string;

  /**
   * The main content of the panel.
   * Can be any React node(s).
   */
  children: React.ReactNode;

  /**
   * Optional additional styling applied to the root container.
   * Uses MUI's SxProps for flexible styling.
   */
  sx?: SxProps<Theme>;

  /**
   * Width of the panel. Defaults to 300px.
   */
  width?: number | string;

  /**
   * Position of the panel from the top.
   */
  top?: number | string;

  /**
   * Position of the panel from the bottom.
   */
  bottom?: number | string;

  /**
   * Position of the panel from the right. Defaults to 16px.
   */
  right?: number | string;

  /**
   * Position of the panel from the left.
   */
  left?: number | string;

  /**
   * Maximum height of the panel. Defaults to calc(100vh - 180px).
   */
  maxHeight?: number | string;

  /**
   * If true, hides the close button in the header.
   * Useful for panels that should only be closed programmatically.
   */
  hideCloseButton?: boolean;

  /**
   * If true, hides the divider between header and content.
   */
  hideHeaderDivider?: boolean;

  /**
   * Additional props passed to the Fade transition component.
   */
  TransitionProps?: Omit<React.ComponentProps<typeof Fade>, 'children' | 'in'>;

  /**
   * Custom data-testid for testing purposes.
   * Defaults to 'config-panel'.
   */
  'data-testid'?: string;

  /**
   * If true, shows a minimize button in the header.
   * When minimized, only the header bar is visible.
   */
  minimizable?: boolean;

  /**
   * If true, shows a maximize button in the header.
   * When maximized, the panel is centered and nearly full-screen.
   */
  maximizable?: boolean;

  /**
   * Current display mode of the panel.
   */
  displayMode?: ConfigPanelDisplayMode;

  /**
   * Callback fired when the display mode changes.
   */
  onDisplayModeChange?: (mode: ConfigPanelDisplayMode) => void;

  /**
   * Title to display when the panel is minimized.
   * Falls back to the regular title if not provided.
   */
  minimizedTitle?: string;

  /**
   * Content rendered between the status bar and the scrollable children area.
   * This content stays pinned and does not scroll (e.g., tab bars, toolbars).
   */
  stickyContent?: React.ReactNode;

  /**
   * Content rendered below the scrollable children area.
   * Stays pinned at the bottom and does not scroll (e.g., action buttons).
   */
  stickyFooter?: React.ReactNode;

  /**
   * Panel ID for layout system integration.
   * When provided, enables drag-to-reposition (via header) and click-to-focus (z-index).
   * When omitted, panel behaves exactly as before (backward compatible).
   */
  panelId?: string;
}

/**
 * ConfigPanel Component
 *
 * A styled floating panel with header, optional status bar, content area,
 * and optional footer. Designed for consistency across the Respondent UI.
 *
 * Features:
 * - Fade in/out animation
 * - Blur backdrop effect
 * - Consistent styling with theme colors
 * - Responsive positioning
 * - Accessible close button with tooltip
 */
const ConfigPanel: React.FC<ConfigPanelProps> = ({
  open,
  onClose,
  title,
  icon,
  statusLabel,
  statusValue,
  statusAction,
  footer,
  children,
  sx,
  width = 300,
  top,
  bottom,
  right,
  left,
  maxHeight = 'calc(100vh - 180px)',
  hideCloseButton = false,
  hideHeaderDivider = false,
  TransitionProps = DEFAULT_TRANSITION_PROPS,
  'data-testid': testId = 'config-panel',
  minimizable = false,
  maximizable = false,
  displayMode = 'normal',
  onDisplayModeChange,
  minimizedTitle,
  stickyContent,
  stickyFooter,
  panelId,
}) => {
  const { isMobile } = useResponsive();
  const panelRef = React.useRef<HTMLDivElement>(null);
  // The panel is not interactive the instant `open` flips: it fades in, and
  // until that finishes its contents cannot take focus. Until then focus is
  // still on whatever opened the panel — a toolbar toggle — so the next Space
  // or Enter re-triggers that button and closes the panel the user just
  // opened. Moving focus in when the transition completes is both the fix for
  // that and the dialog focus behaviour this role already promises.
  const [entered, setEntered] = React.useState(false);
  React.useEffect(() => {
    if (!open) setEntered(false);
  }, [open]);
  const isMinimized = displayMode === 'minimized';
  const isMaximized = displayMode === 'maximized';

  const bringToFront = usePanelLayoutStore((s) => s.bringToFront);
  const zIndex = usePanelLayoutStore((s) =>
    panelId ? selectPanelZIndex(s.zStack, panelId) : 1100,
  );
  const draggable = useDraggable({
    enabled: !!panelId && !isMaximized && !isMobile,
  });

  // On mobile, tapping the header toggles minimize (like MobileEntityPanel)
  const headerTappable = minimizable && isMobile;

  const handleMinimize = () => {
    onDisplayModeChange?.(isMinimized ? 'normal' : 'minimized');
  };

  const handleMaximize = () => {
    onDisplayModeChange?.(isMaximized ? 'normal' : 'maximized');
  };

  const displayTitle = isMinimized && minimizedTitle ? minimizedTitle : title;

  // Compute position and size based on display mode
  const positionSx = isMaximized
    ? {
        position: 'fixed' as const,
        top: '5vh',
        left: '5vw',
        right: '5vw',
        bottom: '5vh',
        width: '90vw',
        maxHeight: '90vh',
      }
    : {
        position: 'absolute' as const,
        ...(top !== undefined && { top }),
        ...(bottom !== undefined && { bottom }),
        ...(right !== undefined && { right }),
        ...(left !== undefined && { left }),
        width: isMinimized && !isMobile ? 'auto' : width,
        maxWidth: isMinimized && !isMobile ? (typeof width === 'number' ? width : 300) : undefined,
        maxHeight: isMinimized ? 'none' : maxHeight,
      };

  // z-index and transform are applied via inline style so they're testable and
  // always take precedence over class-based styles from MUI's sx prop.
  const rootStyle: React.CSSProperties = {
    zIndex: isMaximized ? 1300 : zIndex,
    ...(draggable.offset &&
      !isMaximized && {
        transform: `translate(${draggable.offset.x}px, ${draggable.offset.y}px)`,
      }),
    ...(draggable.isDragging && { transition: 'none' }),
  };

  return (
    <Fade
      in={open}
      {...TransitionProps}
      onEntered={(node, isAppearing) => {
        setEntered(true);
        panelRef.current?.focus({ preventScroll: true });
        TransitionProps?.onEntered?.(node, isAppearing);
      }}
    >
      <Box
        ref={panelRef}
        data-testid={testId}
        role="dialog"
        aria-labelledby="config-panel-title"
        aria-modal="false"
        // Focusable as a target for the focus move above, but not a tab stop.
        tabIndex={-1}
        // "open" means the fade has finished and the contents can take focus.
        // Anything waiting for the panel to be usable should wait for this
        // rather than for the element merely existing.
        data-state={entered ? 'open' : 'opening'}
        onMouseDown={panelId ? () => bringToFront(panelId) : undefined}
        style={rootStyle}
        sx={{
          ...positionSx,
          // The focus move on open is programmatic, not a tab stop, so it must
          // not paint a focus ring on the whole panel.
          '&:focus': { outline: 'none' },
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          bgcolor: 'rgba(5, 5, 5, 0.95)',
          borderRadius: 2,
          border: `1px solid ${semanticColors.primary.alpha20}`,
          backdropFilter: 'blur(16px)',
          boxShadow: `
            0 8px 32px rgba(0, 0, 0, 0.4),
            0 0 40px ${alpha(theme.palette.primary.main, 0.05)}
          `,
          transition: 'all 0.25s ease-in-out',
          ...sx,
        }}
      >
        {/* Header */}
        <Box
          data-testid="config-panel-header"
          onClick={headerTappable ? handleMinimize : undefined}
          onMouseDown={
            panelId && !isMobile && !isMaximized ? draggable.handleProps.onMouseDown : undefined
          }
          style={panelId && !isMobile && !isMaximized ? draggable.handleProps.style : undefined}
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            px: 2,
            py: 1.5,
            flexShrink: 0,
            borderBottom: isMinimized
              ? 'none'
              : hideHeaderDivider
                ? 'none'
                : '1px solid rgba(255, 255, 255, 0.1)',
            ...(headerTappable && {
              cursor: 'pointer',
              WebkitTapHighlightColor: 'transparent',
            }),
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
            {icon && (
              <Box
                data-testid="config-panel-icon"
                sx={{ color: 'primary.main', display: 'flex', flexShrink: 0 }}
              >
                {icon}
              </Box>
            )}
            <Typography
              id="config-panel-title"
              data-testid="config-panel-title"
              noWrap
              sx={{
                fontSize: '0.75rem',
                fontWeight: 700,
                letterSpacing: '0.1em',
                textTransform: 'uppercase',
                color: 'primary.main',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {displayTitle}
            </Typography>
          </Box>
          <Box
            sx={{ display: 'flex', alignItems: 'center', gap: 0.25, flexShrink: 0 }}
            onClick={headerTappable ? (e) => e.stopPropagation() : undefined}
          >
            {minimizable && (
              <IconButton
                size="small"
                onClick={handleMinimize}
                aria-label={isMinimized ? 'Restore panel' : 'Minimize panel'}
                data-testid="config-panel-minimize"
                sx={{
                  color: isMinimized ? 'primary.main' : 'text.secondary',
                  '&:hover': { color: 'primary.main' },
                  p: 0.5,
                }}
              >
                <MinimizeIcon size={iconSizes.sm} />
              </IconButton>
            )}
            {maximizable && (
              <IconButton
                size="small"
                onClick={handleMaximize}
                aria-label={isMaximized ? 'Restore panel' : 'Maximize panel'}
                data-testid="config-panel-maximize"
                sx={{
                  color: isMaximized ? 'primary.main' : 'text.secondary',
                  '&:hover': { color: 'primary.main' },
                  p: 0.5,
                }}
              >
                {isMaximized ? <CloseFullscreenIcon size={14} /> : <OpenInFullIcon size={14} />}
              </IconButton>
            )}
            {!hideCloseButton && (
              <IconButton
                size="small"
                onClick={onClose}
                aria-label="Close panel"
                data-testid="config-panel-close"
                sx={{
                  color: 'text.secondary',
                  '&:hover': { color: 'primary.main' },
                  p: 0.5,
                }}
              >
                <CloseIcon size={iconSizes.sm} />
              </IconButton>
            )}
          </Box>
        </Box>

        {/* Everything below the header is hidden when minimized */}
        {!isMinimized && (
          <>
            {/* Status Bar (optional) */}
            {statusLabel && statusValue && (
              <Box
                data-testid="config-panel-status"
                sx={{
                  px: 2,
                  py: 1.5,
                  flexShrink: 0,
                  bgcolor: alpha(theme.palette.primary.main, 0.03),
                  borderBottom: '1px solid rgba(255, 255, 255, 0.05)',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 1,
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
                    <Typography
                      variant="caption"
                      sx={{ color: 'text.secondary', fontSize: '0.65rem', flexShrink: 0 }}
                    >
                      {statusLabel}
                    </Typography>
                    <Typography
                      variant="caption"
                      data-testid="config-panel-status-value"
                      sx={{
                        color: 'primary.main',
                        fontWeight: 700,
                        letterSpacing: '0.05em',
                      }}
                    >
                      {statusValue}
                    </Typography>
                  </Box>
                  {statusAction && (
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      {statusAction}
                    </Box>
                  )}
                </Box>
              </Box>
            )}

            {/* Sticky content (e.g., tab bars) — does not scroll */}
            {stickyContent && (
              <Box data-testid="config-panel-sticky" sx={{ flexShrink: 0, px: 1.5, pt: 1 }}>
                {stickyContent}
              </Box>
            )}

            {/* Content — only this area scrolls */}
            <Box
              data-testid="config-panel-content"
              sx={{ p: 1.5, flex: 1, overflowY: 'auto', minHeight: 0 }}
            >
              {children}
            </Box>

            {/* Sticky footer (e.g., action buttons) — does not scroll */}
            {stickyFooter && (
              <Box
                data-testid="config-panel-sticky-footer"
                sx={{
                  flexShrink: 0,
                  px: 1.5,
                  pb: 1,
                  borderTop: '1px solid rgba(255, 255, 255, 0.06)',
                }}
              >
                {stickyFooter}
              </Box>
            )}

            {/* Footer (optional) */}
            {footer && (
              <>
                <Divider sx={{ borderColor: 'rgba(255, 255, 255, 0.1)', mx: 2, flexShrink: 0 }} />
                <Box data-testid="config-panel-footer" sx={{ px: 2, py: 1.5, flexShrink: 0 }}>
                  <Typography
                    variant="caption"
                    sx={{
                      color: 'text.secondary',
                      fontSize: '0.6rem',
                      display: 'block',
                      lineHeight: 1.5,
                    }}
                  >
                    {footer}
                  </Typography>
                </Box>
              </>
            )}
          </>
        )}
      </Box>
    </Fade>
  );
};

export default ConfigPanel;
