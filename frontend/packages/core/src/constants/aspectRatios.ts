export const ASPECT_RATIOS = {
  '9:16': {
    width: 9,
    height: 16,
    label: 'Stories/Reels',
    platforms: ['Instagram', 'TikTok', 'YouTube Shorts'],
  },
  '4:5': { width: 4, height: 5, label: 'Portrait Post', platforms: ['Instagram', 'Facebook'] },
  '1:1': { width: 1, height: 1, label: 'Square', platforms: ['Instagram', 'Twitter/X'] },
  '16:9': { width: 16, height: 9, label: 'Landscape', platforms: ['YouTube', 'Twitter/X'] },
  free: { width: 0, height: 0, label: 'Free', platforms: [] as string[] },
} as const;
