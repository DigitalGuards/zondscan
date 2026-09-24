import { expect, test, type Page } from '@playwright/test';

const key = 'zondscan.preferences.v1';
const hash = `0x${'0'.repeat(63)}1`;
const address = `Q${'a'.repeat(40)}`;
const errors = new WeakMap<Page, string[]>();

async function appearance(page: Page, label: string) {
  await page.getByRole('button', { name: /^Appearance:/ }).click();
  await page.getByRole('menuitem', { name: new RegExp(`^${label}`) }).click();
}
async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test.beforeEach(async ({ page }) => {
  const problems: string[] = [];
  errors.set(page, problems);
  page.on('pageerror', (error) => problems.push(error.message));
  page.on('console', (message) => {
    if (message.type() === 'error' && /hydrat|Minified React|maximum update/i.test(message.text()))
      problems.push(message.text());
  });
  // UI tests stay independent of the external market-chart service.
  await page.route('https://s3.tradingview.com/tv.js', (route) =>
    route.fulfill({
      contentType: 'application/javascript',
      body: `window.TradingView={widget:class{constructor(c){const e=document.getElementById(c.container_id);e.dataset.widgetTheme=c.theme;e.textContent='Market chart preview';}}};`,
    })
  );
});
test.afterEach(async ({ page }) => {
  expect(errors.get(page)).toEqual([]);
});

test('sticky header moves search into the bar when the page search scrolls away', async ({
  page,
}) => {
  await page.goto('/');
  const headerSearch = page.locator('[data-search-placement="header"] input');
  const pageSearch = page.locator('[data-search-placement="page"] input');
  await expect(pageSearch).toBeVisible();
  await expect(headerSearch).not.toBeVisible();
  await expect(page.locator('header')).toContainText(/USD\s*0.7126/);
  await expect(page.locator('header')).toContainText('+4.44%');
  await page.keyboard.press('Control+k');
  await expect(pageSearch).toBeFocused();
  await pageSearch.blur();
  await page.evaluate(() => window.scrollTo(0, 700));
  await expect(headerSearch).toBeVisible();
  expect(
    await page.locator('[data-site-navigation]').evaluate((el) => el.getBoundingClientRect().bottom)
  ).toBeLessThan(0);
  expect(
    await page.locator('[data-site-header]').evaluate((el) => el.getBoundingClientRect().top)
  ).toBe(0);
  await page.keyboard.press('Control+k');
  await expect(headerSearch).toBeFocused();
  await headerSearch.fill('233635');
  await headerSearch.press('Enter');
  await expect(page).toHaveURL(/\/block\/233635$/);
  await expect(page.getByText('Additional block details')).toBeVisible();
  await noOverflow(page);
});

test('horizontal menus support keyboard navigation, active sections, and escape', async ({
  page,
}) => {
  await page.goto('/settings');
  const blockchain = page.getByRole('button', { name: 'Blockchain', exact: true });
  await blockchain.focus();
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('menu', { name: 'Blockchain', exact: true })).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL('/transactions/1');
  await expect(page.getByRole('menu')).toHaveCount(0);
  await blockchain.click();
  await expect(page.getByRole('menuitem', { name: /^Transactions Latest/ })).toHaveAttribute(
    'aria-current',
    'page'
  );
  await page.keyboard.press('Escape');
  await expect(blockchain).toBeFocused();
  await page.getByRole('button', { name: 'Network: QRL Testnet v2' }).click();
  const currentNetwork = page.getByRole('menuitem', { name: 'QRL Testnet v2', exact: true });
  await expect(currentNetwork).toHaveAttribute('aria-current', 'true');
  const upcomingNetwork = page.getByRole('menuitem', { name: /QRL Testnet v3 Upcoming/ });
  await expect(upcomingNetwork).toBeDisabled();
  await expect(upcomingNetwork).not.toHaveAttribute('href');
  await expect(page.getByRole('menuitem', { name: /QRL Mainnet/ })).toBeDisabled();
  await page.keyboard.press('Escape');
});

test('all appearance modes persist and auto follows changes to the system theme', async ({
  page,
}) => {
  await page.goto('/settings');
  for (const theme of ['light', 'dim', 'dark']) {
    await appearance(page, theme[0].toUpperCase() + theme.slice(1));
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
    await page.screenshot({ path: `test-results/settings-${theme}.png`, fullPage: true });
  }
  await page.getByRole('radio', { name: 'Auto (system)', exact: true }).focus();
  await page.keyboard.press('Space');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.getByRole('radio', { name: 'Auto (system)', exact: true })).toBeChecked();
});

