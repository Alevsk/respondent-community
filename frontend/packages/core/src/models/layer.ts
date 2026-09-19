// FieldFormat describes formatting rules for a rendered metadata field value.
// Matches the proto FieldFormat message in layers.proto.
export interface FieldFormat {
  type: 'float' | 'integer' | 'string' | 'raw';
  precision?: number;
  prefix?: string;
  suffix?: string;
  transform?: 'upper' | 'lower';
}

// FieldRendererConfig defines how a metadata field is displayed in the entity panel.
// Populated from declarative source YAML via the GetLayers API response.
export interface FieldRendererConfig {
  keys: string[];
  label: string;
  format: FieldFormat;
  priority: number;
}

// LayerDisplayConfig holds rendering metadata for a layer.
// Populated from declarative source YAML and served via the GetLayers API.
export interface LayerDisplayConfig {
  icon: {
    shape: string;
    rotatable: boolean;
    interpolation: boolean;
    scale: number;
  };
  trail: { color: string; width: number; opacity: number };
  style: { color: string; pointSize: number };
  fieldRenderers: FieldRendererConfig[];
  // Optional: color entities by the value of a metadata field (declarative,
  // replaces per-layer hardcoded color logic in the client).
  colorBy?: { field: string; values: Record<string, string>; defaultColor: string };
}

// Per-layer time range limits for historical exploration.
// When present, the custom range picker uses these instead of server defaults.
export interface HistoryConfig {
  maxLookbackHours: number; // 0 = server default (48h)
  maxRangeSpanHours: number; // 0 = server default (24h)
}

// Layer configuration for layer-specific settings
export interface LayerConfig {
  showOrbits?: boolean; // For satellites
  showTrails?: boolean; // For flights
  color?: string;
}

// Layer types from API
export interface Layer {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  mode: string;
  density: number;
  source: string;
  lastUpdate: number;
  count: number;
  color: string;
  pointSize: number;
  displayConfig?: LayerDisplayConfig;
  historyConfig?: HistoryConfig;
  renderingMode?: string; // "map" (default) or "indicator"
  filteringMode?: string; // "viewport" or "" — drives spatial viewport sync
}
