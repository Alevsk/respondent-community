export type FilterPreset = 'NORMAL' | 'CRT' | 'NVG' | 'FLIR';

export type DetectMode = 'panoptic' | 'sparse';

export type ViewMode = 'globe' | 'entity';

export type MobileDrawerType = 'layers' | 'settings' | 'nav' | 'effects' | null;

// AspectRatioKey as a simple union type matching keys from ASPECT_RATIOS constant.
// The ASPECT_RATIOS constant itself stays in shared/constants/aspectRatios.ts.
export type AspectRatioKey = '9:16' | '4:5' | '1:1' | '16:9' | 'free';

export const FILTER_PRESETS: FilterPreset[] = ['NORMAL', 'CRT', 'NVG', 'FLIR'];

export type AppMode = 'dashboard' | 'immersive';