test('saved reading settings apply on routes and survive reload', async ({ page }) => {
  await page.goto('/settings');
  await page.getByLabel('Address display', { exact: true }).selectOption('back');
  await page.getByLabel('Date and time', { exact: true }).selectOption('local');
  await page.getByRole('switch', { name: 'Expand technical details' }).click();
  await page.goto(`/tx/${hash}`);
  await expect(page.locator('time').first()).toContainText('14:30:00');
  await expect(page.locator('details').first()).toHaveAttribute('open', '');
  await page.locator('details summary').first().click();
  await expect(page.locator('details').first()).not.toHaveAttribute('open');
  await page.reload();
  await expect(page.locator('details').first()).toHaveAttribute('open', '');
  await page.goto('/transactions/1');
  await expect(
    page.locator(`[data-explorer-address="${address.toLowerCase()}"]`).first()
  ).toHaveText('Qaaaaaaaaaaaaa...');
  const match = page.locator(`[data-explorer-address="${address.toLowerCase()}"]`).first();
  await match.hover();
  expect(await page.locator('[data-address-highlight]').count()).toBeGreaterThan(1);
  await page.getByRole('link', { name: 'Site settings', exact: true }).click();
  await page.getByRole('switch', { name: 'Highlight matching addresses' }).click();
  await page.goto('/transactions/1');
  await page.locator('[data-explorer-address]').first().hover();
  await expect(page.locator('[data-address-highlight]')).toHaveCount(0);
});

test('zero token transfers are disclosed and recoverable, NFT ID zero stays visible', async ({
  page,
}) => {
  await page.goto(`/tx/${hash}`);
  await expect(page.getByText('1 zero-quantity token transfer hidden.')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'NFT Transfer', exact: true })).toBeVisible();
  await expect(page.getByText('#0', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zero demo (ZERO)', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Show zero transfers' }).click();
  await expect(page.getByRole('link', { name: 'Zero demo (ZERO)', exact: true })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('link', { name: 'Zero demo (ZERO)', exact: true })).toBeVisible();
  await page.goto(`/address/${address}?tab=token-transfers`);
  await expect(
    page.getByRole('tabpanel', { name: /Token Transfers/ }).getByText('ZERO', { exact: true })
  ).toBeVisible();
});

