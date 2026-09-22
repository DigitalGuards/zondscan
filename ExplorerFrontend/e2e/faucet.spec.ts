import { expect, test, type Page } from '@playwright/test';

const SITE_KEY = 'faucet-runtime-key-fixture';
const status = {
  configured: true,
  captchaEnabled: true,
  turnstileSiteKey: SITE_KEY,
  dripQuanta: '100',
  cooldownHours: 24,
};

// Keep these browser fixtures local and intercept every claim request. They
// exercise widget integration without contacting CAPTCHA or sending funds.
async function fixture(page: Page, payload: unknown = status, scriptFails = false) {
  let posts = 0;
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const response = await page.request.get(
      'http://127.0.0.1:18091' + url.pathname.slice('/api'.length) + url.search
    );
    await route.fulfill({ response });
  });
  await page.route('**/faucet/claim', async (route) => {
    if (route.request().method() !== 'GET') {
      posts++;
      await route.abort('blockedbyclient');
      return;
    }
    await route.fulfill({ json: payload });
  });
  await page.route('https://challenges.cloudflare.com/**', async (route) => {
    if (scriptFails) {
      await route.abort('failed');
      return;
    }
    await route.fulfill({
      contentType: 'application/javascript',
      body: `
        window.__faucetTest = { keys: [], renders: 0, removed: 0 };
        window.turnstile = {
          render(element, options) {
            const state = window.__faucetTest;
            state.keys.push(options.sitekey);
            state.renders++;
            state.solve = () => options.callback('fixture-token');
            state.expire = () => options['expired-callback']();
            element.setAttribute('data-testid', 'captcha-fixture');
            element.textContent = 'Verification fixture';
            return 'fixture-widget-' + state.renders;
          },
          reset() {},
          remove() { window.__faucetTest.removed++; }
        };
      `,
    });
  });
  return () => expect(posts).toBe(0);
}

test.beforeEach(async ({ baseURL }) => {
  expect(['127.0.0.1', 'localhost', '[::1]']).toContain(new URL(baseURL!).hostname);
});

for (const width of [390, 1280]) {
  test(`runtime CAPTCHA key gates claims at ${width}px`, async ({ page }) => {
    const assertNoClaims = await fixture(page);
    await page.setViewportSize({ width, height: 900 });
    await page.goto('/faucet');
    await expect(page).toHaveTitle('QRL Testnet v3 Faucet | ZondScan');
    await expect(page.getByTestId('captcha-fixture')).toBeVisible();
    const submit = page.getByRole('button', { name: 'Request testnet Quanta' });
    await page
      .getByRole('textbox', { name: 'QRL address', exact: true })
      .fill('Q' + '1'.repeat(128));
    await expect(submit).toBeDisabled();
    expect(
      await page.evaluate(() => {
        const state = (window as unknown as { __faucetTest: { keys: string[] } }).__faucetTest;
        return state.keys;
      })
    ).toEqual([SITE_KEY]);
    await page.evaluate(() =>
      (window as unknown as { __faucetTest: { solve(): void } }).__faucetTest.solve()
    );
    await expect(submit).toBeEnabled();
    await page.evaluate(() =>
      (window as unknown as { __faucetTest: { expire(): void } }).__faucetTest.expire()
    );
    await expect(submit).toBeDisabled();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
      true
    );
    assertNoClaims();
  });
}

test('missing runtime CAPTCHA key displays a fail-closed error', async ({ page }) => {
  const assertNoClaims = await fixture(page, { ...status, turnstileSiteKey: null });
  await page.goto('/faucet');
  await expect(
    page.getByRole('main', { name: 'QRL Testnet v3 Faucet' }).getByRole('alert')
  ).toContainText('Captcha is unavailable. Please try again later.');
  await expect(page.getByRole('button', { name: 'Request testnet Quanta' })).toBeDisabled();
  await expect(page.getByTestId('captcha-fixture')).toHaveCount(0);
  assertNoClaims();
});

test('blocked CAPTCHA script reports recovery instead of a silent disabled button', async ({
  page,
}) => {
  const assertNoClaims = await fixture(page, status, true);
  await page.goto('/faucet');
  await expect(
    page.getByRole('main', { name: 'QRL Testnet v3 Faucet' }).getByRole('alert')
  ).toContainText('Captcha could not load. Please reload the page.');
  await expect(page.getByRole('button', { name: 'Request testnet Quanta' })).toBeDisabled();
  assertNoClaims();
});

test('malformed status fails closed without displaying a claim form', async ({ page }) => {
  const assertNoClaims = await fixture(page, { configured: 'yes' });
  await page.goto('/faucet');
  await expect(
    page.getByRole('main', { name: 'QRL Testnet v3 Faucet' }).getByRole('alert')
  ).toContainText('Faucet status is unavailable. Please reload the page.');
  await expect(page.getByRole('button', { name: 'Request testnet Quanta' })).toHaveCount(0);
  assertNoClaims();
});
