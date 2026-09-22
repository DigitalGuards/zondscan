import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

const hash = (id: number) => `0x${id.toString(16).padStart(64, '0')}`;
const sender = `Q${'a'.repeat(128)}`;
const recipient = `Q${'b'.repeat(128)}`;
const pageErrors = new WeakMap<Page, string[]>();

async function fixtureState(request: APIRequestContext, id: number, status = 'pending') {
  const response = await request.post(
    `http://127.0.0.1:18091/__fixture/pending/${hash(id)}?status=${status}`
  );
  expect(response.status()).toBe(204);
}

async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test.beforeEach(async ({ page, baseURL }) => {
  const errors: string[] = [];
  pageErrors.set(page, errors);
  page.on('pageerror', (error) => errors.push(error.message));
  expect(new URL(baseURL!).hostname).toBe('127.0.0.1');
  await page.route('**/*', (route) => {
    const hostname = new URL(route.request().url()).hostname;
    return ['127.0.0.1', 'localhost', '[::1]'].includes(hostname)
      ? route.continue()
      : route.abort('blockedbyclient');
  });
  await page.addInitScript(() => {
    Object.defineProperty(window, '__pendingCopies', { value: [] });
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: {
        writeText: async (value: string) => {
          (window as unknown as { __pendingCopies: string[] }).__pendingCopies.push(value);
        },
      },
    });
  });
});

test.afterEach(async ({ page }) => {
  expect(pageErrors.get(page)).toEqual([]);
});

