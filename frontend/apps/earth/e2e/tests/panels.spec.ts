import { test, expect } from '@playwright/test';

test.describe('Earth App — panels', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await expect(page.getByTestId('bottom-toolbar')).toBeVisible();
  });

  test('settings panel opens and closes', async ({ page }) => {
    await page.getByTestId('toolbar-btn-settings').click();
    const panel = page.getByTestId('panel-settings');
    await expect(panel).toBeVisible();
    await panel.getByTestId('config-panel-close').click();
    await expect(panel).not.toBeVisible();
  });

  test('navigation panel opens and closes', async ({ page }) => {
    await page.getByTestId('toolbar-btn-nav').click();
    const panel = page.getByTestId('panel-navigation');
    await expect(panel).toBeVisible();
    await panel.getByTestId('config-panel-close').click();
    await expect(panel).not.toBeVisible();
  });

  test('notification panel opens from bell click', async ({ page }) => {
    await page.getByTestId('notification-bell').click();
    await expect(page.getByTestId('notification-panel')).toBeVisible();
  });

  test('search opens from toolbar button and closes with Escape', async ({ page }) => {
    await page.getByTestId('toolbar-btn-search').click();
    const searchBar = page.getByTestId('entity-search-bar');
    await expect(searchBar).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(searchBar).not.toBeVisible();
  });
});
