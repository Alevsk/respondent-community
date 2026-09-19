import { test, expect } from '@playwright/test';

test.describe('Earth App — app load', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('earth shell is visible', async ({ page }) => {
    await expect(page.getByTestId('earth-shell')).toBeVisible();
  });

  test('Cesium globe canvas renders with nonzero dimensions', async ({ page }) => {
    const globeContainer = page.getByTestId('globe-container');
    await expect(globeContainer).toBeVisible();
    const canvas = globeContainer.locator('canvas').first();
    await expect(canvas).toBeVisible();
    const box = await canvas.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.width).toBeGreaterThan(0);
    expect(box!.height).toBeGreaterThan(0);
  });

  test('notification bell is visible', async ({ page }) => {
    await expect(page.getByTestId('notification-bell')).toBeVisible();
  });
});
