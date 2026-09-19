// --- Indicator types ---

export interface IndicatorValue {
  key: string;
  value: string;
  label: string;
  unit: string;
  level: number; // severity 0-5
  changePct?: number;
  changeAbs?: number;
  format?: string; // "scale" (default), "number", "currency", "percent"
  precision?: number;
  prefix?: string;
}

export interface IndicatorSnapshot {
  layerId: string;
  layerName: string;
  timestampMs: number;
  values: IndicatorValue[];
  summary: string;
  overallLevel: number;
}