for (const width of [390, 1280]) {
  test(`pending transfer shares the mined layout and transitions at ${width}px`, async ({
    page,
    request,
  }, testInfo) => {
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await fixtureState(request, 21);
    await page.setViewportSize({ width, height: 950 });
    await page.goto(`/tx/${hash(21)}`);
    await expect(page).toHaveURL((url) => url.pathname === `/pending/tx/${hash(21)}`);
    const card = page.getByRole('region', { name: 'Transaction Details', exact: true });
    const heading = card.getByRole('heading', { name: 'Transaction Details', exact: true });
    await expect(heading).toBeVisible();
    await expect(card.getByText('Transfer', { exact: true })).toBeVisible();
    await expect(card.getByText('Pending', { exact: true })).toBeVisible();
    await expect(card.getByText('Awaiting block inclusion', { exact: true })).toBeVisible();
    for (const absent of ['Block', 'Timestamp', 'Transaction Fee', 'Confirmed']) {
      await expect(card.getByText(absent, { exact: true })).toHaveCount(0);
    }
    const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
    const links = [
      flow.getByRole('link', { name: sender, exact: true }),
      flow.getByRole('link', { name: recipient, exact: true }),
    ];
    await expect(links[0]).toHaveAttribute('href', `/address/${sender}`);
    await expect(links[1]).toHaveAttribute('href', `/address/${recipient}`);
    await flow.getByRole('button', { name: 'Copy sender address', exact: true }).click();
    await flow.getByRole('button', { name: 'Copy recipient address', exact: true }).click();
    expect(
      await page.evaluate(
        () => (window as unknown as { __pendingCopies: string[] }).__pendingCopies
      )
    ).toEqual([sender, recipient]);
    const boxes = await Promise.all(links.map((link) => link.boundingBox()));
    if (width === 390) {
      expect(boxes[1]!.y).toBeGreaterThan(boxes[0]!.y + boxes[0]!.height);
      for (const link of links) {
        const size = await link.locator('[aria-hidden="true"]').evaluate((element) => ({
          height: element.getBoundingClientRect().height,
          line: parseFloat(getComputedStyle(element).lineHeight),
        }));
        expect(size.height).toBeLessThanOrEqual(size.line + 1);
      }
    } else expect(Math.abs(boxes[0]!.y - boxes[1]!.y)).toBeLessThanOrEqual(2);
    await noOverflow(page);
    await page.screenshot({
      path: testInfo.outputPath(`pending-${width}-dark.png`),
      fullPage: true,
      animations: 'disabled',
    });
    await page.getByRole('button', { name: /^Appearance:/ }).click();
    await page.getByRole('menuitem', { name: /^Light/ }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await page.screenshot({
      path: testInfo.outputPath(`pending-${width}-light.png`),
      fullPage: true,
      animations: 'disabled',
    });
    const pendingBackground = await card.evaluate(
      (element) => getComputedStyle(element).backgroundImage
    );
    const pendingTypeColor = await card
      .getByText('Transfer', { exact: true })
      .evaluate((element) => getComputedStyle(element.parentElement!).color);
    await page.reload();
    await expect(card.getByText('Pending', { exact: true })).toBeVisible();
    await fixtureState(request, 21, 'mined');
    await expect(page).toHaveURL((url) => url.pathname === `/tx/${hash(21)}`, { timeout: 12000 });
    await expect(card.getByText('Confirmed', { exact: true })).toBeVisible();
    await expect(card.getByText('Pending', { exact: true })).toHaveCount(0);
    await expect(card.getByText('Transaction Fee', { exact: true })).toBeVisible();
    await expect(card.getByText('Timestamp', { exact: true })).toBeVisible();
    await expect(links[0]).toHaveAttribute('href', `/address/${sender}`);
    await expect(links[1]).toHaveAttribute('href', `/address/${recipient}`);
    expect(await card.evaluate((element) => getComputedStyle(element).backgroundImage)).toBe(
      pendingBackground
    );
    expect(
      await card
        .getByText('Transfer', { exact: true })
        .evaluate((element) => getComputedStyle(element.parentElement!).color)
    ).toBe(pendingTypeColor);
    await noOverflow(page);
    expect(errors).toEqual([]);
  });

  for (const scenario of [
    { id: 22, kind: 'Contract creation' },
    { id: 23, kind: 'Contract call' },
    { id: 24, kind: 'Contract call' },
    { id: 25, kind: 'Transaction' },
    { id: 28, kind: 'Transaction' },
  ]) {
    test(`pending ${scenario.id} preserves intent at ${width}px`, async ({ page, request }) => {
      await fixtureState(request, scenario.id);
      await page.setViewportSize({ width, height: 950 });
      await page.goto(`/pending/tx/${hash(scenario.id)}`);
      const card = page.getByRole('region', { name: 'Transaction Details', exact: true });
      const header = card
        .getByRole('heading', { name: 'Transaction Details', exact: true })
        .locator('../..');
      await expect(header.getByText(scenario.kind, { exact: true })).toBeVisible();
      await expect(card.getByText('Pending', { exact: true })).toBeVisible();
      const flow = page.getByRole('region', { name: 'Transaction flow', exact: true });
      if (scenario.id === 22) {
        await expect(flow.getByText('Available after confirmation', { exact: true })).toBeVisible();
        await expect(flow.getByRole('link')).toHaveCount(1);
        await expect(flow.getByRole('button', { name: 'Copy contract address' })).toHaveCount(0);
      } else {
        await expect(flow.getByRole('link', { name: recipient, exact: true })).toHaveAttribute(
          'href',
          `/address/${recipient}`
        );
      }
      if (scenario.id === 23) {
        await expect(
          page.getByRole('heading', { name: 'Token Transfer (Pending)', exact: true })
        ).toBeVisible();
        await expect(page.getByText('transfer(address, uint256)', { exact: true })).toBeVisible();
        await expect(page.getByText(/assumes 18 decimals/)).toBeVisible();
      }
      await noOverflow(page);
    });
  }

  test(`temporary lookup failure retains the flow and recovers at ${width}px`, async ({
    page,
    request,
  }) => {
    await fixtureState(request, 27);
    await page.setViewportSize({ width, height: 950 });
    await page.goto(`/pending/tx/${hash(27)}`);
    await expect(page.getByRole('region', { name: 'Transaction flow', exact: true })).toBeVisible();
    await fixtureState(request, 27, 'unavailable');
    await expect(
      page.getByText('Transaction status is temporarily unavailable. Checking again shortly.')
    ).toBeVisible({ timeout: 12000 });
    await expect(page.getByRole('region', { name: 'Transaction flow', exact: true })).toBeVisible();
    await fixtureState(request, 27);
    await expect(
      page.getByText('Transaction status is temporarily unavailable. Checking again shortly.')
    ).toHaveCount(0, { timeout: 12000 });
  });

  test(`dropped transaction stays distinct from confirmation at ${width}px`, async ({
    page,
    request,
  }) => {
    await fixtureState(request, 26);
    await page.setViewportSize({ width, height: 950 });
    await page.goto(`/pending/tx/${hash(26)}`);
    await expect(page.getByRole('region', { name: 'Transaction flow', exact: true })).toBeVisible();
    await fixtureState(request, 26, 'dropped');
    await expect(
      page.getByRole('heading', { name: 'Transaction Not Found', exact: true })
    ).toBeVisible({ timeout: 12000 });
    await expect(page.getByText('Confirmed', { exact: true })).toHaveCount(0);
    await expect(page).toHaveURL((url) => url.pathname === `/pending/tx/${hash(26)}`);
  });
}
