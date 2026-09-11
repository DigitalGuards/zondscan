import { expect, test } from '@playwright/test';

test('phone menu keeps the page usable and exposes appearance, networks and settings', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  const toggle = page.locator('[data-mobile-navigation-toggle]');
  await toggle.click();
  const navigation = page.getByRole('navigation', { name: 'Mobile navigation' });
  await expect(navigation).toBeVisible();
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByRole('link', { name: 'ZondScan home' })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(await page.locator('body').evaluate((el) => getComputedStyle(el).overflow)).not.toBe(
    'hidden'
  );

  await navigation.getByRole('button', { name: 'Appearance', exact: true }).click();
  await navigation.getByRole('radio', { name: 'Light', exact: true }).check();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await expect(navigation).toBeVisible();
  await navigation.getByRole('radio', { name: 'Dark', exact: true }).check();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await navigation.getByRole('button', { name: 'Explorer network', exact: true }).click();
  await expect(navigation.getByText('QRL Testnet v2', { exact: true })).toBeVisible();
  await expect(navigation.getByRole('button', { name: /QRL Testnet v3/ })).toBeDisabled();
  await navigation.getByRole('link', { name: 'Site settings', exact: true }).click();
  await expect(page).toHaveURL('/settings');
  await expect(navigation).not.toBeVisible();
  await toggle.click();
  await navigation.getByRole('link', { name: 'Site settings', exact: true }).focus();
  await page.keyboard.press('Enter');
  await expect(navigation).not.toBeVisible();
  await expect(toggle).toBeFocused();
  await toggle.click();
  await toggle.press('Escape');
  await expect(toggle).toBeFocused();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
});

test('phone menu stays within the viewport with every language', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 });
  await page.goto('/settings');
  for (const locale of ['en', 'zh', 'es', 'ru']) {
    await page.locator('#interface-language').selectOption(locale);
    await page.locator('[data-mobile-navigation-toggle]').click();
    const navigation = page.locator('[data-site-navigation] nav').last();
    await expect(navigation).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true
    );
    expect(
      await navigation.evaluate((el) => el.getBoundingClientRect().left)
    ).toBeGreaterThanOrEqual(0);
    await page.locator('[data-mobile-navigation-toggle]').click();
  }
});

test('hover preserves the open utility menu and keyboard-owned navigation', async ({ page }) => {
  await page.goto('/settings');
  await page.getByRole('button', { name: /^Appearance:/ }).click();
  const blockchain = page.getByRole('button', { name: 'Blockchain', exact: true });
  await blockchain.hover();
  await expect(page.getByRole('menu')).toHaveCount(1);
  await expect(page.getByRole('menu', { name: /^Appearance:/ })).toBeVisible();
  await page.keyboard.press('Escape');
  await blockchain.focus();
  await page.keyboard.press('ArrowDown');
  const transactions = page.getByRole('menuitem', { name: /^Transactions Latest/ });
  await expect(transactions).toBeFocused();
  const resources = page.getByRole('button', { name: 'Resources', exact: true });
  await resources.hover();
  await expect(transactions).toBeFocused();
  await expect(page.getByRole('menu', { name: 'Blockchain', exact: true })).toBeVisible();
  await resources.click();
  await expect(page.getByRole('menu', { name: 'Resources', exact: true })).toBeVisible();
  await expect(page.getByRole('menu')).toHaveCount(1);
});
