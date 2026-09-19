/**
 * layerIcons - Canvas-based icon rendering for layer types.
 *
 * Uses the same icon registry that renders entities on the globe,
 * so icons are fully driven by declarative source YAML definitions.
 */

import React from 'react';
import { getIconCanvas } from '../globe/icons/iconRegistry';
import { theme } from '@respondent/core';

/**
 * Renders a canvas-based layer icon as an <img> element.
 * The icon shape is resolved from the dynamic icon registry (populated
 * from source YAML display configs via the GetLayers API).
 */
export function CanvasLayerIcon({
  layerType,
  color,
  size = 20,
}: {
  layerType: string;
  color?: string;
  size?: number;
}): React.ReactElement {
  const src = React.useMemo(() => {
    return getIconCanvas(layerType, color || theme.palette.primary.main).toDataURL();
  }, [layerType, color]);
  return <img src={src} alt="" width={size} height={size} style={{ display: 'block' }} />;
}