test('mobile navigation expands inline, closes on route selection, and stays within the screen', async ({
  page,
}) => {
  test.setTimeout(60_000);
  for (const width of [360, 390, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/settings');
    await expect(page.getByRole('heading', { name: 'Site settings', exact: true })).toBeVisible();
    await noOverflow(page);
    if (width < 1024) {
      await page.getByRole('button', { name: 'Open navigation' }).click();
      const navigation = page.getByRole('navigation', { name: 'Mobile navigation' });
      await expect(navigation).toBeVisible();
      await expect(page.getByRole('dialog')).toHaveCount(0);
      await expect(page.getByRole('link', { name: 'ZondScan home' })).toBeVisible();
      expect(
        await page.locator('#main-content').evaluate((el) => el.getBoundingClientRect().top)
      ).toBeGreaterThanOrEqual(
        await navigation.evaluate((el) => el.getBoundingClientRect().bottom)
      );
      await navigation.getByRole('button', { name: 'Blockchain', exact: true }).click();
      await navigation.getByRole('link', { name: 'Transactions', exact: true }).click();
      await expect(page).toHaveURL('/transactions/1', { timeout: 20_000 });
      await expect(navigation).not.toBeVisible();
      await noOverflow(page);
      await page.screenshot({ path: `test-results/transactions-${width}.png`, fullPage: true });
      await page.getByRole('button', { name: 'Open navigation' }).click();
      await page.keyboard.press('Escape');
      await expect(page.getByRole('button', { name: 'Open navigation' })).toBeFocused();
    }
  }
});

test('preferences synchronize between tabs and reset cleanly', async ({ page, context }) => {
  await page.goto('/settings');
  const other = await context.newPage();
  await other.goto('/settings');
  await appearance(page, 'Light');
  await expect(other.locator('html')).toHaveAttribute('data-theme', 'light');
  await page.getByRole('button', { name: 'Reset defaults' }).click();
  await expect(other.locator('html')).toHaveAttribute('data-theme', 'dark');
  await other.close();
});

test('blocked or malformed storage keeps the controls usable', async ({ page }) => {
  await page.addInitScript((storageKey) => {
    localStorage.setItem(storageKey, '{invalid');
  }, key);
  await page.goto('/settings');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.evaluate(() => {
    Storage.prototype.setItem = () => {
      throw new DOMException('Blocked', 'SecurityError');
    };
    Storage.prototype.getItem = () => {
      throw new DOMException('Blocked', 'SecurityError');
    };
  });
  await page.getByRole('radio', { name: 'Light', exact: true }).focus();
  await page.keyboard.press('Space');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await expect(
    page.getByRole('status').filter({ hasText: 'Browser storage is unavailable' })
  ).toBeVisible();
  await page.getByRole('link', { name: 'ZondScan home' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
});

test('price and gas failures show unavailable values', async ({ page }) => {
  await page.route('**/overview', (route) => route.fulfill({ status: 503, body: '{}' }));
  await page.route('**/gas/summary', (route) => route.fulfill({ status: 503, body: '{}' }));
  await page.goto('/settings');
  await expect(page.getByRole('link', { name: 'QRL: -', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Gas: -', exact: true })).toBeVisible();
});

test('resizing an open mobile menu to desktop releases the page', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/settings');
  await page.getByRole('button', { name: 'Open navigation' }).click();
  await expect(page.getByRole('navigation', { name: 'Mobile navigation' })).toBeVisible();
  await page.setViewportSize({ width: 1440, height: 1000 });
  await expect(page.getByRole('navigation', { name: 'Mobile navigation' })).toHaveCount(0);
  await page.getByRole('button', { name: 'Blockchain', exact: true }).click();
  await expect(page.getByRole('menu', { name: 'Blockchain', exact: true })).toBeVisible();
});

test('light appearance keeps code examples readable and updates embedded charts', async ({
  page,
}) => {
  await page.goto('/');
  await expect(page.locator('#tradingview_qrl')).toHaveAttribute('data-widget-theme', 'dark');
  await appearance(page, 'Light');
  await expect(page.locator('#tradingview_qrl')).toHaveAttribute('data-widget-theme', 'light');
  await page.goto('/learn/deploy-and-verify-contract');
  await expect(page.getByRole('button', { name: 'Appearance: Light' })).toBeVisible();
  const code = page.locator('.learn-codeblock').first();
  await expect(code).toHaveCSS('background-color', 'rgb(237, 241, 247)');
  await expect(code.locator('code')).toHaveCSS('color', 'rgb(23, 34, 56)');
  expect(
    await page.locator('body').evaluate((el) => getComputedStyle(el).backgroundImage)
  ).toContain('rgb(246, 248, 252)');
});

test('changing zero filtering resets a searched transfer page', async ({ page, context }) => {
  const transfers = Array.from({ length: 40 }, (_, index) => ({
    contractAddress: address,
    from: address,
    to: `Q${'b'.repeat(40)}`,
    amount: index < 15 ? '0' : '1000000000000000000',
    tokenStandard: 'ERC-20',
    tokenSymbol: 'DEMO',
    tokenName: 'Demo token',
    tokenDecimals: 18,
    timestamp: '1789043400',
    blockNumber: '233635',
    txHash: index < 20 ? hash : `0x${'f'.repeat(64)}`,
    logIndex: String(index),
    transferType: 'transfer',
  }));
  await page.route('**/address/*/token-transfers?*', (route) =>
    route.fulfill({ json: { transfers, total: transfers.length } })
  );
  await page.goto('/settings');
  await page.getByRole('switch', { name: 'Hide zero-quantity token transfers' }).click();
  await page.goto(`/address/${address}?tab=token-transfers&ttPage=2`);
  await expect(
    page.getByRole('table', { name: 'Token transfers' }).locator('tbody tr')
  ).toHaveCount(10);
  await expect(page).toHaveURL(/ttPage=2/);
  await page.reload();
  await expect(
    page.getByRole('table', { name: 'Token transfers' }).locator('tbody tr')
  ).toHaveCount(10);
  await expect(page).toHaveURL(/ttPage=2/);
  await page.getByPlaceholder('Search token transfers...').fill(hash);
  await expect(
    page.getByRole('table', { name: 'Token transfers' }).locator('tbody tr')
  ).toHaveCount(10);
  await page.getByRole('button', { name: 'Go to next page', exact: true }).click();
  await expect(page).toHaveURL(/ttPage=2/);
  const settings = await context.newPage();
  await settings.goto('/settings');
  await settings.getByRole('switch', { name: 'Hide zero-quantity token transfers' }).click();
  await expect(
    page.getByRole('table', { name: 'Token transfers' }).locator('tbody tr')
  ).toHaveCount(5);
  await expect(page).not.toHaveURL(/ttPage=2/);
  await settings.close();
});

test('desktop hover menus bridge the gap, switch sections, and close on scroll', async ({
  page,
}) => {
  await page.goto('/settings');
  const blockchain = page.getByRole('button', { name: 'Blockchain', exact: true });
  await blockchain.hover();
  const menu = page.getByRole('menu', { name: 'Blockchain', exact: true });
  await expect(menu).toBeVisible();
  const triggerBounds = await blockchain.boundingBox();
  await page.mouse.move(
    triggerBounds!.x + triggerBounds!.width / 2,
    triggerBounds!.y + triggerBounds!.height + 5
  );
  await expect(menu).toBeVisible();
  await page.getByRole('menuitem', { name: /^Transactions Latest/ }).hover();
  await expect(menu).toBeVisible();
  await page.getByRole('button', { name: 'Tools', exact: true }).hover();
  await expect(page.getByRole('menu', { name: 'Tools', exact: true })).toBeVisible();
  await expect(menu).toHaveCount(0);
  await page.evaluate(() => scrollTo(0, 400));
  await expect(page.getByRole('menu')).toHaveCount(0);
  expect(
    await page.locator('[data-site-header]').evaluate((el) => el.getBoundingClientRect().top)
  ).toBe(0);
});

test('highlights activate only on addresses and clear over surrounding content', async ({
  page,
}) => {
  await page.goto('/transactions/1');
  await page
    .locator('tbody tr')
    .first()
    .hover({ position: { x: 4, y: 4 } });
  await expect(page.locator('[data-address-highlight]')).toHaveCount(0);
  const label = page.locator('[data-explorer-address]').first();
  await label.hover();
  expect(await page.locator('[data-address-highlight]').count()).toBeGreaterThan(1);
  await page.locator('#main-content').hover({ position: { x: 5, y: 5 } });
  await expect(page.locator('[data-address-highlight]')).toHaveCount(0);
  await label.locator('..').focus();
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Tab');
  expect(await page.locator('[data-address-highlight]').count()).toBeGreaterThan(1);
});

test('the footer follows block content without a second viewport of padding', async ({ page }) => {
  await page.goto('/block/233635');
  await expect(page.getByText('Additional block details')).toBeVisible();
  const gap = await page.evaluate(() => {
    const content = document.querySelector('main[aria-labelledby="block-heading"]')!;
    return (
      document.querySelector('footer')!.getBoundingClientRect().top -
      content.getBoundingClientRect().bottom
    );
  });
  expect(gap).toBeLessThan(100);
});

test('language selection translates controls, persists, and updates document language', async ({
  page,
}) => {
  await page.goto('/settings');
  for (const locale of ['zh', 'es', 'ru']) {
    await page.locator('#interface-language').selectOption(locale);
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await expect(page.locator('h1')).not.toHaveText('Site settings');
    await expect(page.locator('[data-site-navigation] nav').first()).not.toContainText(
      'Blockchain'
    );
    await page.reload();
    await expect(page.locator('#interface-language')).toHaveValue(locale);
    await expect(page.locator('html')).toHaveAttribute('lang', locale);
    await noOverflow(page);
  }
  await page.goto('/learn/deploy-and-verify-contract');
  await expect(page.locator('html')).toHaveAttribute('lang', 'ru');
  await expect(page.locator('#main-content')).toHaveAttribute('lang', 'en');
  await page.goto('/settings');
  await page.locator('#interface-language').selectOption('en');
  await expect(page.getByRole('heading', { name: 'Site settings', exact: true })).toBeVisible();
});

test('currency selection converts live estimates and clearly falls back to USD on errors', async ({
  page,
}) => {
  const snapshot = {
    base: 'USD',
    date: new Date().toISOString().slice(0, 10),
    rates: { USD: 1, EUR: 0.9, GBP: 0.8, CHF: 0.85, CAD: 1.3, AUD: 1.5, JPY: 150, CNY: 7 },
  };
  await page.route('**/api/exchange-rates', (route) => route.fulfill({ json: snapshot }));
  await page.goto('/settings');
  await page.getByLabel('Currency', { exact: true }).selectOption('EUR');
  await expect(page.locator('[data-display-currency]')).toHaveAttribute(
    'data-display-currency',
    'EUR'
  );
  await expect(page.locator('[data-display-currency]')).toContainText('0.6413');
  await page.reload();
  await expect(page.getByLabel('Currency', { exact: true })).toHaveValue('EUR');
  await expect(page.locator('[data-display-currency]')).toHaveAttribute(
    'data-display-currency',
    'EUR'
  );
  await page.goto(`/tx/${hash}`);
  await expect(page.locator('[data-fee-currency]')).toHaveAttribute('data-fee-currency', 'EUR');
  await expect(page.locator('[data-fee-currency]')).toContainText('EUR');
  await page.goto('/settings');
  await page.unroute('**/api/exchange-rates');
  await page.route('**/api/exchange-rates', (route) => route.fulfill({ status: 503, json: {} }));
  await page.reload();
  await expect(page.locator('[data-display-currency]')).toHaveAttribute(
    'data-display-currency',
    'USD'
  );
  await expect(page.locator('header')).toContainText('USD fallback');
  await expect(page.getByLabel('Currency', { exact: true })).toHaveValue('EUR');
  await page.goto(`/tx/${hash}`);
  await expect(page.locator('[data-fee-currency]')).toHaveAttribute('data-fee-currency', 'USD');
});
