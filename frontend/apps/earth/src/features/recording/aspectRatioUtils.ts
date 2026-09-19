import type { AspectRatioKey } from '@respondent/core';

const RATIO_VALUES: Record<string, number> = {
  '9:16': 9 / 16,
  '4:5': 4 / 5,
  '1:1': 1,
  '16:9': 16 / 9,
};

export function calculateFrame(
  ratio: AspectRatioKey,
  viewportWidth: number,
  viewportHeight: number,
): { frameWidth: number; frameHeight: number; offsetX: number; offsetY: number } {
  if (ratio === 'free') {
    return { frameWidth: viewportWidth, frameHeight: viewportHeight, offsetX: 0, offsetY: 0 };
  }

  const targetRatio = RATIO_VALUES[ratio] ?? 1;
  let frameWidth: number;
  let frameHeight: number;

  if (viewportWidth / viewportHeight > targetRatio) {
    frameHeight = viewportHeight;
    frameWidth = frameHeight * targetRatio;
  } else {
    frameWidth = viewportWidth;
    frameHeight = frameWidth / targetRatio;
  }

  const offsetX = (viewportWidth - frameWidth) / 2;
  const offsetY = (viewportHeight - frameHeight) / 2;

  return { frameWidth, frameHeight, offsetX, offsetY };
}
