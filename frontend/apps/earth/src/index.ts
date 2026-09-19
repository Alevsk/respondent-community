// Earth public API — consumed by apps/command via @respondent/earth alias

// Store slice (for command to compose into its own store)
export { createImmersiveUISlice } from './stores/createImmersiveUISlice';
export type { ImmersiveUISlice } from './stores/createImmersiveUISlice';

// Top-level shell component (for command to embed in its router)
export { default as EarthShell } from './app/EarthShell';

// Feature components
export { default as GlobeScene } from './features/globe/GlobeScene';
export { default as DataLayersPanel } from './features/layers/DataLayersPanel';
export { default as EffectsPanel } from './features/effects/EffectsPanel';
export { default as SettingsPanel } from './features/settings/SettingsPanel';
export { default as NavigationPanel } from './features/navigation/NavigationPanel';
export { default as EntityDetailPanel } from './features/entity/EntityDetailPanel';
export { default as MobileEntityPanel } from './features/entity/MobileEntityPanel';
export { default as EntitySearchBar } from './features/search/EntitySearchBar';
export { default as WatchlistBar } from './features/search/WatchlistBar';
export { default as IndicatorHUD } from './features/indicators/IndicatorHUD';
export { default as RecordingMode } from './features/recording/RecordingMode';
export { default as CCTVPanel } from './features/cctv/CCTVPanel';
export { default as SavedLocationsModal } from './features/scenes/SavedLocationsModal';

// Search hooks
export { useSearchEntities } from './features/search/useSearchEntities';
export { useWatchlistPersistence } from './features/search/useWatchlistPersistence';

// Shared HUD components
export { default as BottomToolbar } from './shared/hud/BottomToolbar';
export { default as CustomRangePicker } from './shared/hud/CustomRangePicker';
export { default as Header } from './shared/hud/Header';
export { default as MobileHudLayout } from './shared/hud/MobileHudLayout';
export { default as MobileBottomNav } from './shared/hud/MobileBottomNav';
export { default as TelemetryStrip } from './shared/hud/TelemetryStrip';
export { default as RecBlock } from './shared/hud/RecBlock';
export { default as StatusReadout } from './shared/hud/StatusReadout';
export { default as PostProcessOverlay } from './shared/hud/PostProcessOverlay';

// Shared UI components
export { default as MobileDrawer } from './shared/ui/MobileDrawer';
export { default as ErrorBoundary } from './shared/ui/ErrorBoundary';
export { default as ConfigPanel } from './shared/ui/ConfigPanel';

// Shared notifications
export { default as NotificationBell } from './shared/notifications/NotificationBell';
export { default as NotificationPanel } from './shared/notifications/NotificationPanel';
export { useNotificationBackfill } from './shared/notifications/useNotificationBackfill';

// Shared bootstrap
export { default as BootstrapGate } from './shared/bootstrap/BootstrapGate';

// Shared hooks
export { usePanelPosition } from './shared/layout/usePanelPosition';
export { usePanelLayoutStore } from './shared/layout/panelLayoutStore';

// Globe store (viewer camera state)
export { useViewerStore } from './features/globe/store';

// API hooks
export { useLayers, useToggleLayer } from './shared/api/queries';
export { useLayerStream, useViewportSync } from './shared/api/useLayerStream';

// Field renderers
export { applyFormat } from './features/entity/tabs/overview/fieldRenderers';

// Entity detail tabs (for command app reuse)
export { getTabsForLayerType, registerTab } from './features/entity/tabs/tabRegistry';
export type { EntityTabProps, EntityTab } from './features/entity/tabs/tabRegistry';
export { useEntityDetailWithFallback } from './features/entity/useEntityDetailWithFallback';
export { useEntityDetail } from './shared/api/queries';
export type { EntityDetailResponse } from './shared/api/queries';
// Side-effect: register all built-in tabs
import './features/entity/tabs/OverviewTab';
import './features/entity/tabs/AIAnalysisTab';
import './features/entity/tabs/HistoryTab';
import './features/entity/tabs/MetadataTab';
