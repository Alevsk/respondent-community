/**
 * Field Renderer Registry — per-entity-type metadata formatting.
 *
 * Each renderer matches specific metadata keys and provides a display label
 * and formatting function. Renderers can be scoped to specific layer types.
 * Unknown keys fall back to a generic key-value display.
 *
 * Dynamic renderers registered via `registerDynamicRenderers()` (populated
 * from declarative source YAML via the GetLayers API) take precedence over
 * the hardcoded `FIELD_RENDERERS` for their respective layer types.
 */

import type { FieldRendererConfig, FieldFormat } from '@/app/store';

export type { FieldRendererConfig, FieldFormat };

export interface FieldRenderer {
  /** Which metadata keys this renderer handles */
  keys: string[];
  /** Display label */
  label: string;
  /** Format the raw value for display */
  format: (value: string) => string;
  /** Only show for certain layer types. Omit = show for all. */
  layerTypes?: string[];
  /** Display priority (lower = higher in the panel) */
  priority: number;
}

const FIELD_RENDERERS: FieldRenderer[] = [
  // Flight fields
  {
    keys: ['callsign'],
    label: 'CALLSIGN',
    format: (v) => v.toUpperCase(),
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 0,
  },
  {
    keys: ['registration'],
    label: 'REGISTRATION',
    format: (v) => v.toUpperCase(),
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 1,
  },
  {
    keys: ['aircraft_type'],
    label: 'AIRCRAFT',
    format: (v) => v,
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 2,
  },
  {
    keys: ['origin'],
    label: 'ORIGIN',
    format: (v) => v.toUpperCase(),
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 3,
  },
  {
    keys: ['destination'],
    label: 'DESTINATION',
    format: (v) => v.toUpperCase(),
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 4,
  },
  {
    keys: ['squawk'],
    label: 'SQUAWK',
    format: (v) => v,
    layerTypes: ['flights_commercial', 'flights_military'],
    priority: 5,
  },
  // Earthquake fields
  {
    keys: ['magnitude', 'mag'],
    label: 'MAGNITUDE',
    format: (v) => `M${parseFloat(v).toFixed(1)}`,
    layerTypes: ['earthquakes'],
    priority: 0,
  },
  {
    keys: ['depth', 'depth_km'],
    label: 'DEPTH',
    format: (v) => `${parseFloat(v).toFixed(1)} km`,
    layerTypes: ['earthquakes'],
    priority: 1,
  },
  // Satellite fields
  {
    keys: ['norad_id'],
    label: 'NORAD ID',
    format: (v) => v,
    layerTypes: ['satellites'],
    priority: 0,
  },
  {
    keys: ['inclination'],
    label: 'INCLINATION',
    format: (v) => `${parseFloat(v).toFixed(1)}°`,
    layerTypes: ['satellites'],
    priority: 1,
  },
];

export interface ResolvedField {
  label: string;
  value: string;
  priority: number;
}

// ------------------------------------------------------------------
// Dynamic renderer registry
// Populated by registerDynamicRenderers() when the GetLayers API response
// contains FieldRendererConfig entries from declarative source YAML.
// ------------------------------------------------------------------
const dynamicRendererRegistry = new Map<string, FieldRendererConfig[]>();

/**
 * Register field renderer configs for a layer type from a declarative source.
 * Replaces any previously registered configs for this layer type.
 */
export function registerDynamicRenderers(
  layerType: string,
  renderers: FieldRendererConfig[],
): void {
  dynamicRendererRegistry.set(layerType, renderers);
}

/**
 * Apply a structured FieldFormat spec to a raw string value.
 * Used by the dynamic renderer path to format values from declarative sources.
 */
export function applyFormat(value: string, format: FieldFormat): string {
  if (value === '') return '';
  switch (format.type) {
    case 'float': {
      const num = parseFloat(value);
      const formatted = isNaN(num) ? value : num.toFixed(format.precision ?? 1);
      return `${format.prefix ?? ''}${formatted}${format.suffix ?? ''}`;
    }
    case 'integer': {
      const num = parseInt(value, 10);
      const formatted = isNaN(num) ? value : String(num);
      return `${format.prefix ?? ''}${formatted}${format.suffix ?? ''}`;
    }
    case 'string': {
      let result = value;
      if (format.transform === 'upper') result = result.toUpperCase();
      else if (format.transform === 'lower') result = result.toLowerCase();
      return `${format.prefix ?? ''}${result}${format.suffix ?? ''}`;
    }
    case 'raw':
    default:
      return value;
  }
}

/**
 * Resolves metadata fields using the renderer registry.
 * Dynamic renderers for the given layer type take precedence over hardcoded ones.
 * Returns matched fields sorted by priority, plus unmatched fields as generic entries.
 */
export function resolveFields(
  metadata: Record<string, string>,
  layerType: string,
): ResolvedField[] {
  const matched = new Set<string>();
  const fields: ResolvedField[] = [];

  // Check dynamic registry first — declarative source renderers take precedence
  const dynamicConfigs = dynamicRendererRegistry.get(layerType);
  if (dynamicConfigs && dynamicConfigs.length > 0) {
    for (const config of dynamicConfigs) {
      for (const key of config.keys) {
        if (key in metadata) {
          fields.push({
            label: config.label,
            value: applyFormat(metadata[key], config.format),
            priority: config.priority,
          });
          matched.add(key);
          break; // Only use first matching key per renderer
        }
      }
    }
  } else {
    // Fall back to hardcoded renderers for imperative adapter layers
    for (const renderer of FIELD_RENDERERS) {
      // Skip if scoped to different layer types
      if (renderer.layerTypes && !renderer.layerTypes.includes(layerType)) continue;

      for (const key of renderer.keys) {
        if (key in metadata) {
          fields.push({
            label: renderer.label,
            value: renderer.format(metadata[key]),
            priority: renderer.priority,
          });
          matched.add(key);
          break; // Only use first matching key per renderer
        }
      }
    }
  }

  // Add unmatched metadata as generic fields (skip empty values)
  for (const [key, value] of Object.entries(metadata)) {
    if (!matched.has(key) && value !== '') {
      fields.push({
        label: key.toUpperCase().replace(/_/g, ' '),
        value,
        priority: 100,
      });
    }
  }

  // Filter out fields with empty values from rendered output
  return fields.filter((f) => f.value !== '').sort((a, b) => a.priority - b.priority);
}

/** Exposed for testing only — clears all dynamic renderer registrations. */
export function _resetDynamicRenderers(): void {
  dynamicRendererRegistry.clear();
}
