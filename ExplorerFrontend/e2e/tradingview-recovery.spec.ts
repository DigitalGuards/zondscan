import { expect, test, type Route } from '@playwright/test';

const scriptUrl = 'https://s3.tradingview.com/tv.js';
const recoveredWidget = `
window.TradingView = {
  widget: class {
    constructor(config) {
      document.getElementById(config.container_id).textContent = 'Recovered market chart';
    }
  }
};`;

for (const failAfterUnmount of [false, true]) {
  const timing = failAfterUnmount ? 'after unmount' : 'while mounted';
  test(`TradingView recovers from a script failure ${timing}`, async ({ page }) => {
    let requests = 0;
    let pendingRequest: Route | undefined;
    const pageErrors: string[] = [];
    page.on('pageerror', (error) => pageErrors.push(error.message));
    await page.route(scriptUrl, async (route) => {
      requests += 1;
      if (requests === 1) {
        if (failAfterUnmount) pendingRequest = route;
        else await route.abort('failed');
        return;
      }
      await route.fulfill({
        contentType: 'application/javascript',
        body: recoveredWidget,
      });
    });

    await page.goto('/', { waitUntil: 'domcontentloaded' });
    await expect.poll(() => requests).toBe(1);
    await expect(page.locator('#tradingview_qrl')).toBeEmpty();
    await page.getByRole('link', { name: 'Site settings', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Site settings', exact: true })).toBeVisible();
    await expect(page.locator('#tradingview_qrl')).toHaveCount(0);
    if (failAfterUnmount) await pendingRequest!.abort('failed');
    await expect(page.locator('#tradingview-script')).toHaveCount(0);

    // The return uses client navigation, preserving the shared script state.
    await page.getByRole('link', { name: 'ZondScan home', exact: true }).click();
    await expect(page.locator('#tradingview_qrl')).toHaveText('Recovered market chart');
    expect(requests).toBe(2);
    expect(pageErrors).toEqual([]);
  });
}
