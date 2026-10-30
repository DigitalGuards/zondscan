import { expect, test } from '@playwright/test';

const address = `Q${'4'.repeat(128)}`;

for (const width of [390, 1440]) {
  for (const appearance of ['dark', 'light'] as const) {
    test(`verified primary name and address stay visible at ${width}px in ${appearance}`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto('/settings');
      await page.getByRole('radio', { name: appearance === 'dark' ? 'Dark' : 'Light', exact: true }).focus();
      await page.keyboard.press('Space');
      await page.goto(`/address/${address}`);

      await expect(page).toHaveTitle('alice.qrl QNS Address | ZondScan');
      await expect(page.locator('html')).toHaveAttribute('data-theme', appearance);
      const identity = page.locator('[data-qns-resolution="alice.qrl"]');
      await expect(identity).toBeVisible();
      await expect(identity.getByText('alice.qrl', { exact: true })).toBeVisible();
      await expect(identity).toHaveAttribute('data-qns-address', address);
      await expect(identity.locator('[data-qns-address-link]')).toBeVisible();
      await expect(identity.locator('[data-qns-address-link]')).toHaveAttribute('href', `/address/${address}`);
      await expect(identity.locator('[data-copy-address]')).toHaveAttribute('data-copy-address', address);
      await expect(identity.locator('[data-qr-address]')).toHaveAttribute('data-qr-address', address);
      const canonical = await page.locator('link[rel="canonical"]').getAttribute('href');
      expect(canonical && new URL(canonical).pathname).toBe(`/address/${address}`);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    });
  }
}

for (const [digit, outcome] of [['5', 'missing name'], ['6', 'lookup failure']]) {
  test(`direct address retains its identity after ${outcome}`, async ({ page }) => {
    const unnamedAddress = `Q${digit.repeat(128)}`;
    await page.goto(`/address/${unnamedAddress}`);

    await expect(page).toHaveTitle(/^Address Q/);
    await expect(page.locator('[data-qns-resolution]')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Copy address', exact: true })).toBeVisible();
    await expect(page.locator('#address-page-heading')).toContainText(unnamedAddress);
    await expect(page.locator('#address-error-heading')).toHaveCount(0);
  });
}

test('a name URL continues to display its name and resolved address', async ({ page }) => {
  await page.goto('/address/alice.qrl');

  await expect(page).toHaveTitle('alice.qrl QNS Address | ZondScan');
  await expect(page.locator('[data-qns-resolution="alice.qrl"]')).toHaveAttribute('data-qns-address', address);
  const canonical = await page.locator('link[rel="canonical"]').getAttribute('href');
  expect(canonical && new URL(canonical).pathname).toBe('/address/alice.qrl');
});
