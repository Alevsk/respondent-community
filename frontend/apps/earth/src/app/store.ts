import { create } from 'zustand';
import type {
  TimeSlice,
  LayerSlice,
  EntitySlice,
  NotificationSlice,
  IndicatorSlice,
  WatchlistSlice,
  GlobeDisplaySlice,
  ModeSlice,
} from '@respondent/core';
import {
  createTimeSlice,
  createLayerSlice,
  createEntitySlice,
  createNotificationSlice,
  createIndicatorSlice,
  createWatchlistSlice,
  createGlobeDisplaySlice,
  createModeSlice,
} from '@respondent/core';
import { createImmersiveUISlice, type ImmersiveUISlice } from '../stores/createImmersiveUISlice';
import { createClusterSlice, type ClusterSlice } from '../stores/createClusterSlice';

// Re-export types so `import { Entity } from '@/app/store'` works in earth files
export type {
  Entity,
  Observation,
  TrailPoint,
  EntityViewState,
  LayerData,
  DataSource,
  Layer,
  LayerConfig,
  LayerDisplayConfig,
  HistoryConfig,
  FieldFormat,
  FieldRendererConfig,
  IndicatorValue,
  IndicatorSnapshot,
  WatchlistEntity,
  TimeMode,
  TimePreset,
  TimeRange,
  FilterPreset,
  DetectMode,
  ViewMode,
  MobileDrawerType,
  AspectRatioKey,
  InsightEntityRef,
  AIInsightNotification,
  NotificationFilterState,
  EntityRef,
} from '@respondent/core';
export type { AppMode } from '@respondent/core';
export {
  MAX_PINNED_ENTITIES,
  DEFAULT_ENTITY_VIEW_STATE,
  entityKey,
  obsKey,
  FILTER_PRESETS,
  TIME_PRESETS,
} from '@respondent/core';

export type EarthState = TimeSlice &
  LayerSlice &
  EntitySlice &
  NotificationSlice &
  IndicatorSlice &
  WatchlistSlice &
  GlobeDisplaySlice &
  ModeSlice &
  ImmersiveUISlice &
  ClusterSlice;

/* eslint-disable @typescript-eslint/no-explicit-any */
export const useUIStore = create<EarthState>()((set, get, api) => ({
  ...createTimeSlice(set as any, get as any, api as any),
  ...createLayerSlice(set as any, get as any, api as any),
  ...createEntitySlice<EarthState>(set as any, get as any, api as any),
  ...createNotificationSlice(set as any, get as any, api as any),
  ...createIndicatorSlice(set as any, get as any, api as any),
  ...createWatchlistSlice(set as any, get as any, api as any),
  ...createGlobeDisplaySlice(set as any, get as any, api as any),
  ...createModeSlice(set as any, get as any, api as any),
  ...createImmersiveUISlice(set as any, get as any, api as any),
  ...createClusterSlice(set as any, get as any, api as any),
}));
/* eslint-enable @typescript-eslint/no-explicit-any */

export function isIndicatorLayer(layerId: string): boolean {
  return useUIStore.getState().indicatorLayerIds.has(layerId);
}
