import { expect, test } from '@playwright/test';

// Run separately with a configured available v3 destination. The intercepted
// destination proves navigation isolation without pretending this v2 build
// supports the future v3 protocol.
test('available network selection starts a fresh document at the target origin root', async ({
  page,
}) => {
  test.skip(process.env.NEXT_PUBLIC_V3_EXPLORER_AVAILABLE !== 'true');
  const target = new URL(process.env.NEXT_PUBLIC_V3_EXPLORER_URL!).origin + '/';
  const navigationRequests: string[] = [];
  await page.route(`${target}**`, (route) => {
    if (route.request().isNavigationRequest()) navigationRequests.push(route.request().url());
    return route.fulfill({
      contentType: 'text/html',
      body: '<h1>Independent explorer destination</h1>',
    });
  });
  await page.goto('/settings?from=tx&hash=0x123#language');
  await expect(page.getByLabel('Language', { exact: true })).toBeEnabled();
  await page.evaluate(() => {
    (window as Window & { networkNavigationProbe?: string }).networkNavigationProbe =
      'old-document';
    localStorage.setItem('zondscan.preferences.v1', JSON.stringify({ theme: 'dim' }));
  });
  await page.getByRole('button', { name: 'Network: QRL Testnet v2' }).click();
  const destination = page.getByRole('menuitem', { name: 'QRL Testnet v3', exact: true });
  await expect(destination).toHaveAttribute('href', target);
  await page.screenshot({ path: 'test-results/network-selector-available.png' });
  await destination.click();
  await expect(page).toHaveURL(target);
  await expect(
    page.getByRole('heading', { name: 'Independent explorer destination' })
  ).toBeVisible();
  expect(navigationRequests).toEqual([target]);
  expect(
    await page.evaluate(
      () => (window as Window & { networkNavigationProbe?: string }).networkNavigationProbe
    )
  ).toBeUndefined();
  expect(await page.evaluate(() => localStorage.getItem('zondscan.preferences.v1'))).toBeNull();
});
