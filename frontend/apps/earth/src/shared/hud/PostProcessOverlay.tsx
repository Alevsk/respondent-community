/**
 * PostProcessOverlay - Visual effects wrapper for the Cesium globe.
 *
 * Applies CSS-based post-processing effects to simulate different viewing modes:
 * - NORMAL: Subtle vignette for depth
 * - CRT: Retro CRT monitor effect with scanlines, glow, and noise
 * - NVG: Night vision goggle simulation with green tint
 * - FLIR: Thermal/infrared "white-hot" imaging simulation
 *
 * This component wraps its children so it can apply both:
 * 1. CSS `filter` on the container (e.g. grayscale for FLIR) — affects all children
 * 2. Overlay layers rendered on top (noise, vignettes, scanlines, etc.)
 *
 * @example
 * ```tsx
 * <PostProcessOverlay preset="FLIR">
 *   <CesiumViewer />
 * </PostProcessOverlay>
 * ```
 */

import React, { useMemo } from 'react';
import { Box, BoxProps, keyframes, useMediaQuery, useTheme } from '@mui/material';
import { alpha, theme as appTheme, semanticColors } from '@respondent/core';
import { FilterPreset } from '@/app/store';

export interface PostProcessOverlayProps extends Omit<BoxProps, 'preset'> {
  /**
   * The visual filter preset to apply.
   */
  preset: FilterPreset;
  /**
   * Content to wrap (e.g. Cesium viewer container). The preset's CSS filter
   * is applied to a wrapper around these children, ensuring entities and map
   * tiles are both affected.
   */
  children?: React.ReactNode;
}

// Keyframe animations
const scanlineMove = keyframes`
  0% { transform: translateY(0); }
  100% { transform: translateY(4px); }
`;

const crtFlicker = keyframes`
  0% { opacity: 0.98; }
  50% { opacity: 1.02; }
  100% { opacity: 0.98; }
`;

const nvgFlicker = keyframes`
  0% { opacity: 0.04; }
  50% { opacity: 0.06; }
  100% { opacity: 0.04; }
`;

/**
 * Returns a CSS `filter` string to apply on the content container for a given
 * preset, or `undefined` when no container-level filter is needed.
 *
 * Centralises all preset → filter knowledge so consumers never need to
 * know which presets require container-level CSS filters.
 */
const getContainerFilter = (preset: FilterPreset): string | undefined => {
  switch (preset) {
    case 'NVG':
      // Pre-boost luminance separation so bright entities glow brighter
      // through the green color overlay, preserving detail differentiation
      return 'brightness(1.2) contrast(1.3)';
    case 'FLIR':
      // 85% grayscale retains a faint color tint on highly-saturated entities
      // (red disasters, orange fires) so they remain distinguishable.
      // Higher contrast pushes bright entity icons away from the darker map.
      return 'grayscale(0.85) contrast(1.7) brightness(1.1)';
    default:
      return undefined;
  }
};

/**
 * PostProcessOverlay Component
 *
 * Wraps globe content and renders CSS-based visual effects on top.
 * Presets that require container-level CSS filters (e.g. FLIR grayscale)
 * apply them to the wrapper automatically — consumers don't need to know
 * about individual preset implementation details.
 */
