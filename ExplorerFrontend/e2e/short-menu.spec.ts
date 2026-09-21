import { expect, test } from '@playwright/test';

for (const height of [360, 280]) {
  test(`desktop menu keeps keyboard focus visible at 1280x${height}`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height });
    await page.goto('/settings');

    const trigger = page.getByRole('button', { name: 'Tools', exact: true });
    const menu = page.getByRole('menu', { name: 'Tools', exact: true });
    await trigger.click();
    await expect(menu).toBeVisible();
    await trigger.press('ArrowDown');

    const items = menu.getByRole('menuitem');
    for (const key of ['End', 'Home']) {
      await page.keyboard.press(key);
      const item = key === 'End' ? items.last() : items.first();
      await expect(menu).toBeVisible();
      await expect(item).toBeFocused();

      const panelBounds = await menu.boundingBox();
      const itemBounds = await item.boundingBox();
      expect(panelBounds).not.toBeNull();
      expect(itemBounds).not.toBeNull();
      expect(panelBounds!.y + panelBounds!.height).toBeLessThanOrEqual(height - 12);
      expect(itemBounds!.y).toBeGreaterThanOrEqual(panelBounds!.y);
      expect(itemBounds!.y + itemBounds!.height).toBeLessThanOrEqual(
        panelBounds!.y + panelBounds!.height
      );
      expect(await page.evaluate(() => window.scrollY)).toBe(0);
    }
  });
}