const PostProcessOverlay = React.memo<PostProcessOverlayProps>(({ preset, children, ...props }) => {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down('md'));
  const containerFilter = getContainerFilter(preset);

  const overlays = useMemo(() => {
    switch (preset) {
      case 'CRT':
        return (
          <>
            {/* CRT curvature container */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                zIndex: 9990,
                overflow: 'hidden',
                borderRadius: '2%',
                boxShadow: 'inset 0 0 150px 60px rgba(0, 0, 0, 0.8)',
              }}
            />

            {/* Scanlines overlay — wider gap on mobile to avoid moire */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: isMobile
                  ? `repeating-linear-gradient(
                      0deg,
                      rgba(0, 0, 0, 0.12) 0px,
                      rgba(0, 0, 0, 0.12) 1px,
                      transparent 1px,
                      transparent 3px
                    )`
                  : `repeating-linear-gradient(
                      0deg,
                      rgba(0, 0, 0, 0.15) 0px,
                      rgba(0, 0, 0, 0.15) 1px,
                      transparent 1px,
                      transparent 2px
                    )`,
                zIndex: 9991,
                animation: `${scanlineMove} 0.1s linear infinite, ${crtFlicker} 0.1s ease-in-out infinite`,
              }}
            />

            {/* Horizontal scan beam */}
            <Box
              sx={{
                position: 'absolute',
                left: 0,
                right: 0,
                height: '3px',
                pointerEvents: 'none',
                background: `linear-gradient(180deg, transparent, ${semanticColors.primary.alpha10}, transparent)`,
                zIndex: 9992,
                animation: `${scanlineMove} 8s linear infinite`,
                top: '-3px',
              }}
            />

            {/* Color separation / chromatic aberration */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: `
                  linear-gradient(90deg,
                    rgba(255, 0, 0, 0.03) 0%,
                    transparent 10%,
                    transparent 90%,
                    rgba(0, 255, 255, 0.03) 100%
                  )
                `,
                zIndex: 9993,
              }}
            />

            {/* Phosphor glow effect */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                boxShadow: `inset 0 0 100px ${alpha(appTheme.palette.primary.main, 0.05)}`,
                zIndex: 9994,
              }}
            />

            {/* Analog noise/static */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                opacity: 0.08,
                zIndex: 9995,
                background: `url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E")`,
                mixBlendMode: 'overlay',
              }}
            />

            {/* Vignette */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'radial-gradient(ellipse at center, transparent 0%, transparent 50%, rgba(0, 0, 0, 0.6) 100%)',
                zIndex: 9996,
              }}
            />

            {/* CRT glow/bloom */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: `radial-gradient(ellipse at center, ${alpha(appTheme.palette.primary.main, 0.02)} 0%, transparent 70%)`,
                zIndex: 9997,
                filter: 'blur(30px)',
              }}
            />

            {/* Vintage tint overlay */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: 'linear-gradient(rgba(20, 40, 30, 0.15), rgba(20, 40, 30, 0.15))',
                zIndex: 9998,
                mixBlendMode: 'multiply',
              }}
            />
          </>
        );

      case 'NVG':
        return (
          <>
            {/* Green monochromatic base filter — reduced opacity to preserve
                luminance differentiation so bright entities (fires, alerts) appear
                as distinctly brighter green vs the darker map background */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: 'linear-gradient(rgba(0, 60, 0, 0.45), rgba(0, 60, 0, 0.45))',
                mixBlendMode: 'color',
                zIndex: 9990,
              }}
            />

            {/* Brightness boost layer — amplifies bright areas (entity icons)
                so they glow more intensely through the green tint */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: 'linear-gradient(rgba(0, 255, 0, 0.12), rgba(0, 255, 0, 0.12))',
                mixBlendMode: 'screen',
                zIndex: 9991,
              }}
            />

            {/* High contrast overlay — pushes luminance separation so
                entity markers stand apart from terrain */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: 'linear-gradient(rgba(0, 200, 0, 0.08), rgba(0, 200, 0, 0.08))',
                mixBlendMode: 'hard-light',
                zIndex: 9992,
              }}
            />

            {/* Fine noise grain — reduced on mobile for better video compression */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                opacity: isMobile ? 0.15 : 0.25,
                zIndex: 9993,
                background: `url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)' opacity='0.5'/%3E%3C/svg%3E")`,
                animation: `${nvgFlicker} 0.15s ease-in-out infinite`,
              }}
            />

            {/* NVG vignette - darker edges */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'radial-gradient(ellipse at center, transparent 0%, transparent 40%, rgba(0, 30, 0, 0.85) 100%)',
                zIndex: 9994,
              }}
            />

            {/* Circular lens effect */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                boxShadow: 'inset 0 0 200px 80px rgba(0, 0, 0, 0.4)',
                borderRadius: '50%',
                zIndex: 9995,
              }}
            />

            {/* Subtle green glow/bloom */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'radial-gradient(ellipse at center, rgba(0, 255, 0, 0.03) 0%, transparent 50%)',
                zIndex: 9996,
                filter: 'blur(20px)',
              }}
            />

            {/* Scanlines (subtle for NVG) */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: `
                  repeating-linear-gradient(
                    0deg,
                    rgba(0, 50, 0, 0.1) 0px,
                    transparent 1px,
                    transparent 3px
                  )
                `,
                zIndex: 9997,
              }}
            />
          </>
        );

      case 'FLIR':
        return (
          <>
            {/* High-contrast boost — push whites whiter and darks darker */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: 'linear-gradient(rgba(255, 255, 255, 0.08), rgba(0, 0, 0, 0.08))',
                mixBlendMode: 'overlay',
                zIndex: 9990,
              }}
            />

            {/* Iron-bow warm tint — simulates real FLIR color mapping where
                bright/hot areas shift warm (yellow/amber) while cool areas
                stay dark. Gives entity icons a warm glow that distinguishes
                them from the cooler map background */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'linear-gradient(180deg, rgba(255, 200, 50, 0.06) 0%, rgba(180, 80, 20, 0.04) 50%, rgba(40, 10, 60, 0.03) 100%)',
                mixBlendMode: 'screen',
                zIndex: 9991,
              }}
            />

            {/* Luminance-based warm highlight — bright areas (entity icons)
                get an amber cast mimicking "white-hot" FLIR heat signatures */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'radial-gradient(ellipse at center, rgba(255, 220, 150, 0.05) 0%, transparent 70%)',
                mixBlendMode: 'screen',
                zIndex: 9991,
              }}
            />

            {/* Sensor noise — fine-grain thermal sensor static */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                opacity: isMobile ? 0.08 : 0.14,
                zIndex: 9992,
                background: `url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E")`,
                mixBlendMode: 'overlay',
              }}
            />

            {/* Horizontal scan artifact — faint banding like a real FLIR sensor */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: `repeating-linear-gradient(
                  0deg,
                  rgba(255, 255, 255, 0.008) 0px,
                  rgba(255, 255, 255, 0.008) 1px,
                  transparent 1px,
                  transparent 4px
                )`,
                zIndex: 9993,
              }}
            />

            {/* FLIR vignette — heavy, like looking through an optics housing */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background:
                  'radial-gradient(ellipse at center, transparent 0%, transparent 45%, rgba(0, 0, 0, 0.65) 100%)',
                zIndex: 9994,
              }}
            />

            {/* Inner optics ring — subtle circular cutoff like a camera lens */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                boxShadow: 'inset 0 0 120px 40px rgba(0, 0, 0, 0.25)',
                zIndex: 9995,
              }}
            />
          </>
        );

      case 'NORMAL':
      default:
        return (
          <>
            {/* Subtle vignette for depth — reduced on mobile */}
            <Box
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'none',
                background: isMobile
                  ? 'radial-gradient(ellipse at center, transparent 0%, transparent 65%, rgba(0, 0, 0, 0.2) 100%)'
                  : 'radial-gradient(ellipse at center, transparent 0%, transparent 60%, rgba(0, 0, 0, 0.3) 100%)',
                zIndex: 9999,
              }}
            />
          </>
        );
    }
  }, [preset, isMobile]);

  return (
    <Box
      {...props}
      data-testid={`post-process-overlay-${preset.toLowerCase()}`}
      sx={{
        position: 'absolute',
        inset: 0,
        pointerEvents: 'none',
        zIndex: 2,
        // Container-level CSS filter (e.g. grayscale for FLIR) — applied to the
        // wrapper so it affects all children (Cesium tiles + entities).
        filter: containerFilter,
        ...props.sx,
      }}
    >
      {/* Wrapped content (globe viewer) — inherits the container filter */}
      {children && (
        <Box sx={{ position: 'absolute', inset: 0, pointerEvents: 'auto' }}>{children}</Box>
      )}

      {/* Overlay effects rendered on top */}
      {overlays}
    </Box>
  );
});

PostProcessOverlay.displayName = 'PostProcessOverlay';

export default PostProcessOverlay;
